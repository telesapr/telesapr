package ttf

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"io"
	"slices"
)

// WOFF files (sfwoff.c's woff_open_font): the SFNT is rebuilt in memory,
// uncompressing tables with zlib, and read from there.

type woffTable struct {
	tag                            uint32
	offset, compLength, origLength uint32
	checkSum                       uint32
	origOffset                     uint32
}

// woffOpenFont is woff_open_font: the SFNT a WOFF file holds.
func woffOpenFont(data []byte) ([]byte, error) {
	if len(data) < 44 {
		return nil, errInvalidStream
	}
	be := binary.BigEndian
	flavor := be.Uint32(data[4:])
	length := be.Uint32(data[8:])
	numTables := uint32(be.Uint16(data[12:]))
	totalSfntSize := be.Uint32(data[16:])
	metaOffset := be.Uint32(data[24:])
	metaLength := be.Uint32(data[28:])
	metaOrigLength := be.Uint32(data[32:])
	privOffset := be.Uint32(data[36:])
	privLength := be.Uint32(data[40:])

	if flavor == tag("wOFF") || flavor == tag("ttcf") {
		return nil, errInvalidTable
	}
	if uint64(length) != uint64(len(data)) ||
		numTables == 0 || numTables > 0xFFF ||
		44+uint64(numTables)*20 >= uint64(length) ||
		12+uint64(numTables)*16 >= uint64(totalSfntSize) ||
		totalSfntSize&3 != 0 ||
		(metaOffset == 0 && (metaLength != 0 || metaOrigLength != 0)) ||
		(metaLength != 0 && metaOrigLength == 0) ||
		(privOffset == 0 && privLength != 0) {
		return nil, errInvalidTable
	}

	header := make([]byte, 12)
	entrySelector := msb32(numTables)
	searchRange := (1 << entrySelector) * 16
	rangeShift := int(numTables)*16 - searchRange
	be.PutUint32(header[0:], flavor)
	be.PutUint16(header[4:], uint16(numTables))
	be.PutUint16(header[6:], uint16(searchRange))
	be.PutUint16(header[8:], uint16(entrySelector))
	be.PutUint16(header[10:], uint16(rangeShift))

	s := &cffStream{data: data, pos: 44}
	dir, err := s.frame(20 * uint64(numTables))
	if err != nil {
		return nil, err
	}
	tables := make([]woffTable, numTables)
	indices := make([]*woffTable, numTables)
	var oldTag uint32
	for nn := range tables {
		t := &tables[nn]
		p := dir[20*nn:]
		t.tag = be.Uint32(p)
		t.offset = be.Uint32(p[4:])
		t.compLength = be.Uint32(p[8:])
		t.origLength = be.Uint32(p[12:])
		t.checkSum = be.Uint32(p[16:])
		if t.tag <= oldTag {
			return nil, errInvalidTable
		}
		oldTag = t.tag
		indices[nn] = t
	}
	// glibc's qsort is a merge sort; the offsets are distinct anyway
	// once they pass the checks below.
	slices.SortStableFunc(indices, func(a, b *woffTable) int { return cmp.Compare(a.offset, b.offset) })

	woffOffset := 44 + uint64(numTables)*20
	sfntOffset := 12 + uint64(numTables)*16
	for _, t := range indices {
		if uint64(t.offset) != woffOffset ||
			t.compLength > length ||
			t.offset > length-t.compLength ||
			t.origLength > totalSfntSize ||
			sfntOffset > uint64(totalSfntSize-t.origLength) ||
			t.compLength > t.origLength {
			return nil, errInvalidTable
		}
		t.origOffset = uint32(sfntOffset)
		woffOffset += uint64(t.compLength+3) &^ 3
		sfntOffset += uint64(t.origLength+3) &^ 3
	}

	if metaOffset != 0 {
		if uint64(metaOffset) != woffOffset || uint64(metaOffset)+uint64(metaLength) > uint64(length) {
			return nil, errInvalidTable
		}
		woffOffset += uint64(metaLength)
	}
	if privOffset != 0 {
		woffOffset = (woffOffset + 3) &^ 3
		if uint64(privOffset) != woffOffset || uint64(privOffset)+uint64(privLength) > uint64(length) {
			return nil, errInvalidTable
		}
		woffOffset += uint64(privLength)
	}
	if sfntOffset != uint64(totalSfntSize) || woffOffset != uint64(length) {
		return nil, errInvalidTable
	}

	sfnt := make([]byte, totalSfntSize)
	copy(sfnt, header)
	entry := sfnt[12:]
	for nn := range tables {
		t := &tables[nn]
		be.PutUint32(entry[0:], t.tag)
		be.PutUint32(entry[4:], t.checkSum)
		be.PutUint32(entry[8:], t.origOffset)
		be.PutUint32(entry[12:], t.origLength)
		entry = entry[16:]

		if err := s.seek(int64(t.offset)); err != nil {
			return nil, err
		}
		comp, err := s.frame(uint64(t.compLength))
		if err != nil {
			return nil, err
		}
		out := sfnt[t.origOffset : t.origOffset+t.origLength]
		if t.compLength == t.origLength {
			copy(out, comp)
		} else if err := gzipUncompress(out, comp); err != nil {
			return nil, err
		}
		// the padding is zero already
	}
	return sfnt, nil
}

// gzipUncompress is FT_Gzip_Uncompress into out, which the data must
// fill exactly: zlib's inflate with a zlib or gzip header (MAX_WBITS|32),
// which stops after the first gzip member.
func gzipUncompress(out, input []byte) error {
	var r io.Reader
	if len(input) >= 2 && input[0] == 0x1F && input[1] == 0x8B {
		zr, err := gzip.NewReader(bytes.NewReader(input))
		if err != nil {
			return errInvalidTable
		}
		zr.Multistream(false)
		r = zr
	} else {
		zr, err := zlib.NewReader(bytes.NewReader(input))
		if err != nil {
			return errInvalidTable
		}
		r = zr
	}
	got, err := io.ReadAll(io.LimitReader(r, int64(len(out))+1))
	if err != nil || len(got) != len(out) {
		return errInvalidTable
	}
	// the stream must end here, with its checksum checked
	if _, err := r.Read(make([]byte, 1)); err != io.EOF {
		return errInvalidTable
	}
	copy(out, got)
	return nil
}

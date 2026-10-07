// Package ttf reads fonts as C++ SolveSpace's ttf.cpp does, through the
// parts of FreeType 2.14.3 it uses: opening a face and selecting its
// Unicode charmap, a size request, loading unhinted glyph outlines,
// kerning, and outline decomposition. FreeType's drivers for scalable
// fonts are ported: TrueType, Type 1, CFF (OpenType CFF and CFF2 fonts,
// and bare CFF files), PFR and Type 42; its CID-keyed Type 1 driver and
// its bitmap font drivers aren't, since ttf.cpp can't use those fonts.
package ttf

import (
	"encoding/binary"
	"errors"
)

var (
	errUnknownFormat = errors.New("ttf: unknown file format")
	errInvalidTable  = errors.New("ttf: invalid table")
	errTableMissing  = errors.New("ttf: table missing")
	errInvalidArg    = errors.New("ttf: invalid argument")
	errDivideByZero  = errors.New("ttf: divide by zero")
	errInvalidPpem   = errors.New("ttf: invalid pixel size")
	errInvalidGlyph  = errors.New("ttf: invalid glyph index")
	errInvalidOutl   = errors.New("ttf: invalid outline")
	errInvalidComp   = errors.New("ttf: invalid composite glyph")
	errTooManyHints  = errors.New("ttf: too many hints")
	errNoCharmap     = errors.New("ttf: invalid charmap handle")
)

func tag(s string) uint32 { return binary.BigEndian.Uint32([]byte(s)) }

// reader reads big-endian values from a byte slice; reads past its end
// fail, as FreeType's stream reads do.
type reader struct {
	b   []byte
	pos int
	err error
}

func (r *reader) need(n int) bool {
	if r.err != nil {
		return false
	}
	if r.pos < 0 || r.pos+n > len(r.b) {
		r.err = errInvalidTable
		return false
	}
	return true
}

func (r *reader) u16() uint16 {
	if !r.need(2) {
		return 0
	}
	v := binary.BigEndian.Uint16(r.b[r.pos:])
	r.pos += 2
	return v
}

func (r *reader) i16() int16 { return int16(r.u16()) }

func (r *reader) u32() uint32 {
	if !r.need(4) {
		return 0
	}
	v := binary.BigEndian.Uint32(r.b[r.pos:])
	r.pos += 4
	return v
}

// peek16 and peek32 read at p in b, or give 0 past its end (where
// FreeType would read whatever follows).
func peek16(b []byte, p int) uint16 {
	if p < 0 || p+2 > len(b) {
		return 0
	}
	return binary.BigEndian.Uint16(b[p:])
}

func peek32(b []byte, p int) uint32 {
	if p < 0 || p+4 > len(b) {
		return 0
	}
	return binary.BigEndian.Uint32(b[p:])
}

type tableEntry struct {
	tag            uint32
	offset, length uint32
}

// Face is an FT_Face opened by the TrueType driver, with its size
// (FT_Size) and the charmap selected.
type Face struct {
	data   []byte
	tables []tableEntry

	FamilyName, StyleName string

	// head
	unitsPerEM       int64
	headFlags        uint16
	indexToLocFormat int16

	numGlyphs int64

	// hhea and hmtx
	hheaAscender, hheaDescender, hheaLineGap int16
	advanceWidthMax                          uint16
	numberOfHMetrics                         uint16
	hmtxOffset, hmtxSize                     int64

	// OS/2; os2Version is 0xFFFF when the table is missing
	os2Version                               uint16
	fsSelection                              uint16
	typoAscender, typoDescender, typoLineGap int16
	winAscent, winDescent                    uint16

	// FT_FaceRec
	ascender, descender int64

	// glyf and loca
	glyfOffset, glyfLen int64
	loca                []byte
	numLocations        int64

	names []nameRecord

	cmapTable []byte
	charmaps  []*charmap
	charmap   *charmap

	kernTable     []byte
	numKernTables int
	kernAvailBits uint32
	kernOrderBits uint32

	// The size metrics (FT_Size_Metrics) after RequestSize.
	xScale, yScale int64
	xPpem, yPpem   int64

	// The CFF driver's font, for a CFF face.
	cff    *cffFont
	isCFF2 bool

	// post, and maxp's number of glyphs
	postFormat    int64
	postNames     postNames
	isFixedPitch  uint32
	maxpNumGlyphs int64

	// maxp's version 1.0 fields
	maxp maxProfile

	// vhea and vmtx; vertInfo is face->vertical_info
	vertInfo             bool
	vheaNumberOfMetrics  uint16
	vmtxOffset, vmtxSize int64

	// The bytecode (the TrueType driver's, for tricky fonts): the
	// unscaled CVT in 26.6, the font and CVT programs, and hdmx.
	tricky      bool
	cvt         []int32
	fontProgram []byte
	cvtProgram  []byte
	hdmxRecords [][]byte

	// The TrueType driver's size (TT_Size), created with the face.
	size *ttSize

	// The Type 1 driver's font, for a Type 1 face.
	t1 *t1Font

	// The Type 42 driver's font, for a Type 42 face.
	t42 *t42Face

	// The PFR driver's font, for a PFR face, and whether it has
	// outlines (FT_FACE_FLAG_SCALABLE).
	pfr         *pfrFace
	pfrScalable bool
}

// maxProfile is the part of a TT_MaxProfile the interpreter reads.
type maxProfile struct {
	version            int64
	maxTwilightPoints  uint16
	maxStorage         uint16
	maxFunctionDefs    uint16
	maxInstructionDefs uint16
	maxStackElements   uint16
}

// NewFace opens the first face of a font file, as FT_New_Memory_Face
// does, trying FreeType's drivers in their order: the TrueType driver
// takes TrueType fonts (and collections), the Type 1 driver PFA and PFB
// fonts, the CFF driver OpenType CFF fonts and bare CFF files, the PFR
// driver PFR fonts, and the Type 42 driver TrueType fonts in
// PostScript. (The CID-keyed Type 1 driver comes between the CFF and
// PFR drivers; the fonts it takes have no Unicode charmap, which fails
// them in ttf.cpp as their unknown format fails them here.) An error other than an
// unknown format ends the search, as in FT_Open_Face.
func NewFace(data []byte) (*Face, error) {
	// the TrueType driver (tt_face_init)
	f, err := ttFaceInit(data)
	if err == nil {
		return f, nil
	} else if !errors.Is(err, errUnknownFormat) || f == nil {
		return nil, err
	}

	// the Type 1 driver (T1_Face_Init)
	if t1f, err := t1FaceInit(f.data); err == nil {
		return t1f, nil
	} else if !errors.Is(err, errUnknownFormat) {
		return nil, err
	}

	// the CFF driver (cff_face_init), on the SFNT data of a WOFF file
	f = &Face{data: f.data}
	formatTag, sfntErr := f.initFace()
	if sfntErr == nil && formatTag != tag("OTTO") {
		return nil, errUnknownFormat
	}
	if err := f.cffFaceInit(sfntErr == nil); err == nil {
		return f, nil
	} else if !errors.Is(err, errUnknownFormat) {
		return nil, err
	}

	// the PFR driver (pfr_face_init)
	if pf, err := pfrFaceInit(f.data); err == nil {
		return pf, nil
	} else if !errors.Is(err, errUnknownFormat) {
		return nil, err
	}

	// the Type 42 driver (T42_Face_Init)
	return t42FaceInit(f.data)
}

// ttFaceInit is tt_face_init: a TrueType font as a face. For other
// fonts it reports errUnknownFormat with the face, whose data (a WOFF
// file's SFNT data) the next drivers read; an error loading a TrueType
// font comes without it, and ends the search.
func ttFaceInit(data []byte) (*Face, error) {
	f := &Face{data: data}
	formatTag, sfntErr := f.initFace()
	if sfntErr != nil {
		return f, errUnknownFormat
	}
	switch formatTag {
	case 0x00010000, 0x00020000, tag("true"), 0xA56B6264, 0xA56C7374:
	default:
		return f, errUnknownFormat
	}
	if err := f.loadFace(false); err != nil {
		return nil, err
	}
	if err := f.loadLoca(); err != nil {
		return nil, err
	}
	if err := f.loadBytecodeTables(); err != nil {
		return nil, err
	}
	return f, nil
}

// initFace is sfnt_init_face and sfnt_open_font: the table directory,
// through a collection's first font. It returns the font's format tag.
func (f *Face) initFace() (uint32, error) {
	r := &reader{b: f.data}
	offset := 0
	formatTag := r.u32()
	if r.err != nil {
		return 0, errUnknownFormat
	}
	if formatTag == tag("wOFF") || formatTag == tag("wOF2") {
		// The SFNT replaces the file, for this driver and the next.
		open := woffOpenFont
		if formatTag == tag("wOF2") {
			open = woff2OpenFont
		}
		sfnt, err := open(f.data)
		if err != nil {
			return 0, err
		}
		f.data = sfnt
		return f.initFace()
	}
	switch formatTag {
	case 0x00010000, tag("ttcf"), tag("OTTO"), tag("true"), tag("typ1"),
		0xA56B6264, 0xA56C7374, 0x00020000:
	default:
		return 0, errUnknownFormat
	}
	if formatTag == tag("ttcf") {
		r.u32() // version
		count := r.u32()
		if r.err != nil || count == 0 {
			return 0, errUnknownFormat
		}
		offset = int(r.u32())
		if r.err != nil {
			return 0, errUnknownFormat
		}
		r.pos = offset
		formatTag = r.u32()
		if r.err != nil {
			return 0, errUnknownFormat
		}
	}

	r.pos = offset + 4
	numTables := int(r.u16())
	r.pos += 6
	if r.err != nil {
		return 0, errUnknownFormat
	}
	size := uint32(len(f.data))
	for i := 0; i < numTables; i++ {
		var e tableEntry
		e.tag = r.u32()
		r.u32() // checksum
		e.offset = r.u32()
		e.length = r.u32()
		if r.err != nil {
			break
		}
		// tt_face_load_font_dir ignores tables outside the file, and
		// truncates hmtx and vmtx.
		if e.offset > size {
			continue
		} else if e.length > size-e.offset {
			if e.tag == tag("hmtx") || e.tag == tag("vmtx") {
				e.length = (size - e.offset) &^ 3
			} else {
				continue
			}
		}
		// the first of duplicate tables wins
		duplicate := false
		for _, d := range f.tables {
			if d.tag == e.tag {
				duplicate = true
				break
			}
		}
		if !duplicate {
			f.tables = append(f.tables, e)
		}
	}
	if len(f.tables) == 0 {
		return 0, errUnknownFormat
	}
	return formatTag, nil
}

// table is goto_table (tt_face_lookup_table): zero-length tables count
// as missing.
func (f *Face) table(name string) ([]byte, int64, bool) {
	t := tag(name)
	for _, e := range f.tables {
		if e.tag == t && e.length != 0 {
			return f.data[e.offset : e.offset+e.length], int64(e.offset), true
		}
	}
	return nil, 0, false
}

// loadFace is sfnt_load_face, for the fields ttf.cpp relies on, for the
// TrueType driver or the CFF driver.
func (f *Face) loadFace(cff bool) error {
	if !cff {
		if _, _, hasGlyf := f.table("glyf"); !hasGlyf {
			return errUnknownFormat
		}
	}

	_, headPos, ok := f.table("head")
	if !ok {
		return errTableMissing
	}
	head, err := f.frameAt(headPos, 54)
	if err != nil {
		return err
	}
	r := &reader{b: head}
	r.pos = 16
	f.headFlags = r.u16()
	f.unitsPerEM = int64(r.u16())
	r.pos = 50
	f.indexToLocFormat = r.i16()
	if f.unitsPerEM < 16 || f.unitsPerEM > 16384 {
		return errInvalidTable
	}

	if err := f.loadMaxp(); err != nil {
		return err
	}

	if cmap, _, ok := f.table("cmap"); ok {
		f.cmapTable = cmap
	}
	f.loadNames()
	postOK := f.loadPost()

	_, hheaPos, ok := f.table("hhea")
	if !ok {
		return errTableMissing
	}
	hhea, err := f.frameAt(hheaPos, 36)
	if err != nil {
		return err
	}
	r = &reader{b: hhea}
	r.pos = 4
	f.hheaAscender = r.i16()
	f.hheaDescender = r.i16()
	f.hheaLineGap = r.i16()
	f.advanceWidthMax = r.u16()
	r.pos = 34
	f.numberOfHMetrics = r.u16()
	_, hmtxPos, ok := f.table("hmtx")
	if !ok {
		return errTableMissing
	}
	for _, e := range f.tables {
		if e.tag == tag("hmtx") && e.length != 0 {
			f.hmtxOffset = hmtxPos
			f.hmtxSize = int64(e.length)
			break
		}
	}

	if err := f.loadVertical(); err != nil {
		return err
	}

	f.loadOS2()

	f.setNames()
	f.loadKern()

	f.buildCmaps()
	for _, cm := range f.charmaps {
		cm.encoding = findEncoding(cm.platformID, cm.encodingID)
	}
	f.synthesizeUnicodeCmap(postOK && f.postFormat != 0x00030000)

	// The metrics (sfnt_load_face's "Set up metrics").
	if f.os2Version != 0xFFFF && f.fsSelection&128 != 0 {
		f.ascender = int64(f.typoAscender)
		f.descender = int64(f.typoDescender)
	} else {
		f.ascender = int64(f.hheaAscender)
		f.descender = int64(f.hheaDescender)
		if f.ascender == 0 && f.descender == 0 && f.os2Version != 0xFFFF {
			if f.typoAscender != 0 || f.typoDescender != 0 {
				f.ascender = int64(f.typoAscender)
				f.descender = int64(f.typoDescender)
			} else {
				f.ascender = int64(int16(f.winAscent))
				f.descender = -int64(int16(f.winDescent))
			}
		}
	}
	return nil
}

// loadOS2 is tt_face_load_os2; a table that can't be read counts as
// missing, with version 0xFFFF.
func (f *Face) loadOS2() {
	f.os2Version = 0xFFFF
	_, pos, ok := f.table("OS/2")
	if !ok {
		return
	}
	os2, err := f.frameAt(pos, 78)
	if err != nil {
		return
	}
	r := &reader{b: os2}
	version := r.u16()
	r.pos = 62
	f.fsSelection = r.u16()
	r.pos = 68
	f.typoAscender = r.i16()
	f.typoDescender = r.i16()
	f.typoLineGap = r.i16()
	f.winAscent = r.u16()
	f.winDescent = r.u16()
	// the fields of versions 1, 2 and 5
	pos += 78
	for _, extra := range []struct {
		version uint16
		n       int64
	}{{1, 8}, {2, 10}, {5, 4}} {
		if version < extra.version {
			break
		}
		if _, err := f.frameAt(pos, uint64(extra.n)); err != nil {
			return
		}
		pos += extra.n
	}
	f.os2Version = version
}

// loadLoca is tt_face_load_loca.
func (f *Face) loadLoca() error {
	glyf, glyfPos, ok := f.table("glyf")
	if ok {
		f.glyfLen = int64(len(glyf))
		f.glyfOffset = glyfPos
	}
	loca, locaPos, ok := f.table("loca")
	if !ok {
		return errTableMissing
	}
	shift := uint(1)
	if f.indexToLocFormat != 0 {
		shift = 2
	}
	tableLen := int64(len(loca))
	if tableLen > 0x10000<<shift {
		tableLen = 0x10000 << shift
	}
	f.numLocations = tableLen >> shift

	if f.numLocations != f.numGlyphs+1 && f.numLocations < f.numGlyphs+1 {
		newLen := (f.numGlyphs + 1) << shift
		dist := int64(0x7FFFFFFF)
		found := false
		for _, e := range f.tables {
			diff := int64(e.offset) - locaPos
			if diff > 0 && diff < dist {
				dist = diff
				found = true
			}
		}
		if !found {
			dist = int64(len(f.data)) - locaPos
		}
		if newLen <= dist {
			f.numLocations = f.numGlyphs + 1
			tableLen = newLen
		} else {
			f.numGlyphs = 0
			if f.numLocations > 0 {
				f.numGlyphs = f.numLocations - 1
			}
		}
	}
	end := locaPos + tableLen
	if end > int64(len(f.data)) {
		return errInvalidTable
	}
	f.loca = f.data[locaPos:end]
	return nil
}

// location is tt_face_get_location: a glyph's offset in the glyf table
// and its length.
func (f *Face) location(gindex uint32) (int64, int64) {
	var pos1, pos2 int64
	if int64(gindex) < f.numLocations {
		if f.indexToLocFormat != 0 {
			p := int(gindex) * 4
			pos1 = int64(peek32(f.loca, p))
			pos2 = pos1
			if p+8 <= int(f.numLocations)*4 {
				pos2 = int64(peek32(f.loca, p+4))
			}
		} else {
			p := int(gindex) * 2
			pos1 = int64(peek16(f.loca, p))
			pos2 = pos1
			if p+4 <= int(f.numLocations)*2 {
				pos2 = int64(peek16(f.loca, p+2))
			}
			pos1 <<= 1
			pos2 <<= 1
		}
	}

	if pos1 > f.glyfLen {
		return 0, 0
	}
	if pos2 > f.glyfLen {
		if int64(gindex) == f.numLocations-2 {
			pos2 = f.glyfLen
		} else {
			return 0, 0
		}
	}
	if pos2 >= pos1 {
		return pos1, pos2 - pos1
	}
	return pos1, f.glyfLen - pos1
}

// Name table (tt_face_load_name and tt_face_get_name).

type nameRecord struct {
	platformID, encodingID, languageID, nameID uint16
	str                                        []byte
}

func (f *Face) loadNames() {
	name, _, ok := f.table("name")
	if !ok {
		return
	}
	r := &reader{b: name}
	format := r.u16()
	num := int(r.u16())
	storageOffset := int(r.u16())
	if r.err != nil {
		return
	}
	storageStart := 6 + 12*num
	storageLimit := len(name)
	if storageStart > storageLimit {
		return
	}
	var langTagLens []uint16
	if format == 1 {
		r.pos = storageStart
		numLangTags := int(r.u16())
		storageStart += 2 + 4*numLangTags
		for i := 0; i < numLangTags; i++ {
			length := r.u16()
			off := int(r.u16()) + storageOffset
			if r.err != nil {
				return
			}
			if off < storageStart || off+int(length) > storageLimit {
				length = 0
			}
			langTagLens = append(langTagLens, length)
		}
		r.pos = 6
	}

	var recs []nameRecord
	for i := 0; i < num; i++ {
		var rec nameRecord
		rec.platformID = r.u16()
		rec.encodingID = r.u16()
		rec.languageID = r.u16()
		rec.nameID = r.u16()
		length := int(r.u16())
		off := int(r.u16())
		if r.err != nil {
			break
		}
		if length == 0 {
			continue
		}
		off += storageOffset
		if off < storageStart || off+length > storageLimit {
			continue
		}
		if format == 1 && rec.languageID >= 0x8000 {
			if int(rec.languageID-0x8000) >= len(langTagLens) ||
				langTagLens[rec.languageID-0x8000] == 0 {
				continue
			}
		}
		rec.str = name[off : off+length]
		recs = append(recs, rec)
	}

	f.names = recs
}

// setNames picks the family and style names as sfnt_load_face does.
func (f *Face) setNames() {
	const (
		fontFamily           = 1
		fontSubfamily        = 2
		typographicFamily    = 16
		typographicSubfamily = 17
		wwsFamily            = 21
		wwsSubfamily         = 22
	)
	var family, style string
	var ok bool
	if f.os2Version != 0xFFFF && f.fsSelection&256 != 0 {
		if family, ok = getName(f.names, typographicFamily); !ok {
			family, _ = getName(f.names, fontFamily)
		}
		if style, ok = getName(f.names, typographicSubfamily); !ok {
			style, _ = getName(f.names, fontSubfamily)
		}
	} else {
		if family, ok = getName(f.names, wwsFamily); !ok {
			if family, ok = getName(f.names, typographicFamily); !ok {
				family, _ = getName(f.names, fontFamily)
			}
		}
		if style, ok = getName(f.names, wwsSubfamily); !ok {
			if style, ok = getName(f.names, typographicSubfamily); !ok {
				style, _ = getName(f.names, fontSubfamily)
			}
		}
	}
	f.FamilyName, f.StyleName = family, style
}

// getName is tt_face_get_name.
func getName(recs []nameRecord, nameID uint16) (string, bool) {
	const (
		platformMacintosh = 1
		macLangEnglish    = 0
		macIDRoman        = 0
	)
	foundAppleRoman, foundAppleEnglish, foundWin, foundUnicode := -1, -1, -1, -1
	isEnglish := false
	for n, rec := range recs {
		if rec.nameID != nameID {
			continue
		}
		switch rec.platformID {
		case platformAppleUnicode, platformISO:
			foundUnicode = n
		case platformMacintosh:
			if rec.languageID == macLangEnglish {
				foundAppleEnglish = n
			} else if rec.encodingID == macIDRoman {
				foundAppleRoman = n
			}
		case platformMicrosoft:
			if foundWin == -1 || rec.languageID&0x3FF == 0x009 {
				switch rec.encodingID {
				case msIDSymbol, msIDUnicode, msIDUCS4:
					isEnglish = rec.languageID&0x3FF == 0x009
					foundWin = n
				}
			}
		}
	}
	foundApple := foundAppleRoman
	if foundAppleEnglish >= 0 {
		foundApple = foundAppleEnglish
	}

	if foundWin >= 0 && !(foundApple >= 0 && !isEnglish) {
		return asciiFromUTF16(recs[foundWin].str), true
	} else if foundApple >= 0 {
		return asciiFromOther(recs[foundApple].str), true
	} else if foundUnicode >= 0 {
		return asciiFromUTF16(recs[foundUnicode].str), true
	}
	return "", false
}

func asciiFromUTF16(b []byte) string {
	var s []byte
	for n := 0; n < len(b)/2; n++ {
		code := binary.BigEndian.Uint16(b[2*n:])
		if code == 0 {
			break
		}
		if code < 32 || code > 127 {
			code = '?'
		}
		s = append(s, byte(code))
	}
	return string(s)
}

func asciiFromOther(b []byte) string {
	var s []byte
	for _, code := range b {
		if code == 0 {
			break
		}
		if code < 32 || code > 127 {
			code = '?'
		}
		s = append(s, code)
	}
	return string(s)
}

// Kerning (tt_face_load_kern and tt_face_get_kerning).

func (f *Face) loadKern() {
	kern, _, ok := f.table("kern")
	if !ok || len(kern) < 4 {
		return
	}
	f.kernTable = kern
	p := 2
	numTables := int(peek16(kern, p))
	p += 2
	if numTables > 32 {
		numTables = 32
	}
	var avail, ordered uint32
	nn := 0
	for ; nn < numTables; nn++ {
		mask := uint32(1) << nn
		if p+6 > len(kern) {
			break
		}
		pNext := p
		p += 2
		length := int(peek16(kern, p))
		p += 2
		coverage := peek16(kern, p)
		p += 2
		if length <= 6+8 {
			break
		}
		pNext += length
		if pNext > len(kern) {
			pNext = len(kern)
		}
		format := coverage >> 8
		if format != 0 || coverage&3 != 1 || p+8 > pNext {
			p = pNext
			continue
		}
		numPairs := int(peek16(kern, p))
		p += 8
		if pNext-p < 6*numPairs {
			numPairs = (pNext - p) / 6
		}
		avail |= mask

		if numPairs > 0 {
			oldPair := peek32(kern, p)
			p += 6
			count := numPairs - 1
			for ; count > 0; count-- {
				curPair := peek32(kern, p)
				if curPair < oldPair {
					break
				}
				p += 6
				oldPair = curPair
			}
			if count == 0 {
				ordered |= mask
			}
		}
		p = pNext
	}
	f.numKernTables = nn
	f.kernAvailBits = avail
	f.kernOrderBits = ordered
}

func (f *Face) kerning(left, right uint32) int64 {
	var result int64
	kern := f.kernTable
	if kern == nil {
		return 0
	}
	p := 4
	mask := uint32(1)
	for count := f.numKernTables; count > 0 && p+6 <= len(kern); count, mask = count-1, mask<<1 {
		base := p
		p += 2
		length := int(peek16(kern, p))
		p += 2
		coverage := peek16(kern, p)
		p += 2
		next := base + length
		if next > len(kern) {
			next = len(kern)
		}
		if f.kernAvailBits&mask == 0 {
			p = next
			continue
		}
		numPairs := int(peek16(kern, p))
		p += 8
		if next-p < 6*numPairs {
			numPairs = (next - p) / 6
		}

		found := false
		var value int64
		if coverage>>8 == 0 {
			key0 := left<<16 | right
			if f.kernOrderBits&mask != 0 {
				lo, hi := 0, numPairs
				for lo < hi {
					mid := (lo + hi) >> 1
					q := p + 6*mid
					key := peek32(kern, q)
					if key == key0 {
						value = int64(int16(peek16(kern, q+4)))
						found = true
						break
					}
					if key < key0 {
						lo = mid + 1
					} else {
						hi = mid
					}
				}
			} else {
				for c := numPairs; c > 0; c-- {
					key := peek32(kern, p)
					p += 4
					if key == key0 {
						value = int64(int16(peek16(kern, p)))
						found = true
						break
					}
					p += 2
				}
			}
		}
		if found {
			if coverage&8 != 0 {
				result = value
			} else {
				result += value
			}
		}
		p = next
	}
	return result
}

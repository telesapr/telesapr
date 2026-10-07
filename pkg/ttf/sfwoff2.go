package ttf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sort"

	"github.com/andybalholm/brotli"
)

// WOFF2 files (sfwoff2.c's woff2_open_font): the tables are uncompressed
// with Brotli, the transformed glyf, loca and hmtx tables are rebuilt,
// and the SFNT (of a collection's first font) is read from memory.

const (
	woff2FlagsTransform  = 1 << 8
	woff2DefaultMaxSize  = 30 * 1024 * 1024
	woff2DefaultGlyphBuf = 5120
	woff2MaxSfntSize     = 1 << 26

	flagArg1And2AreWords    = 1 << 0
	flagWeHaveAScale        = 1 << 3
	flagMoreComponents      = 1 << 5
	flagWeHaveAnXAndYScale  = 1 << 6
	flagWeHaveATwoByTwo     = 1 << 7
	flagWeHaveInstructions  = 1 << 8
	glyfOnCurve             = 1 << 0
	glyfXShort              = 1 << 1
	glyfYShort              = 1 << 2
	glyfRepeat              = 1 << 3
	glyfThisXIsSame         = 1 << 4
	glyfThisYIsSame         = 1 << 5
	glyfOverlapSimple       = 1 << 6
	contourOffsetEndPoint   = 10
	haveOverlapSimpleBitmap = 0x1

	nContourStream    = 0
	nPointsStream     = 1
	flagStream        = 2
	glyphStream       = 3
	compositeStream   = 4
	bboxStream        = 5
	instructionStream = 6
)

// woff2KnownTags is woff2tags.c's known_tags.
var woff2KnownTags = [63]string{
	"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post", "cvt ",
	"fpgm", "glyf", "loca", "prep", "CFF ", "VORG", "EBDT", "EBLC", "gasp",
	"hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea", "vmtx", "BASE", "GDEF",
	"GPOS", "GSUB", "EBSC", "JSTF", "MATH", "CBDT", "CBLC", "COLR", "CPAL",
	"SVG ", "sbix", "acnt", "avar", "bdat", "bloc", "bsln", "cvar", "fdsc",
	"feat", "fmtx", "fvar", "gvar", "hsty", "just", "lcar", "mort", "morx",
	"opbd", "prop", "trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
}

type woff2Table struct {
	tag                  uint32
	flagByte             byte
	flags                uint32
	srcOffset, srcLength uint64
	transformLength      uint64
	dstOffset, dstLength uint64
}

type woff2Info struct {
	numGlyphs      uint16
	numHMetrics    uint16
	xMins          []int16
	glyfTable      *woff2Table
	locaTable      *woff2Table
	headTable      *woff2Table
	headerChecksum uint64
}

// read255UShort is Read255UShort.
func read255UShort(s *cffStream) (uint16, error) {
	code, err := s.readByte()
	if err != nil {
		return 0, err
	}
	switch code {
	case 253:
		return s.readU16()
	case 255:
		b, err := s.readByte()
		return uint16(b) + 253, err
	case 254:
		b, err := s.readByte()
		return uint16(b) + 253*2, err
	}
	return uint16(code), nil
}

// readBase128 is ReadBase128.
func readBase128(s *cffStream) (uint64, error) {
	var result uint64
	for i := range 5 {
		code, err := s.readByte()
		if err != nil {
			return 0, err
		}
		if i == 0 && code == 0x80 {
			return 0, errInvalidTable
		}
		if result&0xfe000000 != 0 {
			return 0, errInvalidTable
		}
		result = result<<7 | uint64(code&0x7f)
		if code&0x80 == 0 {
			return result, nil
		}
	}
	return 0, errInvalidTable
}

// sfntBuf is the SFNT being written, with write_buf's growth.
type sfntBuf struct {
	b []byte
}

// write is write_buf: it writes src at *offset and advances it.
func (w *sfntBuf) write(offset *uint64, src []byte) error {
	size := uint64(len(src))
	if *offset+size > woff2DefaultMaxSize {
		return errInvalidTable
	}
	if *offset+size > uint64(len(w.b)) {
		nb := make([]byte, *offset+size)
		copy(nb, w.b)
		w.b = nb
	}
	copy(w.b[*offset:], src)
	*offset += size
	return nil
}

// pad4 is pad4.
func (w *sfntBuf) pad4(offset *uint64) error {
	if *offset+3 < *offset {
		return errInvalidTable
	}
	pad := (*offset+3)&^3 - *offset
	if pad > 0 {
		if err := w.write(offset, make([]byte, pad)); err != nil {
			return errInvalidTable
		}
	}
	return nil
}

// computeULongSum is compute_ULong_sum.
func computeULongSum(buf []byte) uint64 {
	var checksum uint64
	size := len(buf)
	aligned := size &^ 3
	i := 0
	for ; i < aligned; i += 4 {
		checksum += uint64(binary.BigEndian.Uint32(buf[i:]))
	}
	for shift := 24; i < size; i, shift = i+1, shift-8 {
		checksum += uint64(uint32(buf[i]) << shift)
	}
	return checksum
}

// woff2Decompress is woff2_decompress (BrotliDecoderDecompress, which
// ignores input after the stream's end).
func woff2Decompress(dstSize uint64, src []byte) ([]byte, error) {
	r := brotli.NewReader(bytes.NewReader(src))
	dst := make([]byte, 0, dstSize)
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if uint64(len(dst))+uint64(n) > dstSize {
			return nil, errInvalidTable
		}
		dst = append(dst, buf[:n]...)
		if errors.Is(err, io.EOF) || (err != nil && err.Error() == "brotli: excessive input") {
			break
		}
		if err != nil {
			return nil, errInvalidTable
		}
	}
	if uint64(len(dst)) != dstSize {
		return nil, errInvalidTable
	}
	return dst, nil
}

func findWOFF2Table(tables []*woff2Table, t uint32) *woff2Table {
	for _, tb := range tables {
		if tb.tag == t {
			return tb
		}
	}
	return nil
}

func withSign(flag byte, baseVal int32) int32 {
	if flag&1 != 0 {
		return baseVal
	}
	return -baseVal
}

func safeIntAddition(a, b int32) (int32, bool) {
	if (a > 0 && b > 0x7FFFFFFF-a) || (a < 0 && b < -0x80000000-a) {
		return 0, false
	}
	return a + b, true
}

type woff2Point struct {
	x, y    int32
	onCurve bool
}

// tripletDecode is triplet_decode.
func tripletDecode(flagsIn, in []byte, nPoints uint64, result []woff2Point) (uint64, error) {
	var x, y int32
	inSize := uint64(len(in))
	if nPoints > inSize {
		return 0, errInvalidTable
	}
	tripletIndex := uint64(0)
	for i := range nPoints {
		flag := flagsIn[i]
		onCurve := flag>>7 == 0
		flag &= 0x7f
		var dataBytes uint64
		switch {
		case flag < 84:
			dataBytes = 1
		case flag < 120:
			dataBytes = 2
		case flag < 124:
			dataBytes = 3
		default:
			dataBytes = 4
		}
		if tripletIndex+dataBytes > inSize {
			return 0, errInvalidTable
		}
		b := func(k uint64) int32 { return int32(in[tripletIndex+k]) }
		var dx, dy int32
		switch {
		case flag < 10:
			dx = 0
			dy = withSign(flag, int32(flag&14)<<7+b(0))
		case flag < 20:
			dx = withSign(flag, int32((flag-10)&14)<<7+b(0))
			dy = 0
		case flag < 84:
			b0 := int32(flag - 20)
			b1 := b(0)
			dx = withSign(flag, 1+(b0&0x30)+(b1>>4))
			dy = withSign(flag>>1, 1+((b0&0x0c)<<2)+(b1&0x0f))
		case flag < 120:
			b0 := int32(flag - 84)
			dx = withSign(flag, 1+((b0/12)<<8)+b(0))
			dy = withSign(flag>>1, 1+(((b0%12)>>2)<<8)+b(1))
		case flag < 124:
			b2 := b(1)
			dx = withSign(flag, (b(0)<<4)+(b2>>4))
			dy = withSign(flag>>1, ((b2&0x0f)<<8)+b(2))
		default:
			dx = withSign(flag, (b(0)<<8)+b(1))
			dy = withSign(flag>>1, (b(2)<<8)+b(3))
		}
		tripletIndex += dataBytes
		var ok bool
		if x, ok = safeIntAddition(x, dx); !ok {
			return 0, errInvalidTable
		}
		if y, ok = safeIntAddition(y, dy); !ok {
			return 0, errInvalidTable
		}
		result[i] = woff2Point{x, y, onCurve}
	}
	return tripletIndex, nil
}

func putShort(p []byte, v int32) { binary.BigEndian.PutUint16(p, uint16(v)) }

// storePoints is store_points; it returns the glyph's size.
func storePoints(points []woff2Point, nContours, instructionLen uint16, haveOverlap bool,
	dst []byte) (uint64, error) {
	dstSize := uint32(len(dst))
	flagOffset := 10 + 2*uint32(nContours) + 2 + uint32(instructionLen)
	lastFlag := byte(0xFF)
	repeatCount := byte(0)
	var lastX, lastY int32
	var xBytes, yBytes uint32

	for i, point := range points {
		var flag byte
		if point.onCurve {
			flag = glyfOnCurve
		}
		dx := point.x - lastX
		dy := point.y - lastY
		if i == 0 && haveOverlap {
			flag |= glyfOverlapSimple
		}
		if dx == 0 {
			flag |= glyfThisXIsSame
		} else if dx > -256 && dx < 256 {
			flag |= glyfXShort
			if dx > 0 {
				flag |= glyfThisXIsSame
			}
			xBytes++
		} else {
			xBytes += 2
		}
		if dy == 0 {
			flag |= glyfThisYIsSame
		} else if dy > -256 && dy < 256 {
			flag |= glyfYShort
			if dy > 0 {
				flag |= glyfThisYIsSame
			}
			yBytes++
		} else {
			yBytes += 2
		}

		if flag == lastFlag && repeatCount != 255 {
			dst[flagOffset-1] |= glyfRepeat
			repeatCount++
		} else {
			if repeatCount != 0 {
				if flagOffset >= dstSize {
					return 0, errInvalidTable
				}
				dst[flagOffset] = repeatCount
				flagOffset++
			}
			if flagOffset >= dstSize {
				return 0, errInvalidTable
			}
			dst[flagOffset] = flag
			flagOffset++
			repeatCount = 0
		}
		lastX = point.x
		lastY = point.y
		lastFlag = flag
	}
	if repeatCount != 0 {
		if flagOffset >= dstSize {
			return 0, errInvalidTable
		}
		dst[flagOffset] = repeatCount
		flagOffset++
	}

	xyBytes := xBytes + yBytes
	if xyBytes < xBytes || flagOffset+xyBytes < flagOffset || flagOffset+xyBytes > dstSize {
		return 0, errInvalidTable
	}
	xOffset := flagOffset
	yOffset := flagOffset + xBytes
	lastX, lastY = 0, 0
	for _, point := range points {
		dx := point.x - lastX
		dy := point.y - lastY
		if dx == 0 {
		} else if dx > -256 && dx < 256 {
			if dx < 0 {
				dst[xOffset] = byte(-dx)
			} else {
				dst[xOffset] = byte(dx)
			}
			xOffset++
		} else {
			putShort(dst[xOffset:], dx)
			xOffset += 2
		}
		lastX += dx
		if dy == 0 {
		} else if dy > -256 && dy < 256 {
			if dy < 0 {
				dst[yOffset] = byte(-dy)
			} else {
				dst[yOffset] = byte(dy)
			}
			yOffset++
		} else {
			putShort(dst[yOffset:], dy)
			yOffset += 2
		}
		lastY += dy
	}
	return uint64(yOffset), nil
}

// computeBBox is compute_bbox: it writes the bbox and returns x_min.
func computeBBox(points []woff2Point, dst []byte) uint16 {
	var xMin, yMin, xMax, yMax int32
	if len(points) > 0 {
		xMin, yMin, xMax, yMax = points[0].x, points[0].y, points[0].x, points[0].y
	}
	for _, p := range points[min(1, len(points)):] {
		xMin = min(p.x, xMin)
		yMin = min(p.y, yMin)
		xMax = max(p.x, xMax)
		yMax = max(p.y, yMax)
	}
	putShort(dst[2:], xMin)
	putShort(dst[4:], yMin)
	putShort(dst[6:], xMax)
	putShort(dst[8:], yMax)
	return uint16(xMin)
}

// compositeGlyphSize is compositeGlyph_size.
func compositeGlyphSize(s *cffStream, offset uint64) (uint64, bool, error) {
	if err := s.seek(int64(offset)); err != nil {
		return 0, false, err
	}
	weHaveInst := false
	flags := uint16(flagMoreComponents)
	for flags&flagMoreComponents != 0 {
		var err error
		if flags, err = s.readU16(); err != nil {
			return 0, false, err
		}
		weHaveInst = weHaveInst || flags&flagWeHaveInstructions != 0
		argSize := int64(2)
		if flags&flagArg1And2AreWords != 0 {
			argSize += 4
		} else {
			argSize += 2
		}
		if flags&flagWeHaveAScale != 0 {
			argSize += 2
		} else if flags&flagWeHaveAnXAndYScale != 0 {
			argSize += 4
		} else if flags&flagWeHaveATwoByTwo != 0 {
			argSize += 8
		}
		if err := s.skip(argSize); err != nil {
			return 0, false, err
		}
	}
	return uint64(s.pos) - offset, weHaveInst, nil
}

type woff2Substream struct {
	start, offset, size uint64
}

// reconstructGlyf is reconstruct_glyf, with store_loca.
func reconstructGlyf(s *cffStream, glyfChecksum, locaChecksum *uint64, w *sfntBuf,
	outOffset *uint64, info *woff2Info) error {
	const numSubstreams = 7
	pos := uint64(s.pos)
	var substreams [numSubstreams]woff2Substream

	if err := s.skip(2); err != nil {
		return errInvalidTable
	}
	optionFlags, err := s.readU16()
	if err != nil {
		return errInvalidTable
	}
	numGlyphs, err := s.readU16()
	if err != nil {
		return errInvalidTable
	}
	indexFormat, err := s.readU16()
	if err != nil {
		return errInvalidTable
	}
	info.numGlyphs = numGlyphs

	expectedLocaLength := uint64(2)
	if indexFormat != 0 {
		expectedLocaLength = 4
	}
	expectedLocaLength *= uint64(numGlyphs) + 1
	if info.locaTable.dstLength != expectedLocaLength {
		return errInvalidTable
	}

	offset := uint64(2 + 2 + 2 + 2 + numSubstreams*4)
	if offset > info.glyfTable.transformLength {
		return errInvalidTable
	}
	for i := range substreams {
		size, err := s.readU32()
		if err != nil {
			return errInvalidTable
		}
		if uint64(size) > info.glyfTable.transformLength-offset {
			return errInvalidTable
		}
		substreams[i] = woff2Substream{pos + offset, pos + offset, uint64(size)}
		offset += uint64(size)
	}

	var overlapBitmapOffset uint64
	if optionFlags&haveOverlapSimpleBitmap != 0 {
		overlapBitmapLength := (uint64(numGlyphs) + 7) >> 3
		if overlapBitmapLength > info.glyfTable.transformLength-offset {
			return errInvalidTable
		}
		overlapBitmapOffset = pos + offset
	}

	locaValues := make([]uint64, int(numGlyphs)+1)
	pointsSize := uint64(0)
	bboxBitmapOffset := substreams[bboxStream].offset
	bboxBitmapLength := ((uint64(numGlyphs) + 31) >> 5) << 2
	substreams[bboxStream].offset += bboxBitmapLength

	glyphBuf := make([]byte, woff2DefaultGlyphBuf)
	info.xMins = make([]int16, numGlyphs)
	glyfStart := *outOffset
	destOffset := *outOffset
	base := s.data

	for i := uint64(0); i < uint64(numGlyphs); i++ {
		glyphSize := uint64(0)
		var xMin uint16

		if err := s.seek(int64(bboxBitmapOffset + i>>3)); err != nil {
			return errInvalidTable
		}
		bboxBitmap, err := s.readByte()
		if err != nil {
			return errInvalidTable
		}
		haveBBox := bboxBitmap&(0x80>>(i&7)) != 0

		if err := s.seek(int64(substreams[nContourStream].offset)); err != nil {
			return errInvalidTable
		}
		nContours, err := s.readU16()
		if err != nil {
			return errInvalidTable
		}
		substreams[nContourStream].offset += 2

		if nContours == 0xffff {
			// a composite glyph
			if !haveBBox {
				return errInvalidTable
			}
			compositeSize, haveInstructions, err := compositeGlyphSize(s, substreams[compositeStream].offset)
			if err != nil {
				return errInvalidTable
			}
			var instructionSize uint16
			if haveInstructions {
				if err := s.seek(int64(substreams[glyphStream].offset)); err != nil {
					return errInvalidTable
				}
				if instructionSize, err = read255UShort(s); err != nil {
					return errInvalidTable
				}
				substreams[glyphStream].offset = uint64(s.pos)
			}
			sizeNeeded := 12 + compositeSize + uint64(instructionSize)
			if uint64(len(glyphBuf)) < sizeNeeded {
				nb := make([]byte, sizeNeeded)
				copy(nb, glyphBuf)
				glyphBuf = nb
			}
			binary.BigEndian.PutUint16(glyphBuf[glyphSize:], nContours)
			glyphSize += 2

			if err := s.seek(int64(substreams[bboxStream].offset)); err != nil {
				return errInvalidTable
			}
			if xMin, err = s.readU16(); err != nil {
				return errInvalidTable
			}
			if err := s.seek(int64(substreams[bboxStream].offset)); err != nil {
				return errInvalidTable
			}
			b, err := s.read(8)
			if err != nil {
				return errInvalidTable
			}
			copy(glyphBuf[glyphSize:], b)
			substreams[bboxStream].offset += 8
			glyphSize += 8

			if err := s.seek(int64(substreams[compositeStream].offset)); err != nil {
				return errInvalidTable
			}
			if b, err = s.read(int64(compositeSize)); err != nil {
				return errInvalidTable
			}
			copy(glyphBuf[glyphSize:], b)
			substreams[compositeStream].offset += compositeSize
			glyphSize += compositeSize

			if haveInstructions {
				binary.BigEndian.PutUint16(glyphBuf[glyphSize:], instructionSize)
				glyphSize += 2
				if err := s.seek(int64(substreams[instructionStream].offset)); err != nil {
					return errInvalidTable
				}
				if b, err = s.read(int64(instructionSize)); err != nil {
					return errInvalidTable
				}
				copy(glyphBuf[glyphSize:], b)
				substreams[instructionStream].offset += uint64(instructionSize)
				glyphSize += uint64(instructionSize)
			}
		} else if nContours > 0 {
			// a simple glyph
			haveOverlap := false
			if overlapBitmapOffset != 0 {
				if err := s.seek(int64(overlapBitmapOffset + i>>3)); err != nil {
					return errInvalidTable
				}
				overlapBitmap, err := s.readByte()
				if err != nil {
					return errInvalidTable
				}
				haveOverlap = overlapBitmap&(0x80>>(i&7)) != 0
			}

			nPointsArr := make([]uint16, nContours)
			if err := s.seek(int64(substreams[nPointsStream].offset)); err != nil {
				return errInvalidTable
			}
			totalNPoints := uint64(0)
			for j := range nPointsArr {
				n, err := read255UShort(s)
				if err != nil {
					return errInvalidTable
				}
				nPointsArr[j] = n
				totalNPoints += uint64(n)
			}
			substreams[nPointsStream].offset = uint64(s.pos)

			pointsSize += totalNPoints
			if pointsSize > substreams[flagStream].size {
				return errInvalidTable
			}
			if substreams[glyphStream].size < substreams[glyphStream].offset-substreams[glyphStream].start {
				return errInvalidTable
			}
			tripletSize := substreams[glyphStream].size -
				(substreams[glyphStream].offset - substreams[glyphStream].start)
			flagsBuf := base[substreams[flagStream].offset:]
			tripletBuf := base[substreams[glyphStream].offset : substreams[glyphStream].offset+tripletSize]

			points := make([]woff2Point, totalNPoints)
			used, err := tripletDecode(flagsBuf, tripletBuf, totalNPoints, points)
			if err != nil {
				return errInvalidTable
			}
			substreams[flagStream].offset += totalNPoints
			substreams[glyphStream].offset += used

			if err := s.seek(int64(substreams[glyphStream].offset)); err != nil {
				return errInvalidTable
			}
			instructionSize, err := read255UShort(s)
			if err != nil {
				return errInvalidTable
			}
			substreams[glyphStream].offset = uint64(s.pos)

			if totalNPoints >= 1<<27 {
				return errInvalidTable
			}
			sizeNeeded := 12 + 2*uint64(nContours) + 5*totalNPoints + uint64(instructionSize)
			if uint64(len(glyphBuf)) < sizeNeeded {
				nb := make([]byte, sizeNeeded)
				copy(nb, glyphBuf)
				glyphBuf = nb
			}
			binary.BigEndian.PutUint16(glyphBuf[glyphSize:], nContours)
			glyphSize += 2

			if haveBBox {
				if err := s.seek(int64(substreams[bboxStream].offset)); err != nil {
					return errInvalidTable
				}
				if xMin, err = s.readU16(); err != nil {
					return errInvalidTable
				}
				if err := s.seek(int64(substreams[bboxStream].offset)); err != nil {
					return errInvalidTable
				}
				b, err := s.read(8)
				if err != nil {
					return errInvalidTable
				}
				copy(glyphBuf[glyphSize:], b)
				substreams[bboxStream].offset += 8
			} else {
				xMin = computeBBox(points, glyphBuf)
			}

			glyphSize = contourOffsetEndPoint
			endPoint := int32(-1)
			for _, n := range nPointsArr {
				endPoint += int32(n)
				if endPoint >= 65536 {
					return errInvalidTable
				}
				putShort(glyphBuf[glyphSize:], endPoint)
				glyphSize += 2
			}
			binary.BigEndian.PutUint16(glyphBuf[glyphSize:], instructionSize)
			glyphSize += 2

			if err := s.seek(int64(substreams[instructionStream].offset)); err != nil {
				return errInvalidTable
			}
			b, err := s.read(int64(instructionSize))
			if err != nil {
				return errInvalidTable
			}
			copy(glyphBuf[glyphSize:], b)
			substreams[instructionStream].offset += uint64(instructionSize)

			if glyphSize, err = storePoints(points, nContours, instructionSize, haveOverlap,
				glyphBuf); err != nil {
				return errInvalidTable
			}
		} else if haveBBox {
			// an empty glyph must not have a bbox
			return errInvalidTable
		}

		locaValues[i] = destOffset - glyfStart
		if err := w.write(&destOffset, glyphBuf[:glyphSize]); err != nil {
			return errInvalidTable
		}
		if err := w.pad4(&destOffset); err != nil {
			return errInvalidTable
		}
		*glyfChecksum += computeULongSum(glyphBuf[:glyphSize])
		info.xMins[i] = int16(xMin)
	}

	info.glyfTable.dstLength = destOffset - info.glyfTable.dstOffset
	info.locaTable.dstOffset = destOffset
	locaValues[numGlyphs] = info.glyfTable.dstLength

	// store_loca
	offsetSize := 2
	if indexFormat != 0 {
		offsetSize = 4
	}
	locaBuf := make([]byte, len(locaValues)*offsetSize)
	for i, v := range locaValues {
		if indexFormat != 0 {
			binary.BigEndian.PutUint32(locaBuf[4*i:], uint32(v))
		} else {
			binary.BigEndian.PutUint16(locaBuf[2*i:], uint16(v>>1))
		}
	}
	*locaChecksum = computeULongSum(locaBuf)
	if err := w.write(&destOffset, locaBuf); err != nil {
		return errInvalidTable
	}
	info.locaTable.dstLength = destOffset - info.locaTable.dstOffset
	*outOffset = destOffset
	return nil
}

// getXMins is get_x_mins.
func getXMins(s *cffStream, tables []*woff2Table, info *woff2Info) error {
	maxpTable := findWOFF2Table(tables, tag("maxp"))
	headTable := findWOFF2Table(tables, tag("head"))
	if maxpTable == nil || headTable == nil || info.locaTable == nil {
		return errInvalidTable
	}
	if err := s.seek(int64(maxpTable.srcOffset)); err != nil {
		return err
	}
	if err := s.skip(8); err != nil {
		return err
	}
	numGlyphs, err := s.readU16()
	if err != nil {
		return err
	}
	info.numGlyphs = numGlyphs
	if err := s.seek(int64(headTable.srcOffset)); err != nil {
		return err
	}
	if err := s.skip(50); err != nil {
		return err
	}
	indexFormat, err := s.readU16()
	if err != nil {
		return err
	}
	offsetSize := uint64(2)
	if indexFormat != 0 {
		offsetSize = 4
	}
	info.xMins = make([]int16, numGlyphs)
	locaOffset := info.locaTable.srcOffset
	for i := range info.xMins {
		if err := s.seek(int64(locaOffset)); err != nil {
			return err
		}
		locaOffset += offsetSize
		var glyfOffset uint64
		if indexFormat != 0 {
			v, err := s.readU32()
			if err != nil {
				return err
			}
			glyfOffset = uint64(v)
		} else {
			v, err := s.readU16()
			if err != nil {
				return err
			}
			glyfOffset = uint64(v) << 1
		}
		glyfOffset += info.glyfTable.srcOffset
		if err := s.seek(int64(glyfOffset)); err != nil {
			return err
		}
		if err := s.skip(2); err != nil {
			return err
		}
		v, err := s.readU16()
		if err != nil {
			return err
		}
		info.xMins[i] = int16(v)
	}
	return nil
}

// reconstructHmtx is reconstruct_hmtx.
func reconstructHmtx(s *cffStream, numGlyphs, numHMetrics uint16, xMins []int16,
	checksum *uint64, w *sfntBuf, outOffset *uint64) error {
	hmtxFlags, err := s.readByte()
	if err != nil {
		return errInvalidTable
	}
	hasProportionalLSBs := hmtxFlags&1 == 0
	hasMonospaceLSBs := hmtxFlags&2 == 0
	if hmtxFlags&0xFC != 0 || (hasProportionalLSBs && hasMonospaceLSBs) ||
		numHMetrics > numGlyphs || numHMetrics < 1 {
		return errInvalidTable
	}
	advanceWidths := make([]uint16, numHMetrics)
	lsbs := make([]int16, numGlyphs)
	for i := range advanceWidths {
		if advanceWidths[i], err = s.readU16(); err != nil {
			return errInvalidTable
		}
	}
	for i := 0; i < int(numHMetrics); i++ {
		if hasProportionalLSBs {
			v, err := s.readU16()
			if err != nil {
				return errInvalidTable
			}
			lsbs[i] = int16(v)
		} else {
			lsbs[i] = xMins[i]
		}
	}
	for i := int(numHMetrics); i < int(numGlyphs); i++ {
		if hasMonospaceLSBs {
			v, err := s.readU16()
			if err != nil {
				return errInvalidTable
			}
			lsbs[i] = int16(v)
		} else {
			lsbs[i] = xMins[i]
		}
	}
	table := make([]byte, 0, 2*int(numHMetrics)+2*int(numGlyphs))
	for i := 0; i < int(numGlyphs); i++ {
		if i < int(numHMetrics) {
			table = binary.BigEndian.AppendUint16(table, advanceWidths[i])
		}
		table = binary.BigEndian.AppendUint16(table, uint16(lsbs[i]))
	}
	*checksum = computeULongSum(table)
	if err := w.write(outOffset, table); err != nil {
		return errInvalidTable
	}
	return nil
}

// reconstructFont is reconstruct_font; it returns the SFNT's size.
func reconstructFont(transformed []byte, indices []*woff2Table, numTables uint16,
	info *woff2Info, w *sfntBuf) (uint64, error) {
	destOffset := 12 + uint64(numTables)*16
	var locaChecksum uint64
	var numHMetrics uint16
	fontChecksum := info.headerChecksum
	isGlyfXform := false
	tableEntryOffset := uint64(12)

	info.glyfTable = findWOFF2Table(indices, tag("glyf"))
	info.locaTable = findWOFF2Table(indices, tag("loca"))
	if (info.glyfTable == nil) != (info.locaTable == nil) {
		return 0, errInvalidTable
	}
	if info.glyfTable != nil &&
		info.glyfTable.flags&woff2FlagsTransform != info.locaTable.flags&woff2FlagsTransform {
		return 0, errInvalidTable
	}

	s := &cffStream{data: transformed}
	for _, tp := range indices {
		table := *tp
		if err := s.seek(int64(table.srcOffset)); err != nil {
			return 0, errInvalidTable
		}
		if table.srcOffset+table.srcLength > uint64(len(transformed)) {
			return 0, errInvalidTable
		}
		if table.tag == tag("hhea") {
			// read_num_hmetrics
			if err := s.skip(34); err != nil {
				return 0, errInvalidTable
			}
			n, err := s.readU16()
			if err != nil {
				return 0, errInvalidTable
			}
			numHMetrics = n
		}
		info.numHMetrics = numHMetrics

		var checksum uint64
		if table.flags&woff2FlagsTransform != woff2FlagsTransform {
			if table.tag == tag("head") {
				if table.srcLength < 12 {
					return 0, errInvalidTable
				}
				binary.BigEndian.PutUint32(transformed[table.srcOffset+8:], 0)
			}
			table.dstOffset = destOffset
			src := transformed[table.srcOffset : table.srcOffset+table.srcLength]
			checksum = computeULongSum(src)
			if err := w.write(&destOffset, src); err != nil {
				return 0, errInvalidTable
			}
		} else {
			switch table.tag {
			case tag("glyf"):
				// As in C, this sets the copy's offset, so reconstruct_glyf
				// sees the directory record's (zero): its glyf length and
				// last loca value are glyf's end offset, while the table
				// entry keeps the directory's length.
				isGlyfXform = true
				table.dstOffset = destOffset
				if err := reconstructGlyf(s, &checksum, &locaChecksum, w, &destOffset, info); err != nil {
					return 0, errInvalidTable
				}
			case tag("loca"):
				checksum = locaChecksum
			case tag("hmtx"):
				if !isGlyfXform {
					if err := getXMins(s, indices, info); err != nil {
						return 0, errInvalidTable
					}
				}
				table.dstOffset = destOffset
				if err := reconstructHmtx(s, info.numGlyphs, info.numHMetrics, info.xMins,
					&checksum, w, &destOffset); err != nil {
					return 0, errInvalidTable
				}
			default:
				return 0, errInvalidTable
			}
		}

		fontChecksum += checksum
		var entry [16]byte
		binary.BigEndian.PutUint32(entry[0:], table.tag)
		binary.BigEndian.PutUint32(entry[4:], uint32(checksum))
		binary.BigEndian.PutUint32(entry[8:], uint32(table.dstOffset))
		binary.BigEndian.PutUint32(entry[12:], uint32(table.dstLength))
		if err := w.write(&tableEntryOffset, entry[:]); err != nil {
			return 0, errInvalidTable
		}
		fontChecksum += computeULongSum(entry[:])
		if err := w.pad4(&destOffset); err != nil {
			return 0, errInvalidTable
		}
		if table.dstOffset+table.dstLength > destOffset {
			return 0, errInvalidTable
		}
	}

	// The directory record's dst_offset is zero, so C writes the
	// checksum adjustment over the SFNT header's entrySelector and
	// rangeShift.
	info.headTable = findWOFF2Table(indices, tag("head"))
	if info.headTable == nil || info.headTable.dstLength < 12 {
		return 0, errInvalidTable
	}
	fontChecksum = 0xB1B0AFBA - fontChecksum
	binary.BigEndian.PutUint32(w.b[info.headTable.dstOffset+8:], uint32(fontChecksum))
	return destOffset, nil
}

// woff2OpenFont is woff2_open_font, for the collection's first face.
func woff2OpenFont(data []byte) ([]byte, error) {
	s := &cffStream{data: data}
	hdr, err := s.frame(48)
	if err != nil {
		return nil, err
	}
	be := binary.BigEndian
	flavor := be.Uint32(hdr[4:])
	length := uint64(be.Uint32(hdr[8:]))
	numTables := be.Uint16(hdr[12:])
	totalSfntSize := uint64(be.Uint32(hdr[16:]))
	totalCompressedSize := uint64(be.Uint32(hdr[20:]))
	metaOffset := uint64(be.Uint32(hdr[28:]))
	metaLength := uint64(be.Uint32(hdr[32:]))
	metaOrigLength := uint64(be.Uint32(hdr[36:]))
	privOffset := uint64(be.Uint32(hdr[40:]))
	privLength := uint64(be.Uint32(hdr[44:]))

	if flavor == tag("wOF2") {
		return nil, errInvalidTable
	}
	if length != uint64(len(data)) || numTables == 0 || numTables > 0xFFF ||
		48+uint64(numTables)*20 >= length ||
		(metaOffset == 0 && (metaLength != 0 || metaOrigLength != 0)) ||
		(metaLength != 0 && metaOrigLength == 0) ||
		metaOffset >= length || length-metaOffset < metaLength ||
		(privOffset == 0 && privLength != 0) ||
		privOffset >= length || length-privOffset < privLength {
		return nil, errInvalidTable
	}

	tables := make([]woff2Table, numTables)
	indices := make([]*woff2Table, numTables)
	srcOffset := uint64(0)
	for nn := range tables {
		t := &tables[nn]
		if t.flagByte, err = s.readByte(); err != nil {
			return nil, err
		}
		if t.flagByte&0x3f == 0x3f {
			if t.tag, err = s.readU32(); err != nil {
				return nil, err
			}
		} else {
			if t.flagByte&0x3f > 62 {
				return nil, errInvalidTable
			}
			t.tag = tag(woff2KnownTags[t.flagByte&0x3f])
		}
		var flags uint32
		xformVersion := uint32(t.flagByte>>6) & 3
		if t.tag == tag("glyf") || t.tag == tag("loca") {
			if xformVersion == 0 {
				flags |= woff2FlagsTransform
			}
		} else if xformVersion != 0 {
			flags |= woff2FlagsTransform
		}
		flags |= xformVersion
		if t.dstLength, err = readBase128(s); err != nil {
			return nil, err
		}
		t.transformLength = t.dstLength
		if flags&woff2FlagsTransform != 0 {
			if t.transformLength, err = readBase128(s); err != nil {
				return nil, err
			}
			if t.tag == tag("loca") && t.transformLength != 0 {
				return nil, errInvalidTable
			}
		}
		if srcOffset+t.transformLength < srcOffset {
			return nil, errInvalidTable
		}
		t.flags = flags
		t.srcOffset = srcOffset
		t.srcLength = t.transformLength
		srcOffset += t.transformLength
		t.dstOffset = 0
		indices[nn] = t
	}
	last := indices[numTables-1]
	uncompressedSize := last.srcOffset + last.srcLength
	if uncompressedSize < last.srcOffset {
		return nil, errInvalidTable
	}

	var numFonts uint16
	var headerVersion uint32
	type ttcFont struct {
		flavor  uint32
		indices []uint16
	}
	var ttcFonts []ttcFont
	if flavor == tag("ttcf") {
		if headerVersion, err = s.readU32(); err != nil {
			return nil, err
		}
		if headerVersion != 0x00010000 && headerVersion != 0x00020000 {
			return nil, errInvalidTable
		}
		if numFonts, err = read255UShort(s); err != nil {
			return nil, err
		}
		if numFonts == 0 {
			return nil, errInvalidTable
		}
		ttcFonts = make([]ttcFont, numFonts)
		for nn := range ttcFonts {
			f := &ttcFonts[nn]
			n, err := read255UShort(s)
			if err != nil {
				return nil, err
			}
			if f.flavor, err = s.readU32(); err != nil {
				return nil, err
			}
			f.indices = make([]uint16, n)
			var glyfIndex, locaIndex uint16
			for j := range f.indices {
				ti, err := read255UShort(s)
				if err != nil {
					return nil, err
				}
				if ti >= numTables {
					return nil, errInvalidTable
				}
				f.indices[j] = ti
				switch indices[ti].tag {
				case tag("loca"):
					locaIndex = ti
				case tag("glyf"):
					glyfIndex = ti
				}
			}
			if glyfIndex > 0 || locaIndex > 0 {
				if glyfIndex > locaIndex || locaIndex-glyfIndex != 1 {
					return nil, errInvalidTable
				}
			}
		}
	}

	compressedOffset := uint64(s.pos)
	fileOffset := (compressedOffset + totalCompressedSize + 3) &^ 3
	if fileOffset > length {
		return nil, errInvalidTable
	}
	if metaOffset != 0 {
		if fileOffset != metaOffset {
			return nil, errInvalidTable
		}
		fileOffset = (metaOffset + metaLength + 3) &^ 3
	}
	if privOffset != 0 {
		if fileOffset != privOffset {
			return nil, errInvalidTable
		}
		fileOffset = (privOffset + privLength + 3) &^ 3
	}
	if fileOffset != (length+3)&^3 {
		return nil, errInvalidTable
	}

	// face index 0
	if headerVersion != 0 {
		f := &ttcFonts[0]
		if len(f.indices) == 0 || len(f.indices) > 0xFFF {
			return nil, errInvalidTable
		}
		sel := make([]*woff2Table, len(f.indices))
		for nn, ti := range f.indices {
			sel[nn] = indices[ti]
		}
		indices = sel
		flavor = f.flavor
		numTables = uint16(len(f.indices))
	}

	sfntSize := 12 + uint64(numTables)*16
	if totalSfntSize > sfntSize {
		if totalSfntSize>>6 > length {
			sfntSize = length << 6
		} else {
			sfntSize = totalSfntSize
		}
		if sfntSize >= woff2MaxSfntSize {
			sfntSize = woff2MaxSfntSize
		}
	}
	w := &sfntBuf{b: make([]byte, sfntSize)}
	entrySelector := msb32(uint32(numTables))
	searchRange := (1 << entrySelector) * 16
	rangeShift := int(numTables)*16 - searchRange
	be.PutUint32(w.b[0:], flavor)
	be.PutUint16(w.b[4:], numTables)
	be.PutUint16(w.b[6:], uint16(searchRange))
	be.PutUint16(w.b[8:], uint16(entrySelector))
	be.PutUint16(w.b[10:], uint16(rangeShift))
	info := &woff2Info{headerChecksum: computeULongSum(w.b[:12])}

	// glibc's qsort is a merge sort
	sort.SliceStable(indices, func(i, j int) bool { return indices[i].tag < indices[j].tag })
	for nn := 1; nn < int(numTables); nn++ {
		if indices[nn].tag == indices[nn-1].tag {
			return nil, errInvalidTable
		}
	}
	if uncompressedSize < 1 || uncompressedSize > woff2MaxSfntSize {
		return nil, errInvalidTable
	}
	compressed, err := s.frame(totalCompressedSize)
	if err != nil {
		return nil, err
	}
	transformed, err := woff2Decompress(uncompressedSize, compressed)
	if err != nil {
		return nil, err
	}
	actual, err := reconstructFont(transformed, indices, numTables, info, w)
	if err != nil {
		return nil, err
	}
	if actual < uint64(len(w.b)) {
		w.b = w.b[:actual]
	}
	return w.b, nil
}

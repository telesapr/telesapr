package ttf

import "math/bits"

// FreeType's PFR driver (pfr's pfrload.c, pfrobjs.c, pfrcmap.c,
// pfrgload.c and pfrdrivr.c): Bitstream's Portable Font Resource
// fonts. Embedded bitmaps aren't loaded (ttf.cpp asks for outlines
// only), but their strike list is read, since it decides whether a font
// without outlines is scalable.

// pfr flags.
const (
	pfrLogStroke      = 0x04
	pfrLog2ByteStroke = 0x08
	pfrLogBold        = 0x10
	pfrLog2ByteBold   = 0x20
	pfrLogExtraItems  = 0x40
	pfrLineJoinMask   = 0x03
	pfrLineJoinMiter  = 0x00

	pfrStrike2ByteXppm   = 0x01
	pfrStrike2ByteYppm   = 0x02
	pfrStrike3ByteSize   = 0x04
	pfrStrike3ByteOffset = 0x08
	pfrStrike2ByteCount  = 0x10

	pfrPhyVertical        = 0x01
	pfrPhy2ByteCharcode   = 0x02
	pfrPhyProportional    = 0x04
	pfrPhyASCIICode       = 0x08
	pfrPhy2ByteGpsSize    = 0x10
	pfrPhy3ByteGpsOffset  = 0x20
	pfrPhyExtraItems      = 0x80
	pfrKern2ByteChar      = 0x01
	pfrKern2ByteAdj       = 0x02
	pfrGlyphYCount        = 0x01
	pfrGlyphXCount        = 0x02
	pfrGlyph1ByteXYCount  = 0x04
	pfrGlyphSingleExtra   = 0x08
	pfrGlyphCompoundExtra = 0x40
	pfrGlyphIsCompound    = 0x80

	pfrSubglyphXScale      = 0x10
	pfrSubglyphYScale      = 0x20
	pfrSubglyph2ByteSize   = 0x40
	pfrSubglyph3ByteOffset = 0x80
)

// pfrCursor reads a frame of the file as FreeType's PFR_NEXT_* macros
// do; PFR_CHECK is check.
type pfrCursor struct {
	b        []byte
	p, limit int
}

func (c *pfrCursor) check(n uint64) bool { return uint64(c.p)+n <= uint64(c.limit) }
func (c *pfrCursor) byte() uint32        { v := c.b[c.p]; c.p++; return uint32(v) }
func (c *pfrCursor) int8() int32         { v := int8(c.b[c.p]); c.p++; return int32(v) }
func (c *pfrCursor) ushort() uint32 {
	v := uint32(c.b[c.p])<<8 | uint32(c.b[c.p+1])
	c.p += 2
	return v
}
func (c *pfrCursor) short() int32 { return int32(int16(c.ushort())) }

// uoff3 and off3 are PFR_NEXT_ULONG and PFR_NEXT_LONG: 3 bytes.
func (c *pfrCursor) uoff3() uint32 {
	v := uint32(c.b[c.p])<<16 | uint32(c.b[c.p+1])<<8 | uint32(c.b[c.p+2])
	c.p += 3
	return v
}
func (c *pfrCursor) off3() int32 { return int32(c.uoff3()<<8) >> 8 }

// pfrHeader is the part of a PFR_HeaderRec the driver reads.
type pfrHeader struct {
	signature          uint32
	version            uint32
	signature2         uint32
	headerSize         uint32
	logDirOffset       uint32
	gpsSectionOffset   uint32
	phyFontMaxSizeHigh uint32
}

type pfrLogFont struct {
	size, offset    uint32
	matrix          [4]int32
	strokeThickness int32
	boldThickness   int32
	miterLimit      int32
	physSize        uint32
	physOffset      uint32
}

type pfrChar struct {
	charCode  uint32
	advance   int32
	gpsSize   uint32
	gpsOffset uint32
}

type pfrKernItem struct {
	pairCount uint32
	flags     uint32
	baseAdj   int32
	pairSize  uint32
	offset    uint64
	pair1     uint32
	pair2     uint32
}

// pfrPhyFont is the part of a PFR_PhyFontRec the driver uses.
type pfrPhyFont struct {
	offset            uint32
	outlineResolution uint32
	metricsResolution uint32
	bbox              BBox
	flags             uint32
	standardAdvance   int32
	ascent, descent   int32
	leading           int32
	fontID            *string
	familyName        *string
	styleName         *string
	numStrikes        uint32
	stemSnapsLoaded   bool
	chars             []pfrChar
	numKernPairs      uint32
	kernItems         []*pfrKernItem
}

// pfrFace is the part of a PFR_FaceRec past its FT_FaceRec.
type pfrFace struct {
	header  pfrHeader
	logFont pfrLogFont
	phyFont pfrPhyFont
	glyph   pfrGlyph
}

var errPfrInvalidTable = errInvalidTable

// pfrExtraItemsParse is pfr_extra_items_parse; parse is called on each
// item of a known type (nil: the items are skipped).
func pfrExtraItemsParse(c *pfrCursor, parse func(itemType uint32, item *pfrCursor) error) error {
	if !c.check(1) {
		return errPfrInvalidTable
	}
	for numItems := c.byte(); numItems > 0; numItems-- {
		if !c.check(2) {
			return errPfrInvalidTable
		}
		itemSize := c.byte()
		itemType := c.byte()
		if !c.check(uint64(itemSize)) {
			return errPfrInvalidTable
		}
		if parse != nil {
			item := &pfrCursor{b: c.b, p: c.p, limit: c.p + int(itemSize)}
			if err := parse(itemType, item); err != nil {
				return err
			}
		}
		c.p += int(itemSize)
	}
	return nil
}

// pfrHeaderLoad is pfr_header_load and pfr_header_check: false for a
// file that isn't a PFR font.
func pfrHeaderLoad(data []byte) (pfrHeader, bool) {
	var h pfrHeader
	if len(data) < 58 {
		return h, false
	}
	u16 := func(p int) uint32 { return uint32(data[p])<<8 | uint32(data[p+1]) }
	h.signature = u16(0)<<16 | u16(2)
	h.version = u16(4)
	h.signature2 = u16(6)
	h.headerSize = u16(8)
	h.logDirOffset = u16(12)
	h.gpsSectionOffset = uint32(data[35])<<16 | u16(36)
	h.phyFontMaxSizeHigh = uint32(data[41])
	if h.signature != 0x50465230 || h.version > 4 || h.headerSize < 58 ||
		h.signature2 != 0x0D0A {
		return h, false
	}
	return h, true
}

// pfrLogFontCount is pfr_log_font_count.
func pfrLogFontCount(data []byte, sectionOffset uint32) (int64, error) {
	size := uint64(len(data))
	if uint64(sectionOffset)+2 > size {
		// the seek or the read fails
		return 0, errInvalidTable
	}
	count := uint64(data[sectionOffset])<<8 | uint64(data[sectionOffset+1])
	if count > ((1<<16)-2)/5 || 2+count*5 >= size-uint64(sectionOffset) ||
		95+count*(5+18) >= size {
		return 0, errPfrInvalidTable
	}
	return int64(count), nil
}

// pfrLogFontLoad is pfr_log_font_load.
func pfrLogFontLoad(lf *pfrLogFont, data []byte, idx uint32, sectionOffset uint32,
	sizeIncrement bool) error {
	size := uint64(len(data))
	s := &pfrCursor{b: data, p: int(sectionOffset), limit: len(data)}
	if uint64(sectionOffset) > size || !s.check(2) {
		return errInvalidTable
	}
	numLogFonts := s.ushort()
	if idx >= numLogFonts {
		return errInvalidArg
	}
	if !s.check(uint64(idx)*5 + 5) {
		return errInvalidTable
	}
	s.p += int(idx) * 5
	lf.size = s.ushort()
	lf.offset = s.uoff3()

	// FT_STREAM_SEEK and FT_FRAME_ENTER
	if uint64(lf.offset) > size || uint64(lf.size) > size-uint64(lf.offset) {
		return errInvalidTable
	}
	c := &pfrCursor{b: data, p: int(lf.offset), limit: int(lf.offset) + int(lf.size)}
	if !c.check(13) {
		return errPfrInvalidTable
	}
	for i := range lf.matrix {
		lf.matrix[i] = c.off3()
	}
	flags := c.byte()
	local := uint64(0)
	if flags&pfrLogStroke != 0 {
		local++
		if flags&pfrLog2ByteStroke != 0 {
			local++
		}
		if flags&pfrLineJoinMask == pfrLineJoinMiter {
			local += 3
		}
	}
	if flags&pfrLogBold != 0 {
		local++
		if flags&pfrLog2ByteBold != 0 {
			local++
		}
	}
	if !c.check(local) {
		return errPfrInvalidTable
	}
	if flags&pfrLogStroke != 0 {
		if flags&pfrLog2ByteStroke != 0 {
			lf.strokeThickness = c.short()
		} else {
			lf.strokeThickness = int32(c.byte())
		}
		if flags&pfrLineJoinMask == pfrLineJoinMiter {
			lf.miterLimit = c.off3()
		}
	}
	if flags&pfrLogBold != 0 {
		if flags&pfrLog2ByteBold != 0 {
			lf.boldThickness = c.short()
		} else {
			lf.boldThickness = int32(c.byte())
		}
	}
	if flags&pfrLogExtraItems != 0 {
		if err := pfrExtraItemsParse(c, nil); err != nil {
			return err
		}
	}
	if !c.check(5) {
		return errPfrInvalidTable
	}
	lf.physSize = c.ushort()
	lf.physOffset = c.uoff3()
	if sizeIncrement {
		if !c.check(1) {
			return errPfrInvalidTable
		}
		lf.physSize += c.byte() << 16
	}
	return nil
}

// pfrExtraItemLoadBitmapInfo is pfr_extra_item_load_bitmap_info, for
// the number of strikes.
func (pf *pfrPhyFont) loadBitmapInfo(c *pfrCursor) error {
	if !c.check(5) {
		return errPfrInvalidTable
	}
	c.p += 3 // bctSize
	flags0 := c.byte()
	count := c.byte()
	size1 := uint64(1 + 1 + 1 + 2 + 2 + 1)
	for _, f := range []uint32{pfrStrike2ByteXppm, pfrStrike2ByteYppm, pfrStrike3ByteSize,
		pfrStrike3ByteOffset, pfrStrike2ByteCount} {
		if flags0&f != 0 {
			size1++
		}
	}
	if !c.check(uint64(count) * size1) {
		return errPfrInvalidTable
	}
	c.p += int(uint64(count) * size1)
	pf.numStrikes += count
	return nil
}

// pfrAuxNameLoad is pfr_aux_name_load: a printable ASCII name, or nil.
func pfrAuxNameLoad(b []byte) *string {
	n := len(b)
	if n > 0 && b[n-1] == 0 {
		n--
	}
	if n == 0 {
		return nil
	}
	for _, c := range b[:n] {
		if c < 32 || c > 127 {
			return nil
		}
	}
	s := string(b[:n])
	return &s
}

// pfrPhyFontLoad is pfr_phy_font_load.
func pfrPhyFontLoad(pf *pfrPhyFont, data []byte, offset, size uint32) error {
	pf.offset = offset
	if uint64(offset) > uint64(len(data)) || uint64(size) > uint64(len(data))-uint64(offset) {
		return errInvalidTable
	}
	c := &pfrCursor{b: data, p: int(offset), limit: int(offset) + int(size)}
	start := c.p

	if !c.check(15) {
		return errPfrInvalidTable
	}
	c.ushort() // font_ref_number
	pf.outlineResolution = c.ushort()
	pf.metricsResolution = c.ushort()
	pf.bbox.XMin = int64(c.short())
	pf.bbox.YMin = int64(c.short())
	pf.bbox.XMax = int64(c.short())
	pf.bbox.YMax = int64(c.short())
	flags := c.byte()
	pf.flags = flags
	if pf.outlineResolution == 0 || pf.metricsResolution == 0 {
		return errPfrInvalidTable
	}

	// the standard advance of a non-proportional font
	if flags&pfrPhyProportional == 0 {
		if !c.check(2) {
			return errPfrInvalidTable
		}
		pf.standardAdvance = c.short()
	}

	if flags&pfrPhyExtraItems != 0 {
		err := pfrExtraItemsParse(c, func(itemType uint32, item *pfrCursor) error {
			switch itemType {
			case 1:
				return pf.loadBitmapInfo(item)
			case 2:
				// the font ID
				if pf.fontID == nil {
					s := cstr(item.b[item.p:item.limit])
					pf.fontID = &s
				}
			case 3:
				// the stem snaps
				if pf.stemSnapsLoaded {
					break
				}
				if !item.check(1) {
					return errPfrInvalidTable
				}
				count := item.byte()
				count = count&15 + count>>4
				if !item.check(uint64(count) * 2) {
					return errPfrInvalidTable
				}
				// FT_QNEW_ARRAY of no snaps is NULL
				pf.stemSnapsLoaded = count > 0
			case 4:
				return pf.loadKerningPairs(item, start)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// The auxiliary data has the family and style names, and the
	// ascent and descent (undocumented).
	if !c.check(3) {
		return errPfrInvalidTable
	}
	numAux := uint64(c.uoff3())
	if numAux > 0 {
		q := c.p
		if !c.check(numAux) {
			return errPfrInvalidTable
		}
		c.p += int(numAux)
		p := c.p
		for numAux > 0 {
			if q+4 > p {
				break
			}
			length := uint64(data[q])<<8 | uint64(data[q+1])
			q += 2
			if length < 4 || length > numAux {
				break
			}
			q2 := q + int(length) - 2
			typ := uint32(data[q])<<8 | uint32(data[q+1])
			q += 2
			switch typ {
			case 1:
				pf.familyName = pfrAuxNameLoad(data[q : q+int(length)-4])
			case 2:
				if q+32 > q2 {
					break
				}
				r := &pfrCursor{b: data, p: q + 10}
				pf.ascent = r.short()
				pf.descent = r.short()
				pf.leading = r.short()
			case 3:
				pf.styleName = pfrAuxNameLoad(data[q : q+int(length)-4])
			}
			q = q2
			numAux -= length
		}
	}

	// the blue values
	if !c.check(1) {
		return errPfrInvalidTable
	}
	count := uint64(c.byte())
	if !c.check(count * 2) {
		return errPfrInvalidTable
	}
	c.p += int(count * 2)

	if !c.check(8) {
		return errPfrInvalidTable
	}
	c.p += 6 // blue fuzz and scale, the standard stems

	// the character descriptors
	numChars := c.ushort()
	if numChars == 0 {
		return errPfrInvalidTable
	}
	size1 := uint64(1 + 1 + 2)
	for _, f := range []uint32{pfrPhy2ByteCharcode, pfrPhyProportional, pfrPhyASCIICode,
		pfrPhy2ByteGpsSize, pfrPhy3ByteGpsOffset} {
		if flags&f != 0 {
			size1++
		}
	}
	if flags&pfrPhyProportional != 0 {
		size1++ // a 2-byte advance
	}
	if !c.check(uint64(numChars) * size1) {
		return errPfrInvalidTable
	}
	pf.chars = make([]pfrChar, numChars)
	for n := range pf.chars {
		ch := &pf.chars[n]
		if flags&pfrPhy2ByteCharcode != 0 {
			ch.charCode = c.ushort()
		} else {
			ch.charCode = c.byte()
		}
		if flags&pfrPhyProportional != 0 {
			ch.advance = c.short()
		} else {
			ch.advance = pf.standardAdvance
		}
		if flags&pfrPhyASCIICode != 0 {
			c.p++
		}
		if flags&pfrPhy2ByteGpsSize != 0 {
			ch.gpsSize = c.ushort()
		} else {
			ch.gpsSize = c.byte()
		}
		if flags&pfrPhy3ByteGpsOffset != 0 {
			ch.gpsOffset = c.uoff3()
		} else {
			ch.gpsOffset = c.ushort()
		}
	}
	return nil
}

// loadKerningPairs is pfr_extra_item_load_kerning_pairs; start is the
// physical font record's.
func (pf *pfrPhyFont) loadKerningPairs(c *pfrCursor, start int) error {
	if !c.check(4) {
		return errPfrInvalidTable
	}
	item := &pfrKernItem{}
	item.pairCount = c.byte()
	item.baseAdj = c.short()
	item.flags = c.byte()
	item.offset = uint64(pf.offset) + uint64(c.p-start)
	item.pairSize = 3
	if item.flags&pfrKern2ByteChar != 0 {
		item.pairSize += 2
	}
	if item.flags&pfrKern2ByteAdj != 0 {
		item.pairSize++
	}
	if !c.check(uint64(item.pairCount) * uint64(item.pairSize)) {
		return errPfrInvalidTable
	}
	if item.pairCount == 0 {
		// an empty item
		return nil
	}
	// the first and last pairs, for the lookup
	first := &pfrCursor{b: c.b, p: c.p}
	last := &pfrCursor{b: c.b, p: c.p + int(item.pairSize*(item.pairCount-1))}
	if item.flags&pfrKern2ByteChar != 0 {
		item.pair1 = pfrKernIndex(first.ushort(), first.ushort())
		item.pair2 = pfrKernIndex(last.ushort(), last.ushort())
	} else {
		item.pair1 = pfrKernIndex(first.byte(), first.byte())
		item.pair2 = pfrKernIndex(last.byte(), last.byte())
	}
	pf.kernItems = append(pf.kernItems, item)
	pf.numKernPairs += item.pairCount
	return nil
}

// pfrKernIndex is PFR_KERN_INDEX.
func pfrKernIndex(g1, g2 uint32) uint32 { return g1<<16 | uint32(uint16(g2)) }

// pfrFaceInit is pfr_face_init: a PFR font as a face. It reports
// errUnknownFormat for other files.
func pfrFaceInit(data []byte) (*Face, error) {
	header, ok := pfrHeaderLoad(data)
	if !ok {
		return nil, errUnknownFormat
	}
	numFaces, err := pfrLogFontCount(data, header.logDirOffset)
	if err != nil {
		return nil, err
	}
	if numFaces <= 0 {
		// face index 0 is invalid
		return nil, errInvalidArg
	}
	pfr := &pfrFace{header: header}
	if err := pfrLogFontLoad(&pfr.logFont, data, 0, header.logDirOffset,
		header.phyFontMaxSizeHigh != 0); err != nil {
		return nil, err
	}
	pf := &pfr.phyFont
	if err := pfrPhyFontLoad(pf, data, pfr.logFont.physOffset, pfr.logFont.physSize); err != nil {
		return nil, err
	}

	f := &Face{data: data, pfr: pfr}
	f.numGlyphs = int64(len(pf.chars)) + 1

	// with gps_offset 0 for every character, the font has bitmaps only
	f.pfrScalable = true
	nn := 0
	for nn < len(pf.chars) && pf.chars[nn].gpsOffset == 0 {
		nn++
	}
	if nn == len(pf.chars) {
		if pf.numStrikes > 0 {
			f.pfrScalable = false
		} else {
			return nil, errInvalidFileFormat
		}
	}

	// without a family name in the auxiliary data, the font ID
	if pf.familyName != nil {
		f.FamilyName = *pf.familyName
	} else if pf.fontID != nil {
		f.FamilyName = *pf.fontID
	}
	// C++ reads a NULL style name (and family name) as a string
	if pf.styleName != nil {
		f.StyleName = *pf.styleName
	}

	f.unitsPerEM = int64(uint16(pf.outlineResolution))
	f.ascender = int64(int16(pf.bbox.YMax))
	f.descender = int64(int16(pf.bbox.YMin))

	// the charmap (pfr_cmap_init): character codes must increase
	for n := 1; n < len(pf.chars); n++ {
		if pf.chars[n-1].charCode >= pf.chars[n].charCode {
			return nil, errPfrInvalidTable
		}
	}
	f.charmaps = append(f.charmaps, &charmap{
		platformID: platformMicrosoft,
		encodingID: msIDUnicode,
		encoding:   encodingUnicode,
		pfr:        pf.chars,
	})
	return f, nil
}

// pfrCmapCharIndex is pfr_cmap_char_index.
func pfrCmapCharIndex(chars []pfrChar, charCode uint32) uint32 {
	min, max := uint32(0), uint32(len(chars))
	mid := min + (max-min)/2
	for min < max {
		g := &chars[mid]
		if g.charCode == charCode {
			return mid + 1
		}
		if g.charCode < charCode {
			min = mid + 1
		} else {
			max = mid
		}
		// reasonable prediction in a continuous block
		mid += charCode - g.charCode
		if mid >= max || mid < min {
			mid = min + (max-min)/2
		}
	}
	return 0
}

// pfrKerning is pfr_get_kerning (with pfr_face_get_kerning): the
// kerning of two glyphs, in font units.
func (f *Face) pfrKerning(glyph1, glyph2 uint32) int64 {
	pf := &f.pfr.phyFont
	var kern int64

	// PFR indexing skips .notdef, which becomes UINT_MAX
	glyph1--
	glyph2--
	if glyph1 < uint32(len(pf.chars)) && glyph2 < uint32(len(pf.chars)) {
		kern = f.pfrFaceKerning(pf, pfrKernIndex(pf.chars[glyph1].charCode,
			pf.chars[glyph2].charCode))
	}

	// from metrics to outline units
	if pf.outlineResolution != pf.metricsResolution && kern != 0 {
		kern = mulDiv(kern, int64(pf.outlineResolution), int64(pf.metricsResolution))
	}
	return kern
}

// pfrFaceKerning is pfr_face_get_kerning's search of the kerning items
// for pair.
func (f *Face) pfrFaceKerning(pf *pfrPhyFont, pair uint32) int64 {
	var item *pfrKernItem
	for _, it := range pf.kernItems {
		if pair >= it.pair1 && pair <= it.pair2 {
			item = it
			break
		}
	}
	if item == nil {
		return 0
	}
	data := f.data
	// FT_STREAM_SEEK and FT_FRAME_ENTER
	frame := uint64(item.pairCount) * uint64(item.pairSize)
	if item.offset > uint64(len(data)) || frame > uint64(len(data))-item.offset {
		return 0
	}

	count := item.pairCount
	size := item.pairSize
	power := uint32(1) << (bits.Len32(count) - 1)
	probe := power * size
	extra := count - power
	base := int(item.offset)
	twoBytes := item.flags&pfrKern2ByteChar != 0
	twoByteAdj := item.flags&pfrKern2ByteAdj != 0
	var p int
	readPair := func() uint32 {
		var v uint32
		if twoBytes {
			v = peek32(data, p)
			p += 4
		} else {
			v = uint32(byteAt(data, p))<<16 | uint32(byteAt(data, p+1))
			p += 2
		}
		return v
	}
	value := func() int64 {
		if twoByteAdj {
			return int64(item.baseAdj) + int64(int16(peek16(data, p)))
		}
		return int64(item.baseAdj) + int64(byteAt(data, p))
	}

	if extra > 0 {
		p = base + int(extra*size)
		cpair := readPair()
		if cpair == pair {
			return value()
		}
		if cpair < pair {
			if twoByteAdj {
				p += 2
			} else {
				p++
			}
			base = p
		}
	}
	for probe > size {
		probe >>= 1
		p = base + int(probe)
		cpair := readPair()
		if cpair == pair {
			return value()
		}
		if cpair < pair {
			base += int(probe)
		}
	}
	p = base
	if readPair() == pair {
		return value()
	}
	return 0
}

// pfrSubGlyph is a PFR_SubGlyphRec.
type pfrSubGlyph struct {
	xScale, yScale int64
	xDelta, yDelta int64
	gpsOffset      uint32
	gpsSize        uint32
}

// pfrGlyph is a PFR_GlyphRec, with its glyph loader: the base outline,
// and the current one (whose contours count its own points).
type pfrGlyph struct {
	xControl  []int64
	numSubs   int
	subs      []pfrSubGlyph
	base, cur Outline
	pathBegun bool
}

// checkPoints is FT_GLYPHLOADER_CHECK_POINTS's limit on the outline's
// size.
func (g *pfrGlyph) checkPoints(points, contours int) error {
	if len(g.base.Points)+len(g.cur.Points)+points > 0xFFFF ||
		len(g.base.Contours)+len(g.cur.Contours)+contours > 0xFFFF {
		return errInvalidArg // Array_Too_Large
	}
	return nil
}

// closeContour is pfr_glyph_close_contour.
func (g *pfrGlyph) closeContour() {
	if !g.pathBegun {
		return
	}
	o := &g.cur
	last := len(o.Points) - 1
	first := 0
	if len(o.Contours) > 0 {
		// the previous contour's last point, as in FreeType
		first = o.Contours[len(o.Contours)-1]
	}
	// if the last point is on the first, delete it
	if last > first {
		if o.Points[first] == o.Points[last] {
			o.Points = o.Points[:last]
			o.Tags = o.Tags[:last]
			last--
		}
	}
	// don't add empty contours
	if last >= first {
		o.Contours = append(o.Contours, int(uint16(last)))
	}
	g.pathBegun = false
}

// lineTo is pfr_glyph_line_to.
func (g *pfrGlyph) lineTo(to Vector) error {
	if !g.pathBegun {
		return errPfrInvalidTable
	}
	if err := g.checkPoints(1, 0); err != nil {
		return err
	}
	g.cur.Points = append(g.cur.Points, to)
	g.cur.Tags = append(g.cur.Tags, curveTagOn)
	return nil
}

// curveTo is pfr_glyph_curve_to.
func (g *pfrGlyph) curveTo(c1, c2, to Vector) error {
	if !g.pathBegun {
		return errPfrInvalidTable
	}
	if err := g.checkPoints(3, 0); err != nil {
		return err
	}
	g.cur.Points = append(g.cur.Points, c1, c2, to)
	g.cur.Tags = append(g.cur.Tags, curveTagCubic, curveTagCubic, curveTagOn)
	return nil
}

// moveTo is pfr_glyph_move_to.
func (g *pfrGlyph) moveTo(to Vector) error {
	g.closeContour()
	g.pathBegun = true
	if err := g.checkPoints(1, 1); err != nil {
		return err
	}
	return g.lineTo(to)
}

// end is pfr_glyph_end: the contour closed, and the current glyph
// added to the base (FT_GlyphLoader_Add).
func (g *pfrGlyph) end() {
	g.closeContour()
	n := len(g.base.Points)
	g.base.Points = append(g.base.Points, g.cur.Points...)
	g.base.Tags = append(g.base.Tags, g.cur.Tags...)
	for _, c := range g.cur.Contours {
		g.base.Contours = append(g.base.Contours, int(uint16(c+n)))
	}
	g.cur = Outline{}
}

// loadSimple is pfr_glyph_load_simple.
func (g *pfrGlyph) loadSimple(c *pfrCursor) error {
	if !c.check(1) {
		return errPfrInvalidTable
	}
	flags := c.byte()
	if flags&pfrGlyphIsCompound != 0 {
		return errPfrInvalidTable
	}
	var xCount, yCount uint32
	if flags&pfrGlyph1ByteXYCount != 0 {
		if !c.check(1) {
			return errPfrInvalidTable
		}
		count := c.byte()
		xCount = count & 15
		yCount = count >> 4
	} else {
		if flags&pfrGlyphXCount != 0 {
			if !c.check(1) {
				return errPfrInvalidTable
			}
			xCount = c.byte()
		}
		if flags&pfrGlyphYCount != 0 {
			if !c.check(1) {
				return errPfrInvalidTable
			}
			yCount = c.byte()
		}
	}
	count := xCount + yCount
	if int(count) > len(g.xControl) {
		g.xControl = append(g.xControl, make([]int64, int(count)-len(g.xControl))...)
	}
	yControl := g.xControl[xCount:]

	var mask uint32
	var x int32
	for i := uint32(0); i < count; i++ {
		if i&7 == 0 {
			if !c.check(1) {
				return errPfrInvalidTable
			}
			mask = c.byte()
		}
		if mask&1 != 0 {
			if !c.check(2) {
				return errPfrInvalidTable
			}
			x = c.short()
		} else {
			if !c.check(1) {
				return errPfrInvalidTable
			}
			x += int32(c.byte())
		}
		g.xControl[i] = int64(x)
		mask >>= 1
	}

	// the secondary stroke and edge definitions (hints) are ignored
	if flags&pfrGlyphSingleExtra != 0 {
		if err := pfrExtraItemsParse(c, nil); err != nil {
			return err
		}
	}

	g.pathBegun = false
	var pos [4]Vector
	for {
		if !c.check(1) {
			return errPfrInvalidTable
		}
		format := c.byte()
		formatLow := format & 15
		var argsFormat, argsCount uint32
		switch format >> 4 {
		case 0: // end glyph
			argsCount = 0
		case 1, 4, 5: // general line, move to inside or outside contour
			argsFormat = formatLow
			argsCount = 1
		case 2: // horizontal line to
			if formatLow >= xCount {
				return errPfrInvalidTable
			}
			pos[0].X = g.xControl[formatLow]
			pos[0].Y = pos[3].Y
			pos[3] = pos[0]
			argsCount = 0
		case 3: // vertical line to
			if formatLow >= yCount {
				return errPfrInvalidTable
			}
			pos[0].X = pos[3].X
			pos[0].Y = yControl[formatLow]
			pos[3] = pos[0]
			argsCount = 0
		case 6: // horizontal to vertical curve
			argsFormat = 0xB8E
			argsCount = 3
		case 7: // vertical to horizontal curve
			argsFormat = 0xE2B
			argsCount = 3
		default: // general curve to
			argsCount = 4
			argsFormat = formatLow
		}

		// the arguments
		cur := 0
		for n := uint32(0); n < argsCount; n++ {
			switch argsFormat & 3 {
			case 0: // 8-bit index
				if !c.check(1) {
					return errPfrInvalidTable
				}
				idx := c.byte()
				if idx >= xCount {
					return errPfrInvalidTable
				}
				pos[cur].X = g.xControl[idx]
			case 1: // 16-bit absolute value
				if !c.check(2) {
					return errPfrInvalidTable
				}
				pos[cur].X = int64(c.short())
			case 2: // 8-bit delta
				if !c.check(1) {
					return errPfrInvalidTable
				}
				pos[cur].X = pos[3].X + int64(c.int8())
			default:
				pos[cur].X = pos[3].X
			}
			switch (argsFormat >> 2) & 3 {
			case 0:
				if !c.check(1) {
					return errPfrInvalidTable
				}
				idx := c.byte()
				if idx >= yCount {
					return errPfrInvalidTable
				}
				pos[cur].Y = yControl[idx]
			case 1:
				if !c.check(2) {
					return errPfrInvalidTable
				}
				pos[cur].Y = int64(c.short())
			case 2:
				if !c.check(1) {
					return errPfrInvalidTable
				}
				pos[cur].Y = pos[3].Y + int64(c.int8())
			default:
				pos[cur].Y = pos[3].Y
			}
			// the general curve's additional format byte
			if n == 0 && argsCount == 4 {
				if !c.check(1) {
					return errPfrInvalidTable
				}
				argsFormat = c.byte()
				argsCount--
			} else {
				argsFormat >>= 4
			}
			// save the previous point
			pos[3] = pos[cur]
			cur++
		}

		var err error
		switch format >> 4 {
		case 0:
			g.end()
			return nil
		case 1, 2, 3:
			err = g.lineTo(pos[0])
		case 4, 5:
			err = g.moveTo(pos[0])
		default:
			err = g.curveTo(pos[0], pos[1], pos[2])
		}
		if err != nil {
			return err
		}
	}
}

// loadCompound is pfr_glyph_load_compound.
func (g *pfrGlyph) loadCompound(c *pfrCursor) error {
	if !c.check(1) {
		return errPfrInvalidTable
	}
	flags := c.byte()
	if flags&pfrGlyphIsCompound == 0 {
		return errPfrInvalidTable
	}
	count := int(flags & 0x3F)
	if flags&pfrGlyphCompoundExtra != 0 {
		if err := pfrExtraItemsParse(c, nil); err != nil {
			return err
		}
	}
	// the number of subglyphs is limited, against endless recursion
	if (g.numSubs+count+3)&^3 > 64 {
		return errPfrInvalidTable
	}
	for len(g.subs) < g.numSubs+count {
		g.subs = append(g.subs, pfrSubGlyph{})
	}
	for i := 0; i < count; i++ {
		sub := &g.subs[g.numSubs]
		var xPos, yPos int64
		if !c.check(1) {
			return errPfrInvalidTable
		}
		format := c.byte()
		sub.xScale = 0x10000
		if format&pfrSubglyphXScale != 0 {
			if !c.check(2) {
				return errPfrInvalidTable
			}
			sub.xScale = int64(c.short()) * 16
		}
		sub.yScale = 0x10000
		if format&pfrSubglyphYScale != 0 {
			if !c.check(2) {
				return errPfrInvalidTable
			}
			sub.yScale = int64(c.short()) * 16
		}
		switch format & 3 {
		case 1:
			if !c.check(2) {
				return errPfrInvalidTable
			}
			xPos = int64(c.short())
		case 2:
			if !c.check(1) {
				return errPfrInvalidTable
			}
			xPos += int64(c.int8())
		}
		switch (format >> 2) & 3 {
		case 1:
			if !c.check(2) {
				return errPfrInvalidTable
			}
			yPos = int64(c.short())
		case 2:
			if !c.check(1) {
				return errPfrInvalidTable
			}
			yPos += int64(c.int8())
		}
		sub.xDelta = xPos
		sub.yDelta = yPos
		if format&pfrSubglyph2ByteSize != 0 {
			if !c.check(2) {
				return errPfrInvalidTable
			}
			sub.gpsSize = c.ushort()
		} else {
			if !c.check(1) {
				return errPfrInvalidTable
			}
			sub.gpsSize = c.byte()
		}
		if format&pfrSubglyph3ByteOffset != 0 {
			if !c.check(3) {
				return errPfrInvalidTable
			}
			sub.gpsOffset = c.uoff3()
		} else {
			if !c.check(2) {
				return errPfrInvalidTable
			}
			sub.gpsOffset = c.ushort()
		}
		g.numSubs++
	}
	return nil
}

// loadRec is pfr_glyph_load_rec.
func (g *pfrGlyph) loadRec(data []byte, gpsOffset, offset, size uint64) error {
	pos := gpsOffset + offset
	if pos > uint64(len(data)) || size > uint64(len(data))-pos {
		return errInvalidTable
	}
	c := &pfrCursor{b: data, p: int(pos), limit: int(pos + size)}
	if size == 0 || data[c.p]&pfrGlyphIsCompound == 0 {
		return g.loadSimple(c)
	}

	oldCount := g.numSubs
	if err := g.loadCompound(c); err != nil {
		return err
	}
	count := g.numSubs - oldCount
	for n := 0; n < count; n++ {
		sub := g.subs[oldCount+n]
		oldPoints := len(g.base.Points)
		if err := g.loadRec(data, gpsOffset, uint64(sub.gpsOffset), uint64(sub.gpsSize)); err != nil {
			return err
		}
		// translate and scale the new points
		pts := g.base.Points[oldPoints:]
		for i := range pts {
			if sub.xScale != 0x10000 || sub.yScale != 0x10000 {
				pts[i].X = mulFix(pts[i].X, sub.xScale) + sub.xDelta
				pts[i].Y = mulFix(pts[i].Y, sub.yScale) + sub.yDelta
			} else {
				pts[i].X += sub.xDelta
				pts[i].Y += sub.yDelta
			}
		}
	}
	return nil
}

// loadPFRGlyph is pfr_slot_load with FT_LOAD_NO_BITMAP |
// FT_LOAD_NO_HINTING.
func (f *Face) loadPFRGlyph(gindex uint32) (*Glyph, error) {
	pfr := f.pfr
	pf := &pfr.phyFont
	if gindex > 0 {
		gindex--
	}
	if gindex >= uint32(len(pf.chars)) {
		return nil, errInvalidArg
	}
	// FT_Load_Glyph loads unscaled glyphs for a zero ppem.
	scaled := f.xPpem != 0 && f.yPpem != 0
	gchar := &pf.chars[gindex]

	// pfr_glyph_load
	g := &pfr.glyph
	g.base, g.cur = Outline{}, Outline{}
	g.numSubs = 0
	if err := g.loadRec(f.data, uint64(pfr.header.gpsSectionOffset), uint64(gchar.gpsOffset),
		uint64(gchar.gpsSize)); err != nil {
		return nil, err
	}

	out := &Glyph{Outline: Outline{
		Points:   append([]Vector(nil), g.base.Points...),
		Tags:     append([]byte(nil), g.base.Tags...),
		Contours: append([]int(nil), g.base.Contours...),
	}}
	advance := int64(gchar.advance)
	if pf.metricsResolution != pf.outlineResolution {
		advance = mulDiv(advance, int64(pf.outlineResolution), int64(pf.metricsResolution))
	}
	var horiAdvance int64
	if pf.flags&pfrPhyVertical == 0 {
		horiAdvance = advance
	}
	if scaled {
		for i := range out.Outline.Points {
			out.Outline.Points[i].X = mulFix(out.Outline.Points[i].X, f.xScale)
			out.Outline.Points[i].Y = mulFix(out.Outline.Points[i].Y, f.yScale)
		}
		horiAdvance = mulFix(horiAdvance, f.xScale)
	}
	cbox := out.Outline.CBox()
	out.HoriBearingX = cbox.XMin
	out.AdvanceX = horiAdvance
	if err := out.Outline.check(); err != nil {
		return nil, err
	}
	return out, nil
}

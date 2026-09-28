package ttf

import (
	"encoding/binary"
	"errors"
)

// FreeType's CFF driver: loading the CFF and CFF2 tables (cffload.c) and
// parsing their DICTs (cffparse.c).

var (
	errInvalidStream   = errors.New("ttf: invalid stream operation")
	errInvalidFormat   = errors.New("ttf: invalid file format")
	errStackUnderflow  = errors.New("ttf: stack underflow")
	errSyntax          = errors.New("ttf: syntax error")
	errInvalidGlyphFmt = errors.New("ttf: invalid glyph format")
)

// cffStream is an FT_Stream over a font file in memory, with its
// position; reads past the end fail as FreeType's do.
type cffStream struct {
	data []byte
	pos  int64
}

func (s *cffStream) size() int64 { return int64(len(s.data)) }

// seek is FT_Stream_Seek: seeking to the end is allowed.
func (s *cffStream) seek(pos int64) error {
	if pos < 0 || pos > s.size() {
		return errInvalidStream
	}
	s.pos = pos
	return nil
}

// skip is FT_Stream_Skip.
func (s *cffStream) skip(n int64) error {
	if n < 0 {
		return errInvalidStream
	}
	return s.seek(s.pos + n)
}

func (s *cffStream) readByte() (byte, error) {
	if s.pos >= s.size() {
		return 0, errInvalidStream
	}
	b := s.data[s.pos]
	s.pos++
	return b, nil
}

func (s *cffStream) readU16() (uint16, error) {
	if s.pos+1 >= s.size() {
		return 0, errInvalidStream
	}
	v := binary.BigEndian.Uint16(s.data[s.pos:])
	s.pos += 2
	return v, nil
}

func (s *cffStream) readU32() (uint32, error) {
	if s.pos+3 >= s.size() {
		return 0, errInvalidStream
	}
	v := binary.BigEndian.Uint32(s.data[s.pos:])
	s.pos += 4
	return v, nil
}

// read is FT_Stream_Read.
func (s *cffStream) read(n int64) ([]byte, error) {
	if s.pos >= s.size() {
		return nil, errInvalidStream
	}
	got := s.size() - s.pos
	if got > n {
		got = n
	}
	b := s.data[s.pos : s.pos+got]
	s.pos += got
	if got < n {
		return nil, errInvalidStream
	}
	return b, nil
}

// frame is FT_Stream_EnterFrame (and FT_Stream_ExtractFrame) for a
// memory stream: even an empty frame at the end fails.
func (s *cffStream) frame(n uint64) ([]byte, error) {
	if s.pos >= s.size() || uint64(s.size()-s.pos) < n {
		return nil, errInvalidStream
	}
	b := s.data[s.pos : s.pos+int64(n)]
	s.pos += int64(n)
	return b, nil
}

// cffIndex is a CFF_IndexRec.
type cffIndex struct {
	valid      bool // the C++ stream pointer is set
	start      int64
	hdrSize    int64
	count      uint32
	offSize    byte
	dataOffset int64
	dataSize   uint64
	offsets    []uint64
	bytes      []byte // the loaded data, if any
}

// readOffset is cff_index_read_offset.
func (idx *cffIndex) readOffset(s *cffStream) (uint64, error) {
	tmp, err := s.read(int64(idx.offSize))
	if err != nil {
		return 0, err
	}
	var result uint64
	for _, b := range tmp {
		result = result<<8 | uint64(b)
	}
	return result, nil
}

// cffIndexInit is cff_index_init.
func cffIndexInit(s *cffStream, load, cff2 bool) (*cffIndex, error) {
	idx := &cffIndex{valid: true, start: s.pos}
	var count uint32
	if cff2 {
		v, err := s.readU32()
		if err != nil {
			return idx, err
		}
		count = v
		idx.hdrSize = 5
	} else {
		v, err := s.readU16()
		if err != nil {
			return idx, err
		}
		count = uint32(v)
		idx.hdrSize = 3
	}
	if count > 0 {
		offsize, err := s.readByte()
		if err != nil {
			return idx, err
		}
		if offsize < 1 || offsize > 4 {
			return idx, errInvalidTable
		}
		idx.count = count
		idx.offSize = offsize
		size := uint64(count+1) * uint64(offsize)
		idx.dataOffset = idx.start + idx.hdrSize + int64(size)
		if err := s.skip(int64(size - uint64(offsize))); err != nil {
			return idx, err
		}
		size, err = idx.readOffset(s)
		if err != nil {
			return idx, err
		}
		if size == 0 {
			return idx, errInvalidTable
		}
		size--
		idx.dataSize = size
		if load {
			if idx.bytes, err = s.frame(size); err != nil {
				return idx, err
			}
		} else if err := s.skip(int64(size)); err != nil {
			return idx, err
		}
	}
	return idx, nil
}

// loadOffsets is cff_index_load_offsets.
func (idx *cffIndex) loadOffsets(s *cffStream) error {
	if idx.count == 0 || idx.offsets != nil {
		return nil
	}
	offsize := int(idx.offSize)
	dataSize := uint64(idx.count+1) * uint64(offsize)
	if err := s.seek(idx.start + idx.hdrSize); err != nil {
		return err
	}
	p, err := s.frame(dataSize)
	if err != nil {
		return err
	}
	offsets := make([]uint64, idx.count+1)
	for i := range offsets {
		var v uint64
		for _, b := range p[i*offsize : (i+1)*offsize] {
			v = v<<8 | uint64(b)
		}
		offsets[i] = v
	}
	idx.offsets = offsets
	return nil
}

// getPointers is cff_index_get_pointers without a pool: the start of
// each element (and the end of the last) in idx.bytes.
func (idx *cffIndex) getPointers(s *cffStream) ([]uint64, error) {
	if idx.offsets == nil {
		if err := idx.loadOffsets(s); err != nil {
			return nil, err
		}
	}
	if idx.count == 0 {
		return nil, nil
	}
	tbl := make([]uint64, idx.count+1)
	cur := idx.offsets[0] - 1
	if cur != 0 {
		cur = 0
	}
	tbl[0] = cur
	for n := uint32(1); n <= idx.count; n++ {
		next := idx.offsets[n] - 1
		if next < cur {
			next = cur
		} else if next > idx.dataSize {
			next = idx.dataSize
		}
		tbl[n] = next
		cur = next
	}
	return tbl, nil
}

// getStrings is cff_index_get_pointers with a pool: the elements copied
// into a pool as C strings. As in C, an empty element points at where
// the next one is copied, so it reads as that one.
func (idx *cffIndex) getStrings(s *cffStream) ([]byte, []uint64, error) {
	if idx.offsets == nil {
		if err := idx.loadOffsets(s); err != nil {
			return nil, nil, err
		}
	}
	if idx.count == 0 {
		return nil, nil, nil
	}
	pool := make([]byte, idx.dataSize+uint64(idx.count))
	tbl := make([]uint64, idx.count+1)
	org := idx.bytes
	cur := idx.offsets[0] - 1
	if cur != 0 {
		cur = 0
	}
	tbl[0] = cur
	extra := uint64(0)
	for n := uint32(1); n <= idx.count; n++ {
		next := idx.offsets[n] - 1
		if next < cur {
			next = cur
		} else if next > idx.dataSize {
			next = idx.dataSize
		}
		tbl[n] = next + extra
		if next != cur {
			copy(pool[tbl[n-1]:tbl[n]], org[cur:cur+(tbl[n]-tbl[n-1])])
			pool[tbl[n]] = 0
			tbl[n]++
			extra++
		}
		cur = next
	}
	return pool, tbl, nil
}

// accessElement is cff_index_access_element: an element's bytes (nil
// for an empty one).
func (idx *cffIndex) accessElement(s *cffStream, element uint32) ([]byte, error) {
	if idx == nil || idx.count <= element {
		return nil, errInvalidArg
	}
	var off1, off2 uint64
	if idx.offsets == nil {
		pos := int64(element) * int64(idx.offSize)
		if err := s.seek(idx.start + idx.hdrSize + pos); err != nil {
			return nil, err
		}
		var err error
		if off1, err = idx.readOffset(s); err != nil {
			return nil, err
		}
		if off1 != 0 {
			for {
				element++
				if off2, err = idx.readOffset(s); err != nil {
					return nil, err
				}
				if !(off2 == 0 && element < idx.count) {
					break
				}
			}
		}
	} else {
		off1 = idx.offsets[element]
		if off1 != 0 {
			for {
				element++
				off2 = idx.offsets[element]
				if !(off2 == 0 && element < idx.count) {
					break
				}
			}
		}
	}

	size := uint64(s.size())
	if off2 > size+1 || uint64(idx.dataOffset) > size-off2+1 {
		off2 = size - uint64(idx.dataOffset) + 1
	}
	if off1 != 0 && off2 > off1 {
		if idx.bytes != nil {
			return idx.bytes[off1-1 : off1-1+(off2-off1)], nil
		}
		if err := s.seek(idx.dataOffset + int64(off1) - 1); err != nil {
			return nil, err
		}
		return s.frame(off2 - off1)
	}
	return nil, nil
}

// DICT parsing (cffparse.c).

const (
	cffMaxStackDepth = 96
	cff2MaxStack     = 513
	cff2DefaultStack = 513

	cffCodeTopDict   = 0x1000
	cffCodePrivate   = 0x2000
	cff2CodeTopDict  = 0x3000
	cff2CodeFontDict = 0x4000
	cff2CodePrivate  = 0x5000
)

var powerTens = [...]int64{
	1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000,
}

var powerTenLimits = [...]int64{
	maxInt64 / 1, maxInt64 / 10, maxInt64 / 100, maxInt64 / 1000,
	maxInt64 / 10000, maxInt64 / 100000, maxInt64 / 1000000,
	maxInt64 / 10000000, maxInt64 / 100000000, maxInt64 / 1000000000,
}

const (
	maxInt64 = 1<<63 - 1
	minInt64 = -1 << 63
)

// A parser operand is the DICT (or blend stack) from the operand on;
// its length is the distance to the parser's limit.

// parseInteger is cff_parse_integer.
func parseInteger(d []byte) int64 {
	v := int(d[0])
	p := d[1:]
	switch {
	case v == 28:
		if len(p) < 2 {
			return 0
		}
		return int64(int16(uint16(p[0])<<8 | uint16(p[1])))
	case v == 29:
		if len(p) < 4 {
			return 0
		}
		return int64(uint64(p[0])<<24 | uint64(p[1])<<16 | uint64(p[2])<<8 | uint64(p[3]))
	case v < 247:
		return int64(v - 139)
	case v < 251:
		if len(p) < 1 {
			return 0
		}
		return int64((v-247)*256 + int(p[0]) + 108)
	default:
		if len(p) < 1 {
			return 0
		}
		return int64(-(v-251)*256 - int(p[0]) - 108)
	}
}

// parseReal is cff_parse_real; scaling is nil unless the dynamic
// scaling is wanted.
func parseReal(d []byte, powerTen int64, scaling *int64) int64 {
	if scaling != nil {
		*scaling = 0
	}
	var result, number, exponent int64
	var sign, exponentSign, haveOverflow bool
	var exponentAdd, integerLength, fractionLength int64

	p := 0
	phase := uint(4)
	var nib int
	// nibble reads the next nibble; ok is false past the end.
	nibble := func() bool {
		if phase != 0 {
			p++
			if p+1 > len(d) {
				return false
			}
		}
		nib = int(d[p]>>phase) & 0xF
		phase = 4 - phase
		return true
	}

	result = 0
	finish := func() int64 {
		if sign {
			return -result
		}
		return result
	}

	for {
		if !nibble() {
			return 0 // Bad
		}
		if nib == 0xE {
			sign = true
		} else if nib > 9 {
			break
		} else {
			if number >= 0xCCCCCCC {
				exponentAdd++
			} else if nib != 0 || number != 0 {
				integerLength++
				number = number*10 + int64(nib)
			}
		}
	}

	if nib == 0xA {
		for {
			if !nibble() {
				return 0
			}
			if nib >= 10 {
				break
			}
			if nib == 0 && number == 0 {
				exponentAdd--
			} else if number < 0xCCCCCCC && fractionLength < 9 {
				fractionLength++
				number = number*10 + int64(nib)
			}
		}
	}

	if nib == 12 {
		exponentSign = true
		nib = 11
	}
	if nib == 11 {
		for {
			if !nibble() {
				return 0
			}
			if nib >= 10 {
				break
			}
			if exponent > 1000 {
				haveOverflow = true
			} else {
				exponent = exponent*10 + int64(nib)
			}
		}
		if exponentSign {
			exponent = -exponent
		}
	}

	if number == 0 {
		return finish()
	}
	if haveOverflow {
		if exponentSign {
			result = 0
		} else {
			result = 0x7FFFFFFF
		}
		return finish()
	}

	exponent += powerTen + exponentAdd

	if scaling != nil {
		fractionLength += integerLength
		exponent += integerLength
		if fractionLength <= 5 {
			if number > 0x7FFF {
				result = divFix(number, 10)
				*scaling = exponent - fractionLength + 1
			} else {
				if exponent > 0 {
					newFractionLength := exponent
					if newFractionLength > 5 {
						newFractionLength = 5
					}
					shift := newFractionLength - fractionLength
					if shift > 0 {
						exponent -= newFractionLength
						number *= powerTens[shift]
						if number > 0x7FFF {
							number /= 10
							exponent++
						}
					} else {
						exponent -= fractionLength
					}
				} else {
					exponent -= fractionLength
				}
				result = int64(uint64(number) << 16)
				*scaling = exponent
			}
		} else {
			if number/powerTens[fractionLength-5] > 0x7FFF {
				result = divFix(number, powerTens[fractionLength-4])
				*scaling = exponent - 4
			} else {
				result = divFix(number, powerTens[fractionLength-5])
				*scaling = exponent - 5
			}
		}
	} else {
		integerLength += exponent
		fractionLength -= exponent
		if integerLength > 5 {
			result = 0x7FFFFFFF
			return finish()
		}
		if integerLength < -5 {
			result = 0
			return finish()
		}
		if integerLength < 0 {
			number /= powerTens[-integerLength]
			fractionLength += integerLength
		}
		if fractionLength == 10 {
			number /= 10
			fractionLength--
		}
		if fractionLength > 0 {
			if number/powerTens[fractionLength] > 0x7FFF {
				return finish()
			}
			result = divFix(number, powerTens[fractionLength])
		} else {
			number *= powerTens[-fractionLength]
			if number > 0x7FFF {
				result = 0x7FFFFFFF
				return finish()
			}
			result = int64(uint64(number) << 16)
		}
	}
	return finish()
}

// parseNum is cff_parse_num.
func parseNum(d []byte) int64 {
	switch d[0] {
	case 30:
		return parseReal(d, 0, nil) >> 16
	case 255:
		v := uint32(d[1])<<16 | uint32(d[2])<<8 | uint32(d[3])
		return int64(int16(uint16((v + 0x80) >> 8)))
	}
	return parseInteger(d)
}

func abs64s(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// doFixed is do_fixed.
func doFixed(d []byte, scaling int64) int64 {
	switch d[0] {
	case 30:
		return parseReal(d, scaling, nil)
	case 255:
		val := int64(int32(uint32(d[1])<<24 | uint32(d[2])<<16 | uint32(d[3])<<8 | uint32(d[4])))
		if scaling != 0 {
			if abs64s(val) > powerTenLimits[scaling] {
				if val > 0 {
					return 0x7FFFFFFF
				}
				return -0x7FFFFFFF
			}
			val *= powerTens[scaling]
		}
		return val
	}
	val := parseInteger(d)
	if scaling != 0 {
		if abs64s(val)<<16 > powerTenLimits[scaling] {
			if val > 0 {
				return 0x7FFFFFFF
			}
			return -0x7FFFFFFF
		}
		val *= powerTens[scaling]
	}
	if val > 0x7FFF {
		return 0x7FFFFFFF
	} else if val < -0x7FFF {
		return -0x7FFFFFFF
	}
	return int64(uint64(val) << 16)
}

// parseFixed is cff_parse_fixed.
func parseFixed(d []byte) int64 { return doFixed(d, 0) }

// parseFixedDynamic is cff_parse_fixed_dynamic.
func parseFixedDynamic(d []byte, scaling *int64) int64 {
	if d[0] == 30 {
		return parseReal(d, 0, scaling)
	}
	number := parseInteger(d)
	if number > 0x7FFF {
		integerLength := 5
		for ; integerLength < 10; integerLength++ {
			if number < powerTens[integerLength] {
				break
			}
		}
		if number/powerTens[integerLength-5] > 0x7FFF {
			*scaling = int64(integerLength - 4)
			return divFix(number, powerTens[integerLength-4])
		}
		*scaling = int64(integerLength - 5)
		return divFix(number, powerTens[integerLength-5])
	}
	*scaling = 0
	return int64(uint64(number) << 16)
}

// Matrix is an FT_Matrix.
type Matrix struct{ XX, XY, YX, YY int64 }

// matrixCheck is FT_Matrix_Check.
func matrixCheck(m Matrix) bool {
	xx, xy, yx, yy := m.XX, m.XY, m.YX, m.YY
	val := abs64s(xx) | abs64s(xy) | abs64s(yx) | abs64s(yy)
	if val == 0 || val > 0x7FFFFFFF {
		return false
	}
	shift := msb32(uint32(val)) - 12
	if shift > 0 {
		xx >>= shift
		xy >>= shift
		yx >>= shift
		yy >>= shift
	}
	temp1 := 32 * uint64(abs64s(xx*yy-xy*yx))
	temp2 := uint64(xx*xx) + uint64(xy*xy) + uint64(yx*yx) + uint64(yy*yy)
	return temp1 > temp2
}

// msb32 is FT_MSB.
func msb32(x uint32) int {
	n := 31
	for x&0x80000000 == 0 {
		x <<= 1
		n--
	}
	return n
}

// cffFontDict is a CFF_FontRecDictRec.
type cffFontDict struct {
	version, notice, copyright, fullName, familyName, weight uint32
	isFixedPitch                                             byte
	italicAngle                                              int64
	underlinePosition                                        int16
	underlineThickness                                       uint16
	paintType, charstringType                                int32
	fontMatrix                                               Matrix
	hasFontMatrix                                            bool
	unitsPerEm                                               uint64
	fontOffset                                               Vector
	uniqueID                                                 int64
	fontBBox                                                 BBox
	strokeWidth                                              int64
	charsetOffset, encodingOffset, charstringsOffset         uint64
	privateOffset, privateSize                               uint64
	syntheticBase                                            int64
	embeddedPostscript                                       uint32

	cidRegistry, cidOrdering            uint32
	cidSupplement                       int64
	cidFontVersion, cidFontRevision     int64
	cidFontType                         int64
	cidCount, cidUIDBase                uint64
	cidFDArrayOffset, cidFDSelectOffset uint64
	cidFontName                         uint32
	numDesigns, numAxes                 uint16
	vstoreOffset                        uint64
	maxstack                            uint32
}

// cffPrivate is a CFF_PrivateRec.
type cffPrivate struct {
	numBlueValues, numOtherBlues, numFamilyBlues, numFamilyOtherBlues byte
	blueValues                                                        [14]int64
	otherBlues                                                        [10]int64
	familyBlues                                                       [14]int64
	familyOtherBlues                                                  [10]int64
	blueScale                                                         int64
	blueShift, blueFuzz                                               int64
	standardWidth, standardHeight                                     int64
	numSnapWidths, numSnapHeights                                     byte
	snapWidths, snapHeights                                           [13]int64
	forceBold                                                         byte
	forceBoldThreshold                                                int64
	lenIV, languageGroup                                              int32
	expansionFactor                                                   int64
	initialRandomSeed                                                 int64
	localSubrsOffset                                                  uint64
	defaultWidth, nominalWidth                                        int64

	vsindex uint32
	subfont *cffSubFont
}

// cffBlend is a CFF_BlendRec.
type cffBlend struct {
	builtBV, usedBV bool
	font            *cffFont
	lastVsindex     uint32
	lenNDV          uint32
	lastNDV         []int64
	lenBV           uint32
	bv              []int32
}

// cffSubFont is a CFF_SubFontRec.
type cffSubFont struct {
	fontDict    cffFontDict
	privateDict cffPrivate

	blend  cffBlend
	lenNDV uint32
	ndv    []int64

	localSubrsIndex *cffIndex
	localSubrs      []uint64 // element starts in localSubrsIndex.bytes

	random uint32
}

// cffVarData, cffAxisCoords and cffVStore are CFF_VarData,
// CFF_AxisCoords and CFF_VStoreRec.
type cffVarData struct {
	regionIndices []uint16
}

type cffAxisCoords struct {
	startCoord, peakCoord, endCoord int64
}

type cffVStore struct {
	dataCount     uint32
	varData       []cffVarData
	axisCount     uint16
	regionCount   uint32
	varRegionList [][]cffAxisCoords
}

// cffEncoding and cffCharset are CFF_EncodingRec and CFF_CharsetRec.
type cffEncoding struct {
	format uint32
	offset uint64
	count  uint32
	sids   [256]uint16
	codes  [256]uint16
}

type cffCharset struct {
	format    uint32
	offset    uint64
	sids      []uint16
	cids      []uint16
	maxCID    uint32
	numGlyphs uint32
}

// cffFDSelect is a CFF_FDSelectRec.
type cffFDSelect struct {
	format     byte
	rangeCount uint32
	data       []byte
	cacheFirst uint32
	cacheCount uint32
	cacheFD    byte
}

const cffMaxCIDFonts = 256

// cffFont is a CFF_FontRec.
type cffFont struct {
	stream     *cffStream
	baseOffset int64
	numFaces   uint32
	numGlyphs  uint32

	versionMajor, versionMinor, headerSize byte
	topDictLength                          uint32
	cff2                                   bool

	nameIndex        *cffIndex
	fontDictIndex    *cffIndex
	globalSubrsIndex *cffIndex
	encoding         cffEncoding
	charset          cffCharset
	charstringsIndex *cffIndex
	globalSubrs      []uint64
	numStrings       uint32
	stringPool       []byte
	strings          []uint64
	topFont          cffSubFont
	numSubfonts      uint32
	subfonts         []*cffSubFont
	fdSelect         cffFDSelect
	fontName         string
	hasFontName      bool
	vstore           cffVStore
	psnames          bool // the psnames module is available
	cf2              *cf2Font

	face    *Face
	numAxes uint16 // of the variation blend, if any
}

// getString is cff_index_get_string, as a C string.
func (f *cffFont) getString(element uint32) (string, bool) {
	if element >= f.numStrings {
		return "", false
	}
	return cString(string(f.stringPool), int(f.strings[element])), true
}

// getSIDString is cff_index_get_sid_string.
func (f *cffFont) getSIDString(sid uint32) (string, bool) {
	if sid == 0xFFFF {
		return "", false
	}
	if sid > 390 {
		return f.getString(sid - 391)
	}
	if !f.psnames {
		return "", false
	}
	return standardString(uint(sid))
}

// getName is cff_index_get_name.
func (f *cffFont) getName(element uint32) (string, bool) {
	idx := f.nameIndex
	if idx == nil || !idx.valid {
		return "", false
	}
	b, err := idx.accessElement(f.stream, element)
	if err != nil {
		return "", false
	}
	return cString(string(b), 0), true
}

// fdSelectGet is cff_fd_select_get.
func (fs *cffFDSelect) get(glyphIndex uint32) byte {
	var fd byte
	if fs.data == nil {
		return 0
	}
	switch fs.format {
	case 0:
		fd = fs.data[glyphIndex]
	case 3:
		if glyphIndex-fs.cacheFirst < fs.cacheCount {
			return fs.cacheFD
		}
		p := fs.data
		first := uint32(binary.BigEndian.Uint16(p))
		i := 2
		for {
			if glyphIndex < first {
				break
			}
			fd2 := p[i]
			limit := uint32(binary.BigEndian.Uint16(p[i+1:]))
			i += 3
			if glyphIndex < limit {
				fd = fd2
				fs.cacheFirst = first
				fs.cacheCount = limit - first
				fs.cacheFD = fd2
				break
			}
			first = limit
			if i >= len(p) {
				break
			}
		}
	}
	return fd
}

// loadFDSelect is CFF_Load_FD_Select.
func loadFDSelect(fs *cffFDSelect, numGlyphs uint32, s *cffStream, offset int64) error {
	if err := s.seek(offset); err != nil {
		return err
	}
	format, err := s.readByte()
	if err != nil {
		return err
	}
	fs.format = format
	fs.cacheCount = 0
	var dataSize uint64
	switch format {
	case 0:
		dataSize = uint64(numGlyphs)
	case 3:
		numRanges, err := s.readU16()
		if err != nil {
			return err
		}
		if numRanges == 0 {
			return errInvalidFormat
		}
		dataSize = uint64(numRanges)*3 + 2
	default:
		return errInvalidFormat
	}
	fs.data, err = s.frame(dataSize)
	return err
}

// computeCIDs is cff_charset_compute_cids.
func (cs *cffCharset) computeCIDs(numGlyphs uint32) {
	if cs.maxCID > 0 {
		return
	}
	var maxCID uint16
	for i := uint32(0); i < numGlyphs; i++ {
		if cs.sids[i] > maxCID {
			maxCID = cs.sids[i]
		}
	}
	cs.cids = make([]uint16, int(maxCID)+1)
	for i := numGlyphs - 1; i < numGlyphs; i-- {
		cs.cids[cs.sids[i]] = uint16(i)
	}
	cs.maxCID = uint32(maxCID)
	cs.numGlyphs = numGlyphs
}

// cidToGindex is cff_charset_cid_to_gindex.
func (cs *cffCharset) cidToGindex(cid uint32) uint32 {
	if cid <= cs.maxCID {
		return uint32(cs.cids[cid])
	}
	return 0
}

// loadCharset is cff_charset_load.
func loadCharset(cs *cffCharset, numGlyphs uint32, s *cffStream, baseOffset int64,
	offset uint64, invert bool) (err error) {
	defer func() {
		if err != nil {
			cs.sids = nil
			cs.cids = nil
			cs.format = 0
			cs.offset = 0
		}
	}()
	if offset > 2 {
		cs.offset = uint64(baseOffset) + offset
		if err := s.seek(int64(cs.offset)); err != nil {
			return err
		}
		format, err := s.readByte()
		if err != nil {
			return err
		}
		cs.format = uint32(format)
		cs.sids = make([]uint16, numGlyphs)
		cs.sids[0] = 0
		switch cs.format {
		case 0:
			if numGlyphs > 0 {
				p, err := s.frame(uint64(numGlyphs-1) * 2)
				if err != nil {
					return err
				}
				for j := uint32(1); j < numGlyphs; j++ {
					cs.sids[j] = binary.BigEndian.Uint16(p[2*(j-1):])
				}
			}
		case 1, 2:
			j := uint32(1)
			for j < numGlyphs {
				glyphSID, err := s.readU16()
				if err != nil {
					return err
				}
				var nleft uint32
				if cs.format == 2 {
					v, err := s.readU16()
					if err != nil {
						return err
					}
					nleft = uint32(v)
				} else {
					v, err := s.readByte()
					if err != nil {
						return err
					}
					nleft = uint32(v)
				}
				if int64(glyphSID) > 0xFFFF-int64(nleft) {
					nleft = uint32(0xFFFF - int64(glyphSID))
				}
				for i := uint32(0); j < numGlyphs && i <= nleft; i, j, glyphSID = i+1, j+1, glyphSID+1 {
					cs.sids[j] = glyphSID
				}
			}
		default:
			return errInvalidFormat
		}
	} else {
		cs.offset = offset
		var table []uint16
		switch offset {
		case 0:
			if numGlyphs > 229 {
				return errInvalidFormat
			}
			table = cffISOAdobeCharset[:]
		case 1:
			if numGlyphs > 166 {
				return errInvalidFormat
			}
			table = cffExpertCharset[:]
		case 2:
			if numGlyphs > 87 {
				return errInvalidFormat
			}
			table = cffExpertSubsetCharset[:]
		}
		cs.sids = make([]uint16, numGlyphs)
		copy(cs.sids, table[:numGlyphs])
	}
	if invert {
		cs.computeCIDs(numGlyphs)
	}
	return nil
}

// fdot14ToFixed is FT_fdot14ToFixed.
func fdot14ToFixed(x int64) int64 { return int64(uint64(x) << 2) }

// loadVStore is cff_vstore_load.
func loadVStore(vs *cffVStore, s *cffStream, baseOffset int64, offset uint64) (err error) {
	defer func() {
		if err != nil {
			*vs = cffVStore{}
		}
	}()
	if offset == 0 {
		return nil
	}
	if err := s.seek(baseOffset + int64(offset)); err != nil {
		return err
	}
	if err := s.skip(2); err != nil {
		return err
	}
	vsOffset := s.pos
	format, err := s.readU16()
	if err != nil {
		return err
	}
	if format != 1 {
		return errInvalidFormat
	}
	regionListOffset, err := s.readU32()
	if err != nil {
		return err
	}
	dataCount, err := s.readU16()
	if err != nil {
		return err
	}
	dataOffsets := make([]uint32, dataCount)
	for i := range dataOffsets {
		if dataOffsets[i], err = s.readU32(); err != nil {
			return err
		}
	}
	if err := s.seek(vsOffset + int64(regionListOffset)); err != nil {
		return err
	}
	if vs.axisCount, err = s.readU16(); err != nil {
		return err
	}
	regionCount, err := s.readU16()
	if err != nil {
		return err
	}
	for i := 0; i < int(regionCount); i++ {
		axes := make([]cffAxisCoords, vs.axisCount)
		vs.varRegionList = append(vs.varRegionList, axes)
		vs.regionCount++
		for j := range axes {
			var v [3]int64
			for k := range v {
				u, err := s.readU16()
				if err != nil {
					return err
				}
				v[k] = int64(int16(u))
			}
			start, peak, end := v[0], v[1], v[2]
			if (start < 0 && end > 0) || start > peak || peak > end {
				peak = 0
			}
			axes[j] = cffAxisCoords{fdot14ToFixed(start), fdot14ToFixed(peak), fdot14ToFixed(end)}
		}
	}
	for i := 0; i < int(dataCount); i++ {
		if err := s.seek(vsOffset + int64(dataOffsets[i])); err != nil {
			return err
		}
		if err := s.skip(4); err != nil {
			return err
		}
		n, err := s.readU16()
		if err != nil {
			return err
		}
		data := cffVarData{regionIndices: make([]uint16, n)}
		vs.varData = append(vs.varData, data)
		vs.dataCount++
		for j := range data.regionIndices {
			if data.regionIndices[j], err = s.readU16(); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildVector is cff_blend_build_vector.
func (b *cffBlend) buildVector(vsindex, lenNDV uint32, ndv []int64) error {
	if !(lenNDV == 0 || ndv != nil) {
		return errInvalidFormat
	}
	b.builtBV = false
	vs := &b.font.vstore
	if lenNDV != 0 && lenNDV != uint32(vs.axisCount) {
		return errInvalidFormat
	}
	if vsindex >= vs.dataCount {
		return errInvalidFormat
	}
	varData := &vs.varData[vsindex]
	length := uint32(len(varData.regionIndices)) + 1
	if uint32(len(b.bv)) < length {
		b.bv = append(b.bv, make([]int32, int(length)-len(b.bv))...)
	}
	b.lenBV = length
	for master := uint32(0); master < length; master++ {
		if master == 0 {
			b.bv[master] = 0x10000
			continue
		}
		idx := uint32(varData.regionIndices[master-1])
		if idx >= vs.regionCount {
			return errInvalidFormat
		}
		varRegion := vs.varRegionList[idx]
		if lenNDV == 0 {
			b.bv[master] = 0
			continue
		}
		b.bv[master] = 0x10000
		for j := uint32(0); j < lenNDV; j++ {
			axis := &varRegion[j]
			if axis.peakCoord == ndv[j] || axis.peakCoord == 0 {
				continue
			} else if ndv[j] <= axis.startCoord || ndv[j] >= axis.endCoord {
				b.bv[master] = 0
				break
			} else if ndv[j] < axis.peakCoord {
				b.bv[master] = int32(mulDiv(int64(b.bv[master]), ndv[j]-axis.startCoord,
					axis.peakCoord-axis.startCoord))
			} else {
				b.bv[master] = int32(mulDiv(int64(b.bv[master]), axis.endCoord-ndv[j],
					axis.endCoord-axis.peakCoord))
			}
		}
	}
	b.lastVsindex = vsindex
	if lenNDV != 0 {
		b.lastNDV = append([]int64(nil), ndv[:lenNDV]...)
	}
	b.lenNDV = lenNDV
	b.builtBV = true
	return nil
}

// checkVector is cff_blend_check_vector: whether the vector needs to be
// built.
func (b *cffBlend) checkVector(vsindex, lenNDV uint32, ndv []int64) bool {
	if !b.builtBV || b.lastVsindex != vsindex || b.lenNDV != lenNDV {
		return true
	}
	if lenNDV != 0 {
		for i := uint32(0); i < lenNDV; i++ {
			if ndv[i] != b.lastNDV[i] {
				return true
			}
		}
	}
	return false
}

// cffParser is a CFF_ParserRec. Its stack holds operands as slices of
// the DICT from the operand to the parser's limit (or of a blend
// result).
type cffParser struct {
	stack      [][]byte
	stackSize  int
	objectCode uint32
	top        *cffFontDict
	priv       *cffPrivate
	numDesigns uint16
	numAxes    uint16
}

// run is cff_parser_run.
func (ps *cffParser) run(dict []byte) error {
	ps.stack = ps.stack[:0]
	p := 0
	limit := len(dict)
	for p < limit {
		v := dict[p]
		if v >= 27 && v != 31 && v != 255 {
			if len(ps.stack) >= ps.stackSize {
				return errInvalidArg
			}
			ps.stack = append(ps.stack, dict[p:])
			switch {
			case v == 30:
				p++
				for {
					if p >= limit {
						return nil
					}
					if dict[p]>>4 == 15 || dict[p]&0xF == 15 {
						break
					}
					p++
				}
			case v == 28:
				p += 2
			case v == 29:
				p += 4
			case v > 246:
				p++
			}
		} else {
			if len(ps.stack) >= ps.stackSize {
				return errInvalidArg
			}
			code := uint32(v)
			if v == 12 {
				p++
				if p >= limit {
					return errInvalidArg
				}
				code = 0x100 | uint32(dict[p])
			}
			code |= ps.objectCode
			isBlend, err := ps.field(code)
			if err != nil {
				return err
			}
			if !isBlend {
				ps.stack = ps.stack[:0]
			}
		}
		p++
	}
	return nil
}

// field handles the operator code (the cff_field_handlers table); it
// reports whether the operator was blend, which keeps the stack.
func (ps *cffParser) field(code uint32) (bool, error) {
	numArgs := len(ps.stack)
	top, priv := ps.top, ps.priv

	num := func() int64 { return parseNum(ps.stack[0]) }
	// need is the table's argument check for a non-delta field.
	need := func() bool { return numArgs >= 1 }
	delta := func(arr []int64, count *byte, max int, fixed bool) {
		n := numArgs
		if n > max {
			n = max
		}
		*count = byte(n)
		var val int64
		for i := 0; i < n; i++ {
			if fixed {
				val += parseFixed(ps.stack[i])
			} else {
				val += parseNum(ps.stack[i])
			}
			arr[i] = val
		}
	}

	switch code {
	// Top DICT (CFF)
	case 0 | cffCodeTopDict, 1 | cffCodeTopDict, 0x100 | cffCodeTopDict, 2 | cffCodeTopDict,
		3 | cffCodeTopDict, 4 | cffCodeTopDict, 0x115 | cffCodeTopDict, 0x126 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		val := uint32(int32(num()))
		switch code &^ cffCodeTopDict {
		case 0:
			top.version = val
		case 1:
			top.notice = val
		case 0x100:
			top.copyright = val
		case 2:
			top.fullName = val
		case 3:
			top.familyName = val
		case 4:
			top.weight = val
		case 0x115:
			top.embeddedPostscript = val
		case 0x126:
			top.cidFontName = val
		}
	case 0x101 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.isFixedPitch = byte(num())
	case 0x102 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.italicAngle = parseFixed(ps.stack[0])
	case 0x103 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.underlinePosition = int16(num())
	case 0x104 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.underlineThickness = uint16(num())
	case 0x105 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.paintType = int32(num())
	case 0x106 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.charstringType = int32(num())
	case 0x107 | cffCodeTopDict, 0x107 | cff2CodeTopDict, 0x107 | cff2CodeFontDict:
		if !need() {
			return false, errInvalidArg
		}
		return false, ps.parseFontMatrix()
	case 13 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.uniqueID = num()
	case 5 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		if numArgs < 4 {
			return false, errStackUnderflow
		}
		top.fontBBox.XMin = roundFix(parseFixed(ps.stack[0]))
		top.fontBBox.YMin = roundFix(parseFixed(ps.stack[1]))
		top.fontBBox.XMax = roundFix(parseFixed(ps.stack[2]))
		top.fontBBox.YMax = roundFix(parseFixed(ps.stack[3]))
	case 0x108 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.strokeWidth = num()
	case 15 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.charsetOffset = uint64(num())
	case 16 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.encodingOffset = uint64(num())
	case 17 | cffCodeTopDict, 17 | cff2CodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.charstringsOffset = uint64(num())
	case 18 | cffCodeTopDict, 18 | cff2CodeFontDict:
		if !need() {
			return false, errInvalidArg
		}
		if numArgs < 2 {
			return false, errStackUnderflow
		}
		size := parseNum(ps.stack[0])
		if size < 0 {
			return false, errInvalidFormat
		}
		top.privateSize = uint64(size)
		offset := parseNum(ps.stack[1])
		if offset < 0 {
			return false, errInvalidFormat
		}
		top.privateOffset = uint64(offset)
	case 0x114 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.syntheticBase = num()
	case 0x118 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		if numArgs < 5 {
			return false, errStackUnderflow
		}
		numDesigns := parseNum(ps.stack[0])
		if numDesigns > 16 || numDesigns < 2 {
			return false, errInvalidFormat
		}
		top.numDesigns = uint16(numDesigns)
		top.numAxes = uint16(numArgs - 4)
		ps.numDesigns = top.numDesigns
		ps.numAxes = top.numAxes
	case 0x11E | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		if numArgs < 3 {
			return false, errStackUnderflow
		}
		top.cidRegistry = uint32(int32(parseNum(ps.stack[0])))
		top.cidOrdering = uint32(int32(parseNum(ps.stack[1])))
		top.cidSupplement = parseNum(ps.stack[2])
	case 0x11F | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.cidFontVersion = num()
	case 0x120 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.cidFontRevision = num()
	case 0x121 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.cidFontType = num()
	case 0x122 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.cidCount = uint64(num())
	case 0x123 | cffCodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.cidUIDBase = uint64(num())
	case 0x124 | cffCodeTopDict, 0x124 | cff2CodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.cidFDArrayOffset = uint64(num())
	case 0x125 | cffCodeTopDict, 0x125 | cff2CodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.cidFDSelectOffset = uint64(num())
	case 24 | cff2CodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.vstoreOffset = uint64(num())
	case 25 | cff2CodeTopDict:
		if !need() {
			return false, errInvalidArg
		}
		top.maxstack = uint32(int32(num()))
		if top.maxstack > cff2MaxStack {
			top.maxstack = cff2MaxStack
		}
		if top.maxstack < cff2DefaultStack {
			top.maxstack = cff2DefaultStack
		}

	// Private DICT (CFF and CFF2)
	case 6 | cffCodePrivate, 6 | cff2CodePrivate:
		delta(priv.blueValues[:], &priv.numBlueValues, 14, true)
	case 7 | cffCodePrivate, 7 | cff2CodePrivate:
		delta(priv.otherBlues[:], &priv.numOtherBlues, 10, true)
	case 8 | cffCodePrivate, 8 | cff2CodePrivate:
		delta(priv.familyBlues[:], &priv.numFamilyBlues, 14, true)
	case 9 | cffCodePrivate, 9 | cff2CodePrivate:
		delta(priv.familyOtherBlues[:], &priv.numFamilyOtherBlues, 10, true)
	case 0x109 | cffCodePrivate, 0x109 | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.blueScale = doFixed(ps.stack[0], 3)
	case 0x10A | cffCodePrivate, 0x10A | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.blueShift = num()
	case 0x10B | cffCodePrivate, 0x10B | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.blueFuzz = num()
	case 10 | cffCodePrivate, 10 | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.standardWidth = num()
	case 11 | cffCodePrivate, 11 | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.standardHeight = num()
	case 0x10C | cffCodePrivate, 0x10C | cff2CodePrivate:
		delta(priv.snapWidths[:], &priv.numSnapWidths, 13, false)
	case 0x10D | cffCodePrivate, 0x10D | cff2CodePrivate:
		delta(priv.snapHeights[:], &priv.numSnapHeights, 13, false)
	case 0x10E | cffCodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.forceBold = byte(num())
	case 0x10F | cffCodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.forceBoldThreshold = parseFixed(ps.stack[0])
	case 0x110 | cffCodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.lenIV = int32(num())
	case 0x111 | cffCodePrivate, 0x111 | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.languageGroup = int32(num())
	case 0x112 | cffCodePrivate, 0x112 | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.expansionFactor = parseFixed(ps.stack[0])
	case 0x113 | cffCodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.initialRandomSeed = num()
	case 19 | cffCodePrivate, 19 | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.localSubrsOffset = uint64(num())
	case 20 | cffCodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.defaultWidth = num()
	case 21 | cffCodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		priv.nominalWidth = num()
	case 22 | cff2CodePrivate:
		if !need() {
			return false, errInvalidArg
		}
		// cff_parse_vsindex
		if priv == nil || priv.subfont == nil {
			return false, errInvalidFormat
		}
		if priv.subfont.blend.usedBV {
			return false, errSyntax
		}
		priv.vsindex = uint32(int32(num()))
	case 23 | cff2CodePrivate:
		if !need() {
			return true, errInvalidArg
		}
		return true, ps.parseBlend()
	}
	return false, nil
}

// roundFix is FT_RoundFix.
func roundFix(a int64) int64 {
	var neg int64
	if a < 0 {
		neg = 1
	}
	return (a + 0x8000 - neg) &^ 0xFFFF
}

// parseFontMatrix is cff_parse_font_matrix.
func (ps *cffParser) parseFontMatrix() error {
	dict := ps.top
	if len(ps.stack) < 6 {
		return errStackUnderflow
	}
	var values [6]int64
	var scalings [6]int64
	dict.hasFontMatrix = true
	maxScaling := int64(minInt64)
	minScaling := int64(maxInt64)
	for i := 0; i < 6; i++ {
		values[i] = parseFixedDynamic(ps.stack[i], &scalings[i])
		if values[i] != 0 {
			if scalings[i] > maxScaling {
				maxScaling = scalings[i]
			}
			if scalings[i] < minScaling {
				minScaling = scalings[i]
			}
		}
	}

	unlikely := func() error {
		dict.fontMatrix = Matrix{0x10000, 0, 0, 0x10000}
		dict.fontOffset = Vector{}
		dict.unitsPerEm = 1
		return nil
	}
	// (max_scaling - min_scaling) wraps like C's signed longs.
	diff := int64(uint64(maxScaling) - uint64(minScaling))
	if maxScaling < -9 || maxScaling > 0 || diff < 0 || diff > 9 {
		return unlikely()
	}
	for i := 0; i < 6; i++ {
		value := values[i]
		if value == 0 {
			continue
		}
		divisor := powerTens[maxScaling-scalings[i]]
		halfDivisor := divisor >> 1
		if value < 0 {
			if minInt64+halfDivisor < value {
				values[i] = (value - halfDivisor) / divisor
			} else {
				values[i] = minInt64 / divisor
			}
		} else {
			if maxInt64-halfDivisor > value {
				values[i] = (value + halfDivisor) / divisor
			} else {
				values[i] = maxInt64 / divisor
			}
		}
	}
	dict.fontMatrix = Matrix{XX: values[0], YX: values[1], XY: values[2], YY: values[3]}
	dict.fontOffset = Vector{values[4], values[5]}
	dict.unitsPerEm = uint64(powerTens[-maxScaling])
	if !matrixCheck(dict.fontMatrix) {
		return unlikely()
	}
	return nil
}

// parseBlend is cff_parse_blend with cff_blend_doBlend.
func (ps *cffParser) parseBlend() error {
	priv := ps.priv
	if priv == nil || priv.subfont == nil {
		return errInvalidFormat
	}
	subFont := priv.subfont
	blend := &subFont.blend
	if blend.checkVector(priv.vsindex, subFont.lenNDV, subFont.ndv) {
		if err := blend.buildVector(priv.vsindex, subFont.lenNDV, subFont.ndv); err != nil {
			return err
		}
	}
	numBlends := uint32(int32(parseNum(ps.stack[len(ps.stack)-1])))
	if numBlends > uint32(ps.stackSize) {
		return errInvalidFormat
	}

	// cff_blend_doBlend
	numOperands := numBlends * blend.lenBV
	count := uint32(len(ps.stack) - 1)
	var err error
	if numOperands > count {
		err = errStackUnderflow
	} else {
		base := count - numOperands
		delta := base + numBlends
		for i := uint32(0); i < numBlends; i++ {
			sum := parseFixed(ps.stack[i+base])
			for j := uint32(1); j < blend.lenBV; j++ {
				sum += mulFix(parseFixed(ps.stack[delta]), int64(blend.bv[j]))
				delta++
			}
			u := uint32(sum)
			ps.stack[i+base] = []byte{255, byte(u >> 24), byte(u >> 16), byte(u >> 8), byte(u)}
		}
		ps.stack = ps.stack[:base+numBlends]
	}
	blend.usedBV = true
	return err
}

// Loading (cffload.c).

// defaultPrivateBlueScale and defaultExpansionFactor are the
// (FT_Fixed)( 0.039625 * 0x10000L * 1000 ) and
// (FT_Fixed)( 0.06 * 0x10000L ) defaults, computed in doubles.
var (
	blueScaleFactor        = 0.039625
	expansionFactorDefault = 0.06
)

// loadPrivateDict is cff_load_private_dict.
func (f *cffFont) loadPrivateDict(subfont *cffSubFont, lenNDV uint32, ndv []int64) error {
	top := &subfont.fontDict
	priv := &subfont.privateDict
	s := f.stream

	subfont.blend.font = f
	subfont.blend.usedBV = false

	if top.privateOffset == 0 || top.privateSize == 0 {
		return nil
	}
	*priv = cffPrivate{}
	priv.blueShift = 7
	priv.blueFuzz = 1
	priv.lenIV = -1
	priv.expansionFactor = int64(expansionFactorDefault * 0x10000)
	priv.blueScale = int64(blueScaleFactor * 0x10000 * 1000)

	priv.subfont = subfont
	subfont.lenNDV = lenNDV
	subfont.ndv = ndv

	stackSize := cffMaxStackDepth + 1
	code := uint32(cffCodePrivate)
	if f.cff2 {
		stackSize = int(f.topFont.fontDict.maxstack) + 1
		code = cff2CodePrivate
	}
	ps := &cffParser{stackSize: stackSize, objectCode: code, priv: priv,
		numDesigns: top.numDesigns, numAxes: top.numAxes}

	if err := s.seek(f.baseOffset + int64(top.privateOffset)); err != nil {
		return err
	}
	dict, err := s.frame(top.privateSize)
	if err != nil {
		return err
	}
	if err := ps.run(dict); err != nil {
		return err
	}

	priv.numBlueValues &^= 1
	if priv.initialRandomSeed < 0 {
		priv.initialRandomSeed = -priv.initialRandomSeed
	} else if priv.initialRandomSeed == 0 {
		priv.initialRandomSeed = 987654321
	}
	if priv.blueShift > 1000 || priv.blueShift < 0 {
		priv.blueShift = 7
	}
	if priv.blueFuzz > 1000 || priv.blueFuzz < 0 {
		priv.blueFuzz = 1
	}
	return nil
}

// driverRandomSeed is the CFF driver's random_seed, which C++ computes
// from memory addresses; only the charstring `random' operator uses it,
// so no font can depend on its value.
var driverRandomSeed int32 = 0x2A5D6B

// cffRandom is cff_random.
func cffRandom(r uint32) uint32 {
	r ^= r << 13
	r ^= r >> 17
	r ^= r << 5
	return r
}

// loadSubfont is cff_subfont_load.
func (f *cffFont) loadSubfont(subfont *cffSubFont, idx *cffIndex, fontIndex uint32,
	baseOffset int64, code uint32) error {
	s := f.stream
	top := &subfont.fontDict
	priv := &subfont.privateDict
	cff2 := code == cff2CodeTopDict || code == cff2CodeFontDict
	stackSize := cffMaxStackDepth
	if cff2 {
		stackSize = cff2DefaultStack
	}
	ps := &cffParser{stackSize: stackSize, objectCode: code, top: top}

	*top = cffFontDict{}
	top.underlinePosition = -100
	top.underlineThickness = 50
	top.charstringType = 2
	top.fontMatrix.XX = 0x10000
	top.fontMatrix.YY = 0x10000
	top.cidCount = 8720
	top.version = 0xFFFF
	top.notice = 0xFFFF
	top.copyright = 0xFFFF
	top.fullName = 0xFFFF
	top.familyName = 0xFFFF
	top.weight = 0xFFFF
	top.embeddedPostscript = 0xFFFF
	top.cidRegistry = 0xFFFF
	top.cidOrdering = 0xFFFF
	top.cidFontName = 0xFFFF
	top.maxstack = 48
	if cff2 {
		top.maxstack = cff2DefaultStack
	}

	var dict []byte
	var err error
	if idx.count != 0 {
		dict, err = idx.accessElement(s, fontIndex)
	} else {
		// CFF2's top DICT has a fake index.
		if err := s.seek(idx.dataOffset); err != nil {
			return err
		}
		if dict, err = s.frame(idx.dataSize); err != nil {
			return err
		}
	}
	if err == nil {
		err = ps.run(dict)
	}
	if err != nil {
		return err
	}

	if top.cidRegistry != 0xFFFF {
		return nil
	}
	if err := f.loadPrivateDict(subfont, 0, nil); err != nil {
		return err
	}
	if !cff2 {
		// the face's random_seed is -1, so the driver's seed is used
		subfont.random = uint32(driverRandomSeed)
		if driverRandomSeed != 0 {
			for {
				driverRandomSeed = int32(cffRandom(uint32(driverRandomSeed)))
				if driverRandomSeed >= 0 {
					break
				}
			}
		}
		if subfont.random == 0 {
			subfont.random = uint32(priv.initialRandomSeed)
		}
	}

	if priv.localSubrsOffset != 0 {
		if err := s.seek(baseOffset + int64(top.privateOffset) + int64(priv.localSubrsOffset)); err != nil {
			return err
		}
		subfont.localSubrsIndex, err = cffIndexInit(s, true, cff2)
		if err != nil {
			return err
		}
		if subfont.localSubrs, err = subfont.localSubrsIndex.getPointers(s); err != nil {
			return err
		}
	}
	return nil
}

// loadEncoding is cff_encoding_load.
func loadEncoding(enc *cffEncoding, cs *cffCharset, numGlyphs uint32, s *cffStream,
	baseOffset int64, offset uint64) error {
	if cs.sids == nil {
		return errInvalidFormat
	}
	if offset > 1 {
		enc.sids = [256]uint16{}
		enc.codes = [256]uint16{}
		enc.offset = uint64(baseOffset) + offset
		if err := s.seek(int64(enc.offset)); err != nil {
			return err
		}
		format, err := s.readByte()
		if err != nil {
			return err
		}
		enc.format = uint32(format)
		count, err := s.readByte()
		if err != nil {
			return err
		}
		switch enc.format & 0x7F {
		case 0:
			enc.count = uint32(count) + 1
			p, err := s.frame(uint64(count))
			if err != nil {
				return err
			}
			for j := uint32(1); j <= uint32(count); j++ {
				glyphCode := p[j-1]
				if j < numGlyphs {
					enc.codes[glyphCode] = uint16(j)
					enc.sids[glyphCode] = cs.sids[j]
				}
			}
		case 1:
			enc.count = 0
			i := uint32(1)
			var nleft uint32
			for j := uint32(0); j < uint32(count); j, i = j+1, i+nleft {
				gc, err := s.readByte()
				if err != nil {
					return err
				}
				n, err := s.readByte()
				if err != nil {
					return err
				}
				glyphCode := uint32(gc)
				nleft = uint32(n) + 1
				if nleft > enc.count {
					enc.count = nleft
				}
				for k := i; k < nleft+i; k, glyphCode = k+1, glyphCode+1 {
					if k < numGlyphs && glyphCode < 256 {
						enc.codes[glyphCode] = uint16(k)
						enc.sids[glyphCode] = cs.sids[k]
					}
				}
			}
			if enc.count > 256 {
				enc.count = 256
			}
		default:
			return errInvalidFormat
		}

		if enc.format&0x80 != 0 {
			count, err := s.readByte()
			if err != nil {
				return err
			}
			for j := 0; j < int(count); j++ {
				glyphCode, err := s.readByte()
				if err != nil {
					return err
				}
				glyphSID, err := s.readU16()
				if err != nil {
					return err
				}
				enc.sids[glyphCode] = glyphSID
				for gindex := uint32(0); gindex < numGlyphs; gindex++ {
					if cs.sids[gindex] == glyphSID {
						enc.codes[glyphCode] = uint16(gindex)
						break
					}
				}
			}
		}
		return nil
	}

	switch offset {
	case 0:
		enc.sids = cffStandardEncoding
	case 1:
		enc.sids = cffExpertEncoding
	}
	enc.offset = offset
	enc.count = 0
	cs.computeCIDs(numGlyphs)
	for j := uint32(0); j < 256; j++ {
		sid := uint32(enc.sids[j])
		var gid uint32
		if sid != 0 {
			gid = cs.cidToGindex(sid)
		}
		if gid != 0 {
			enc.codes[j] = uint16(gid)
			enc.count = j + 1
		} else {
			enc.codes[j] = 0
			enc.sids[j] = 0
		}
	}
	return nil
}

// loadCFFFont is cff_font_load, for the CFF (or CFF2) table at the
// stream's position.
func loadCFFFont(s *cffStream, faceIndex int, pureCFF, cff2 bool) (*cffFont, error) {
	f := &cffFont{stream: s, cff2: cff2, psnames: true}
	baseOffset := s.pos
	f.baseOffset = baseOffset
	dict := &f.topFont.fontDict

	hdr, err := s.frame(3)
	if err != nil {
		return nil, err
	}
	f.versionMajor, f.versionMinor, f.headerSize = hdr[0], hdr[1], hdr[2]
	if cff2 {
		if f.versionMajor != 2 || f.headerSize < 5 {
			return nil, errUnknownFormat
		}
		v, err := s.readU16()
		if err != nil {
			return nil, err
		}
		f.topDictLength = uint32(v)
	} else {
		absoluteOffset, err := s.readByte()
		if err != nil {
			return nil, err
		}
		if f.versionMajor != 1 || f.headerSize < 4 || absoluteOffset > 4 {
			return nil, errUnknownFormat
		}
	}

	if err := s.seek(baseOffset + int64(f.headerSize)); err != nil {
		if pureCFF {
			return nil, errUnknownFormat
		}
		return nil, err
	}

	var stringIndex *cffIndex
	if cff2 {
		f.fontDictIndex = &cffIndex{dataOffset: s.pos, dataSize: uint64(f.topDictLength)}
		if err := s.skip(int64(f.topDictLength)); err != nil {
			return nil, err
		}
		if f.globalSubrsIndex, err = cffIndexInit(s, true, cff2); err != nil {
			return nil, err
		}
		stringIndex = &cffIndex{}
	} else {
		if f.nameIndex, err = cffIndexInit(s, false, cff2); err != nil {
			if pureCFF {
				return nil, errUnknownFormat
			}
			return nil, err
		}
		if f.nameIndex.count > 1 && f.nameIndex.dataSize < uint64(f.nameIndex.count) {
			if pureCFF {
				return nil, errUnknownFormat
			}
			return nil, errInvalidFormat
		}
		if f.fontDictIndex, err = cffIndexInit(s, false, cff2); err != nil {
			return nil, err
		}
		if stringIndex, err = cffIndexInit(s, true, cff2); err != nil {
			return nil, err
		}
		if f.globalSubrsIndex, err = cffIndexInit(s, true, cff2); err != nil {
			return nil, err
		}
		if f.stringPool, f.strings, err = stringIndex.getStrings(s); err != nil {
			return nil, err
		}
		if f.nameIndex.count > f.fontDictIndex.count {
			return nil, errInvalidFormat
		}
	}
	f.numStrings = stringIndex.count

	var subfontIndex uint32
	if pureCFF {
		subfontIndex = uint32(faceIndex & 0xFFFF)
		if faceIndex > 0 && subfontIndex >= f.nameIndex.count {
			return nil, errInvalidArg
		}
		f.numFaces = f.nameIndex.count
	} else {
		if f.nameIndex != nil && f.nameIndex.count > 1 {
			return nil, errInvalidFormat
		}
	}

	code := uint32(cffCodeTopDict)
	if cff2 {
		code = cff2CodeTopDict
	}
	if err := f.loadSubfont(&f.topFont, f.fontDictIndex, subfontIndex, baseOffset, code); err != nil {
		return nil, err
	}

	if err := s.seek(baseOffset + int64(dict.charstringsOffset)); err != nil {
		return nil, err
	}
	if f.charstringsIndex, err = cffIndexInit(s, false, cff2); err != nil {
		return nil, err
	}

	if dict.cidRegistry != 0xFFFF || cff2 {
		if err := loadVStore(&f.vstore, s, baseOffset, dict.vstoreOffset); err != nil {
			return nil, err
		}
		if err := s.seek(baseOffset + int64(dict.cidFDArrayOffset)); err != nil {
			return nil, err
		}
		fdIndex, err := cffIndexInit(s, false, cff2)
		if err != nil {
			return nil, err
		}
		if fdIndex.count <= cffMaxCIDFonts {
			f.numSubfonts = fdIndex.count
			f.subfonts = make([]*cffSubFont, fdIndex.count)
			for i := range f.subfonts {
				f.subfonts[i] = &cffSubFont{}
			}
			code := uint32(cffCodeTopDict)
			if cff2 {
				code = cff2CodeFontDict
			}
			for i := uint32(0); i < fdIndex.count; i++ {
				if err := f.loadSubfont(f.subfonts[i], fdIndex, i, baseOffset, code); err != nil {
					return nil, err
				}
			}
			if !cff2 || fdIndex.count > 1 {
				if err := loadFDSelect(&f.fdSelect, f.charstringsIndex.count, s,
					baseOffset+int64(dict.cidFDSelectOffset)); err != nil {
					return nil, err
				}
			}
		}
	} else {
		f.numSubfonts = 0
	}

	if dict.charstringsOffset == 0 {
		return nil, errInvalidFormat
	}
	f.numGlyphs = f.charstringsIndex.count

	if f.globalSubrs, err = f.globalSubrsIndex.getPointers(s); err != nil {
		return nil, err
	}

	if !cff2 && f.numGlyphs > 0 {
		invert := dict.cidRegistry != 0xFFFF && pureCFF
		if err := loadCharset(&f.charset, f.numGlyphs, s, baseOffset, dict.charsetOffset, invert); err != nil {
			return nil, err
		}
		if dict.cidRegistry == 0xFFFF {
			if err := loadEncoding(&f.encoding, &f.charset, f.numGlyphs, s, baseOffset,
				dict.encodingOffset); err != nil {
				return nil, err
			}
		}
	}

	f.fontName, f.hasFontName = f.getName(subfontIndex)
	return f, nil
}

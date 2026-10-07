package ttf

import (
	"bytes"
	"errors"
)

// FreeType's Type 1 font loader (type1's t1load.c, with its keyword
// table t1tokens.h and the multiple-master parsing): the font and
// private dictionaries, the encoding, Subrs and CharStrings.

// psFontInfo is a PS_FontInfoRec; nil strings are NULL.
type psFontInfo struct {
	version, notice, fullName, familyName, weight *string
	italicAngle                                   int64
	isFixedPitch                                  byte
	underlinePosition                             int16
	underlineThickness                            uint16
}

// psFontExtra is a PS_FontExtraRec.
type psFontExtra struct{ fsType uint16 }

// psPrivate is a PS_PrivateRec.
type psPrivate struct {
	uniqueID, lenIV                                                   int32
	numBlueValues, numOtherBlues, numFamilyBlues, numFamilyOtherBlues byte
	blueValues                                                        [14]int16
	otherBlues                                                        [10]int16
	familyBlues                                                       [14]int16
	familyOtherBlues                                                  [10]int16
	blueScale                                                         int64
	blueShift, blueFuzz                                               int32
	standardWidth, standardHeight                                     [1]uint16
	numSnapWidths, numSnapHeights                                     byte
	forceBold                                                         byte
	snapWidths, snapHeights                                           [13]int16
	expansionFactor                                                   int64
	languageGroup, password                                           int64
	minFeature                                                        [2]int16
}

// t1BBox is an FT_BBox of 16.16 values, as the font dictionary has it.
type t1BBox struct{ xMin, yMin, xMax, yMax int64 }

// Encoding types.
const (
	t1EncodingNone = iota
	t1EncodingArray
	t1EncodingStandard
	t1EncodingISOLatin1
	t1EncodingExpert
)

// t1Encoding is a T1_EncodingRec.
type t1Encoding struct {
	numChars            int
	codeFirst, codeLast int
	charIndex           []uint16
	charName            []string
}

// psDesignMap is a PS_DesignMapRec.
type psDesignMap struct {
	numPoints    byte
	designPoints []int64
	blendPoints  []int64
}

// psBlend is a PS_BlendRec: index 0 of the per-design arrays is the
// font's own record.
type psBlend struct {
	numDesigns, numAxis    uint32
	axisNames              [4]*string
	designPos              [][]int64
	designMap              [4]psDesignMap
	weightVector           []int64
	defaultWeightVector    []int64
	fontInfos              []*psFontInfo
	privates               []*psPrivate
	bboxes                 []*t1BBox
	defaultDesignVector    [16]int64
	numDefaultDesignVector byte
}

// t1Font is a T1_FontRec with the T1_FaceRec fields the loader sets.
type t1Font struct {
	fontInfo    psFontInfo
	fontExtra   psFontExtra
	privateDict psPrivate
	fontName    *string

	encodingType int
	encoding     t1Encoding

	numSubrs  int32
	subrs     [][]byte        // nil: no Subrs
	subrsHash map[int32]int32 // ft_hash_num: FT_Int keys

	numGlyphs      int32
	glyphNames     []*string
	charstrings    [][]byte
	charstringsSet bool

	paintType, fontType byte
	fontMatrix          Matrix
	fontOffset          Vector
	fontBBox            t1BBox
	strokeWidth         int64

	unitsPerEM uint16 // the root face's, from the font matrix

	ndvIdx, cdvIdx int32
	lenBuildchar   uint32
	buildchar      []int64
	blend          *psBlend
}

// t1Loader is a T1_LoaderRec.
type t1Loader struct {
	parser              *t1Parser
	encodingTable       *psTable
	numChars            int
	subrs               *psTable
	subrsHash           map[int32]int32
	numSubrs            int32
	charstrings         *psTable
	glyphNames          *psTable
	swapTable           *psTable
	numGlyphs           int32
	keywordsEncountered int
}

const (
	t1PrivateSeen          = 1 << 0
	t1FontdirAfterPrivate  = 1 << 1
	t1FieldDictFontdict    = 1 << 0
	t1FieldDictPrivate     = 1 << 1
	t1MaxMMAxis            = 4
	t1MaxMMDesigns         = 16
	t1MaxMMMapPoints       = 20
	t1MaxTableElements     = 32
	t1TableExtend          = 5
	t1FieldTypeBool        = 1
	t1FieldTypeInteger     = 2
	t1FieldTypeFixed       = 3
	t1FieldTypeFixed1000   = 4
	t1FieldTypeString      = 5
	t1FieldTypeKey         = 6
	t1FieldTypeBBox        = 7
	t1FieldTypeMMBBox      = 8
	t1FieldTypeIntArray    = 9
	t1FieldTypeCallback    = 11
	t1LocationFontInfo     = 1
	t1LocationFontExtra    = 2
	t1LocationPrivate      = 3
	t1LocationBBox         = 4
	t1LocationFontDict     = 5
	t1LocationFace         = 6
	t1LocationBlend        = 7
	t1FieldLocationDefault = 0
)

// t1Field is a T1_FieldRec. Integer values are stored by setInt at an
// element index (the byte offset of a table's element), truncated to
// the field's C type; setCount stores a table's count as a byte.
type t1Field struct {
	ident    string
	location int
	typ      int
	dict     int
	arrayMax int
	setInt   func(obj any, elem int, v int64)
	setCount func(obj any, n byte)
	setStr   func(obj any, s string)
	setBBox  func(obj any, b t1BBox)
	reader   func(f *t1Font, l *t1Loader)
}

func fiStr(get func(*psFontInfo) **string) func(any, string) {
	return func(o any, s string) { *get(o.(*psFontInfo)) = &s }
}

func privTable(get func(*psPrivate) []int16, count func(*psPrivate) *byte) (
	func(any, int, int64), func(any, byte)) {
	set := func(o any, elem int, v int64) { get(o.(*psPrivate))[elem] = int16(v) }
	var setCount func(any, byte)
	if count != nil {
		setCount = func(o any, n byte) { *count(o.(*psPrivate)) = n }
	}
	return set, setCount
}

var t1Keywords []t1Field

func init() {
	fd := t1FieldDictFontdict
	pd := t1FieldDictPrivate
	blueValues, numBlueValues := privTable(func(p *psPrivate) []int16 { return p.blueValues[:] },
		func(p *psPrivate) *byte { return &p.numBlueValues })
	otherBlues, numOtherBlues := privTable(func(p *psPrivate) []int16 { return p.otherBlues[:] },
		func(p *psPrivate) *byte { return &p.numOtherBlues })
	familyBlues, numFamilyBlues := privTable(func(p *psPrivate) []int16 { return p.familyBlues[:] },
		func(p *psPrivate) *byte { return &p.numFamilyBlues })
	familyOtherBlues, numFamilyOtherBlues := privTable(
		func(p *psPrivate) []int16 { return p.familyOtherBlues[:] },
		func(p *psPrivate) *byte { return &p.numFamilyOtherBlues })
	snapWidths, numSnapWidths := privTable(func(p *psPrivate) []int16 { return p.snapWidths[:] },
		func(p *psPrivate) *byte { return &p.numSnapWidths })
	snapHeights, numSnapHeights := privTable(func(p *psPrivate) []int16 { return p.snapHeights[:] },
		func(p *psPrivate) *byte { return &p.numSnapHeights })
	minFeature, _ := privTable(func(p *psPrivate) []int16 { return p.minFeature[:] }, nil)

	t1Keywords = []t1Field{
		{ident: "version", location: t1LocationFontInfo, typ: t1FieldTypeString, dict: fd,
			setStr: fiStr(func(i *psFontInfo) **string { return &i.version })},
		{ident: "Notice", location: t1LocationFontInfo, typ: t1FieldTypeString, dict: fd,
			setStr: fiStr(func(i *psFontInfo) **string { return &i.notice })},
		{ident: "FullName", location: t1LocationFontInfo, typ: t1FieldTypeString, dict: fd,
			setStr: fiStr(func(i *psFontInfo) **string { return &i.fullName })},
		{ident: "FamilyName", location: t1LocationFontInfo, typ: t1FieldTypeString, dict: fd,
			setStr: fiStr(func(i *psFontInfo) **string { return &i.familyName })},
		{ident: "Weight", location: t1LocationFontInfo, typ: t1FieldTypeString, dict: fd,
			setStr: fiStr(func(i *psFontInfo) **string { return &i.weight })},
		{ident: "ItalicAngle", location: t1LocationFontInfo, typ: t1FieldTypeFixed, dict: fd,
			setInt: func(o any, _ int, v int64) { o.(*psFontInfo).italicAngle = v }},
		{ident: "isFixedPitch", location: t1LocationFontInfo, typ: t1FieldTypeBool, dict: fd,
			setInt: func(o any, _ int, v int64) { o.(*psFontInfo).isFixedPitch = byte(v) }},
		{ident: "UnderlinePosition", location: t1LocationFontInfo, typ: t1FieldTypeInteger, dict: fd,
			setInt: func(o any, _ int, v int64) { o.(*psFontInfo).underlinePosition = int16(v) }},
		{ident: "UnderlineThickness", location: t1LocationFontInfo, typ: t1FieldTypeInteger, dict: fd,
			setInt: func(o any, _ int, v int64) { o.(*psFontInfo).underlineThickness = uint16(v) }},

		{ident: "FSType", location: t1LocationFontExtra, typ: t1FieldTypeInteger, dict: fd,
			setInt: func(o any, _ int, v int64) { o.(*psFontExtra).fsType = uint16(v) }},

		{ident: "UniqueID", location: t1LocationPrivate, typ: t1FieldTypeInteger, dict: fd | pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).uniqueID = int32(v) }},
		{ident: "lenIV", location: t1LocationPrivate, typ: t1FieldTypeInteger, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).lenIV = int32(v) }},
		{ident: "LanguageGroup", location: t1LocationPrivate, typ: t1FieldTypeInteger, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).languageGroup = v }},
		{ident: "password", location: t1LocationPrivate, typ: t1FieldTypeInteger, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).password = v }},
		{ident: "BlueScale", location: t1LocationPrivate, typ: t1FieldTypeFixed1000, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).blueScale = v }},
		{ident: "BlueShift", location: t1LocationPrivate, typ: t1FieldTypeInteger, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).blueShift = int32(v) }},
		{ident: "BlueFuzz", location: t1LocationPrivate, typ: t1FieldTypeInteger, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).blueFuzz = int32(v) }},
		{ident: "BlueValues", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 14, setInt: blueValues, setCount: numBlueValues},
		{ident: "OtherBlues", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 10, setInt: otherBlues, setCount: numOtherBlues},
		{ident: "FamilyBlues", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 14, setInt: familyBlues, setCount: numFamilyBlues},
		{ident: "FamilyOtherBlues", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 10, setInt: familyOtherBlues, setCount: numFamilyOtherBlues},
		{ident: "StdHW", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 1, setInt: func(o any, e int, v int64) { o.(*psPrivate).standardWidth[e] = uint16(v) }},
		{ident: "StdVW", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 1, setInt: func(o any, e int, v int64) { o.(*psPrivate).standardHeight[e] = uint16(v) }},
		{ident: "MinFeature", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 2, setInt: minFeature},
		{ident: "StemSnapH", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 12, setInt: snapWidths, setCount: numSnapWidths},
		{ident: "StemSnapV", location: t1LocationPrivate, typ: t1FieldTypeIntArray, dict: pd,
			arrayMax: 12, setInt: snapHeights, setCount: numSnapHeights},
		{ident: "ExpansionFactor", location: t1LocationPrivate, typ: t1FieldTypeFixed, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).expansionFactor = v }},
		{ident: "ForceBold", location: t1LocationPrivate, typ: t1FieldTypeBool, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*psPrivate).forceBold = byte(v) }},

		{ident: "FontName", location: t1LocationFontDict, typ: t1FieldTypeKey, dict: fd,
			setStr: func(o any, s string) { o.(*t1Font).fontName = &s }},
		{ident: "PaintType", location: t1LocationFontDict, typ: t1FieldTypeInteger, dict: fd,
			setInt: func(o any, _ int, v int64) { o.(*t1Font).paintType = byte(v) }},
		{ident: "FontType", location: t1LocationFontDict, typ: t1FieldTypeInteger, dict: fd,
			setInt: func(o any, _ int, v int64) { o.(*t1Font).fontType = byte(v) }},
		{ident: "StrokeWidth", location: t1LocationFontDict, typ: t1FieldTypeFixed, dict: fd,
			setInt: func(o any, _ int, v int64) { o.(*t1Font).strokeWidth = v }},

		{ident: "FontBBox", location: t1LocationBBox, typ: t1FieldTypeBBox, dict: fd,
			setBBox: func(o any, b t1BBox) { *o.(*t1BBox) = b }},

		{ident: "NDV", location: t1LocationFace, typ: t1FieldTypeInteger, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*t1Font).ndvIdx = int32(v) }},
		{ident: "CDV", location: t1LocationFace, typ: t1FieldTypeInteger, dict: pd,
			setInt: func(o any, _ int, v int64) { o.(*t1Font).cdvIdx = int32(v) }},

		{ident: "DesignVector", location: t1LocationBlend, typ: t1FieldTypeIntArray, dict: fd,
			arrayMax: t1MaxMMDesigns,
			setInt:   func(o any, e int, v int64) { o.(*psBlend).defaultDesignVector[e] = v },
			setCount: func(o any, n byte) { o.(*psBlend).numDefaultDesignVector = n }},

		{ident: "FontMatrix", typ: t1FieldTypeCallback, dict: fd, reader: t1ParseFontMatrix},
		{ident: "Encoding", typ: t1FieldTypeCallback, dict: fd, reader: t1ParseEncoding},
		{ident: "Subrs", typ: t1FieldTypeCallback, dict: pd, reader: t1ParseSubrs},
		{ident: "CharStrings", typ: t1FieldTypeCallback, dict: pd, reader: t1ParseCharstrings},
		{ident: "Private", typ: t1FieldTypeCallback, dict: fd, reader: t1ParsePrivate},
		{ident: "BlendDesignPositions", typ: t1FieldTypeCallback, dict: fd,
			reader: t1ParseBlendDesignPositions},
		{ident: "BlendDesignMap", typ: t1FieldTypeCallback, dict: fd, reader: t1ParseBlendDesignMap},
		{ident: "BlendAxisTypes", typ: t1FieldTypeCallback, dict: fd, reader: t1ParseBlendAxisTypes},
		{ident: "WeightVector", typ: t1FieldTypeCallback, dict: fd, reader: t1ParseWeightVector},
		{ident: "BuildCharArray", typ: t1FieldTypeCallback, dict: pd, reader: t1ParseBuildchar},
	}
}

// psLoadField is ps_parser_load_field. objects[0] is the font's own
// record, objects[1:] the designs'.
func psLoadField(p *psParser, field *t1Field, typ int, elem int, objects []any, maxObjects int) error {
	tok := p.toToken()
	if tok.typ == t1TokenNone {
		return errInvalidFileFormat
	}
	count := 1
	idx := 0
	cur := tok.start
	limit := tok.limit
	b := p.base

	if typ == t1FieldTypeBBox {
		oldCur, oldLimit := p.cursor, p.limit
		p.cursor = tok.start + 1
		p.limit = tok.limit - 1
		tok2 := p.toToken()
		p.cursor, p.limit = oldCur, oldLimit
		if tok2.typ == t1TokenArray {
			typ = t1FieldTypeMMBBox
			if maxObjects == 0 {
				return errInvalidFileFormat
			}
			idx = 1
			cur++
			limit--
		}
	} else if tok.typ == t1TokenArray {
		count = maxObjects
		if maxObjects == 0 {
			return errInvalidFileFormat
		}
		idx = 1
		cur++
		limit--
	}

	for ; count > 0; count, idx = count-1, idx+1 {
		psSkipSpaces(b, &cur, limit)
		switch typ {
		case t1FieldTypeBool, t1FieldTypeFixed, t1FieldTypeFixed1000, t1FieldTypeInteger:
			var val int64
			switch typ {
			case t1FieldTypeBool:
				val = psToBool(b, &cur, limit)
			case t1FieldTypeFixed:
				val = psConvToFixed(b, &cur, limit, 0)
			case t1FieldTypeFixed1000:
				val = psConvToFixed(b, &cur, limit, 3)
			default:
				val = psConvToInt(b, &cur, limit)
			}
			field.setInt(objects[idx], elem, val)

		case t1FieldTypeString, t1FieldTypeKey:
			if cur >= limit {
				break
			}
			n := limit - cur
			if tok.typ == t1TokenKey {
				n--
				cur++
			} else if tok.typ == t1TokenString {
				cur++
				n -= 2
			} else {
				return errInvalidFileFormat
			}
			if n < 0 {
				// FT_UInt wrapped: the allocation fails
				return errInvalidArg
			}
			// FT_QALLOC of len+1 bytes, copied and NUL-terminated
			s := string(b[cur : cur+n])
			field.setStr(objects[idx], cstr([]byte(s)))

		case t1FieldTypeBBox:
			var temp [4]int64
			if psToFixedArray(b, &cur, limit, 4, temp[:], 0) < 4 {
				return errInvalidFileFormat
			}
			field.setBBox(objects[idx], t1BBox{roundFix(temp[0]), roundFix(temp[1]),
				roundFix(temp[2]), roundFix(temp[3])})

		case t1FieldTypeMMBBox:
			temp := make([]int64, maxObjects*4)
			for i := range 4 {
				result := psToFixedArray(b, &cur, limit, maxObjects, temp[i*maxObjects:], 0)
				if result < 0 || result < maxObjects {
					return errInvalidFileFormat
				}
				psSkipSpaces(b, &cur, limit)
			}
			for i := range maxObjects {
				field.setBBox(objects[i], t1BBox{roundFix(temp[i]), roundFix(temp[i+maxObjects]),
					roundFix(temp[i+2*maxObjects]), roundFix(temp[i+3*maxObjects])})
			}

		default:
			return errInvalidFileFormat
		}
	}
	return nil
}

// psLoadFieldTable is ps_parser_load_field_table.
func psLoadFieldTable(p *psParser, field *t1Field, objects []any, maxObjects int) error {
	elements, numElements := p.toTokenArray(t1MaxTableElements)
	if numElements < 0 {
		return errIgnore
	}
	if numElements > field.arrayMax {
		numElements = field.arrayMax
	}
	// only T1_MAX_TABLE_ELEMENTS tokens were kept
	if numElements > len(elements) {
		numElements = len(elements)
	}
	oldCursor, oldLimit := p.cursor, p.limit
	if field.setCount != nil {
		field.setCount(objects[0], byte(numElements))
	}
	var err error
	for i := 0; i < numElements; i++ {
		p.cursor = elements[i].start
		p.limit = elements[i].limit
		if err = psLoadField(p, field, t1FieldTypeInteger, i, objects, maxObjects); err != nil {
			break
		}
	}
	p.cursor, p.limit = oldCursor, oldLimit
	return err
}

// t1LoadKeyword is t1_load_keyword.
func t1LoadKeyword(f *t1Font, l *t1Loader, field *t1Field) error {
	if field.typ == t1FieldTypeCallback {
		field.reader(f, l)
		return l.parser.root.err
	}
	blend := f.blend
	if blend != nil && blend.numDesigns == 0 {
		blend = nil
	}
	var objects []any
	maxObjects := 0
	switch field.location {
	case t1LocationFontInfo:
		objects = []any{&f.fontInfo}
		if blend != nil {
			objects = make([]any, len(blend.fontInfos))
			for i, o := range blend.fontInfos {
				objects[i] = o
			}
			maxObjects = int(blend.numDesigns)
		}
	case t1LocationFontExtra:
		objects = []any{&f.fontExtra}
	case t1LocationPrivate:
		objects = []any{&f.privateDict}
		if blend != nil {
			objects = make([]any, len(blend.privates))
			for i, o := range blend.privates {
				objects[i] = o
			}
			maxObjects = int(blend.numDesigns)
		}
	case t1LocationBBox:
		objects = []any{&f.fontBBox}
		if blend != nil {
			objects = make([]any, len(blend.bboxes))
			for i, o := range blend.bboxes {
				objects[i] = o
			}
			maxObjects = int(blend.numDesigns)
		}
	case t1LocationFace, t1LocationFontDict:
		objects = []any{f}
	case t1LocationBlend:
		if f.blend == nil {
			// not valid at this point
			return nil
		}
		objects = []any{f.blend}
	}
	if field.typ == t1FieldTypeIntArray {
		return psLoadFieldTable(&l.parser.root, field, objects, maxObjects)
	}
	return psLoadField(&l.parser.root, field, field.typ, 0, objects, maxObjects)
}

func t1ParsePrivate(f *t1Font, l *t1Loader) { l.keywordsEncountered |= t1PrivateSeen }

// readBinaryData is read_binary_data: the data after `RD' (or `-|').
func readBinaryData(parser *t1Parser) ([]byte, bool) {
	p := &parser.root
	p.skipSpaces()
	cur := p.cursor
	if cur < p.limit && isPSDigit(p.base[cur]) {
		s := p.toInt()
		p.skipPSToken()
		base := p.cursor + 1
		if s >= 0 && s < int64(p.limit-base) {
			p.cursor += int(s) + 1
			return p.base[base : base+int(s)], p.err == nil
		}
	}
	p.err = errInvalidFileFormat
	return nil, false
}

// t1ParseFontMatrix is t1_parse_font_matrix.
func t1ParseFontMatrix(f *t1Font, l *t1Loader) {
	p := &l.parser.root
	var temp [6]int64
	if p.toFixedArray(6, temp[:], 3) < 6 {
		p.err = errInvalidFileFormat
		return
	}
	tempScale := abs64(temp[3])
	if tempScale == 0 {
		p.err = errInvalidFileFormat
		return
	}
	if tempScale != 0x10000 {
		f.unitsPerEM = uint16(divFix(1000, tempScale))
		temp[0] = divFix(temp[0], tempScale)
		temp[1] = divFix(temp[1], tempScale)
		temp[2] = divFix(temp[2], tempScale)
		temp[4] = divFix(temp[4], tempScale)
		temp[5] = divFix(temp[5], tempScale)
		if temp[3] < 0 {
			temp[3] = -0x10000
		} else {
			temp[3] = 0x10000
		}
	}
	m := Matrix{XX: temp[0], YX: temp[1], XY: temp[2], YY: temp[3]}
	f.fontMatrix = m
	if !matrixCheck(m) {
		p.err = errInvalidFileFormat
		return
	}
	f.fontOffset = Vector{temp[4] >> 16, temp[5] >> 16}
}

// t1ParseEncoding is parse_encoding.
func t1ParseEncoding(f *t1Font, l *t1Loader) {
	p := &l.parser.root
	b := p.base
	limit := p.limit
	p.skipSpaces()
	cur := p.cursor
	if cur >= limit {
		p.err = errInvalidFileFormat
		return
	}
	if isPSDigit(b[cur]) || b[cur] == '[' {
		var count int
		onlyImmediates := false
		if b[cur] == '[' {
			count = 256
			onlyImmediates = true
			p.cursor++
		} else {
			count = int(int32(p.toInt()))
		}
		arraySize := min(count, 256)
		p.skipSpaces()
		if p.cursor >= limit {
			return
		}
		if arraySize < 0 {
			// FT_QNEW_ARRAY with a negative count fails
			p.err = errInvalidArg
			return
		}
		l.numChars = arraySize
		f.encoding.numChars = arraySize
		f.encoding.charIndex = make([]uint16, arraySize)
		f.encoding.charName = make([]string, arraySize)
		table := newPSTable(arraySize)
		l.encodingTable = table
		for n := 0; n < arraySize; n++ {
			_ = table.add(n, []byte(".notdef\x00")) // (void) in FreeType
		}

		n := 0
		p.skipSpaces()
		for p.cursor < limit {
			cur = p.cursor
			if b[cur] == 'd' && cur+3 < limit {
				if b[cur+1] == 'e' && b[cur+2] == 'f' && isPSDelim(b[cur+3]) {
					cur += 3
					break
				}
			}
			if b[cur] == ']' {
				cur++
				break
			}
			if isPSDigit(b[cur]) || onlyImmediates {
				var charcode int
				if onlyImmediates {
					charcode = n
				} else {
					charcode = int(int32(p.toInt()))
					p.skipSpaces()
					if cur == p.cursor {
						p.err = errUnknownFormat
						return
					}
				}
				cur = p.cursor
				if cur+2 < limit && b[cur] == '/' && n < count {
					cur++
					p.cursor = cur
					p.skipPSToken()
					if p.cursor >= limit {
						return
					}
					if p.err != nil {
						return
					}
					length := p.cursor - cur
					if n < arraySize {
						obj := append([]byte(nil), b[cur:cur+length+1]...)
						obj[length] = 0
						if err := table.add(charcode, obj); err != nil {
							p.err = err
							return
						}
					}
					n++
				} else if onlyImmediates {
					p.err = errUnknownFormat
					return
				}
			} else {
				p.skipPSToken()
				if p.err != nil {
					return
				}
			}
			p.skipSpaces()
		}
		f.encodingType = t1EncodingArray
		p.cursor = cur
	} else {
		switch {
		case cur+17 < limit && bytes.HasPrefix(b[cur:], []byte("StandardEncoding")):
			f.encodingType = t1EncodingStandard
		case cur+15 < limit && bytes.HasPrefix(b[cur:], []byte("ExpertEncoding")):
			f.encodingType = t1EncodingExpert
		case cur+18 < limit && bytes.HasPrefix(b[cur:], []byte("ISOLatin1Encoding")):
			f.encodingType = t1EncodingISOLatin1
		default:
			p.err = errIgnore
		}
	}
}

// t1ParseSubrs is parse_subrs.
func t1ParseSubrs(f *t1Font, l *t1Loader) {
	p := &l.parser.root
	p.skipSpaces()
	if p.cursor < p.limit && p.base[p.cursor] == '[' {
		p.skipPSToken()
		p.skipSpaces()
		if p.cursor >= p.limit || p.base[p.cursor] != ']' {
			p.err = errInvalidFileFormat
		}
		return
	}
	numSubrs := int32(p.toInt())
	if numSubrs < 0 {
		p.err = errInvalidFileFormat
		return
	}
	if p.limit >= p.cursor && int64(numSubrs) > int64((p.limit-p.cursor)>>3) {
		numSubrs = int32((p.limit - p.cursor) >> 3)
		if l.subrsHash == nil {
			l.subrsHash = map[int32]int32{}
		}
	}
	p.skipPSToken()
	if p.err != nil {
		return
	}
	p.skipSpaces()
	if l.numSubrs == 0 {
		l.subrs = newPSTable(int(numSubrs))
	}
	for count := int64(0); ; count++ {
		if p.cursor+4 >= p.limit || !bytes.HasPrefix(p.base[p.cursor:], []byte("dup")) {
			break
		}
		p.skipPSToken()
		idx := p.toInt()
		data, ok := readBinaryData(l.parser)
		if !ok {
			return
		}
		p.skipPSToken()
		if p.err != nil {
			return
		}
		p.skipSpaces()
		if p.cursor+4 < p.limit && bytes.HasPrefix(p.base[p.cursor:], []byte("put")) {
			p.skipPSToken()
			p.skipSpaces()
		}
		if l.subrsHash != nil {
			l.subrsHash[int32(idx)] = int32(count)
			idx = count
		}
		if l.numSubrs != 0 {
			continue
		}
		var err error
		if lenIV := f.privateDict.lenIV; lenIV >= 0 {
			if len(data) < int(lenIV) {
				p.err = errInvalidFileFormat
				return
			}
			temp := append([]byte(nil), data...)
			t1Decrypt(temp, 4330)
			err = l.subrs.add(int(int32(idx)), temp[lenIV:])
		} else {
			err = l.subrs.add(int(int32(idx)), data)
		}
		if err != nil {
			p.err = err
			return
		}
	}
	if l.numSubrs == 0 {
		l.numSubrs = numSubrs
	}
}

// t1ParseCharstrings is parse_charstrings.
func t1ParseCharstrings(f *t1Font, l *t1Loader) {
	p := &l.parser.root
	b := p.base
	cur := p.cursor
	limit := p.limit
	notdefIndex := 0
	notdefFound := false
	fail := func(err error) { p.err = err }

	numGlyphs := int32(p.toInt())
	if numGlyphs < 0 {
		fail(errInvalidFileFormat)
		return
	}
	if int64(numGlyphs) > int64((limit-cur)>>3) {
		numGlyphs = int32((limit - cur) >> 3)
	}
	if numGlyphs == 0 || p.err != nil {
		return
	}
	if l.numGlyphs == 0 {
		l.charstrings = newPSTable(int(numGlyphs) + 1 + t1TableExtend)
		l.glyphNames = newPSTable(int(numGlyphs) + 1 + t1TableExtend)
		l.swapTable = newPSTable(4)
	}
	codeTable, nameTable, swapTable := l.charstrings, l.glyphNames, l.swapTable

	n := 0
	for {
		p.skipSpaces()
		cur = p.cursor
		if cur >= limit {
			break
		}
		if cur+3 < limit && isPSDelim(b[cur+3]) {
			if b[cur] == 'd' && b[cur+1] == 'e' && b[cur+2] == 'f' {
				if n != 0 {
					break
				}
			}
			if b[cur] == 'e' && b[cur+1] == 'n' && b[cur+2] == 'd' {
				break
			}
		}
		p.skipPSToken()
		if p.cursor >= limit {
			fail(errInvalidFileFormat)
			return
		}
		if p.err != nil {
			return
		}
		if b[cur] == '/' {
			if cur+2 >= limit {
				fail(errInvalidFileFormat)
				return
			}
			cur++
			length := p.cursor - cur
			data, ok := readBinaryData(l.parser)
			if !ok {
				return
			}
			if l.numGlyphs != 0 {
				continue
			}
			name := append([]byte(nil), b[cur:cur+length+1]...)
			name[length] = 0
			if err := nameTable.add(n, name); err != nil {
				fail(err)
				return
			}
			if b[cur] == '.' && cstr(nameTable.elements[n]) == ".notdef" {
				notdefIndex = n
				notdefFound = true
			}
			var err error
			if lenIV := f.privateDict.lenIV; lenIV >= 0 && n < int(numGlyphs)+t1TableExtend {
				if len(data) <= int(lenIV) {
					fail(errInvalidFileFormat)
					return
				}
				temp := append([]byte(nil), data...)
				t1Decrypt(temp, 4330)
				err = codeTable.add(n, temp[lenIV:])
			} else {
				err = codeTable.add(n, data)
			}
			if err != nil {
				fail(err)
				return
			}
			n++
		}
	}
	if n == 0 {
		fail(errInvalidFileFormat)
		return
	}
	l.numGlyphs = int32(n)

	elem := func(t *psTable, i int) []byte { return t.elements[i][:t.lengths[i]] }
	if notdefFound && cstr(nameTable.elements[0]) != ".notdef" {
		steps := []struct {
			dst *psTable
			di  int
			src *psTable
			si  int
		}{
			{swapTable, 0, nameTable, 0},
			{swapTable, 1, codeTable, 0},
			{swapTable, 2, nameTable, notdefIndex},
			{swapTable, 3, codeTable, notdefIndex},
			{nameTable, notdefIndex, swapTable, 0},
			{codeTable, notdefIndex, swapTable, 1},
			{nameTable, 0, swapTable, 2},
			{codeTable, 0, swapTable, 3},
		}
		for _, s := range steps {
			if err := s.dst.add(s.di, elem(s.src, s.si)); err != nil {
				fail(err)
				return
			}
		}
	} else if !notdefFound {
		notdefGlyph := []byte{0x8B, 0xF7, 0xE1, 0x0D, 0x0E}
		steps := []func() error{
			func() error { return swapTable.add(0, elem(nameTable, 0)) },
			func() error { return swapTable.add(1, elem(codeTable, 0)) },
			func() error { return nameTable.add(0, []byte(".notdef\x00")) },
			func() error { return codeTable.add(0, notdefGlyph) },
			func() error { return nameTable.add(n, elem(swapTable, 0)) },
			func() error { return codeTable.add(n, elem(swapTable, 1)) },
		}
		for _, s := range steps {
			if err := s(); err != nil {
				fail(err)
				return
			}
		}
		l.numGlyphs++
	}
}

// t1AllocateBlend is t1_allocate_blend.
func t1AllocateBlend(f *t1Font, numDesigns, numAxis uint32) error {
	blend := f.blend
	if blend == nil {
		blend = &psBlend{}
		f.blend = blend
	}
	if numDesigns > 0 {
		if blend.numDesigns == 0 {
			blend.fontInfos = make([]*psFontInfo, numDesigns+1)
			blend.privates = make([]*psPrivate, numDesigns+1)
			blend.bboxes = make([]*t1BBox, numDesigns+1)
			blend.fontInfos[0] = &f.fontInfo
			blend.privates[0] = &f.privateDict
			blend.bboxes[0] = &f.fontBBox
			for nn := uint32(1); nn <= numDesigns; nn++ {
				blend.fontInfos[nn] = &psFontInfo{}
				blend.privates[nn] = &psPrivate{}
				blend.bboxes[nn] = &t1BBox{}
			}
			blend.numDesigns = numDesigns
		} else if blend.numDesigns != numDesigns {
			return errInvalidFileFormat
		}
	}
	if numAxis > 0 {
		if blend.numAxis != 0 && blend.numAxis != numAxis {
			return errInvalidFileFormat
		}
		blend.numAxis = numAxis
	}
	return nil
}

// t1ParseBlendAxisTypes is parse_blend_axis_types.
func t1ParseBlendAxisTypes(f *t1Font, l *t1Loader) {
	p := &l.parser.root
	tokens, numAxis := p.toTokenArray(t1MaxMMAxis)
	err := func() error {
		if numAxis < 0 {
			return errIgnore
		}
		if numAxis == 0 || numAxis > t1MaxMMAxis {
			return errInvalidFileFormat
		}
		if err := t1AllocateBlend(f, 0, uint32(numAxis)); err != nil {
			return err
		}
		for n := range numAxis {
			tok := tokens[n]
			if p.base[tok.start] == '/' {
				tok.start++
			}
			if tok.limit-tok.start == 0 {
				return errInvalidFileFormat
			}
			name := string(p.base[tok.start:tok.limit])
			f.blend.axisNames[n] = &name
		}
		return nil
	}()
	p.err = err
}

// t1ParseBlendDesignPositions is parse_blend_design_positions.
func t1ParseBlendDesignPositions(f *t1Font, l *t1Loader) {
	p := &l.parser.root
	designTokens, numDesigns := p.toTokenArray(t1MaxMMDesigns)
	err := func() error {
		if numDesigns < 0 {
			return errIgnore
		}
		if numDesigns == 0 || numDesigns > t1MaxMMDesigns {
			return errInvalidFileFormat
		}
		oldCursor, oldLimit := p.cursor, p.limit
		numAxis := 0
		designPos := make([][]int64, numDesigns)
		for n := range numDesigns {
			tok := designTokens[n]
			p.cursor = tok.start
			p.limit = tok.limit
			axisTokens, nAxis := p.toTokenArray(t1MaxMMAxis)
			if n == 0 {
				if nAxis <= 0 || nAxis > t1MaxMMAxis {
					return errInvalidFileFormat
				}
				numAxis = nAxis
				if err := t1AllocateBlend(f, uint32(numDesigns), uint32(numAxis)); err != nil {
					return err
				}
				for nn := range designPos {
					designPos[nn] = make([]int64, numAxis)
				}
			} else if nAxis != numAxis {
				return errInvalidFileFormat
			}
			for axis := range nAxis {
				tok2 := axisTokens[axis]
				p.cursor = tok2.start
				p.limit = tok2.limit
				designPos[n][axis] = p.toFixed(0)
			}
		}
		p.cursor, p.limit = oldCursor, oldLimit
		f.blend.designPos = designPos
		return nil
	}()
	p.err = err
}

// t1ParseBlendDesignMap is parse_blend_design_map.
func t1ParseBlendDesignMap(f *t1Font, l *t1Loader) {
	p := &l.parser.root
	axisTokens, numAxis := p.toTokenArray(t1MaxMMAxis)
	err := func() error {
		if numAxis < 0 {
			return errIgnore
		}
		if numAxis == 0 || numAxis > t1MaxMMAxis {
			return errInvalidFileFormat
		}
		oldCursor, oldLimit := p.cursor, p.limit
		if err := t1AllocateBlend(f, 0, uint32(numAxis)); err != nil {
			return err
		}
		blend := f.blend
		for n := range numAxis {
			m := &blend.designMap[n]
			tok := axisTokens[n]
			p.cursor = tok.start
			p.limit = tok.limit
			pointTokens, numPoints := p.toTokenArray(t1MaxMMMapPoints)
			if numPoints <= 0 || numPoints > t1MaxMMMapPoints {
				return errInvalidFileFormat
			}
			if m.designPoints != nil {
				return errInvalidFileFormat
			}
			m.designPoints = make([]int64, numPoints)
			m.blendPoints = make([]int64, numPoints)
			m.numPoints = byte(numPoints)
			for q := range numPoints {
				pt := pointTokens[q]
				p.cursor = pt.start + 1
				p.limit = pt.limit - 1
				m.designPoints[q] = p.toInt()
				m.blendPoints[q] = p.toFixed(0)
			}
		}
		p.cursor, p.limit = oldCursor, oldLimit
		return nil
	}()
	p.err = err
}

// t1ParseWeightVector is parse_weight_vector.
func t1ParseWeightVector(f *t1Font, l *t1Loader) {
	p := &l.parser.root
	designTokens, numDesigns := p.toTokenArray(t1MaxMMDesigns)
	err := func() error {
		if numDesigns < 0 {
			return errIgnore
		}
		if numDesigns == 0 || numDesigns > t1MaxMMDesigns {
			return errInvalidFileFormat
		}
		blend := f.blend
		if blend == nil || blend.numDesigns == 0 {
			if err := t1AllocateBlend(f, uint32(numDesigns), 0); err != nil {
				return err
			}
			blend = f.blend
		} else if blend.numDesigns != uint32(numDesigns) {
			return errInvalidFileFormat
		}
		if blend.weightVector == nil {
			wv := make([]int64, numDesigns*2)
			blend.weightVector = wv[:numDesigns]
			blend.defaultWeightVector = wv[numDesigns:]
		}
		oldCursor, oldLimit := p.cursor, p.limit
		for n := range numDesigns {
			tok := designTokens[n]
			p.cursor = tok.start
			p.limit = tok.limit
			v := p.toFixed(0)
			blend.defaultWeightVector[n] = v
			blend.weightVector[n] = v
		}
		p.cursor, p.limit = oldCursor, oldLimit
		return nil
	}()
	p.err = err
}

// t1ParseBuildchar is parse_buildchar.
func t1ParseBuildchar(f *t1Font, l *t1Loader) {
	f.lenBuildchar = uint32(l.parser.root.toFixedArray(0, nil, 0))
}

// isPSToken is IS_PS_TOKEN.
func isPSToken(b []byte, cur, limit int, token string) bool {
	n := len(token) + 1 // sizeof, with the NUL
	if b[cur] != token[0] {
		return false
	}
	if !(cur+n == limit || (cur+n < limit && isPSDelim(b[cur+n-1]))) {
		return false
	}
	return bytes.HasPrefix(b[cur:limit], []byte(token))
}

// t1ParseDict is parse_dict.
func t1ParseDict(f *t1Font, l *t1Loader, base []byte) error {
	p := &l.parser.root
	p.base = base
	p.cursor = 0
	p.limit = len(base)
	p.err = nil
	b := base
	limit := p.limit
	startBinary := 0
	haveInteger := false

	p.skipSpaces()
	for p.cursor < limit {
		cur := p.cursor
		switch {
		case isPSToken(b, cur, limit, "eexec"), isPSToken(b, cur, limit, "closefile"):
			return p.err
		case isPSToken(b, cur, limit, "FontDirectory"):
			if l.keywordsEncountered&t1PrivateSeen != 0 {
				l.keywordsEncountered |= t1FontdirAfterPrivate
			}
			p.cursor += 13
		case isPSDigit(b[cur]):
			startBinary = cur
			p.skipPSToken()
			if p.err != nil {
				return p.err
			}
			haveInteger = true
		case b[cur] == 'R' && cur+6 < limit && b[cur+1] == 'D' && haveInteger,
			b[cur] == '-' && cur+6 < limit && b[cur+1] == '|' && haveInteger:
			p.cursor = startBinary
			if _, ok := readBinaryData(l.parser); !ok {
				return errInvalidFileFormat
			}
			haveInteger = false
		case b[cur] == '/' && cur+2 < limit:
			cur++
			p.cursor = cur
			p.skipPSToken()
			if p.err != nil {
				return p.err
			}
			n := p.cursor - cur
			if n > 0 && n < 22 && p.cursor < limit {
				name := string(b[cur : cur+n])
				for i := range t1Keywords {
					kw := &t1Keywords[i]
					if kw.ident != name {
						continue
					}
					dict := t1FieldDictFontdict
					if l.keywordsEncountered&t1PrivateSeen != 0 {
						dict = t1FieldDictPrivate
					}
					if dict&kw.dict == 0 {
						break
					}
					if l.keywordsEncountered&t1FontdirAfterPrivate == 0 || kw.ident == "CharStrings" {
						p.err = t1LoadKeyword(f, l, kw)
						if p.err != nil {
							if errors.Is(p.err, errIgnore) {
								p.err = nil
							} else {
								return p.err
							}
						}
					}
					break
				}
			}
			haveInteger = false
		default:
			p.skipPSToken()
			if p.err != nil {
				return p.err
			}
			haveInteger = false
		}
		p.skipSpaces()
	}
	return p.err
}

// t1DoneBlend is T1_Done_Blend.
func (f *t1Font) doneBlend() { f.blend = nil }

// t1OpenFace is T1_Open_Face.
func t1OpenFace(data []byte) (*t1Font, error) {
	f := &t1Font{ndvIdx: -1, cdvIdx: -1}
	priv := &f.privateDict
	priv.blueShift = 7
	priv.blueFuzz = 1
	priv.lenIV = 4
	// C evaluates these in double precision and truncates
	d006, d039625 := 0.06, 0.039625
	priv.expansionFactor = int64(d006 * 0x10000)
	priv.blueScale = int64(d039625 * 0x10000 * 1000)

	parser, err := newT1Parser(data)
	if err != nil {
		return nil, err
	}
	l := &t1Loader{parser: parser}
	if err := t1ParseDict(f, l, parser.baseDict); err != nil {
		return nil, err
	}
	if err := parser.getPrivateDict(); err != nil {
		return nil, err
	}
	if err := t1ParseDict(f, l, parser.privateDict); err != nil {
		return nil, err
	}
	priv.numBlueValues &^= 1

	if blend := f.blend; blend != nil && blend.numDesigns != 1<<blend.numAxis {
		f.doneBlend()
	}
	if blend := f.blend; blend != nil && blend.numDefaultDesignVector != 0 &&
		uint32(blend.numDefaultDesignVector) != blend.numAxis {
		blend.numDefaultDesignVector = 0
	}
	if blend := f.blend; blend != nil && (blend.numDesigns == 0 || blend.numAxis == 0) {
		f.doneBlend()
	}
	if blend := f.blend; blend != nil && blend.weightVector == nil {
		f.doneBlend()
	}
	if blend := f.blend; blend != nil && blend.designPos == nil {
		f.doneBlend()
	}
	if blend := f.blend; blend != nil {
		for i := uint32(0); i < blend.numAxis; i++ {
			if blend.designMap[i].numPoints == 0 {
				f.doneBlend()
				break
			}
		}
	}
	if f.blend != nil {
		if f.lenBuildchar > 0 {
			if f.lenBuildchar > 0x0FFFFFFF {
				// FT_NEW_ARRAY: Array_Too_Large
				return nil, errInvalidArg
			}
			f.buildchar = make([]int64, f.lenBuildchar)
		}
	} else {
		f.lenBuildchar = 0
	}

	f.numGlyphs = l.numGlyphs
	if l.subrs != nil {
		f.numSubrs = l.numSubrs
		f.subrs = make([][]byte, len(l.subrs.elements))
		for i, e := range l.subrs.elements {
			if e != nil {
				f.subrs[i] = e[:l.subrs.lengths[i]]
			}
		}
		f.subrsHash = l.subrsHash
	}
	if l.charstrings == nil {
		err = errInvalidFileFormat
	} else {
		f.charstringsSet = true
		f.charstrings = make([][]byte, len(l.charstrings.elements))
		for i, e := range l.charstrings.elements {
			if e != nil {
				f.charstrings[i] = e[:l.charstrings.lengths[i]]
			}
		}
		f.glyphNames = make([]*string, len(l.glyphNames.elements))
		for i, e := range l.glyphNames.elements {
			if e != nil {
				s := cstr(e)
				f.glyphNames[i] = &s
			}
		}
	}

	if f.encodingType == t1EncodingArray {
		minChar, maxChar := 0, 0
		table := l.encodingTable
		for charcode := 0; charcode < table.maxElems; charcode++ {
			f.encoding.charIndex[charcode] = 0
			f.encoding.charName[charcode] = ".notdef"
			if table.elements[charcode] == nil {
				continue
			}
			charName := cstr(table.elements[charcode])
			for idx := 0; idx < int(f.numGlyphs); idx++ {
				glyphName := f.glyphNames[idx]
				if glyphName != nil && charName == *glyphName {
					f.encoding.charIndex[charcode] = uint16(idx)
					f.encoding.charName[charcode] = *glyphName
					if *glyphName != ".notdef" {
						if charcode < minChar {
							minChar = charcode
						}
						if charcode >= maxChar {
							maxChar = charcode + 1
						}
					}
					break
				}
			}
		}
		f.encoding.codeFirst = minChar
		f.encoding.codeLast = maxChar
		f.encoding.numChars = l.numChars
	}

	if priv.blueShift > 1000 || priv.blueShift < 0 {
		priv.blueShift = 7
	}
	if priv.blueFuzz > 1000 || priv.blueFuzz < 0 {
		priv.blueFuzz = 1
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

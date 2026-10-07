package ttf

import (
	"bytes"
	"errors"
)

// FreeType's Type 42 driver (type42's t42parse.c and t42objs.c): a
// TrueType font wrapped in a PostScript font dictionary. The dictionary
// is read with the Type 1 parser's tools; the glyphs are the embedded
// TrueType font's, loaded by the TrueType driver.

// t42Face is the part of a T42_FaceRec past its T1_FontRec.
type t42Face struct {
	type1 *t1Font
	ttf   *Face // the embedded TrueType font, opened by the TrueType driver
}

// t42Loader is a T42_LoaderRec, with the face's ttf_data.
type t42Loader struct {
	parser        psParser
	numChars      int
	encodingTable *psTable
	numGlyphs     int32
	glyphNames    *psTable
	charstrings   *psTable
	swapTable     *psTable

	ttfData []byte // nil: none
}

type t42Field struct {
	*t1Field
	reader func(f *t1Font, l *t42Loader)
}

var t42Keywords []t42Field

func init() {
	field := func(ident string) t42Field {
		for i := range t1Keywords {
			if t1Keywords[i].ident == ident {
				return t42Field{t1Field: &t1Keywords[i]}
			}
		}
		panic("ttf: no Type 1 keyword " + ident)
	}
	callback := func(ident string, reader func(*t1Font, *t42Loader)) t42Field {
		return t42Field{t1Field: &t1Field{ident: ident, typ: t1FieldTypeCallback}, reader: reader}
	}
	t42Keywords = []t42Field{
		field("version"), field("Notice"), field("FullName"), field("FamilyName"),
		field("Weight"), field("ItalicAngle"), field("isFixedPitch"),
		field("UnderlinePosition"), field("UnderlineThickness"),
		field("FSType"),
		field("FontName"), field("PaintType"), field("FontType"), field("StrokeWidth"),
		field("FontBBox"),
		callback("FontMatrix", t42ParseFontMatrix),
		callback("Encoding", t42ParseEncoding),
		callback("CharStrings", t42ParseCharstrings),
		callback("sfnts", t42ParseSfnts),
	}
}

// t42IsSpace is t42_is_space.
func t42IsSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == 0
}

// toBytesDelimited is ps_parser_to_bytes with delimiters: it returns the
// number of bytes read (0 when there was nothing to read) and the error.
func (p *psParser) toBytesDelimited(out []byte, maxBytes uint64) (uint32, error) {
	p.skipSpaces()
	cur := p.cursor
	if cur >= p.limit {
		return 0, nil
	}
	if p.base[cur] != '<' {
		return 0, errInvalidFileFormat
	}
	cur++
	n := psConvASCIIHexDecode(p.base, &cur, p.limit, out, maxBytes)
	p.cursor = cur
	if cur < p.limit && p.base[cur] != '>' {
		return n, errInvalidFileFormat
	}
	p.cursor++
	return n, nil
}

// t42ParseFontMatrix is t42_parse_font_matrix.
func t42ParseFontMatrix(f *t1Font, l *t42Loader) {
	p := &l.parser
	var temp [6]int64
	if p.toFixedArray(6, temp[:], 0) < 6 {
		p.err = errInvalidFileFormat
		return
	}
	tempScale := abs64(temp[3])
	if tempScale == 0 {
		p.err = errInvalidFileFormat
		return
	}
	if tempScale != 0x10000 {
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

// t42ParseEncoding is t42_parse_encoding.
func t42ParseEncoding(f *t1Font, l *t42Loader) {
	p := &l.parser
	b := p.base
	limit := p.limit
	p.skipSpaces()
	cur := p.cursor
	if cur >= limit {
		p.err = errInvalidFileFormat
		return
	}
	if !(isPSDigit(b[cur]) || b[cur] == '[') {
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
		return
	}

	var count int
	onlyImmediates := false
	if b[cur] == '[' {
		count = 256
		onlyImmediates = true
		p.cursor++
	} else {
		count = int(int32(p.toInt()))
	}
	// only composite fonts (which aren't supported) can have more
	if count > 256 {
		p.err = errInvalidFileFormat
		return
	}
	p.skipSpaces()
	if p.cursor >= limit {
		return
	}
	if count < 0 {
		// FT_QNEW_ARRAY with a negative count fails
		p.err = errInvalidArg
		return
	}
	l.numChars = count
	f.encoding.numChars = count
	f.encoding.charIndex = make([]uint16, count)
	f.encoding.charName = make([]string, count)
	table := newPSTable(count)
	l.encodingTable = table
	for n := 0; n < count; n++ {
		_ = table.add(n, []byte(".notdef\x00")) // (void) in FreeType
	}

	// Read `charcode /charname' records, or only immediates for an
	// array given as `[ ... ]'.
	n := 0
	p.skipSpaces()
	for p.cursor < limit {
		cur = p.cursor
		// stop at `def' or `]'
		if b[cur] == 'd' && cur+3 < limit {
			if b[cur+1] == 'e' && b[cur+2] == 'f' && t42IsSpace(b[cur+3]) {
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
				// protect against an invalid charcode
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
				obj := append([]byte(nil), b[cur:cur+length+1]...)
				obj[length] = 0
				if err := table.add(charcode, obj); err != nil {
					p.err = err
					return
				}
				n++
			} else if onlyImmediates {
				// not a Type 42 encoding (a CID Type 42 font's, perhaps)
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
}

// t42 sfnts loading states.
const (
	t42BeforeStart = iota
	t42BeforeTableDir
	t42OtherTables
)

// t42ParseSfnts is t42_parse_sfnts: the TrueType font from the sfnts
// array of hex strings, or of binary strings after `RD'.
func t42ParseSfnts(f *t1Font, l *t42Loader) {
	p := &l.parser
	b := p.base
	limit := p.limit

	// There should only be one sfnts array, but free any previous.
	l.ttfData = nil

	var ttf []byte
	var stringBuf []byte // a hex string's bytes, or binary data in b
	haveString := false
	allocated := false
	var stringSize uint64
	numTables := 0
	status := t42BeforeStart
	var ttfCount, ttfReserved int64 = 0, 12

	fail := func(err error) {
		p.err = err
		l.ttfData = nil
	}

	p.skipSpaces()
	if p.cursor >= limit || b[p.cursor] != '[' {
		fail(errInvalidFileFormat)
		return
	}
	p.cursor++
	p.skipSpaces()
	ttf = make([]byte, ttfReserved)

	for p.cursor < limit {
		cur := p.cursor
		switch {
		case b[cur] == ']':
			p.cursor++
			l.ttfData = ttf[:ttfCount]
			if p.err != nil {
				l.ttfData = nil
			}
			return

		case b[cur] == '<':
			if haveString && !allocated {
				// can't handle mixed binary and hex strings
				fail(errInvalidFileFormat)
				return
			}
			p.skipPSToken()
			if p.err != nil {
				l.ttfData = nil
				return
			}
			// don't include delimiters
			stringSize = uint64((p.cursor - cur - 2 + 1) / 2)
			if stringSize == 0 {
				fail(errInvalidFileFormat)
				return
			}
			stringBuf = make([]byte, stringSize)
			haveString = true
			allocated = true
			p.cursor = cur
			realSize, _ := p.toBytesDelimited(stringBuf, stringSize)
			stringSize = uint64(realSize)

		case isPSDigit(b[cur]):
			if allocated {
				// can't handle mixed binary and hex strings
				fail(errInvalidFileFormat)
				return
			}
			tmp := p.toInt()
			if tmp < 0 {
				fail(errInvalidFileFormat)
				return
			}
			stringSize = uint64(tmp)
			p.skipPSToken() // `RD'
			if p.err != nil {
				return
			}
			// one space after `RD'
			start := p.cursor + 1
			if uint64(limit-p.cursor) <= stringSize {
				fail(errInvalidFileFormat)
				return
			}
			p.cursor += int(stringSize) + 1
			stringBuf = b[start : start+int(stringSize)]
			haveString = true
		}

		if !haveString {
			fail(errInvalidFileFormat)
			return
		}

		// A string can have a trailing zero (odd) byte for padding.
		if stringSize&1 != 0 && stringBuf[stringSize-1] == 0 {
			stringSize--
		}
		if stringSize == 0 {
			fail(errInvalidFileFormat)
			return
		}

		// Copy the string to the TrueType data, checking it.
		size := uint64(limit - p.cursor)
		for n := uint64(0); n < stringSize; n++ {
			if status == t42BeforeStart {
				// the offset table, 12 bytes
				if ttfCount < 12 {
					ttf[ttfCount] = stringBuf[n]
					ttfCount++
					continue
				}
				numTables = 16*int(ttf[4]) + int(ttf[5])
				status = t42BeforeTableDir
				ttfReserved = 12 + 16*int64(numTables)
				if int64(size) < ttfReserved {
					fail(errInvalidFileFormat)
					return
				}
				ttf = append(ttf, make([]byte, ttfReserved-int64(len(ttf)))...)
			}
			if status == t42BeforeTableDir {
				// the table directory
				if ttfCount < ttfReserved {
					ttf[ttfCount] = stringBuf[n]
					ttfCount++
					continue
				}
				for i := 0; i < numTables; i++ {
					length := uint64(peek32(ttf, 12+16*i+12))
					if length > size || ttfReserved > int64(size-length) {
						fail(errInvalidFileFormat)
						return
					}
					// padded to 4 bytes
					ttfReserved += int64((length + 3) &^ 3)
				}
				ttfReserved++
				status = t42OtherTables
				ttf = append(ttf, make([]byte, ttfReserved-int64(len(ttf)))...)
			}
			// all other tables are just copied
			if ttfCount >= ttfReserved {
				// too much binary data
				fail(errInvalidFileFormat)
				return
			}
			ttf[ttfCount] = stringBuf[n]
			ttfCount++
		}
		p.skipSpaces()
	}

	// the array has no end
	fail(errInvalidFileFormat)
}

// t42ParseCharstrings is t42_parse_charstrings: the glyph names, and
// the TrueType glyph index of each (as text).
func t42ParseCharstrings(f *t1Font, l *t42Loader) {
	p := &l.parser
	b := p.base
	limit := p.limit
	notdefIndex := 0
	notdefFound := false

	p.skipSpaces()
	if p.cursor >= limit {
		p.err = errInvalidFileFormat
		return
	}
	if isPSDigit(b[p.cursor]) {
		l.numGlyphs = int32(p.toInt())
		if p.err != nil {
			return
		}
		if l.numGlyphs < 0 {
			p.err = errInvalidFileFormat
			return
		}
		// we certainly need more than 4 bytes per glyph
		if int64(l.numGlyphs) > int64(limit-p.cursor)>>2 {
			l.numGlyphs = int32((limit - p.cursor) >> 2)
		}
	} else if b[p.cursor] == '<' {
		// `<< ... >>': count the `/'s in the dictionary for its size
		count := int32(0)
		p.skipPSToken()
		if p.err != nil {
			return
		}
		p.skipSpaces()
		cur := p.cursor
		for p.cursor < limit {
			if b[p.cursor] == '/' {
				count++
			} else if b[p.cursor] == '>' {
				l.numGlyphs = count
				p.cursor = cur // rewind
				break
			}
			p.skipPSToken()
			if p.err != nil {
				return
			}
			p.skipSpaces()
		}
	} else {
		p.err = errInvalidFileFormat
		return
	}

	if p.cursor >= limit {
		p.err = errInvalidFileFormat
		return
	}

	// unlike Type 1, one CharStrings array only
	if l.swapTable != nil {
		p.err = errInvalidFileFormat
		return
	}
	codeTable := newPSTable(int(l.numGlyphs))
	l.charstrings = codeTable
	nameTable := newPSTable(int(l.numGlyphs))
	l.glyphNames = nameTable
	// for swapping the /.notdef glyph with glyph 0
	swapTable := newPSTable(4)
	l.swapTable = swapTable

	n := 0
	for {
		// `/glyphname' + index [+ `def'], or `(glyphname)' [+ `cvn'] +
		// index [+ `def'] (as LilyPond writes them)
		p.skipSpaces()
		cur := p.cursor
		if cur >= limit {
			break
		}
		// stop at `end' or `>'
		if b[cur] == 'e' && cur+3 < limit && b[cur+1] == 'n' && b[cur+2] == 'd' &&
			t42IsSpace(b[cur+3]) {
			break
		}
		if b[cur] == '>' {
			break
		}

		p.skipPSToken()
		if p.cursor >= limit {
			p.err = errInvalidFileFormat
			return
		}
		if p.err != nil {
			return
		}

		if b[cur] == '/' || b[cur] == '(' {
			haveLiteral := b[cur] == '('
			extra := 2
			if haveLiteral {
				extra = 3
			}
			if cur+extra >= limit {
				p.err = errInvalidFileFormat
				return
			}
			cur++ // skip `/'
			length := p.cursor - cur
			if haveLiteral {
				length--
			}
			obj := append([]byte(nil), b[cur:cur+length+1]...)
			obj[length] = 0
			if err := nameTable.add(n, obj); err != nil {
				p.err = err
				return
			}
			// record the index of /.notdef
			if b[cur] == '.' && cstr(nameTable.elements[n]) == ".notdef" {
				notdefIndex = n
				notdefFound = true
			}

			p.skipSpaces()
			if haveLiteral {
				p.skipPSToken()
			}
			cur = p.cursor
			p.toInt()
			if p.cursor >= limit {
				p.err = errInvalidFileFormat
				return
			}
			length = p.cursor - cur
			obj = append([]byte(nil), b[cur:cur+length+1]...)
			obj[length] = 0
			if err := codeTable.add(n, obj); err != nil {
				p.err = err
				return
			}
			n++
			if n >= int(l.numGlyphs) {
				break
			}
		}
	}
	l.numGlyphs = int32(n)

	if !notdefFound {
		p.err = errInvalidFileFormat
		return
	}

	// If /.notdef isn't glyph 0, swap them.
	if cstr(nameTable.elements[0]) != ".notdef" {
		elem := func(t *psTable, i int) []byte { return t.elements[i][:t.lengths[i]] }
		for _, e := range []error{
			swapTable.add(0, elem(nameTable, 0)),
			swapTable.add(1, elem(codeTable, 0)),
			swapTable.add(2, elem(nameTable, notdefIndex)),
			swapTable.add(3, elem(codeTable, notdefIndex)),
			nameTable.add(notdefIndex, elem(swapTable, 0)),
			codeTable.add(notdefIndex, elem(swapTable, 1)),
			nameTable.add(0, elem(swapTable, 2)),
			codeTable.add(0, elem(swapTable, 3)),
		} {
			if e != nil {
				p.err = e
				return
			}
		}
	}
}

// t42LoadKeyword is t42_load_keyword.
func t42LoadKeyword(f *t1Font, l *t42Loader, field *t42Field) error {
	if field.typ == t1FieldTypeCallback {
		field.reader(f, l)
		return l.parser.err
	}
	var object any
	switch field.location {
	case t1LocationFontInfo:
		object = &f.fontInfo
	case t1LocationFontExtra:
		object = &f.fontExtra
	case t1LocationBBox:
		object = &f.fontBBox
	default:
		object = f
	}
	objects := []any{object}
	if field.typ == t1FieldTypeIntArray {
		return psLoadFieldTable(&l.parser, field.t1Field, objects, 0)
	}
	return psLoadField(&l.parser, field.t1Field, field.typ, 0, objects, 0)
}

// t42ParseDict is t42_parse_dict.
func t42ParseDict(f *t1Font, l *t42Loader, base []byte) error {
	p := &l.parser
	p.base = base
	p.cursor = 0
	p.limit = len(base)
	p.err = nil
	b := base
	limit := p.limit

	p.skipSpaces()
	for p.cursor < limit {
		cur := p.cursor
		switch {
		case b[cur] == 'F' && cur+25 < limit && bytes.HasPrefix(b[cur:], []byte("FontDirectory")):
			// `FontDirectory' causes problems for some fonts
			p.skipPSToken()
			p.skipSpaces()
			cur = p.cursor
			cur2 := cur
			// look up the `known' keyword
			for cur < limit {
				if b[cur] == 'k' && cur+5 < limit && bytes.HasPrefix(b[cur:], []byte("known")) {
					break
				}
				p.skipPSToken()
				if p.err != nil {
					return p.err
				}
				p.skipSpaces()
				cur = p.cursor
			}
			if cur < limit {
				// skip `known' and the token after it, unless an array
				p.skipPSToken()
				if tok := p.toToken(); tok.typ == t1TokenArray {
					cur2 = p.cursor
				}
			}
			p.cursor = cur2

		case b[cur] == '/' && cur+2 < limit:
			// an immediate
			cur++
			p.cursor = cur
			p.skipPSToken()
			if p.err != nil {
				return p.err
			}
			n := p.cursor - cur
			if n > 0 && n < 22 && p.cursor < limit {
				name := string(b[cur : cur+n])
				for i := range t42Keywords {
					kw := &t42Keywords[i]
					if kw.ident == name {
						p.err = t42LoadKeyword(f, l, kw)
						if p.err != nil {
							return p.err
						}
						break
					}
				}
			}

		default:
			p.skipPSToken()
			if p.err != nil {
				return p.err
			}
		}
		p.skipSpaces()
	}
	return p.err
}

// t42OpenFace is T42_Open_Face: the font dictionary, and the TrueType
// font's data.
func t42OpenFace(data []byte) (*t1Font, []byte, error) {
	if len(data) < 17 {
		// FT_FRAME_ENTER fails
		return nil, nil, errInvalidTable
	}
	if !bytes.HasPrefix(data, []byte("%!PS-TrueTypeFont")) {
		return nil, nil, errUnknownFormat
	}
	f := &t1Font{}
	l := &t42Loader{}
	l.parser.init(data)
	if err := t42ParseDict(f, l, data); err != nil {
		return nil, nil, err
	}
	if f.fontType != 42 {
		return nil, nil, errUnknownFormat
	}

	// the charstrings and glyph names tables, to the Type 1 data
	f.numGlyphs = l.numGlyphs
	var err error
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

	// the encoding of a custom array: the glyph of each name
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
				glyphName := *f.glyphNames[idx]
				if charName == glyphName {
					f.encoding.charIndex[charcode] = uint16(idx)
					f.encoding.charName[charcode] = glyphName
					if glyphName != ".notdef" {
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
	if err != nil {
		return nil, nil, err
	}
	return f, l.ttfData, nil
}

// t42FaceInit is T42_Face_Init: a Type 42 font as a face. It reports
// errUnknownFormat for other files.
func t42FaceInit(data []byte) (*Face, error) {
	type1, ttfData, err := t42OpenFace(data)
	if err != nil {
		return nil, err
	}
	f := &Face{data: data, t42: &t42Face{type1: type1}}
	f.numGlyphs = int64(type1.numGlyphs)
	info := &type1.fontInfo

	// the style name; some broken fonts only have a /FontName
	style := "Regular"
	if info.familyName != nil {
		f.FamilyName = *info.familyName
		if info.fullName != nil {
			full, fam := *info.fullName, f.FamilyName
			famAt := func(j int) byte {
				if j < len(fam) {
					return fam[j]
				}
				return 0
			}
			for i, j := 0, 0; i < len(full); {
				if full[i] == famAt(j) {
					i++
					j++
				} else if full[i] == ' ' || full[i] == '-' {
					i++
				} else if famAt(j) == ' ' || famAt(j) == '-' {
					j++
				} else {
					if famAt(j) == 0 {
						style = full[i:]
					}
					break
				}
			}
		}
	} else if type1.fontName != nil {
		f.FamilyName = *type1.fontName
	}
	f.StyleName = style

	// the embedded TrueType font, through the TrueType driver only
	ttf, err := ttFaceInit(ttfData)
	if err != nil {
		return nil, err
	}
	f.t42.ttf = ttf
	// the TrueType font's metrics, not FontInfo's
	f.unitsPerEM = ttf.unitsPerEM
	f.ascender = ttf.ascender
	f.descender = ttf.descender

	// a Unicode charmap from the glyph names
	glyphName := func(idx uint32) (string, bool) {
		if name := type1.glyphNames[idx]; name != nil {
			return *name, true
		}
		return "", false
	}
	u, uerr := unicodesInit(uint32(type1.numGlyphs), glyphName)
	if uerr == nil {
		f.charmaps = append(f.charmaps, &charmap{
			platformID: platformMicrosoft,
			encodingID: msIDUnicode,
			encoding:   encodingUnicode,
			synthetic:  u,
		})
	} else if !errors.Is(uerr, errNoUnicodeGlyphName) {
		return nil, uerr
	}
	if type1.encodingType == t1EncodingISOLatin1 && uerr != nil {
		// its charmap has the Unicode class, whose init fails the same
		return nil, uerr
	}
	return f, nil
}

// cStrtol10 is strtol(s, NULL, 10) on a C string.
func cStrtol10(b []byte) int64 {
	s := cstr(b)
	i := 0
	for i < len(s) && (s[i] == ' ' || (s[i] >= '\t' && s[i] <= '\r')) {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	var v uint64
	overflow := false
	for ; i < len(s) && isPSDigit(s[i]); i++ {
		d := uint64(s[i] - '0')
		if v > (1<<63-d)/10 {
			overflow = true
		}
		if !overflow {
			v = v*10 + d
		}
	}
	switch {
	case overflow && neg:
		return -1 << 63
	case overflow:
		return 1<<63 - 1
	case neg:
		return -int64(v)
	}
	return int64(v)
}

// loadT42Glyph is T42_GlyphSlot_Load: the embedded TrueType font's glyph
// that the charstring names.
func (f *Face) loadT42Glyph(glyphIndex uint32) (*Glyph, error) {
	cs := f.t42.type1.charstrings
	if int(glyphIndex) >= len(cs) {
		// FreeType reads past the array
		return nil, errInvalidArg
	}
	return f.t42.ttf.LoadGlyph(uint32(cStrtol10(cs[glyphIndex])))
}

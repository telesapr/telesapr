package ttf

import "errors"

// FreeType's PostScript parsing for the Type 1 driver: psaux's psconv.c
// (numbers, hex and eexec decoding), psobjs.c (the PS_Parser tokenizer
// and field loading, PS_Table), and type1's t1parse.c (the PFA/PFB
// segments and the private dictionary).

var (
	errInvalidFileFormat = errors.New("ttf: invalid file format")
	errIgnore            = errors.New("ttf: ignore")
)

func isPSNewline(c byte) bool { return c == '\r' || c == '\n' }

func isPSSpace(c byte) bool {
	return c == ' ' || isPSNewline(c) || c == '\t' || c == '\f' || c == 0
}

func isPSSpecial(c byte) bool {
	switch c {
	case '/', '(', ')', '<', '>', '[', ']', '{', '}', '%':
		return true
	}
	return false
}

func isPSDelim(c byte) bool { return isPSSpace(c) || isPSSpecial(c) }

func isPSDigit(c byte) bool { return '0' <= c && c <= '9' }

func isPSXDigit(c byte) bool {
	return isPSDigit(c) || ('A' <= c && c <= 'F') || ('a' <= c && c <= 'f')
}

// psCharTable is psconv.c's ft_char_table: digit values in bases up to 36.
var psCharTable = [128]int8{
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, -1, -1, -1, -1, -1, -1,
	-1, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24,
	25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, -1, -1, -1, -1, -1,
	-1, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24,
	25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, -1, -1, -1, -1, -1,
}

// psConvStrtol is PS_Conv_Strtol.
func psConvStrtol(b []byte, p *int, limit int, base int64) int64 {
	q := *p
	if q >= limit {
		return 0
	}
	if base < 2 || base > 36 {
		return 0
	}
	sign := false
	if b[q] == '-' || b[q] == '+' {
		sign = b[q] == '-'
		q++
		if q == limit {
			return 0
		}
		if b[q] == '-' || b[q] == '+' {
			return 0
		}
	}
	numLimit := int64(0x7FFFFFFF) / base
	cLimit := int8(int64(0x7FFFFFFF) % base)
	var num int64
	overflow := false
	for ; q < limit; q++ {
		if isPSSpace(b[q]) || b[q] >= 0x80 {
			break
		}
		c := psCharTable[b[q]&0x7F]
		if c < 0 || int64(c) >= base {
			break
		}
		if num > numLimit || (num == numLimit && c > cLimit) {
			overflow = true
		} else {
			num = num*base + int64(c)
		}
	}
	*p = q
	if overflow {
		num = 0x7FFFFFFF
	}
	if sign {
		num = -num
	}
	return num
}

// psConvToInt is PS_Conv_ToInt.
func psConvToInt(b []byte, cursor *int, limit int) int64 {
	p := *cursor
	curp := p
	num := psConvStrtol(b, &p, limit, 10)
	if p == curp {
		return 0
	}
	if p < limit && b[p] == '#' {
		p++
		curp = p
		num = psConvStrtol(b, &p, limit, num)
		if p == curp {
			return 0
		}
	}
	*cursor = p
	return num
}

// psConvToFixed is PS_Conv_ToFixed.
func psConvToFixed(b []byte, cursor *int, limit int, powerTen int64) int64 {
	p := *cursor
	var integral, decimal int64
	divider := int64(1)
	sign, overflow, underflow := false, false, false
	if p >= limit {
		return 0
	}
	if b[p] == '-' || b[p] == '+' {
		sign = b[p] == '-'
		p++
		if p == limit {
			return 0
		}
		if b[p] == '-' || b[p] == '+' {
			return 0
		}
	}
	if b[p] != '.' {
		curp := p
		integral = psConvToInt(b, &p, limit)
		if p == curp {
			return 0
		}
		if integral > 0x7FFF {
			overflow = true
		} else {
			integral = int64(uint32(integral) << 16)
		}
	}
	if p < limit && b[p] == '.' {
		p++
		for ; p < limit; p++ {
			if isPSSpace(b[p]) || b[p] >= 0x80 {
				break
			}
			c := psCharTable[b[p]&0x7F]
			if c < 0 || c >= 10 {
				break
			}
			if divider < 0xCCCCCCC && decimal < 0xCCCCCCC {
				decimal = decimal*10 + int64(c)
				if integral == 0 && powerTen > 0 {
					powerTen--
				} else {
					divider *= 10
				}
			}
		}
	}
	if p+1 < limit && (b[p] == 'e' || b[p] == 'E') {
		p++
		curp := p
		exponent := psConvToInt(b, &p, limit)
		if curp == p {
			return 0
		}
		switch {
		case exponent > 1000:
			overflow = true

		case exponent < -1000:
			underflow = true

		default:
			powerTen += exponent
		}
	}
	*cursor = p
	if integral == 0 && decimal == 0 {
		return 0
	}
	if overflow {
		integral = 0x7FFFFFFF
		goto exit
	}
	if underflow {
		return 0
	}
	for powerTen > 0 {
		if integral >= 0xCCCCCCC {
			integral = 0x7FFFFFFF
			goto exit
		}
		integral *= 10
		if decimal >= 0xCCCCCCC {
			if divider == 1 {
				integral = 0x7FFFFFFF
				goto exit
			}
			divider /= 10
		} else {
			decimal *= 10
		}
		powerTen--
	}
	for powerTen < 0 {
		integral /= 10
		if divider < 0xCCCCCCC {
			divider *= 10
		} else {
			decimal /= 10
		}
		if integral == 0 && decimal == 0 {
			return 0
		}
		powerTen++
	}
	if decimal != 0 {
		decimal = divFix(decimal, divider)
		integral += decimal
	}
exit:
	if sign {
		integral = -integral
	}
	return integral
}

// psConvASCIIHexDecode is PS_Conv_ASCIIHexDecode: it decodes from
// b[*cursor:limit] into out, at most n bytes.
func psConvASCIIHexDecode(b []byte, cursor *int, limit int, out []byte, n uint64) uint32 {
	p := *cursor
	if p >= limit {
		return 0
	}
	n *= 2
	if n > uint64(limit-p) {
		n = uint64(limit - p)
	}
	var w uint32
	pad := uint32(1)
	r := uint64(0)
	for ; r < n; r++ {
		c := uint32(b[p+int(r)])
		if isPSSpace(byte(c)) {
			continue
		}
		if c >= 0x80 {
			break
		}
		c = uint32(psCharTable[c&0x7F])
		if c >= 16 {
			break
		}
		pad = pad<<4 | c
		if pad&0x100 != 0 {
			out[w] = byte(pad)
			w++
			pad = 1
		}
	}
	if pad != 1 {
		out[w] = byte(pad << 4)
		w++
	}
	*cursor = p + int(r)
	return w
}

// t1Decrypt is t1_decrypt (PS_Conv_EexecDecode in place).
func t1Decrypt(buf []byte, seed uint16) {
	s := uint32(seed)
	for i, v := range buf {
		val := uint32(v)
		b := val ^ (s >> 8)
		s = ((val+s)*52845 + 22719) & 0xFFFF
		buf[i] = byte(b)
	}
}

// Token types.
const (
	t1TokenNone = iota
	t1TokenAny
	t1TokenString
	t1TokenArray
	t1TokenKey
)

// t1Token is a T1_TokenRec: offsets into the parser's buffer.
type t1Token struct {
	typ          int
	start, limit int
	valid        bool // start and limit aren't NULL
}

// psParser is a PS_ParserRec over a buffer.
type psParser struct {
	base   []byte
	cursor int
	limit  int
	err    error
}

func (p *psParser) init(base []byte) {
	*p = psParser{base: base, limit: len(base)}
}

// psSkipComment is skip_comment.
func psSkipComment(b []byte, cur *int, limit int) {
	c := *cur
	for c < limit {
		if isPSNewline(b[c]) {
			break
		}
		c++
	}
	*cur = c
}

// psSkipSpaces is skip_spaces.
func psSkipSpaces(b []byte, cur *int, limit int) {
	c := *cur
	for c < limit {
		if !isPSSpace(b[c]) {
			if b[c] == '%' {
				psSkipComment(b, &c, limit)
			} else {
				break
			}
		}
		c++
	}
	*cur = c
}

// psSkipLiteralString is skip_literal_string.
func psSkipLiteralString(b []byte, cur *int, limit int) error {
	c := *cur
	embed := 0
	err := errInvalidFileFormat
	for c < limit {
		ch := b[c]
		c++
		if ch == '\\' { //nolint:gocritic // a branch breaks out of the loop
			if c == limit {
				break
			}
			switch b[c] {
			case 'n', 'r', 't', 'b', 'f', '\\', '(', ')':
				c++
			default:
				for i := 0; i < 3 && c < limit; i++ {
					if '0' > b[c] || b[c] > '7' {
						break
					}
					c++
				}
			}
		} else if ch == '(' {
			embed++
		} else if ch == ')' {
			embed--
			if embed == 0 {
				err = nil
				break
			}
		}
	}
	*cur = c
	return err
}

// psSkipString is skip_string.
func psSkipString(b []byte, cur *int, limit int) error {
	c := *cur
	var err error
	for {
		c++
		if c >= limit {
			break
		}
		psSkipSpaces(b, &c, limit)
		if c >= limit {
			break
		}
		if !isPSXDigit(b[c]) {
			break
		}
	}
	if c < limit && b[c] != '>' {
		err = errInvalidFileFormat
	} else {
		c++
	}
	*cur = c
	return err
}

// psSkipProcedure is skip_procedure.
func psSkipProcedure(b []byte, cur *int, limit int) error {
	var err error
	embed := 0
	c := *cur
loop:
	for ; c < limit && err == nil; c++ {
		switch b[c] {
		case '{':
			embed++
		case '}':
			embed--
			if embed == 0 {
				c++
				break loop
			}
		case '(':
			err = psSkipLiteralString(b, &c, limit)
		case '<':
			err = psSkipString(b, &c, limit)
		case '%':
			psSkipComment(b, &c, limit)
		}
	}
	if embed != 0 {
		err = errInvalidFileFormat
	}
	*cur = c
	return err
}

// skipPSToken is ps_parser_skip_PS_token.
func (p *psParser) skipPSToken() {
	b := p.base
	cur := p.cursor
	limit := p.limit
	var err error

	psSkipSpaces(b, &cur, limit)
	if cur >= limit {
		goto exit
	}
	if b[cur] == '[' || b[cur] == ']' {
		cur++
		goto exit
	}
	if b[cur] == '{' {
		err = psSkipProcedure(b, &cur, limit)
		goto exit
	}
	if b[cur] == '(' {
		err = psSkipLiteralString(b, &cur, limit)
		goto exit
	}
	if b[cur] == '<' {
		if cur+1 < limit && b[cur+1] == '<' {
			cur += 2
		} else {
			err = psSkipString(b, &cur, limit)
		}
		goto exit
	}
	if b[cur] == '>' {
		cur++
		if cur >= limit || b[cur] != '>' {
			err = errInvalidFileFormat
			goto exit
		}
		cur++
		goto exit
	}
	if b[cur] == '/' {
		cur++
	}
	for cur < limit {
		if isPSDelim(b[cur]) {
			break
		}
		cur++
	}
exit:
	if cur < limit && cur == p.cursor {
		err = errInvalidFileFormat
	}
	if cur > limit {
		cur = limit
	}
	p.err = err
	p.cursor = cur
}

// skipSpaces is ps_parser_skip_spaces.
func (p *psParser) skipSpaces() { psSkipSpaces(p.base, &p.cursor, p.limit) }

// toToken is ps_parser_to_token.
func (p *psParser) toToken() t1Token {
	var tok t1Token
	p.skipSpaces()
	b := p.base
	cur := p.cursor
	limit := p.limit
	if cur >= limit {
		return tok
	}
	switch b[cur] {
	case '(':
		tok.typ = t1TokenString
		tok.start = cur
		if psSkipLiteralString(b, &cur, limit) == nil {
			tok.limit, tok.valid = cur, true
		}
	case '{':
		tok.typ = t1TokenArray
		tok.start = cur
		if psSkipProcedure(b, &cur, limit) == nil {
			tok.limit, tok.valid = cur, true
		}
	case '[':
		tok.typ = t1TokenArray
		embed := 1
		tok.start = cur
		cur++
		p.cursor = cur
		p.skipSpaces()
		cur = p.cursor
		for cur < limit && p.err == nil {
			if b[cur] == '[' {
				embed++
			} else if b[cur] == ']' {
				embed--
				if embed <= 0 {
					cur++
					tok.limit, tok.valid = cur, true
					break
				}
			}
			p.cursor = cur
			p.skipPSToken()
			p.skipSpaces()
			cur = p.cursor
		}
	default:
		tok.start = cur
		if b[cur] == '/' {
			tok.typ = t1TokenKey
		} else {
			tok.typ = t1TokenAny
		}
		p.skipPSToken()
		cur = p.cursor
		if p.err == nil {
			tok.limit, tok.valid = cur, true
		}
	}
	if !tok.valid {
		tok.start = 0
		tok.typ = t1TokenNone
	}
	p.cursor = cur
	return tok
}

// toTokenArray is ps_parser_to_token_array: the elements of an array
// token, up to max stored, and their count (-1 for no array).
func (p *psParser) toTokenArray(limit int) ([]t1Token, int) {
	master := p.toToken()
	if master.typ != t1TokenArray {
		return nil, -1
	}
	oldCursor, oldLimit := p.cursor, p.limit
	p.cursor = master.start + 1
	p.limit = master.limit - 1
	var tokens []t1Token
	n := 0
	for p.cursor < p.limit {
		tok := p.toToken()
		if tok.typ == t1TokenNone {
			break
		}
		if n < limit {
			tokens = append(tokens, tok)
		}
		n++
	}
	p.cursor, p.limit = oldCursor, oldLimit
	return tokens, n
}

// psToFixedArray is ps_tofixedarray, counting only when values is nil.
func psToFixedArray(b []byte, cur *int, limit int, maxValues int, values []int64, powerTen int64) int {
	c := *cur
	count := 0
	defer func() { *cur = c }()
	if c >= limit {
		return count
	}
	var ender byte
	switch b[c] {
	case '[':
		ender = ']'
	case '{':
		ender = '}'
	}
	if ender != 0 {
		c++
	}
	for c < limit {
		psSkipSpaces(b, &c, limit)
		if c >= limit {
			return count
		}
		if b[c] == ender {
			c++
			break
		}
		old := c
		if values != nil && count >= maxValues {
			break
		}
		v := psConvToFixed(b, &c, limit, powerTen)
		if values != nil {
			values[count] = v
		}
		if old == c {
			return -1
		}
		count++
		if ender == 0 {
			break
		}
	}
	return count
}

// psToBool is ps_tobool.
func psToBool(b []byte, cur *int, limit int) int64 {
	c := *cur
	var result int64
	if c+3 < limit && b[c] == 't' && b[c+1] == 'r' && b[c+2] == 'u' && b[c+3] == 'e' {
		result = 1
		c += 5
	} else if c+4 < limit && b[c] == 'f' && b[c+1] == 'a' && b[c+2] == 'l' && b[c+3] == 's' &&
		b[c+4] == 'e' {
		c += 6
	}
	*cur = c
	return result
}

// toInt is ps_parser_to_int.
func (p *psParser) toInt() int64 {
	p.skipSpaces()
	return psConvToInt(p.base, &p.cursor, p.limit)
}

// toFixed is ps_parser_to_fixed.
func (p *psParser) toFixed(powerTen int64) int64 {
	p.skipSpaces()
	return psConvToFixed(p.base, &p.cursor, p.limit, powerTen)
}

// toFixedArray is ps_parser_to_fixed_array.
func (p *psParser) toFixedArray(limit int, values []int64, powerTen int64) int {
	p.skipSpaces()
	return psToFixedArray(p.base, &p.cursor, p.limit, limit, values, powerTen)
}

// toBytes is ps_parser_to_bytes without delimiters.
func (p *psParser) toBytes(out []byte, maxBytes uint64) uint32 {
	p.skipSpaces()
	if p.cursor >= p.limit {
		return 0
	}
	cur := p.cursor
	n := psConvASCIIHexDecode(p.base, &cur, p.limit, out, maxBytes)
	p.cursor = cur
	return n
}

// psTable is a PS_TableRec: elements are copies (nil for none).
type psTable struct {
	init      bool
	maxElems  int
	elements  [][]byte
	lengths   []uint32
	capacity  uint64
	cursorLen uint64
}

// newPSTable is ps_table_new.
func newPSTable(count int) *psTable {
	return &psTable{init: true, maxElems: count, elements: make([][]byte, count),
		lengths: make([]uint32, count)}
}

// add is ps_table_add. An element added while the table has no block
// yet, with no bytes, stays NULL, as in C.
func (t *psTable) add(idx int, object []byte) error {
	if idx < 0 || idx >= t.maxElems {
		return errInvalidArg
	}
	length := uint64(len(object))
	if t.cursorLen+length > t.capacity {
		newSize := t.capacity
		for newSize < t.cursorLen+length {
			newSize += (newSize >> 2) + 1
			newSize = (newSize + 1023) &^ 1023
		}
		t.capacity = newSize
	}
	if t.capacity == 0 {
		t.elements[idx] = nil
	} else {
		t.elements[idx] = append(make([]byte, 0, length), object...)
	}
	t.lengths[idx] = uint32(length)
	t.cursorLen += length
	return nil
}

// cstr is an element as a C string (up to its first NUL).
func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

// t1Parser is a T1_ParserRec over an in-memory (mmap'ed) file.
type t1Parser struct {
	root        psParser
	data        []byte
	baseDict    []byte
	privateDict []byte
	inPFB       bool
}

// readPFBTag is read_pfb_tag: the tag and the segment size.
func readPFBTag(s *cffStream) (tag uint16, size uint32, err error) {
	t, err := s.readU16()
	if err != nil {
		return 0, 0, err
	}
	if t == 0x8001 || t == 0x8002 {
		sz, err := s.readU32LE()
		if err == nil {
			size = sz
		}
	}
	return t, size, nil
}

// checkType1Format is check_type1_format.
func checkType1Format(data []byte, header string) error {
	s := &cffStream{data: data}
	tag, _, err := readPFBTag(s)
	if err != nil {
		return err
	}
	if tag != 0x8001 {
		if err := s.seek(0); err != nil {
			return err
		}
	}
	p, err := s.frame(uint64(len(header)))
	if err != nil {
		return err
	}
	if string(p) != header {
		return errUnknownFormat
	}
	return nil
}

// newT1Parser is T1_New_Parser.
func newT1Parser(data []byte) (*t1Parser, error) {
	p := &t1Parser{data: data}
	if err := checkType1Format(data, "%!PS-AdobeFont"); err != nil {
		if !errors.Is(err, errUnknownFormat) {
			return nil, err
		}
		if err := checkType1Format(data, "%!FontType"); err != nil {
			return nil, err
		}
	}
	s := &cffStream{data: data}
	tag, size, err := readPFBTag(s)
	if err != nil {
		return nil, err
	}
	var start int64
	if tag != 0x8001 {
		size = uint32(len(data))
	} else {
		p.inPFB = true
		start = s.pos
	}
	// FT_STREAM_SKIP on the memory stream
	if err := s.seek(start + int64(size)); err != nil {
		return nil, err
	}
	p.baseDict = data[start : start+int64(size)]
	p.root.init(p.baseDict)
	return p, nil
}

// getPrivateDict is T1_Get_Private_Dict.
func (p *t1Parser) getPrivateDict() error {
	if p.inPFB {
		s := &cffStream{data: p.data, pos: int64(len(p.baseDict)) + 6}
		startPos := s.pos
		var total uint64
		for {
			tag, size, err := readPFBTag(s)
			if err != nil {
				return err
			}
			if tag != 0x8002 {
				break
			}
			total += uint64(size)
			if err := s.skip(int64(size)); err != nil {
				return err
			}
		}
		if total == 0 {
			return errInvalidFileFormat
		}
		s.pos = startPos
		priv := make([]byte, 0, total)
		for {
			tag, size, err := readPFBTag(s)
			if err != nil || tag != 0x8002 {
				break
			}
			seg, err := s.read(int64(size))
			if err != nil {
				return err
			}
			priv = append(priv, seg...)
		}
		p.privateDict = priv
	} else {
		root := &p.root
		root.base = p.baseDict
		root.cursor = 0
		root.limit = len(p.baseDict)
		b := p.baseDict
		cur, limit := 0, root.limit
		found := false
		for cur < limit {
			if b[cur] == 'e' && cur+9 < limit {
				if b[cur+1] == 'e' && b[cur+2] == 'x' && b[cur+3] == 'e' && b[cur+4] == 'c' {
					found = true
					break
				}
			}
			root.skipPSToken()
			if root.err != nil {
				break
			}
			root.skipSpaces()
			cur = root.cursor
		}
		if !found {
			return errInvalidFileFormat
		}
		root.limit = len(p.baseDict)
		root.skipPSToken()
		cur = root.cursor
		limit = root.limit

		posLF := -1
		for i := cur; i < limit; i++ {
			if b[i] == '\n' {
				posLF = i
				break
			}
		}
		posCR := -1
		for i := cur; i < limit; i++ {
			if b[i] == '\r' {
				posCR = i
				break
			}
		}
		// pos_lf > memchr('\r'), with a NULL memchr result lowest
		testCR := posLF < 0 || posLF > posCR
		for cur < limit && (b[cur] == ' ' || b[cur] == '\t' || (testCR && b[cur] == '\r') ||
			b[cur] == '\n') {
			cur++
		}
		if cur >= limit {
			return errInvalidFileFormat
		}
		size := len(p.baseDict) - cur
		priv := make([]byte, size+1)
		if cur+3 < limit && isPSXDigit(b[cur]) && isPSXDigit(b[cur+1]) &&
			isPSXDigit(b[cur+2]) && isPSXDigit(b[cur+3]) {
			root.cursor = cur
			n := root.toBytes(priv, uint64(size))
			priv[n] = 0
			p.privateDict = priv[:n]
		} else {
			copy(priv, b[cur:])
			p.privateDict = priv[:size]
		}
	}
	t1Decrypt(p.privateDict, 55665)
	if len(p.privateDict) < 4 {
		return errInvalidFileFormat
	}
	copy(p.privateDict, "    ")
	p.root.base = p.privateDict
	p.root.cursor = 0
	p.root.limit = len(p.privateDict)
	return nil
}

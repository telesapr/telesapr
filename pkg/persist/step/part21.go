// Package step reads STEP files (ISO 10303-21, the "Part 21" exchange
// structure) and turns the boundary representation they hold into a
// triangle mesh that a link group shows as a reference: AP203, AP214
// and AP242 solids, shells and faces on planes, cylinders, cones,
// spheres, tori, B-spline surfaces and swept surfaces. What it can't
// read it counts and names in a Report, and skips.
//
// The reader has three layers. part21.go tokenizes and parses the file
// into instances keyed by their number, knowing nothing of what they
// mean. brep.go maps the topology and geometry entities onto the
// package's own structures, with the units and assembly placements.
// tessellate.go evaluates each face's surface in its own parameters,
// trims it by its loops and triangulates it; it does not go through
// srf.SSurface, so the degree of a surface is not limited.
package step

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Kind is the kind of a parameter value.
type Kind uint8

const (
	// KindOmitted is an unset optional parameter, written $.
	KindOmitted Kind = iota
	// KindDerived is a parameter a subtype redeclares, written *.
	KindDerived
	// KindInteger is a number without a decimal point.
	KindInteger
	// KindReal is a number with a decimal point or an exponent.
	KindReal
	// KindString is a string, decoded from its escapes.
	KindString
	// KindEnum is an enumeration value such as .T., held without dots.
	KindEnum
	// KindBinary is a binary value, held as written, without quotes.
	KindBinary
	// KindRef is a reference to another instance, #n.
	KindRef
	// KindList is a parenthesised list of values.
	KindList
	// KindTyped is a typed parameter such as LENGTH_MEASURE(2.5): Str
	// is the type's name and List its one value.
	KindTyped
)

func (k Kind) String() string {
	switch k {
	case KindOmitted:
		return "$"
	case KindDerived:
		return "*"
	case KindInteger:
		return "integer"
	case KindReal:
		return "real"
	case KindString:
		return "string"
	case KindEnum:
		return "enumeration"
	case KindBinary:
		return "binary"
	case KindRef:
		return "reference"
	case KindList:
		return "list"
	case KindTyped:
		return "typed value"
	}
	return "unknown"
}

// Value is one parameter of a record.
type Value struct {
	Kind Kind
	// Num is the value of an integer or a real.
	Num float64
	// Str is a string's text, an enumeration's name, a binary's digits
	// or a typed value's type name.
	Str string
	// Ref is the instance number a reference names.
	Ref int
	// List holds a list's items, or a typed value's one value.
	List []Value
}

// Record is one entity's name and its parameters. A simple instance has
// one record; a complex instance, written #n=(A()B()...), has one per
// partial entity, in the order written.
type Record struct {
	Name   string
	Params []Value
}

// Instance is one numbered entity instance of the data section.
type Instance struct {
	ID      int
	Records []Record
	// Line is the line the instance starts on.
	Line int
}

// Record is the instance's record named name, or nil. A simple
// instance answers for its one record only.
func (in *Instance) Record(name string) *Record {
	for i := range in.Records {
		if in.Records[i].Name == name {
			return &in.Records[i]
		}
	}
	return nil
}

// Is reports whether the instance has a record named name.
func (in *Instance) Is(name string) bool { return in.Record(name) != nil }

// Name is the instance's entity name: its one record's, or for a
// complex instance the records' names joined with '+'.
func (in *Instance) Name() string {
	if len(in.Records) == 1 {
		return in.Records[0].Name
	}
	names := make([]string, len(in.Records))
	for i := range in.Records {
		names[i] = in.Records[i].Name
	}
	return strings.Join(names, "+")
}

// File is a parsed exchange structure.
type File struct {
	// Header holds the header section's records: FILE_DESCRIPTION,
	// FILE_NAME and FILE_SCHEMA.
	Header []Record
	// Instances are the data section's instances by number.
	Instances map[int]*Instance
	// Order lists the instance numbers in the order written.
	Order []int
}

// Schema is the first schema the header names, upper case, or "".
func (f *File) Schema() string {
	for _, r := range f.Header {
		if r.Name != "FILE_SCHEMA" || len(r.Params) == 0 {
			continue
		}
		l := r.Params[0]
		if l.Kind == KindList && len(l.List) > 0 && l.List[0].Kind == KindString {
			// A schema name may be followed by its object identifier:
			// AUTOMOTIVE_DESIGN { 1 0 10303 214 1 1 1 1 }.
			name := strings.ToUpper(strings.TrimSpace(l.List[0].Str))
			if i := strings.IndexAny(name, " {"); i >= 0 {
				name = name[:i]
			}
			return name
		}
	}
	return ""
}

// Protocol names the application protocol of a schema, such as AP214
// for AUTOMOTIVE_DESIGN, or gives the schema back when it knows none.
func Protocol(schema string) string {
	switch {
	case schema == "CONFIG_CONTROL_DESIGN":
		return "AP203"
	case strings.HasPrefix(schema, "AP203"):
		return "AP203 edition 2"
	case strings.HasPrefix(schema, "AUTOMOTIVE_DESIGN"):
		return "AP214"
	case strings.HasPrefix(schema, "AP242"):
		return "AP242"
	case strings.HasPrefix(schema, "AP209"):
		return "AP209"
	case schema == "":
		return "an unnamed schema"
	}
	return schema
}

// Get is instance n, or nil.
func (f *File) Get(n int) *Instance { return f.Instances[n] }

// SyntaxError is a fault in the exchange structure, with the line it
// is on.
type SyntaxError struct {
	Line int
	Msg  string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

// Parse reads an exchange structure. It returns the first syntax
// error it finds; an instance number used twice is one.
func Parse(data []byte) (*File, error) {
	p := &parser{src: data, line: 1}
	f, err := p.file()
	if err != nil {
		return nil, err
	}
	return f, nil
}

type parser struct {
	src  []byte
	pos  int
	line int
}

func (p *parser) errorf(format string, args ...any) error {
	return &SyntaxError{Line: p.line, Msg: fmt.Sprintf(format, args...)}
}

// skip passes over white space and comments.
func (p *parser) skip() error {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == '\n':
			p.line++
			p.pos++
		case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
			p.pos++
		case c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '*':
			start := p.line
			p.pos += 2
			for {
				if p.pos+1 >= len(p.src) {
					return &SyntaxError{Line: start, Msg: "comment not closed"}
				}
				if p.src[p.pos] == '\n' {
					p.line++
				}
				if p.src[p.pos] == '*' && p.src[p.pos+1] == '/' {
					p.pos += 2
					break
				}
				p.pos++
			}
		default:
			return nil
		}
	}
	return nil
}

// peek is the next significant byte, or 0 at the end.
func (p *parser) peek() (byte, error) {
	if err := p.skip(); err != nil {
		return 0, err
	}
	if p.pos >= len(p.src) {
		return 0, nil
	}
	return p.src[p.pos], nil
}

func (p *parser) expect(c byte) error {
	got, err := p.peek()
	if err != nil {
		return err
	}
	if got != c {
		if got == 0 {
			return p.errorf("expected %q, found the end of the file", c)
		}
		return p.errorf("expected %q, found %q", c, got)
	}
	p.pos++
	return nil
}

func isKeywordStart(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' || c == '!'
}

func isKeywordByte(c byte) bool {
	return isKeywordStart(c) || c >= '0' && c <= '9' || c == '-'
}

// keyword reads a keyword, upper case.
func (p *parser) keyword() (string, error) {
	if _, err := p.peek(); err != nil {
		return "", err
	}
	start := p.pos
	if p.pos >= len(p.src) || !isKeywordStart(p.src[p.pos]) {
		return "", p.errorf("expected a keyword")
	}
	for p.pos < len(p.src) && isKeywordByte(p.src[p.pos]) {
		p.pos++
	}
	return upper(p.src[start:p.pos]), nil
}

// upper is b in upper case, without copying when it already is.
func upper(b []byte) string {
	for _, c := range b {
		if c >= 'a' && c <= 'z' {
			return strings.ToUpper(string(b))
		}
	}
	return string(b)
}

func (p *parser) file() (*File, error) {
	if kw, err := p.keyword(); err != nil || kw != "ISO-10303-21" {
		return nil, p.errorf("not a STEP file: it doesn't start with ISO-10303-21")
	}
	if err := p.expect(';'); err != nil {
		return nil, err
	}
	f := &File{Instances: make(map[int]*Instance)}
	for {
		c, err := p.peek()
		if err != nil {
			return nil, err
		}
		if c == 0 {
			return nil, p.errorf("the file ends before END-ISO-10303-21")
		}
		kw, err := p.keyword()
		if err != nil {
			return nil, err
		}
		switch kw {
		case "HEADER":
			if err := p.expect(';'); err != nil {
				return nil, err
			}
			if err := p.header(f); err != nil {
				return nil, err
			}
		case "DATA":
			// DATA may name its schema: DATA('name', ('schema'));
			if c, err := p.peek(); err != nil {
				return nil, err
			} else if c == '(' {
				if _, err := p.list(); err != nil {
					return nil, err
				}
			}
			if err := p.expect(';'); err != nil {
				return nil, err
			}
			if err := p.data(f); err != nil {
				return nil, err
			}
		case "ANCHOR", "REFERENCE", "SIGNATURE":
			if err := p.skipSection(); err != nil {
				return nil, err
			}
		case "END-ISO-10303-21":
			// What follows the end, such as a signature, is ignored.
			_ = p.expect(';')
			return f, nil
		default:
			return nil, p.errorf("unexpected %s", kw)
		}
	}
}

func (p *parser) header(f *File) error {
	for {
		kw, err := p.keyword()
		if err != nil {
			return err
		}
		if kw == "ENDSEC" {
			return p.expect(';')
		}
		params, err := p.list()
		if err != nil {
			return err
		}
		if err := p.expect(';'); err != nil {
			return err
		}
		f.Header = append(f.Header, Record{Name: kw, Params: params})
	}
}

// skipSection passes over a section this reader doesn't use, to its
// ENDSEC.
func (p *parser) skipSection() error {
	for {
		c, err := p.peek()
		if err != nil {
			return err
		}
		switch {
		case c == 0:
			return p.errorf("section not closed")
		case c == '\'':
			if _, err := p.str(); err != nil {
				return err
			}
		case isKeywordStart(c):
			kw, err := p.keyword()
			if err != nil {
				return err
			}
			if kw == "ENDSEC" {
				return p.expect(';')
			}
		default:
			p.pos++
		}
	}
}

func (p *parser) data(f *File) error {
	for {
		c, err := p.peek()
		if err != nil {
			return err
		}
		if c != '#' {
			kw, err := p.keyword()
			if err != nil {
				return err
			}
			if kw != "ENDSEC" {
				return p.errorf("expected an instance or ENDSEC, found %s", kw)
			}
			return p.expect(';')
		}
		line := p.line
		p.pos++
		id, err := p.integer()
		if err != nil {
			return err
		}
		if err := p.expect('='); err != nil {
			return err
		}
		in := &Instance{ID: id, Line: line}
		c, err = p.peek()
		if err != nil {
			return err
		}
		if c == '(' {
			// A complex instance: a list of partial entities.
			p.pos++
			for {
				c, err := p.peek()
				if err != nil {
					return err
				}
				if c == ')' {
					p.pos++
					break
				}
				r, err := p.record()
				if err != nil {
					return err
				}
				in.Records = append(in.Records, r)
			}
			if len(in.Records) == 0 {
				return p.errorf("#%d has no entities", id)
			}
		} else {
			r, err := p.record()
			if err != nil {
				return err
			}
			in.Records = []Record{r}
		}
		if err := p.expect(';'); err != nil {
			return err
		}
		if _, dup := f.Instances[id]; dup {
			return &SyntaxError{Line: line, Msg: fmt.Sprintf("#%d is defined twice", id)}
		}
		f.Instances[id] = in
		f.Order = append(f.Order, id)
	}
}

func (p *parser) record() (Record, error) {
	kw, err := p.keyword()
	if err != nil {
		return Record{}, err
	}
	params, err := p.list()
	if err != nil {
		return Record{}, err
	}
	return Record{Name: kw, Params: params}, nil
}

// integer reads an unsigned decimal integer, as instance numbers are.
func (p *parser) integer() (int, error) {
	start := p.pos
	for p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9' {
		p.pos++
	}
	if start == p.pos {
		return 0, p.errorf("expected an instance number")
	}
	n, err := strconv.Atoi(string(p.src[start:p.pos]))
	if err != nil {
		return 0, p.errorf("instance number %s is out of range", p.src[start:p.pos])
	}
	return n, nil
}

// maxDepth bounds the nesting of lists, so a hostile file can't
// exhaust the stack.
const maxDepth = 64

// list reads a parenthesised, comma-separated list of values.
func (p *parser) list() ([]Value, error) { return p.listAt(0) }

func (p *parser) listAt(depth int) ([]Value, error) {
	if depth > maxDepth {
		return nil, p.errorf("lists nested too deeply")
	}
	if err := p.expect('('); err != nil {
		return nil, err
	}
	var vals []Value
	c, err := p.peek()
	if err != nil {
		return nil, err
	}
	if c == ')' {
		p.pos++
		return vals, nil
	}
	for {
		v, err := p.value(depth)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
		c, err := p.peek()
		if err != nil {
			return nil, err
		}
		switch c {
		case ',':
			p.pos++
		case ')':
			p.pos++
			return vals, nil
		case 0:
			return nil, p.errorf("list not closed")
		default:
			return nil, p.errorf("expected ',' or ')', found %q", c)
		}
	}
}

func (p *parser) value(depth int) (Value, error) {
	c, err := p.peek()
	if err != nil {
		return Value{}, err
	}
	switch {
	case c == '$':
		p.pos++
		return Value{Kind: KindOmitted}, nil
	case c == '*':
		p.pos++
		return Value{Kind: KindDerived}, nil
	case c == '#':
		p.pos++
		n, err := p.integer()
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindRef, Ref: n}, nil
	case c == '\'':
		s, err := p.str()
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindString, Str: s}, nil
	case c == '"':
		p.pos++
		start := p.pos
		for p.pos < len(p.src) && p.src[p.pos] != '"' {
			p.pos++
		}
		if p.pos >= len(p.src) {
			return Value{}, p.errorf("binary value not closed")
		}
		s := string(p.src[start:p.pos])
		p.pos++
		return Value{Kind: KindBinary, Str: s}, nil
	case c == '.':
		// An enumeration, or a real written without a leading digit.
		if p.pos+1 < len(p.src) && p.src[p.pos+1] >= '0' && p.src[p.pos+1] <= '9' {
			return p.number()
		}
		p.pos++
		start := p.pos
		for p.pos < len(p.src) && isKeywordByte(p.src[p.pos]) {
			p.pos++
		}
		if p.pos >= len(p.src) || p.src[p.pos] != '.' {
			return Value{}, p.errorf("enumeration not closed")
		}
		s := upper(p.src[start:p.pos])
		p.pos++
		return Value{Kind: KindEnum, Str: s}, nil
	case c == '(':
		l, err := p.listAt(depth + 1)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindList, List: l}, nil
	case c == '+' || c == '-' || c >= '0' && c <= '9':
		return p.number()
	case isKeywordStart(c):
		kw, err := p.keyword()
		if err != nil {
			return Value{}, err
		}
		l, err := p.listAt(depth + 1)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: KindTyped, Str: kw, List: l}, nil
	case c == 0:
		return Value{}, p.errorf("the file ends inside a list")
	}
	return Value{}, p.errorf("unexpected %q", c)
}

func (p *parser) number() (Value, error) {
	start := p.pos
	if p.src[p.pos] == '+' || p.src[p.pos] == '-' {
		p.pos++
	}
	real := false
	digits := 0
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c >= '0' && c <= '9':
			digits++
		case c == '.':
			real = true
		case c == 'E' || c == 'e':
			real = true
			if p.pos+1 < len(p.src) && (p.src[p.pos+1] == '+' || p.src[p.pos+1] == '-') {
				p.pos++
			}
		default:
			goto done
		}
		p.pos++
	}
done:
	text := string(p.src[start:p.pos])
	if digits == 0 {
		return Value{}, p.errorf("bad number %q", text)
	}
	if !real {
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			// Too long for an integer: keep it as a real.
			fl, ferr := strconv.ParseFloat(text, 64)
			if ferr != nil {
				return Value{}, p.errorf("bad number %q", text)
			}
			return Value{Kind: KindReal, Num: fl}, nil
		}
		return Value{Kind: KindInteger, Num: float64(n)}, nil
	}
	// Part 21 allows "1." and "1.E3", which ParseFloat takes as they
	// are.
	fl, err := strconv.ParseFloat(text, 64)
	if err != nil {
		var ne *strconv.NumError
		if errors.As(err, &ne) && errors.Is(ne.Err, strconv.ErrRange) {
			return Value{Kind: KindReal, Num: fl}, nil
		}
		return Value{}, p.errorf("bad number %q", text)
	}
	return Value{Kind: KindReal, Num: fl}, nil
}

// str reads a quoted string and decodes it: ” is a quote, \X2\...\X0\
// and \X4\...\X0\ are UTF-16 and UTF-32 code units in hex, \X\hh is one
// ISO 8859-1 byte, \S\c is c with its top bit set, \P?\ picks a code
// page and is dropped, and \\ is a backslash.
func (p *parser) str() (string, error) {
	start := p.line
	p.pos++ // the opening quote
	var b strings.Builder
	for {
		if p.pos >= len(p.src) {
			return "", &SyntaxError{Line: start, Msg: "string not closed"}
		}
		c := p.src[p.pos]
		switch c {
		case '\'':
			if p.pos+1 < len(p.src) && p.src[p.pos+1] == '\'' {
				b.WriteByte('\'')
				p.pos += 2
				continue
			}
			p.pos++
			return b.String(), nil
		case '\\':
			p.escape(&b)
		case '\n':
			// Lines break inside long strings; the break isn't text.
			p.line++
			p.pos++
		case '\r':
			p.pos++
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
}

// escape decodes the control directive at p.pos into b. One that isn't
// recognised is kept as written.
func (p *parser) escape(b *strings.Builder) {
	rest := p.src[p.pos:]
	hexVal := func(s []byte) (uint32, bool) {
		v, err := strconv.ParseUint(string(s), 16, 32)
		return uint32(v), err == nil
	}
	switch {
	case len(rest) >= 2 && rest[1] == '\\':
		b.WriteByte('\\')
		p.pos += 2
		return
	case len(rest) >= 4 && rest[1] == 'S' && rest[2] == '\\':
		b.WriteRune(rune(rest[3]) | 0x80)
		p.pos += 4
		return
	case len(rest) >= 4 && rest[1] == 'P' && rest[3] == '\\':
		p.pos += 4
		return
	case len(rest) >= 5 && rest[1] == 'X' && rest[2] == '\\':
		if v, ok := hexVal(rest[3:5]); ok {
			b.WriteRune(rune(v))
			p.pos += 5
			return
		}
	case len(rest) >= 4 && rest[1] == 'X' && (rest[2] == '2' || rest[2] == '4') && rest[3] == '\\':
		width := 4
		if rest[2] == '4' {
			width = 8
		}
		end := strings.Index(string(rest[4:]), `\X0\`)
		if end >= 0 && end%width == 0 {
			units := rest[4 : 4+end]
			var u16 []uint16
			for i := 0; i < len(units); i += width {
				v, ok := hexVal(units[i : i+width])
				if !ok {
					u16 = nil
					break
				}
				if width == 4 {
					u16 = append(u16, uint16(v))
				} else if utf8.ValidRune(rune(v)) {
					b.WriteRune(rune(v))
				}
			}
			for _, r := range utf16.Decode(u16) {
				b.WriteRune(r)
			}
			p.pos += 4 + end + 4
			return
		}
	}
	b.WriteByte('\\')
	p.pos++
}

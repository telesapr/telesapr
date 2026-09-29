// Package res holds SolveSpace's bundled resources (icons and fonts)
// and the types that load them: Pixmap, BitmapFont and VectorFont. It
// ports resource.h and resource.cpp, without gettext.
package res

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"strings"
)

// The files are copies of solvespace/res: icons/graphics-window,
// icons/text-window, fonts/private, fonts/unifont.hex.gz and
// fonts/unicode.lff.gz.
//
//go:embed data
var data embed.FS

// LoadResource returns the resource at name, a path such as
// "icons/graphics-window/line.png" (C++ Platform::LoadResource).
func LoadResource(name string) ([]byte, error) {
	b, err := data.ReadFile("data/" + name)
	if err != nil {
		return nil, fmt.Errorf("cannot read resource %s: %w", name, err)
	}
	return b, nil
}

func mustLoadResource(name string) []byte {
	b, err := LoadResource(name)
	if err != nil {
		panic(err)
	}
	return b
}

// LoadString returns a text resource without carriage returns (C++
// LoadString).
func LoadString(name string) string {
	return strings.ReplaceAll(string(mustLoadResource(name)), "\r", "")
}

// LoadStringFromGzip returns a gzipped text resource, inflated (C++
// LoadStringFromGzip).
func LoadStringFromGzip(name string) string {
	zr, err := gzip.NewReader(bytes.NewReader(mustLoadResource(name)))
	if err != nil {
		panic(fmt.Errorf("cannot inflate resource %s: %w", name, err))
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		panic(fmt.Errorf("cannot inflate resource %s: %w", name, err))
	}
	return string(b)
}

// LoadPng returns a PNG resource as a pixmap (C++ LoadPng).
func LoadPng(name string) *Pixmap {
	pm, err := FromPng(mustLoadResource(name), false)
	if err != nil {
		panic(fmt.Errorf("cannot load pixmap %s: %w", name, err))
	}
	return pm
}

// asciiReader is C++'s ASCIIReader over a string.
type asciiReader struct {
	s   string
	pos int
}

func (r *asciiReader) atEnd() bool { return r.pos >= len(r.s) }

func (r *asciiReader) readChar() byte {
	if r.atEnd() {
		panic("res: unexpected EOF")
	}
	c := r.s[r.pos]
	r.pos++
	return c
}

func (r *asciiReader) tryChar(c byte) bool {
	if !r.atEnd() && r.s[r.pos] == c {
		r.pos++
		return true
	}
	return false
}

func (r *asciiReader) expectChar(c byte) {
	if !r.tryChar(c) {
		panic(fmt.Sprintf("res: expecting character %q at %d", c, r.pos))
	}
}

func (r *asciiReader) countUntilEol() int {
	if i := strings.IndexByte(r.s[r.pos:], '\n'); i >= 0 {
		return i
	}
	return len(r.s) - r.pos
}

func (r *asciiReader) skipUntilEol() {
	r.pos += r.countUntilEol()
}

func (r *asciiReader) read4HexBits() uint8 {
	c := r.readChar()
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return 10 + c - 'a'
	case c >= 'A' && c <= 'F':
		return 10 + c - 'A'
	}
	panic("res: unexpected hex digit")
}

func (r *asciiReader) read8HexBits() uint8 {
	h := r.read4HexBits()
	l := r.read4HexBits()
	return h<<4 + l
}

func (r *asciiReader) read16HexBits() uint16 {
	h := uint16(r.read8HexBits())
	l := uint16(r.read8HexBits())
	return h<<8 + l
}

// readFloatDecimal reads a number as strtod does; the fonts only use
// plain decimals.
func (r *asciiReader) readFloatDecimal() float64 {
	n, v := parseDecimalPrefix(r.s[r.pos:])
	if n == 0 {
		panic("res: cannot read a floating-point number")
	}
	r.pos += n
	return v
}

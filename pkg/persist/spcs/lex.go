// Package spcs reads and writes spacesolve's own file format, .spcs,
// described in docs/spcs-format.md.
//
// The syntax layer (lex.go, parse.go, print.go) turns text into a tree
// of statements and back, and knows nothing about sketches. The model
// layer (encode.go, decode.go) maps that tree to a sketch.Sketch.
package spcs

import (
	"fmt"
	"strconv"
	"strings"
)

// Pos is a place in a file: a line and a column, both from 1. The
// column counts bytes.
type Pos struct {
	Line, Col int
}

func (p Pos) String() string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

// Error is a syntax error, or a model error found while decoding, at a
// place in the file.
type Error struct {
	Pos Pos
	Msg string
}

func (e *Error) Error() string { return e.Pos.String() + ": " + e.Msg }

type tokenKind int

const (
	tokWord tokenKind = iota
	tokString
	tokOpen  // {
	tokClose // }
	tokExpr  // = and the rest of the line
)

type token struct {
	kind tokenKind
	pos  Pos
	text string // a word, an unquoted string, or an expression
}

// isWordByte reports whether c may be part of a word.
func isWordByte(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n', '{', '}', '"', '=', '#':
		return false
	}
	return true
}

// lexLine splits one line, without its line end, into tokens.
func lexLine(line string, lineNo int) ([]token, error) {
	var toks []token
	i := 0
	for i < len(line) {
		c := line[i]
		pos := Pos{lineNo, i + 1}
		switch c {
		case ' ', '\t':
			i++
		case '#':
			return toks, nil
		case '{':
			toks = append(toks, token{kind: tokOpen, pos: pos})
			i++
		case '}':
			toks = append(toks, token{kind: tokClose, pos: pos})
			i++
		case '=':
			rest := line[i+1:]
			if h := strings.IndexByte(rest, '#'); h >= 0 {
				rest = rest[:h]
			}
			expr := strings.TrimSpace(rest)
			if expr == "" {
				return nil, &Error{pos, "expected an expression after '='"}
			}
			return append(toks, token{kind: tokExpr, pos: pos, text: expr}), nil
		case '"':
			j := i + 1
			for j < len(line) && line[j] != '"' {
				if line[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(line) {
				return nil, &Error{pos, "string not closed on its line"}
			}
			s, err := strconv.Unquote(line[i : j+1])
			if err != nil {
				return nil, &Error{pos, "bad string " + line[i:j+1]}
			}
			toks = append(toks, token{kind: tokString, pos: pos, text: s})
			i = j + 1
		case '\r':
			if i != len(line)-1 {
				return nil, &Error{pos, "carriage return inside a line"}
			}
			i++
		default:
			j := i
			for j < len(line) && isWordByte(line[j]) {
				j++
			}
			toks = append(toks, token{kind: tokWord, pos: pos, text: line[i:j]})
			i = j
		}
	}
	return toks, nil
}

package spcs

import (
	"strings"
)

// Value is one value of a statement: a word, or a string when Quoted.
type Value struct {
	Pos    Pos
	Text   string
	Quoted bool
}

// Word makes an unquoted value.
func Word(s string) Value { return Value{Text: s} }

// String makes a quoted value.
func String(s string) Value { return Value{Text: s, Quoted: true} }

// Node is a statement: a field, a field with an expression, or a block
// with statements inside.
type Node struct {
	Pos  Pos
	Name string
	Args []Value

	// HasExpr is set for "name = expression", with the expression's
	// text, trimmed, in Expr, and its place in ExprPos.
	HasExpr bool
	Expr    string
	ExprPos Pos

	// Block is set for "name values { ... }", with what is inside in
	// Children.
	Block    bool
	Children []*Node
}

// File is a parsed file: its statements in order, the header first.
type File struct {
	Nodes []*Node
}

// Parse reads the syntax of a file. The first statement must be the
// header, "spacesolve <major>", but its version is not checked here.
func Parse(data []byte) (*File, error) {
	f := &File{}
	type open struct {
		node *Node
	}
	var stack []open
	add := func(n *Node) {
		if len(stack) == 0 {
			f.Nodes = append(f.Nodes, n)
		} else {
			top := stack[len(stack)-1].node
			top.Children = append(top.Children, n)
		}
	}

	text := string(data)
	lineNo := 0
	for len(text) > 0 || lineNo == 0 {
		lineNo++
		line := text
		if nl := strings.IndexByte(text, '\n'); nl >= 0 {
			line, text = text[:nl], text[nl+1:]
		} else {
			text = ""
		}
		toks, err := lexLine(line, lineNo)
		if err != nil {
			return nil, err
		}
		if len(toks) == 0 {
			continue
		}

		first := toks[0]
		if first.kind == tokClose {
			if len(toks) > 1 {
				return nil, &Error{toks[1].pos, "'}' must stand on a line of its own"}
			}
			if len(stack) == 0 {
				return nil, &Error{first.pos, "'}' with no block open"}
			}
			stack = stack[:len(stack)-1]
			continue
		}
		if first.kind != tokWord {
			return nil, &Error{first.pos, "a statement must start with a name"}
		}
		if len(f.Nodes) == 0 && len(stack) == 0 && first.text != "spacesolve" {
			return nil, &Error{first.pos, "not a spacesolve file: the first statement must be 'spacesolve <version>'"}
		}

		n := &Node{Pos: first.pos, Name: first.text}
		rest := toks[1:]
		for len(rest) > 0 {
			t := rest[0]
			rest = rest[1:]
			switch t.kind {
			case tokWord:
				n.Args = append(n.Args, Value{Pos: t.pos, Text: t.text})
			case tokString:
				n.Args = append(n.Args, Value{Pos: t.pos, Text: t.text, Quoted: true})
			case tokExpr:
				n.HasExpr, n.Expr, n.ExprPos = true, t.text, t.pos
			case tokOpen:
				if len(rest) > 0 {
					return nil, &Error{rest[0].pos, "nothing may follow '{' on its line"}
				}
				n.Block = true
			case tokClose:
				return nil, &Error{t.pos, "'}' must stand on a line of its own"}
			}
		}
		if len(f.Nodes) == 0 && len(stack) == 0 && n.Block {
			return nil, &Error{n.Pos, "the header is not a block"}
		}
		add(n)
		if n.Block {
			stack = append(stack, open{n})
		}
	}
	if len(stack) > 0 {
		n := stack[len(stack)-1].node
		return nil, &Error{n.Pos, "block '" + n.Name + "' is not closed"}
	}
	if len(f.Nodes) == 0 {
		return nil, &Error{Pos{1, 1}, "the file is empty"}
	}
	return f, nil
}

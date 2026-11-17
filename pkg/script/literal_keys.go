package script

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"slices"
	"strconv"
	"strings"

	"github.com/traefik/yaegi/interp"
)

// The interpreter (yaegi 0.16.1) works out the type of each element of
// a composite literal written without a key before it compiles the
// element, and the indexing of a slice whose type is one of compiled
// Go, as one package api or os returns, has no type there. So
// "[]api.Entity{tl[0].Start()}", with tl from doc.Sketch.Polyline,
// panics with a nil pointer while it is compiled: the method's receiver,
// tl[0], has no type to look the method up on. An element with a key,
// "[]api.Entity{0: tl[0].Start()}", is compiled the way any other
// expression is, and works. Go gives a slice or an array literal's
// element written without a key the index after the one before, so
// keying such an element with that index means the same and evaluates
// the same, in the same order: nothing is moved out of the literal. The
// elements of a struct, or of a literal whose type isn't spelled out as
// a slice, an array or a map, can't be keyed so: those are left as they
// are, and said when the interpreter fails on them.

// literalKeys is what keyElements did to source.
type literalKeys struct {
	// src is the source with the keys put in.
	src string
	// inserts are the keys, at offsets of the source as it was.
	inserts []insert
	// left are the elements that need a key and couldn't be given one,
	// at offsets of the source as it was, in order.
	left []element
}

// element is an element of a composite literal in source.
type element struct {
	at   int
	text string
}

// keyElements gives each element of a slice or an array literal that
// the interpreter can't compile without a key, an element that calls a
// method on or reads a field of an indexed value, the key it has
// anyway. Source that doesn't parse is returned as it is.
func keyElements(src string) literalKeys {
	out := literalKeys{src: src}
	if !strings.Contains(src, "{") || !strings.Contains(src, "[") {
		return out
	}
	f, offset, ok := parseAny(src)
	if !ok {
		return out
	}
	text := func(n ast.Node) string { return src[offset(n.Pos()):offset(n.End())] }
	// The type of each literal whose type is left out, as the inner
	// ones of "[][]float64{{1, 2}}", from the literal it is in.
	elided := map[*ast.CompositeLit]ast.Expr{}
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		typ := lit.Type
		if typ == nil {
			typ = elided[lit]
		}
		typ = unparen(typ)
		inner := func(x ast.Expr, t ast.Expr) {
			if l, ok := x.(*ast.CompositeLit); ok && l.Type == nil {
				if star, ok := unparen(t).(*ast.StarExpr); ok {
					// {...} in a []*T is &T{...}.
					t = star.X
				}
				elided[l] = t
			}
		}
		switch t := typ.(type) {
		case *ast.ArrayType:
			base, n := "", 0
			for _, x := range lit.Elts {
				if kv, ok := x.(*ast.KeyValueExpr); ok {
					inner(kv.Value, t.Elt)
					base, n = text(kv.Key), 1
					continue
				}
				inner(x, t.Elt)
				switch {
				case !needsKey(x):
				case strings.Contains(base, "\n"):
					// A key put in holds no line break, so that no line moves.
					out.left = append(out.left, element{at: offset(x.Pos()), text: text(x)})
				default:
					out.inserts = append(out.inserts, insert{at: offset(x.Pos()), text: keyText(base, n)})
				}
				n++
			}
			return true
		case *ast.MapType:
			for _, x := range lit.Elts {
				if kv, ok := x.(*ast.KeyValueExpr); ok {
					inner(kv.Key, t.Key)
					inner(kv.Value, t.Value)
				}
			}
			return true
		}
		for _, x := range lit.Elts {
			if _, isKV := x.(*ast.KeyValueExpr); !isKV && needsKey(x) {
				out.left = append(out.left, element{at: offset(x.Pos()), text: text(x)})
			}
		}
		return true
	})
	if len(out.inserts) == 0 {
		return out
	}
	slices.SortStableFunc(out.inserts, func(a, b insert) int { return a.at - b.at })
	s := src
	for _, in := range slices.Backward(out.inserts) {
		s = s[:in.at] + in.text + s[in.at:]
	}
	out.src = s
	return out
}

// needsKey reports whether x, an element of a composite literal without
// a key, is one the interpreter can't compile without one: the type it
// works out for the element goes through an index expression, as
// "tl[0].Start()", "fs[0].Name" or "(*ps[0]).X" do. Its search follows
// what the type of an expression depends on: a call's function (and
// append's first argument), a selector's value, and what parentheses, a
// dereference, a slice or an index hold. An index expression that is
// the element itself is compiled as it is, and is no trouble.
func needsKey(x ast.Expr) bool {
	for top := true; ; top = false {
		switch e := x.(type) {
		case *ast.CallExpr:
			if id, ok := e.Fun.(*ast.Ident); ok && id.Name == "append" && len(e.Args) > 0 {
				x = e.Args[0]
			} else {
				x = e.Fun
			}
		case *ast.SelectorExpr:
			x = e.X
		case *ast.ParenExpr:
			x = e.X
		case *ast.StarExpr:
			x = e.X
		case *ast.SliceExpr:
			x = e.X
		case *ast.IndexExpr:
			return !top
		default:
			return false
		}
	}
}

// unparen is x without the parentheses round it.
func unparen(x ast.Expr) ast.Expr {
	for {
		p, ok := x.(*ast.ParenExpr)
		if !ok {
			return x
		}
		x = p.X
	}
}

// keyText is the key put before an element: its index, or the key of
// the last keyed element before it and how far on it is.
func keyText(base string, n int) string {
	if base == "" {
		return strconv.Itoa(n) + ": "
	}
	return "(" + base + ") + " + strconv.Itoa(n) + ": "
}

// errCompileFault is the interpreter failing while it compiles a
// script: a fault of the interpreter's, not of the script.
var errCompileFault = errors.New("the interpreter (yaegi) failed while compiling")

// compiling reports whether p is a panic of the interpreter's while it
// compiled source, before any of it ran. A panic of the script itself,
// or of what it calls, is in what the interpreter runs after.
func compiling(p interp.Panic) bool {
	return bytes.Contains(p.Stack, []byte(interpPath+".(*Interpreter).CompileAST("))
}

// compileFault is the error for the interpreter failing with what
// while it compiled src: at an element of a literal that it can't
// compile and that couldn't be keyed, with what gets round it, or else
// without a place, which the interpreter doesn't say.
func compileFault(src string, what error) *Error {
	keys := keyElements(src)
	if len(keys.left) == 0 {
		return &Error{Err: fmt.Errorf("%w the script (%w), which is no fault of the script's; "+
			"writing the statement another way, as with a value put in a variable first, may get round it",
			errCompileFault, what)}
	}
	el := keys.left[0]
	text := strings.Join(strings.Fields(el.text), " ")
	if r := []rune(text); len(r) > 60 {
		text = string(r[:59]) + "…"
	}
	line := 1 + strings.Count(src[:el.at], "\n")
	col := el.at - strings.LastIndex(src[:el.at], "\n")
	return &Error{Line: line, Col: col, Err: fmt.Errorf(
		"%w %s as an element of a literal (%w); put it in a variable first and write the variable in the literal, as v := %s",
		errCompileFault, text, what, text)}
}

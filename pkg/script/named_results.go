package script

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

// zeroNamedResults works round two faults of the interpreter (yaegi
// 0.16.1) with the results a function names. It gives a call's results
// the places of the variables the caller assigns them to, with what
// those hold: a named result that a call leaves alone returns what was
// there, so "for ... { v := f(x) }" sees the v of the iteration before.
// Each function of src that names its results sets them to their zero
// values first, as Go does on entry, in statements after the opening
// brace of its body, on its line, so no line of src moves. And the
// interpreter builds a composite literal assigned to a named result,
// "r = T{...}", in a place of its own, and the caller gets the zero
// value: in parentheses, which mean nothing to Go, the literal is
// assigned as any other value is. Source that doesn't parse is
// returned as it is, for the interpreter to say what is wrong.
func zeroNamedResults(src string) string {
	if !strings.Contains(src, "func") {
		return src
	}
	f, offset, ok := parseAny(src)
	if !ok {
		return src
	}
	type insert struct {
		at   int
		text string
	}
	var inserts []insert
	ast.Inspect(f, func(n ast.Node) bool {
		var typ *ast.FuncType
		var body *ast.BlockStmt
		switch fn := n.(type) {
		case *ast.FuncDecl:
			typ, body = fn.Type, fn.Body
		case *ast.FuncLit:
			typ, body = fn.Type, fn.Body
		default:
			return true
		}
		if body == nil || typ.Results == nil {
			return true
		}
		var names, zeros []string
		for _, field := range typ.Results.List {
			t := src[offset(field.Type.Pos()):offset(field.Type.End())]
			for _, name := range field.Names {
				if name.Name == "_" {
					continue
				}
				names = append(names, name.Name)
				zeros = append(zeros, "*new("+t+")")
			}
		}
		if len(names) == 0 {
			return true
		}
		inserts = append(inserts, insert{
			at:   offset(body.Lbrace) + 1,
			text: " " + strings.Join(names, ", ") + " = " + strings.Join(zeros, ", ") + ";",
		})
		// In closures too, which assign the results as the body does; a
		// name of a closure's own is wrapped as well, for nothing.
		ast.Inspect(body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != len(as.Rhs) {
				return true
			}
			for i, lhs := range as.Lhs {
				id, ok := lhs.(*ast.Ident)
				lit, isLit := as.Rhs[i].(*ast.CompositeLit)
				if ok && isLit && slices.Contains(names, id.Name) {
					inserts = append(inserts, insert{at: offset(lit.Pos()), text: "("}, insert{at: offset(lit.End()), text: ")"})
				}
			}
			return true
		})
		return true
	})
	if len(inserts) == 0 {
		return src
	}
	// From the end, so that each offset still holds.
	slices.SortFunc(inserts, func(a, b insert) int { return b.at - a.at })
	for _, in := range inserts {
		src = src[:in.at] + in.text + src[in.at:]
	}
	return src
}

// parseAny parses src as what the interpreter takes: a file, with a
// package clause, declarations, or statements. offset gives the offset
// in src of a position in the result.
func parseAny(src string) (f *ast.File, offset func(token.Pos) int, ok bool) {
	for _, wrap := range [][2]string{{"", ""}, {"package p\n", ""}, {"package p\nfunc _() {\n", "\n}"}} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "", wrap[0]+src+wrap[1], parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		n := len(wrap[0])
		return f, func(p token.Pos) int { return fset.Position(p).Offset - n }, true
	}
	return nil, nil, false
}

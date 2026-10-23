package script

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
)

// The interpreter (yaegi 0.16.1) says a panic of a whole program was at
// the first statement of the function it happened in, and then at that
// of each function the panic leaves, the outermost last: a script that
// fails in main is reported at "doc := Current()". So a program is run
// with every function telling the engine where it is. Each function
// enters a frame and leaves it by a deferred call, and each statement
// says where it starts in its frame. A deferred call runs before the
// interpreter writes about the panic, so the frame left last before
// the first such line is the one the panic was in: a statement that
// failed after a call returned is found in its own frame, not in the
// called one.

// place is where a statement of a script starts, from 1; line 0 is
// none.
type place struct{ line, col int }

// insert is text put into source at an offset of it.
type insert struct {
	at   int
	text string
}

// placeStatements returns src, a whole program, with its functions
// telling the engine where they are, and the inserts made. It moves no
// line. Source that doesn't parse, or has no function, is returned as
// it is.
func placeStatements(src string) (string, []insert) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return src, nil
	}
	offset := func(p token.Pos) int { return fset.Position(p).Offset }
	var inserts []insert
	statements := func(list []ast.Stmt) {
		for _, s := range list {
			if _, empty := s.(*ast.EmptyStmt); empty {
				continue
			}
			p := fset.Position(s.Pos())
			inserts = append(inserts, insert{
				at:   p.Offset,
				text: "telesaprConsole.At(telesaprFrame, " + strconv.Itoa(p.Line) + ", " + strconv.Itoa(p.Column) + "); ",
			})
		}
	}
	// The bodies of switches and selects list clauses, not statements.
	clauses := map[*ast.BlockStmt]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SwitchStmt:
			clauses[n.Body] = true
		case *ast.TypeSwitchStmt:
			clauses[n.Body] = true
		case *ast.SelectStmt:
			clauses[n.Body] = true
		case *ast.FuncDecl:
			if n.Body != nil {
				inserts = append(inserts, insert{at: offset(n.Body.Lbrace) + 1, text: frameEntry})
			}
		case *ast.FuncLit:
			inserts = append(inserts, insert{at: offset(n.Body.Lbrace) + 1, text: frameEntry})
		case *ast.BlockStmt:
			if !clauses[n] {
				statements(n.List)
			}
		case *ast.CaseClause:
			statements(n.Body)
		case *ast.CommClause:
			statements(n.Body)
		}
		return true
	})
	if len(inserts) == 0 {
		return src, nil
	}
	// The import goes on the package clause's line.
	inserts = append(inserts, insert{at: offset(f.Name.End()), text: "; import telesaprConsole \"" + showPath + "\""})
	// In order, a function's entry before its first statement's place.
	slices.SortStableFunc(inserts, func(a, b insert) int { return a.at - b.at })
	out := src
	for _, in := range slices.Backward(inserts) {
		out = out[:in.at] + in.text + out[in.at:]
	}
	return out, inserts
}

// frameEntry is what a function of a placed program starts with.
const frameEntry = " telesaprFrame := telesaprConsole.Enter(); defer telesaprConsole.Leave(telesaprFrame);"

// unplace is where a place in source with inserts put into it is in
// the source as it was: what was put in before it on its line is taken
// off its column. No insert holds a line break, so lines don't move.
func unplace(src string, inserts []insert, line, col int) (int, int) {
	start := 0 // the offset in src of the line's start
	for range line - 1 {
		i := strings.IndexByte(src[start:], '\n')
		if i < 0 {
			return line, col
		}
		start += i + 1
	}
	// Walk the line as it was put together, an insert at a time.
	at := start     // offset in src
	moved := 0      // characters put in before at
	left := col - 1 // characters of the placed line still to go
	for _, in := range inserts {
		if in.at < start {
			continue
		}
		if in.at-at > left {
			break
		}
		left -= in.at - at
		at = in.at
		if left < len(in.text) {
			// In the inserted text itself: say its place in src.
			return line, at - start + 1
		}
		left -= len(in.text)
		moved += len(in.text)
	}
	return line, col - moved
}

// enterFrame is the console's Enter for a placed program: a new frame,
// whose number its statements and its leaving give.
func (e *Engine) enterFrame() int {
	e.told.Lock()
	defer e.told.Unlock()
	e.frames = append(e.frames, place{})
	return len(e.frames) - 1
}

// atStatement is the console's At: the statement at line and col of
// frame starts.
func (e *Engine) atStatement(frame, line, col int) {
	e.told.Lock()
	defer e.told.Unlock()
	if frame < len(e.frames) {
		e.frames[frame] = place{line, col}
	}
}

// leaveFrame is the console's Leave: the function of frame returns or
// panics, and the statement it was at is the one a panic was at if the
// interpreter writes of one next.
func (e *Engine) leaveFrame(frame int) {
	e.told.Lock()
	defer e.told.Unlock()
	if frame < len(e.frames) {
		e.left = e.frames[frame]
		e.frames = e.frames[:frame]
	}
}

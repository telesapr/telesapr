package script

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// TestErrorColumns checks that the line and column an error is given
// point into what the user typed, through each rewrite the engine
// makes before it compiles: the wrapper a console shows an expression
// through, the keys put into composite literals, the statements put
// into functions that name their results, and the prefix the
// interpreter itself puts before what isn't a file. Each case runs as
// a console shows it (show), as a statement (run) and as the engine
// evaluates it (eval).
func TestErrorColumns(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		src  string
		// at is the text the error points at, by its first occurrence.
		at string
		// showAt is where an error of the script's running is placed when
		// the expression runs inside the console's wrapper: the
		// interpreter says the wrapper's call, which is where src starts,
		// where it says the node itself outside it. Without it, it is at.
		showAt string
		// message is what the error says.
		message string
	}{
		{name: "compile error at start", src: "undefinedName + 1", at: "undefinedName", message: "undefined"},
		{name: "compile error after an operand", src: "1 + undefinedName", at: "undefinedName", message: "undefined"},
		{name: "compile error in a call", src: "fmt.Println(undefinedName)", at: "undefinedName", message: "undefined"},
		{name: "compile error in a function literal", src: "func() int { return undefinedName }()", at: "undefinedName", message: "undefined"},
		{name: "compile error in a literal with a key", src: "len([]api.Entity{tl[0].Start(), tl[1].Start()}) + undefinedName", at: "undefinedName", message: "undefined"},
		{name: "compile error after a named result", src: "func() (r int) { return 1 }() + undefinedName", at: "undefinedName", message: "undefined"},
		{name: "compile error after a literal in a named result", src: "func() (r []int) { r = []int{1}; return }()[0] + undefinedName", at: "undefinedName", message: "undefined"},
		{name: "compile fault of an undefined name in a literal in a literal", src: "[]int{1, undefinedName}", at: "undefinedName", message: "undefined: undefinedName"},
		{name: "compile fault of an undefined name in a keyed literal", src: "struct{ E []int }{E: []int{undefinedName}}", at: "undefinedName", message: "undefined: undefinedName"},
		{name: "panic in a function that names its results", src: "func() (r int) { return []int{1}[zero+5] }()", at: "func", message: "index out of range"},
		{name: "panic in an expression", src: "1 + []int{1}[zero+5]", at: "[]int", showAt: "1", message: "index out of range"},
		{name: "panic in a literal with a key", src: "[]api.Entity{tl[0].Start(), tl[9].Start()}", at: "tl[0]", showAt: "[]api", message: "index out of range"},
		{name: "panic in a literal with a key as an argument", src: "len([]api.Entity{tl[0].Start(), tl[9].Start()})", at: "tl[0]", showAt: "len", message: "index out of range"},
	} {
		for _, mode := range []string{"show", "run", "eval"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				t.Parallel()
				e, err := New(Options{Host: newHost(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
				if err != nil {
					t.Fatal(err)
				}
				ctx := context.Background()
				if _, err := e.Run(ctx, "setup", "zero := 0\ntl := doc.Sketch.Polyline(false, V(0, 0, 0), V(10, 0, 0), V(10, 5, 0))"); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "show":
					_, err = e.show(ctx, c.src)
				case "run":
					err = e.run(ctx, c.src)
				default:
					_, err = e.eval(ctx, c.src)
				}
				serr, ok := errors.AsType[*Error](err)
				if !ok || !strings.Contains(serr.Err.Error(), c.message) {
					t.Fatalf("got %v, want an error that says %q", err, c.message)
				}
				at := c.at
				if mode == "show" && c.showAt != "" {
					at = c.showAt
				}
				if want := strings.Index(c.src, at) + 1; serr.Line != 1 || serr.Col != want {
					t.Errorf("%s of %q is placed at %d:%d, want 1:%d", mode, c.src, serr.Line, serr.Col, want)
				}
			})
		}
	}
}

// TestErrorColumnsFed checks the same through Feed, which a console
// and the command line's repl enter what is typed with: a statement of
// several lines keeps the place, on its first line too, and one line
// gives the message alone.
func TestErrorColumnsFed(t *testing.T) {
	t.Parallel()
	e, err := New(Options{Host: newHost(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	feed := func(lines ...string) error {
		var err error
		for _, l := range lines {
			if _, _, err = e.Feed(ctx, l); err != nil {
				break
			}
		}
		return err
	}
	first := "fmt.Sprint(1, undefinedName,"
	err = feed(first, "2)")
	serr, ok := errors.AsType[*Error](err)
	if want := strings.Index(first, "undefinedName") + 1; !ok || serr.Line != 1 || serr.Col != want || !strings.Contains(serr.Err.Error(), "undefined") {
		t.Errorf("a call over two lines: %v, want 1:%d: undefined", err, want)
	}
	err = feed("fmt.Sprint(1,", " undefinedName)")
	if serr, ok = errors.AsType[*Error](err); !ok || serr.Line != 2 || serr.Col != 2 {
		t.Errorf("a call over two lines, the second at fault: %v, want 2:2", err)
	}
	err = feed("1 + undefinedName")
	if _, ok = errors.AsType[*Error](err); ok || err == nil || !strings.HasPrefix(err.Error(), "undefined:") {
		t.Errorf("one line: %v, want the message alone", err)
	}
}

// TestStripWrapper checks that a place the interpreter gives inside
// what an expression is wrapped in comes out in the expression: a
// column on the first line loses the wrapper, one in the wrapper is the
// start, and a later line is left alone.
func TestStripWrapper(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		line, col, n, wantCol int
	}{
		{1, 22, 21, 1},
		{1, 40, 21, 19},
		{1, 5, 21, 1},
		{2, 7, 21, 7},
	} {
		err := stripWrapper(&Error{Line: c.line, Col: c.col, Err: errors.New("x")}, c.n)
		if serr, _ := errors.AsType[*Error](err); serr == nil || serr.Col != c.wantCol {
			t.Errorf("%d:%d less %d is %d, want %d", c.line, c.col, c.n, serr.Col, c.wantCol)
		}
	}
}

// A compile fault of the interpreter's that says nowhere it is, taken
// by a console or Eval, is at the start of the statement, as it is in a
// script run.
func TestAtStatement(t *testing.T) {
	t.Parallel()
	err := atStatement(&Error{Err: errCompileFault})
	if serr, ok := errors.AsType[*Error](err); !ok || serr.Line != 1 || serr.Col != 1 {
		t.Errorf("a compile fault is placed at %v, want 1:1", err)
	}
	other := &Error{Err: errors.New("x")}
	if got := atStatement(other); !errors.Is(got, other) || other.Line != 0 {
		t.Errorf("another error is placed at line %d, want none", other.Line)
	}
}

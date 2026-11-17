package script

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// TestKeyElements checks which elements get a key, and that the key is
// the index Go gives the element.
func TestKeyElements(t *testing.T) {
	t.Parallel()
	cases := []struct{ src, want, left string }{
		{`xs := []api.Entity{tl[0].Start()}`, `xs := []api.Entity{0: tl[0].Start()}`, ""},
		{`xs := []float64{1, tl[2].Length(), 3, (tl[1]).Length()}`, `xs := []float64{1, 1: tl[2].Length(), 3, 3: (tl[1]).Length()}`, ""},
		{`xs := []string{n: "a", gs[0].Name()}`, `xs := []string{n: "a", (n) + 1: gs[0].Name()}`, ""},
		{`xs := [...]string{gs[0].Name()}`, `xs := [...]string{0: gs[0].Name()}`, ""},
		{`xs := [][]string{{gs[0].Name()}}`, `xs := [][]string{{0: gs[0].Name()}}`, ""},
		{`m := map[string][]string{"a": {gs[0].Name()}}`, `m := map[string][]string{"a": {0: gs[0].Name()}}`, ""},
		{`xs := []float64{fs[0].X, *ps[0], vs[0:2][1].Y}`, `xs := []float64{0: fs[0].X, 1: *ps[0], 2: vs[0:2][1].Y}`, ""},
		{`xs := [][]api.Entity{append(ls[0], e)}`, `xs := [][]api.Entity{0: append(ls[0], e)}`, ""},
		// In a function, a closure, a call's arguments and a loop's
		// header alike: nothing moves.
		{"func f() []string { return []string{gs[0].Name()} }", "func f() []string { return []string{0: gs[0].Name()} }", ""},
		{"for i := 0; i < len([]string{gs[i].Name()}); i++ {}", "for i := 0; i < len([]string{0: gs[i].Name()}); i++ {}", ""},
		{"show(func() []string { return []string{gs[0].Name()} }())", "show(func() []string { return []string{0: gs[0].Name()} }())", ""},
		// What has no trouble is left alone.
		{`xs := []api.Entity{tl[0], tl[1]}`, "", ""},
		{`xs := []string{g.Name(), strings.ToUpper(gs[0].Name()), fmt.Sprint(len(gs[0].Name()))}`, "", ""},
		{`m := map[string]string{gs[0].Name(): gs[1].Name()}`, "", ""},
		{`p := P{N: gs[0].Name()}`, "", ""},
		// What can't be keyed is said.
		{`p := P{gs[0].Name()}`, "", "gs[0].Name()"},
		{`xs := Names{gs[0].Name()}`, "", "gs[0].Name()"},
		{"xs := []string{n +\n1: \"a\", gs[0].Name()}", "", "gs[0].Name()"},
		// Source that doesn't parse is the interpreter's to report.
		{`xs := []string{gs[0].Name()`, "", ""},
	}
	for _, c := range cases {
		got := keyElements(c.src)
		want := c.want
		if want == "" {
			want = c.src
		}
		if got.src != want {
			t.Errorf("keyElements(%q) = %q, want %q", c.src, got.src, want)
		}
		var left string
		if len(got.left) > 0 {
			left = got.left[0].text
			if c.src[got.left[0].at:][:len(left)] != left {
				t.Errorf("keyElements(%q) left %q at %d, which isn't where it is", c.src, left, got.left[0].at)
			}
		}
		if left != c.left {
			t.Errorf("keyElements(%q) left %q, want %q", c.src, left, c.left)
		}
	}
}

// TestYaegiLiteralOfCompiledSliceElement is the interpreter's fault as
// it is upstream, without this program: a method called on an element
// of a slice of compiled Go, in a slice literal, panics while it is
// compiled. When an upgrade of yaegi fixes it, keyElements is no longer
// needed.
func TestYaegiLiteralOfCompiledSliceElement(t *testing.T) {
	t.Parallel()
	program := func(literal string) string {
		return `package main

import "os"

func main() {
	ds, _ := os.ReadDir(".")
	if len(ds) == 0 {
		return
	}
	xs := ` + literal + `
	_ = xs
}
`
	}
	run := func(src string) error {
		i := interp.New(interp.Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
		if err := i.Use(stdlib.Symbols); err != nil {
			t.Fatal(err)
		}
		_, err := i.EvalWithContext(context.Background(), src)
		return err
	}
	err := run(program(`[]string{ds[0].Name()}`))
	var p interp.Panic
	if !errors.As(err, &p) || !compiling(p) {
		t.Errorf("the interpreter compiles []string{ds[0].Name()} (%v): the fault keyElements works round is fixed", err)
	}
	if err := run(keyElements(program(`[]string{ds[0].Name()}`)).src); err != nil {
		t.Errorf("keyed, the literal fails: %v", err)
	}
}

// TestLiteralOfIndexedMethod runs a literal of methods called on the
// elements of a slice the scripting interface returned, in each way a
// script is run: the console's lines, statements run at once (as
// run_script and telesapr-cli eval do), and a program (telesapr-cli run).
func TestLiteralOfIndexedMethod(t *testing.T) {
	t.Parallel()
	const statements = `tl := doc.Sketch.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(10, 5, 0))
ents := []api.Entity{tl[0].Start(), tl[1].Start(), tl[2].Start()}
names := []string{doc.Groups()[0].Name()}
lengths := [][]float64{{tl[0].Length(), tl[1].Length()}}
fmt.Println(len(ents), ents[1].Pos(), ents[1] == tl[1].Start(), names[0] != "", lengths)`
	const want = "3 {10 0 0} true true [[10 5]]\n"
	ctx := context.Background()

	t.Run("console", func(t *testing.T) {
		t.Parallel()
		got := console(t, newHost(), strings.Split(statements, "\n")...)
		if got != want {
			t.Errorf("the console showed %q, want %q", got, want)
		}
	})
	t.Run("run", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		e, err := New(Options{Host: newHost(), Stdout: &out, Stderr: &out})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Run(ctx, "script", statements); err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Errorf("Run printed %q, want %q", out.String(), want)
		}
	})
	t.Run("program", func(t *testing.T) {
		t.Parallel()
		program := "package main\n\nimport (\n\t\"fmt\"\n\n\t. \"github.com/telesapr/telesapr/pkg/script/api\"\n\t\"github.com/telesapr/telesapr/pkg/script/api\"\n)\n\nfunc main() {\n\tdoc := Current()\n" +
			statements + "\n}\n"
		var out bytes.Buffer
		if err := RunSource(ctx, Options{Host: newHost(), Stdout: &out, Stderr: &out}, "a.go", program); err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Errorf("the program printed %q, want %q", out.String(), want)
		}
	})
}

// TestLiteralKeyPlaces checks that an error on a line a key was put
// into is placed in the line as it was written, and that what can't be
// keyed is said where it is, with what gets round it.
func TestLiteralKeyPlaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	newEngine := func(t *testing.T) *Engine {
		e, err := New(Options{Host: newHost(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
		if err != nil {
			t.Fatal(err)
		}
		return e
	}

	t.Run("after a key", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t)
		const line = "xs := []api.Entity{tl[0].Start(), tl[1].Start()}; fmt.Println(xs, undefinedName)"
		src := "tl := doc.Sketch.Polyline(false, V(0, 0, 0), V(10, 0, 0), V(10, 5, 0))\n" + line
		_, err := e.Run(ctx, "script", src)
		var serr *Error
		if !errors.As(err, &serr) || !strings.Contains(serr.Err.Error(), "undefined") {
			t.Fatalf("got %v, want undefinedName undefined", err)
		}
		if col := strings.Index(line, "undefinedName") + 1; serr.Line != 2 || serr.Col != col {
			t.Errorf("the error is placed at %d:%d, want 2:%d", serr.Line, serr.Col, col)
		}
	})

	t.Run("struct literal", func(t *testing.T) {
		t.Parallel()
		e := newEngine(t)
		src := "type P struct{ N string }\ngs := doc.Groups()\np := P{gs[0].Name()}\nfmt.Println(p)"
		_, err := e.Run(ctx, "script", src)
		var serr *Error
		if !errors.As(err, &serr) || !errors.Is(err, errCompileFault) {
			t.Fatalf("got %v, want the interpreter's fault", err)
		}
		if serr.Line != 3 || serr.Col != 8 {
			t.Errorf("the fault is placed at %d:%d, want 3:8", serr.Line, serr.Col)
		}
		if msg := err.Error(); !strings.Contains(msg, "gs[0].Name()") || !strings.Contains(msg, "put it in a variable first") {
			t.Errorf("the error doesn't name the element and the way round it: %s", msg)
		}
	})

	t.Run("program", func(t *testing.T) {
		t.Parallel()
		program := "package main\n\nimport . \"github.com/telesapr/telesapr/pkg/script/api\"\n\ntype P struct{ N string }\n\nfunc main() {\n\tgs := Current().Groups()\n\tp := P{gs[0].Name()}\n\t_ = p\n}\n"
		err := RunSource(ctx, Options{Host: newHost(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}, "a.go", program)
		var serr *Error
		if !errors.As(err, &serr) || !errors.Is(err, errCompileFault) {
			t.Fatalf("got %v, want the interpreter's fault", err)
		}
		if serr.File != "a.go" || serr.Line != 9 || serr.Col != 9 {
			t.Errorf("the fault is placed at %s:%d:%d, want a.go:9:9", serr.File, serr.Line, serr.Col)
		}
	})
}

// TestCompileFaultWithoutPlace checks the words for a panic of the
// interpreter's while it compiles that keyElements knows nothing of.
func TestCompileFaultWithoutPlace(t *testing.T) {
	t.Parallel()
	err := compileFault("x := 1", errors.New("nil pointer"))
	if err.Line != 0 || !errors.Is(err, errCompileFault) || !strings.Contains(err.Error(), "nil pointer") {
		t.Errorf("got %v at line %d", err, err.Line)
	}
}

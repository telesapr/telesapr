package script

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

func newHost() *api.SketchHost {
	sk := sketch.NewSketch()
	sk.NewFile()
	return api.NewSketchHost(sk, "")
}

// console feeds lines to a new engine as a console does, and returns
// what it showed.
func console(t *testing.T, host api.Host, lines ...string) string {
	t.Helper()
	var out bytes.Buffer
	e, err := New(Options{Host: host, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.REPL(context.Background(), strings.NewReader(strings.Join(lines, "\n")), &out, false); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestConsole(t *testing.T) {
	host := newHost()
	got := console(t, host,
		`1 + 2`,
		`x := 21`,
		`x * 2`,
		`fmt.Println("hello", math.Sqrt(16))`,
		`l := doc.Line(V(0, 0, 0), V(30, 40, 0))`,
		`l.Length()`,
		`l`,
		`func double(v float64) float64 {`,
		`	return 2 * v`,
		`}`,
		`double(l.Length())`,
		`for i := 0; i < 2; i++ {`,
		`	fmt.Println("i", i)`,
		`}`,
		`doc.Groups()`,
		`strings.ToUpper("abc")`,
		`l.Radius()`,
		`undefined(3)`,
		`var a []int`,
		`a[3]`,
		`doc.Active().Dof()`,
		`l.Start().Pos()`,
		`doc.Active().Bounds()`,
		`strconv.Atoi("x1")`,
		`[]int{1, 2}`,
		`nil`,
	)
	want := `3
42
hello 4
50
line r004-line-segment
100
i 0
i 1
0: g002-sketch-in-plane
"ABC"
error: line r004-line-segment isn't a circle or an arc
error: undefined: undefined
error: panic: reflect: slice index out of range
4
(0, 0, 0)
(0, 0, 0), (0, 0, 0)
0, strconv.Atoi: parsing "x1": invalid syntax
[1 2]
nil
`
	if got != want {
		t.Errorf("the console showed:\n%s\nwant:\n%s", got, want)
	}
	if n := len(host.Sketch().Requests); n != 4 {
		t.Errorf("the sketch has %d requests, want the 3 references and the line", n)
	}
}

// Each line of a console is one step of undo, if it changed anything.
func TestConsoleUndo(t *testing.T) {
	host := newHost()
	console(t, host,
		`a := doc.Line(V(0, 0, 0), V(10, 0, 0))`,
		`a.Length()`,
		`b := doc.Line(V(0, 0, 0), V(0, 10, 0))`,
		`doc.Coincident(a.Start(), b.Start()); doc.Horizontal(a)`,
	)
	sk := host.Sketch()
	if got := sk.Undo.Cnt; got != 3 {
		t.Errorf("%d steps of undo, want 3", got)
	}
	if !api.NewDoc(host).Undo() {
		t.Fatal("nothing to undo")
	}
	sk = host.Sketch()
	if len(sk.Constraints) != 0 || len(sk.Requests) != 5 {
		t.Errorf("after undo: %d constraints and %d requests, want 0 and 5", len(sk.Constraints), len(sk.Requests))
	}
}

const program = `package main

import (
	"fmt"
	"os"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	doc := api.Current()
	sides := doc.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	doc.Length(sides[0], 20)
	doc.Length(sides[1], 10)
	ex := doc.Extrude(4, "block")
	fmt.Printf("%s %.0f %v\n", ex, ex.Volume(), os.Args[1:])
}
`

func TestRunProgram(t *testing.T) {
	var out bytes.Buffer
	host := newHost()
	err := RunSource(context.Background(), Options{Host: host, Stdout: &out, Stderr: &out,
		Args: []string{"block.go", "-n", "3"}}, "block.go", program)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "g003-block 800 [-n 3]\n"; got != want {
		t.Errorf("the program printed %q, want %q", got, want)
	}
	if got := host.Sketch().Undo.Cnt; got != 1 {
		t.Errorf("%d steps of undo for the program, want 1", got)
	}
}

func TestRunStatements(t *testing.T) {
	var out bytes.Buffer
	src := `#!/usr/bin/env spacesolve-cli run
// A plate, as statements.
w, h := 40.0, 25.0

func area() float64 {
	return w * h
}

sides := doc.Rect(V(0, 0, 0), V(w, h, 0))
if len(sides) != 4 {
	panic("a rectangle has four sides")
}
fmt.Println(area())
doc.Circle(V(1, 1, 0), 2).Pos()
`
	err := RunSource(context.Background(), Options{Host: newHost(), Stdout: &out, Stderr: &out}, "plate.go", src)
	if got, want := out.String(), "1000\n"; !strings.HasPrefix(got, want) {
		t.Errorf("the script printed %q, want %q first", got, want)
	}
	var serr *Error
	if !errors.As(err, &serr) || serr.Line != 14 {
		t.Fatalf("the error is %v, want one at line 14", err)
	}
	if got, want := err.Error(), "plate.go:14:1: circle r008-circle, radius 2.000 isn't a point; see Start, End, Center and Points"; got != want {
		t.Errorf("the error is %q, want %q", got, want)
	}

	err = RunSource(context.Background(), Options{Host: newHost(), Stdout: &out, Stderr: &out}, "bad.go",
		"x := 1\n\ny := x +\n\tnope\n")
	if err == nil || err.Error() != "bad.go:4:2: undefined: nope" {
		t.Errorf("the error is %v, want bad.go:4:2: undefined: nope", err)
	}

	// A definition the engine has to make itself reports its
	// expression's error at its place.
	err = RunSource(context.Background(), Options{Host: newHost(), Stdout: &out, Stderr: &out}, "def.go",
		"x := 1\n\ng := doc.Groups()[nope]\n")
	if err == nil || err.Error() != "def.go:3:19: undefined: nope" {
		t.Errorf("the error is %v, want def.go:3:19: undefined: nope", err)
	}

	err = RunSource(context.Background(), Options{Host: newHost(), Stdout: &out, Stderr: &out}, "open.go",
		"x := 1\nif x > 0 {\n\tx++\n")
	if err == nil || err.Error() != "open.go:2:1: the script ends in the middle of a statement" {
		t.Errorf("the error is %v, want open.go:2:1: the script ends in the middle of a statement", err)
	}
}

func TestExitAndCancel(t *testing.T) {
	var out bytes.Buffer
	opts := Options{Host: newHost(), Stdout: &out, Stderr: &out}
	err := RunSource(context.Background(), opts, "exit.go", "fmt.Println(\"bye\")\nos.Exit(3)\nfmt.Println(\"not reached\")\n")
	var exit Exit
	if !errors.As(err, &exit) || exit != 3 {
		t.Errorf("os.Exit(3) gave %v", err)
	}
	if strings.Contains(out.String(), "not reached") {
		t.Errorf("the script went on after os.Exit: %q", out.String())
	}

	e, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := e.Eval(context.Background(), "for { time.Sleep(time.Millisecond) }")
		done <- err
	}()
	for !e.Running() {
		time.Sleep(time.Millisecond)
	}
	if _, err := e.Eval(context.Background(), "1"); !errors.Is(err, ErrBusy) {
		t.Errorf("a second script while one runs gave %v, want ErrBusy", err)
	}
	e.Cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("the cancelled script gave %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the script didn't stop")
	}
}

// A script's other documents come from the engine's options.
func TestOpenThroughOptions(t *testing.T) {
	var out bytes.Buffer
	opened := ""
	opts := Options{Host: newHost(), Stdout: &out, Stderr: &out,
		Open: func(path string) (api.Host, error) {
			opened = path
			return newHost(), nil
		}}
	err := RunSource(context.Background(), opts, "open.go",
		"other, err := Open(\"part.slvs\")\nfmt.Println(err, len(other.Groups()), other != doc)\n")
	if err != nil {
		t.Fatal(err)
	}
	if opened != "part.slvs" || out.String() != "<nil> 1 true\n" {
		t.Errorf("opened %q, printed %q", opened, out.String())
	}
}

// A variable defined from a value whose type the interpreter can't
// work out beforehand is defined all the same, once, with that type.
func TestDefinitionsOfProgramTypes(t *testing.T) {
	host := newHost()
	got := console(t, host,
		`a := doc.Groups()[0]`,
		`a`,
		`gs := doc.Groups()`,
		`b := gs[0]`,
		`b.Name()`,
		`n := doc.Groups()[0].Name()`,
		`n`,
		`var w = doc.Groups()`,
		`len(w)`,
		`var v []Group = doc.Groups()`,
		`v[0]`,
		`l := doc.Polyline(false, V(0, 0, 0), V(3, 0, 0), V(3, 4, 0))[1]`,
		`l.Length()`,
		`lo, hi := doc.Active().Bounds()`,
		`hi`,
		`s := doc.Active().Name()[0]`,
		`s`,
		`a = doc.Active()`,
		`a := 5`,
		`a + 1`,
		`q := nope()[0]`,
		`c, d := doc.Groups()[0]`,
	)
	want := `g002-sketch-in-plane
"sketch-in-plane"
"sketch-in-plane"
1
g002-sketch-in-plane
4
(0, 0, 0)
115
6
error: constant definition loop, or a name that isn't defined
error: assignment mismatch: 2 variables but 1 values
`
	if got != want {
		t.Errorf("the console showed:\n%s\nwant:\n%s", got, want)
	}
	// The polyline was drawn once.
	if n := len(host.Sketch().Requests); n != 5 {
		t.Errorf("the sketch has %d requests, want the 3 references and 2 lines", n)
	}
}

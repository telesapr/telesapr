package script

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// A script's run-time error says in plain words what went wrong, and
// quotes the line it went wrong on.
func TestRunTimeErrorWords(t *testing.T) {
	t.Parallel()
	long := "fmt.Println(xs[0]" + strings.Repeat(" + 1", 40) + ")"
	for _, c := range []struct{ name, src, want string }{
		{"index", "xs := []int{}\nfmt.Println(xs[3])",
			"script:2:1: index out of range\n    fmt.Println(xs[3])"},
		{"nil map", "var m map[string]int\n  m[\"a\"] = 1",
			"script:2:3: assignment to a nil map\n    m[\"a\"] = 1"},
		{"division", "a, b := 1, 0\nfmt.Println(a / b)",
			"script:2:1: division by zero\n    fmt.Println(a / b)"},
		{"nil pointer", "var g *api.Group\nfmt.Println(g.Name())",
			"script:2:1: nil pointer or nil interface\n    fmt.Println(g.Name())"},
		{"conversion", "var x any = 1\ny := x.(string)\nfmt.Println(y)",
			"script:2:6: interface {} is int, not string\n    y := x.(string)"},
		// An error of package api is kept as it is.
		{"api", "l := doc.Sketch.Line(V(0, 0, 0), V(30, 40, 0))\nl.Radius()",
			"script:2:1: line r004-line-segment isn't a circle or an arc\n    l.Radius()"},
		{"own panic", "panic(\"boom\")", "script:1:1: panic: boom\n    panic(\"boom\")"},
		// The second statement to fail is the one quoted.
		{"second", "xs := []int{1, 2}\nys := []int{}\nfmt.Println(xs[0])\nfmt.Println(ys[1])\nfmt.Println(xs[1])",
			"script:4:1: index out of range\n    fmt.Println(ys[1])"},
		{"long line", "xs := []int{}\n" + long,
			"script:2:1: index out of range\n    " + string([]rune(long)[:maxSource-1]) + "…"},
		{"program", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\txs := []int{}\n\tfmt.Println(xs[3])\n}\n",
			"script:7:2: index out of range\n    fmt.Println(xs[3])"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e, err := New(Options{Host: newHost(), Stdout: io.Discard, Stderr: io.Discard})
			if err != nil {
				t.Fatal(err)
			}
			_, err = e.Run(context.Background(), "script", c.src)
			if err == nil || err.Error() != c.want {
				t.Errorf("the error is\n%v\nwant\n%s", err, c.want)
			}
			if serr, ok := errors.AsType[*Error](err); !ok || serr.Source == "" {
				t.Errorf("%v is not an Error that quotes its source", err)
			}
		})
	}
}

// Only the wording of the runtime's, reflect's and the interpreter's
// panics changes; an error of another package is kept.
func TestPanicError(t *testing.T) {
	t.Parallel()
	apiErr := errors.New("SolidEdges is for a chamfer group")
	for _, c := range []struct {
		value any
		want  string
	}{
		{"reflect: array index out of range", "index out of range"},
		{"reflect: slice index out of range", "index out of range"},
		{"something else", "panic: something else"},
		{apiErr, apiErr.Error()},
		{42, "panic: 42"},
	} {
		if got := panicError(c.value).Error(); got != c.want {
			t.Errorf("panicError(%v) = %q, want %q", c.value, got, c.want)
		}
	}
	for _, c := range []struct{ msg, want string }{
		{"index out of range [3] with length 0", "index out of range [3] with length 0"},
		{"invalid memory address or nil pointer dereference", "nil pointer"},
		{"interface conversion: interface {} is int, not string", "interface {} is int, not string"},
	} {
		if got, ok := plainPanic(c.msg); !ok || got != c.want {
			t.Errorf("plainPanic(%q) = %q, %v, want %q", c.msg, got, ok, c.want)
		}
	}
}

// A runtime panic keeps the numbers it says.
func TestPanicErrorRuntime(t *testing.T) {
	t.Parallel()
	var value any
	func() {
		defer func() { value = recover() }()
		xs := []int{}
		i := 3
		_ = xs[i]
	}()
	if got, want := panicError(value).Error(), "index out of range [3] with length 0"; got != want {
		t.Errorf("panicError = %q, want %q", got, want)
	}
	func() {
		defer func() { value = recover() }()
		var p *struct{ X int }
		_ = p.X
	}()
	if got, want := panicError(value).Error(), "nil pointer"; got != want {
		t.Errorf("panicError = %q, want %q", got, want)
	}
}

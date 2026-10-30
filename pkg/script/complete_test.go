package script

import (
	"context"
	"io"
	"slices"
	"testing"
)

// TestComplete completes names as a console's Tab does: the console's
// variables and Go's names, a package's, and what follows a chain of
// fields, calls and indexes, from types alone.
func TestComplete(t *testing.T) {
	t.Parallel()
	e, err := New(Options{Host: newHost(), Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"circle := doc.Sketch.Circle(V(0, 0, 0), 2)", "groups := doc.Groups()", "var anything any = doc"} {
		if _, _, err := e.Feed(context.Background(), line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}
	for _, c := range []struct {
		src       string
		start     int
		has, hasn []string
	}{
		{"cir", 0, []string{"circle"}, nil},
		{"x := do", 5, []string{"doc"}, []string{"circle"}},
		{"he", 0, []string{"help("}, nil},
		{"fo", 0, []string{"for"}, nil},
		{"le", 0, []string{"len("}, nil},
		{"str", 0, []string{"strings"}, nil},
		{"Ve", 0, nil, nil},
		{"V", 0, []string{"V("}, nil},
		{"strings.ToU", 8, []string{"strings.ToUpper("[8:]}, []string{"Split("}},
		{"math.P", 5, []string{"Pi"}, nil},
		{"doc.", 4, []string{"Sketch", "Groups(", "Active("}, []string{"h"}},
		{"doc.Sketch.Li", 11, []string{"Line("}, []string{"Circle("}},
		{"doc.Active().Requ", 13, []string{"Requests("}, nil},
		{"groups[0].Na", 10, []string{"Name("}, nil},
		{"doc.Groups()[1].Requ", 16, []string{"Requests("}, nil},
		{"circle.Rad", 7, []string{"Radius("}, nil},
		{"anything.Sk", 9, []string{"Sketch"}, nil},
		{"fmt.Println(circle.Ra", 19, []string{"Radius("}, nil},
		{"V(1, 2, 3).", 11, []string{"X", "Plus("}, nil},
		{"nothing.", 8, nil, []string{"Sketch"}},
		{"math.Pi.", 8, nil, nil},
		{"doc.Sketch.Line.", 16, nil, nil},
		{"(1).", 4, nil, nil},
		{"x).", 3, nil, nil},
		{"strings.Nothing.", 16, nil, nil},
	} {
		start, got := e.Complete(c.src, len(c.src))
		if start != c.start {
			t.Errorf("%q: the word starts at %d, want %d", c.src, start, c.start)
		}
		for _, w := range c.has {
			if !slices.Contains(got, w) {
				t.Errorf("%q offers %v, without %q", c.src, got, w)
			}
		}
		for _, w := range c.hasn {
			if slices.Contains(got, w) {
				t.Errorf("%q offers %q", c.src, w)
			}
		}
		if !slices.IsSorted(got) {
			t.Errorf("%q: %v isn't sorted", c.src, got)
		}
	}
	// The cursor in the middle of what is typed completes what is
	// before it.
	if start, got := e.Complete("circ := 1", 4); start != 0 || !slices.Contains(got, "circle") {
		t.Errorf("in the middle: %d %v", start, got)
	}
}

// TestUnfinished tells what Return should enter from what goes on to
// another line, counting what the engine already has.
func TestUnfinished(t *testing.T) {
	t.Parallel()
	e, err := New(Options{Host: newHost(), Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	for src, want := range map[string]bool{
		"":                     false,
		"x := 1":               false,
		"for i := 0; i < 2; {": true,
		"f(1,":                 true,
		"if true {\n}":         false,
		"`raw":                 true,
	} {
		if got := e.Unfinished(src); got != want {
			t.Errorf("Unfinished(%q) = %v", src, got)
		}
	}
	if _, more, _ := e.Feed(context.Background(), "if true {"); !more {
		t.Fatal("an if wasn't left pending")
	}
	if e.Unfinished("}") || !e.Unfinished("x := 1") {
		t.Error("the pending lines aren't counted")
	}
}

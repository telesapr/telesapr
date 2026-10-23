package script

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A program that fails is reported at the statement that failed, in
// the function it is in, not at the start of main; and a program that
// doesn't compile is reported where it is wrong, the statements' places
// put in or not.
func TestProgramPanicPlace(t *testing.T) {
	t.Parallel()
	const head = `package main

import (
	"fmt"

	. "github.com/telesapr/telesapr/pkg/script/api"
)

`
	for _, c := range []struct {
		name, body string
		line, col  int
		message    string
	}{
		{"in main", `func main() {
	doc := Current()
	fmt.Println(1)
	c := doc.Sketch.Circle(V(0, 0, 0), 2)

	c.Start()
	fmt.Println("after")
}
`, 14, 2, "has no start"},
		{"in a function", `func start(doc *Doc) {
	c := doc.Sketch.Circle(V(0, 0, 0), 2)
	if c.Radius() > 1 {
		c.Start()
	}
}

func main() {
	doc := Current()
	start(doc)
}
`, 12, 3, "has no start"},
		{"after a function returned", `func circle(doc *Doc) Entity {
	c := doc.Sketch.Circle(V(0, 0, 0), 2)
	return c
}

func main() {
	doc := Current()
	fmt.Println(1); circle(doc).Start()
}
`, 16, 18, "has no start"},
		{"in a closure", `func main() {
	doc := Current()
	each := func(f func()) { f() }
	each(func() {
		doc.Sketch.Circle(V(0, 0, 0), 2).Start()
	})
}
`, 13, 3, "has no start"},
		{"in a switch", `func main() {
	doc := Current()
	switch c := doc.Sketch.Circle(V(0, 0, 0), 2); {
	case c.Radius() > 1:
		c.Start()
	}
}
`, 13, 3, "has no start"},
		{"not compiled", `func main() {
	doc := Current()
	fmt.Println(doc, nowhere)
}
`, 11, 19, "undefined: nowhere"},
	} {
		err := RunSource(context.Background(), Options{Host: newHost(), Stdout: &strings.Builder{}, Stderr: &strings.Builder{}}, "p.go", head+c.body)
		var serr *Error
		if !errors.As(err, &serr) || serr.Line != c.line || serr.Col != c.col || !strings.Contains(serr.Err.Error(), c.message) {
			t.Errorf("%s: %v, want p.go:%d:%d: ...%s", c.name, err, c.line, c.col, c.message)
		}
	}
}

// unplace takes what was put in before a place on its line off its
// column, and says a place inside an insert is where the insert went.
func TestUnplace(t *testing.T) {
	t.Parallel()
	src := "ab\ncdef\ng"
	inserts := []insert{{at: 3, text: "XX"}, {at: 5, text: "YYY"}}
	// Placed: "ab\nXXcdYYYef\ng".
	for _, c := range []struct{ line, col, want int }{
		{1, 2, 2},
		{2, 1, 1}, // in XX: the insert's place
		{2, 3, 1}, // c
		{2, 4, 2}, // d
		{2, 6, 3}, // in YYY
		{2, 8, 3}, // e
		{2, 9, 4}, // f
		{3, 1, 1},
	} {
		if _, col := unplace(src, inserts, c.line, c.col); col != c.want {
			t.Errorf("%d:%d is column %d, want %d", c.line, c.col, col, c.want)
		}
	}
}

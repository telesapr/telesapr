package script

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// Run takes several statements at once, keeps what they define for
// the next run, shows the last expression, and is one step of undo.
func TestEngineRun(t *testing.T) {
	host := newHost()
	var out bytes.Buffer
	e, err := New(Options{Host: host, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	shown, err := e.Run(ctx, "", `
r := doc.Rect(V(0, 0, 0), V(40, 20, 0))
for _, side := range r {
	fmt.Println(side.Length())
}
plate := doc.Extrude(5, "plate")
plate.Kind()
`)
	if err != nil {
		t.Fatal(err)
	}
	if shown != `"extrude"` {
		t.Errorf("the last expression is shown as %s, want \"extrude\"", shown)
	}
	if got := strings.Fields(out.String()); len(got) != 4 {
		t.Errorf("the loop printed %q, want four lengths", out.String())
	}

	// What it defined is there for the next run.
	shown, err = e.Run(ctx, "", "len(r)\nplate.Bounds()")
	if err != nil || shown != "(0, 0, 0), (40, 20, 5)" {
		t.Errorf("the next run shows %s (%v), want the plate's bounds", shown, err)
	}

	// The whole run was one step of undo.
	if got := len(e.Doc().Groups()); got != 2 {
		t.Fatalf("%d groups after the run, want 2", got)
	}
	if !e.Doc().Undo() {
		t.Fatalf("nothing to undo after the run")
	}
	if got, requests := len(e.Doc().Groups()), len(e.Doc().Active().Requests()); got != 1 || requests != 0 {
		t.Errorf("after one undo: %d groups and %d requests, want 1 and none", got, requests)
	}

	// A statement that ends nothing shown.
	if shown, err = e.Run(ctx, "", "x := 3\n"); err != nil || shown != "" {
		t.Errorf("a definition shows %q (%v), want nothing", shown, err)
	}

	// An error names the line it is on, and what ran before it stays.
	_, err = e.Run(ctx, "", "y := x + 1\n\ndoc.Line(V(0, 0, 0), V(1, 0, 0)).Radius()\nz := 5")
	var serr *Error
	if !errors.As(err, &serr) || serr.Line != 3 || !strings.Contains(serr.Error(), "isn't a circle") {
		t.Errorf("the error is %v, want one on line 3 about a circle", err)
	}
	if shown, err = e.Run(ctx, "", "y"); err != nil || shown != "4" {
		t.Errorf("y is %s (%v) after the failed run, want 4", shown, err)
	}
	if _, err = e.Run(ctx, "", "z"); err == nil {
		t.Errorf("z is defined, though the run stopped before it")
	}
	if _, err = e.Run(ctx, "", "for i := 0; i < 3; i++ {"); err == nil {
		t.Errorf("a statement that doesn't end ran without an error")
	}
}

// A whole program given to Run runs as a file does: on the sketch,
// without what the engine defined.
func TestEngineRunProgram(t *testing.T) {
	host := newHost()
	var out bytes.Buffer
	e, err := New(Options{Host: host, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := e.Run(ctx, "", "secret := 7"); err != nil {
		t.Fatal(err)
	}
	const program = `package main

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Circle(api.V(0, 0, 0), 5)
	fmt.Println("requests:", len(doc.Active().Requests()))
}
`
	shown, err := e.Run(ctx, "part.go", program)
	if err != nil || shown != "" {
		t.Fatalf("the program gave %q, %v", shown, err)
	}
	if got := out.String(); got != "requests: 1\n" {
		t.Errorf("the program printed %q", got)
	}
	if got := len(e.Doc().Active().Requests()); got != 1 {
		t.Errorf("the engine's sketch has %d requests after the program, want 1", got)
	}
	_, err = e.Run(ctx, "part.go", strings.Replace(program, "doc.Circle(", "fmt.Println(secret); doc.Circle(", 1))
	var serr *Error
	if !errors.As(err, &serr) || serr.File != "part.go" {
		t.Errorf("a program that names what the engine defined: %v, want an error in part.go", err)
	}
}

func TestHelpByName(t *testing.T) {
	var out bytes.Buffer
	if !Help(&out, "") || out.String() != helpText {
		t.Errorf("Help of no name wrote %q, want the summary", out.String())
	}
	out.Reset()
	if !Help(&out, "Doc.Describe") || !strings.Contains(out.String(), "func (d *Doc) Describe() (s string)") {
		t.Errorf("Help of Doc.Describe wrote %q", out.String())
	}
	out.Reset()
	if Help(&out, "Doc.NoSuchThing") || out.Len() != 0 {
		t.Errorf("Help of a name nothing has wrote %q", out.String())
	}
}

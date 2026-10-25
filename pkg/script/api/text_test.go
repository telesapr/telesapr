package api

import (
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/ttf"
)

// A text's string, font and kerning are read back and changed after it
// is drawn; the corner stays where it was and the width follows the
// string.
func TestTextEdits(t *testing.T) {
	t.Parallel()
	d := New()
	text := d.Sketch.Text("Abc", "", V(5, 5, 0), 10)
	if text.Text() != "Abc" || text.Font() != ttf.EmbeddedFont || text.Kerning() {
		t.Fatalf("text %q, font %q, kerning %v", text.Text(), text.Font(), text.Kerning())
	}

	d.Constrain.Lock(text.Points()[1])
	text.SetText("Abcdef")
	text.SetKerning(true)
	text.SetFont("")
	d.Edit.Regen()
	if text.Text() != "Abcdef" || !text.Kerning() {
		t.Errorf("text %q, kerning %v after the edits", text.Text(), text.Kerning())
	}
	aspect := ttf.DefaultFonts.AspectRatio(ttf.EmbeddedFont, "Abcdef", true)
	pts := text.Points()
	if p := pts[1].Pos(); !near(p.X, 5) || !near(p.Y, 5) {
		t.Errorf("the corner moved to %v", p)
	}
	if w, h := pts[2].Pos().X-pts[1].Pos().X, pts[0].Pos().Y-pts[1].Pos().Y; !near(w, aspect*h) {
		t.Errorf("the text is %g wide and %g high, want the ratio %g", w, h, aspect)
	}
	if !d.Active().Solved() {
		t.Errorf("the text doesn't solve")
	}
}

// With nothing holding a text, a change of width leaves its left
// corners and its height where they were: left to the solver, the four
// corners all moved, and the text grew.
func TestTextEditKeepsCorner(t *testing.T) {
	t.Parallel()
	d := New()
	text := d.Sketch.Text("To", "", V(-5, -5, 0), 10)
	text.SetKerning(true)
	text.SetText("Toto")
	d.Edit.Regen()
	pts := text.Points()
	if p, q := pts[0].Pos(), pts[1].Pos(); !near(p.X, -5) || !near(p.Y, 5) || !near(q.X, -5) || !near(q.Y, -5) {
		t.Errorf("the left corners moved to %v and %v", p, q)
	}
	aspect := ttf.DefaultFonts.AspectRatio(ttf.EmbeddedFont, "Toto", true)
	if w := pts[2].Pos().X - pts[1].Pos().X; !near(w, aspect*10) {
		t.Errorf("the text is %g wide, want %g", w, aspect*10)
	}
}

// The fonts a text can take are listed, and the built-in one is among
// them.
func TestSketchFonts(t *testing.T) {
	t.Parallel()
	d := New()
	fonts := d.Sketch.Fonts()
	if !slices.Contains(fonts, ttf.EmbeddedFont) {
		t.Errorf("fonts %v lack the built-in %q", fonts, ttf.EmbeddedFont)
	}
	text := d.Sketch.Text("Abc", fonts[0], V(0, 0, 0), 5)
	if text.Font() != fonts[0] {
		t.Errorf("font %q, want %q", text.Font(), fonts[0])
	}
}

// What is not a text, and a font that is not there, fail with an error.
func TestTextEditMisuse(t *testing.T) {
	t.Parallel()
	d := New()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	text := d.Sketch.Text("x", "", V(0, 5, 0), 5)
	for name, fn := range map[string]func(){
		"text of a line":   func() { line.Text() },
		"set text of line": func() { line.SetText("x") },
		"kerning of line":  func() { line.SetKerning(true) },
		"lost font":        func() { text.SetFont("no-such-font.ttf") },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: want an error", name)
				}
			}()
			fn()
		}()
	}
	if text.Font() != ttf.EmbeddedFont {
		t.Errorf("a refused font changed it to %q", text.Font())
	}
}

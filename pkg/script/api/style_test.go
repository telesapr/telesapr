package api

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// Every field of a style is read back as it was set, and a style of the
// user's own is made, named and deleted.
func TestStyleFields(t *testing.T) {
	t.Parallel()
	d := New()
	s := d.NewStyle()
	if !s.Valid() || !s.Custom() {
		t.Fatalf("a new style is %v, custom %t", s.Valid(), s.Custom())
	}
	s.SetName("hidden")
	if s.Name() != "hidden" || d.Style("hidden").H != s.H {
		t.Errorf("the style is %q, and the list has it as %q", s.Name(), d.Style("hidden").Name())
	}
	s.SetColor(10, 20, 30)
	s.SetFillColor(40, 50, 60)
	s.SetFilled(true)
	s.SetWidthAs(core.UnitsMM)
	s.SetWidth(2.5)
	s.SetStipple(core.StippleDashDot)
	s.SetStippleScale(3)
	s.SetTextHeight(9)
	s.SetTextAngle(45)
	s.SetZIndex(7)
	s.SetVisible(false)
	s.SetExportable(false)
	for _, c := range []struct {
		what string
		got  any
		want any
	}{
		{"name", s.Name(), "hidden"},
		{"colour", colorTuple(s.Color()), colorTuple(10, 20, 30)},
		{"fill colour", colorTuple(s.FillColor()), colorTuple(40, 50, 60)},
		{"filled", s.Filled(), true},
		{"width", s.Width(), 2.5},
		{"width as", s.WidthAs(), core.UnitsMM},
		{"stipple", s.Stipple(), core.StippleDashDot},
		{"stipple scale", s.StippleScale(), 3.0},
		{"text height", s.TextHeight(), 9.0},
		{"text angle", s.TextAngle(), 45.0},
		{"z-index", s.ZIndex(), 7},
		{"visible", s.Visible(), false},
		{"exportable", s.Exportable(), false},
	} {
		if c.got != c.want {
			t.Errorf("the %s is %v, want %v", c.what, c.got, c.want)
		}
	}
	s.Delete()
	if s.Valid() {
		t.Error("the style is still there after it is deleted")
	}
	failsWith(t, `no style named "hidden"`, func() { d.Style("hidden") })
}

// A style is given to what is drawn in it, and the drawing says which
// style that is.
func TestStyleOfGeometry(t *testing.T) {
	t.Parallel()
	d := New()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	d.Constrain.Horizontal(line)
	if got := line.Request().Style().H; got.V != core.StyleActiveGrp {
		t.Errorf("a new request is drawn in the style %v, want the active group's", got)
	}
	s := d.NewStyle()
	line.Request().SetStyle(s)
	if line.Request().Style().H != s.H {
		t.Errorf("the line is drawn in %v, want %v", line.Request().Style(), s)
	}
	if n := len(s.Requests()); n != 1 {
		t.Errorf("the style is drawn in by %d requests, want 1", n)
	}
	// The zero Style takes the style away.
	line.Request().SetStyle(Style{})
	if got := line.Request().Style().H; got.V != core.StyleActiveGrp || len(s.Requests()) != 0 {
		t.Errorf("after clearing, the line is drawn in %v and %d requests are in the style", got, len(s.Requests()))
	}
}

// Only what the window styles can be given a style by a script: a
// comment and a line, not a dimension or a point.
func TestStyleOnlyStylable(t *testing.T) {
	t.Parallel()
	d := New()
	s := d.NewStyle()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	dim := d.Constrain.Distance(line.Start(), line.End(), 10)
	note := d.Constrain.Comment(V(0, -5, 0), "a note")
	pt := d.Sketch.Point(V(5, 5, 0))

	sk := d.Host().(*SketchHost).Sketch()
	// A refusal leaves the sketch alone, and remembers no step of undo.
	sk.Unsaved = false
	undo := sk.Undo.Cnt
	for what, fn := range map[string]func(){
		"a dimension": func() { dim.SetStyle(s) },
		"a point":     func() { pt.Request().SetStyle(s) },
	} {
		func() {
			defer func() {
				err, _ := recover().(error)
				if !errors.Is(err, ErrNotStylable) {
					t.Errorf("styling %s failed with %v, want ErrNotStylable", what, err)
				}
			}()
			fn()
		}()
	}
	if sk.Undo.Cnt != undo || sk.Unsaved {
		t.Errorf("a refused style left undo at %d (was %d), unsaved %t", sk.Undo.Cnt, undo, sk.Unsaved)
	}
	note.SetStyle(s)
	line.Request().SetStyle(s)
	if dim.Style().H.V != core.StyleConstraint {
		t.Errorf("the dimension is drawn in %v after a refused style", dim.Style())
	}
	if note.Style().H != s.H {
		t.Errorf("the comment is drawn in %v, want %v", note.Style(), s)
	}
	if got := len(s.Constraints()); got != 1 {
		t.Errorf("%d constraints are in the style, want the comment", got)
	}
	// And back to the default.
	note.SetStyle(Style{})
	if note.Style().H.V != core.StyleConstraint {
		t.Errorf("the comment is drawn in %v after clearing, want the constraint style", note.Style())
	}
}

// Listing or looking up the program's own styles reads, and leaves the
// sketch as it was, so that nothing is marked unsaved or saved with it.
func TestStyleReadsLeaveSketch(t *testing.T) {
	t.Parallel()
	d := New()
	sk := d.Host().(*SketchHost).Sketch()
	sk.Unsaved = false
	n := len(sk.Styles)
	styles := d.Styles()
	if len(styles) < 16 {
		t.Fatalf("the drawing lists %d styles", len(styles))
	}
	edge := d.Style("#def-solid-edge")
	if !edge.Valid() || edge.ZIndex() == 0 {
		t.Fatalf("the solid-edge style reads as valid %t, level %d", edge.Valid(), edge.ZIndex())
	}
	_, _, _ = edge.Color()
	if len(sk.Styles) != n || sk.Unsaved {
		t.Errorf("reading changed the sketch: %d styles, was %d; unsaved %t", len(sk.Styles), n, sk.Unsaved)
	}
	// A change to a default style makes it, as a write.
	edge.SetZIndex(9)
	if edge.ZIndex() != 9 || len(sk.Styles) != n+1 {
		t.Errorf("after the change the level is %d and the sketch has %d styles, want 9 and %d", edge.ZIndex(), len(sk.Styles), n+1)
	}
}

// The styles of the program's own are there to be changed, and not to
// be renamed away or deleted.
func TestBuiltinStyles(t *testing.T) {
	t.Parallel()
	d := New()
	styles := d.Styles()
	if len(styles) < 16 {
		t.Fatalf("the drawing has %d styles", len(styles))
	}
	edge := d.Style("#def-solid-edge")
	if !edge.Valid() {
		t.Fatal("there is no solid-edge style")
	}
	if edge.Custom() {
		t.Error("the solid-edge style is one of the user's own")
	}
	if edge.ZIndex() == 0 {
		t.Error("the solid-edge style is drawn under everything")
	}
	edge.SetZIndex(9)
	if edge.ZIndex() != 9 {
		t.Errorf("the solid-edge style is at %d, want 9", edge.ZIndex())
	}
	for name, fn := range map[string]func(){
		"rename":  func() { edge.SetName("edges") },
		"delete":  func() { edge.Delete() },
		"nothing": func() { d.Style("no such style").SetZIndex(1) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
	if edge.Name() != "#def-solid-edge" || !edge.Valid() {
		t.Errorf("the style is now %q, valid %t", edge.Name(), edge.Valid())
	}
}

// A field of a style is one step of undo, and misuse is an error that
// changes nothing.
func TestStyleUndoAndMisuse(t *testing.T) {
	t.Parallel()
	d := New()
	s := d.NewStyle()
	step := d.Host().(*SketchHost).BeginEval
	step()
	s.SetColor(1, 2, 3)
	if r, g, b := s.Color(); r != 1 || g != 2 || b != 3 {
		t.Fatalf("the colour is %d %d %d", r, g, b)
	}
	if !d.Edit.Undo() {
		t.Fatal("can't undo the colour")
	}
	if r, _, _ := s.Color(); r == 1 {
		t.Error("the colour is still set after undo")
	}
	before := s.Name()
	width, scale := s.Width(), s.StippleScale()
	for name, fn := range map[string]func(){
		"a colour out of range": func() { s.SetColor(0, 0, 300) },
		"a stipple it has not":  func() { s.SetStipple(core.StipplePattern(99)) },
		"a width as it is not":  func() { s.SetWidthAs(core.UnitsAs(7)) },
		"an empty name":         func() { s.SetName("  ") },
		"a negative width":      func() { s.SetWidth(-1) },
		"a width not a number":  func() { s.SetWidth(math.NaN()) },
		"a negative stipple":    func() { s.SetStippleScale(-2) },
		"an infinite stipple":   func() { s.SetStippleScale(math.Inf(1)) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
	if s.Name() != before || s.Width() != width || s.StippleScale() != scale {
		t.Errorf("the style is named %q, width %g, stipple scale %g", s.Name(), s.Width(), s.StippleScale())
	}
	if !strings.Contains(s.String(), s.Name()) {
		t.Errorf("the style is %q", s)
	}
}

// colorTuple is the three channels of a colour, for comparing.
func colorTuple(red, green, blue int) string {
	return fmt.Sprintf("%d,%d,%d", red, green, blue)
}

// The text height's units and the text's alignment are the style
// screen's own, reached by a script: switching units keeps the size the
// text is drawn at.
func TestStyleTextUnitsAndAlignment(t *testing.T) {
	t.Parallel()
	d := New()
	s := d.NewStyle()
	s.SetTextHeightAs(core.UnitsPixels)
	s.SetTextHeight(20)
	s.SetTextHeightAs(core.UnitsMM)
	if s.TextHeightAs() != core.UnitsMM || s.TextHeight() == 20 {
		t.Errorf("text height %v as %v: want it converted to mm", s.TextHeight(), s.TextHeightAs())
	}
	s.SetTextHeightAs(core.UnitsPixels)
	if got := s.TextHeight(); got < 19.999 || got > 20.001 {
		t.Errorf("a round trip through mm gives %v, want 20", got)
	}
	s.SetWidthAs(core.UnitsPixels)
	s.SetWidth(3)
	s.SetStippleScale(15)
	s.SetWidthAs(core.UnitsMM)
	if s.WidthAs() != core.UnitsMM || s.Width() == 3 || s.StippleScale() == 15 {
		t.Errorf("width %v, stipple %v as %v: want both converted to mm", s.Width(), s.StippleScale(), s.WidthAs())
	}
	s.SetWidthAs(core.UnitsPixels)
	if w, st := s.Width(), s.StippleScale(); math.Abs(w-3) > 1e-9 || math.Abs(st-15) > 1e-9 {
		t.Errorf("a round trip through mm gives width %v, stipple %v, want 3 and 15", w, st)
	}
	if h, v := s.TextAlign(); h != "center" || v != "center" {
		t.Errorf("a new style aligns %s, %s", h, v)
	}
	d.Host().(*SketchHost).BeginEval()
	s.SetTextAlign("right", "top")
	if h, v := s.TextAlign(); h != "right" || v != "top" {
		t.Errorf("aligned %s, %s, want right, top", h, v)
	}
	d.Host().(*SketchHost).BeginEval()
	s.SetTextAlign("left", "bottom")
	if h, v := s.TextAlign(); h != "left" || v != "bottom" {
		t.Errorf("aligned %s, %s, want left, bottom", h, v)
	}
	d.Edit.Undo()
	if h, v := s.TextAlign(); h != "right" || v != "top" {
		t.Errorf("after undo aligned %s, %s, want right, top", h, v)
	}
	failsWith(t, "horizontal alignment", func() { s.SetTextAlign("up", "top") })
	failsWith(t, "vertical alignment", func() { s.SetTextAlign("left", "up") })
	failsWith(t, "text height is in pixels", func() { s.SetTextHeightAs(7) })
}

// ResetStyles puts the program's styles back and leaves the user's own,
// as one step of undo.
func TestResetStyles(t *testing.T) {
	t.Parallel()
	d := New()
	own := d.NewStyle()
	own.SetWidth(9)
	def := d.Style("#def-solid-edge")
	def.SetColor(1, 2, 3)
	d.Host().(*SketchHost).BeginEval() // a script's changes are one step: close it
	d.ResetStyles()
	if r, g, b := def.Color(); r == 1 && g == 2 && b == 3 {
		t.Error("the default style kept its colour")
	}
	if own.Width() != 9 {
		t.Errorf("the user's style has width %v, want 9", own.Width())
	}
	d.Edit.Undo()
	if r, g, b := def.Color(); r != 1 || g != 2 || b != 3 {
		t.Errorf("after undo the colour is %d %d %d", r, g, b)
	}
}

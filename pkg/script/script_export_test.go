package script

import (
	"context"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// rebuild runs d's script on a new document and gives it, failing the
// test on a note or an error.
func rebuild(t *testing.T, d *api.Doc) *api.Doc {
	t.Helper()
	src, notes := d.Script()
	if len(notes) > 0 {
		t.Errorf("not written: %v\n%s", notes, src)
	}
	sk := sketch.NewSketch()
	sk.NewFile()
	host := api.NewSketchHost(sk, "")
	if err := RunSource(context.Background(), Options{Host: host}, "model.go", src); err != nil {
		t.Fatalf("running the script: %v\n%s", err, src)
	}
	return api.NewDoc(host)
}

// Splines through more points, open and closed, come back point for
// point.
func TestScriptSplines(t *testing.T) {
	t.Parallel()
	d := api.New()
	open := d.Sketch.Spline(api.V(0, 0, 0), api.V(5, 5, 0), api.V(10, 0, 0), api.V(15, -5, 0), api.V(20, 0, 0), api.V(25, 5, 0))
	closed := d.Sketch.PeriodicSpline(api.V(0, 20, 0), api.V(10, 30, 0), api.V(20, 20, 0), api.V(10, 10, 0))
	made := rebuild(t, d)
	for _, e := range []api.Entity{open, closed} {
		want := e.Points()
		got := made.Entity(e.H.V).Points()
		if len(got) != len(want) {
			t.Fatalf("%v has %d points, want %d", e.H, len(got), len(want))
		}
		for i := range want {
			if got[i].Pos().Minus(want[i].Pos()).Magnitude() > 1e-9 {
				t.Errorf("%v point %d at %v, want %v", e.H, i, got[i].Pos(), want[i].Pos())
			}
		}
	}
	if typ := made.Entity(closed.H.V).Request().Type(); typ != sketch.RequestCubicPeriodic {
		t.Errorf("the closed spline came back as %v", typ)
	}
}

// Styles, a style's settings, a request and a comment drawn in one, a
// kerned text and an equal angle between four lines come back.
func TestScriptStylesTextAngles(t *testing.T) {
	t.Parallel()
	d := api.New()
	st := d.NewStyle()
	st.SetName("thick")
	st.SetColor(200, 10, 20)
	st.SetWidth(4)
	st.SetTextAlign("left", "top")
	d.Style("#def-construction").SetZIndex(7)
	line := d.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	line.Request().SetStyle(st)
	note := d.Constrain.Comment(api.V(5, 5, 0), "here")
	note.SetStyle(st)
	text := d.Sketch.Text("To", "Gentium-R.ttf", api.V(0, 20, 0), 10)
	text.SetKerning(true)
	l2 := d.Sketch.Line(api.V(0, 0, 0), api.V(10, 10, 0))
	l3 := d.Sketch.Line(api.V(20, 0, 0), api.V(30, 0, 0))
	l4 := d.Sketch.Line(api.V(20, 0, 0), api.V(30, 10, 0))
	d.Constrain.EqualAngle(line, l2, l3, l4)

	made := rebuild(t, d)
	got := made.Style("thick")
	if r, g, b := got.Color(); r != 200 || g != 10 || b != 20 || got.Width() != 4 {
		t.Errorf("the style came back as %d %d %d, width %g", r, g, b, got.Width())
	}
	if h, v := got.TextAlign(); h != "left" || v != "top" {
		t.Errorf("the style's text is aligned %s %s", h, v)
	}
	if z := made.Style("#def-construction").ZIndex(); z != 7 {
		t.Errorf("the construction style's z index is %d", z)
	}
	if s := made.Entity(line.H.V).Request().Style(); s.Name() != "thick" {
		t.Errorf("the line is drawn in %q", s.Name())
	}
	var comment, angle bool
	for _, c := range made.Active().Constraints() {
		switch c.Type() {
		case core.ConstraintComment:
			comment = c.Style().Name() == "thick"
		case core.ConstraintEqualAngle:
			angle = true
		}
	}
	if !comment || !angle {
		t.Errorf("the comment's style came back %v, the equal angle %v", comment, angle)
	}
	mt := made.Entity(text.H.V)
	if !mt.Kerning() {
		t.Errorf("the text isn't kerned")
	}
	for i, p := range text.Points() {
		if q := mt.Points()[i].Pos(); q.Minus(p.Pos()).Magnitude() > 1e-9 {
			t.Errorf("the text's corner %d is at %v, want %v", i, q, p.Pos())
		}
	}
}

// A face's own appearance comes back on the face at the same place.
func TestScriptFaceAppearance(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	box := d.Features.Extrude(5, "box")
	top := api.V(5, 5, 5)
	d.View.SetFaceAppearance(box.Solid().FaceAt(top), "metal")
	src, _ := d.Script()
	if !strings.Contains(src, `"metal"`) {
		t.Fatalf("the script has no face appearance:\n%s", src)
	}
	made := rebuild(t, d)
	g := made.Groups()[len(made.Groups())-1]
	if a := made.View.FaceAppearance(g.Solid().FaceAt(top)); a != "metal" {
		t.Errorf("the top face's appearance is %q", a)
	}
	if a := made.View.FaceAppearance(g.Solid().FaceAt(api.V(5, 5, 0))); a == "metal" {
		t.Errorf("the bottom face took the top's appearance")
	}
}

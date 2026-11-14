package script

import (
	"context"
	"math"
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

// rebuildNoted runs d's script on a new document and gives it with the
// script and its notes, failing the test on an error.
func rebuildNoted(t *testing.T, d *api.Doc) (made *api.Doc, notes []string, src string) {
	t.Helper()
	src, notes = d.Script()
	sk := sketch.NewSketch()
	sk.NewFile()
	host := api.NewSketchHost(sk, "")
	if err := RunSource(context.Background(), Options{Host: host}, "model.go", src); err != nil {
		t.Fatalf("running the script: %v\n%s", err, src)
	}
	return api.NewDoc(host), notes, src
}

// threadedPlate is a plate with a hole of 8.5, threaded M10 on the
// hole's face, and a slot cut after it; and the thread's groups: blank,
// lathe, ridge sketch and helix.
func threadedPlate(t *testing.T) (*api.Doc, []api.Group) {
	t.Helper()
	d := api.New()
	d.Sketch.Rect(api.V(-15, -15, 0), api.V(15, 15, 0))
	d.Features.Extrude(10, "plate")
	d.Features.SketchIn(d.Workplane("XY"), "hole")
	d.Sketch.Circle(api.V(0, 0, 0), 4.25)
	hole := d.Features.Extrude(12, "drill")
	hole.SetCombine(sketch.CombineDifference)
	th := d.Features.ThreadOnFace(hole.Solid().FaceAt(api.V(4.25, 0, 5)), api.ThreadOptions{Length: 6})
	d.Features.SketchIn(d.Workplane("XY"), "slot-outline")
	d.Sketch.Rect(api.V(10, -2, 0), api.V(14, 2, 0))
	slot := d.Features.Extrude(12, "slot")
	slot.SetCombine(sketch.CombineDifference)
	return d, th.AsThread().Groups()
}

// sameSolid fails the test unless the last group of got is sound and
// has the volume of want's, to the tolerance their curves are followed
// at.
func sameSolid(t *testing.T, want, got *api.Doc, src string) {
	t.Helper()
	wg, gg := want.Groups(), got.Groups()
	w, g := wg[len(wg)-1].Analyze(), gg[len(gg)-1].Analyze()
	r := g.Check()
	if !r.Sound() || math.Abs(w.Volume()-g.Volume()) > 1e-3*w.Volume() {
		t.Errorf("made again: volume %.4f, sound %v; want %.4f\n%s", g.Volume(), r.Sound(), w.Volume(), src)
	}
}

// A thread made on a face whose lathe and helix were moved on their own
// to the end, as one moves a bore after the cuts, is written group by
// group: its datum and sketches with the rest, and its helix as a
// helix, so that the script runs and makes the same part.
func TestScriptThreadGroupsApart(t *testing.T) {
	t.Parallel()
	d, groups := threadedPlate(t)
	for _, i := range []int{1, 3} {
		if err := groups[i].History().MoveToEnd(); err != nil {
			t.Fatal(err)
		}
	}
	// Made again up to the end, where the slot is cut first.
	d.Activate(groups[3])
	made, notes, src := rebuildNoted(t, d)
	if !strings.Contains(strings.Join(notes, "\n"), "written as the helix of its ridge") {
		t.Errorf("notes %q", notes)
	}
	sameSolid(t, d, made, src)
}

// A thread whose face is gone from the solid before it keeps its datum
// where it was; the script draws the datum's sketch through a point
// where it lies, and puts the thread's circle on that sketch's origin,
// so that the thread is cut where it was.
func TestScriptThreadDatumFaceGone(t *testing.T) {
	t.Parallel()
	d, _ := threadedPlate(t)
	// The drill's solid left out: the hole's face is gone, and the
	// thread's own boring makes the hole.
	d.Group("drill").SetSuppress(api.SuppressSolid)
	datum := d.Group("M10-end")
	if p := datum.Problem(); !strings.Contains(p, "no longer in the solid") {
		t.Fatalf("the datum's problem is %q", p)
	}
	made, _, src := rebuildNoted(t, d)
	if !strings.Contains(src, ".Workplane().Center()") {
		t.Errorf("the circle isn't put on its datum's origin:\n%s", src)
	}
	sameSolid(t, d, made, src)
}

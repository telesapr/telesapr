package api

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// SetCopies after SetExpr drops the expression: it would otherwise be
// evaluated again at the next regeneration and put its own value back.
func TestSetCopiesClearsExpression(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(2, 2, 0))
	d.Features.Extrude(2, "")
	g := d.Features.Translate(2, V(5, 0, 0), "")
	rep := g.AsRepeat()
	if err := g.SetExpr(sketch.GroupCopies, "2+2"); err != nil {
		t.Fatal(err)
	}
	if n := rep.Copies(); n != 4 {
		t.Fatalf("expression gave %d copies, want 4", n)
	}
	rep.SetCopies(7)
	if n, e := rep.Copies(), g.Expr(sketch.GroupCopies); n != 7 || e != "" {
		t.Errorf("copies %d, expression %q; want 7 and none", n, e)
	}
	d.Host().Sketch().GenerateAll()
	if n := rep.Copies(); n != 7 {
		t.Errorf("after regeneration %d copies, want 7", n)
	}
}

// A rotate keeps the angle the script gave when its count changes; the
// window re-spaces the copies, a script states its angle itself.
func TestSetCopiesRespacesRotate(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(4, -1, 0), V(6, 1, 0))
	d.Features.Extrude(2, "")
	rg := d.Features.Rotate(2, d.Origin(), d.Workplane("XY").Normal(), 90, "")
	rot := rg.AsRepeat()
	rot.SetCopies(5)
	sk := d.Host().Sketch()
	got := sk.MustParam(rg.H.Param(3)).Val
	if want := math.Pi / 8; math.Abs(got-want) > 1e-12 {
		t.Errorf("angle guess %g, want %g", got, want)
	}
}

// SetColor keeps the opacity the group has.
func TestSetColorKeepsOpacity(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(2, 2, 0))
	ex := d.Features.Extrude(2, "")
	sk := d.Host().Sketch()
	sk.Group(ex.H).Color.Alpha = 100
	ex.SetColor(10, 20, 30)
	c := sk.Group(ex.H).Color
	if c.Red != 10 || c.Green != 20 || c.Blue != 30 || c.Alpha != 100 {
		t.Errorf("colour %+v, want 10 20 30 with alpha 100", c)
	}
}

// SetCombine leaves the extrusion as the script gave it, so a cut with a
// positive depth goes that way, and refuses a mode that does not exist.
func TestSetCombineKeepsExtrusion(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(2, 2, 0))
	ex := d.Features.Extrude(2, "")
	sk := d.Host().Sketch()
	z := func() float64 { return sk.Group(ex.H).ExtrusionGetVector(sk).Z }
	before := z()
	ex.SetCombine(sketch.CombineDifference)
	if got := z(); got != before || sk.Group(ex.H).MeshCombine != sketch.CombineDifference {
		t.Errorf("vector z %g, want %g, and the mode set", got, before)
	}
	failsWith(t, "not known", func() { ex.SetCombine(sketch.CombineAs(9)) })
	if sk.Group(ex.H).MeshCombine != sketch.CombineDifference {
		t.Error("a refused mode changed the group")
	}
}

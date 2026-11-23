package api

import (
	"math"
	"path/filepath"
	"testing"
)

// A workplane drawn in 3d turns to the orientation a script gives it,
// by itself or by its normal, the sketch solves, and the turn is one
// step of undo; a circle in a workplane has a normal that is fixed.
func TestEntityOrientation(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.Sketch3D("free")
	wp := d.Sketch.NewWorkplane(V(5, 5, 5), V(1, 0, 0), V(0, 1, 0))

	wp.SetOrientation(V(0, 1, 0), V(0, 0.5, 2))
	u, v := wp.Orientation()
	if !nearVec(u, V(0, 1, 0)) || !nearVec(v, V(0, 0, 1)) {
		t.Errorf("orientation %v %v", u, v)
	}
	if nu, _ := wp.Normal().Orientation(); !nearVec(nu, u) {
		t.Errorf("the normal has u %v, the workplane %v", nu, u)
	}
	if !d.Active().Solved() {
		t.Error("the sketch didn't solve after the turn")
	}
	// A step of its own, as each call of a script run from the window is.
	d.Host().(*SketchHost).BeginEval()
	wp.Normal().SetOrientation(V(1, 0, 0), V(0, 1, 0))
	if u, _ := wp.Orientation(); !nearVec(u, V(1, 0, 0)) {
		t.Errorf("u %v after turning the normal", u)
	}
	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	if u, _ := wp.Orientation(); !nearVec(u, V(0, 1, 0)) {
		t.Errorf("u %v after undo", u)
	}

	mustFail(t, "parallel", "must differ", func() { wp.SetOrientation(V(1, 0, 0), V(2, 0, 0)) })
	mustFail(t, "nan", "vector of numbers", func() { wp.SetOrientation(V(math.NaN(), 0, 0), V(0, 1, 0)) })
	pt := d.Sketch.Point(V(1, 2, 3))
	mustFail(t, "point", "no normal", func() { pt.SetOrientation(V(1, 0, 0), V(0, 1, 0)) })
	mustFail(t, "no normal", "no normal", func() { pt.Orientation() })

	d.Features.SketchIn(wp, "")
	c := d.Sketch.Circle(V(1, 1, 0), 2)
	mustFail(t, "fixed", "fixed normal", func() { c.SetOrientation(V(1, 0, 0), V(0, 1, 0)) })
}

// A point of a linked part turns the part about the point, as Shift or
// Ctrl dragging it does, in one step of undo; a point of a sketch, a
// zero axis and an angle that is no number are refused.
func TestEntityTurn(t *testing.T) {
	t.Parallel()
	d := New()
	g, err := d.Assembly.Link(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "box.step"), "")
	if err != nil {
		t.Fatal(err)
	}
	var pt Entity
	for _, e := range g.Entities() {
		if e.Kind() == "point" && e.Pos().Magnitude() > 1 {
			pt = e
			break
		}
	}
	if pt.H.V == 0 {
		t.Fatal("no point of the part away from its origin")
	}
	at := pt.Pos()
	d.Host().(*SketchHost).BeginEval()
	pt.Turn(V(0, 0, 1), 90)
	if u, _ := g.AsLink().Orientation(); !nearVec(pt.Pos(), at) || u.Minus(V(0, 1, 0)).Magnitude() > 1e-9 {
		t.Errorf("turned a quarter about z through %v: the point at %v, the part's x %v", at, pt.Pos(), u)
	}
	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	if u, _ := g.AsLink().Orientation(); !nearVec(u, V(1, 0, 0)) {
		t.Errorf("after undo the part's x is %v", u)
	}
	mustFail(t, "no axis", "zero vector", func() { pt.Turn(V(0, 0, 0), 10) })
	mustFail(t, "nan", "a turn of", func() { pt.Turn(V(0, 0, 1), math.NaN()) })
	d.Features.Sketch3D("free")
	p := d.Sketch.Point(V(1, 2, 3))
	mustFail(t, "sketch point", "linked part", func() { p.Turn(V(0, 0, 1), 10) })
}

// nearVec reports whether two vectors are within a nanometre.
func nearVec(a, b Vec) bool { return a.Minus(b).Magnitude() < 1e-9 }

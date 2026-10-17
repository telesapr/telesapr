package api

import (
	"math"
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

// nearVec reports whether two vectors are within a nanometre.
func nearVec(a, b Vec) bool { return a.Minus(b).Magnitude() < 1e-9 }

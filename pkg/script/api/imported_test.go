package api

import (
	"math"
	"path/filepath"
	"testing"
)

// An arc drawn in 3d keeps the plane it was given and turns as
// SetOrientation turns it; it is refused in a workplane, where Arc
// draws one, and for a plane of one direction.
func TestArcIn3D(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.Sketch3D("free")
	a := d.Sketch.ArcIn3D(V(0, 0, 0), V(5, 0, 0), V(0, 0, 5), V(1, 0, 0), V(0, 0, 1))
	d.Edit.Regen()
	if u, v := a.Orientation(); u.Minus(V(1, 0, 0)).Magnitude() > 1e-9 || v.Minus(V(0, 0, 1)).Magnitude() > 1e-9 {
		t.Errorf("the arc's plane is %v, %v", u, v)
	}
	if p := a.End().Pos(); p.Minus(V(0, 0, 5)).Magnitude() > 1e-9 {
		t.Errorf("the arc ends at %v", p)
	}
	failsWith(t, "must differ", func() { d.Sketch.ArcIn3D(V(0, 0, 0), V(1, 0, 0), V(0, 1, 0), V(1, 0, 0), V(2, 0, 0)) })
	d.Features.SketchIn(d.Workplane("XY"), "flat")
	failsWith(t, "in a workplane, Arc", func() {
		d.Sketch.ArcIn3D(V(0, 0, 0), V(1, 0, 0), V(0, 1, 0), V(1, 0, 0), V(0, 1, 0))
	})
}

// A workplane drawn in a workplane has its origin in it, two degrees
// of freedom, and lies parallel to it. It is refused in 3d, where
// NewWorkplane draws one.
func TestNewWorkplaneIn(t *testing.T) {
	t.Parallel()
	d := New()
	g := d.Features.SketchIn(d.Workplane("XZ"), "upright")
	d.Edit.Regen()
	before := g.Dof()
	wp := d.Sketch.NewWorkplaneIn(V(3, 0, 4))
	d.Edit.Regen()
	if got := g.Dof() - before; got != 2 {
		t.Errorf("the workplane adds %d degrees of freedom, want 2, its origin's", got)
	}
	u, v := wp.Orientation()
	if wu, wv := d.Workplane("XZ").Orientation(); u.Minus(wu).Magnitude() > 1e-9 || v.Minus(wv).Magnitude() > 1e-9 {
		t.Errorf("the workplane is turned to %v, %v, want XZ's %v, %v", u, v, wu, wv)
	}
	d.Features.Sketch3D("free")
	failsWith(t, "NewWorkplane draws one in 3d", func() { d.Sketch.NewWorkplaneIn(V(0, 0, 0)) })
}

// Again adds an equal constraint, which Comment and Raw never do, as one
// undo step.
func TestConstrainAgain(t *testing.T) {
	t.Parallel()
	d := New()
	step := d.Host().(*SketchHost).BeginEval
	g := d.Features.SketchIn(d.Workplane("XY"), "")
	l := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	h := d.Constrain.Horizontal(l)
	step()
	if again := d.Constrain.Horizontal(l); again != h {
		t.Errorf("a second horizontal was added")
	}
	step()
	dup := d.Constrain.Again(h)
	if dup == h || len(g.Constraints()) != 2 || dup.Type() != h.Type() {
		t.Errorf("Again gave %v of %v; the group has %d constraints", dup, h, len(g.Constraints()))
	}
	step()
	d.Edit.Undo()
	if n := len(g.Constraints()); n != 1 {
		t.Errorf("after undo the group has %d constraints, want 1", n)
	}
}

// A picture whose file isn't there keeps its frame and path: what a
// drawing with a picture gone has, and File > Export > Script writes.
func TestImageFrameMissing(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.SketchIn(d.Workplane("XY"), "")
	missing := filepath.Join(t.TempDir(), "gone.png")
	img := d.Sketch.ImageFrame(missing, V(1, 2, 0), 10, 30)
	d.Edit.Regen()
	pts := img.Points()
	if len(pts) != 4 {
		t.Fatalf("the image has %d points", len(pts))
	}
	if w := pts[2].Pos().Minus(pts[1].Pos()).Magnitude(); math.Abs(w-30) > 1e-9 {
		t.Errorf("the frame is %g wide, want 30", w)
	}
	if h := pts[0].Pos().Minus(pts[1].Pos()).Magnitude(); math.Abs(h-10) > 1e-9 {
		t.Errorf("the frame is %g high, want 10", h)
	}
	failsWith(t, "can't be", func() { d.Sketch.ImageFrame(missing, V(0, 0, 0), 0, 1) })
}

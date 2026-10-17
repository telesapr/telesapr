package api

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// constraintPoints are the points constraint c holds.
func constraintPoints(d *Doc, c Constraint) (a, b core.HEntity) {
	d.read(func(sk *sketch.Sketch) {
		k := c.get(sk)
		a, b = k.PtA, k.PtB
	})
	return a, b
}

// A finished spline with constraints on its points keeps them, on the
// points they were meant for, when it is closed: the last point is
// merged into the first, the points after the start control point move
// down one place, and what held a control point goes with it. It is
// one undo step.
func TestMakePeriodicConstraints(t *testing.T) {
	t.Parallel()
	d := New()
	step := d.Host().(*SketchHost).BeginEval // each a console line's undo step
	s := d.Sketch.Cubic(V(-30, 0, 0), V(-20, 15, 0), V(20, -15, 0), V(30, 0, 0))
	for _, p := range []Vec{V(-20, 15, 0), V(0, 0, 0), V(20, -15, 0)} {
		s.AddSplinePoint(p)
	}
	pts := s.Points()
	if len(pts) != 7 {
		t.Fatalf("the spline has %d points", len(pts))
	}
	anchor := d.Sketch.Line(V(0, 50, 0), V(10, 50, 0)).Start()
	dist := func(p Entity) Constraint {
		return d.Constrain.Distance(anchor, p, anchor.Pos().DistanceTo(p.Pos()))
	}
	onLast, onInner, onControl, onEndControl := dist(pts[6]), dist(pts[3]), dist(pts[1]), dist(pts[5])
	loop := d.Constrain.Coincident(pts[0], pts[6])
	step()

	s.MakePeriodic()

	if _, b := constraintPoints(d, onLast); b != pts[0].H {
		t.Errorf("the constraint on the last point is on e%03x, want the first, e%03x", b.V, pts[0].H.V)
	}
	// The inner point was the fourth and is the third now: the entity
	// of the third point.
	if _, b := constraintPoints(d, onInner); b != pts[2].H {
		t.Errorf("the constraint on the inner point is on e%03x, want e%03x", b.V, pts[2].H.V)
	}
	for name, c := range map[string]Constraint{
		"a control point":       onControl,
		"the end control point": onEndControl,
		"the two ends":          loop,
	} {
		if c.Valid() {
			t.Errorf("the constraint on %s is still there", name)
		}
	}
	if s.Kind() != "periodic-cubic" || len(s.Points()) != 4 {
		t.Errorf("closing gave a %s of %d points", s.Kind(), len(s.Points()))
	}
	if !d.Active().Solved() {
		t.Errorf("the sketch does not solve: %s", d.Active().Problem())
	}

	step()
	if !d.Edit.Undo() || s.Kind() != "cubic" || len(s.Points()) != 7 {
		t.Fatalf("undo: a %s of %d points", s.Kind(), len(s.Points()))
	}
	if !onControl.Valid() || !loop.Valid() {
		t.Error("undo did not bring the constraints back")
	}
	if _, b := constraintPoints(d, onLast); b != pts[6].H {
		t.Errorf("after undo the last point's constraint is on e%03x", b.V)
	}
}

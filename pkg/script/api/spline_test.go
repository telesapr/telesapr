package api

import (
	"strings"
	"testing"
)

// failsWith runs fn and reports whether it failed with an error whose
// text holds want.
func failsWith(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		t.Helper()
		err, ok := recover().(error)
		if !ok || !strings.Contains(err.Error(), want) {
			t.Errorf("failed with %v, want an error holding %q", err, want)
		}
	}()
	fn()
}

// A point added to an open cubic goes in the span nearest it and the
// curve passes through it; removed, the spline has its four points
// again; each is one undo step.
func TestSplinePoints(t *testing.T) {
	t.Parallel()
	d := New()
	step := d.Host().(*SketchHost).BeginEval // each a console line's undo step
	d.Features.SketchIn(d.Workplane("XY"), "")
	c := d.Sketch.Cubic(V(0, 0, 0), V(10, 10, 0), V(20, 10, 0), V(30, 0, 0))
	if n := len(c.Points()); n != 4 {
		t.Fatalf("a new cubic has %d points", n)
	}
	step()
	c.AddSplinePoint(V(15, 12, 0))
	pts := c.Points()
	if len(pts) != 5 {
		t.Fatalf("%d points after one was added", len(pts))
	}
	if p := pts[2].Pos(); p.DistanceTo(V(15, 12, 0)) > 1e-9 {
		t.Errorf("the new point is at %v", p)
	}

	failsWith(t, "an end", func() { pts[0].RemoveSplinePoint() })
	failsWith(t, "an end", func() { pts[1].RemoveSplinePoint() })

	step()
	pts[2].RemoveSplinePoint()
	if n := len(c.Points()); n != 4 {
		t.Fatalf("%d points after one was removed", n)
	}
	failsWith(t, "an end", func() { c.Points()[2].RemoveSplinePoint() })
	step()

	if !d.Edit.Undo() || len(c.Points()) != 5 {
		t.Errorf("undo of the removal gave %d points", len(c.Points()))
	}
	if !d.Edit.Undo() || len(c.Points()) != 4 {
		t.Errorf("undo of the addition gave %d points", len(c.Points()))
	}
	failsWith(t, "not part of a cubic spline", func() { d.Sketch.Line(V(0, 0, 0), V(5, 0, 0)).AddSplinePoint(V(1, 1, 0)) })
}

// The coincidences of a point are deleted, and a point without any says
// so.
func TestDeleteCoincidences(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.SketchIn(d.Workplane("XY"), "")
	a := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	b := d.Sketch.Line(V(10, 0, 0), V(10, 10, 0))
	d.Constrain.Coincident(a.End(), b.Start())
	n := len(d.Active().Constraints())
	a.End().DeleteCoincidences()
	if got := len(d.Active().Constraints()); got != n-1 {
		t.Errorf("%d constraints after deleting the coincidence, want %d", got, n-1)
	}
	failsWith(t, "coincident with no other", func() { a.End().DeleteCoincidences() })
	failsWith(t, "not a point", func() { a.DeleteCoincidences() })
}

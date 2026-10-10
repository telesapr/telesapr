package api

import (
	"testing"
)

// A line moved by a vector keeps its length and moves both ends; an
// end held on the origin comes back to it; what has
// no points of its own can't be moved.
func TestMoveBy(t *testing.T) {
	t.Parallel()
	d := New()
	l := d.Line(V(0, 0, 0), V(10, 0, 0))
	d.Regen()
	l.MoveBy(V(1, 2, 0))
	if l.Start().Pos().DistanceTo(V(1, 2, 0)) > 1e-9 || l.End().Pos().DistanceTo(V(11, 2, 0)) > 1e-9 {
		t.Errorf("moved to %v, %v", l.Start().Pos(), l.End().Pos())
	}
	d.Coincident(l.Start(), d.Origin())
	d.Regen()
	l.Start().MoveBy(V(5, 5, 0))
	if l.Start().Pos().DistanceTo(V(0, 0, 0)) > 1e-9 {
		t.Errorf("a point on the origin moved to %v", l.Start().Pos())
	}
	c := d.Circle(V(20, 0, 0), 3)
	c.MoveBy(V(0, 1, 0))
	if c.Center().Pos().DistanceTo(V(20, 1, 0)) > 1e-9 || c.Radius() != 3 {
		t.Errorf("a circle moved to %v, radius %g", c.Center().Pos(), c.Radius())
	}
	failsWith(t, "no points to move", func() { d.Workplane("XY").MoveBy(V(1, 0, 0)) })
}

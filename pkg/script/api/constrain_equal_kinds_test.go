package api

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// Four entities are taken by kind, not by place: two line segments and
// two points in any order make the same constraint, the first point
// measured from the first line segment.
func TestEqualFourInAnyOrder(t *testing.T) {
	t.Parallel()
	// Indexes into l1, l2, p1, p2.
	orders := [][]int{{0, 1, 2, 3}, {0, 2, 1, 3}, {1, 0, 3, 2}, {2, 3, 0, 1}, {1, 3, 0, 2}}
	for _, order := range orders {
		d := New()
		l1 := d.Sketch.Line(V(0, 0, 0), V(20, 0, 0))
		l2 := d.Sketch.Line(V(0, 10, 0), V(20, 10, 0))
		p1 := d.Sketch.Point(V(5, 4, 0))
		p2 := d.Sketch.Point(V(5, 12, 0))
		kinds := []Entity{l1, l2, p1, p2}
		es := make([]Entity, 4)
		for i, j := range order {
			es[i] = kinds[j]
		}
		c := d.Constrain.Equal(es...)
		if c.Type() != core.ConstraintEqPtLnDistances {
			t.Fatalf("order %v: the constraint is a %v", order, c.Type())
		}
	}
}

// Three or more line segments, or circles and arcs, make one constraint
// of the first against each other one, as one undo step.
func TestEqualMany(t *testing.T) {
	t.Parallel()
	d := New()
	lines := []Entity{
		d.Sketch.Line(V(0, 0, 0), V(10, 0, 0)),
		d.Sketch.Line(V(0, 5, 0), V(12, 5, 0)),
		d.Sketch.Line(V(0, 9, 0), V(15, 9, 0)),
		d.Sketch.Line(V(0, 13, 0), V(9, 13, 0)),
	}
	n := len(d.Active().Constraints())
	first := d.Constrain.Equal(lines...)
	if first.Type() != core.ConstraintEqualLengthLines {
		t.Fatalf("the constraint is a %v", first.Type())
	}
	if got := len(d.Active().Constraints()); got != n+3 {
		t.Fatalf("%d constraints, want %d", got, n+3)
	}
	if !d.Edit.Undo() || len(d.Active().Constraints()) != n {
		t.Errorf("one undo leaves %d constraints, want %d", len(d.Active().Constraints()), n)
	}

	c := New()
	rounds := []Entity{
		c.Sketch.Circle(V(0, 0, 0), 3),
		c.Sketch.Circle(V(20, 0, 0), 4),
		c.Sketch.Arc(V(40, 0, 0), V(45, 0, 0), V(40, 5, 0)),
	}
	m := len(c.Active().Constraints())
	rc := c.Constrain.Equal(rounds...)
	if rc.Type() != core.ConstraintEqualRadius {
		t.Fatalf("the constraint is a %v", rc.Type())
	}
	if got := len(c.Active().Constraints()); got != m+2 {
		t.Fatalf("%d constraints, want %d", got, m+2)
	}
	if !c.Active().Solved() {
		t.Errorf("the sketch does not solve: %s", c.Active().Problem())
	}
}

// Entities that don't fit make an error, not a constraint or a panic
// out of the solver.
func TestEqualBadMix(t *testing.T) {
	t.Parallel()
	d := New()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	other := d.Sketch.Line(V(0, 5, 0), V(10, 5, 0))
	circle := d.Sketch.Circle(V(0, 20, 0), 3)
	point := d.Sketch.Point(V(3, 3, 0))
	n := len(d.Active().Constraints())
	failsWith(t, "cannot be made equal", func() { d.Constrain.Equal(line, circle) })
	failsWith(t, "cannot be made equal", func() { d.Constrain.Equal(line, other, circle) })
	failsWith(t, "cannot be made equal", func() { d.Constrain.Equal(line, other, line, point) })
	failsWith(t, "cannot be made equal", func() { d.Constrain.Equal(point, point) })
	failsWith(t, "cannot be made equal", func() { d.Constrain.Equal(line) })
	if got := len(d.Active().Constraints()); got != n {
		t.Errorf("%d constraints after the failures, want %d", got, n)
	}
}

package api

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// crossedLines draws a line of the given length along x from the
// origin and a line along y from it, and holds every point of both
// where they are drawn, so the only thing a constraint can move is
// what else is drawn.
func crossedLines(d *Doc, length float64) (along, up Entity) {
	along = d.Sketch.Line(V(0, 0, 0), V(length, 0, 0))
	up = d.Sketch.Line(V(0, 0, 0), V(0, 10, 0))
	for _, l := range []Entity{along, up} {
		d.Constrain.Lock(l.Start())
		d.Constrain.Lock(l.End())
	}
	return along, up
}

// mirrorSetup draws a line along x, a construction line along y through
// the same end, and a point on it, and holds the line's start there.
// The point may then be moved along the construction line.
func mirrorSetup(d *Doc) (along, up, point Entity) {
	along = d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	d.Constrain.Horizontal(along)
	up = d.Sketch.Line(V(0, 0, 0), V(0, 10, 0))
	up.SetConstruction(true)
	d.Constrain.Vertical(up)
	d.Constrain.Lock(along.Start())
	point = d.Sketch.Point(V(0, 4, 0))
	d.Constrain.OnSegment(point, up)
	return along, up, point
}

// A point as far from a line segment as that line segment is long: the
// line's length, and the distance from the point to another line segment.
func TestEqualLengthPointDistance(t *testing.T) {
	for _, at := range []Vec{V(4, 3, 0), V(14, 3, 0), V(-30, -20, 0)} {
		d := New()
		along, up := crossedLines(d, 10)
		point := d.Sketch.Point(at)
		c := d.Constrain.Equal(along, up, point)
		if c.Type() != core.ConstraintEqLenPtLineD {
			t.Fatalf("the constraint is a %v", c.Type())
		}
		// Both line segments are held where they are drawn, so the
		// point's distance from the one along y is its x, which the
		// constraint makes the length of the one along x.
		if got, want := math.Abs(point.Pos().X), 10.0; !near(got, want) {
			t.Errorf("a point drawn at %v is %g from the line, want the line's length %g", at, got, want)
		}
		if !d.Active().Solved() {
			t.Errorf("the sketch does not solve: %s", d.Active().Problem())
		}
	}
}

// Two points as far from a line segment as each other, measured from
// one line segment or from one each.
func TestEqualPointLineDistances(t *testing.T) {
	d := New()
	along, up, point := mirrorSetup(d)
	second := d.Sketch.Point(V(0, -4, 0))
	d.Constrain.On(second, up)
	// One line segment, both points: the distances are equal, so the
	// points stay as far from the line as each other.
	one := d.Constrain.Equal(along, point, second)
	if one.Type() != core.ConstraintEqPtLnDistances {
		t.Fatalf("the constraint is a %v", one.Type())
	}
	point.MoveTo(V(0, 9, 0))
	if got, moved := second.Pos().Y, point.Pos().Y; !near(got, -moved) || near(got, 0) {
		t.Errorf("the points are at %g and %g, want as far on either side as each other", moved, got)
	}

	// Two line segments, a point each: each point is as far from its
	// own line segment as the other from its.
	other := New()
	cross, _, a := mirrorSetup(other)
	base := other.Sketch.Line(V(0, 0, 0), V(20, 0, 0))
	b := other.Sketch.Point(V(6, -6, 0))
	two := other.Constrain.Equal(cross, a, base, b)
	if two.Type() != core.ConstraintEqPtLnDistances {
		t.Fatalf("the constraint is a %v", two.Type())
	}
	if !other.Active().Solved() {
		t.Errorf("the second sketch does not solve: %s", other.Active().Problem())
	}
}

// Two angles as large as each other, as two pairs of line segments and
// as two that share one.
func TestEqualAngle(t *testing.T) {
	d := New()
	// Four rays from one point, the first and third of them horizontal:
	// the second is at 30 degrees and the fourth at 60, so the equality
	// has to turn the fourth.
	a := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	b := d.Sketch.Line(V(0, 0, 0), V(10, 5.7735026919, 0))
	c := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	e := d.Sketch.Line(V(0, 0, 0), V(5, 8.6602540378, 0))
	for _, l := range []Entity{a, b, c, e} {
		d.Constrain.Coincident(l.Start(), a.Start())
	}
	d.Constrain.Horizontal(a)
	d.Constrain.Horizontal(c)
	d.Constrain.Lock(a.Start())
	eq := d.Constrain.EqualAngle(a, b, c, e)
	if eq.Type() != core.ConstraintEqualAngle {
		t.Fatalf("the constraint is a %v", eq.Type())
	}
	if got := rayAngle(e); !near(got, rayAngle(b)) {
		t.Errorf("the fourth ray is at %g degrees, want the second's %g", got, rayAngle(b))
	}
	// Moving one of the rays turns the other with it.
	e.End().MoveTo(V(10, 20, 0))
	if got := rayAngle(e); !near(got, rayAngle(b)) {
		t.Errorf("the fourth ray is at %g degrees, want the second's %g", got, rayAngle(b))
	}

	// Three line segments make the angles that share the second, which
	// the constraint turns into the bisector.
	other := New()
	f := other.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	g := other.Sketch.Line(V(0, 0, 0), V(5, 8.6602540378, 0))
	h := other.Sketch.Line(V(0, 0, 0), V(0, 10, 0))
	for _, l := range []Entity{f, g, h} {
		other.Constrain.Coincident(l.Start(), f.Start())
	}
	other.Constrain.Horizontal(f)
	other.Constrain.Vertical(h)
	other.Constrain.Lock(f.Start())
	three := other.Constrain.EqualAngle(f, g, h)
	if three.Type() != core.ConstraintEqualAngle {
		t.Fatalf("the constraint is a %v", three.Type())
	}
	if got, want := rayAngle(g), 45.0; !near(got, want) {
		t.Errorf("the middle ray is at %g degrees, want the bisector's %g", got, want)
	}
}

// rayAngle is the angle in degrees a line segment makes with x.
func rayAngle(l Entity) float64 {
	dir := l.End().Pos().Minus(l.Start().Pos())
	return math.Atan2(dir.Y, dir.X) * 180 / math.Pi
}

// A distance measured along a direction, which is how the gap between
// two parallel faces is held.
func TestProjectedDistance(t *testing.T) {
	d := New()
	along := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	d.Constrain.Horizontal(along)
	a := d.Sketch.Point(V(0, 3, 0))
	b := d.Sketch.Point(V(5, 3, 0))
	d.Constrain.Lock(a)
	c := d.Constrain.ProjectedDistance(a, b, along, 5)
	if c.Type() != core.ConstraintProjPtDistance {
		t.Fatalf("the constraint is a %v", c.Type())
	}
	b.MoveTo(V(20, 3, 0))
	if got := b.Pos().X; !near(got, 5) {
		t.Errorf("the point is at %g along the line, want 5", got)
	}
	if !d.Active().Solved() {
		t.Errorf("the sketch does not solve: %s", d.Active().Problem())
	}
}

// What Equal is given, it cannot make equal; the misuse is an error and
// nothing is added.
func TestEqualMisuse(t *testing.T) {
	d := New()
	along, up, point := mirrorSetup(d)
	for name, fn := range map[string]func(){
		"one line":              func() { d.Constrain.Equal(along) },
		"a line and a point":    func() { d.Constrain.Equal(along, point) },
		"two points":            func() { d.Constrain.Equal(point, d.Sketch.Point(V(0, -4, 0))) },
		"five of them":          func() { d.Constrain.Equal(along, along, along, point, point) },
		"two angles of two":     func() { d.Constrain.EqualAngle(along, up) },
		"five angles":           func() { d.Constrain.EqualAngle(along, up, along, up, along) },
		"projected from a line": func() { d.Constrain.ProjectedDistance(along, point, along, 5) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
}

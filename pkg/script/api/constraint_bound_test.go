package api

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// boundLine is a sketch in XY with a line from the origin, its start
// locked there, along x for len.
func boundLine(t *testing.T, length float64) (*Doc, Group, Entity) {
	t.Helper()
	d := New()
	g := d.SketchIn(d.Workplane("XY"), "")
	l := d.Line(V(0, 0, 0), V(length, 0, 0))
	d.Lock(l.Start())
	d.Regen()
	return d, g, l
}

// A minimum or a maximum length holds where the line would cross it,
// and takes a degree of freedom only there; a line with room to spare
// keeps both its end's freedoms.
func TestDistanceBounds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		max         bool
		from, bound float64
		want        float64
		dof         int
	}{
		{false, 5, 10, 10, 1},
		{false, 20, 10, 20, 2},
		{true, 5, 10, 5, 2},
		{true, 20, 10, 10, 1},
	} {
		d, g, l := boundLine(t, tc.from)
		var c Constraint
		if tc.max {
			c = d.MaxDistance(l, Entity{}, tc.bound)
		} else {
			c = d.MinDistance(l, Entity{}, tc.bound)
		}
		d.Regen()
		if !g.Solved() || math.Abs(l.Length()-tc.want) > 1e-9 || g.Dof() != tc.dof {
			t.Errorf("%v: from %g, bound %g: solved %v, length %g, dof %d; want %g, %d",
				c, tc.from, tc.bound, g.Solved(), l.Length(), g.Dof(), tc.want, tc.dof)
		}
		if c.Value() != tc.bound {
			t.Errorf("%v: value %g, want %g", c, c.Value(), tc.bound)
		}
	}
}

// A line's end moved past its bounds is brought back to them, and
// moved between them stays where it was put; the minimum and maximum
// may be on two points as well as on a line.
func TestDistanceBoundsMoved(t *testing.T) {
	t.Parallel()
	d, g, l := boundLine(t, 30)
	d.MinDistance(l.Start(), l.End(), 20)
	d.MaxDistance(l.Start(), l.End(), 40)
	for _, tc := range []struct{ to, want float64 }{
		{10, 20}, {30, 30}, {60, 40}, {25, 25}, {-5, 20},
	} {
		l.End().MoveTo(V(tc.to, 0, 0))
		d.Regen()
		if !g.Solved() || math.Abs(l.Length()-tc.want) > 1e-9 {
			t.Errorf("moved to %g: solved %v, length %g, want %g", tc.to, g.Solved(), l.Length(), tc.want)
		}
	}
}

// A point on a segment stays between its ends: beyond one it is
// brought to that end. Inside, it slides along the segment, one degree
// of freedom; at an end it has none.
func TestOnSegment(t *testing.T) {
	t.Parallel()
	d := New()
	g := d.SketchIn(d.Workplane("XY"), "")
	l := d.Line(V(0, 0, 0), V(10, 0, 0))
	d.Lock(l.Start())
	d.Lock(l.End())
	p := d.Point(V(4, 3, 0))
	c := d.OnSegment(p, l)
	d.Regen()
	if !g.Solved() || p.Pos().DistanceTo(V(4, 0, 0)) > 1e-9 || g.Dof() != 1 {
		t.Errorf("%v: solved %v, at %v, dof %d; want (4, 0, 0), 1", c, g.Solved(), p.Pos(), g.Dof())
	}
	for _, tc := range []struct {
		to, want Vec
		dof      int
	}{
		{V(14, 2, 0), V(10, 0, 0), 0},
		{V(-3, -1, 0), V(0, 0, 0), 0},
		{V(7, 1, 0), V(7, 0, 0), 1},
	} {
		p.MoveTo(tc.to)
		d.Regen()
		if !g.Solved() || p.Pos().DistanceTo(tc.want) > 1e-9 || g.Dof() != tc.dof {
			t.Errorf("moved to %v: solved %v, at %v, dof %d; want %v, %d", tc.to, g.Solved(), p.Pos(), g.Dof(), tc.want, tc.dof)
		}
	}
}

// Bounds that can't hold together, or a bound past a length set
// exactly, fail to solve, and the conflict names the bound.
func TestDistanceBoundConflicts(t *testing.T) {
	t.Parallel()
	d, g, l := boundLine(t, 30)
	d.MinDistance(l, Entity{}, 20)
	hi := d.MaxDistance(l, Entity{}, 10)
	d.Regen()
	if g.Solved() {
		t.Error("a minimum of 20 and a maximum of 10 solved")
	}
	named := false
	for _, c := range g.Conflicts() {
		named = named || c.H == hi.H || strings.Contains(c.String(), "min-distance")
	}
	if !named {
		t.Errorf("the conflicts %v don't name a bound", g.Conflicts())
	}

	d, g, l = boundLine(t, 30)
	d.Length(l, 5)
	lo := d.MinDistance(l, Entity{}, 20)
	d.Regen()
	if g.Solved() {
		t.Error("a length of 5 with a minimum of 20 solved")
	}
	if cs := g.Conflicts(); len(cs) == 0 || cs[0].H != lo.H {
		t.Errorf("the conflicts %v aren't the minimum", cs)
	}
	lo.Set(4)
	d.Regen()
	if !g.Solved() || math.Abs(l.Length()-5) > 1e-9 || g.Dof() != 1 {
		t.Errorf("a length of 5 with a minimum of 4: solved %v, length %g, dof %d", g.Solved(), l.Length(), g.Dof())
	}
}

// The bounds take expressions, are described by their kind, and come
// back from a file and through undo and redo as they were.
func TestDistanceBoundsSavedAndUndone(t *testing.T) {
	t.Parallel()
	d, g, l := boundLine(t, 30)
	if err := d.SetParameter("reach", "12"); err != nil {
		t.Fatal(err)
	}
	lo := d.MinDistance(l, Entity{}, 1)
	if err := lo.SetExpr("reach*3"); err != nil {
		t.Fatal(err)
	}
	p := d.Point(V(50, 5, 0))
	d.OnSegment(p, l)
	d.Regen()
	if got := lo.String(); got != "c002-min-distance = 36" {
		t.Errorf("described as %q", got)
	}
	// The point, past the end, draws the line out to it: a minimum
	// bounds only from below.
	if l.Length() < 36-1e-9 || p.Pos().DistanceTo(l.End().Pos()) > 1e-9 {
		t.Fatalf("length %g, point %v, end %v", l.Length(), p.Pos(), l.End().Pos())
	}
	dof := g.Dof()

	path := filepath.Join(t.TempDir(), "bounds.tlspr")
	if err := d.Save(path); err != nil {
		t.Fatal(err)
	}
	e := New()
	if err := e.Load(path); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, c := range e.Groups()[len(e.Groups())-1].Constraints() {
		kinds = append(kinds, c.String())
	}
	if got := strings.Join(kinds, ", "); !strings.Contains(got, "min-distance = 36") || !strings.Contains(got, "pt-on-segment") {
		t.Errorf("loaded %s", got)
	}
	if eg := e.Groups()[len(e.Groups())-1]; !eg.Solved() || eg.Dof() != dof {
		t.Errorf("loaded: solved %v, dof %d, want %d", eg.Solved(), eg.Dof(), dof)
	}

	// Each step of a script is one of undo, as at the console.
	d.Host().(*SketchHost).BeginEval()
	q := d.Point(V(5, 5, 0))
	d.Host().(*SketchHost).BeginEval()
	d.OnSegment(q, l)
	d.Host().(*SketchHost).BeginEval()
	d.Regen()
	n := len(g.Constraints())
	if !d.Undo() || len(g.Constraints()) != n-1 {
		t.Fatalf("undo left %d constraints of %d", len(g.Constraints()), n)
	}
	if !d.Redo() || len(g.Constraints()) != n {
		t.Fatalf("redo made %d constraints of %d", len(g.Constraints()), n)
	}
	d.Regen()
	a, b := l.Start().Pos(), l.End().Pos()
	if off := q.Pos().DistanceToLine(a, b.Minus(a)); off > 1e-9 || g.Dof() != dof+1 {
		t.Errorf("after redo: point %v off the segment by %g, dof %d", q.Pos(), off, g.Dof())
	}
}

// What a bound can't be on is refused with a message.
func TestDistanceBoundsBadInput(t *testing.T) {
	t.Parallel()
	d, _, l := boundLine(t, 30)
	c := d.Circle(V(50, 0, 0), 5)
	failsWith(t, "isn't a line segment", func() { d.MinDistance(c, Entity{}, 3) })
	failsWith(t, "isn't a point", func() { d.MaxDistance(l, l.Start(), 3) })
	failsWith(t, "isn't a line segment", func() { d.OnSegment(l.Start(), c) })
	failsWith(t, "isn't a point", func() { d.OnSegment(l, l) })
}

// A maximum at the value a length dimension holds is no conflict: the
// sketch solves, not redundant, with the freedom the dimension leaves.
func TestDistanceBoundAtDimension(t *testing.T) {
	t.Parallel()
	d, g, l := boundLine(t, 30)
	d.Length(l, 40)
	d.Regen()
	dof := g.Dof()
	d.MaxDistance(l, Entity{}, 40)
	d.MinDistance(l, Entity{}, 40)
	d.Regen()
	if !g.Solved() || g.Dof() != dof || len(g.Conflicts()) != 0 || math.Abs(l.Length()-40) > 1e-9 {
		t.Errorf("solved %v, dof %d (%d before), conflicts %v, length %g", g.Solved(), g.Dof(), dof, g.Conflicts(), l.Length())
	}
}

// The bounds over a grid: a line along x from a locked start, its end
// put at each of seven places, with a minimum, a maximum or both, and
// a point on a segment put at each of seven places along and off it.
// The line ends at the place clamped to the bounds, the point at its
// foot clamped to the segment, and the freedom left is one inside and
// none at a bound. Rate fixed before the first run: none fails. The
// first run failed 4 of 56 on the test's own reckoning, which had an
// end put behind the start brought forward to the minimum, where a
// minimum distance pushes it further behind; the code was right.
func TestDistanceBoundsSweep(t *testing.T) {
	t.Parallel()
	g := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("bounds", "min", "max", "both", "segment"),
			sweep.Floats("to", -5, 2, 10, 17.5, 25, 31, 40),
			sweep.Floats("off", 0, 3),
		},
		MaxFailRate: 0,
	}
	g.RunAll(func(p sweep.Point) error {
		d := New()
		sk := d.SketchIn(d.Workplane("XY"), "")
		to, off := p.Float("to"), p.Float("off")
		var moved Entity
		var want Vec
		lo, hi := math.Inf(-1), math.Inf(1)
		switch p.String("bounds") {
		case "segment":
			l := d.Line(V(5, 0, 0), V(30, 0, 0))
			d.Lock(l.Start())
			d.Lock(l.End())
			moved = d.Point(V(12, 1, 0))
			d.OnSegment(moved, l)
			lo, hi = 5, 30
			want = V(min(max(to, lo), hi), 0, 0)
		default:
			l := d.Line(V(0, 0, 0), V(20, 0, 0))
			d.Lock(l.Start())
			d.Horizontal(l)
			if b := p.String("bounds"); b != "max" {
				d.MinDistance(l, Entity{}, 10)
				lo = 10
			}
			if b := p.String("bounds"); b != "min" {
				d.MaxDistance(l, Entity{}, 30)
				hi = 30
			}
			moved = l.End()
			// The end stays on the line's horizontal: off is the
			// segment's alone. A distance has no side: an end put
			// behind the start stays behind it.
			off = 0
			want = V(math.Copysign(min(max(math.Abs(to), lo), hi), to), 0, 0)
		}
		moved.MoveTo(V(to, off, 0))
		d.Regen()
		dof := 1
		if x := math.Abs(want.X); x == lo || x == hi {
			dof = 0
		}
		if !sk.Solved() || moved.Pos().DistanceTo(want) > 1e-6 || sk.Dof() != dof {
			return fmt.Errorf("solved %v, at %v, %d DOF; want %v, %d", sk.Solved(), moved.Pos(), sk.Dof(), want, dof)
		}
		return nil
	}).Require(t)
}

// BenchmarkDistanceBounds regenerates twenty rods, each between a
// minimum and a maximum length with a slider on a slot under its end,
// with every end put past its maximum: each solve holds forty bounds,
// one at a time.
func BenchmarkDistanceBounds(b *testing.B) {
	d := New()
	d.SketchIn(d.Workplane("XY"), "")
	var ends []Entity
	for i := range 20 {
		y := float64(i) * 20
		rod := d.Line(V(0, y, 0), V(30, y, 0))
		d.Lock(rod.Start())
		d.Horizontal(rod)
		d.MinDistance(rod, Entity{}, 20)
		d.MaxDistance(rod, Entity{}, 40)
		slot := d.Line(V(0, y-10, 0), V(35, y-10, 0))
		d.Lock(slot.Start())
		d.Lock(slot.End())
		slider := d.Point(V(30, y-10, 0))
		d.OnSegment(slider, slot)
		d.Vertical(slider, rod.End())
		ends = append(ends, rod.End())
	}
	d.Regen()
	x := 10.0
	for b.Loop() {
		x = 70 - x
		for i, e := range ends {
			e.MoveTo(V(x, float64(i)*20, 0))
		}
		d.Regen()
	}
}

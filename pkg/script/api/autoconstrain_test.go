package api

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// Auto-constrain from a script (roadmap 29.4): an arc drawn from a
// line's end along it is made tangent to it, an arc from another arc's
// end along it tangent to that, a circle nearly as big as another the
// same size, an arc nearly as big as a circle too, and a line nearly
// square to the line it starts on perpendicular; one step of undo each.
func TestAutoConstrain(t *testing.T) {
	d := New()
	line := d.Line(V(0, 0, 0), V(10, 0, 0))
	// From (10, 0), centre above it: the radius there is square to the line.
	arc := d.Arc(V(10, 5, 0), V(10, 0, 0), V(15, 5, 0))
	// From (15, 5) on, about a centre on the first arc's radius there.
	arc2 := d.Arc(V(25, 5, 0), V(20, 5, 0), V(25, 10, 0))
	_ = arc2
	big := d.Circle(V(50, 0, 0), 8)
	small := d.Circle(V(80, 0, 0), 8.05)
	wheel := d.Circle(V(50, 80, 0), 12)
	other := d.Arc(V(50, 40, 0), V(62.1, 40, 0), V(50, 52.1, 0))
	far := d.Line(V(100, 0, 0), V(103, 11, 0))
	// From far's end, square to it.
	square := d.Line(V(103, 11, 0), V(103-11*0.7, 11+3*0.7, 0))

	if c, ok := d.AutoConstrain(far); ok {
		t.Errorf("a line like nothing got %v", c)
	}
	for _, c := range []struct {
		name string
		e    Entity
		want core.ConstraintType
	}{
		{"arc from the line", arc, core.ConstraintArcLineTangent},
		{"circle as big as another", small, core.ConstraintEqualRadius},
		{"arc as big as a circle", other, core.ConstraintEqualRadius},
		{"line square to the line", square, core.ConstraintPerpendicular},
	} {
		got, ok := d.AutoConstrain(c.e)
		if !ok || got.Type() != c.want {
			t.Errorf("%s: %v %v, want type %d", c.name, got, ok, c.want)
		}
	}
	_, _, _ = line, big, wheel

	// One step of undo: a script's changes are one, so close it first.
	d.Undo()
	d.Redo()
	n := len(d.Active().Constraints())
	if _, ok := d.AutoConstrain(d.Circle(V(0, 60, 0), 8.03)); !ok {
		t.Fatal("a third circle as big got nothing")
	}
	if len(d.Active().Constraints()) != n+1 {
		t.Fatalf("%d constraints, want %d", len(d.Active().Constraints()), n+1)
	}
	d.Undo()
	if len(d.Active().Constraints()) != n {
		t.Errorf("undo left %d constraints, want %d", len(d.Active().Constraints()), n)
	}
}

// Arcs that meet end to end with their radii in line are made tangent.
func TestAutoConstrainArcs(t *testing.T) {
	d := New()
	d.Arc(V(0, 0, 0), V(10, 0, 0), V(0, 10, 0))
	// Starts at (0, 10), its centre further along the first's radius.
	b := d.Arc(V(0, 15, 0), V(0, 10, 0), V(5, 15, 0))
	c, ok := d.AutoConstrain(b)
	if !ok || c.Type() != core.ConstraintCurveCurveTangent {
		t.Errorf("%v %v, want a curve-curve tangent", c, ok)
	}
}

// What can't be auto-constrained says why.
func TestAutoConstrainMisuse(t *testing.T) {
	d := New()
	l := d.Line(V(0, 0, 0), V(10, 0, 0))
	for name, f := range map[string]func(){
		"a point":     func() { d.AutoConstrain(l.Start()) },
		"no entity":   func() { d.AutoConstrain(Entity{}) },
		"a workplane": func() { d.AutoConstrain(d.Workplane("XY")) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			f()
		}()
	}
}

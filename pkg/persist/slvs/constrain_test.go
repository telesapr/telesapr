package slvs

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// TestTryConstrain adds constraints to a free line the way C++'s
// Constrain menu does: a first horizontal constraint is added, a second
// one would be redundant and isn't, and the scheduled regeneration
// solves the line level.
func TestTryConstrain(t *testing.T) {
	doc, err := LoadFile(fixture("request/line_segment/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	var line *core.Entity
	for i := range sk.Entities {
		if sk.Entities[i].Type == core.EntityLineSegment {
			line = &sk.Entities[i]
		}
	}
	if line == nil {
		t.Fatal("no line")
	}
	hl, p0, p1 := line.H, line.Point[0], line.Point[1]

	var rank int
	if how := sk.TestRankForGroup(sk.ActiveGroup(), &rank); how != core.SolveOkay || rank != 0 {
		t.Errorf("before: %v rank %d, want OKAY 0", how, rank)
	}

	hc := sk.TryConstrain(core.ConstraintHorizontal, core.NoEntity, core.NoEntity, hl, core.NoEntity, false, false)
	if hc.V == 0 {
		t.Fatal("horizontal constraint not added")
	}
	if c := sk.GetConstraint(hc); c.DescriptionString() != "c001-horizontal" {
		t.Errorf("description %q", c.DescriptionString())
	}
	if again := sk.TryConstrain(core.ConstraintHorizontal, core.NoEntity, core.NoEntity, hl, core.NoEntity, false, false); again.V != 0 {
		t.Errorf("redundant constraint added as %v", again)
	}
	if len(sk.Constraints) != 1 || !sk.Unsaved {
		t.Errorf("%d constraints, unsaved %v; want 1, true", len(sk.Constraints), sk.Unsaved)
	}

	sk.Refresh()
	a := sk.MustGetEntity(p0).PointGetNum(sk)
	b := sk.MustGetEntity(p1).PointGetNum(sk)
	if math.Abs(a.Y-b.Y) > 1e-9 {
		t.Errorf("line not level after solving: %v %v", a, b)
	}
	if g := sk.GetGroup(sk.ActiveGroup()); g.Solved.How != core.SolveOkay || g.Solved.Dof != 3 {
		t.Errorf("solve %v dof %d, want OKAY 3", g.Solved.How, g.Solved.Dof)
	}

	// A new constraint's handle is one more than the largest in use,
	// even after a deletion.
	sk.ConstrainCoincident(p0, p1)
	sk.DeleteAllConstraintsFor(core.ConstraintHorizontal, hl, core.NoEntity)
	if hc2 := sk.ConstrainCoincident(p0, p1); hc2.V != 3 {
		t.Errorf("handle %v, want 3", hc2)
	}
}

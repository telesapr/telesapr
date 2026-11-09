package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// TryConstrain decides on the rank before the solve, where a new
// parallel between two lines at an angle is not yet satisfied: the
// rank there must not call it redundant (the witness question of
// doc/dev-notes/solver-literature.md).
func TestTryConstrainParallelAtAnAngle(t *testing.T) {
	t.Parallel()
	sk, hg, hrs := linesSketch(t, 2)
	hc := sk.TryConstrain(core.ConstraintParallel, core.NoEntity, core.NoEntity,
		hrs[0].Entity(0), hrs[1].Entity(0), false, false)
	if hc.V == 0 {
		t.Fatal("parallel between lines at an angle left out as redundant")
	}
	sk.GenerateAll()
	if g := sk.Group(hg); g.Solved.How != core.SolveOkay {
		t.Errorf("solved %v", g.Solved.How)
	}
	// One line horizontal, the other vertical, then parallel: that
	// conflicts, and is not redundant either: it is kept, and the group
	// does not solve.
	sk2, hg2, hrs2 := linesSketch(t, 2)
	sk2.constrainIn(hg2, core.ConstraintHorizontal, hrs2[0].Entity(0), core.NoEntity)
	sk2.constrainIn(hg2, core.ConstraintVertical, hrs2[1].Entity(0), core.NoEntity)
	sk2.GenerateAll()
	if hc := sk2.TryConstrain(core.ConstraintParallel, core.NoEntity, core.NoEntity,
		hrs2[0].Entity(0), hrs2[1].Entity(0), false, false); hc.V == 0 {
		t.Error("parallel between a horizontal and a vertical line left out as redundant")
	}
	// Two horizontal lines: parallel says again what they say.
	sk3, hg3, hrs3 := linesSketch(t, 2)
	sk3.constrainIn(hg3, core.ConstraintHorizontal, hrs3[0].Entity(0), core.NoEntity)
	sk3.constrainIn(hg3, core.ConstraintHorizontal, hrs3[1].Entity(0), core.NoEntity)
	sk3.GenerateAll()
	if hc := sk3.TryConstrain(core.ConstraintParallel, core.NoEntity, core.NoEntity,
		hrs3[0].Entity(0), hrs3[1].Entity(0), false, false); hc.V != 0 {
		t.Error("parallel between two horizontal lines kept")
	}
}

package solver

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// A drag solved by a kept system takes its steps with the factorisation
// kept from the moves before, converges as a new system's Newton steps
// do, and comes to rest near where they come: the sketch has freedom
// left, so the steps, taken from another factorisation, reach another
// point of the solutions, within a 500th of a move of 0.05 here; with
// nothing dragged, it takes none.
func TestKeptDragSteps(t *testing.T) {
	t.Parallel()
	ch := makeChainDimensioned(60, false, 20)
	ch.last = ch.unknowns[11] // a point the dimensions hold
	ch.system().Solve(ch.g, true, false, true)
	kept := ch.system()
	for step := range 8 {
		ch.sk[ch.last].Val += 0.05
		before := snapshot(ch.sk)
		s := ch.system()
		s.Dragged = map[core.HParam]bool{ch.last: true}
		how, dof, _ := s.Solve(ch.g, true, false, false)
		fresh := snapshot(ch.sk)

		restore(ch.sk, before)
		kept.Reseed(ch.sk.get)
		kept.Dragged = map[core.HParam]bool{ch.last: true}
		howK, dofK, _ := kept.Solve(ch.g, true, false, false)
		if how != core.SolveOkay || how != howK || dof != dofK {
			t.Fatalf("step %d: new %v %d, kept %v %d", step, how, dof, howK, dofK)
		}
		for h, p := range ch.sk {
			if q := fresh[h]; math.Abs(p.Val-q.Val) > 1e-4 {
				t.Fatalf("step %d: param %d kept %v, new %v", step, h.V, p.Val, q.Val)
			}
		}
	}
	if kept.keptSteps == 0 {
		t.Error("the drag took no step with a kept factorisation")
	}

	steps := kept.keptSteps
	kept.Reseed(ch.sk.get)
	ch.sk[ch.last].Val += 0.05
	kept.Reseed(ch.sk.get)
	if how, _, _ := kept.Solve(ch.g, true, false, false); how != core.SolveOkay {
		t.Fatalf("after the drag: %v", how)
	}
	if kept.keptSteps != steps {
		t.Error("a solve with nothing dragged took a kept step")
	}
}

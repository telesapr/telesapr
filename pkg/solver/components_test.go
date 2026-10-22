package solver

import (
	"math"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// Profiles that share no unknown, solved a component at a time, come to
// the solution solving them as one system comes to, within the
// solver's tolerance, with the same freedom; a profile whose
// dimensions can't be met is blamed as the whole system would blame it.
func TestSolveComponentsAsWhole(t *testing.T) {
	t.Parallel()
	for _, broken := range []bool{false, true} {
		var got [2]map[core.HParam]core.Param
		var how [2]core.SolveResult
		var dof [2]int
		var bad [2][]core.HConstraint
		for n, whole := range []bool{true, false} {
			ch := makeProfiles(4, 12)
			if broken {
				// The third profile's first point pinned twice, apart.
				ch.g = withEquation(ch.g, p(core.HParam{V: 2*13*2 + 1}).Minus(c(2003)))
			}
			s := ch.system()
			s.whole = whole
			how[n], dof[n], bad[n] = s.Solve(ch.g, true, false, false)
			got[n] = snapshot(ch.sk)
		}
		if how[0] != how[1] || dof[0] != dof[1] {
			t.Fatalf("broken %v: whole %v %d, components %v %d", broken, how[0], dof[0], how[1], dof[1])
		}
		if broken {
			if how[1] == core.SolveOkay || !slices.Equal(bad[0], bad[1]) {
				t.Errorf("the broken profile is blamed for %v, whole %v (%v)", bad[1], bad[0], how[1])
			}
			continue
		}
		for h, p := range got[0] {
			if q := got[1][h]; math.Abs(p.Val-q.Val) > 1e-9 {
				t.Errorf("param %d: whole %v, components %v", h.V, p.Val, q.Val)
			}
		}
	}
}

// A system of one component, or of none, is solved as one, by
// NewtonSolve itself.
func TestComponentsOfChain(t *testing.T) {
	t.Parallel()
	ch := makeProfiles(3, 5)
	s := ch.system()
	s.writeEquationsExceptFor(core.NoConstraint, ch.g)
	s.clearParamTags()
	s.clearEqTags()
	s.WriteJacobian(0)
	comps := s.components()
	if len(comps) != 3 {
		t.Fatalf("%d components, want 3", len(comps))
	}
	for k, c := range comps {
		if len(c.rows) != 7 || len(c.cols) != 12 {
			t.Errorf("component %d: %d rows, %d columns", k, len(c.rows), len(c.cols))
		}
	}
	one := makeProfiles(1, 5)
	s = one.system()
	if how, _, _ := s.Solve(one.g, true, false, false); how != core.SolveOkay {
		t.Errorf("one profile: %v", how)
	}
}

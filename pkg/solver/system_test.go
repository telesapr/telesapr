package solver

import (
	"math"
	"testing"
	"time"

	"github.com/spacesolve/spacesolve/pkg/core"
)

func TestQRRankAndDeadColumns(t *testing.T) {
	// Column 2 is column 0 plus column 1, so it's dead.
	a := [][]float64{
		{1, 0, 1},
		{0, 1, 1},
		{1, 1, 2},
	}
	f := factorSparseQR(spFromRows(a, 3, 3))
	if f.rank != 2 {
		t.Fatalf("rank %d, want 2", f.rank)
	}

	// Full rank: the basic solution is the solution.
	b := [][]float64{{2, 1}, {1, 3}}
	x := factorSparseQR(spFromRows(b, 2, 2)).solve([]float64{5, 10})
	if math.Abs(x[0]-1) > 1e-12 || math.Abs(x[1]-3) > 1e-12 {
		t.Errorf("solve = %v, want [1 3]", x)
	}
}

// testSketch is the sketch's param table for a hand-built system.
type testSketch map[core.HParam]*core.Param

func (ts testSketch) param(v uint32, val float64) core.HParam {
	h := core.HParam{V: v}
	ts[h] = &core.Param{H: h, Val: val}
	return h
}

func (ts testSketch) get(h core.HParam) *core.Param { return ts[h] }

// testGroup is a group whose constraints each write the equations eqs
// lists for them; handle i+1 is constraint i.
func testGroup(eqs [][]*core.Expr) *Group {
	g := &Group{FindToFixTimeout: time.Second}
	for i := range eqs {
		g.Constraints = append(g.Constraints, &core.Constraint{ConstraintBase: core.ConstraintBase{
			H: core.HConstraint{V: uint32(i + 1)}, Type: core.ConstraintPtPtDistance}})
	}
	g.WriteEquationsExceptFor = func(hc core.HConstraint, l *[]core.Equation) {
		for i, ce := range eqs {
			h := core.HConstraint{V: uint32(i + 1)}
			if h == hc {
				continue
			}
			for k, e := range ce {
				*l = append(*l, core.Equation{H: h.Equation(k), Expr: e.DeepCopy()})
			}
		}
	}
	return g
}

func newSystem(sk testSketch, unknowns ...core.HParam) *System {
	s := &System{SK: sk.get}
	for _, h := range unknowns {
		p := *sk[h]
		s.AddParam(p)
	}
	return s
}

func p(h core.HParam) *core.Expr { return core.NewParam(h) }
func c(v float64) *core.Expr     { return core.NewConstant(v) }

func TestSolveSubstitutionAndAlone(t *testing.T) {
	sk := testSketch{}
	a, b := sk.param(1, 0), sk.param(2, 0)
	// a = b, and b^2 = 25 (solved alone, from b = 1 it goes to 5).
	sk[b].Val = 1
	g := testGroup([][]*core.Expr{
		{p(a).Minus(p(b))},
		{p(b).Square().Minus(c(25))},
	})
	s := newSystem(sk, a, b)
	var dof int
	var bad []core.HConstraint
	how := s.Solve(g, &dof, &bad, true, false, false)
	if how != core.SolveOkay {
		t.Fatalf("how %d", how)
	}
	if math.Abs(sk[a].Val-5) > 1e-9 || math.Abs(sk[b].Val-5) > 1e-9 {
		t.Errorf("a %g b %g, want 5 5", sk[a].Val, sk[b].Val)
	}
	if !sk[a].Known || !sk[b].Known {
		t.Error("solved params should be marked known")
	}
	if dof != 0 {
		t.Errorf("dof %d", dof)
	}
}

func TestSolveUnderdeterminedMinimumNorm(t *testing.T) {
	sk := testSketch{}
	x, y := sk.param(1, 0), sk.param(2, 0)
	g := testGroup([][]*core.Expr{{p(x).Plus(p(y)).Minus(c(2))}})
	s := newSystem(sk, x, y)
	var dof int
	var bad []core.HConstraint
	if how := s.Solve(g, &dof, &bad, true, false, true); how != core.SolveOkay {
		t.Fatalf("how %d", how)
	}
	// The minimum-norm step from (0, 0) lands on (1, 1).
	if math.Abs(sk[x].Val-1) > 1e-12 || math.Abs(sk[y].Val-1) > 1e-12 {
		t.Errorf("x %g y %g, want 1 1", sk[x].Val, sk[y].Val)
	}
	if dof != 1 {
		t.Errorf("dof %d, want 1", dof)
	}
}

func TestSolveKnownParamIsConstant(t *testing.T) {
	sk := testSketch{}
	x := sk.param(1, 0)
	k := sk.param(2, 7)
	sk[k].Known = true
	g := testGroup([][]*core.Expr{{p(x).Times(p(x)).Minus(p(k).Times(p(k)))}})
	sk[x].Val = 1
	s := newSystem(sk, x)
	var dof int
	var bad []core.HConstraint
	if how := s.Solve(g, &dof, &bad, true, false, true); how != core.SolveOkay {
		t.Fatalf("how %d", how)
	}
	if math.Abs(sk[x].Val-7) > 1e-9 || sk[k].Val != 7 {
		t.Errorf("x %g k %g, want 7 7", sk[x].Val, sk[k].Val)
	}
}

func TestSolveRedundantFindsConstraints(t *testing.T) {
	sk := testSketch{}
	x, y := sk.param(1, 0), sk.param(2, 0)
	// Constraints 1 and 2 say the same thing; constraint 3 fixes y.
	g := testGroup([][]*core.Expr{
		{p(x).Plus(p(y)).Minus(c(3))},
		{p(x).Plus(p(y)).Minus(c(3)).Times(c(2))},
		{p(y).Minus(c(1))},
	})
	s := newSystem(sk, x, y)
	var dof int
	var bad []core.HConstraint
	how := s.Solve(g, &dof, &bad, true, false, true)
	if how != core.SolveRedundantOkay {
		t.Fatalf("how %d, want REDUNDANT_OKAY", how)
	}
	if math.Abs(sk[x].Val-2) > 1e-9 || math.Abs(sk[y].Val-1) > 1e-9 {
		t.Errorf("x %g y %g, want 2 1", sk[x].Val, sk[y].Val)
	}
	// Removing either copy fixes the Jacobian; removing the one on y
	// doesn't (y is solved alone, so it's not in the rank test).
	if len(bad) != 2 || bad[0].V != 1 || bad[1].V != 2 {
		t.Errorf("bad %v, want constraints 1 and 2", bad)
	}
}

func TestSolveInconsistentDoesNotConverge(t *testing.T) {
	sk := testSketch{}
	x, y := sk.param(1, 0), sk.param(2, 0)
	g := testGroup([][]*core.Expr{
		{p(x).Plus(p(y)).Minus(c(1))},
		{p(x).Plus(p(y)).Minus(c(2))},
	})
	s := newSystem(sk, x, y)
	var dof int
	var bad []core.HConstraint
	how := s.Solve(g, &dof, &bad, true, false, true)
	if how != core.SolveRedundantDidntConverge {
		t.Fatalf("how %d, want REDUNDANT_DIDNT_CONVERGE", how)
	}
	if len(bad) == 0 {
		t.Error("expected the unsatisfied constraints to be reported")
	}
	if sk[x].Val != 0 || sk[y].Val != 0 {
		t.Errorf("a failed solve must leave the sketch alone: x %g y %g", sk[x].Val, sk[y].Val)
	}
}

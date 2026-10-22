package solver

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// A system kept and reseeded solves each step of a drag, and of a
// sketch whose known values change, to the same bits as a new system,
// taking the Jacobians it wrote before where the equations and the
// known values they read are the same, and writing them again where
// they are not.
func TestKeptSystemSolvesAsNew(t *testing.T) {
	t.Parallel()
	for _, ch := range []*chain{makeChain(30, true), makeChainDimensioned(40, false, 12), makeChain(25, false)} {
		// A known param one equation reads: folded into the Jacobian
		// as a constant.
		outside := ch.sk.param(9999, 0)
		ch.sk[outside].Known = true
		ch.g = withEquation(ch.g, p(ch.unknowns[0]).Minus(p(outside)))
		ch.system().Solve(ch.g, true, false, true)
		kept := ch.system()
		for step := range 6 {
			ch.sk[ch.last].Val += 0.25
			if step == 3 {
				ch.sk[outside].Val = 0.5
			}
			before := snapshot(ch.sk)
			s := ch.system()
			s.Dragged = map[core.HParam]bool{ch.last: true}
			how, dof, _ := s.Solve(ch.g, true, true, false)
			fresh := snapshot(ch.sk)

			restore(ch.sk, before)
			kept.Reseed(ch.sk.get)
			kept.Dragged = map[core.HParam]bool{ch.last: true}
			howK, dofK, _ := kept.Solve(ch.g, true, true, false)
			if how != howK || dof != dofK {
				t.Fatalf("step %d: new %v %d, kept %v %d", step, how, dof, howK, dofK)
			}
			for h, p := range ch.sk {
				if q := fresh[h]; math.Float64bits(p.Val) != math.Float64bits(q.Val) || p.Free != q.Free {
					t.Fatalf("step %d: param %d kept %v, new %v", step, h.V, p.Val, q.Val)
				}
			}
		}
		if kept.JacobiansKept() == 0 {
			t.Errorf("the kept system wrote every Jacobian again")
		}
	}
}

// withEquation is g with one more constraint, writing e.
func withEquation(g *Group, e *core.Expr) *Group {
	h := core.HConstraint{V: uint32(len(g.Constraints) + 1)}
	out := *g
	out.Constraints = append(append([]*core.Constraint(nil), g.Constraints...),
		&core.Constraint{H: h, Type: core.ConstraintPtPtDistance})
	out.WriteEquationsExceptFor = func(hc core.HConstraint, l *[]core.Equation) {
		g.WriteEquationsExceptFor(hc, l)
		if hc != h {
			*l = append(*l, core.Equation{H: h.Equation(0), Expr: e.DeepCopy()})
		}
	}
	return &out
}

// snapshot copies the sketch's params.
func snapshot(sk testSketch) map[core.HParam]core.Param {
	out := make(map[core.HParam]core.Param, len(sk))
	for h, p := range sk {
		out[h] = *p
	}
	return out
}

// restore puts the sketch's params back as snapshot took them.
func restore(sk testSketch, from map[core.HParam]core.Param) {
	for h, p := range from {
		*sk[h] = p
	}
}

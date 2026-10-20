package solver

import (
	"fmt"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// chain is a hand-built sketch of n segments joined end to end in a
// staircase, each horizontal or vertical in turn, as a sketch of the
// window would generate it: the points' coordinates are the unknowns,
// and each segment writes one equation. A closed chain also fixes its
// first point and gives every segment a length, which leaves it with
// no freedom: the square system a dimensioned sketch is. The staircase
// satisfies its equations already, so a solve of it is the one Newton
// step a settled sketch takes.
type chain struct {
	sk       testSketch
	g        *Group
	unknowns []core.HParam
	// last is the x of the last point, the param a drag moves.
	last core.HParam
}

func makeChain(n int, closed bool) *chain {
	ch := &chain{sk: testSketch{}}
	xs := make([]core.HParam, n+1)
	ys := make([]core.HParam, n+1)
	for i := range n + 1 {
		xs[i] = ch.sk.param(uint32(2*i+1), float64((i+1)/2*10))
		ys[i] = ch.sk.param(uint32(2*i+2), float64(i/2*10))
		ch.unknowns = append(ch.unknowns, xs[i], ys[i])
	}
	var eqs [][]*core.Expr
	for i := range n {
		if i%2 == 0 {
			eqs = append(eqs, []*core.Expr{p(ys[i]).Minus(p(ys[i+1]))})
		} else {
			eqs = append(eqs, []*core.Expr{p(xs[i]).Minus(p(xs[i+1]))})
		}
	}
	if closed {
		eqs = append(eqs, []*core.Expr{p(xs[0])}, []*core.Expr{p(ys[0])})
		for i := range n {
			d := p(xs[i]).Minus(p(xs[i+1])).Square().Plus(p(ys[i]).Minus(p(ys[i+1])).Square())
			eqs = append(eqs, []*core.Expr{d.Minus(c(100))})
		}
	}
	ch.g = testGroup(eqs)
	ch.last = xs[n]
	return ch
}

// system is the chain's system as the sketch writes it for each solve:
// a fresh copy of the params, unknown again, and the equations.
func (ch *chain) system() *System {
	s := &System{SK: ch.sk.get}
	for _, h := range ch.unknowns {
		p := *ch.sk[h]
		p.Known = false
		s.AddParam(p)
	}
	return s
}

func benchmarkSolve(b *testing.B, n int, closed bool) {
	ch := makeChain(n, closed)
	// The first solve settles the sketch, as a regeneration does.
	ch.system().Solve(ch.g, true, false, true)
	b.ResetTimer()
	for b.Loop() {
		s := ch.system()
		if how, _, _ := s.Solve(ch.g, true, false, false); how != core.SolveOkay {
			b.Fatalf("how %d", how)
		}
	}
}

// BenchmarkSolveChain solves a chain of n segments with freedom left,
// the shape of a sketch being drawn: the rank test factors a Jacobian
// with more unknowns than equations.
func BenchmarkSolveChain(b *testing.B) {
	for _, n := range []int{20, 100, 400} {
		b.Run(fmt.Sprint(n), func(b *testing.B) { benchmarkSolve(b, n, false) })
	}
}

// BenchmarkSolveClosed solves a chain with every point dimensioned,
// the shape of a finished sketch: the Jacobian is square.
func BenchmarkSolveClosed(b *testing.B) {
	for _, n := range []int{20, 100, 400} {
		b.Run(fmt.Sprint(n), func(b *testing.B) { benchmarkSolve(b, n, true) })
	}
}

// BenchmarkSolveDrag is a drag of the last point of an open chain of
// 100 segments: each solve starts from the sketch moved a little, with
// the point's x marked as dragged, as the window does on each mouse
// move.
func BenchmarkSolveDrag(b *testing.B) {
	ch := makeChain(100, false)
	ch.system().Solve(ch.g, true, false, true)
	b.ResetTimer()
	for b.Loop() {
		ch.sk[ch.last].Val += 0.1
		s := ch.system()
		s.Dragged = map[core.HParam]bool{ch.last: true}
		if how, _, _ := s.Solve(ch.g, true, false, false); how != core.SolveOkay {
			b.Fatalf("how %d", how)
		}
	}
}

// BenchmarkWriteJacobian is the symbolic part of a solve alone: the
// equations copied with their params bound, folded and differentiated.
func BenchmarkWriteJacobian(b *testing.B) {
	ch := makeChain(100, true)
	s := ch.system()
	s.writeEquationsExceptFor(core.NoConstraint, ch.g)
	s.clearParamTags()
	s.clearEqTags()
	b.ResetTimer()
	for b.Loop() {
		if !s.WriteJacobian(0) {
			b.Fatal("too many unknowns")
		}
	}
}

// evaluatedSystem is the chain's system with its Jacobian written and
// evaluated, as the rank test finds it.
func evaluatedSystem(b *testing.B, ch *chain) *System {
	b.Helper()
	s := ch.system()
	s.writeEquationsExceptFor(core.NoConstraint, ch.g)
	s.clearParamTags()
	s.clearEqTags()
	if !s.WriteJacobian(0) {
		b.Fatal("too many unknowns")
	}
	s.EvalJacobian()
	return s
}

// jacobianOf is the evaluated Jacobian of the chain, as the rank test
// factors it.
func jacobianOf(b *testing.B, ch *chain) *spMat {
	b.Helper()
	s := evaluatedSystem(b, ch)
	return spFromRows(s.mat.Anum, s.mat.m, s.mat.n)
}

// BenchmarkFactorRank factors the Jacobian of an open chain as the
// rank test does: wide, with every column past the rank dead.
func BenchmarkFactorRank(b *testing.B) {
	for _, n := range []int{100, 400} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a := jacobianOf(b, makeChain(n, false))
			b.ResetTimer()
			for b.Loop() {
				factorSparseQR(a)
			}
		})
	}
}

// BenchmarkFactorWide factors the Jacobian of a sketch of 300 segments
// of which only the first k are constrained, the shape of the rank test
// as a sketch is being constrained: wide, with most columns dead, and
// the elimination tree computed again for each of them.
func BenchmarkFactorWide(b *testing.B) {
	for _, k := range []int{10, 50} {
		b.Run(fmt.Sprint(k), func(b *testing.B) {
			s := evaluatedSystem(b, makeChain(300, false))
			a := spFromRows(s.mat.Anum[:k], k, s.mat.n)
			b.ResetTimer()
			for b.Loop() {
				factorSparseQR(a)
			}
		})
	}
}

// BenchmarkFactorLeastSquares factors the transposed Jacobian of an
// open chain, as the minimum-norm Newton step does: tall and of full
// rank.
func BenchmarkFactorLeastSquares(b *testing.B) {
	for _, n := range []int{100, 400} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a := jacobianOf(b, makeChain(n, false)).transpose()
			b.ResetTimer()
			for b.Loop() {
				factorSparseQR(a)
			}
		})
	}
}

// BenchmarkFactorSquare factors the Jacobian of a closed chain, the
// square system of a dimensioned sketch.
func BenchmarkFactorSquare(b *testing.B) {
	for _, n := range []int{100, 400} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a := jacobianOf(b, makeChain(n, true))
			b.ResetTimer()
			for b.Loop() {
				factorSparseQR(a)
			}
		})
	}
}

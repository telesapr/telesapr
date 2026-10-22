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
	return makeChainDimensioned(n, closed, 0)
}

// makeChainDimensioned is makeChain with lengths on the first
// dimensioned segments only, and no horizontal or vertical
// constraints: a sketch being dimensioned, whose equations are not
// solved by substitution.
func makeChainDimensioned(n int, closed bool, dimensioned int) *chain {
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
		switch {
		case dimensioned > 0 && i < dimensioned:
			d := p(xs[i]).Minus(p(xs[i+1])).Square().Plus(p(ys[i]).Minus(p(ys[i+1])).Square())
			eqs = append(eqs, []*core.Expr{d.Minus(c(100))})
		case dimensioned > 0:
		case i%2 == 0:
			eqs = append(eqs, []*core.Expr{p(ys[i]).Minus(p(ys[i+1]))})
		default:
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
	benchmarkSolveChain(b, makeChain(n, closed))
}

func benchmarkSolveChain(b *testing.B, ch *chain) {
	b.Run("new", func(b *testing.B) { benchmarkSolveSystem(b, ch, false) })
	b.Run("kept", func(b *testing.B) { benchmarkSolveSystem(b, ch, true) })
}

// benchmarkSolveSystem solves ch over and over, with a new system each
// time or with one kept and reseeded, as the sketch keeps one per
// group.
func benchmarkSolveSystem(b *testing.B, ch *chain, keep bool) {
	// The first solve settles the sketch, as a regeneration does.
	kept := ch.system()
	kept.Solve(ch.g, true, false, true)
	b.ResetTimer()
	for b.Loop() {
		s := ch.system()
		if keep {
			kept.Reseed(ch.sk.get)
			s = kept
		}
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

// BenchmarkSolveDimensioning solves a chain of 300 segments of which
// the first k have a length, the shape of a sketch being dimensioned:
// the rank test factors a Jacobian with far more unknowns than
// equations, and nothing is solved by substitution.
func BenchmarkSolveDimensioning(b *testing.B) {
	for _, k := range []int{10, 50} {
		b.Run(fmt.Sprint(k), func(b *testing.B) { benchmarkSolveChain(b, makeChainDimensioned(300, false, k)) })
	}
}

// BenchmarkSolveDrag is a drag of the last point of an open chain of
// 100 segments: each solve starts from the sketch moved a little, with
// the point's x marked as dragged, as the window does on each mouse
// move.
func BenchmarkSolveDrag(b *testing.B) {
	b.Run("new", func(b *testing.B) { benchmarkSolveDrag(b, false) })
	b.Run("kept", func(b *testing.B) { benchmarkSolveDrag(b, true) })
}

func benchmarkSolveDrag(b *testing.B, keep bool) {
	ch := makeChain(100, false)
	kept := ch.system()
	kept.Solve(ch.g, true, false, true)
	b.ResetTimer()
	for b.Loop() {
		ch.sk[ch.last].Val += 0.1
		s := ch.system()
		if keep {
			kept.Reseed(ch.sk.get)
			s = kept
		}
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

// BenchmarkRankWide is the rank alone of the matrices of
// BenchmarkFactorWide, as the rank test asks for it.
func BenchmarkRankWide(b *testing.B) {
	for _, k := range []int{10, 50} {
		b.Run(fmt.Sprint(k), func(b *testing.B) {
			s := evaluatedSystem(b, makeChain(300, false))
			a := spFromRows(s.mat.Anum[:k], k, s.mat.n)
			b.ResetTimer()
			for b.Loop() {
				s.rankOf(a)
			}
		})
	}
}

// BenchmarkRankSquare is the rank alone of the matrices of
// BenchmarkFactorSquare.
func BenchmarkRankSquare(b *testing.B) {
	for _, n := range []int{100, 400} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := evaluatedSystem(b, makeChain(n, true))
			a := spFromRows(s.mat.Anum, s.mat.m, s.mat.n)
			b.ResetTimer()
			for b.Loop() {
				s.rankOf(a)
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

// BenchmarkStepFactor factors the Jacobian of an open chain as the
// minimum-norm Newton step does: transposed, its rows matched to its
// columns (rowmatch.go), the analysis kept.
func BenchmarkStepFactor(b *testing.B) {
	for _, n := range []int{100, 400} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			a := jacobianOf(b, makeChain(n, false))
			var c matchedCache
			b.ResetTimer()
			for b.Loop() {
				factorLeastSquares(a, &c)
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

// makeProfiles is k closed chains of n segments each, every point
// dimensioned, sharing no unknown: a sketch of k profiles. Each chain
// starts a little off its dimensions, so that a solve takes Newton
// steps.
func makeProfiles(k, n int) *chain {
	ch := &chain{sk: testSketch{}}
	var eqs [][]*core.Expr
	h := uint32(1)
	for pr := range k {
		xs := make([]core.HParam, n+1)
		ys := make([]core.HParam, n+1)
		for i := range n + 1 {
			off := float64(pr*1000) + 0.01*float64(pr+1)*float64(i%3)
			xs[i] = ch.sk.param(h, off+float64((i+1)/2*10))
			ys[i] = ch.sk.param(h+1, float64(i/2*10))
			h += 2
			ch.unknowns = append(ch.unknowns, xs[i], ys[i])
		}
		eqs = append(eqs, []*core.Expr{p(xs[0]).Minus(c(float64(pr * 1000)))}, []*core.Expr{p(ys[0])})
		for i := range n {
			d := p(xs[i]).Minus(p(xs[i+1])).Square().Plus(p(ys[i]).Minus(p(ys[i+1])).Square())
			eqs = append(eqs, []*core.Expr{d.Minus(c(100))})
		}
		ch.last = xs[n]
	}
	ch.g = testGroup(eqs)
	return ch
}

// BenchmarkSolveProfiles solves k profiles of 50 segments each from
// off their dimensions: whole, as one system, and a connected
// component at a time.
func BenchmarkSolveProfiles(b *testing.B) {
	for _, k := range []int{1, 4, 16} {
		for _, whole := range []bool{true, false} {
			name := fmt.Sprintf("%d/components", k)
			if whole {
				name = fmt.Sprintf("%d/whole", k)
			}
			b.Run(name, func(b *testing.B) {
				ch := makeProfiles(k, 50)
				start := snapshot(ch.sk)
				for b.Loop() {
					restore(ch.sk, start)
					s := ch.system()
					s.whole = whole
					if how, _, _ := s.Solve(ch.g, true, false, false); how != core.SolveOkay {
						b.Fatalf("how %d", how)
					}
				}
			})
		}
	}
}

// BenchmarkSolveDragDimensioned drags a point of a chain of 300
// segments whose first 50 have a length, which the drag pulls on: the
// Newton steps factor the lengths' Jacobian, with a new system each
// move or with one kept, whose steps take the factorisation kept from
// the moves before (drag_step.go).
func BenchmarkSolveDragDimensioned(b *testing.B) {
	for _, keep := range []bool{false, true} {
		name := "new"
		if keep {
			name = "kept"
		}
		b.Run(name, func(b *testing.B) {
			ch := makeChainDimensioned(300, false, 50)
			ch.last = ch.unknowns[41]
			kept := ch.system()
			kept.Solve(ch.g, true, false, true)
			b.ResetTimer()
			for b.Loop() {
				ch.sk[ch.last].Val += 0.01
				s := ch.system()
				if keep {
					kept.Reseed(ch.sk.get)
					s = kept
				}
				s.Dragged = map[core.HParam]bool{ch.last: true}
				if how, _, _ := s.Solve(ch.g, true, false, false); how != core.SolveOkay {
					b.Fatalf("how %d", how)
				}
			}
		})
	}
}

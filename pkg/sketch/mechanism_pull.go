package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// pullSteps bounds the steps of one pull. A step is linear, and kept
// small (pullMaxTurn), so a long pull round a hinge takes several: a
// turn of 90° some eight. A pull stops before that where the point is
// there, or comes hardly nearer, as at the end of a chain stretched
// toward it; a step of a drag, a few pixels, takes one or two.
const pullSteps = 32

// turnWeight is the length, in mm, a part's turn is weighed by against
// its move, where a pull could do either: a quaternion's components
// are numbers near 1, and a change of a hundredth in them swings a point
// 60 mm off by more than half a millimetre, so weighed alike every pull
// would turn the part about its origin rather than move it. Weighed by
// a length l, a pull of d at a point r from the origin that a move or a
// turn could make turns the part by about 4·d·r/l² as well as moving it
// (0.5° for d 40, r 60 and l a metre); weighed by 100 m, a part its mates
// leave free to slide slides where it is pulled, as before mechanisms,
// turning by a millionth, and one they leave only turning turns.
const turnWeight = 1e5

// pullMaxTurn bounds how much a pull's step changes a part's quaternion
// component, about half the turn in radians: some 11° a step.
const pullMaxTurn = 0.1

// pullable reports whether point he can be pulled along its
// mechanism's freedom: a point of a part that moves in a mechanism,
// placed by the part's move and turn, whose mechanism is solved.
func (sk *Sketch) pullable(he core.HEntity) bool {
	e := sk.Entity(he)
	if e == nil || e.Type != core.EntityPointNRotTrans {
		return false
	}
	g := sk.Group(e.Group)
	return movesInMechanism(g) && g.IsSolvedOkay()
}

// pullPoint moves point he of a part of a mechanism toward to, along
// the freedom the mechanism leaves, so that the parts joined to it
// move and turn with it, rather than putting it there and letting the
// solve pull it back, as PointForceTo does: a part hinged at its own
// origin can come back to its hinge only by its move, never by its
// turn, so it would snap back. Each step finds the mechanism's null
// space (solver.System.NullSpace), takes the least-squares step in it
// that moves the point nearest to, and solves the mechanism to take
// up what the step being linear left. The part's params are then
// recorded as pulled, so that the regeneration after keeps them as
// near as it can while it moves the others.
func (sk *Sketch) pullPoint(he core.HEntity, to geom.Vector) {
	block := sk.Mechanism(sk.Entity(he).Group)
	params := sk.groupParams(block...)
	last := math.Inf(1)
	for range pullSteps {
		e := sk.Entity(he)
		r := to.Minus(e.PointGetNum(sk))
		// Done where the point is there, or where the last step brought
		// it less than a tenth nearer: at the end of a chain stretched
		// toward the target each step gains less than the one before.
		if r.Magnitude() < core.LengthEps || r.Magnitude() > 0.9*last {
			break
		}
		last = r.Magnitude()
		sys, sg := sk.writeEqSystem(block, params)
		hs, null := sys.NullSpace(sg)
		if len(null) == 0 {
			break
		}
		// The point's Jacobian over the unknowns, then over the null
		// space: a, three rows of k.
		ex := e.PointGetExprs(sk)
		k := len(null)
		var a [3][]float64
		for i, c := range []*core.Expr{ex.X, ex.Y, ex.Z} {
			a[i] = make([]float64, k)
			for j, h := range hs {
				d := core.EvalExpr(sk, c.Derivative(h))
				if d == 0 {
					continue
				}
				for v := range k {
					a[i][v] += d * null[v][j]
				}
			}
		}
		w := sk.pullWeights(block, hs)
		y := leastSquares3(a, [3]float64{r.X, r.Y, r.Z}, null, w)
		step := make([]float64, len(hs))
		maxMove, maxTurn := 0.0, 0.0
		for j := range hs {
			for v := range k {
				step[j] += y[v] * null[v][j]
			}
			if w[j] > 1 {
				maxTurn = math.Max(maxTurn, math.Abs(step[j]))
			} else {
				maxMove = math.Max(maxMove, math.Abs(step[j]))
			}
		}
		// The step is linear, and good only while it is small. Near a
		// stretched chain, where the point can't go all the way, the
		// least-squares step that comes nearest runs off by hundreds
		// of millimetres and whole turns, and the solve can't bring the
		// parts back onto their mates. So a step moves no part by more
		// than twice the way left to go, nor turns it by more than some
		// 11° (pullMaxTurn); the next step goes on from there.
		scale := math.Max(1, math.Max(maxMove/(2*r.Magnitude()), maxTurn/pullMaxTurn))
		for j, h := range hs {
			sk.MustParam(h).Val += step[j] / scale
		}
		sk.pulled = sk.partParams(he)
		sk.solveBlock(block, params, false)
	}
	sk.pulled = sk.partParams(he)
	sk.MarkGroupDirty(block[0], false)
}

// pullWeights are what each of params, unknowns of the mechanism
// block, is weighed by in a pull: turnWeight for a part's turn (the
// quaternion of a link group's placement), 1 for anything else.
func (sk *Sketch) pullWeights(block []core.HGroup, params []core.HParam) []float64 {
	turn := make(map[core.HParam]bool)
	for _, hg := range block {
		for i := 3; i < 7; i++ {
			turn[hg.Param(i)] = true
		}
	}
	w := make([]float64, len(params))
	for j, h := range params {
		w[j] = 1
		if turn[h] {
			w[j] = turnWeight
		}
	}
	return w
}

// partParams are the params that place the part point he is a point
// of: its move and its turn.
func (sk *Sketch) partParams(he core.HEntity) []core.HParam {
	e := sk.Entity(he)
	return append([]core.HParam(nil), e.Param[:7]...)
}

// leastSquares3 is the y, of k, that makes a·y nearest b, a three rows
// of k, by the normal equations. Where a has fewer than k independent
// columns (a mechanism with more freedom than the point can show), of
// the y that do so equally well it takes the one whose step null·y, a
// change of the unknowns, is least weighed by w (pullWeights): the
// equations are damped by that weighed size, slightly, so that they
// stay positive definite.
func leastSquares3(a [3][]float64, b [3]float64, null [][]float64, w []float64) []float64 {
	k := len(null)
	m := make([][]float64, k)
	y := make([]float64, k)
	metric := make([][]float64, k)
	var traceA, traceM float64
	for c := range k {
		m[c] = make([]float64, k)
		metric[c] = make([]float64, k)
		for d := range k {
			for i := range 3 {
				m[c][d] += a[i][c] * a[i][d]
			}
			for j, wj := range w {
				metric[c][d] += wj * wj * null[c][j] * null[d][j]
			}
		}
		traceA += m[c][c]
		traceM += metric[c][c]
		for i := range 3 {
			y[c] += a[i][c] * b[i]
		}
	}
	damp := 1e-9 * traceA / traceM
	for c := range k {
		for d := range k {
			m[c][d] += damp * metric[c][d]
		}
	}
	// Cholesky, m = l·lᵀ in m's lower half, then the two triangles.
	for c := range k {
		for d := range c + 1 {
			sum := m[c][d]
			for e := range d {
				sum -= m[c][e] * m[d][e]
			}
			if c == d {
				m[c][c] = math.Sqrt(sum)
			} else {
				m[c][d] = sum / m[d][d]
			}
		}
	}
	for c := range k {
		for e := range c {
			y[c] -= m[c][e] * y[e]
		}
		y[c] /= m[c][c]
	}
	for c := k - 1; c >= 0; c-- {
		for e := c + 1; e < k; e++ {
			y[c] -= m[e][c] * y[e]
		}
		y[c] /= m[c][c]
	}
	return y
}

package sketch

import (
	"math"
	"slices"

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
// turn, so it would snap back. The pull (pullPart) aims the point's
// three coordinates at to.
func (sk *Sketch) pullPoint(he core.HEntity, to geom.Vector) {
	e := sk.Entity(he)
	sk.pullPart(e.Group, sk.partParams(he), pointGoal{he: he, to: to})
}

// pullGoal is what a pull aims at, as rows of numbers: residual is how
// far each row is from its aim at the params' values now, and reach the
// length, in mm, that a step may move a part by on its way there; rows
// is each row's change for a change of the unknowns hs, len(hs) numbers
// for each row of the residual.
type pullGoal interface {
	residual(sk *Sketch) (b []float64, reach float64)
	rows(sk *Sketch, hs []core.HParam) [][]float64
}

// pointGoal aims point he at to: three rows, its coordinates.
type pointGoal struct {
	he core.HEntity
	to geom.Vector
}

func (g pointGoal) residual(sk *Sketch) ([]float64, float64) {
	r := g.to.Minus(sk.Entity(g.he).PointGetNum(sk))
	return []float64{r.X, r.Y, r.Z}, r.Magnitude()
}

func (g pointGoal) rows(sk *Sketch, hs []core.HParam) [][]float64 {
	ex := sk.Entity(g.he).PointGetExprs(sk)
	a := make([][]float64, 3)
	for i, c := range []*core.Expr{ex.X, ex.Y, ex.Z} {
		a[i] = make([]float64, len(hs))
		for j, h := range hs {
			a[i][j] = core.EvalExpr(sk, c.Derivative(h))
		}
	}
	return a
}

// pullPart moves the mechanism of group hg toward what goal aims at,
// along the freedom the mechanism leaves. Each step finds the
// mechanism's null space (solver.System.NullSpace), takes the
// least-squares step in it that comes nearest the aim, and solves the
// mechanism to take up what the step being linear left. A drive of the
// mechanism, a parameter that is all of a dimension's expression and a
// number itself (pullDrives), takes part as an unknown: its motion
// joins the null space, and its new value is written into the
// parameter, the step cut short where it would cross a bound of the
// dimension it drives. So a part whose only freedom is a drive, as a
// lid on a driven hinge, drags and turns too. part, the params of the
// part pulled, are then recorded as pulled, so that the regeneration
// after keeps them as near as it can while it moves the others.
func (sk *Sketch) pullPart(hg core.HGroup, part []core.HParam, goal pullGoal) {
	block := sk.Mechanism(hg)
	sk.planPulled = core.HGroup{}
	if sk.pullByPlan(block, part, goal) {
		sk.pulled = part
		sk.MarkGroupDirty(block[0], false)
		return
	}
	params := sk.groupParams(block...)
	last := math.Inf(1)
	capped := false
	for range pullSteps {
		b, reach := goal.residual(sk)
		res := magnitude(b)
		// Done where the aim is reached, or where the last step brought
		// it less than a tenth nearer: at the end of a chain stretched
		// toward the target each step gains less than the one before. A
		// step cut short (pullScale) gains less by being short, as a long
		// pull or turn round a hinge: it goes on while it gains a
		// hundredth.
		if gain := res / last; res < core.LengthEps || gain > 0.9 && (!capped || gain > 0.99) {
			break
		}
		last = res
		sys, sg := sk.writeEqSystem(block, params)
		hs, null := sys.NullSpace(sg)
		// A drive leaves no null vector, but its motion is one the pull
		// may take, writing the drive's new value into its parameter.
		var drives []pullDrive
		if hs != nil {
			drives = sk.pullDrives(block, hs)
		}
		rows := goal.rows(sk, hs)
		w := sk.pullWeights(block, hs)
		var y []float64
		var basis [][]float64
		scale, room := 1.0, 1.0
		for {
			basis = slices.Clone(null)
			for _, d := range drives {
				basis = append(basis, d.by)
			}
			if len(basis) == 0 {
				break
			}
			y = leastSquares(overBasis(rows, basis), b, basis, w)
			scale = sk.pullScale(hs, basis, y, w, reach)
			// A drive at its bound, pulled past it, is held there and the
			// rest solved for again; one short of it stops the step there.
			room = 1.0
			blocked := -1
			for i, d := range drives {
				t := d.room(sk, y[len(null)+i]/scale)
				if t < 1e-9 {
					blocked = i
					break
				}
				room = math.Min(room, t)
			}
			if blocked < 0 {
				break
			}
			drives = slices.Delete(drives, blocked, blocked+1)
		}
		if len(basis) == 0 {
			break
		}
		capped = scale > 1
		for j, h := range hs {
			step := 0.0
			for v := range basis {
				step += y[v] * basis[v][j]
			}
			sk.MustParam(h).Val += step * room / scale
		}
		for i, d := range drives {
			sk.writeDrive(d.name, d.at+y[len(null)+i]*room/scale)
		}
		sk.pulled = part
		sk.pulling = true
		sk.solveBlock(block, params, false)
		sk.pulling = false
	}
	sk.pulled = part
	sk.MarkGroupDirty(block[0], false)
}

// overBasis is rows, each a change of what a pull aims at for a change
// of the unknowns, over the motions basis, each a change of the
// unknowns: a row of len(basis) for each row.
func overBasis(rows, basis [][]float64) [][]float64 {
	a := make([][]float64, len(rows))
	for i, row := range rows {
		a[i] = make([]float64, len(basis))
		for j, d := range row {
			if d == 0 {
				continue
			}
			for v := range basis {
				a[i][v] += d * basis[v][j]
			}
		}
	}
	return a
}

// magnitude is the length of v.
func magnitude(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}

// pullScale is what a pull's step, basis·y, is divided by. The step is
// linear, and good only while it is small. Near a stretched chain,
// where the point can't go all the way, the least-squares step that
// comes nearest runs off by hundreds of millimetres and whole turns,
// and the solve can't bring the parts back onto their mates. So a step
// moves no part by more than twice the way left to go, dist, nor turns
// it by more than some 11° (pullMaxTurn); the next step goes on from
// there.
func (sk *Sketch) pullScale(hs []core.HParam, basis [][]float64, y, w []float64, dist float64) float64 {
	maxMove, maxTurn := 0.0, 0.0
	for j := range hs {
		step := 0.0
		for v := range basis {
			step += y[v] * basis[v][j]
		}
		if w[j] > 1 {
			maxTurn = math.Max(maxTurn, math.Abs(step))
		} else {
			maxMove = math.Max(maxMove, math.Abs(step))
		}
	}
	return math.Max(1, math.Max(maxMove/(2*dist), maxTurn/pullMaxTurn))
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

// leastSquares is the y, of k, that makes a·y nearest b, a rows of k,
// by the normal equations. Where a has fewer than k independent
// columns (a mechanism with more freedom than the aim can show), of
// the y that do so equally well it takes the one whose step null·y, a
// change of the unknowns, is least weighed by w (pullWeights): the
// equations are damped by that weighed size, slightly, so that they
// stay positive definite.
func leastSquares(a [][]float64, b []float64, null [][]float64, w []float64) []float64 {
	k := len(null)
	m := make([][]float64, k)
	y := make([]float64, k)
	metric := make([][]float64, k)
	var traceA, traceM float64
	for c := range k {
		m[c] = make([]float64, k)
		metric[c] = make([]float64, k)
		for d := range k {
			for i := range a {
				m[c][d] += a[i][c] * a[i][d]
			}
			for j, wj := range w {
				metric[c][d] += wj * wj * null[c][j] * null[d][j]
			}
		}
		traceA += m[c][c]
		traceM += metric[c][c]
		for i := range a {
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

package sketch

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// A mechanism has, for one value of its drive, as many assemblies as
// its loops can close: a slider-crank's crank above the slide or below
// it for one place of the slide, a four-bar's elbow up or down. Newton
// solves from where the parts were, and goes to whichever assembly it
// comes to first, so a large step of a drive, as a motion study's, can
// land the mechanism on the other one: it flips. So a mechanism whose
// drive changed is solved from where it was in steps (followDrive), by
// the Newton homotopy: its equations less what was left of them where
// it was, F(x) - (1-t)·F(x0) = 0, which x0 solves at t = 0, carried to
// t = 1, F(x) = 0. A step is kept where it lands where two half steps
// land too; where it doesn't, it is halved. Of the assemblies, that
// keeps the one the mechanism moves to continuously from where it was,
// the nearest the previous configuration. Where the mechanism's plan
// (mechanism_plan.go) stands for the solve there is nothing to follow;
// where it places the parts and the solve runs after, they hold their
// equations and there is nothing to follow either; only where the plan
// can't place them does the follow take the step from where they were.

// followDriveMin is the largest residual, in mm or the equations' own
// units, of a mechanism's equations where it was that it is solved from
// in one step: what is left after a pull's step, or a regeneration with
// nothing changed, is solved as before.
const followDriveMin = 1e-3

// followDriveDepth bounds the halvings of a step: a step of a drive is
// never cut finer than 1/64 of it.
const followDriveDepth = 6

// followAgree is how near, as a share of the way the step went, a
// step's landing must be to two half steps' to be kept. A step along a
// mechanism's freedom lands where two halves do but for the path's
// curvature; one onto the other assembly lands a whole step away.
const followAgree = 0.1

// homotopy is the share of the equations' residual where a mechanism
// was, r0 by equation, that writeEqSystem takes off its equations while
// a drive is followed: s, 1-t.
type homotopy struct {
	r0 map[core.HEquation]float64
	s  float64
}

// offset takes h's share of the residual off each of eqs that had one;
// a nil h, no drive followed, leaves them as they are.
func (h *homotopy) offset(eqs []core.Equation) {
	if h == nil {
		return
	}
	for i := range eqs {
		if r := h.r0[eqs[i].H]; r != 0 {
			eqs[i].Expr = eqs[i].Expr.Minus(core.NewConstant(h.s * r))
		}
	}
}

// blockKey names the constraints of the block of groups hgs, whose
// solve a drive's following needs as it was at the last solve: a mate
// added or taken out is not a drive changed, and the parts are put
// where the new mates have them, as before.
func (sk *Sketch) blockKey(hgs []core.HGroup) string {
	var b strings.Builder
	for _, hg := range hgs {
		b.WriteString(strconv.FormatUint(uint64(hg.V), 16))
		b.WriteByte(';')
	}
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if slices.Contains(hgs, c.Group) {
			b.WriteString(strconv.FormatUint(uint64(c.H.V), 16))
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(int(c.Type)))
			if c.Reference {
				b.WriteByte('r')
			}
			b.WriteString(strconv.Itoa(int(c.Limit)))
			b.WriteByte(',')
		}
	}
	return b.String()
}

// followDrive solves the mechanism hgs, for params, from where it is to
// where its constraints now put it, in steps (see above), leaving it at
// the last step's solution for the block's own solve. It does so only
// for a mechanism that was solved with the same constraints at its last
// solve, and whose equations are now further than followDriveMin from
// holding; a step that fails to solve ends the following at the step
// before.
func (sk *Sketch) followDrive(hgs []core.HGroup, params []core.HParam, dofCheckOk bool) {
	key := sk.blockKey(hgs)
	was, ok := sk.solvedKeys[hgs[0]]
	if sk.NoFollow || sk.pulling || !ok || was != key {
		return
	}
	for _, hg := range hgs {
		if !sk.Group(hg).IsSolvedOkay() {
			return
		}
	}
	var eqs []core.Equation
	for _, hg := range hgs {
		sk.WriteEquationsExceptFor(hg, core.NoConstraint, &eqs)
	}
	h := &homotopy{r0: make(map[core.HEquation]float64, len(eqs))}
	worst := 0.0
	for _, eq := range eqs {
		r := core.EvalExpr(sk, eq.Expr)
		if math.IsNaN(r) || math.IsInf(r, 0) {
			return
		}
		h.r0[eq.H] = r
		worst = max(worst, math.Abs(r))
	}
	if worst < followDriveMin {
		return
	}
	sk.FollowedDrives++
	turn := sk.pullWeights(hgs, params)
	solveTo := func(t float64) bool {
		h.s = 1 - t
		sk.homotopy = h
		defer func() { sk.homotopy = nil }()
		return solvedOkay(sk.solveStep(hgs, params, dofCheckOk))
	}
	// far is how far apart two configurations are, a part's turn weighed
	// by a lever of 100 mm, a quaternion component near a radian's half.
	far := func(a, b []float64) float64 {
		d := 0.0
		for i := range a {
			w := 1.0
			if turn[i] > 1 {
				w = 100
			}
			d = max(d, w*math.Abs(a[i]-b[i]))
		}
		return d
	}
	minStep := math.Ldexp(1, -followDriveDepth)
	t, step := 0.0, 1.0
	for t < 1 {
		step = min(step, 1-t)
		from := sk.paramValues(params)
		whole := solveTo(t + step)
		atWhole := sk.paramValues(params)
		if step <= minStep {
			if !whole {
				sk.setParamValues(params, from)
				return
			}
			t += step
			continue
		}
		sk.setParamValues(params, from)
		halves := solveTo(t+step/2) && solveTo(t+step)
		if halves && whole && far(atWhole, sk.paramValues(params)) <= followAgree*far(from, atWhole)+1e-6 {
			sk.FollowDriveSteps++
			t += step
			step *= 2
			continue
		}
		sk.setParamValues(params, from)
		step /= 2
	}
}

// noteSolved remembers the constraints of the block of groups hgs,
// solved now with every member solved, for followDrive at its next solve.
func (sk *Sketch) noteSolved(hgs []core.HGroup) {
	if !movesInMechanism(sk.Group(hgs[0])) {
		return
	}
	if sk.solvedKeys == nil {
		sk.solvedKeys = make(map[core.HGroup]string)
	}
	for _, hg := range hgs {
		if !sk.Group(hg).IsSolvedOkay() {
			delete(sk.solvedKeys, hgs[0])
			return
		}
	}
	sk.solvedKeys[hgs[0]] = sk.blockKey(hgs)
}

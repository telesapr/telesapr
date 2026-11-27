package sketch

import (
	"bytes"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/solver"
)

// A drag of a mechanism's part run by its plan (mechanism_plan.go):
// rather than steps along the null space of the mechanism's equations,
// each followed by a solve (pullPart), the pointer is mapped onto the
// plan's own unknowns, the free turns and slides of the parts and the
// mechanism's drives, and every part is placed by the plan from them.
// A step is Gauss–Newton on what the pull aims at, the loop's closing
// mates held to first order, then the loop closed again by the plan's
// own search from there, as near as it can stay; a step that the loop
// can't close from, or that turns a part by more than a step may, is
// halved. The parts are where the plan puts them, on their mates to
// round-off, so the solve that follows has nothing to take up.
//
// Where the plan doesn't cover the mechanism (a constraint it doesn't
// know, a part nothing joins, a bound that isn't a drive's, a loop it
// can't close from where the parts are) the pull runs as before.

// dragWeight is how much more a loop's closing mate counts than what
// the pull aims at, in a drag step's least squares: the step keeps the
// loop closed to first order, and the plan's search closes it whole.
const dragWeight = 1e4

// dragMaxTurn bounds, in radians, how far a drag step turns any part:
// some 11°, as pullMaxTurn bounds a pull's step, so that the loop's
// search, starting from the step, stays in the assembly it was in.
const dragMaxTurn = 0.2

// dragTol is how near zero what every mate misses by must come for a
// drag step's loops to be closed: a part turned by the one-angle search
// (searchTurn) lands within some 1e-13 of a radian, 1e-11 at the end of
// a lid, more than the plan's own planTol, and well inside the
// solver's tolerance, which the parts are checked on after.
const dragTol = solver.ConvergeTolerance / 100

// dragGain is how much nearer, in mm (or the tenth of a microradian a
// turn's aim is weighed to, turnLever), a drag step must bring the aim
// for another to be taken.
const dragGain = 1e-6

// dragDelta is the change of an unknown of the plan, a radian, a
// millimetre or a unit of a drive, its derivatives are read from by a
// central difference.
const dragDelta = 1e-6

// planDrag is a drag run by a mechanism's plan.
type planDrag struct {
	sk    *Sketch
	block []core.HGroup
	r     *planRun
	// pb is the body pulled; goal what the pull aims at.
	pb   int
	goal pullGoal
	// x is the loop's unknowns, nv of them, then each drive's change
	// from where it was (at0).
	x      []float64
	nv     int
	drives []pullDrive
	at0    []float64
	// driveMates are, of each drive, the live mates of the dimensions it
	// drives.
	driveMates [][]int
	// weights are each unknown's in the least squares' damping, as
	// pullWeights weighs a pull's.
	weights []float64
}

// pullByPlan pulls part, the seven params of a part of mechanism block,
// toward what goal aims at by the mechanism's plan, and reports whether
// it could; where it can't, nothing is changed.
func (sk *Sketch) pullByPlan(block []core.HGroup, part []core.HParam, goal pullGoal) bool {
	if sk.NoMechanismPlan || len(part) == 0 || !movesInMechanism(sk.Group(block[0])) {
		return false
	}
	pb := slices.IndexFunc(block, func(hg core.HGroup) bool { return hg.Param(0) == part[0] }) + 1
	if pb == 0 {
		return false
	}
	for _, hg := range block {
		if !sk.Group(hg).IsSolvedOkay() {
			return false
		}
	}
	p := sk.planOf(block)
	// A part nothing joins stays where it is in the plan, where a pull
	// moves it and what is joined to it as one.
	if p == nil || slices.Contains(p.anchored[1:], true) {
		return false
	}
	if res, ok := sk.blockResidual(block); !ok || res > solver.ConvergeTolerance {
		return false
	}
	fs := sk.newFreedomSetting(block)
	drives, ok := sk.planDrives(fs)
	if !ok {
		return false
	}
	restore := sk.saveParams(sk.planParams(block), block)
	putBack := func() bool {
		for c, v := range fs.valA {
			c.ValA = v
		}
		return restore()
	}
	r, ok := sk.startRun(p, block)
	if !ok {
		return putBack()
	}
	vars, ok := r.closeLoops(false, dragTol)
	if !ok || len(vars)+len(drives) == 0 {
		return putBack()
	}
	dr := &planDrag{sk: sk, block: block, r: r, pb: pb, goal: goal, nv: len(vars), drives: drives}
	dr.x = slices.Concat(vars, make([]float64, len(drives)))
	for _, d := range drives {
		dr.at0 = append(dr.at0, d.at)
		var ms []int
		for mi := range p.mates {
			if slices.ContainsFunc(d.dims, func(c *core.Constraint) bool { return c.H == p.mates[mi].c }) {
				ms = append(ms, mi)
			}
		}
		dr.driveMates = append(dr.driveMates, ms)
	}
	for j := range dr.x {
		w := 1.0
		if j < dr.nv && r.isTurn[j] {
			// A radian of a turn against a quaternion's component, about
			// half of one.
			w = turnWeight / 2
		}
		dr.weights = append(dr.weights, w)
	}
	if !dr.run() {
		return putBack()
	}
	sk.writeRun(r, block)
	if res, ok := sk.blockResidual(block); !ok || res > solver.ConvergeTolerance {
		return putBack()
	}
	for c, v := range fs.valA {
		c.ValA = v
	}
	for i, d := range drives {
		if v := dr.at0[i] + dr.x[dr.nv+i]; v != dr.at0[i] {
			sk.writeDrive(d.name, v)
		}
	}
	sk.PlanPulls++
	sk.planPulled = block[0]
	return true
}

// placedByPull reports whether a pull by the plan placed mechanism hgs
// since the last regeneration and its placement stands for the solve:
// the parts hold every equation, no bound is met or crossed, and the
// solve before left every member solved, as the plan's constraints
// are the same (planKey). The groups keep what that solve wrote: their
// freedom, what was removed or implied; the mates are the same, and
// the parts on them. A bound met is left to the solve, which holds it.
func (sk *Sketch) placedByPull(hgs []core.HGroup) bool {
	if sk.planPulled != hgs[0] {
		return false
	}
	p := sk.plans[hgs[0]]
	if p == nil || !p.covered || !bytes.Equal(p.key, sk.planKey(hgs)) || !sk.boundsClear(hgs) {
		return false
	}
	for _, hg := range hgs {
		if g := sk.Group(hg); !g.IsSolvedOkay() || g.Solved.Timeout || g.SketchDiagnostic != nil {
			return false
		}
	}
	if res, ok := sk.blockResidual(hgs); !ok || res > solver.ConvergeTolerance {
		return false
	}
	for _, hg := range hgs {
		sk.recalculateGroupEntities(hg)
	}
	return true
}

// planDrives are the drives of the block a drag by its plan may move,
// each a parameter that is all of some dimension's expression and whose
// own is a number, with the bounds beside the dimensions it drives; ok
// false where the block has a bound that no drive's dimension measures,
// which the plan doesn't hold, so that the pull's solve, which does,
// takes the drag.
func (sk *Sketch) planDrives(fs *freedomSetting) (out []pullDrive, ok bool) {
	held := map[core.HConstraint]bool{}
	for _, name := range fs.names {
		if !sk.writableDrive(name) {
			continue
		}
		d := pullDrive{name: name, at: fs.vars[name], dims: fs.drives[name], fs: fs}
		for ci := range sk.Constraints {
			b := &sk.Constraints[ci]
			if !fs.bounds[b.H] || b.Limit == core.LimitNone {
				continue
			}
			for _, c := range d.dims {
				if sameMeasure(b, c) {
					d.bounds = append(d.bounds, driveBound{dim: c, bound: b})
					held[b.H] = true
				}
			}
		}
		out = append(out, d)
	}
	for h := range fs.bounds {
		if c := sk.Constraint(h); c != nil && c.Limit != core.LimitNone && !held[h] {
			return nil, false
		}
	}
	return out, true
}

// setDrives gives each drive the value x says, and the live mates of
// the dimensions it drives their values then.
func (dr *planDrag) setDrives(x []float64) {
	for i, d := range dr.drives {
		d.fs.setDrive(dr.sk, d.name, dr.at0[i]+x[dr.nv+i])
		for _, mi := range dr.driveMates[i] {
			dr.r.mates[mi].setValue(dr.sk.Constraint(dr.r.plan.mates[mi].c))
		}
	}
}

// eval places the mechanism at unknowns x, and returns what its loops'
// mates miss by and what the pull's aim does, with the reach of a step.
func (dr *planDrag) eval(x []float64) (f, b []float64, reach float64) {
	dr.setDrives(x)
	f = dr.r.evalPlan(x[:dr.nv], false)
	b, reach = dr.aim()
	return f, b, reach
}

// aim is what the pull's goal misses by with the pulled part where the
// run has placed it: its params are given the placement, which writeRun
// or the params' restore puts right after.
func (dr *planDrag) aim() ([]float64, float64) {
	pb := dr.pb
	dr.sk.setPlacement(dr.block[pb-1], dr.r.pl[pb], dr.r.start[pb].q)
	return dr.goal.residual(dr.sk)
}

// close closes the loops at x's drives from x's loop unknowns, and
// leaves the mechanism placed there; false where it can't.
func (dr *planDrag) close(x []float64) bool {
	dr.setDrives(x)
	if maxAbs(dr.r.evalPlan(x[:dr.nv], false)) <= dragTol {
		return true
	}
	if dr.nv == 0 {
		return false
	}
	dr.r.levenberg(x[:dr.nv])
	return maxAbs(dr.r.evalPlan(x[:dr.nv], false)) <= dragTol
}

// run takes the drag's steps; false where the plan loses the
// mechanism, its loops not closed where it ends.
func (dr *planDrag) run() bool {
	last := math.Inf(1)
	for range pullSteps {
		f, b, reach := dr.eval(dr.x)
		res := magnitude(b)
		// Done where the aim is reached, or where the last step brought it
		// less than dragGain nearer: a step is cheap, so the drag goes on
		// to where the point comes nearest along its path, as at the end of
		// a chain stretched toward the target, rather than stopping where
		// a step gains less than a tenth, as a pull's solve does.
		if res < core.LengthEps || last-res < dragGain {
			break
		}
		last = res
		dx := dr.step(f, b)
		if dx == nil || !dr.take(dx, res, reach) {
			break
		}
	}
	// Each drive moved as writeDrive will write it, to a millionth, and the
	// loops closed there, so that the parts hold their mates at it.
	for i := range dr.drives {
		j := dr.nv + i
		if dr.x[j] == 0 {
			continue
		}
		dr.x[j] = math.Round((dr.at0[i]+dr.x[j])*1e6)/1e6 - dr.at0[i]
	}
	return dr.close(dr.x)
}

// step is the Gauss–Newton step of the unknowns from x toward the aim,
// the loops kept closed to first order, a drive held where a step would
// carry it past its bound; nil where no unknown moves the aim.
func (dr *planDrag) step(f, b []float64) []float64 {
	n := len(dr.x)
	cols := make([][]float64, n)
	trial := slices.Clone(dr.x)
	for j := range n {
		trial[j] = dr.x[j] + dragDelta
		fp, bp, _ := dr.eval(trial)
		trial[j] = dr.x[j] - dragDelta
		fm, bm, _ := dr.eval(trial)
		trial[j] = dr.x[j]
		col := make([]float64, 0, len(fp)+len(bp))
		for i := range fp {
			col = append(col, dragWeight*(fp[i]-fm[i])/(2*dragDelta))
		}
		for i := range bp {
			col = append(col, (bp[i]-bm[i])/(2*dragDelta))
		}
		cols[j] = col
	}
	// Back where x places the mechanism, its drives' values with it.
	dr.eval(dr.x)
	rhs := make([]float64, 0, len(f)+len(b))
	for _, v := range f {
		rhs = append(rhs, -dragWeight*v)
	}
	for _, v := range b {
		rhs = append(rhs, -v)
	}
	active := make([]bool, n)
	for j := range n {
		active[j] = slices.ContainsFunc(cols[j], func(v float64) bool { return v != 0 })
	}
	for {
		var idx []int
		for j := range n {
			if active[j] {
				idx = append(idx, j)
			}
		}
		if len(idx) == 0 {
			return nil
		}
		a := make([][]float64, len(rhs))
		basis := make([][]float64, len(idx))
		w := make([]float64, len(idx))
		for i := range rhs {
			a[i] = make([]float64, len(idx))
			for k, j := range idx {
				a[i][k] = cols[j][i]
			}
		}
		for k, j := range idx {
			basis[k] = make([]float64, len(idx))
			basis[k][k] = 1
			w[k] = dr.weights[j]
		}
		y := leastSquares(a, rhs, basis, w)
		dx := make([]float64, n)
		for k, j := range idx {
			dx[j] = y[k]
		}
		// A drive at its bound, pulled past it, is held there and the
		// rest solved for again, as a pull holds it.
		blocked := false
		for i := range dr.drives {
			j := dr.nv + i
			d := &dr.drives[i]
			d.at = dr.at0[i] + dr.x[j]
			if active[j] && dr.room(i, dx[j]) < 1e-9 {
				active[j], blocked = false, true
			}
		}
		if !blocked {
			return dx
		}
	}
}

// take moves the unknowns by s·dx, s 1 or less: cut short so that no
// part turns by more than dragMaxTurn, no slide moves more than twice
// the way left to go (reach), and no drive past its bound, and halved
// until the loops close from the step and the aim, res now, comes
// nearer. It reports false where no step would do.
func (dr *planDrag) take(dx []float64, res, reach float64) bool {
	s := 1.0
	for j, v := range dx {
		switch {
		case j < dr.nv && dr.r.isTurn[j]:
			s = math.Min(s, dragMaxTurn/math.Max(math.Abs(v), 1e-300))
		case j < dr.nv:
			s = math.Min(s, 2*reach/math.Max(math.Abs(v), 1e-300))
		default:
			s = math.Min(s, dr.room(j-dr.nv, v))
		}
	}
	was := slices.Clone(dr.r.pl)
	trial := make([]float64, len(dx))
	for range 12 {
		for j := range dx {
			trial[j] = dr.x[j] + s*dx[j]
		}
		if dr.close(trial) && turnedAtMost(was, dr.r.pl, 1.25*dragMaxTurn) {
			if b, _ := dr.aim(); magnitude(b) < res {
				copy(dr.x, trial)
				return true
			}
		}
		s /= 2
	}
	dr.eval(dr.x)
	return false
}

// room is how much of a change dz drive i can take within its bounds,
// from 0 to 1, as pullDrive.room has it, but none where it is a hair
// past a bound already, as the halving to a bound can leave it, and the
// change would carry it further.
func (dr *planDrag) room(i int, dz float64) float64 {
	d := &dr.drives[i]
	if !d.within(dr.sk, d.at) {
		if d.within(dr.sk, d.at+dz) {
			return 1
		}
		return 0
	}
	return d.room(dr.sk, dz)
}

// turnedAtMost reports whether no body turns by more than most radians
// from placements was to now.
func turnedAtMost(was, now []place, most float64) bool {
	for b := 1; b < len(now); b++ {
		if rotationVector(now[b].q.Times(was[b].q.Inverse())).Magnitude() > most {
			return false
		}
	}
	return true
}

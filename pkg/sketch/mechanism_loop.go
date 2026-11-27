package sketch

import (
	"math"
)

// A loop of parts, a four-bar's or an engine's crank, rod and piston,
// has a part whose mates join it to two bodies placed before it: placed
// from the one, what its mates to the other miss by is what the loop
// is open by. The plan closes it as Kramer does, over the freedom the
// parts before it leave: the turns their mates leave free and their
// slides, the loop's unknowns, few (one for a four-bar driven by its
// crank or for the engine, the coupler's or the rod's turn). The search
// starts where the parts are, all unknowns zero, so that of two ways to
// close a loop (a crank-rocker's two assemblies) it takes the one
// nearest, and steps of a motion study don't flip it. Where that
// doesn't close it, as from a start far off, a four-bar drawn at full
// stretch, the search starts again from each of a sweep of each turn
// round the whole circle, nearest first.

// planTol is how near zero what every mate misses by must come for the
// plan to have placed the mechanism, in mm or the sine of an angle:
// well inside the solver's own (solver.ConvergeTolerance).
const planTol = 1e-11

// planSweep is how many starts round a whole turn the loop's search
// tries when it can't close from where the parts are.
const planSweep = 36

// evalPlan places every body in order with the loop's unknowns vars,
// and returns what the mates miss by, every body's in turn.
func (r *planRun) evalPlan(vars []float64, structure bool) []float64 {
	var f []float64
	for _, b := range r.plan.order {
		if r.plan.anchored[b] {
			r.pl[b] = r.start[b]
			continue
		}
		r.placeBody(b, vars, structure)
		f = append(f, r.bodies[b].g...)
	}
	return f
}

// closeLoops places the mechanism, its loops closed, and returns the
// loop's unknowns it is closed at; false where the search can't close
// them. Without sweep the search starts only from where the parts are,
// as a drag's must, to keep the assembly they are in; and a drag takes
// them as closed within tol, which may be more than planTol
// (mechanism_drag.go).
func (r *planRun) closeLoops(sweep bool, tol float64) ([]float64, bool) {
	f := r.evalPlan(nil, true)
	vars := make([]float64, r.nv)
	if maxAbs(f) <= tol {
		return vars, true
	}
	if r.nv == 0 {
		return nil, false
	}
	if r.levenberg(vars) || tol > planTol && maxAbs(r.evalPlan(vars, false)) <= tol {
		return vars, true
	}
	if !sweep {
		return nil, false
	}
	for j := r.nv - 1; j >= 0; j-- {
		if !r.isTurn[j] {
			continue
		}
		for k := 1; k < planSweep; k++ {
			// 10°, -10°, 20°, -20°... from where it is.
			a := float64((k+1)/2) * 2 * math.Pi / planSweep
			if k%2 == 0 {
				a = -a
			}
			clear(vars)
			vars[j] = a
			if r.levenberg(vars) {
				return vars, true
			}
		}
	}
	return nil, false
}

// levenberg solves for the loop's unknowns from vars by
// Levenberg–Marquardt, its Jacobian by central differences; with more
// unknowns than the loop holds, as a four-bar whose crank is free, the
// damped step stays near where it starts. It leaves the bodies placed
// at the unknowns it ends with, and reports whether every mate holds.
func (r *planRun) levenberg(vars []float64) bool {
	const h = 1e-7
	n := len(vars)
	f := r.evalPlan(vars, false)
	lambda := 1e-6
	trial := make([]float64, n)
	for range 60 {
		if maxAbs(f) <= planTol {
			return true
		}
		jac := make([][]float64, n)
		for j := range n {
			copy(trial, vars)
			trial[j] = vars[j] + h
			fp := r.evalPlan(trial, false)
			trial[j] = vars[j] - h
			fm := r.evalPlan(trial, false)
			jac[j] = make([]float64, len(f))
			for i := range f {
				jac[j][i] = (fp[i] - fm[i]) / (2 * h)
			}
		}
		a := make([][]float64, n)
		y := make([]float64, n)
		for i := range n {
			a[i] = make([]float64, n)
			for j := range n {
				for k := range f {
					a[i][j] += jac[i][k] * jac[j][k]
				}
			}
			for k := range f {
				y[i] -= jac[i][k] * f[k]
			}
		}
		better := false
		for lambda < 1e8 {
			m := make([][]float64, n)
			for i := range n {
				m[i] = append([]float64(nil), a[i]...)
				m[i][i] += lambda * (a[i][i] + 1e-9)
			}
			step, ok := solveDense(m, y)
			if !ok {
				lambda *= 10
				continue
			}
			// A step of a loop's turn is kept below a radian: the loop
			// is far from linear over more.
			if big := maxAbs(step); big > 1 {
				for i := range step {
					step[i] /= big
				}
			}
			for i := range n {
				trial[i] = vars[i] + step[i]
			}
			if nf := r.evalPlan(trial, false); sumSq(nf) < sumSq(f) {
				copy(vars, trial)
				f = nf
				lambda = math.Max(lambda/10, 1e-12)
				better = true
				break
			}
			lambda *= 10
		}
		if !better {
			break
		}
	}
	f = r.evalPlan(vars, false)
	return maxAbs(f) <= planTol
}

// solveDense solves m·x = y by Gaussian elimination with partial
// pivoting; false where m is singular.
func solveDense(m [][]float64, y []float64) ([]float64, bool) {
	n := len(y)
	x := append([]float64(nil), y...)
	for c := range n {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(m[r][c]) > math.Abs(m[p][c]) {
				p = r
			}
		}
		if math.Abs(m[p][c]) < 1e-300 {
			return nil, false
		}
		m[c], m[p] = m[p], m[c]
		x[c], x[p] = x[p], x[c]
		for r := c + 1; r < n; r++ {
			k := m[r][c] / m[c][c]
			for j := c; j < n; j++ {
				m[r][j] -= k * m[c][j]
			}
			x[r] -= k * x[c]
		}
	}
	for c := n - 1; c >= 0; c-- {
		for j := c + 1; j < n; j++ {
			x[c] -= m[c][j] * x[j]
		}
		x[c] /= m[c][c]
	}
	return x, true
}

// maxAbs is the greatest magnitude in v, 0 for none.
func maxAbs(v []float64) float64 {
	m := 0.0
	for _, x := range v {
		m = math.Max(m, math.Abs(x))
	}
	return m
}

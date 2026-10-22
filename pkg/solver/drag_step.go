package solver

import "math"

// While the user drags, each mouse move solves the group again from
// where the last left it, a little off: the same equations, near the
// same point. The Newton step's factorisation of the Jacobian is then
// most of a solve, and the one made at the last move, or at the step
// before, is nearly the Jacobian's now. So during a drag a factorisation
// is kept, and the next steps are taken with it, the Jacobian left
// unevaluated (a chord method), for as long as each step at least halves
// what is left of the equations; when one doesn't, the next step
// factors afresh. The steps converge to the same tolerance as Newton's,
// to another point within it: a dragged sketch comes to rest in other
// last bits. A solve with nothing dragged never takes a kept step.

// minKeptStep is the fewest equations a system has for its step to be
// kept: the equations solved alone are one each, and factor at once.
const minKeptStep = 4

// maxKeptSteps bounds the factorisations a System keeps, one for each
// Jacobian it solves with during a drag.
const maxKeptSteps = 64

// keptStep is a factorisation of the scaled Jacobian, with the scale
// it was factored at.
type keptStep struct {
	sf    *stepFactor
	scale []float64
}

// keepStep keeps sf, the Newton step's factorisation of the Jacobian
// written as mat.id, scaled by scale, while something is dragged.
func (s *System) keepStep(sf *stepFactor, scale []float64) {
	if len(s.Dragged) == 0 || s.mat.m < minKeptStep {
		return
	}
	if s.kept == nil || len(s.kept) >= maxKeptSteps {
		s.kept = make(map[uint64]keptStep)
	}
	s.kept[s.mat.id] = keptStep{sf: sf, scale: scale}
}

// keptStepFor is the factorisation kept for the Jacobian written now,
// if one is and the same params are dragged.
func (s *System) keptStepFor() (keptStep, bool) {
	if len(s.Dragged) == 0 || s.mat.m < minKeptStep {
		return keptStep{}, false
	}
	k, ok := s.kept[s.mat.id]
	if !ok || k.sf.a.rows != s.mat.m || k.sf.a.cols != s.mat.n {
		return keptStep{}, false
	}
	for c, h := range s.mat.param {
		if (k.scale[c] != 1) != s.isDragged(h) {
			return keptStep{}, false
		}
	}
	return k, true
}

// takeKeptStep is the step SolveLeastSquares would take, from the kept
// factorisation in place of the Jacobian evaluated now.
func (s *System) takeKeptStep(k keptStep) {
	s.keptSteps++
	s.mat.X = k.sf.solve(s.mat.Bnum)
	for c := range s.mat.n {
		s.mat.X[c] *= k.scale[c]
	}
}

// residual is the size of what is left of the equations.
func residual(b []float64) float64 {
	var sum float64
	for _, v := range b {
		sum += v * v
	}
	return math.Sqrt(sum)
}

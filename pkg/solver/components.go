package solver

import (
	"math"
	"slices"
)

// componentTag is the first tag newtonSolveComponents puts on the
// equations and params of a component, counting down: it meets none of
// the tags Solve puts (the equations solved alone count up from 1).
const componentTag = -1

// newtonSolveComponents is NewtonSolve of the system written for tag 0,
// one connected component at a time: a sketch of several profiles that
// share no unknown is several systems, and each is solved alone, to its
// own convergence, rather than all of them stepped together until the
// slowest converges, each step factoring them all. It leaves the system
// for tag 0 written and evaluated at the solution, as NewtonSolve does,
// and reports whether every equation converged.
func (s *System) newtonSolveComponents() bool {
	if s.whole {
		return s.NewtonSolve()
	}
	comps := s.components()
	if len(comps) <= 1 {
		return s.NewtonSolve()
	}
	eqs := slices.Clone(s.mat.eq)
	params := slices.Clone(s.mat.param)
	ok := true
	for k, c := range comps {
		tag := componentTag - k
		for _, i := range c.rows {
			eqs[i].Tag = tag
		}
		for _, j := range c.cols {
			s.findParam(params[j]).Tag = tag
		}
		s.WriteJacobian(tag)
		if !s.NewtonSolve() {
			ok = false
			break
		}
	}
	for _, e := range eqs {
		e.Tag = 0
	}
	for _, h := range params {
		s.findParam(h).Tag = 0
	}
	s.WriteJacobian(0)
	s.mat.Bnum = make([]float64, s.mat.m)
	for i := range s.mat.m {
		s.mat.Bnum[i] = s.mat.B[i].Eval(nil, nil)
		if isReasonable(s.mat.Bnum[i]) || math.Abs(s.mat.Bnum[i]) > ConvergeTolerance {
			ok = false
		}
	}
	return ok
}

// component is a set of the written system's equations, by row, and of
// its unknowns, by column, that no equation joins to another set.
type component struct {
	rows, cols []int
}

// components splits the system written for tag 0 into its connected
// components, in the order of their first equation. An equation that
// reads no unknown is in none: it is checked once the components are
// solved.
func (s *System) components() []component {
	m, n := s.mat.m, s.mat.n
	parent := make([]int, n)
	for j := range n {
		parent[j] = j
	}
	find := func(j int) int {
		for parent[j] != j {
			parent[j] = parent[parent[j]]
			j = parent[j]
		}
		return j
	}
	firstCol := make([]int, m)
	for i := range m {
		firstCol[i] = -1
		for j, pd := range s.mat.A[i] {
			if pd == nil {
				continue
			}
			if firstCol[i] < 0 {
				firstCol[i] = j
				continue
			}
			if a, b := find(firstCol[i]), find(j); a != b {
				parent[max(a, b)] = min(a, b)
			}
		}
	}
	index := make(map[int]int)
	var comps []component
	for i := range m {
		if firstCol[i] < 0 {
			continue
		}
		r := find(firstCol[i])
		k, ok := index[r]
		if !ok {
			k = len(comps)
			index[r] = k
			comps = append(comps, component{})
		}
		comps[k].rows = append(comps[k].rows, i)
	}
	for j := range n {
		// An unknown no equation reads stays where it is, in none.
		if k, ok := index[find(j)]; ok {
			comps[k].cols = append(comps[k].cols, j)
		}
	}
	return comps
}

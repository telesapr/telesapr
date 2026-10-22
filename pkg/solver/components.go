package solver

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
)

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
	// Each component's system is the whole's rows and columns for it:
	// the partials are those WriteJacobian would write for the
	// component alone, an equation reading no unknown of another, so
	// they are taken rather than written again.
	whole := s.mat
	ok := true
	for k, c := range comps {
		s.mat.m, s.mat.n = len(c.rows), len(c.cols)
		s.mat.eq = make([]*Equation, len(c.rows))
		s.mat.B = make([]*core.Expr, len(c.rows))
		s.mat.A = make([][]*core.Expr, len(c.rows))
		for r, i := range c.rows {
			s.mat.eq[r], s.mat.B[r] = whole.eq[i], whole.B[i]
			s.mat.A[r] = make([]*core.Expr, len(c.cols))
			for q, j := range c.cols {
				s.mat.A[r][q] = whole.A[i][j]
			}
		}
		s.mat.param = make([]core.HParam, len(c.cols))
		for q, j := range c.cols {
			s.mat.param[q] = whole.param[j]
		}
		s.mat.id = s.written.componentID(whole.id, k)
		if !s.NewtonSolve() {
			ok = false
			break
		}
	}
	s.mat.m, s.mat.n = whole.m, whole.n
	s.mat.eq, s.mat.param = whole.eq, whole.param
	s.mat.A, s.mat.B, s.mat.id = whole.A, whole.B, whole.id
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

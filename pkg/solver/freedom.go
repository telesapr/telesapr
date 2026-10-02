package solver

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// This file is not a port: C++ SolveSpace finds the free params only
// on demand (MarkParamsFree), with a rank test per param. FreeParams
// answers for every param at once, so that it can run after each solve.

// FreeParams reports, for each unknown of the group, whether its
// equations leave it free to move, to first order, at the unknowns'
// current values. It writes the Jacobian of all the group's equations
// over all its unknowns, and does not solve or change anything; the
// sketch should be solved first. Nil when the system is too large to
// write.
//
// An unknown p is fixed when its unit vector e_p lies in the row space
// of the Jacobian J: some combination of the equations' gradients then
// moves p alone. That is what MarkParamsFree tests one param at a time
// (removing p's column loses rank exactly when e_p is in the row
// space). Here one reduction of J to reduced row echelon form answers
// for all of them: e_p is in the row space exactly when p's column has
// a pivot and the pivot's row is zero everywhere else.
func (s *System) FreeParams(g *Group) map[core.HParam]bool {
	s.writeEquationsExceptFor(core.NoConstraint, g)
	s.clearParamTags()
	s.clearEqTags()
	if !s.WriteJacobian(0) {
		return nil
	}
	s.EvalJacobian()
	m, n := s.mat.m, s.mat.n
	a := make([][]float64, m)
	for i := range a {
		a[i] = append([]float64(nil), s.mat.Anum[i]...)
	}
	fixed := rrefFixedColumns(a, m, n)
	free := make(map[core.HParam]bool, n)
	for j, h := range s.mat.param {
		free[h] = !fixed[j]
	}
	return free
}

// rrefFixedColumns reduces a, m rows of n, to reduced row echelon
// form in place, with partial pivoting, and reports for each column
// whether its unit vector lies in the row space.
func rrefFixedColumns(a [][]float64, m, n int) []bool {
	maxAbs := 0.0
	for i := 0; i < m; i++ {
		for _, v := range a[i] {
			maxAbs = math.Max(maxAbs, math.Abs(v))
		}
	}
	// A pivot smaller than this is taken as zero, as the rank test's
	// QR does with its threshold.
	tol := float64(20*(m+n)) * maxAbs * epsilon
	pivotRow := make([]int, n)
	row := 0
	for j := 0; j < n; j++ {
		pivotRow[j] = -1
		if row == m {
			continue
		}
		best, bestAbs := -1, tol
		for i := row; i < m; i++ {
			if v := math.Abs(a[i][j]); v > bestAbs {
				best, bestAbs = i, v
			}
		}
		if best < 0 {
			continue
		}
		a[row], a[best] = a[best], a[row]
		pr := a[row]
		inv := 1 / pr[j]
		for k := j; k < n; k++ {
			pr[k] *= inv
		}
		pr[j] = 1
		for i := 0; i < m; i++ {
			if i == row || a[i][j] == 0 {
				continue
			}
			f := a[i][j]
			ri := a[i]
			for k := j; k < n; k++ {
				ri[k] -= f * pr[k]
			}
			ri[j] = 0
		}
		pivotRow[j] = row
		row++
	}
	fixed := make([]bool, n)
	for j := 0; j < n; j++ {
		r := pivotRow[j]
		if r < 0 {
			continue
		}
		alone := true
		for k := 0; k < n; k++ {
			if k != j && math.Abs(a[r][k]) > 1e-9 {
				alone = false
				break
			}
		}
		fixed[j] = alone
	}
	return fixed
}

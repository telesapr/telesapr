package solver

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
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
	// As the solver does, and for the same speed: an equation a = b
	// between two unknowns takes b out, so the Jacobian is written over
	// fewer of them. b is then exactly as free as what replaced it.
	subMap := s.SolveBySubstitution()
	if !s.WriteJacobian(0) {
		return nil
	}
	s.EvalJacobian()
	m, n := s.mat.m, s.mat.n
	a := make([][]float64, m)
	for i := range a {
		a[i] = append([]float64(nil), s.mat.Anum[i]...)
	}
	fixed := fixedColumns(a, m, n)
	free := make(map[core.HParam]bool, n)
	for j, h := range s.mat.param {
		free[h] = !fixed[j]
	}
	for h, by := range subMap {
		free[h] = free[by.H]
	}
	return free
}

// fixedColumns is rrefFixedColumns, faster on what sketches make. Two
// cheap reductions come first, each of which keeps the answer exact:
//
//   - A row that is c*(e_a - e_b), an equation a = b such as a
//     coincidence makes, merges a and b: e_a - e_b is in the row space,
//     so e_a is in it exactly when e_b is, and the other rows can be
//     taken over the merged unknowns, each class summing its columns.
//   - A row with one nonzero, an unknown that one equation fixes, puts
//     that unit vector in the row space; the column then leaves every
//     other row, which is a row operation, and may leave another row
//     with one nonzero.
//
// Only the rows and classes left go to the dense reduction.
func fixedColumns(a [][]float64, m, n int) []bool {
	maxAbs := 0.0
	for i := range m {
		for _, v := range a[i] {
			maxAbs = math.Max(maxAbs, math.Abs(v))
		}
	}
	tol := float64(20*(m+n)) * maxAbs * epsilon
	nonzero := func(row []float64) []int {
		var idx []int
		for j, v := range row {
			if math.Abs(v) > tol {
				idx = append(idx, j)
			}
		}
		return idx
	}

	// Classes of unknowns merged by equalities.
	parent := make([]int, n)
	for j := range parent {
		parent[j] = j
	}
	find := func(j int) int {
		for parent[j] != j {
			parent[j] = parent[parent[j]]
			j = parent[j]
		}
		return j
	}
	used := make([]bool, m)
	for i := range m {
		idx := nonzero(a[i])
		if len(idx) == 2 {
			x, y := a[i][idx[0]], a[i][idx[1]]
			if math.Abs(x+y) <= tol*math.Max(1, math.Abs(x)) {
				used[i] = true
				if rx, ry := find(idx[0]), find(idx[1]); rx != ry {
					parent[ry] = rx
				}
			}
		}
	}

	// The other rows over the classes.
	var rows [][]float64
	for i := range m {
		if used[i] {
			continue
		}
		r := make([]float64, n)
		found := false
		for j, v := range a[i] {
			if v != 0 {
				r[find(j)] += v
				found = true
			}
		}
		if found {
			rows = append(rows, r)
		}
	}

	// Classes fixed by a row with one nonzero, until none is left.
	classFixed := make([]bool, n)
	for changed := true; changed; {
		changed = false
		for _, r := range rows {
			idx := nonzero(r)
			if len(idx) != 1 || classFixed[idx[0]] {
				continue
			}
			c := idx[0]
			classFixed[c] = true
			changed = true
			for _, rr := range rows {
				rr[c] = 0
			}
		}
	}

	// The rest, dense, over the classes still open.
	var open []int
	for j := range n {
		if find(j) == j && !classFixed[j] {
			open = append(open, j)
		}
	}
	var rest [][]float64
	for _, r := range rows {
		rr := make([]float64, len(open))
		found := false
		for k, j := range open {
			rr[k] = r[j]
			if math.Abs(r[j]) > tol {
				found = true
			}
		}
		if found {
			rest = append(rest, rr)
		}
	}
	openFixed := rrefFixedColumns(rest, len(rest), len(open))
	for k, j := range open {
		classFixed[j] = openFixed[k]
	}
	fixed := make([]bool, n)
	for j := range n {
		fixed[j] = classFixed[find(j)]
	}
	return fixed
}

// rrefFixedColumns reduces a, m rows of n, to reduced row echelon
// form in place, with partial pivoting, and reports for each column
// whether its unit vector lies in the row space.
func rrefFixedColumns(a [][]float64, m, n int) []bool {
	maxAbs := 0.0
	for i := range m {
		for _, v := range a[i] {
			maxAbs = math.Max(maxAbs, math.Abs(v))
		}
	}
	// A pivot smaller than this is taken as zero, as the rank test's
	// QR does with its threshold.
	tol := float64(20*(m+n)) * maxAbs * epsilon
	pivotRow := make([]int, n)
	row := 0
	for j := range n {
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
		for i := range m {
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
	for j := range n {
		r := pivotRow[j]
		if r < 0 {
			continue
		}
		alone := true
		for k := range n {
			if k != j && math.Abs(a[r][k]) > 1e-9 {
				alone = false
				break
			}
		}
		fixed[j] = alone
	}
	return fixed
}

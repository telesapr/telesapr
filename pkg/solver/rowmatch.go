package solver

import "slices"

// Eigen's symbolic analysis assumes that row i of the matrix factored
// has an entry in column i once the columns are ordered: coletree
// starts each row's first column at its own index, and factorize adds
// a structural zero on the diagonal where there is none. The rows of a
// Jacobian are equations in handle order and its columns params, so
// nothing relates row i to column i; the assumed diagonal couples rows
// far apart, the elimination tree becomes a long chain, and the factors
// of a chain of n segments hold about n²/8 entries where 2n would do.
// Renumbering the rows so that row i has an entry in the i-th column
// ordered restores the band: a chain of 400 segments factors in a
// hundredth of the time. A row permutation leaves A'A, and so R up to
// rounding, and the rank as it is.

// matchRows is the row permutation that puts at row i a row with an
// entry in column i of a ordered by permC (column permC[k] of the
// matrix factored is column k of a), where there is one: newRow[r] is
// where row r goes. Columns are matched greedily in order, each to its
// first row not yet taken; the rows left over fill the places left
// over, both in order, so that the same pattern always gives the same
// permutation.
func matchRows(a *spMat, permC []int) []int {
	m := a.rows
	newRow := make([]int, m)
	for i := range newRow {
		newRow[i] = -1
	}
	taken := make([]bool, m)
	order := make([]int, a.cols) // order[j] is the column of a ordered j-th
	for k, j := range permC {
		order[j] = k
	}
	for j := range min(m, a.cols) {
		for _, r := range a.idx[order[j]] {
			if newRow[r] < 0 {
				newRow[r] = j
				taken[j] = true
				break
			}
		}
	}
	place := 0
	for r := range m {
		if newRow[r] >= 0 {
			continue
		}
		for taken[place] {
			place++
		}
		newRow[r] = place
		taken[place] = true
	}
	return newRow
}

// permuteRows is a with row r moved to newRow[r], each column's
// entries sorted by row as a compressed matrix keeps them.
func permuteRows(a *spMat, newRow []int) *spMat {
	p := &spMat{rows: a.rows, cols: a.cols, idx: make([][]int, a.cols), val: make([][]float64, a.cols)}
	nnz := 0
	for j := range a.cols {
		nnz += len(a.idx[j])
	}
	idx := make([]int, nnz)
	val := make([]float64, nnz)
	at := 0
	for j := range a.cols {
		n := len(a.idx[j])
		ci, cv := idx[at:at+n:at+n], val[at:at+n:at+n]
		for k, r := range a.idx[j] {
			ci[k], cv[k] = newRow[r], a.val[j][k]
		}
		// Insertion sort: a column of a Jacobian holds a few entries.
		for k := 1; k < n; k++ {
			for q := k; q > 0 && ci[q-1] > ci[q]; q-- {
				ci[q-1], ci[q] = ci[q], ci[q-1]
				cv[q-1], cv[q] = cv[q], cv[q-1]
			}
		}
		p.idx[j], p.val[j] = ci, cv
		at += n
	}
	return p
}

// matchedCache keeps, for the last pattern factored with its rows
// matched, the row permutation and the analysis of the permuted
// pattern, which depend on the pattern alone.
type matchedCache struct {
	rows int
	// idx is a copy of the pattern before its rows were permuted.
	idx    [][]int
	newRow []int
	sym    *qrSymbolic
}

// matched is a with its rows matched to its ordered columns, and the
// analysis of that matrix: kept if a has the pattern last seen.
func (c *matchedCache) matched(a *spMat) (*spMat, *qrSymbolic) {
	if c.matches(a) {
		return permuteRows(a, c.newRow), c.sym
	}
	permC := colamdOrdering(a.rows, a.cols, a.idx)
	c.rows = a.rows
	c.idx = make([][]int, a.cols)
	for j := range a.cols {
		c.idx[j] = slices.Clone(a.idx[j])
	}
	c.newRow = matchRows(a, permC)
	p := permuteRows(a, c.newRow)
	c.sym = analyzeSparseQROrdered(p, permC)
	return p, c.sym
}

// matches reports whether a has the pattern c was made for.
func (c *matchedCache) matches(a *spMat) bool {
	if c.sym == nil || a.rows != c.rows || a.cols != len(c.idx) {
		return false
	}
	for j := range a.cols {
		if !slices.Equal(a.idx[j], c.idx[j]) {
			return false
		}
	}
	return true
}

// factorMatched factors a with its rows matched, through c.
func factorMatched(a *spMat, c *matchedCache, rankOnly bool) *sparseQR {
	p, sy := c.matched(a)
	return factorSparseQRAnalyzed(p, sy, rankOnly)
}

package solver

import "math"

// spMat is a column-major sparse matrix as Eigen stores one: each
// column's row indices and values, in the order they were inserted.
// Eigen's sparse kernels sum in storage order, so the order is kept
// wherever Eigen keeps it.
type spMat struct {
	rows, cols int
	idx        [][]int
	val        [][]float64
}

// spFromRows makes the m x n sparse matrix with the nonzero entries of
// a (a[i][j], row i, column j), as C++ System::EvalJacobian does: an
// entry that is exactly zero is not stored.
func spFromRows(a [][]float64, m, n int) *spMat {
	s := &spMat{rows: m, cols: n, idx: make([][]int, n), val: make([][]float64, n)}
	for j := range n {
		for i := range m {
			if a[i][j] != 0 {
				s.idx[j] = append(s.idx[j], i)
				s.val[j] = append(s.val[j], a[i][j])
			}
		}
	}
	return s
}

// transpose returns s' with sorted columns, as assigning a transposed
// sparse matrix to a SparseMatrix does.
func (s *spMat) transpose() *spMat {
	t := &spMat{rows: s.cols, cols: s.rows, idx: make([][]int, s.rows), val: make([][]float64, s.rows)}
	for j := range s.cols {
		for k, i := range s.idx[j] {
			t.idx[i] = append(t.idx[i], j)
			t.val[i] = append(t.val[i], s.val[j][k])
		}
	}
	return t
}

// colDot is SparseMatrixBase::dot of column j with the dense vector x.
func (s *spMat) colDot(j int, x []float64) float64 {
	res := 0.0
	for k, i := range s.idx[j] {
		res += s.val[j][k] * x[i]
	}
	return res
}

// mulTransposed returns s' x, as Eigen's product of a transposed
// column-major matrix with a dense vector computes it.
func (s *spMat) mulTransposed(x []float64) []float64 {
	res := make([]float64, s.cols)
	for j := range s.cols {
		tmp := 0.0
		for k, i := range s.idx[j] {
			tmp += s.val[j][k] * x[i]
		}
		res[j] += 1 * tmp
	}
	return res
}

// mulSelfTransposed returns s s', as Eigen's conservative sparse
// product computes it for a column-major s: s is copied to row-major,
// each row of the product is accumulated over the columns of s in
// order, and the row-major result is transposed back, which sorts it.
func (s *spMat) mulSelfTransposed() *spMat {
	m := s.rows
	rowIdx := make([][]int, m)
	rowVal := make([][]float64, m)
	for j := range s.cols {
		for k, i := range s.idx[j] {
			rowIdx[i] = append(rowIdx[i], j)
			rowVal[i] = append(rowVal[i], s.val[j][k])
		}
	}
	// resRow[j] is row j of the product, in first-touch order.
	resIdx := make([][]int, m)
	resVal := make([][]float64, m)
	mask := make([]bool, m)
	values := make([]float64, m)
	for j := range m {
		var indices []int
		for kk, k := range rowIdx[j] {
			y := rowVal[j][kk]
			for ii, i := range s.idx[k] {
				x := s.val[k][ii]
				if !mask[i] {
					mask[i] = true
					values[i] = x * y
					indices = append(indices, i)
				} else {
					values[i] += x * y
				}
			}
		}
		for _, i := range indices {
			resIdx[j] = append(resIdx[j], i)
			resVal[j] = append(resVal[j], values[i])
			mask[i] = false
		}
	}
	t := &spMat{rows: m, cols: m, idx: make([][]int, m), val: make([][]float64, m)}
	for j := range m {
		for k, i := range resIdx[j] {
			t.idx[i] = append(t.idx[i], j)
			t.val[i] = append(t.val[i], resVal[j][k])
		}
	}
	return t
}

// sparseQR is Eigen's SparseQR with COLAMDOrdering (the Eigen 3.4 that
// C++ SolveSpace vendors in extlib/eigen), the rank-revealing
// QR C++ System uses: A P = Q R. The columns are ordered by COLAMD, then
// factored left to right (left-looking, following the column
// elimination tree); a column whose Householder pivot |beta| falls
// below the threshold is dead: it gets no reflection, and the
// permutation moves it after the live ones.
type sparseQR struct {
	m, n int
	// q is the Householder vectors, r the triangular factor, both in
	// Eigen's storage order.
	q, r    *spMat
	hcoeffs []float64
	// perm[k] is the column of A at column k of R (Eigen's
	// colsPermutation().indices()).
	perm []int
	rank int
}

// coletree is Eigen's internal::coletree: the column elimination tree
// of the matrix whose column k is cols(perm[k]), and each row's first
// column.
func coletree(m, nc int, cols func(int) []int, perm []int) (parent, firstRowElt []int) {
	diagSize := min(nc, m)
	root := make([]int, nc)
	pp := make([]int, nc)
	parent = make([]int, nc)
	firstRowElt = make([]int, m)
	for i := range firstRowElt {
		firstRowElt[i] = nc
	}
	for i := range diagSize {
		firstRowElt[i] = i
	}
	for col := range nc {
		for _, row := range cols(perm[col]) {
			firstRowElt[row] = min(firstRowElt[row], col)
		}
	}
	for col := range nc {
		foundDiag := col >= m
		pp[col] = col
		cset := col
		root[cset] = col
		parent[col] = nc
		rows := cols(perm[col])
		for k := 0; k < len(rows) || !foundDiag; k++ {
			i := col
			if k < len(rows) {
				i = rows[k]
			}
			if i == col {
				foundDiag = true
			}
			row := firstRowElt[i]
			if row >= col {
				continue
			}
			rset := etreeFind(row, pp)
			rroot := root[rset]
			if rroot != col {
				parent[rroot] = col
				pp[cset] = rset
				cset = rset
				root[cset] = col
			}
		}
	}
	return parent, firstRowElt
}

// etreeFind finds the root of the set containing i, with path halving.
func etreeFind(i int, pp []int) int {
	p := pp[i]
	gp := pp[p]
	for gp != p {
		pp[i] = gp
		i = gp
		p = pp[i]
		gp = pp[p]
	}
	return p
}

// epsilon is the machine epsilon of float64 (Eigen NumTraits::epsilon).
var epsilon = math.Nextafter(1, 2) - 1

// factorSparseQR is SparseQR::compute: analyzePattern, then factorize.
func factorSparseQR(a *spMat) *sparseQR {
	m, n := a.rows, a.cols
	diagSize := min(m, n)

	// analyzePattern.
	permC := colamdOrdering(m, n, a.idx)
	outputPerm := make([]int, n) // m_perm_c.inverse()
	for i, p := range permC {
		outputPerm[p] = i
	}
	cols := func(j int) []int { return a.idx[j] }
	etree, firstRowElt := coletree(m, n, cols, outputPerm)

	// factorize. Column permC[i] of pmat is column i of a.
	pmatIdx := make([][]int, n)
	pmatVal := make([][]float64, n)
	for i := range n {
		pmatIdx[permC[i]] = a.idx[i]
		pmatVal[permC[i]] = a.val[i]
	}
	pcols := func(j int) []int { return pmatIdx[j] }

	max2Norm := 0.0
	for j := range n {
		sum := 0.0
		for _, v := range pmatVal[j] {
			sum += v * v
		}
		max2Norm = maxi(max2Norm, math.Sqrt(sum))
	}
	if max2Norm == 0 {
		max2Norm = 1
	}
	pivotThreshold := float64(20*(m+n)) * max2Norm * epsilon

	pivotperm := make([]int, n)
	for i := range pivotperm {
		pivotperm[i] = i
	}

	f := &sparseQR{m: m, n: n, hcoeffs: make([]float64, diagSize)}
	q := &spMat{rows: m, cols: diagSize, idx: make([][]int, diagSize), val: make([][]float64, diagSize)}
	r := &spMat{rows: m, cols: n, idx: make([][]int, n), val: make([][]float64, n)}
	mark := make([]int, max(m, n))
	ridx := make([]int, n)
	qidx := make([]int, m)
	tval := make([]float64, m)

	nonzeroCol := 0
	for col := range n {
		for i := range mark {
			mark[i] = -1
		}
		mark[nonzeroCol] = col
		qidx[0] = nonzeroCol
		nzcolR, nzcolQ := 0, 1
		foundDiag := nonzeroCol >= m
		for i := range tval {
			tval[i] = 0
		}

		// Symbolic factorization: the nonzero pattern of column col of
		// R and Q, through the column elimination tree.
		pidx, pval := pmatIdx[col], pmatVal[col]
		for itp := 0; itp < len(pidx) || !foundDiag; itp++ {
			curIdx := nonzeroCol
			if itp < len(pidx) {
				curIdx = pidx[itp]
			}
			if curIdx == nonzeroCol {
				foundDiag = true
			}
			st := firstRowElt[curIdx]
			if st < 0 {
				panic("solver: empty row found during numerical factorization")
			}
			bi := nzcolR
			for ; mark[st] != col; st = etree[st] {
				ridx[nzcolR] = st
				mark[st] = col
				nzcolR++
			}
			nt := nzcolR - bi
			for i := range nt / 2 {
				ridx[bi+i], ridx[nzcolR-i-1] = ridx[nzcolR-i-1], ridx[bi+i]
			}
			if itp < len(pidx) {
				tval[curIdx] = pval[itp]
			} else {
				tval[curIdx] = 0
			}
			if curIdx > nonzeroCol && mark[curIdx] != col {
				qidx[nzcolQ] = curIdx
				mark[curIdx] = col
				nzcolQ++
			}
		}

		// Apply the Householder reflections found so far, in reverse
		// order of the pattern of R.
		for i := nzcolR - 1; i >= 0; i-- {
			curIdx := ridx[i]
			var tdot float64
			if curIdx < nonzeroCol {
				tdot = q.colDot(curIdx, tval)
				tdot *= f.hcoeffs[curIdx]
				for k, iq := range q.idx[curIdx] {
					tval[iq] -= tdot * q.val[curIdx][k]
				}
			}
			// Detect fill-in for the current column of Q.
			if etree[ridx[i]] == nonzeroCol && curIdx < nonzeroCol {
				for _, iQ := range q.idx[curIdx] {
					if mark[iQ] != col {
						qidx[nzcolQ] = iQ
						nzcolQ++
						mark[iQ] = col
					}
				}
			}
		}

		tau, beta := 0.0, 0.0
		if nonzeroCol < diagSize {
			c0 := 0.0
			if nzcolQ > 0 {
				c0 = tval[qidx[0]]
			}
			sqrNorm := 0.0
			for itq := 1; itq < nzcolQ; itq++ {
				sqrNorm += tval[qidx[itq]] * tval[qidx[itq]]
			}
			if sqrNorm == 0 {
				beta = c0
				tval[qidx[0]] = 1
			} else {
				beta = math.Sqrt(c0*c0 + sqrNorm)
				if c0 >= 0 {
					beta = -beta
				}
				tval[qidx[0]] = 1
				for itq := 1; itq < nzcolQ; itq++ {
					tval[qidx[itq]] /= c0 - beta
				}
				tau = (beta - c0) / beta
			}
		}

		// Insert values in R.
		for i := nzcolR - 1; i >= 0; i-- {
			curIdx := ridx[i]
			if curIdx < nonzeroCol {
				r.idx[col] = append(r.idx[col], curIdx)
				r.val[col] = append(r.val[col], tval[curIdx])
				tval[curIdx] = 0
			}
		}

		if nonzeroCol < diagSize && math.Abs(beta) >= pivotThreshold {
			r.idx[col] = append(r.idx[col], nonzeroCol)
			r.val[col] = append(r.val[col], beta)
			f.hcoeffs[nonzeroCol] = tau
			for itq := range nzcolQ {
				iQ := qidx[itq]
				q.idx[nonzeroCol] = append(q.idx[nonzeroCol], iQ)
				q.val[nonzeroCol] = append(q.val[nonzeroCol], tval[iQ])
				tval[iQ] = 0
			}
			nonzeroCol++
		} else {
			// Zero pivot found: move this column to the end.
			for j := nonzeroCol; j < n-1; j++ {
				pivotperm[j], pivotperm[j+1] = pivotperm[j+1], pivotperm[j]
			}
			// Recompute the column elimination tree.
			etree, firstRowElt = coletree(m, n, pcols, pivotperm)
		}
	}
	for k := nonzeroCol; k < diagSize; k++ {
		f.hcoeffs[k] = 0
	}

	f.rank = nonzeroCol
	f.perm = outputPerm
	if nonzeroCol < n {
		// Put the dead columns of R at the end, and update the
		// permutation. The permuted R is built with
		// SparseMatrix::insert, which sorts each column.
		pr := &spMat{rows: m, cols: n, idx: make([][]int, n), val: make([][]float64, n)}
		perm := make([]int, n)
		for j := range n {
			idx, val := r.idx[pivotperm[j]], r.val[pivotperm[j]]
			for k := range idx {
				p := len(pr.idx[j])
				pr.idx[j] = append(pr.idx[j], 0)
				pr.val[j] = append(pr.val[j], 0)
				for p > 0 && pr.idx[j][p-1] > idx[k] {
					pr.idx[j][p] = pr.idx[j][p-1]
					pr.val[j][p] = pr.val[j][p-1]
					p--
				}
				pr.idx[j][p] = idx[k]
				pr.val[j][p] = val[k]
			}
			perm[j] = outputPerm[pivotperm[j]]
		}
		r, f.perm = pr, perm
	}
	f.q, f.r = q, r
	return f
}

// maxi is Eigen's numext::maxi, std::max.
func maxi(a, b float64) float64 {
	if a < b {
		return b
	}
	return a
}

// applyQt overwrites b with Q' b (SparseQR_QProduct, transposed).
func (f *sparseQR) applyQt(b []float64) {
	for k := range min(f.m, f.n) {
		tau := f.q.colDot(k, b)
		if tau == 0 {
			continue
		}
		tau *= f.hcoeffs[k]
		for i, iq := range f.q.idx[k] {
			b[iq] -= tau * f.q.val[k][i]
		}
	}
}

// solveUpper solves R11 x = b in place, R11 the leading rank x rank
// block of R, as Eigen's column-major upper triangular sparse solve
// does.
func (f *sparseQR) solveUpper(b []float64) {
	for i := f.rank - 1; i >= 0; i-- {
		if b[i] == 0 {
			continue
		}
		idx, val := f.r.idx[i], f.r.val[i]
		k := 0
		for idx[k] != i {
			k++
		}
		b[i] /= val[k]
		tmp := b[i]
		for k := 0; k < len(idx) && idx[k] < i; k++ {
			b[idx[k]] = -tmp*val[k] + b[idx[k]]
		}
	}
}

// solveUpperTransposed solves R11' x = b in place, as Eigen's row-major
// lower triangular sparse solve does with the transposed R.
func (f *sparseQR) solveUpperTransposed(b []float64) {
	for i := range f.rank {
		tmp := b[i]
		lastVal := 0.0
		idx, val := f.r.idx[i], f.r.val[i]
		for k := 0; k < len(idx) && idx[k] < f.rank; k++ {
			lastVal = val[k]
			if idx[k] == i {
				break
			}
			tmp = -lastVal*b[idx[k]] + tmp
		}
		b[i] = tmp / lastVal
	}
}

// solve returns the basic least-squares solution of A x = b: the
// components on the dead columns are zero (SparseQR::solve).
func (f *sparseQR) solve(b []float64) []float64 {
	y := make([]float64, max(f.m, f.n))
	copy(y, b)
	f.applyQt(y[:f.m])
	f.solveUpper(y)
	for i := f.rank; i < len(y); i++ {
		y[i] = 0
	}
	x := make([]float64, f.n)
	for k := range f.n {
		x[f.perm[k]] = y[k]
	}
	return x
}

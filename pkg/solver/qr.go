package solver

import "math"

// qr is a rank-revealing Householder QR, A P = Q R, computed the way
// Eigen's SparseQR (which C++ System uses) does it: columns are taken
// left to right, and a column whose Householder pivot |beta| falls
// below the threshold is dead: it gets no reflection, and the
// permutation moves it after the live ones. The rank is the number of
// live columns.
//
// Eigen first reorders the columns with COLAMD to reduce fill-in; this
// dense version keeps them in their natural order. That leaves the
// minimum-norm Newton step of a full-rank system unchanged, but a
// rank-deficient system can have a different set of dead columns.
type qr struct {
	m, n int
	// v[k] is the k-th Householder vector (v[k][k] == 1, zero above).
	v   [][]float64
	tau []float64
	// r[k] is column perm[k] of R: its first min(k+1, rank) entries.
	r    [][]float64
	perm []int
	rank int
}

// qrThreshold is Eigen SparseQR's default pivot threshold,
// 20 (m + n) max|A_j| eps, after SuiteSparseQR.
func qrThreshold(a [][]float64, m, n int) float64 {
	max2Norm := 0.0
	for j := 0; j < n; j++ {
		s := 0.0
		for i := 0; i < m; i++ {
			s += a[i][j] * a[i][j]
		}
		max2Norm = math.Max(max2Norm, math.Sqrt(s))
	}
	if max2Norm == 0 {
		max2Norm = 1
	}
	return 20 * float64(m+n) * max2Norm * epsilon
}

// epsilon is the machine epsilon of float64 (Eigen NumTraits::epsilon).
var epsilon = math.Nextafter(1, 2) - 1

// factorQR factors the m x n matrix a (a[i][j], row i, column j).
func factorQR(a [][]float64, m, n int) *qr {
	f := &qr{m: m, n: n}
	threshold := qrThreshold(a, m, n)
	diagSize := min(m, n)

	var live, dead []int
	var liveR, deadR [][]float64
	for col := 0; col < n; col++ {
		tval := make([]float64, m)
		for i := 0; i < m; i++ {
			tval[i] = a[i][col]
		}
		// Apply the Householder reflections found so far.
		for k := 0; k < f.rank; k++ {
			tdot := 0.0
			for i := k; i < m; i++ {
				tdot += f.v[k][i] * tval[i]
			}
			tdot *= f.tau[k]
			for i := k; i < m; i++ {
				tval[i] -= f.v[k][i] * tdot
			}
		}

		k := f.rank
		tau, beta := 0.0, 0.0
		var hv []float64
		if k < diagSize {
			// Compute the Householder reflection that eliminates the
			// current column below row k.
			c0 := tval[k]
			sqrNorm := 0.0
			for i := k + 1; i < m; i++ {
				sqrNorm += tval[i] * tval[i]
			}
			hv = make([]float64, m)
			hv[k] = 1
			if sqrNorm == 0 {
				beta = c0
			} else {
				beta = math.Sqrt(c0*c0 + sqrNorm)
				if c0 >= 0 {
					beta = -beta
				}
				for i := k + 1; i < m; i++ {
					hv[i] = tval[i] / (c0 - beta)
				}
				tau = (beta - c0) / beta
			}
		}

		rcol := make([]float64, k, k+1)
		copy(rcol, tval[:k])
		if k < diagSize && math.Abs(beta) >= threshold {
			f.v = append(f.v, hv)
			f.tau = append(f.tau, tau)
			f.rank++
			live = append(live, col)
			liveR = append(liveR, append(rcol, beta))
		} else {
			// Zero pivot found: move this column to the end.
			dead = append(dead, col)
			deadR = append(deadR, rcol)
		}
	}
	f.perm = append(live, dead...)
	f.r = append(liveR, deadR...)
	return f
}

// applyQt overwrites b with Q' b.
func (f *qr) applyQt(b []float64) {
	for k := 0; k < f.rank; k++ {
		tdot := 0.0
		for i := k; i < f.m; i++ {
			tdot += f.v[k][i] * b[i]
		}
		tdot *= f.tau[k]
		for i := k; i < f.m; i++ {
			b[i] -= f.v[k][i] * tdot
		}
	}
}

// solveR solves R11 x = b in place, R11 the leading rank x rank block.
func (f *qr) solveR(b []float64) {
	for i := f.rank - 1; i >= 0; i-- {
		s := b[i]
		for j := i + 1; j < f.rank; j++ {
			s -= f.r[j][i] * b[j]
		}
		b[i] = s / f.r[i][i]
	}
}

// solveRt solves R11' x = b in place.
func (f *qr) solveRt(b []float64) {
	for i := 0; i < f.rank; i++ {
		s := b[i]
		for j := 0; j < i; j++ {
			s -= f.r[i][j] * b[j]
		}
		b[i] = s / f.r[i][i]
	}
}

// solve returns the basic least-squares solution of A x = b: the
// components on the dead columns are zero (Eigen SparseQR::solve).
func (f *qr) solve(b []float64) []float64 {
	y := make([]float64, max(f.m, f.n))
	copy(y, b)
	f.applyQt(y[:f.m])
	f.solveR(y)
	for i := f.rank; i < len(y); i++ {
		y[i] = 0
	}
	x := make([]float64, f.n)
	for k := 0; k < f.n; k++ {
		x[f.perm[k]] = y[k]
	}
	return x
}

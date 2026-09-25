package solver

import (
	"fmt"

	"gonum.org/v1/gonum/mat"
)

// gonumLeastSquares solves the overdetermined or underdetermined
// least-squares system J*delta = -r. The Go port uses the normal
// equations approach because SolveSpace's Newton-step Jacobian is
// always dense and small (<= a few hundred rows).
//
// The transformation is: form A = J^T J + tol*I, b' = -J^T r, solve
// A * delta = b' via Cholesky. tol is the regularization parameter
// (also used as the rank-detection threshold).
//
// We use Cholesky rather than the original code's Gauss elimination
// because Cholesky is numerically superior for symmetric
// positive-(semi)definite systems and is what Eigen would use here
// for the normal-equation formulation.
func gonumLeastSquares(jacobian [][]float64, residuals []float64, tol float64) ([]float64, int, error) {
	m := len(jacobian)
	if m == 0 {
		return nil, 0, nil
	}
	if len(residuals) != m {
		return nil, 0, fmt.Errorf("solver: mismatch between Jacobian rows and residuals")
	}
	jCols := len(jacobian[0])
	if jCols == 0 {
		return []float64{}, 0, nil
	}

	jData := make([]float64, 0, m*jCols)
	for i := 0; i < m; i++ {
		if len(jacobian[i]) != jCols {
			return nil, 0, fmt.Errorf("solver: Jacobian row %d has %d cols, expected %d", i, len(jacobian[i]), jCols)
		}
		jData = append(jData, jacobian[i]...)
	}
	J := mat.NewDense(m, jCols, jData)

	jt := mat.NewDense(jCols, m, nil)
	jt.CloneFrom(J.T())

	// rhsVec = -residuals
	rhsData := make([]float64, m)
	for i, r := range residuals {
		rhsData[i] = -r
	}
	rhsVec := mat.NewVecDense(m, rhsData)

	// jtR = J^T * (-r)
	var jtR mat.VecDense
	jtR.MulVec(jt, rhsVec)

	// jtjSym = J^T J with regularization on the diagonal.
	jtjData := make([]float64, jCols*jCols)
	for i := 0; i < jCols; i++ {
		for j := 0; j < jCols; j++ {
			sum := 0.0
			for k := 0; k < m; k++ {
				sum += jData[k*jCols+i] * jData[k*jCols+j]
			}
			jtjData[i*jCols+j] = sum
		}
		jtjData[i*jCols+i] += tol
	}
	jtjSym := mat.NewSymDense(jCols, jtjData)

	var chol mat.Cholesky
	if !chol.Factorize(jtjSym) {
		// Regularization was insufficient; retry with a larger tol.
		for i := 0; i < jCols; i++ {
			jtjData[i*jCols+i] += tol * 1e3
		}
		jtjSym = mat.NewSymDense(jCols, jtjData)
		if !chol.Factorize(jtjSym) {
			return nil, 0, fmt.Errorf("solver: normal equations not positive definite even with regularization")
		}
	}

	x := mat.NewVecDense(jCols, nil)
	if err := chol.SolveVecTo(x, &jtR); err != nil {
		return nil, 0, fmt.Errorf("solver: Cholesky solve: %w", err)
	}

	// Rank is the number of pivots above tol in the original J. We
	// estimate this as the number of eigenvalues of J^T J above tol^2
	// which equals the trace-rank of the Cholesky factor's diagonal.
	rank := jCols
	u := mat.NewTriDense(jCols, mat.Upper, nil)
	chol.UTo(u)
	for i := 0; i < jCols; i++ {
		if u.At(i, i) < tol {
			rank = i
			break
		}
	}

	out := make([]float64, jCols)
	for i := 0; i < jCols; i++ {
		out[i] = x.AtVec(i)
	}
	return out, rank, nil
}

// gonumSymmetricSolve solves the square symmetric positive-definite
// system A x = b. The caller is expected to have formed A as a
// normal-equation matrix. We add a tiny regularization tol to the
// diagonal for stability, then solve via Cholesky factorization.
func gonumSymmetricSolve(a [][]float64, b []float64, tol float64) ([]float64, error) {
	n := len(a)
	if n == 0 {
		return nil, nil
	}
	data := make([]float64, n*n)
	for i := 0; i < n; i++ {
		if len(a[i]) != n {
			return nil, fmt.Errorf("solver: symmetric system not square at row %d (got %d)", i, len(a[i]))
		}
		for j := 0; j < n; j++ {
			data[i*n+j] = a[i][j]
		}
		data[i*n+i] += tol
	}
	A := mat.NewSymDense(n, data)
	var chol mat.Cholesky
	if chol.Factorize(A) {
		x := mat.NewVecDense(n, nil)
		bVec := mat.NewVecDense(n, b)
		if err := chol.SolveVecTo(x, bVec); err != nil {
			return nil, fmt.Errorf("solver: Cholesky solve: %w", err)
		}
		return x.RawVector().Data, nil
	}
	// Fall back to LU with partial pivoting (slower but always works
	// for non-positive-definite A).
	dense := mat.NewDense(n, n, data)
	var lu mat.LU
	lu.Factorize(dense)
	x := mat.NewVecDense(n, nil)
	bVec := mat.NewVecDense(n, b)
	if err := lu.SolveVecTo(x, false, bVec); err != nil {
		return nil, fmt.Errorf("solver: LU solve: %w", err)
	}
	return x.RawVector().Data, nil
}

package geom

// BandedMatrix solves a banded linear system with two extra dense
// right-hand columns (for periodic splines), without pivoting.
type BandedMatrix struct {
	A [BandedMaxUnknowns][BandedMaxUnknowns]float64
	B [BandedMaxUnknowns]float64
	X [BandedMaxUnknowns]float64
	N int
}

const (
	BandedMaxUnknowns = 16
	bandedRightOfDiag = 1
	bandedLeftOfDiag  = 2
)

// Solve solves the system for its right-hand sides.
func (m *BandedMatrix) Solve() {
	n := m.N
	// Reduce the matrix to upper triangular form.
	for i := range n {
		for ip := i + 1; ip < n && ip <= i+bandedLeftOfDiag; ip++ {
			temp := m.A[ip][i] / m.A[i][i]

			for jp := i; jp < n-2 && jp <= i+bandedRightOfDiag; jp++ {
				m.A[ip][jp] -= temp * m.A[i][jp]
			}
			m.A[ip][n-2] -= temp * m.A[i][n-2]
			m.A[ip][n-1] -= temp * m.A[i][n-1]

			m.B[ip] -= temp * m.B[i]
		}
	}

	// And back-substitute.
	for i := n - 1; i >= 0; i-- {
		temp := m.B[i]

		if i < n-1 {
			temp -= m.X[n-1] * m.A[i][n-1]
		}
		if i < n-2 {
			temp -= m.X[n-2] * m.A[i][n-2]
		}

		for j := min(n-3, i+bandedRightOfDiag); j > i; j-- {
			temp -= m.X[j] * m.A[i][j]
		}
		m.X[i] = temp / m.A[i][i]
	}
}

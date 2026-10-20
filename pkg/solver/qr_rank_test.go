package solver

import (
	"math/rand"
	"testing"
)

// randomSparse is an m x n matrix of density about d, with some
// columns copied or scaled from others, some zero, and some rows
// repeated, so that its rank falls short in the ways a Jacobian's can.
func randomSparse(rng *rand.Rand, m, n int, d float64) *spMat {
	a := make([][]float64, m)
	for i := range a {
		a[i] = make([]float64, n)
		for j := range n {
			if rng.Float64() < d {
				a[i][j] = float64(rng.Intn(19) - 9)
			}
		}
	}
	for range n / 4 {
		src, dst := rng.Intn(n), rng.Intn(n)
		scale := float64(rng.Intn(5) - 2)
		for i := range m {
			a[i][dst] = scale * a[i][src]
		}
	}
	for range m / 4 {
		src, dst := rng.Intn(m), rng.Intn(m)
		copy(a[dst], a[src])
	}
	return spFromRows(a, m, n)
}

// TestSparseQRRankMatchesFactor holds the rank-only factorisation to
// the rank the full one finds, on the shapes the solver factors: wide,
// as the rank test of a sketch with freedom left, tall and square.
func TestSparseQRRankMatchesFactor(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(7))
	for range 400 {
		m, n := 1+rng.Intn(24), 1+rng.Intn(24)
		if rng.Intn(3) == 0 {
			n += 20 // wide, with most columns dead
		}
		a := randomSparse(rng, m, n, 0.1+0.3*rng.Float64())
		want := factorSparseQR(a).rank
		if got := sparseQRRank(a, analyzeSparseQR(a)); got != want {
			t.Fatalf("%dx%d: rank only %d, factor %d", m, n, got, want)
		}
	}
}

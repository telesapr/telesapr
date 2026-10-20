package solver

import (
	"math/rand"
	"slices"
	"testing"
)

// sameFactors reports whether two factorisations are the same in every
// value and index.
func sameFactors(a, b *sparseQR) bool {
	if a.rank != b.rank || !slices.Equal(a.perm, b.perm) || !slices.Equal(a.hcoeffs, b.hcoeffs) {
		return false
	}
	same := func(x, y *spMat) bool {
		if x.rows != y.rows || x.cols != y.cols {
			return false
		}
		for j := range x.cols {
			if !slices.Equal(x.idx[j], y.idx[j]) || !slices.Equal(x.val[j], y.val[j]) {
				return false
			}
		}
		return true
	}
	return same(a.q, b.q) && same(a.r, b.r)
}

// TestSymbolicCacheSameFactors holds a factorisation through the cache,
// with the analysis of an earlier matrix of the same pattern, to the
// factorisation analysed afresh; and a matrix of another pattern to a
// new analysis.
func TestSymbolicCacheSameFactors(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(11))
	for range 200 {
		m, n := 1+rng.Intn(24), 1+rng.Intn(24)
		a := randomSparse(rng, m, n, 0.1+0.3*rng.Float64())
		var cache symbolicCache
		first := cache.analysis(a)
		// The same pattern with other values: the Newton step's next
		// iteration.
		b := &spMat{rows: m, cols: n, idx: a.idx, val: make([][]float64, n)}
		for j := range n {
			b.val[j] = make([]float64, len(a.val[j]))
			for k := range b.val[j] {
				b.val[j][k] = float64(rng.Intn(19) - 9)
				if b.val[j][k] == 0 {
					b.val[j][k] = 1
				}
			}
		}
		if cache.analysis(b) != first {
			t.Fatalf("%dx%d: the same pattern was analysed again", m, n)
		}
		if !sameFactors(factorSparseQRAnalyzed(b, cache.analysis(b), false), factorSparseQR(b)) {
			t.Fatalf("%dx%d: factors differ through the cache", m, n)
		}
		if got, want := sparseQRRank(b, cache.analysis(b)), factorSparseQR(b).rank; got != want {
			t.Fatalf("%dx%d: rank %d through the cache, %d afresh", m, n, got, want)
		}
		// Another pattern: a new analysis.
		c := randomSparse(rng, m, n, 0.1+0.3*rng.Float64())
		if (cache.analysis(c) == first) != first.matches(c) {
			t.Fatalf("%dx%d: analysis kept for another pattern", m, n)
		}
	}
}

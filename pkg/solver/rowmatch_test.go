package solver

import (
	"math"
	"math/rand"
	"slices"
	"testing"
)

// TestMatchRows holds matchRows to a permutation that puts, at each
// place it can, a row with an entry in the column ordered there.
func TestMatchRows(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(11))
	for range 300 {
		m, n := 1+rng.Intn(30), 1+rng.Intn(30)
		a := randomSparse(rng, m, n, 0.05+0.3*rng.Float64())
		permC := colamdOrdering(a.rows, a.cols, a.idx)
		newRow := matchRows(a, permC)
		seen := slices.Clone(newRow)
		slices.Sort(seen)
		for i, r := range seen {
			if r != i {
				t.Fatalf("%dx%d: %v is not a permutation", m, n, newRow)
			}
		}
		p := permuteRows(a, newRow)
		for j := range a.cols {
			if !slices.IsSorted(p.idx[j]) {
				t.Fatalf("%dx%d: column %d unsorted: %v", m, n, j, p.idx[j])
			}
		}
	}
	// Rows shuffled out of a diagonal come back to it, whatever the
	// column order.
	m := 40
	shuffled := make([][]float64, m)
	for i, r := range rng.Perm(m) {
		shuffled[r] = make([]float64, m)
		shuffled[r][i] = float64(1 + i)
		if i+1 < m {
			shuffled[r][i+1] = 1
		}
	}
	a := spFromRows(shuffled, m, m)
	permC := colamdOrdering(a.rows, a.cols, a.idx)
	p := permuteRows(a, matchRows(a, permC))
	for k, j := range permC {
		if !slices.Contains(p.idx[k], j) {
			t.Fatalf("column %d, placed %d, has no entry in row %d: %v", k, j, j, p.idx[k])
		}
	}
}

// TestRankMatchedSame holds the rank test, which factors the matrix
// transposed where it is wide and with its rows matched, to the rank
// of the matrix factored as it is, on the shapes the solver meets.
func TestRankMatchedSame(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(13))
	for range 400 {
		m, n := 1+rng.Intn(24), 1+rng.Intn(24)
		if rng.Intn(3) == 0 {
			n += 20
		}
		a := randomSparse(rng, m, n, 0.1+0.3*rng.Float64())
		var s System
		if got, want := s.rankOf(a), factorSparseQR(a).rank; got != want {
			t.Fatalf("%dx%d: matched %d, as it is %d", m, n, got, want)
		}
		// The second time through the kept permutation and analysis.
		if got, want := s.rankOf(a), factorSparseQR(a).rank; got != want {
			t.Fatalf("%dx%d kept: matched %d, as it is %d", m, n, got, want)
		}
	}
}

// TestLeastSquaresMatchedSame holds the minimum-norm step with the
// rows of A' matched to the one factored as it is, to rounding: some
// of the cases are near singular, so rounding is relative to the
// largest of the step.
func TestLeastSquaresMatchedSame(t *testing.T) {
	t.Parallel()
	for _, c := range genLeastSquaresCases() {
		if c.a.rows > c.a.cols {
			continue
		}
		at := c.a.transpose()
		plain := (&stepFactor{a: c.a, f: factorSparseQR(at), wide: true}).solve(c.b)
		matched := leastSquares(c.a, c.b, &matchedCache{})
		size := 1.0
		for _, x := range plain {
			size = max(size, math.Abs(x))
		}
		if size > 1e6 {
			continue // a near-singular case, its step all rounding
		}
		for i := range plain {
			if math.Abs(plain[i]-matched[i]) > 1e-7*size {
				t.Fatalf("%dx%d: x[%d] %g as it is, %g matched", c.a.rows, c.a.cols, i, plain[i], matched[i])
			}
		}
	}
}

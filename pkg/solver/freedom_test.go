package solver

import (
	"math/rand"
	"testing"
)

// rankOf is the rank of a, m rows of n, by the rank test's QR.
func rankOf(a [][]float64, m, n int) int {
	if m == 0 || n == 0 {
		return 0
	}
	return factorSparseQR(spFromRows(a, m, n)).rank
}

// rrefFixedColumns and fixedColumns agree with the definition MarkParamsFree tests one
// column at a time: column j is fixed when removing it loses rank.
// The matrices are built as sketches make them: sparse rows, rows that
// fix one unknown, equalities between two, and rows that repeat a
// combination of others.
func TestRrefFixedColumnsVsRank(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := range 20000 {
		n := 1 + rng.Intn(12)
		m := rng.Intn(n + 3)
		a := make([][]float64, m)
		for i := range a {
			a[i] = make([]float64, n)
			switch k := rng.Intn(5); {
			case k == 0:
				a[i][rng.Intn(n)] = rng.NormFloat64()
			case k == 1 && n > 1:
				x, y := rng.Intn(n), rng.Intn(n)
				a[i][x]++
				a[i][y]--
			case k == 2 && i >= 2:
				for j := range a[i] {
					a[i][j] = 2*a[i-1][j] - 0.5*a[i-2][j]
				}
			default:
				for j := range a[i] {
					if rng.Intn(3) == 0 {
						a[i][j] = float64(rng.Intn(7) - 3)
					}
				}
			}
		}
		full := rankOf(a, m, n)
		cp := make([][]float64, m)
		for i := range a {
			cp[i] = append([]float64(nil), a[i]...)
		}
		got := rrefFixedColumns(cp, m, n)
		for i := range a {
			cp[i] = append(cp[i][:0], a[i]...)
		}
		fast := fixedColumns(cp, m, n)
		for j := range n {
			without := make([][]float64, m)
			for i := range a {
				without[i] = append(append([]float64(nil), a[i][:j]...), a[i][j+1:]...)
			}
			// Removing a column loses rank exactly when that column's
			// unit vector is in the row space.
			want := rankOf(without, m, n-1) < full
			if got[j] != want || fast[j] != want {
				t.Fatalf("trial %d: column %d of %v: fixed %v (dense) %v (reduced first), want %v",
					trial, j, a, got[j], fast[j], want)
			}
		}
	}
}

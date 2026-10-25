package solver

import (
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
)

// sparseQRDir holds what SparseQR gives for genLeastSquaresCases
// (solutions.txt): the rank it finds for A, then X as
// System::SolveLeastSquares computes it (A unscaled).
var sparseQRDir = filepath.Join("..", "..", "testdata", "solver", "sparseqr")

type lsCase struct {
	a *spMat
	b []float64
}

// genLeastSquaresCases makes Jacobian-like systems: sparse, with
// integer and real entries of mixed magnitudes, some with zero rows,
// repeated rows, or rows that combine others, and some with more
// equations than unknowns.
func genLeastSquaresCases() []lsCase {
	rnd := rand.New(rand.NewSource(1))
	var cases []lsCase
	for c := range 500 {
		m := 1 + rnd.Intn(24)
		n := 1 + rnd.Intn(24)
		density := 0.1 + 0.4*rnd.Float64()
		if c >= 400 {
			// Large and sparse, like a big sketch's Jacobian.
			m = 40 + rnd.Intn(120)
			n = 40 + rnd.Intn(120)
			density = (1 + 4*rnd.Float64()) / float64(n)
		}
		if c%4 != 0 && m > n {
			m, n = n, m
		}
		a := make([][]float64, m)
		for i := range a {
			a[i] = make([]float64, n)
			for j := range a[i] {
				if rnd.Float64() >= density {
					continue
				}
				switch rnd.Intn(4) {
				case 0:
					a[i][j] = float64(rnd.Intn(5) - 2)
				case 1:
					a[i][j] = rnd.NormFloat64()
				case 2:
					a[i][j] = rnd.NormFloat64() * math.Pow(10, float64(rnd.Intn(7)-3))
				default:
					a[i][j] = float64(rnd.Intn(200)-100) / 20
				}
			}
		}
		for i := range a {
			switch rnd.Intn(12) {
			case 0:
				// A repeated row.
				copy(a[i], a[rnd.Intn(m)])
			case 1:
				// A combination of two rows.
				p, q := a[rnd.Intn(m)], a[rnd.Intn(m)]
				for j := range a[i] {
					a[i][j] = p[j] - 3*q[j]
				}
			case 2:
				// No entries at all.
				for j := range a[i] {
					a[i][j] = 0
				}
			}
		}
		b := make([]float64, m)
		for i := range b {
			b[i] = rnd.NormFloat64() * math.Pow(10, float64(rnd.Intn(5)-2))
		}
		cases = append(cases, lsCase{spFromRows(a, m, n), b})
	}
	return cases
}

// TestSparseQRGolden requires, bit for bit, the rank SparseQR finds for
// each of genLeastSquaresCases and the least-squares solution
// (solutions.txt): the solver's results, and so the goldens of every
// solved sketch, move with any change to its arithmetic. With
// TELESAPR_REGEN_GOLDEN=1 it rewrites solutions.txt.
func TestSparseQRGolden(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	for _, c := range genLeastSquaresCases() {
		fmt.Fprintf(&sb, "rank %d\n", factorSparseQR(c.a).rank)
		for _, x := range leastSquares(c.a, c.b, &matchedCache{}) {
			sb.WriteString(hexFloat(x) + "\n")
		}
	}
	golden.Check(t, filepath.Join(sparseQRDir, "solutions.txt"), []byte(sb.String()))
}

// hexFloat formats x as C's %a does, with no padding in the exponent,
// which is how the golden was first written.
func hexFloat(x float64) string {
	s := strconv.FormatFloat(x, 'x', -1, 64)
	i := strings.LastIndexAny(s, "+-")
	if exp := strings.TrimLeft(s[i+1:], "0"); exp != "" {
		return s[:i+1] + exp
	}
	return s[:i+1] + "0"
}

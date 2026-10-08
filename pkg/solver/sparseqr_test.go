package solver

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// sparseQRDir holds the generated least-squares cases (cases.txt) and
// what a SolveSpace harness, built against SolveSpace's Eigen with its flags,
// printed for them (eigen.txt): the rank SparseQR finds for A, then X
// as System::SolveLeastSquares computes it (A unscaled).
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

func writeLeastSquaresCases(path string, cases []lsCase) error {
	var sb strings.Builder
	for _, c := range cases {
		nnz := 0
		for j := range c.a.idx {
			nnz += len(c.a.idx[j])
		}
		fmt.Fprintf(&sb, "%d %d %d\n", c.a.rows, c.a.cols, nnz)
		for j := range c.a.idx {
			for k, i := range c.a.idx[j] {
				fmt.Fprintf(&sb, "%d %d %x\n", i, j, c.a.val[j][k])
			}
		}
		for _, v := range c.b {
			fmt.Fprintf(&sb, "%x\n", v)
		}
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func readLeastSquaresCases(t *testing.T, path string) []lsCase {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Fields(string(data))
	pos := 0
	next := func() string { pos++; return f[pos-1] }
	nextInt := func() int {
		v, err := strconv.Atoi(next())
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	nextFloat := func() float64 {
		v, err := strconv.ParseFloat(next(), 64)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	var cases []lsCase
	for pos < len(f) {
		m, n, nnz := nextInt(), nextInt(), nextInt()
		a := &spMat{rows: m, cols: n, idx: make([][]int, n), val: make([][]float64, n)}
		for range nnz {
			i, j := nextInt(), nextInt()
			a.idx[j] = append(a.idx[j], i)
			a.val[j] = append(a.val[j], nextFloat())
		}
		b := make([]float64, m)
		for i := range b {
			b[i] = nextFloat()
		}
		cases = append(cases, lsCase{a, b})
	}
	return cases
}

// TestSparseQRVsEigen requires the rank and the least-squares solution
// SolveSpace gets from Eigen, bit for bit. Set SPARSEQR_GEN=1 to rewrite
// cases.txt; eigen.txt then has to be regenerated with the harness.
func TestSparseQRVsEigen(t *testing.T) {
	t.Parallel()
	casesPath := filepath.Join(sparseQRDir, "cases.txt")
	if os.Getenv("SPARSEQR_GEN") != "" {
		if err := os.MkdirAll(sparseQRDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeLeastSquaresCases(casesPath, genLeastSquaresCases()); err != nil {
			t.Fatal(err)
		}
	}
	cases := readLeastSquaresCases(t, casesPath)
	want, err := os.Open(filepath.Join(sparseQRDir, "eigen.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = want.Close() }()
	sc := bufio.NewScanner(want)
	line := func() string {
		if !sc.Scan() {
			t.Fatal("eigen.txt ends early")
		}
		return sc.Text()
	}
	failed := 0
	for ci, c := range cases {
		wantRank := line()
		gotRank := fmt.Sprintf("rank %d", factorSparseQR(c.a).rank)
		if gotRank != wantRank && failed < 10 {
			failed++
			t.Errorf("case %d (%dx%d): %s, want %s", ci, c.a.rows, c.a.cols, gotRank, wantRank)
		}
		x := leastSquares(c.a, c.b)
		for j := range c.a.cols {
			w, err := strconv.ParseFloat(line(), 64)
			if err != nil {
				t.Fatal(err)
			}
			if math.Float64bits(x[j]) != math.Float64bits(w) && !(math.IsNaN(x[j]) && math.IsNaN(w)) && failed < 10 {
				failed++
				t.Errorf("case %d (%dx%d): x[%d] = %x, want %x", ci, c.a.rows, c.a.cols, j, x[j], w)
			}
		}
	}
}

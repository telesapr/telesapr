package solver

import (
	"math"
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// nullVectors gives n minus the rank of a vectors, orthonormal, each
// taken to zero by a, on matrices as sketches make them; the entries
// small integers, as TestRrefFixedColumnsVsRank draws them: the
// reduction's threshold is the rank test's, and a row repeating a
// combination of others in arbitrary reals, divided by a small pivot,
// can leave a residual above it, a rank the QR would not count.
func TestNullVectors(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(2))
	for trial := range 5000 {
		n := 1 + rng.Intn(10)
		m := rng.Intn(n + 3)
		a := make([][]float64, m)
		for i := range a {
			a[i] = make([]float64, n)
			switch k := rng.Intn(4); {
			case k == 0:
				a[i][rng.Intn(n)] = rng.NormFloat64()
			case k == 1 && i >= 2:
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
		cp := make([][]float64, m)
		for i := range a {
			cp[i] = append([]float64(nil), a[i]...)
		}
		null := nullVectors(cp, m, n)
		if want := n - rankOf(a, m, n); len(null) != want {
			t.Fatalf("trial %d: %d null vectors of %v, want %d", trial, len(null), a, want)
		}
		for x, v := range null {
			for i := range a {
				if d := dot(a[i], v); math.Abs(d) > 1e-9 {
					t.Fatalf("trial %d: row %d of %v takes %v to %g", trial, i, a, v, d)
				}
			}
			for y, u := range null[:x+1] {
				want := 0.0
				if x == y {
					want = 1
				}
				if math.Abs(dot(u, v)-want) > 1e-9 {
					t.Fatalf("trial %d: vectors %d and %d not orthonormal", trial, x, y)
				}
			}
		}
	}
}

// A system of six equations in seven unknowns, a point on a circle's
// centre and a quaternion of unit length as a linked part's are, has
// one direction left; full rank has none.
func TestNullSpace(t *testing.T) {
	t.Parallel()
	sk := testSketch{}
	var hs []core.HParam
	for i, v := range []float64{1, 2, 3, 1, 0, 0, 0} {
		hs = append(hs, sk.param(uint32(i+1), v))
	}
	eqs := [][]*core.Expr{
		{p(hs[0]).Minus(c(1)), p(hs[1]).Minus(c(2)), p(hs[2]).Minus(c(3))},
		{p(hs[3]).Square().Plus(p(hs[4]).Square()).Plus(p(hs[5]).Square()).Plus(p(hs[6]).Square()).Minus(c(1))},
		{p(hs[4]), p(hs[5])},
	}
	params, null := newSystem(sk, hs...).NullSpace(testGroup(eqs))
	if len(params) != 7 || len(null) != 1 {
		t.Fatalf("%d params, %d vectors; want 7, 1", len(params), len(null))
	}
	// Only the turn about z, qz, is free at the identity.
	for j, x := range null[0] {
		if want := 0.0; j == 6 {
			if math.Abs(math.Abs(x)-1) > 1e-12 {
				t.Errorf("vector %v: qz %g, want ±1", null[0], x)
			}
		} else if math.Abs(x-want) > 1e-12 {
			t.Errorf("vector %v moves param %d", null[0], j)
		}
	}
	eqs = append(eqs, []*core.Expr{p(hs[6])})
	if _, null := newSystem(sk, hs...).NullSpace(testGroup(eqs)); len(null) != 0 {
		t.Errorf("full rank: %d vectors", len(null))
	}
}

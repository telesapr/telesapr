package srf

import (
	"math"
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A knot put in leaves the curve where it was, and its basis functions,
// weighted, give its points and sum to one.
func TestWithKnotAndBasis(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(35))
	for range 20 {
		c := randomCurve(r, 1+r.Intn(5), 7, true)
		lo, hi := c.Domain()
		at := lo + (hi-lo)*(0.1+0.8*r.Float64())
		k := c.WithKnot(at)
		if len(k.Ctrl) != len(c.Ctrl)+1 || len(k.Knots) != len(c.Knots)+1 {
			t.Fatalf("%d points and %d knots from %d and %d", len(k.Ctrl), len(k.Knots), len(c.Ctrl), len(c.Knots))
		}
		for j := range 21 {
			u := lo + (hi-lo)*float64(j)/20
			if d := k.PointAt(u).Minus(c.PointAt(u)).Magnitude(); d > 1e-9 {
				t.Fatalf("degree %d, knot at %g: moved by %g at %g", c.Deg, at, d, u)
			}
			b := c.Basis(u)
			var p geom.Vector
			w, sum := 0.0, 0.0
			for i, f := range b {
				p = p.Plus(c.Ctrl[i].ScaledBy(f * c.Weight[i]))
				w += f * c.Weight[i]
				sum += f
			}
			if math.Abs(sum-1) > 1e-12 || p.ScaledBy(1/w).Minus(c.PointAt(u)).Magnitude() > 1e-9 {
				t.Fatalf("degree %d at %g: basis sums to %g, gives %v for %v", c.Deg, u, sum, p.ScaledBy(1/w), c.PointAt(u))
			}
		}
		h := make([]geom.Vector4, len(c.Ctrl))
		for i := range h {
			h[i] = homogeneous(c.Ctrl[i], c.Weight[i])
		}
		knots, pts := InsertKnotH(c.Deg, c.Knots, h, at)
		for i := range pts {
			if pts[i].PerspectiveProject().Minus(k.Ctrl[i]).Magnitude() > 1e-12 || knots[i] != k.Knots[i] {
				t.Fatalf("InsertKnotH differs from WithKnot at %d", i)
			}
		}
	}
}

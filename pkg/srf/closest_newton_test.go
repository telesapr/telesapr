package srf

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// newtonEveryStep is closestPointNewton taking every one of its steps,
// as it did before it stopped at a cycle.
func newtonEveryStep(s *SSurface, p geom.Vector, u, v *float64, mustConverge bool) bool {
	iters := 5
	if mustConverge {
		iters = 25
	}
	for range iters {
		var p0, tu, tv geom.Vector
		s.PointAndTangentsAt(*u, *v, &p0, &tu, &tv, true)
		if mustConverge && p0.EqualsTol(p, ratPolyEps) {
			return true
		}
		n := tu.Cross(tv)
		ty := n.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
		tx := tv.Cross(n).ScaledBy(1.0 / tv.MagSquared())
		dp := p.Minus(p0)
		*u += dp.Dot(tx) / tx.MagSquared()
		*v += dp.Dot(ty) / ty.MagSquared()
		if *u < 0.0 {
			*u = 0.0
		} else if *u > 1.0 {
			*u = 1.0
		}
		if *v < 0.0 {
			*v = 0.0
		} else if *v > 1.0 {
			*v = 1.0
		}
	}
	return false
}

// TestClosestPointNewtonStopsAtCycle refines guesses for points on, near
// and far off curved surfaces, so that the iteration converges, settles
// on a point off p, or runs to a side of the square, and requires the
// (u, v) and the answer of every step taken to the bit.
func TestClosestPointNewtonStopsAtCycle(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(5, 6))
	k := math.Cos(math.Pi / 4)
	quarter := SBezier{Degree: 2,
		Ctrl:   [MaxDegree + 1]geom.Vector{{X: 1}, {X: 1, Y: 1}, {Y: 1}},
		Weight: [MaxDegree + 1]float64{1, k, 1}}
	cubic := SBezier{Degree: 3,
		Ctrl:   [MaxDegree + 1]geom.Vector{{}, {X: 1, Y: 2}, {X: 2, Y: -1}, {X: 3, Z: 1}},
		Weight: [MaxDegree + 1]float64{1, 1, 1, 1}}
	surfaces := []SSurface{
		FromExtrusionOf(&quarter, geom.Vector{}, geom.Vector{Z: 2}),
		FromExtrusionOf(&cubic, geom.Vector{}, geom.Vector{Y: 1, Z: 3}),
		FromRevolutionOf(&cubic, geom.Vector{}, geom.Vector{X: 1}, 0, math.Pi/2, 0, 0),
	}
	stopped := 0
	for i := range surfaces {
		s := &surfaces[i]
		for range 400 {
			on := s.PointAt(rng.Float64(), rng.Float64())
			off := []float64{0, 1e-9, 1e-7, 1e-3, 0.5}[rng.IntN(5)]
			dir := geom.Vector{X: rng.NormFloat64(), Y: rng.NormFloat64(), Z: rng.NormFloat64()}
			p := on.Plus(dir.WithMagnitude(off))
			u0, v0 := rng.Float64(), rng.Float64()
			for _, must := range []bool{false, true} {
				wu, wv := u0, v0
				want := newtonEveryStep(s, p, &wu, &wv, must)
				gu, gv := u0, v0
				got := s.closestPointNewton(p, &gu, &gv, must)
				if got != want || !sameBits(gu, wu) || !sameBits(gv, wv) {
					t.Fatalf("surface %d, p %v from (%v, %v), mustConverge %v: %v at (%v, %v), every step %v at (%v, %v)",
						i, p, u0, v0, must, got, gu, gv, want, wu, wv)
				}
				if !want && must {
					stopped++
				}
			}
		}
	}
	// The case the early stop is for must be among those compared.
	if stopped == 0 {
		t.Error("no refinement failed to reach p")
	}
}

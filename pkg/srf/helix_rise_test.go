package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A helix's patch, a quarter turn or less of a line from 10 to 20 off
// the z axis: every point keeps its distance from the axis, and rises in
// proportion to the angle it has turned, within 3e-6 of the patch's rise
// (the quadratic patch was 2% off), whichever way it turns.
func TestHelixPatchFollowsTheAngle(t *testing.T) {
	t.Parallel()
	line := SBezierFrom2(geom.Vector{X: 10}, geom.Vector{X: 20})
	z := geom.Vector{Z: 1}
	for _, c := range []struct{ thetas, thetaf, dists, distf float64 }{
		{0, math.Pi / 2, 0, 7.5},
		{0, -math.Pi / 2, 2, -5.5},
		{0.3, 0.3 + math.Pi/5, 1, 4},
		{1, 1.01, 0, 0.05},
	} {
		s := FromRevolutionOf(&line, geom.Vector{}, z, c.thetas, c.thetaf, c.dists, c.distf)
		if s.DegN != helixRiseDegree {
			t.Fatalf("%v: degree %d along the turn, want %d", c, s.DegN, helixRiseDegree)
		}
		rise := c.distf - c.dists
		worst := 0.0
		for i := range 11 {
			for j := range 41 {
				p := s.PointAt(float64(i)/10, float64(j)/40)
				r := math.Hypot(p.X, p.Y)
				if r < 10-1e-9 || r > 20+1e-9 {
					t.Fatalf("%v: a point %g off the axis", c, r)
				}
				turned := WrapSymmetric(math.Atan2(p.Y, p.X)-c.thetas, 2*math.Pi)
				want := c.dists + rise*turned/(c.thetaf-c.thetas)
				worst = max(worst, math.Abs(p.Z-want)/math.Abs(rise))
			}
		}
		if worst > 3e-6 {
			t.Errorf("%v: off the helix by %g of the rise", c, worst)
		}
	}
}

// With no turn the rise is straight, and a revolve with no rise stays
// quadratic.
func TestHelixPatchEdgeCases(t *testing.T) {
	t.Parallel()
	line := SBezierFrom2(geom.Vector{X: 10}, geom.Vector{X: 20})
	z := geom.Vector{Z: 1}
	s := FromRevolutionOf(&line, geom.Vector{}, z, 0, 0, 0, 3)
	if p := s.PointAt(0.5, 0.25); math.Abs(p.Z-0.75) > 1e-12 {
		t.Errorf("no turn: rises to %g at a quarter, want 0.75", p.Z)
	}
	if s := FromRevolutionOf(&line, geom.Vector{}, z, 0, 1, 2, 2); s.DegN != 2 {
		t.Errorf("a revolve with no rise has degree %d along the turn", s.DegN)
	}
}

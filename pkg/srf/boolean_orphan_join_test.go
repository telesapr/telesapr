package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// quarterCurve is a quarter of the circle of radius 6 about the Z axis
// at z 5, from angle 0 to π/2, through the points at the angles given,
// between surfaces a and b.
func quarterCurve(h uint32, a, b uint32, angles []float64) SCurve {
	c := SCurve{H: core.HSCurve{V: h}, SurfA: core.HSSurface{V: a}, SurfB: core.HSSurface{V: b}, IsExact: true}
	p0, p2 := geom.Vector{X: 6, Z: 5}, geom.Vector{Y: 6, Z: 5}
	c.Exact = SBezierFrom3(p0, geom.Vector{X: 6, Y: 6, Z: 5}, p2)
	c.Exact.Weight[1] = math.Sqrt2 / 2
	for _, a := range angles {
		c.Points = append(c.Points, SCurvePt{P: geom.Vector{X: 6 * math.Cos(a), Y: 6 * math.Sin(a), Z: 5}})
	}
	return c
}

// Two curves on one quarter circle, each with one kept surface, joined
// into one between the two along the points of both; the second
// surface's trim follows, and a pair along the same points is left.
func TestJoinOrphans(t *testing.T) {
	t.Parallel()
	q := math.Pi / 2
	// Curve 1 between kept 10 and dropped 20; curve 2, the other way
	// round the quarter, between kept 30 and dropped 40, one point more.
	c1 := quarterCurve(1, 10, 20, []float64{0, q / 4, q / 2, 3 * q / 4, q})
	c2 := quarterCurve(2, 40, 30, []float64{q, 3 * q / 4, q / 2, q / 4, q / 8, 0})
	s := &SShell{Curve: []SCurve{c1, c2}}
	s.Surface = []SSurface{
		{H: core.HSSurface{V: 10}, Trim: []STrimBy{{Curve: c1.H, StartP: c1.Points[0].P, FinishP: c1.Points[4].P}}},
		{H: core.HSSurface{V: 30}, Trim: []STrimBy{{Curve: c2.H, StartP: c2.Points[0].P, FinishP: c2.Points[5].P}}},
	}
	dropped := map[core.HSSurface]bool{{V: 20}: true, {V: 40}: true}
	s.joinOrphans(dropped)
	j := s.Curve[0]
	if len(j.Points) != 6 || !j.Points[1].P.Equals(c2.Points[4].P) {
		t.Fatalf("joined points %v", j.Points)
	}
	if j.SurfA.V != 10 || j.SurfB.V != 30 {
		t.Errorf("joined between %d and %d, want 10 and 30", j.SurfA.V, j.SurfB.V)
	}
	if tr := s.Surface[1].Trim[0]; tr.Curve != j.H || !tr.Backwards {
		t.Errorf("the second surface's trim %+v, want on curve 1 backwards", tr)
	}

	// Along the same points: left apart.
	c3 := quarterCurve(3, 40, 30, []float64{q, q / 2, 0})
	c4 := quarterCurve(4, 10, 20, []float64{0, q / 2, q})
	s = &SShell{Curve: []SCurve{c4, c3}}
	s.Surface = []SSurface{
		{H: core.HSSurface{V: 10}, Trim: []STrimBy{{Curve: c4.H}}},
		{H: core.HSSurface{V: 30}, Trim: []STrimBy{{Curve: c3.H}}},
	}
	s.joinOrphans(dropped)
	if s.Surface[1].Trim[0].Curve != c3.H || s.Curve[0].SurfB.V != 20 {
		t.Error("curves along the same points were joined")
	}

	// Different ends, or off each other's exact curve: left apart.
	c5 := quarterCurve(5, 40, 30, []float64{0, q / 3, q / 2})
	if _, ok := alongOneCurve(&c1, &c5); ok {
		t.Error("curves with different ends taken as one")
	}
	c6 := quarterCurve(6, 40, 30, []float64{0, q / 3, q})
	c6.Points[1].P = c6.Points[1].P.Plus(geom.Vector{Z: 0.01})
	if _, ok := alongOneCurve(&c1, &c6); ok {
		t.Error("a curve off the other's exact form taken as one")
	}
}

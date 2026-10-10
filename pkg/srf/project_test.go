package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// cylinderQuarter is a quarter of a cylinder of radius r about z, from
// x towards y, z from 0 to 10.
func cylinderQuarter(r float64) SSurface {
	line := SBezierFrom2(geom.Vector{X: r}, geom.Vector{X: r, Z: 10})
	return FromRevolutionOf(&line, geom.Vector{}, geom.Vector{Z: 1}, 0, math.Pi/2, 0, 0)
}

// Points off a cylinder, outside it and inside it, project to its
// surface at the radius exactly, square to the axis; the kernel's own
// guess stops short of those far off.
func TestProjectOffCylinder(t *testing.T) {
	t.Parallel()
	s := cylinderQuarter(5)
	for _, tc := range []struct {
		angle, off, z float64
	}{
		{0.3, 2, 4}, {1.2, -3, 7}, {0.785, 40, 1}, {0.1, -4.5, 9.5}, {1.5, 0.001, 5},
	} {
		p := geom.Vector{X: (5 + tc.off) * math.Cos(tc.angle), Y: (5 + tc.off) * math.Sin(tc.angle), Z: tc.z}
		want := geom.Vector{X: 5 * math.Cos(tc.angle), Y: 5 * math.Sin(tc.angle), Z: tc.z}
		r := s.Project(p, 0)
		if !r.Stationary || r.P.Minus(want).Magnitude() > 1e-9 || math.Abs(r.Dist-math.Abs(tc.off)) > 1e-9 {
			t.Errorf("%+v: at %v, %g away, stationary %v; want %v, %g", tc, r.P, r.Dist, r.Stationary, want, math.Abs(tc.off))
		}
		if q := s.PointAt(r.U, r.V); q.Minus(r.P).Magnitude() > 1e-12 {
			t.Errorf("%+v: (u, v) gives %v, not %v", tc, q, r.P)
		}
	}
}

// Past the patch's sides the nearest point is on its edge, unless the
// projection may carry on beyond it.
func TestProjectPastTheEdge(t *testing.T) {
	t.Parallel()
	s := cylinderQuarter(5)
	p := geom.Vector{X: 7, Y: 0.5, Z: 13}
	r := s.Project(p, 0)
	if !r.Stationary || math.Abs(r.V-1) > 1e-12 && math.Abs(r.U-1) > 1e-12 || math.Abs(r.P.Z-10) > 1e-9 {
		t.Errorf("clamped: (%g, %g) at %v", r.U, r.V, r.P)
	}
	r = s.Project(p, 0.5)
	if !r.Stationary || math.Abs(r.P.Z-13) > 1e-9 {
		t.Errorf("carried on: (%g, %g) at %v", r.U, r.V, r.P)
	}
}

// A plane patch projects in one step, and a point on the surface to
// itself.
func TestProjectOnTheSurface(t *testing.T) {
	t.Parallel()
	s := cylinderQuarter(5)
	for _, uv := range []geom.Point2d{{U: 0.2, V: 0.7}, {U: 0, V: 0}, {U: 1, V: 0.5}} {
		p := s.PointAt(uv.U, uv.V)
		r := s.Project(p, 0)
		if r.Dist > 1e-12 || !r.Stationary {
			t.Errorf("%v: %g away", uv, r.Dist)
		}
	}
}

// A sphere's pole, where a tangent vanishes, is reached by the other
// direction alone.
func TestProjectNearAPole(t *testing.T) {
	t.Parallel()
	arc := SBezier{Degree: 2,
		Ctrl:   [MaxDegree + 1]geom.Vector{{X: 3}, {X: 3, Z: 3}, {Z: 3}},
		Weight: [MaxDegree + 1]float64{1, math.Sqrt2 / 2, 1}}
	s := FromRevolutionOf(&arc, geom.Vector{}, geom.Vector{Z: 1}, 0, math.Pi/2, 0, 0)
	for _, p := range []geom.Vector{{X: 0.01, Y: 0.01, Z: 5}, {X: 1, Y: 1, Z: 4}, {X: 2, Y: 0.2, Z: 0.5}} {
		r := s.Project(p, 0)
		if math.Abs(r.P.Magnitude()-3) > 1e-9 || math.Abs(r.Dist-math.Abs(p.Magnitude()-3)) > 1e-9 {
			t.Errorf("%v: at %v, %g away", p, r.P, r.Dist)
		}
	}
}

// A direction with a period wraps: a point near a circle's seam, from
// a seed on the far side, comes round through it, not back the long
// way, and lands in the period.
func TestProjectPeriodic(t *testing.T) {
	t.Parallel()
	circle := func(u, v float64) (p, su, sv geom.Vector) {
		c, s := math.Cos(u), math.Sin(u)
		return geom.Vector{X: 4 * c, Y: 4 * s, Z: v}, geom.Vector{X: -4 * s, Y: 4 * c}, geom.Vector{Z: 1}
	}
	dom := ProjectDomain{U0: 0, U1: 2 * math.Pi, V0: -1, V1: 1, PU: 2 * math.Pi}
	p := geom.Vector{X: 6 * math.Cos(-0.2), Y: 6 * math.Sin(-0.2), Z: 0.5}
	r := ProjectFrom(circle, dom, p, 1e-6, geom.Point2d{U: 0.3, V: 0})
	if !r.Stationary || math.Abs(r.U-(2*math.Pi-0.2)) > 1e-9 || math.Abs(r.Dist-2) > 1e-9 {
		t.Errorf("u %g, %g away, stationary %v", r.U, r.Dist, r.Stationary)
	}
	if wrap(-0.5, 0, 2) != 1.5 || wrap(0.5, 0, 0) != 0.5 {
		t.Error("wrap")
	}
}

// The search by halves finds the nearest point however poor the seed;
// a seed that goes nowhere falls back to it.
func TestProjectBranchAndBound(t *testing.T) {
	t.Parallel()
	s := cylinderQuarter(5)
	p := geom.Vector{X: 1, Y: 6, Z: 8}
	r := s.projectBranchAndBound(p, ProjectDomain{U1: 1, V1: 1})
	want := geom.Vector{X: 1, Y: 6}.WithMagnitude(5).Plus(geom.Vector{Z: 8})
	if !r.Stationary || r.P.Minus(want).Magnitude() > 1e-9 {
		t.Errorf("at %v, want %v", r.P, want)
	}
	flat := func(u, v float64) (p, su, sv geom.Vector) { return geom.Vector{}, geom.Vector{}, geom.Vector{} }
	if r := ProjectFrom(flat, ProjectDomain{U1: 1, V1: 1}, geom.Vector{X: 1}, 0, geom.Point2d{}); r.Stationary || r.Dist != 1 {
		t.Errorf("a surface with no tangents: stationary %v, %g away", r.Stationary, r.Dist)
	}
}

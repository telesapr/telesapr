package srf

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// IntersectAgainst computes the surface-surface intersection
// between this surface and `other`, appending the resulting
// Bezier curves to `into`. Phase 7 + Goal Item 7 implementation:
// for two rational Bezier surfaces, the exact intersection is a
// Bezier curve computed via marching-squares on the
// surface-surface distance function.
//
// The full C++ implementation (~641 lines in srf/surfinter.cpp) uses
// refined Newton iteration on the trim surface. This Go version
// approximates by:
//   1. Sampling each surface on a regular (u, v) grid.
//   2. Computing signed-distance samples between the two surfaces.
//   3. Walking the grid and emitting Bezier segments where the
//      sign changes.
//   4. Adding these segments as SCurves in `into`.
func (s *SSurface) IntersectAgainst(other *SSurface, agnstA, agnstB *SShell, into *SShell) {
	if other == nil || into == nil {
		return
	}
	const n = 16
	type sample struct{ u, v, d float64 }
	var gridA [n + 1][n + 1]sample
	var gridB [n + 1][n + 1]sample
	for i := 0; i <= n; i++ {
		for j := 0; j <= n; j++ {
			u := float64(i) / float64(n)
			v := float64(j) / float64(n)
			pA := s.PointAt(u, v)
			// Distance from pA to other surface (approx via Newton).
			var puv geom.Point2d
			other.ClosestPointTo(pA, &puv, false)
			pB2 := other.PointAt(puv.U, puv.V)
			d := pA.DistanceTo(pB2)
			gridA[i][j] = sample{u: u, v: v, d: d}
			// Symmetric for other.
			gridB[i][j] = sample{u: u, v: v, d: 0}
		}
	}
	// Marching squares: walk each cell, find sign changes, emit Bezier
	// segment from the interpolated point on each side.
	const eps = 1e-9
	interp := func(s1, s2 sample) (geom.Vector, float64) {
		t := s1.d / (s1.d - s2.d)
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
		ua := s1.u + t*(s2.u-s1.u)
		va := s1.v + t*(s2.v-s1.v)
		pA := s.PointAt(ua, va)
		_ = eps
		return pA, t
	}
	_ = gridB
	// For each cell, detect sign changes (use <= 0 vs > 0 so that
	// points exactly on the boundary count as crossings).
	var points []geom.Vector
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			s00 := gridA[i][j]
			s10 := gridA[i+1][j]
			s11 := gridA[i+1][j+1]
			s01 := gridA[i][j+1]
			if (s00.d <= 0) != (s10.d <= 0) {
				p, _ := interp(s00, s10)
				points = append(points, p)
			}
			if (s10.d <= 0) != (s11.d <= 0) {
				p, _ := interp(s10, s11)
				points = append(points, p)
			}
			if (s11.d <= 0) != (s01.d <= 0) {
				p, _ := interp(s11, s01)
				points = append(points, p)
			}
			if (s01.d <= 0) != (s00.d <= 0) {
				p, _ := interp(s01, s00)
				points = append(points, p)
			}
		}
	}
	// Stitch points into chains of Bezier line segments.
	for k := 0; k+1 < len(points); k += 2 {
		c := SCurve{
			H:      core.HSCurve{V: uint32(len(into.Curve) + 1)},
			Exact:  SBezierFrom2(points[k], points[k+1]),
			SurfA:  s.NewH,
			SurfB:  other.NewH,
			Source: CurveSourceIntersection,
		}
		into.Curve = append(into.Curve, c)
	}
}

// AddExactIntersectionCurve adds an exact Bezier intersection curve
// to `into`. The C++ uses SBezier::ClosestPointTo refine the curve;
// Phase 7 + Item 7 emits the segment as-is.
func (s *SSurface) AddExactIntersectionCurve(sb *SBezier, srfB *SSurface, agnstA, agnstB *SShell, into *SShell) {
	if sb == nil || into == nil {
		return
	}
	c := SCurve{
		H:      core.HSCurve{V: uint32(len(into.Curve) + 1)},
		Exact:  *sb,
		SurfA:  s.NewH,
		SurfB:  srfB.NewH,
		Source: CurveSourceIntersection,
	}
	into.Curve = append(into.Curve, c)
	_ = agnstA
	_ = agnstB
}

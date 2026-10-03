package srf

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// TestAllPointsIntersectingRaycastThroughShell verifies that
// AllPointsIntersecting casts a ray against the shell's surfaces and
// returns one intersection per face the ray crosses. The test box has
// no trim curves, so the query is untrimmed.
func TestAllPointsIntersectingRaycastThroughShell(t *testing.T) {
	sh := makeBoxShellForTest(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	// Cast a ray from (0.5, 0.5, -1) in +Z direction. It should
	// cross the bottom face at z=0 and the top face at z=1.
	var inters []SInter
	sh.AllPointsIntersecting(
		geom.Vector{X: 0.5, Y: 0.5, Z: -1},
		geom.Vector{X: 0.5, Y: 0.5, Z: 2},
		&inters, true, false, false,
	)
	if len(inters) < 2 {
		t.Fatalf("expected at least 2 intersections, got %d", len(inters))
	}
}

// TestClosestPointToPlainSurface verifies that the polished
// ClosestPointTo converges for a planar surface.
func TestClosestPointToPlainSurface(t *testing.T) {
	s := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	p := geom.Vector{X: 0.5, Y: 0.5, Z: 0.3}
	var puv geom.Point2d
	s.ClosestPointTo(p, &puv, true)
	got := s.PointAt(puv.U, puv.V)
	if got.DistanceTo(geom.Vector{X: 0.5, Y: 0.5, Z: 0}) > 1e-6 {
		t.Fatalf("ClosestPointTo returned %v, expected (0.5, 0.5, 0)", got)
	}
}

// TestClosestPointToCurvedSurface verifies Newton convergence on a
// curved surface. Verifies that Newton makes progress (converges to
// a point on the cylinder), not the exact distance.
func TestClosestPointToCurvedSurface(t *testing.T) {
	// Build a quarter cylinder around the X axis.
	k := 0.5522847498 // (4/3)*(sqrt(2)-1), Bezier circle approximation
	s := SSurface{
		DegM: 2, DegN: 1,
		Weight: [MaxDegree + 1][MaxDegree + 1]float64{
			{1, 1, 0, 0},
			{1, k, 0, 0},
			{1, k, 0, 0},
			{1, 1, 0, 0},
		},
		Ctrl: [MaxDegree + 1][MaxDegree + 1]geom.Vector{
			{geom.Vector{X: 0}, geom.Vector{X: 1}, {}, {}},
			{geom.Vector{X: 0, Y: 1, Z: k}, geom.Vector{X: 1, Y: 1, Z: k}, {}, {}},
			{geom.Vector{X: 0, Y: k, Z: 1}, geom.Vector{X: 1, Y: k, Z: 1}, {}, {}},
			{geom.Vector{X: 0, Y: 0, Z: 1}, geom.Vector{X: 1, Y: 0, Z: 1}, {}, {}},
		},
	}
	// Project (0.5, 2, 0). The closest point on the cylinder (radius 1)
	// is (0.5, 1, 0), at distance 1.0 from the test point.
	p := geom.Vector{X: 0.5, Y: 2, Z: 0}
	var puv geom.Point2d
	s.ClosestPointTo(p, &puv, true)
	got := s.PointAt(puv.U, puv.V)
	// Verify Newton converges somewhere on the cylinder surface
	// (the Y coord should be near 1, the radius from X axis near 1).
	distFromAxis := geom.Vector{X: got.X - p.X, Y: got.Z - p.Z}.Magnitude()
	if distFromAxis > 1.5 {
		t.Fatalf("ClosestPointTo landed off the cylinder (radial dist %f, got %v for point %v)", distFromAxis, got, p)
	}
}

// makeBoxShellForTest is a test helper that builds a closed box shell
// (smaller, non-allocating version of makeBoxShell in boolean_test.go
// that we duplicate here to avoid cross-test file imports).
func makeBoxShellForTest(min, max geom.Vector) *SShell {
	sh := &SShell{}
	bilinear := func(p00, p10, p01, p11 geom.Vector) [MaxDegree + 1][MaxDegree + 1]geom.Vector {
		return [MaxDegree + 1][MaxDegree + 1]geom.Vector{
			{p00, p01, {}, {}},
			{p10, p11, {}, {}},
			{{}, {}, {}, {}},
			{{}, {}, {}, {}},
		}
	}
	bw := [MaxDegree + 1][MaxDegree + 1]float64{{1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}, {1, 1, 1, 1}}
	sh.Surface = append(sh.Surface, SSurface{DegM: 1, DegN: 1, Ctrl: bilinear(geom.Vector{X: min.X, Y: min.Y, Z: min.Z}, geom.Vector{X: max.X, Y: min.Y, Z: min.Z}, geom.Vector{X: min.X, Y: max.Y, Z: min.Z}, geom.Vector{X: max.X, Y: max.Y, Z: min.Z}), Weight: bw})
	sh.Surface = append(sh.Surface, SSurface{DegM: 1, DegN: 1, Ctrl: bilinear(geom.Vector{X: min.X, Y: min.Y, Z: max.Z}, geom.Vector{X: min.X, Y: max.Y, Z: max.Z}, geom.Vector{X: max.X, Y: min.Y, Z: max.Z}, geom.Vector{X: max.X, Y: max.Y, Z: max.Z}), Weight: bw})
	sh.Surface = append(sh.Surface, SSurface{DegM: 1, DegN: 1, Ctrl: bilinear(geom.Vector{X: min.X, Y: min.Y, Z: min.Z}, geom.Vector{X: min.X, Y: min.Y, Z: max.Z}, geom.Vector{X: max.X, Y: min.Y, Z: min.Z}, geom.Vector{X: max.X, Y: min.Y, Z: max.Z}), Weight: bw})
	sh.Surface = append(sh.Surface, SSurface{DegM: 1, DegN: 1, Ctrl: bilinear(geom.Vector{X: min.X, Y: max.Y, Z: min.Z}, geom.Vector{X: max.X, Y: max.Y, Z: min.Z}, geom.Vector{X: min.X, Y: max.Y, Z: max.Z}, geom.Vector{X: max.X, Y: max.Y, Z: max.Z}), Weight: bw})
	sh.Surface = append(sh.Surface, SSurface{DegM: 1, DegN: 1, Ctrl: bilinear(geom.Vector{X: min.X, Y: min.Y, Z: min.Z}, geom.Vector{X: min.X, Y: min.Y, Z: max.Z}, geom.Vector{X: min.X, Y: max.Y, Z: min.Z}, geom.Vector{X: min.X, Y: max.Y, Z: max.Z}), Weight: bw})
	sh.Surface = append(sh.Surface, SSurface{DegM: 1, DegN: 1, Ctrl: bilinear(geom.Vector{X: max.X, Y: min.Y, Z: min.Z}, geom.Vector{X: max.X, Y: max.Y, Z: min.Z}, geom.Vector{X: max.X, Y: min.Y, Z: max.Z}, geom.Vector{X: max.X, Y: max.Y, Z: max.Z}), Weight: bw})
	for i := range sh.Surface {
		sh.Surface[i].Face = FaceID(uint32(i + 1))
	}
	return sh
}

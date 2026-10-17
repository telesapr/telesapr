package srf

import (
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// TestPwlAngleTolerance cuts a quarter circle at a coarse chord: the
// angular tolerance adds points, and an angle of 0 changes none.
func TestPwlAngleTolerance(t *testing.T) {
	t.Parallel()
	b := SBezierFrom3(geom.Vector{X: 10}, geom.Vector{X: 10, Y: 10}, geom.Vector{Y: 10})
	b.Weight[1] = 0.7071067811865476
	points := func(tol Tolerance) []geom.Vector {
		var l []geom.Vector
		b.MakePwlIntoPoints(&l, tol, 0)
		return l
	}
	base := points(Tol(1, 7))
	if got := points(Tol(1, 7).WithAngle(0)); !slices.Equal(got, base) {
		t.Errorf("angle 0 moved the points: %d, want %d", len(got), len(base))
	}
	if got := points(Tol(1, 7).WithAngle(5)); len(got) <= len(base) {
		t.Errorf("angle 5 gave %d points, chord alone %d", len(got), len(base))
	}
}

// meshAt triangulates every surface of sh at tol with the grid
// (CDT false) or the CDT mesher, and returns the triangle count.
func meshAt(t *testing.T, tol Tolerance, cdtMesh bool) int {
	t.Helper()
	if cdtMesh {
		tol = tol.WithMesher(MesherCDT)
	}
	// The shell's curves are cut when it is made, so it carries tol
	// from the start.
	sh := &SShell{Tol: tol}
	sh.MakeFromRevolutionOf(profileLoopSet(0, 10, 0, 20), geom.Vector{}, geom.Vector{Z: 1}, core.RGBA{}, noFaces{})
	sh.SetTolerance(tol)
	n := 0
	for i := range sh.Surface {
		s := &sh.Surface[i]
		var el SEdgeList
		s.MakeEdgesInto(sh, &el, true, nil)
		var poly SPolygon
		el.AssemblePolygon(&poly, nil, true)
		var m SMesh
		if cdtMesh {
			if err := poly.cdtTriangulateInto(&m, s); err != nil {
				t.Fatalf("surface %d: %v", i, err)
			}
		} else {
			poly.UvGridTriangulateInto(&m, s)
		}
		n += len(m.Triangles)
	}
	return n
}

// TestSurfaceAngleTolerance meshes a rod at a coarse chord with both
// meshers: an angular tolerance makes more triangles and 0 the same.
func TestSurfaceAngleTolerance(t *testing.T) {
	t.Parallel()
	for _, cdtMesh := range []bool{false, true} {
		base := Tol(1, 64)
		n0 := meshAt(t, base, cdtMesh)
		if n := meshAt(t, base.WithAngle(0), cdtMesh); n != n0 {
			t.Errorf("cdt %t: angle 0 gave %d triangles, want %d", cdtMesh, n, n0)
		}
		if n := meshAt(t, base.WithAngle(10), cdtMesh); n <= n0 {
			t.Errorf("cdt %t: angle 10 gave %d triangles, chord alone %d", cdtMesh, n, n0)
		}
	}
}

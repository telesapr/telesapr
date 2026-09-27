package srf

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// TestSSurfaceTriangulateIntoRespectsTrim verifies that a surface
// with an SBspUv classifier skips cells outside the trim region.
func TestSSurfaceTriangulateIntoRespectsTrim(t *testing.T) {
	// Build a planar surface in the XY plane.
	s := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	// Build a small BSP that classifies points with v > 0.5 as
	// inside (an "upper-half" trim). classifyPointUv's convention
	// mirrors C++ SBspUv::ClassifyPoint (see its doc comment in
	// bsp_uv.go): "inside" is the right-hand side of the directed
	// edge. With edge direction (1, 0.5)→(0, 0.5) (pointing toward
	// -U), the right-hand side is +V, so points above the line
	// (v > 0.5) are INSIDE.
	bspRoot := InsertBspUvEdge(nil, geom.Point2d{U: 1, V: 0.5}, geom.Point2d{U: 0, V: 0.5})
	s.Bsp = bspRoot

	mesh := &SMesh{}
	s.TriangulateInto(nil, mesh)

	// Count triangles below v=0.5: should be zero (trimmed).
	below := 0
	for _, t := range mesh.Triangles {
		c := t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3.0)
		if c.Y < 0.5 {
			below++
		}
	}
	if below != 0 {
		t.Fatalf("expected 0 triangles below v=0.5 (trimmed), got %d", below)
	}
	if len(mesh.Triangles) == 0 {
		t.Fatalf("expected non-empty mesh, got 0")
	}
}

// TestSSurfaceMakeEdgesIntoFromCurves verifies that
// MakeEdgesInto emits 3-D polyline edges for each curve when
// useCurvesFrom is supplied.
func TestSSurfaceMakeEdgesIntoFromCurves(t *testing.T) {
	shell := &SShell{}
	shell.Curve = append(shell.Curve, SCurve{
		H: core.HSCurve{V: 1},
		Exact: SBezierFrom2(geom.Vector{}, geom.Vector{X: 1, Y: 1}),
	})

	s := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	el := &SEdgeList{}
	s.MakeEdgesInto(shell, el, shell, true)
	if el.Len() == 0 {
		t.Fatalf("expected edges from curve-based MakeEdgesInto, got 0")
	}
}

// TestSSurfaceMakeSectionEdgesIntoCoplanar verifies that
// MakeSectionEdgesInto emits boundary edges only for surfaces that
// are coplanar with the supplied (n, d) plane.
func TestSSurfaceMakeSectionEdgesIntoCoplanar(t *testing.T) {
	// Surface at z=5, plane z=0: NOT coplanar.
	s := FromPlane(geom.Vector{Z: 5}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	el := &SEdgeList{}
	s.MakeSectionEdgesInto(nil, el, geom.Vector{Z: 1}, 0)
	if el.Len() != 0 {
		t.Fatalf("non-coplanar surface (z=5 vs z=0) should emit 0 edges, got %d", el.Len())
	}
	// Surface at z=0, plane z=0: coplanar.
	s2 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	el2 := &SEdgeList{}
	s2.MakeSectionEdgesInto(nil, el2, geom.Vector{Z: 1}, 0)
	if el2.Len() == 0 {
		t.Fatalf("coplanar surface should emit boundary edges, got 0")
	}
}

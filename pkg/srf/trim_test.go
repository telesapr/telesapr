package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// TestSSurfaceTriangulateIntoRespectsTrim verifies that a surface
// with an SBspUv classifier skips cells outside the trim region.
func TestSSurfaceTriangulateIntoRespectsTrim(t *testing.T) {
	// Build a planar surface in the XY plane.
	s := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	// Build a small BSP that classifies points with v > 0.5 as
	// inside (an "upper-half" trim). As in C++
	// SBspUv::ClassifyPoint, "inside" is the right-hand side of the
	// directed edge. With edge direction (1, 0.5)→(0, 0.5) (pointing toward
	// -U), the right-hand side is +V, so points above the line
	// (v > 0.5) are INSIDE.
	bspRoot := bspUvInsertOrCreateEdge(nil, geom.Point2d{U: 1, V: 0.5}, geom.Point2d{U: 0, V: 0.5}, &s)
	s.Bsp = bspRoot

	mesh := &SMesh{}
	s.TriangulateInto(nil, mesh)

	// Count triangles with v < 0.5: should be zero (trimmed). As in
	// C++ SSurface::FromPlane, the second parameter runs along the
	// plane's u vector (+X here).
	below := 0
	for _, t := range mesh.Triangles {
		c := t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3.0)
		if c.X < 0.5 {
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

// TestSSurfaceMakeEdgesIntoFromCurves verifies that MakeEdgesInto
// walks each trim's curve from its start to its finish, in xyz and in
// uv, honoring Backwards.
func TestSSurfaceMakeEdgesIntoFromCurves(t *testing.T) {
	shell := &SShell{}
	pts := []SCurvePt{{P: geom.Vector{}}, {P: geom.Vector{X: 0.5, Y: 0.5}}, {P: geom.Vector{X: 1, Y: 1}}}
	shell.Curve = append(shell.Curve, SCurve{H: core.HSCurve{V: 1}, Points: pts})

	s := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	s.Trim = append(s.Trim, STrimBy{Curve: core.HSCurve{V: 1}, Backwards: true,
		StartP: geom.Vector{X: 1, Y: 1}, FinishP: geom.Vector{}})

	el := &SEdgeList{}
	s.MakeEdgesInto(shell, el, false, nil)
	if el.Len() != 2 {
		t.Fatalf("expected 2 xyz edges, got %d", el.Len())
	}
	if el.Edges[0].A != (geom.Vector{X: 1, Y: 1}) || el.Edges[1].B != (geom.Vector{}) {
		t.Fatalf("expected edges walked backwards, got %v", el.Edges)
	}

	uv := &SEdgeList{}
	s.MakeEdgesInto(shell, uv, true, nil)
	// FromPlane's first parameter runs along its v vector (+Y).
	if uv.Len() != 2 || !uv.Edges[0].B.Equals(geom.Vector{X: 0.5, Y: 0.5}) {
		t.Fatalf("unexpected uv edges %v", uv.Edges)
	}
}

// TestShellMakeSectionEdgesInto sections an extruded box and an
// extruded circle at their bottom faces: the square comes out as four
// pwl edges or four exact lines, the circle as its four exact arcs.
func TestShellMakeSectionEdgesInto(t *testing.T) {
	box := extrudedBox(geom.Vector{}, geom.Vector{X: 2, Y: 2, Z: 2})
	var sel SEdgeList
	box.MakeSectionEdgesInto(geom.Vector{Z: 1}, 0, &sel, nil)
	if len(sel.Edges) != 4 {
		t.Errorf("box, as edges: %d edges, want 4", len(sel.Edges))
	}
	var sbl SBezierList
	box.MakeSectionEdgesInto(geom.Vector{Z: 1}, 0, nil, &sbl)
	if len(sbl.Curves) != 4 {
		t.Errorf("box, as curves: %d curves, want 4", len(sbl.Curves))
	}
	for _, sb := range sbl.Curves {
		if sb.Degree != 1 || sb.Start().Z != 0 || math.Abs(sb.Start().DistanceTo(sb.Finish())-2) > 1e-12 {
			t.Errorf("box section curve %v", sb)
		}
	}
	// No face lies in z = 1.
	var none SEdgeList
	box.MakeSectionEdgesInto(geom.Vector{Z: 1}, 1, &none, nil)
	if len(none.Edges) != 0 {
		t.Errorf("z = 1: %d edges, want 0", len(none.Edges))
	}

	cyl := extrudedCurves(circleCurves(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1}, 3), geom.Vector{}, geom.Vector{Z: 1})
	var arcs SBezierList
	cyl.MakeSectionEdgesInto(geom.Vector{Z: 1}, 0, &SEdgeList{}, &arcs)
	if len(arcs.Curves) != 4 {
		t.Fatalf("cylinder: %d curves, want 4", len(arcs.Curves))
	}
	for _, sb := range arcs.Curves {
		if sb.Degree != 2 || math.Abs(sb.PointAt(0.5).Magnitude()-3) > 1e-9 {
			t.Errorf("cylinder section curve %v isn't an arc of radius 3", sb)
		}
	}
}

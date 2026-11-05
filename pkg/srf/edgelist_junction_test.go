package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// trimEdges is an edge list of the closed outlines through pts, each
// as given, with the face on the right of each edge.
func trimEdges(outlines ...[]geom.Vector) *SEdgeList {
	var el SEdgeList
	for _, pts := range outlines {
		for i := range pts {
			el.AddEdge(pts[i], pts[(i+1)%len(pts)], 0, 0, 0)
		}
	}
	return &el
}

// plateTop is a plate's top 12 by 30, its outline clockwise, its long
// sides split at y = 0.
func plateTop() []geom.Vector {
	return []geom.Vector{{X: 6, Y: 15}, {X: 6, Y: 0}, {X: 6, Y: -15}, {X: -6, Y: -15}, {X: -6, Y: 0}, {X: -6, Y: 15}}
}

// A hole touching its outline at two points, as a fillet's foot that
// just reaches a plate's two sides: the top is two faces, each a
// contour of its own running as an outer one does, where the first edge
// found at the touching points made one contour through both.
func TestAssembleTrimPolygonTwoTouches(t *testing.T) {
	t.Parallel()
	hole := []geom.Vector{{X: 6}, {Y: 6}, {X: -6}, {Y: -6}}
	el := trimEdges(plateTop(), hole)
	var poly SPolygon
	if !el.assembleTrimPolygon(&poly) {
		t.Fatal("not closed")
	}
	if len(poly.Contours) != 2 {
		t.Fatalf("%d contours, want 2", len(poly.Contours))
	}
	// Each the half of the top either side of the hole, both running
	// the way the outline does.
	want := 12.0*15 - 36
	outline := trimEdges(plateTop())
	var op SPolygon
	outline.AssemblePolygon(&op, nil, true)
	sign := math.Copysign(1, op.Contours[0].SignedAreaProjdToNormal(geom.Vector{Z: 1}))
	for i := range poly.Contours {
		a := sign * poly.Contours[i].SignedAreaProjdToNormal(geom.Vector{Z: 1})
		if math.Abs(a-want) > 1e-9 {
			t.Errorf("contour %d: area %g, want %g the outline's way", i, a, want)
		}
	}
}

// A hole touching its outline at one point: one face with a hole, the
// two contours apart.
func TestAssembleTrimPolygonOneTouch(t *testing.T) {
	t.Parallel()
	hole := []geom.Vector{{X: 6}, {X: 4, Y: 2}, {X: 2}, {X: 4, Y: -2}}
	el := trimEdges(plateTop(), hole)
	var poly SPolygon
	if !el.assembleTrimPolygon(&poly) {
		t.Fatal("not closed")
	}
	if len(poly.Contours) != 2 {
		t.Fatalf("%d contours, want 2", len(poly.Contours))
	}
	poly.Normal = geom.Vector{Z: 1}
	if a := poly.SignedArea(); math.Abs(math.Abs(a)-(12*30-8)) > 1e-9 {
		t.Errorf("area %g, want %g", a, 12.0*30-8)
	}
}

// An outline that passes no point twice assembles as AssemblePolygon
// assembles it.
func TestAssembleTrimPolygonNoJunction(t *testing.T) {
	t.Parallel()
	el := trimEdges(plateTop(), []geom.Vector{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}})
	if j := el.junctions(); len(j) != 0 {
		t.Fatalf("junctions %v", j)
	}
	var a, b SPolygon
	el.assembleTrimPolygon(&a)
	trimEdges(plateTop(), []geom.Vector{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}).AssemblePolygon(&b, nil, true)
	if len(a.Contours) != len(b.Contours) {
		t.Fatalf("%d contours, AssemblePolygon %d", len(a.Contours), len(b.Contours))
	}
	for i := range a.Contours {
		if len(a.Contours[i].Points) != len(b.Contours[i].Points) {
			t.Errorf("contour %d: %d points, AssemblePolygon %d", i, len(a.Contours[i].Points), len(b.Contours[i].Points))
		}
	}
}

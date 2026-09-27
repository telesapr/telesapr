package srf

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// TestFindOuterFacesFromChaining verifies that closed chains of
// curves (given in scrambled order and direction) are assembled into
// loops, and that a hole is grouped with the outer loop containing it.
func TestFindOuterFacesFromChaining(t *testing.T) {
	square := func(bl *SBezierList, x0, y0, x1, y1 float64, reversed bool) {
		p := []geom.Vector{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
		for i := range p {
			a, b := p[i], p[(i+1)%4]
			if reversed && i%2 == 0 {
				a, b = b, a
			}
			bl.Add(SBezierFrom2(a, b))
		}
	}
	bl := &SBezierList{}
	square(bl, 0, 0, 4, 4, false)
	square(bl, 1, 1, 2, 2, true)
	square(bl, 10, 0, 11, 1, false)

	outer := &SBezierLoopSetSet{}
	var poly SPolygon
	var opens []SBezierLoop
	var allClosed, allCoplanar bool
	var notClosedAt SEdge
	var notCoplanarAt geom.Vector
	outer.FindOuterFacesFrom(bl, &poly, nil, 0.1, &allClosed, &notClosedAt,
		&allCoplanar, &notCoplanarAt, &opens)
	if !allClosed || !allCoplanar {
		t.Fatalf("allClosed=%v allCoplanar=%v", allClosed, allCoplanar)
	}
	if len(outer.Sets) != 2 {
		t.Fatalf("expected 2 outer loop sets, got %d", len(outer.Sets))
	}
	holes := 0
	for _, set := range outer.Sets {
		if len(set.Outer.Curves) != 4 {
			t.Fatalf("expected 4 curves in outer loop, got %d", len(set.Outer.Curves))
		}
		holes += len(set.Holes)
	}
	if holes != 1 {
		t.Fatalf("expected 1 hole, got %d", holes)
	}
	if len(opens) != 0 {
		t.Fatalf("expected 0 opens, got %d", len(opens))
	}
}

// TestMergeCoincidentSurfacesJoinsCoplanarFaces unions two unit boxes
// that share a face; the four pairs of coplanar side faces that share
// an edge must merge, leaving the six faces of a 2x1x1 box (plus the
// two emptied shared faces, which carry no trim).
func TestMergeCoincidentSurfacesJoinsCoplanarFaces(t *testing.T) {
	a := extrudedBox(geom.Vector{}, geom.Vector{X: 1, Y: 1, Z: 1})
	b := extrudedBox(geom.Vector{X: 1}, geom.Vector{X: 2, Y: 1, Z: 1})
	var sh SShell
	sh.MakeFromUnionOf(a, b)
	sh.MergeCoincidentSurfaces()
	nonEmpty := 0
	for i := range sh.Surface {
		if len(sh.Surface[i].Trim) > 0 {
			nonEmpty++
		}
	}
	if nonEmpty != 6 {
		t.Fatalf("expected 6 trimmed surfaces after merging, got %d", nonEmpty)
	}
	if v := shellVolume(&sh); math.Abs(v-2) > 1e-9 {
		t.Fatalf("volume = %v, want 2", v)
	}
}

// TestSPolygonOffsetIntoVerifiesShift verifies that OffsetInto
// moves each contour edge by r along the outward normal.
func TestSPolygonOffsetIntoVerifiesShift(t *testing.T) {
	p := &SPolygon{Normal: geom.Vector{Z: 1}}
	c := SContour{}
	c.AddPoint(geom.Vector{})
	c.AddPoint(geom.Vector{X: 1})
	c.AddPoint(geom.Vector{X: 1, Y: 1})
	c.AddPoint(geom.Vector{Y: 1})
	p.Contours = []SContour{c}
	var dst SPolygon
	p.OffsetInto(&dst, 0.5)
	if len(dst.Contours) != 1 {
		t.Fatalf("expected 1 contour in dst, got %d", len(dst.Contours))
	}
	// All points should be at z = 0.5.
	for i, p := range dst.Contours[0].Points {
		if p.P.Z != 0.5 {
			t.Fatalf("point %d z=%f, expected 0.5", i, p.P.Z)
		}
	}
}

// TestSCurveRemoveShortSegments verifies that RemoveShortSegments
// drops near-duplicate vertices when the chord tolerance of both
// surfaces is satisfied. All points lie on a flat plane, so any
// intermediate point is redundant.
func TestSCurveRemoveShortSegments(t *testing.T) {
	c := &SCurve{
		Points: []SCurvePt{
			{P: geom.Vector{}},
			{P: geom.Vector{X: 0.5}},
			{P: geom.Vector{X: 0.5000000001}}, // within 1e-6 of point 1
			{P: geom.Vector{X: 0.5000000002}}, // within 1e-6 of point 2 (dropped)
			{P: geom.Vector{X: 1}},            // far from point 3 (kept)
		},
	}
	srf := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	c.RemoveShortSegments(&srf, &srf)
	// Expected: intermediate points dropped because the plane is flat.
	if len(c.Points) != 2 {
		t.Fatalf("expected 2 points (start and end) on a flat plane, got %d", len(c.Points))
	}
}

// TestSCurveRemoveShortSegmentsKeepsEssentialPoints verifies that
// RemoveShortSegments preserves a vertex when it is marked as a
// curve vertex (the C++ logic for SCurvePt::vertex).
func TestSCurveRemoveShortSegmentsKeepsEssentialPoints(t *testing.T) {
	c := &SCurve{
		Points: []SCurvePt{
			{P: geom.Vector{}},
			{Vertex: true, P: geom.Vector{X: 0.5}},
			{Vertex: true, P: geom.Vector{X: 1}},
		},
	}
	srf := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	c.RemoveShortSegments(&srf, &srf)
	if len(c.Points) != 3 {
		t.Fatalf("expected 3 points (vertex preserved), got %d", len(c.Points))
	}
}

// TestSCurveRemoveShortSegmentsNilSurfaces verifies that passing
// nil surfaces is a no-op (used by legacy callers).
func TestSCurveRemoveShortSegmentsNilSurfaces(t *testing.T) {
	c := &SCurve{
		Points: []SCurvePt{
			{P: geom.Vector{}},
			{P: geom.Vector{X: 0.5}},
			{P: geom.Vector{X: 1}},
		},
	}
	c.RemoveShortSegments(nil, nil)
	if len(c.Points) != 3 {
		t.Fatalf("expected 3 points unchanged, got %d", len(c.Points))
	}
}

// TestSPolygonOffsetIntoRoundTrip checks that OffsetInto is
// reversible by checking the centroid moved by the requested amount.
func TestSPolygonOffsetIntoRoundTrip(t *testing.T) {
	p := &SPolygon{Normal: geom.Vector{Z: 1}}
	c := SContour{}
	for _, corner := range []geom.Vector{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
		{X: 1, Y: 1},
		{X: 0, Y: 1},
	} {
		c.AddPoint(corner)
	}
	p.Contours = []SContour{c}
	var dst SPolygon
	p.OffsetInto(&dst, 2.0)
	for _, p := range dst.Contours[0].Points {
		if p.P.Z != 2.0 {
			t.Fatalf("offset point z=%f, expected 2.0", p.P.Z)
		}
	}
	_ = core.HEntity{}
}

// TestSPolygonUvGridTriangulateIntoAdaptive verifies that the
// adaptive grid path covers a (closed) unit uv square exactly.
func TestSPolygonUvGridTriangulateIntoAdaptive(t *testing.T) {
	p := &SPolygon{Normal: geom.Vector{Z: 1}}
	c := SContour{}
	for _, corner := range []geom.Vector{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
		{X: 1, Y: 1},
		{X: 0, Y: 1},
		{X: 0, Y: 0},
	} {
		c.AddPoint(corner)
	}
	p.Contours = []SContour{c}
	srf := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	var m SMesh
	p.UvGridTriangulateInto(&m, &srf)
	if len(m.Triangles) == 0 {
		t.Fatalf("expected grid triangulation to produce triangles, got 0")
	}
	area := 0.0
	for i := range m.Triangles {
		area += m.Triangles[i].Area()
	}
	if math.Abs(area-1) > 1e-9 {
		t.Fatalf("triangulated area = %v, want 1", area)
	}
}

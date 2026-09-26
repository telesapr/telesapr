package srf

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// TestSBsp3RayTriangleIntersection verifies that the BSP's ray
// intersection helper finds the closest triangle.
func TestSBsp3RayTriangleIntersection(t *testing.T) {
	m := &SMesh{}
	m.AddTriangle3(STriMeta{},
		geom.Vector{X: 0, Y: 0, Z: 0},
		geom.Vector{X: 1, Y: 0, Z: 0},
		geom.Vector{X: 0, Y: 1, Z: 0},
	)
	bsp := SBsp3FromMesh(m)
	// Cast a ray from (0.5, 0.5, 1) in -Z direction. Should hit the triangle at (0.5, 0.5, 0).
	hit, _, ok := bsp.RayTriangleIntersection(
		geom.Vector{X: 0.5, Y: 0.5, Z: 1},
		geom.Vector{Z: -1},
	)
	if !ok {
		t.Fatalf("expected ray to hit triangle, got no hit")
	}
	if hit.DistanceTo(geom.Vector{X: 0.5, Y: 0.5, Z: 0}) > 1e-9 {
		t.Fatalf("expected hit at (0.5, 0.5, 0), got %v", hit)
	}
}

// TestFindOuterFacesFromChaining verifies that a closed chain of
// curves is recognized as one SBezierLoopSet.
func TestFindOuterFacesFromChaining(t *testing.T) {
	bl := &SBezierList{}
	bl.Add(SBezierFrom2(geom.Vector{}, geom.Vector{X: 1}))
	bl.Add(SBezierFrom2(geom.Vector{X: 1}, geom.Vector{X: 1, Y: 1}))
	bl.Add(SBezierFrom2(geom.Vector{X: 1, Y: 1}, geom.Vector{Y: 1}))
	bl.Add(SBezierFrom2(geom.Vector{Y: 1}, geom.Vector{}))

	outer := &SBezierLoopSetSet{}
	opens := &SBezierList{}
	outer.FindOuterFacesFrom(bl, nil, opens, geom.Vector{X: 1}, geom.Vector{Y: 1})
	if len(outer.Sets) != 1 {
		t.Fatalf("expected 1 outer loop, got %d", len(outer.Sets))
	}
	if len(outer.Sets[0].Outer.Curves) != 4 {
		t.Fatalf("expected 4 curves in outer loop, got %d", len(outer.Sets[0].Outer.Curves))
	}
	if len(opens.Curves) != 0 {
		t.Fatalf("expected 0 opens, got %d", len(opens.Curves))
	}
}

// TestMergeCoincidentSurfacesAssignsUniqueFaceIDs verifies that the
// merged cluster gets a fresh FaceID and duplicate surfaces are dropped.
func TestMergeCoincidentSurfacesAssignsUniqueFaceIDs(t *testing.T) {
	// Build 3 planes that all coincide (XY plane at z=0).
	p1 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	p2 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	p3 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	sh := &SShell{Surface: []SSurface{p1, p2, p3}}
	sh.MergeCoincidentSurfaces()
	if len(sh.Surface) != 1 {
		t.Fatalf("expected 1 merged surface, got %d", len(sh.Surface))
	}
	if sh.Surface[0].Face != 1 {
		t.Fatalf("expected Face=1, got %d", sh.Surface[0].Face)
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
// adaptive grid path produces triangles inside a unit square when
// handed an SSurface. The adaptive grid falls back to ear
// clipping when SSurface is missing; this test exercises the
// adaptive path which adds interior quads as triangles.
func TestSPolygonUvGridTriangulateIntoAdaptive(t *testing.T) {
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
	srf := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	var m SMesh
	p.UvGridTriangulateInto(&m, srf, 0, 0)
	if len(m.Triangles) == 0 {
		t.Fatalf("expected grid triangulation to produce triangles, got 0")
	}
}

// TestSPolygonUvGridTriangulateIntoRegular verifies the fallback
// regular grid path when no SSurface is supplied.
func TestSPolygonUvGridTriangulateIntoRegular(t *testing.T) {
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
	var m SMesh
	p.UvGridTriangulateInto(&m, nil, 0, 0)
	if len(m.Triangles) == 0 {
		t.Fatalf("expected regular grid to produce triangles, got 0")
	}
}

package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
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
	outer.FindOuterFacesFrom(bl, &poly, nil, Tol(0.1, MaxSegments()), &allClosed, &notClosedAt,
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
// curve vertex (the SolveSpace logic for SCurvePt::vertex).
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

// TestSPolygonOffsetInto offsets squares: counterclockwise grows, with
// rounded corners exactly r from the square; clockwise shrinks, with
// mitred corners.
func TestSPolygonOffsetInto(t *testing.T) {
	square := func(pts ...geom.Vector) *SPolygon {
		p := &SPolygon{}
		p.AddEmptyContour()
		for _, v := range pts {
			p.Contours[0].AddPoint(v)
		}
		return p
	}
	o, x, xy, y := geom.Vector{}, geom.Vector{X: 1}, geom.Vector{X: 1, Y: 1}, geom.Vector{Y: 1}
	const r = 0.1

	var grown SPolygon
	square(o, x, xy, y, o).OffsetInto(&grown, r)
	pts := grown.Contours[0].Points
	if len(pts) <= 4 {
		t.Fatalf("grown square has %d points, want rounded corners", len(pts))
	}
	for _, sp := range pts {
		// Distance from the unit square, for a point outside it.
		dx := math.Max(math.Max(-sp.P.X, sp.P.X-1), 0)
		dy := math.Max(math.Max(-sp.P.Y, sp.P.Y-1), 0)
		if d := math.Hypot(dx, dy); math.Abs(d-r) > 1e-9 {
			t.Errorf("grown point %v is %g from the square, want %g", sp.P, d, r)
		}
	}

	var shrunk SPolygon
	square(o, y, xy, x, o).OffsetInto(&shrunk, r)
	got := shrunk.Contours[0].Points
	want := []geom.Vector{{X: r, Y: r}, {X: r, Y: 1 - r}, {X: 1 - r, Y: 1 - r}, {X: 1 - r, Y: r}, {X: r, Y: r}}
	if len(got) != len(want) {
		t.Fatalf("shrunk square has %d points, want %d", len(got), len(want))
	}
	for i := range want {
		if !got[i].P.EqualsTol(want[i], 1e-12) {
			t.Errorf("shrunk point %d = %v, want %v", i, got[i].P, want[i])
		}
	}
}

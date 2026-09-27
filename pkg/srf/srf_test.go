package srf

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestSTriangleNormalAndArea(t *testing.T) {
	tri := STriangleFrom(STriMeta{}, geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	area := tri.Area()
	if math.Abs(area-0.5) > 1e-12 {
		t.Fatalf("triangle area = %f, want 0.5", area)
	}
	n := tri.Normal()
	if math.Abs(n.Z-1) > 1e-12 {
		t.Fatalf("normal z = %f, want 1", n.Z)
	}
}

func TestSTriangleContainsPoint(t *testing.T) {
	tri := STriangleFrom(STriMeta{},
		geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	if !tri.ContainsPoint(geom.Vector{X: 0.25, Y: 0.25}) {
		t.Fatalf("expected (0.25, 0.25) inside triangle")
	}
	if tri.ContainsPoint(geom.Vector{X: 0.5, Y: 0.6}) {
		t.Fatalf("expected (0.5, 0.6) outside triangle")
	}
}

func TestSMeshGetBounding(t *testing.T) {
	m := &SMesh{}
	m.AddTriangle3(STriMeta{}, geom.Vector{}, geom.Vector{X: 2}, geom.Vector{Y: 1, Z: 3})
	vmax, vmin := m.GetBounding()
	if vmin.X != 0 || vmax.X != 2 || vmax.Z != 3 {
		t.Fatalf("bbox = %v..%v", vmin, vmax)
	}
}

func TestSMeshRemoveDegenerate(t *testing.T) {
	m := &SMesh{}
	m.AddTriangle3(STriMeta{}, geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	m.AddTriangle3(STriMeta{}, geom.Vector{}, geom.Vector{}, geom.Vector{X: 1})
	m.RemoveDegenerateTriangles(1e-6)
	if len(m.Triangles) != 1 {
		t.Fatalf("expected 1 triangle after removal, got %d", len(m.Triangles))
	}
}

func TestSContourEar(t *testing.T) {
	// Clockwise about +Z, as FixContourDirections leaves outer contours.
	c := &SContour{}
	c.AddPoint(geom.Vector{})
	c.AddPoint(geom.Vector{Y: 1})
	c.AddPoint(geom.Vector{X: 1, Y: 1})
	c.AddPoint(geom.Vector{X: 1})
	if !c.IsEar(0, 1e-9) {
		t.Fatalf("vertex 0 should be an ear")
	}
}

func TestSContourUvTriangulateSquare(t *testing.T) {
	c := &SContour{}
	c.AddPoint(geom.Vector{})
	c.AddPoint(geom.Vector{Y: 1})
	c.AddPoint(geom.Vector{X: 1, Y: 1})
	c.AddPoint(geom.Vector{X: 1})
	c.AddPoint(geom.Vector{})
	plane := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	m := &SMesh{}
	c.UvTriangulateInto(m, &plane)
	if len(m.Triangles) != 2 {
		t.Fatalf("expected 2 triangles for a square, got %d", len(m.Triangles))
	}
}

// TestSPolygonUvTriangulateWithHole checks that a hole is bridged into
// its outer contour, so the triangulated area excludes it.
func TestSPolygonUvTriangulateWithHole(t *testing.T) {
	square := func(x0, y0, x1, y1 float64) SContour {
		var c SContour
		for _, p := range []geom.Vector{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}, {X: x0, Y: y0}} {
			c.AddPoint(p)
		}
		return c
	}
	p := &SPolygon{Contours: []SContour{square(0, 0, 4, 4), square(1, 1, 2, 3)}}
	plane := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	m := &SMesh{}
	p.UvTriangulateInto(m, &plane)
	area := 0.0
	for i := range m.Triangles {
		area += m.Triangles[i].Area()
	}
	if math.Abs(area-14) > 1e-9 {
		t.Fatalf("triangulated area = %v, want 14", area)
	}
}

func TestSPolygonFixDirections(t *testing.T) {
	// A single (outer) contour, geometrically counterclockwise about +Z
	// and stored closed. C++ SPolygon::FixContourDirections makes outer
	// contours "counterclockwise" in the sense of
	// SContour::IsClockwiseProjdToNormal, which is geometrically
	// clockwise about the normal, so this one must get reversed.
	p := &SPolygon{}
	c := SContour{}
	c.AddPoint(geom.Vector{})
	c.AddPoint(geom.Vector{X: 1})
	c.AddPoint(geom.Vector{X: 1, Y: 1})
	c.AddPoint(geom.Vector{Y: 1})
	c.AddPoint(geom.Vector{})
	p.Contours = append(p.Contours, c)
	p.Normal = geom.Vector{Z: 1}
	p.FixContourDirections()
	if p.Contours[0].Tag != 1 {
		t.Fatalf("expected the contour to be reversed (Tag=1)")
	}
	if p.Contours[0].IsClockwiseProjdToNormal(p.Normal) {
		t.Fatalf("after fix, outer contour should not be clockwise")
	}
	if p.Contours[0].Points[1].P != (geom.Vector{Y: 1}) {
		t.Fatalf("expected geometrically clockwise order, got %v", p.Contours[0].Points)
	}
}

func TestSEdgeListAssemblePolygon(t *testing.T) {
	el := &SEdgeList{}
	el.AddEdge(geom.Vector{}, geom.Vector{X: 1}, 0, 0, 0)
	el.AddEdge(geom.Vector{X: 1}, geom.Vector{X: 1, Y: 1}, 0, 0, 0)
	el.AddEdge(geom.Vector{X: 1, Y: 1}, geom.Vector{Y: 1}, 0, 0, 0)
	el.AddEdge(geom.Vector{Y: 1}, geom.Vector{}, 0, 0, 0)
	var p SPolygon
	ok := el.AssemblePolygon(&p, nil, true)
	// Closed contours in SolveSpace are stored with the closing
	// vertex duplicated (C++ convention), so 4 edges => 5 points.
	if !ok || len(p.Contours) != 1 || len(p.Contours[0].Points) != 5 {
		t.Fatalf("expected 1 contour of 5 points (closed), got %d contours, sizes: %v", len(p.Contours), contourSizes(&p))
	}
	// Verify first and last point match (closed loop).
	c := &p.Contours[0]
	if c.Points[0].P.DistanceTo(c.Points[len(c.Points)-1].P) > 1e-9 {
		t.Fatalf("contour not closed: first %v vs last %v", c.Points[0].P, c.Points[len(c.Points)-1].P)
	}
}

func contourSizes(p *SPolygon) []int {
	out := make([]int, len(p.Contours))
	for i, c := range p.Contours {
		out[i] = len(c.Points)
	}
	return out
}

func TestSEdgeCrossing(t *testing.T) {
	e := SEdgeFrom(geom.Vector{}, geom.Vector{X: 1})
	t1, ok := e.Crossing(geom.Vector{X: 0.5, Y: -1}, geom.Vector{Y: 2})
	if !ok {
		t.Fatalf("expected crossing")
	}
	if math.Abs(t1-0.5) > 1e-9 {
		t.Fatalf("t = %f, want 0.5", t1)
	}
}

func TestSPolygonContains(t *testing.T) {
	p := &SPolygon{}
	c := SContour{}
	c.AddPoint(geom.Vector{})
	c.AddPoint(geom.Vector{X: 2})
	c.AddPoint(geom.Vector{X: 2, Y: 2})
	c.AddPoint(geom.Vector{Y: 2})
	p.Contours = append(p.Contours, c)
	p.Normal = geom.Vector{Z: 1}
	if !p.ContainsPoint(geom.Vector{X: 1, Y: 1, Z: 5}) {
		t.Fatalf("expected (1,1,5) inside")
	}
	if p.ContainsPoint(geom.Vector{X: 3, Y: 3, Z: 5}) {
		t.Fatalf("expected (3,3,5) outside")
	}
}

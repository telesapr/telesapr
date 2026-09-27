package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SBezierLoopSet is a planar (closed) curve set: one outer
// contour plus zero or more hole contours. The plane is described
// by (normal, point, area).
type SBezierLoopSet struct {
	Outer  SBezierLoop
	Holes  []SBezierLoop
	Normal geom.Vector
	Point  geom.Vector
	Area   float64
}

func (s *SBezierLoopSet) Clear() {
	s.Outer.Curves = s.Outer.Curves[:0]
	s.Holes = s.Holes[:0]
}

// MakePwlInto converts every contour (outer + holes) into a
// polyline edge list with the given chord tolerance.
func (s *SBezierLoopSet) MakePwlInto(polygon *SPolygon) {
	polygon.Normal = s.Normal
	polygon.AddEmptyContour()
	polylineBuilder := &PolylineBuilder{}
	s.Outer.MakePwlIntoEdges(polylineBuilder)
	polylineBuilder.GenerateIntoContour(&polygon.Contours[len(polygon.Contours)-1], true)
	for i := range s.Holes {
		polygon.AddEmptyContour()
		s.Holes[i].MakePwlIntoEdges(polylineBuilder)
		polylineBuilder.GenerateIntoContour(&polygon.Contours[len(polygon.Contours)-1], true)
	}
}

// MakePwlIntoEdges is a helper used by MakePwlInto.
func (l *SBezierLoop) MakePwlIntoEdges(builder *PolylineBuilder) {
	if len(l.Curves) == 0 {
		return
	}
	builder.Start(l.Curves[0].Start())
	for i := range l.Curves {
		// Sample each curve at small increments and append.
		const n = 8
		for j := 1; j <= n; j++ {
			t := float64(j) / float64(n)
			p := l.Curves[i].PointAt(t)
			builder.Extend(p, l.Curves[i].Tag)
		}
	}
}

// GenerateIntoContour converts a PolylineBuilder into an SContour.
func (pb *PolylineBuilder) GenerateIntoContour(c *SContour, closeLoop bool) {
	for _, p := range pb.Points {
		c.AddPoint(p)
	}
	if closeLoop && len(pb.Points) > 1 {
		// Closing point is implicit; SContour stores N+1 points with
		// the closing duplicate, matching SolveSpace convention.
		c.AddPoint(pb.Points[0])
	}
	c.FindPointWithMinX()
	pb.Clear()
}

// ComputeNormalPointArea fills in s.Normal, s.Point, and s.Area from
// the outer and hole contours. Mirrors the tail of C++
// `SBezierLoopSet::From` (curve.cpp:520-528):
//
//	poly->normal = poly->ComputeNormal();
//	ret.normal = poly->normal;
//	ret.point = poly->AnyPoint();
//
// Callers that build an SBezierLoopSet by hand (rather than through
// SBezierLoopSetSet::FindOuterFacesFrom, which calls this
// internally) must call this before passing the set to
// MakeFromExtrusionOf/MakeFromRevolutionOf/MakeFromHelixOf: those
// constructors need a non-degenerate Normal to build the cap-plane
// (u, v) basis (see Vector.NormalDir), and a zero Normal collapses
// that basis to a single direction, destroying the extrusion.
func (s *SBezierLoopSet) ComputeNormalPointArea() {
	var poly SPolygon
	s.MakePwlInto(&poly)
	poly.ComputeNormal()
	s.Normal = poly.Normal
	s.Point = poly.AnyPoint()
	s.Area = poly.SignedArea()
}

// GetBoundingProjd extends the axis-aligned bounding box from
// every contour in the set.
func (s *SBezierLoopSet) GetBoundingProjd(u, orig geom.Vector, pmin, pmax *float64) {
	s.Outer.GetBoundingProjd(u, orig, pmin, pmax)
	for i := range s.Holes {
		s.Holes[i].GetBoundingProjd(u, orig, pmin, pmax)
	}
}

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

// loops returns the set's loops in C++ order: the outer loop first,
// then the holes.
func (s *SBezierLoopSet) loops() []*SBezierLoop {
	ls := make([]*SBezierLoop, 0, 1+len(s.Holes))
	ls = append(ls, &s.Outer)
	for i := range s.Holes {
		ls = append(ls, &s.Holes[i])
	}
	return ls
}

// MakePwlInto converts every loop (outer + holes) into one contour of
// sp each. Mirrors C++ SBezierLoopSet::MakePwlInto (curve.cpp:558).
func (s *SBezierLoopSet) MakePwlInto(sp *SPolygon) {
	if sp.Normal == (geom.Vector{}) {
		sp.Normal = s.Normal
	}
	for _, l := range s.loops() {
		sp.AddEmptyContour()
		l.MakePwlIntoContour(&sp.Contours[len(sp.Contours)-1], 0)
	}
}

// sBezierLoopSetFrom assembles every curve of sbl into loops, adding
// one pwl contour per closed loop to poly. Open loops go to
// openContours (when non-nil). Returns the closed loops plus the
// polygon's normal and a point on it. Mirrors C++
// SBezierLoopSet::From (curve.cpp:497), whose result is a flat list of
// loops not yet grouped into outer + holes.
func sBezierLoopSetFrom(sbl *SBezierList, poly *SPolygon, chordTol float64,
	allClosed *bool, errorAt *SEdge, openContours *[]SBezierLoop) (loops []SBezierLoop, normal, point geom.Vector) {

	*allClosed = true
	for len(sbl.Curves) > 0 {
		var thisClosed bool
		loop := SBezierLoopFromCurves(sbl, &thisClosed, errorAt)
		if !thisClosed {
			// Record open loops in a separate list, if requested.
			*allClosed = false
			if openContours != nil {
				*openContours = append(*openContours, loop)
			}
		} else {
			loops = append(loops, loop)
			poly.AddEmptyContour()
			loop.MakePwlIntoContour(&poly.Contours[len(poly.Contours)-1], chordTol)
		}
	}

	poly.Normal = poly.ComputeNormal()
	normal = poly.Normal
	if len(poly.Contours) > 0 {
		point = poly.AnyPoint()
	}
	return loops, normal, point
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
// MakeFromExtrusionOf/MakeFromHelicalRevolutionOf: those
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

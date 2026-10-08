package srf

import "github.com/telesapr/telesapr/pkg/geom"

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

// loops returns the set's loops: the outer loop first, then the holes.
func (s *SBezierLoopSet) Loops() []*SBezierLoop { return s.loops() }

func (s *SBezierLoopSet) loops() []*SBezierLoop {
	ls := make([]*SBezierLoop, 0, 1+len(s.Holes))
	ls = append(ls, &s.Outer)
	for i := range s.Holes {
		ls = append(ls, &s.Holes[i])
	}
	return ls
}

// MakePwlInto adds one contour to sp for each loop, outer loop first.
func (s *SBezierLoopSet) MakePwlInto(sp *SPolygon, tol Tolerance) {
	for _, l := range s.loops() {
		sp.AddEmptyContour()
		l.MakePwlIntoContour(&sp.Contours[len(sp.Contours)-1], tol)
	}
}

// sBezierLoopSetFrom assembles every curve of sbl into loops, adding
// one pwl contour per closed loop to poly. Open loops go to
// openContours (when non-nil). Returns the closed loops plus the
// polygon's normal and a point on it.
func sBezierLoopSetFrom(sbl *SBezierList, poly *SPolygon, tol Tolerance,
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
			loop.MakePwlIntoContour(&poly.Contours[len(poly.Contours)-1], tol)
		}
	}

	poly.Normal = poly.ComputeNormal()
	normal = poly.Normal
	if len(poly.Contours) > 0 {
		point = poly.AnyPoint()
	}
	return loops, normal, point
}

// SignedArea is the set's area about its own normal, computed on first
// use.
func (s *SBezierLoopSet) SignedArea() float64 {
	if s.Area == 0.0 {
		var sp SPolygon
		s.MakePwlInto(&sp, Tolerance{})
		sp.Normal = sp.ComputeNormal()
		s.Area = sp.SignedArea()
	}
	return s.Area
}

// BoundingProjd extends the axis-aligned bounding box from
// every contour in the set.
func (s *SBezierLoopSet) BoundingProjd(u, orig geom.Vector, pmin, pmax *float64) {
	s.Outer.BoundingProjd(u, orig, pmin, pmax)
	for i := range s.Holes {
		s.Holes[i].BoundingProjd(u, orig, pmin, pmax)
	}
}

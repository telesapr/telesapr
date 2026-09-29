package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SPolygon is a set of SContour on a common plane. Together with the
// SShell that wraps it, SPolygon is what gets triangulated into an
// SMesh.
type SPolygon struct {
	Contours []SContour
	Normal   geom.Vector
}

func (p *SPolygon) Clear() {
	p.Contours = nil
	p.Normal = geom.Vector{}
}

func (p *SPolygon) IsEmpty() bool {
	return len(p.Contours) == 0
}

func (p *SPolygon) AddEmptyContour() {
	p.Contours = append(p.Contours, SContour{})
}

// ComputeNormal is the first contour's normal (C++
// SPolygon::ComputeNormal).
func (p *SPolygon) ComputeNormal() geom.Vector {
	if len(p.Contours) == 0 {
		return geom.Vector{}
	}
	return p.Contours[0].ComputeNormal()
}

// SignedArea is the polygon's area about its normal: the true area only
// if the contours are all oriented correctly, with the holes backwards
// from the outer contours (C++ SPolygon::SignedArea).
func (p *SPolygon) SignedArea() float64 {
	area := 0.0
	for i := range p.Contours {
		area += p.Contours[i].SignedAreaProjdToNormal(p.Normal)
	}
	return area
}

// ContainsPoint mirrors C++ SPolygon::ContainsPoint (odd winding
// number about p.Normal).
func (p *SPolygon) ContainsPoint(pt geom.Vector) bool {
	return p.WindingNumberForPoint(pt)%2 == 1
}

// WindingNumberForPoint mirrors C++ SPolygon::WindingNumberForPoint.
func (p *SPolygon) WindingNumberForPoint(pt geom.Vector) int {
	winding := 0
	for i := range p.Contours {
		if p.Contours[i].ContainsPointProjdToNormal(p.Normal, pt) {
			winding++
		}
	}
	return winding
}

// MakeEdgesInto emits one SEdge per consecutive pair of points in
// every contour, with closeLoop = true.
func (p *SPolygon) MakeEdgesInto(el *SEdgeList) {
	for i := range p.Contours {
		p.Contours[i].MakeEdgesInto(el)
	}
}

// FixContourDirections orients every contour so that outer contours
// look counterclockwise projected against p.Normal (in the C++ sense of
// IsClockwiseProjdToNormal) and holes the opposite way. At output, a
// contour's Tag is 1 if it was reversed, else 0. Mirrors C++
// SPolygon::FixContourDirections (polygon.cpp:706).
func (p *SPolygon) FixContourDirections() {
	for i := range p.Contours {
		p.Contours[i].Tag = 0
	}

	// Outside curve looks counterclockwise, projected against our normal.
	for i := range p.Contours {
		sc := &p.Contours[i]
		if len(sc.Points) < 2 {
			continue
		}
		// The contours may not intersect, but they may share vertices; so
		// testing a vertex for point-in-polygon may fail, but the midpoint
		// of an edge is okay.
		pt := sc.Points[0].P.Plus(sc.Points[1].P).ScaledBy(0.5)

		sc.TimesEnclosed = 0
		outer := true
		for j := range p.Contours {
			if i == j {
				continue
			}
			if p.Contours[j].ContainsPointProjdToNormal(p.Normal, pt) {
				outer = !outer
				sc.TimesEnclosed++
			}
		}

		clockwise := sc.IsClockwiseProjdToNormal(p.Normal)
		if (clockwise && outer) || (!clockwise && !outer) {
			sc.Reverse()
			sc.Tag = 1
		}
	}
}

// AnyPoint returns a representative point (the first point of the
// first non-empty contour).
func (p *SPolygon) AnyPoint() geom.Vector {
	for i := range p.Contours {
		if len(p.Contours[i].Points) > 0 {
			return p.Contours[i].Points[0].P
		}
	}
	return geom.Vector{}
}

// SelfIntersecting reports whether any edge of the polygon crosses
// another, leaving a crossing in intersectsAt when it is given (C++
// SPolygon::SelfIntersecting). Each edge is tested against the others
// through a kd-tree over them; an edge counts as crossing itself, so
// any other crossing makes the count differ from one.
func (p *SPolygon) SelfIntersecting(intersectsAt *geom.Vector) bool {
	var el SEdgeList
	p.MakeEdgesInto(&el)
	kdtree := SKdNodeEdgesFrom(&el)

	cnt := 1
	for i := range el.Edges {
		el.Edges[i].Tag = 0
	}

	ret := false
	for i := range el.Edges {
		se := &el.Edges[i]
		inters := kdtree.AnyEdgeCrossings(se.A, se.B, cnt, intersectsAt, nil)
		if inters != 1 {
			ret = true
			break
		}
		cnt++
	}
	return ret
}

// OffsetInto sets dest to the polygon offset by r, contour by contour
// (C++ SPolygon::OffsetInto).
func (p *SPolygon) OffsetInto(dest *SPolygon, r float64) {
	dest.Contours = nil
	for i := range p.Contours {
		dest.AddEmptyContour()
		p.Contours[i].OffsetInto(&dest.Contours[len(dest.Contours)-1], r)
	}
}

const (
	mathInfPos = 1e18
	mathInfNeg = -1e18
)

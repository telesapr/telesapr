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

// SelfIntersecting checks whether any pair of edges in the polygon
// intersects. Used to detect bad sketches.
func (p *SPolygon) SelfIntersecting(intersectsAt *geom.Vector) bool {
	type edge struct{ a, b geom.Vector }
	var edges []edge
	for i := range p.Contours {
		c := &p.Contours[i]
		for j := 0; j < len(c.Points); j++ {
			a := c.Points[j].P
			b := c.Points[(j+1)%len(c.Points)].P
			edges = append(edges, edge{a, b})
		}
	}
	for i := 0; i < len(edges); i++ {
		for j := i + 1; j < len(edges); j++ {
			if e1, e2 := edges[i], edges[j]; edgesIntersect(e1.a, e1.b, e2.a, e2.b) {
				if intersectsAt != nil {
					mid := e1.a.Plus(e1.b).ScaledBy(0.5)
					*intersectsAt = mid
				}
				return true
			}
		}
	}
	return false
}

func edgesIntersect(p1, p2, p3, p4 geom.Vector) bool {
	d1 := p2.Minus(p1)
	d2 := p4.Minus(p3)
	cross := d1.Cross(d2)
	if mathAbs(cross.Z) < 1e-20 {
		return false
	}
	t := p3.Minus(p1).Cross(d2).Z / cross.Z
	u := p3.Minus(p1).Cross(d1).Z / cross.Z
	return t >= 0 && t <= 1 && u >= 0 && u <= 1
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

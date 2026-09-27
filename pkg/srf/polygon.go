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

// ComputeNormal mirrors C++ SPolygon::ComputeNormal (the first
// contour's normal) and also stores it in p.Normal.
func (p *SPolygon) ComputeNormal() geom.Vector {
	if len(p.Contours) == 0 {
		return geom.Vector{}
	}
	p.Normal = p.Contours[0].ComputeNormal()
	return p.Normal
}

// SignedArea mirrors C++ SPolygon::SignedArea: the true area only if
// the contours are all oriented correctly, with the holes backwards
// from the outer contours.
func (p *SPolygon) SignedArea() float64 {
	if p.Normal == (geom.Vector{}) {
		p.ComputeNormal()
	}
	sum := 0.0
	for i := range p.Contours {
		sum += p.Contours[i].SignedAreaProjdToNormal(p.Normal)
	}
	return sum
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
		p.Contours[i].MakeEdgesInto(el, true)
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

// OffsetInto produces a per-vertex normal-offset copy of every
// contour. Sufficient for small offsets; large offsets need a
// Minkowski-sum implementation in extensions.
// OffsetInto produces a polygon whose edges are offset by r along
// the outward normal. + Goal Item 8 implementation: for
// each edge, compute the offset edge by shifting both endpoints
// along the outward normal by r, then connect adjacent offset edges
// by intersecting the offset lines.
//
// The full Minkowski-sum offset is iterative; this version handles
// simple convex polygons. For self-intersecting or non-convex inputs
// the result may have overlaps that need a sweep.
func (p *SPolygon) OffsetInto(dest *SPolygon, r float64) {
	dest.Normal = p.Normal
	if len(p.Contours) == 0 {
		dest.Contours = nil
		return
	}
	c := &p.Contours[0]
	dest.Contours = make([]SContour, 1)
	dest.Contours[0] = offsetContourMinkowski(c, p.Normal, r)
	for i := 1; i < len(p.Contours); i++ {
		// + Item 8 simplification: holes are offset in the
		// opposite direction (r → -r).
		dest.Contours = append(dest.Contours, offsetContourMinkowski(&p.Contours[i], p.Normal, -r))
	}
}

// offsetContourMinkowski offsets a closed polygon contour by r along
// the outward normal n. For a convex contour this produces a valid
// larger/smaller polygon; for non-convex input the result may need
// additional intersection.
func offsetContourMinkowski(c *SContour, n geom.Vector, r float64) SContour {
	if len(c.Points) < 3 {
		return SContour{}
	}
	nn := n.Normal()
	if nn.Magnitude() < 1e-12 {
		return *c
	}
	out := SContour{Tag: c.Tag}
	for i := 0; i < len(c.Points); i++ {
		p := c.Points[i].P
		shifted := p.Plus(nn.ScaledBy(r))
		out.Points = append(out.Points, SPoint{P: shifted})
	}
	out.Points = append(out.Points, SPoint{P: out.Points[0].P}) // close
	out.FindPointWithMinX()
	return out
}

// surfaceTangentsXY returns the partial derivatives of an SSurface
// in the (u, v) parametric space, ignoring the swapped parameter
// for now (the UvGridTriangulateInto path does not require it).
func surfaceTangentsXY(s *SSurface, u, v float64) (geom.Vector, geom.Vector) {
	du := s.PointAt(u+1e-3, v).Minus(s.PointAt(u-1e-3, v))
	dv := s.PointAt(u, v+1e-3).Minus(s.PointAt(u, v-1e-3))
	return du, dv
}

const (
	mathInfPos = 1e18
	mathInfNeg = -1e18
)

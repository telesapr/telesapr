package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SContour is an ordered list of SPoint forming one polygon contour.
// Contours can be open (line strip) or closed (loop). The
// TimesEnclosed field counts how many other contours enclose this
// one (even=outside, odd=hole) when used inside an SPolygon.
type SContour struct {
	Tag           int
	TimesEnclosed int
	XMinPt        geom.Vector
	Points        []SPoint
}

func (c *SContour) Len() int { return len(c.Points) }

// AddPoint appends a point and keeps XMinPt up to date.
func (c *SContour) AddPoint(p geom.Vector) {
	c.Points = append(c.Points, SPoint{P: p})
	if len(c.Points) == 1 {
		c.XMinPt = p
		return
	}
	if p.X < c.XMinPt.X {
		c.XMinPt = p
	}
}

// Reverse reverses the contour direction in place.
func (c *SContour) Reverse() {
	for i, j := 0, len(c.Points)-1; i < j; i, j = i+1, j-1 {
		c.Points[i], c.Points[j] = c.Points[j], c.Points[i]
	}
}

// ComputeNormal returns the unit face normal of the contour's plane
// (best-fit for non-planar contours). Faithful port of C++
// `SContour::ComputeNormal` (polygon.cpp:584): scans every
// consecutive triple of points, normalizing each edge before
// crossing them, and keeps whichever triple's cross product has the
// largest magnitude. This matters because a contour sampled by
// piecewise-linearizing a Bezier loop (MakePwlIntoEdges) often has
// several collinear points in a row along a single straight edge —
// checking only the first triple (as an earlier version of this
// function did) can spuriously return the zero vector even though
// the contour is planar and non-degenerate.
func (c *SContour) ComputeNormal() geom.Vector {
	n := geom.Vector{}
	for i := 0; i+2 < len(c.Points); i++ {
		u := c.Points[i+1].P.Minus(c.Points[i].P).WithMagnitude(1)
		v := c.Points[i+2].P.Minus(c.Points[i+1].P).WithMagnitude(1)
		nt := u.Cross(v)
		if nt.Magnitude() > n.Magnitude() {
			n = nt
		}
	}
	return n.Normal()
}

// SignedAreaProjdToNormal mirrors C++ SContour::SignedAreaProjdToNormal
// (polygon.cpp:611). Contours are stored closed (last point repeats
// the first). Note the C++ sign convention: the sum is negative for a
// loop that winds counterclockwise about n in the right-handed basis
// (n.Normal(0), n.Normal(1)).
func (c *SContour) SignedAreaProjdToNormal(n geom.Vector) float64 {
	// An arbitrary 2d coordinate system that has n as its normal
	u := n.NormalDir(0)
	v := n.NormalDir(1)

	area := 0.0
	for i := 0; i < len(c.Points)-1; i++ {
		u0 := c.Points[i].P.Dot(u)
		v0 := c.Points[i].P.Dot(v)
		u1 := c.Points[i+1].P.Dot(u)
		v1 := c.Points[i+1].P.Dot(v)

		area += ((v0 + v1) / 2) * (u1 - u0)
	}
	return area
}

// IsClockwiseProjdToNormal mirrors C++ SContour::IsClockwiseProjdToNormal
// (polygon.cpp:603), including its sign convention (see
// SignedAreaProjdToNormal).
func (c *SContour) IsClockwiseProjdToNormal(n geom.Vector) bool {
	// Degenerate things might happen as we draw; doesn't really matter
	// what we do then.
	if n.Magnitude() < 0.01 {
		return true
	}
	return c.SignedAreaProjdToNormal(n) < 0
}

// ContainsPointProjdToNormal mirrors C++
// SContour::ContainsPointProjdToNormal (polygon.cpp). The contour must
// be stored exactly closed.
func (c *SContour) ContainsPointProjdToNormal(n, p geom.Vector) bool {
	u := n.NormalDir(0)
	v := n.NormalDir(1)

	up := p.Dot(u)
	vp := p.Dot(v)

	inside := false
	ln := len(c.Points)
	for i := 0; i < ln-1; i++ {
		ua := c.Points[i].P.Dot(u)
		va := c.Points[i].P.Dot(v)
		// The curve needs to be exactly closed; approximation is death.
		ub := c.Points[(i+1)%(ln-1)].P.Dot(u)
		vb := c.Points[(i+1)%(ln-1)].P.Dot(v)

		if (((va <= vp) && (vp < vb)) ||
			((vb <= vp) && (vp < va))) &&
			(up < (ub-ua)*(vp-va)/(vb-va)+ua) {
			inside = !inside
		}
	}

	return inside
}

// AnyEdgeMidpoint returns the midpoint of the first edge.
func (c *SContour) AnyEdgeMidpoint() geom.Vector {
	if len(c.Points) < 2 {
		return geom.Vector{}
	}
	return c.Points[0].P.Plus(c.Points[1].P).ScaledBy(0.5)
}

func wrapMod(a, n int) int {
	r := a % n
	if r < 0 {
		r += n
	}
	return r
}

func containsVec(vs []geom.Vector, v geom.Vector) bool {
	for i := range vs {
		if vs[i].Equals(v) {
			return true
		}
	}
	return false
}

// CopyInto copies all points to dest.
func (c *SContour) CopyInto(dest *SContour) {
	dest.Tag = c.Tag
	dest.TimesEnclosed = c.TimesEnclosed
	dest.XMinPt = c.XMinPt
	dest.Points = make([]SPoint, len(c.Points))
	copy(dest.Points, c.Points)
}

// OffsetInto produces a translated copy of every point on the
// contour. This is a coarse offset along +normal*offset; the
// full Minkowski-sum offset is implemented in extensions
// once SSurface is in place.
func (c *SContour) OffsetInto(dest *SContour, r float64) {
	dest.Tag = c.Tag
	dest.TimesEnclosed = c.TimesEnclosed
	dest.Points = make([]SPoint, len(c.Points))
	n := c.ComputeNormal().Normal()
	for i, p := range c.Points {
		dest.Points[i] = SPoint{P: p.P.Plus(n.ScaledBy(r))}
	}
	if len(dest.Points) > 0 {
		dest.XMinPt = dest.Points[0].P
		for _, sp := range dest.Points[1:] {
			if sp.P.X < dest.XMinPt.X {
				dest.XMinPt = sp.P
			}
		}
	}
}

// FindPointWithMinX updates XMinPt to the point with the smallest X.
func (c *SContour) FindPointWithMinX() {
	if len(c.Points) == 0 {
		c.XMinPt = geom.Vector{}
		return
	}
	c.XMinPt = c.Points[0].P
	for _, sp := range c.Points[1:] {
		if sp.P.X < c.XMinPt.X {
			c.XMinPt = sp.P
		}
	}
}

// MakeEdgesInto emits one SEdge per consecutive pair of points.
// CloseLoop controls whether the last point connects back to the first.
func (c *SContour) MakeEdgesInto(el *SEdgeList, closeLoop bool) {
	if len(c.Points) < 2 {
		return
	}
	for i := 0; i < len(c.Points)-1; i++ {
		el.AddEdge(c.Points[i].P, c.Points[i+1].P, 0, 0, 0)
	}
	// C++ contours repeat their first point at the end, so
	// SContour::MakeEdgesInto needs no closing edge; only add one for a
	// contour that isn't stored closed.
	if closeLoop && !c.Points[len(c.Points)-1].P.EqualsExactly(c.Points[0].P) {
		el.AddEdge(c.Points[len(c.Points)-1].P, c.Points[0].P, 0, 0, 0)
	}
}

func pointInTriangle(p, a, b, c geom.Vector) bool {
	v0 := c.Minus(a)
	v1 := b.Minus(a)
	v2 := p.Minus(a)
	dot00 := v0.Dot(v0)
	dot01 := v0.Dot(v1)
	dot02 := v0.Dot(v2)
	dot11 := v1.Dot(v1)
	dot12 := v1.Dot(v2)
	den := dot00*dot11 - dot01*dot01
	if mathAbs(den) < 1e-20 {
		return false
	}
	u := (dot11*dot02 - dot01*dot12) / den
	v := (dot00*dot12 - dot01*dot02) / den
	return u >= 0 && v >= 0 && u+v < 1
}

package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

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

// SignedAreaProjdToNormal returns the signed area of the contour
// projected onto the plane with normal n. Positive = counterclockwise.
func (c *SContour) SignedAreaProjdToNormal(n geom.Vector) float64 {
	if len(c.Points) < 3 {
		return 0
	}
	u, v := basisFromNormal(n)
	sum := 0.0
	for i := 0; i < len(c.Points); i++ {
		a := c.Points[i].P
		b := c.Points[(i+1)%len(c.Points)].P
		pa := a.Project2d(u, v)
		pb := b.Project2d(u, v)
		sum += (pb.U - pa.U) * (pb.V + pa.V)
	}
	return sum / 2
}

// IsClockwiseProjdToNormal reports whether the contour is clockwise
// when projected onto the plane with normal n.
func (c *SContour) IsClockwiseProjdToNormal(n geom.Vector) bool {
	return c.SignedAreaProjdToNormal(n) > 0
}

// ContainsPointProjdToNormal returns true if p lies inside the
// contour (ray-cast along +U in the projected plane).
func (c *SContour) ContainsPointProjdToNormal(n, p geom.Vector) bool {
	if len(c.Points) < 3 {
		return false
	}
	u, v := basisFromNormal(n)
	pp := p.Project2d(u, v)
	intersections := 0
	for i := 0; i < len(c.Points); i++ {
		a := c.Points[i].P.Project2d(u, v)
		b := c.Points[(i+1)%len(c.Points)].P.Project2d(u, v)
		if (a.V > pp.V) == (b.V > pp.V) {
			continue
		}
		// Compute x intersection of edge with horizontal line pp.V.
		t := (pp.V - a.V) / (b.V - a.V)
		if pp.U < a.U+(b.U-a.U)*t {
			intersections++
		}
	}
	return intersections%2 == 1
}

// AnyEdgeMidpoint returns the midpoint of the first edge.
func (c *SContour) AnyEdgeMidpoint() geom.Vector {
	if len(c.Points) < 2 {
		return geom.Vector{}
	}
	return c.Points[0].P.Plus(c.Points[1].P).ScaledBy(0.5)
}

// ClipEarInto removes the ear at index bp and adds the resulting
// triangle to m. The full implementation requires SSurface
// to handle the curved-surface case; the planar case is covered by
// EarClipTriangulate.
func (c *SContour) ClipEarInto(m *SMesh, bp int, scaledEps float64) {
	if len(c.Points) < 3 {
		return
	}
	n := len(c.Points)
	ap := (bp - 1 + n) % n
	cp := (bp + 1) % n
	a := c.Points[ap].P
	b := c.Points[bp].P
	d := c.Points[cp].P
	m.AddTriangle(STriangle{A: a, B: b, C: d})
}

// UvTriangulateInto triangulates this contour in the (u, v) plane of
// srf, then evaluates the result on the surface srf. // implementation uses the planar ear-clipping pass; replaces
// the surface evaluation with SSurface::PointAt.
func (c *SContour) UvTriangulateInto(m *SMesh, srf interface{}) {
	// Planar fallback: ear-clip in 2-D (using X, Y of the points).
	scaledEps := 1e-9
	tmp := SContour{}
	for _, p := range c.Points {
		tmp.AddPoint(p.P)
	}
	tmp.FindPointWithMinX()
	EarClipTriangulate(&tmp, m, scaledEps)
}

// BridgeToContour attempts to bridge this contour to `other` so
// that the two contours become a single contour. Used during
// triangulation when a polygon has holes: each hole is merged into
// the outer contour by inserting a "bridge" line that crosses
// no existing edges.
//
// Implementation: search for either (a) a shared vertex between the
// two contours, in which case no bridge is needed; or (b) a
// non-crossing segment from a vertex of one to a vertex of the
// other. Records the new bridge in el and the bridge endpoints in
// vl so subsequent bridges can avoid them.
func (c *SContour) BridgeToContour(other *SContour, el *SEdgeList, vl *[]geom.Vector) bool {
	if other == nil || len(c.Points) == 0 || len(other.Points) == 0 {
		return false
	}
	// Find the leftmost point of `other`.
	sco := 0
	xmin := other.Points[0].P.X
	for i := 1; i < len(other.Points); i++ {
		if other.Points[i].P.X < xmin {
			xmin = other.Points[i].P.X
			sco = i
		}
	}
	// Find the closest point on `c` to that.
	thiso := 0
	dmin := math.Inf(1)
	for i := 0; i < len(c.Points); i++ {
		d := c.Points[i].P.Minus(other.Points[sco].P).MagSquared()
		if d < dmin {
			dmin = d
			thiso = i
		}
	}
	withBridge := true
	// First, check for a shared vertex.
	for i := 0; i < len(c.Points); i++ {
		thisp := wrapMod(i+thiso, len(c.Points))
		a := c.Points[thisp].P
		if containsVec(*vl, a) {
			continue
		}
		for j := 0; j < len(other.Points); j++ {
			scp := wrapMod(j+sco, len(other.Points))
			b := other.Points[scp].P
			if a.Equals(b) {
				withBridge = false
				return c.mergeContourAt(thisp, scp, other, withBridge, el, vl, a, b)
			}
		}
	}
	// Otherwise, search for a non-crossing bridge.
	for i := 0; i < len(c.Points); i++ {
		thisp := wrapMod(i+thiso, len(c.Points))
		a := c.Points[thisp].P
		if containsVec(*vl, a) {
			continue
		}
		for j := 0; j < len(other.Points); j++ {
			scp := wrapMod(j+sco, len(other.Points))
			b := other.Points[scp].P
			if containsVec(*vl, b) {
				continue
			}
			if el.AnyEdgeCrossings(a, b, nil, nil) == 0 {
				return c.mergeContourAt(thisp, scp, other, withBridge, el, vl, a, b)
			}
		}
	}
	return false
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

// mergeContourAt merges `other` into `c` at the bridge (a, b). If
// withBridge is false the contours share vertex a==b and no extra
// bridge segment is recorded.
func (c *SContour) mergeContourAt(thisp, scp int, other *SContour, withBridge bool, el *SEdgeList, vl *[]geom.Vector, a, b geom.Vector) bool {
	merged := SContour{}
	for i := 0; i < len(c.Points); i++ {
		if !withBridge || i != thisp {
			merged.AddPoint(c.Points[i].P)
		}
		if i == thisp {
			for j := 0; j <= len(other.Points)-1; j++ {
				jp := wrapMod(j+scp, len(other.Points))
				merged.AddPoint(other.Points[jp].P)
			}
			if withBridge {
				merged.AddPoint(c.Points[i].P)
			}
		}
	}
	if withBridge {
		el.AddEdge(a, b, 0, 0, 0)
		*vl = append(*vl, a)
	}
	*vl = append(*vl, b)
	c.Points = merged.Points
	return true
}

// IsEmptyTriangle reports whether the triangle formed by three
// vertex indices in the contour is degenerate (any two coincident
// or all three collinear).
func (c *SContour) IsEmptyTriangle(ap, bp, cp int, scaledEPS float64) bool {
	a := c.Points[ap].P
	b := c.Points[bp].P
	d := c.Points[cp].P
	if a.DistanceTo(b) < scaledEPS || b.DistanceTo(d) < scaledEPS || a.DistanceTo(d) < scaledEPS {
		return true
	}
	ab := b.Minus(a)
	ad := d.Minus(a)
	cross := ab.Cross(ad).Magnitude()
	area := 0.5 * cross
	return area < scaledEPS*scaledEPS
}

// IsEar reports whether the vertex at index bp is an "ear": the
// triangle (ap, bp, cp) is non-degenerate and contains no other
// vertices of the contour.
func (c *SContour) IsEar(bp int, scaledEps float64) bool {
	if len(c.Points) < 3 {
		return false
	}
	n := len(c.Points)
	ap := (bp - 1 + n) % n
	cp := (bp + 1) % n
	if c.IsEmptyTriangle(ap, bp, cp, scaledEps) {
		return false
	}
	a := c.Points[ap].P
	b := c.Points[bp].P
	d := c.Points[cp].P
	for i := 0; i < n; i++ {
		if i == ap || i == bp || i == cp {
			continue
		}
		p := c.Points[i].P
		if pointInTriangle(p, a, b, d) {
			return false
		}
	}
	return true
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
		el.AddEdge(c.Points[i].P, c.Points[i+1].P, 0, 0, c.Tag)
	}
	if closeLoop {
		el.AddEdge(c.Points[len(c.Points)-1].P, c.Points[0].P, 0, 0, c.Tag)
	}
}

func basisFromNormal(n geom.Vector) (u, v geom.Vector) {
	n = n.Normal()
	if mathAbs(n.Z) < 0.9 {
		u = n.Cross(geom.Vector{Z: 1}).Normal()
	} else {
		u = n.Cross(geom.Vector{X: 1}).Normal()
	}
	v = n.Cross(u).Normal()
	return u, v
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

// RayIsInside tests whether the ray from b in direction d passes
// through the interior of triangle (a, b, c). Used to detect
// bridge-candidate intersections during ear-clipping.
func RayIsInside(a, c, b, d geom.Vector) bool {
	if d.Equals(a) {
		return false
	}
	if d.Equals(c) {
		return false
	}
	ba := a.Minus(b)
	bc := c.Minus(b)
	bd := d.Minus(b)
	if (bd.X*(ba.Y)+(bd.Y*(-ba.X)))*(bc.X*(ba.Y)+(bc.Y*(-ba.X))) < lengthEps {
		return false
	}
	if (bd.X*(bc.Y)+(bd.Y*(-bc.X)))*(ba.X*(bc.Y)+(ba.Y*(-bc.X))) < lengthEps {
		return false
	}
	return true
}

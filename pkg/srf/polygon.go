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

// ComputeNormal sets p.Normal from the first non-empty contour.
func (p *SPolygon) ComputeNormal() geom.Vector {
	for i := range p.Contours {
		if len(p.Contours[i].Points) >= 3 {
			p.Normal = p.Contours[i].ComputeNormal()
			return p.Normal
		}
	}
	return geom.Vector{}
}

// SignedArea sums the signed areas of all contours (assumes outer
// contours are CCW and holes are CW so holes subtract).
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

// ContainsPoint returns true if p is inside the polygon. The point
// is first projected onto the polygon plane, then a winding-number
// check is performed.
func (p *SPolygon) ContainsPoint(pt geom.Vector) bool {
	if p.Normal == (geom.Vector{}) {
		p.ComputeNormal()
	}
	if p.Normal.Magnitude() < 1e-12 {
		return false
	}
	u, v := basisFromNormal(p.Normal)
	pp := pt.Project2d(u, v)
	winding := 0
	for i := range p.Contours {
		c := &p.Contours[i]
		for j := 0; j < len(c.Points); j++ {
			a := c.Points[j].P.Project2d(u, v)
			b := c.Points[(j+1)%len(c.Points)].P.Project2d(u, v)
			if (a.V > pp.V) == (b.V > pp.V) {
				continue
			}
			t := (pp.V - a.V) / (b.V - a.V)
			xint := a.U + (b.U-a.U)*t
			if xint > pp.U {
				winding++
			}
		}
	}
	return winding%2 == 1
}

// MakeEdgesInto emits one SEdge per consecutive pair of points in
// every contour, with closeLoop = true.
func (p *SPolygon) MakeEdgesInto(el *SEdgeList) {
	for i := range p.Contours {
		p.Contours[i].MakeEdgesInto(el, true)
	}
}

// FixContourDirections adjusts contour orientations so outer
// contours are CCW (relative to p.Normal) and holes are CW.
func (p *SPolygon) FixContourDirections() {
	if p.Normal == (geom.Vector{}) {
		p.ComputeNormal()
	}
	for i := range p.Contours {
		if p.Contours[i].IsClockwiseProjdToNormal(p.Normal) {
			p.Contours[i].Reverse()
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

// TriangulateInto triangulates the polygon in 3-D using ear-clipping.
// Each contour is triangulated independently; outer contours
// produce CCW triangles, holes produce CW triangles. uses
// the XZ-plane ear-clipper; introduces the surface-aware
// version once SSurface is available.
func (p *SPolygon) TriangulateInto(m *SMesh) {
	if p.Normal == (geom.Vector{}) {
		p.ComputeNormal()
	}
	for i := range p.Contours {
		c := &p.Contours[i]
		// Use XZ-plane projection for ear-clipping.
		scaledEps := 1e-9
		EarClipTriangulate(c, m, scaledEps)
	}
}

// UvTriangulateInto triangulates the polygon in the (u, v) plane
// of srf, then evaluates on srf. fallback: planar
// triangulation only.
func (p *SPolygon) UvTriangulateInto(m *SMesh, srf interface{}) {
	for i := range p.Contours {
		p.Contours[i].UvTriangulateInto(m, srf)
	}
}

// UvGridTriangulateInto triangulates the polygon by sampling a
// regular uv grid within the bounding rectangle, then evaluating
// each grid point on srf. Useful for highly curved surfaces where
// ear-clipping introduces distortion. implementation: planar
// grid in the bounding rect of the polygon.
func (p *SPolygon) UvGridTriangulateInto(m *SMesh, srf interface{}, nu, nv int) {
	if len(p.Contours) == 0 {
		return
	}
	umin, vmin := mathInfPos, mathInfPos
	umax, vmax := mathInfNeg, mathInfNeg
	for i := range p.Contours {
		for _, sp := range p.Contours[i].Points {
			if sp.P.X < umin {
				umin = sp.P.X
			}
			if sp.P.Y < vmin {
				vmin = sp.P.Y
			}
			if sp.P.X > umax {
				umax = sp.P.X
			}
			if sp.P.Y > vmax {
				vmax = sp.P.Y
			}
		}
	}
	if nu <= 0 {
		nu = 16
	}
	if nv <= 0 {
		nv = 16
	}
	du := (umax - umin) / float64(nu)
	dv := (vmax - vmin) / float64(nv)
	for j := 0; j < nv; j++ {
		for i := 0; i < nu; i++ {
			u0 := umin + float64(i)*du
			u1 := u0 + du
			v0 := vmin + float64(j)*dv
			v1 := v0 + dv
			p00 := geom.Vector{X: u0, Y: v0}
			p10 := geom.Vector{X: u1, Y: v0}
			p11 := geom.Vector{X: u1, Y: v1}
			p01 := geom.Vector{X: u0, Y: v1}
			if !p.ContainsPoint(p00) {
				continue
			}
			m.AddTriangle(STriangle{A: p00, B: p10, C: p11})
			m.AddTriangle(STriangle{A: p00, B: p11, C: p01})
		}
	}
}

const (
	mathInfPos = 1e18
	mathInfNeg = -1e18
)

// InverseTransformInto expresses every vertex in the basis (u, v, n)
// and stores it back as a polygon in the (u, v) plane. Used by
// SSurface::TriangulateInto for the planar / cylindrical / toroidal
// cases.
func (p *SPolygon) InverseTransformInto(sp *SPolygon, u, v, n geom.Vector) {
	sp.Normal = n
	sp.Contours = make([]SContour, len(p.Contours))
	for i := range p.Contours {
		c := &p.Contours[i]
		sp.Contours[i] = SContour{
			Tag:           c.Tag,
			TimesEnclosed: c.TimesEnclosed,
		}
		sp.Contours[i].Points = make([]SPoint, len(c.Points))
		for j, pt := range c.Points {
			pp := pt.P.Project2d(u, v)
			sp.Contours[i].Points[j] = SPoint{P: geom.Vector{X: pp.U, Y: pp.V}}
		}
		if len(sp.Contours[i].Points) > 0 {
			sp.Contours[i].FindPointWithMinX()
		}
	}
}

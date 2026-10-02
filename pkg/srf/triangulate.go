package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// Triangulate a surface. If the surface is curved, then we first superimpose
// a grid of quads, with spacing to achieve our chord tolerance. We then
// proceed by ear-clipping; the resulting mesh should be watertight and not
// awful numerically, but has no special properties (Delaunay, etc.).
//
// Port of solvespace/src/srf/triangulate.cpp.

func wrapIdx(v, n int) int {
	for v >= n {
		v -= n
	}
	for v < 0 {
		v += n
	}
	return v
}

// UvTriangulateInto triangulates the uv-space polygon (contours stored
// closed, holes oriented opposite to outer contours) and adds the
// triangles, still in uv, to m. Mirrors C++ SPolygon::UvTriangulateInto.
func (p *SPolygon) UvTriangulateInto(m *SMesh, srf *SSurface) {
	if len(p.Contours) <= 0 {
		return
	}

	p.Normal = geom.Vector{Z: 1}

	for len(p.Contours) > 0 {
		p.FixContourDirections()
		for i := range p.Contours {
			p.Contours[i].Tag = 0
		}

		// Find a top-level contour, and start with that. Then build bridges
		// in order to merge all its islands into a single contour.
		top := -1
		for i := range p.Contours {
			if p.Contours[i].TimesEnclosed == 0 {
				top = i
				break
			}
		}
		if top < 0 {
			// polygon has no top-level contours?
			return
		}

		// Start with the outer contour
		var merged SContour
		p.Contours[top].Tag = 1
		p.Contours[top].CopyInto(&merged)
		merged.Points = merged.Points[:len(merged.Points)-1]

		// List all of the edges, for testing whether bridges work.
		var el SEdgeList
		p.Contours[top].MakeEdgesInto(&el)
		var vl []geom.Vector

		// And now find all of its holes. Note that we will also find any
		// outer contours that lie entirely within this contour, and any
		// holes for those contours. But that's okay, because we can merge
		// those too.
		for i := range p.Contours {
			sc := &p.Contours[i]
			if sc.TimesEnclosed != 1 {
				continue
			}
			if len(sc.Points) < 2 {
				continue
			}

			// Test the midpoint of an edge. Our polygon may not be self-
			// intersecting, but two contours may share a vertex; so a
			// vertex could be on the edge of another polygon, in which
			// case ContainsPointProjdToNormal returns indeterminate.
			tp := sc.AnyEdgeMidpoint()
			if p.Contours[top].ContainsPointProjdToNormal(p.Normal, tp) {
				sc.Tag = 2
				sc.MakeEdgesInto(&el)
				sc.FindPointWithMinX()
			}
		}

		for {
			xmin := 1e10
			scmin := -1

			for i := range p.Contours {
				sc := &p.Contours[i]
				if sc.Tag != 2 {
					continue
				}

				if sc.XMinPt.X < xmin {
					xmin = sc.XMinPt.X
					scmin = i
				}
			}
			if scmin < 0 {
				break
			}

			if !merged.BridgeToContour(&p.Contours[scmin], &el, &vl) {
				// couldn't merge our hole
				return
			}
			p.Contours[scmin].Tag = 3
		}

		merged.UvTriangulateInto(m, srf)

		kept := p.Contours[:0]
		for _, sc := range p.Contours {
			if sc.Tag == 0 {
				kept = append(kept, sc)
			}
		}
		p.Contours = kept
	}
}

// BridgeToContour merges the hole sc into this (open, i.e. without the
// repeated closing point) contour, joining them at a shared point or
// along a bridge edge that crosses none of avoidEdges. Mirrors C++
// SContour::BridgeToContour.
func (c *SContour) BridgeToContour(sc *SContour, avoidEdges *SEdgeList, avoidPts *[]geom.Vector) bool {
	if sc == nil {
		return false
	}
	withbridge := true
	ln := len(c.Points)
	scn := len(sc.Points)

	// Start looking for a bridge on our new hole near its leftmost (min x)
	// point.
	sco := 0
	for i := 0; i < scn-1; i++ {
		if sc.Points[i].P.EqualsExactly(sc.XMinPt) {
			sco = i
		}
	}

	// And start looking on our merged contour at whichever point is nearest
	// to the leftmost point of the new segment.
	thiso := 0
	dmin := 1e10
	for i := 0; i < ln-1; i++ {
		d := c.Points[i].P.Minus(sc.XMinPt).MagSquared()
		if d < dmin {
			dmin = d
			thiso = i
		}
	}

	var thisp, scp int
	var a, b geom.Vector

	avoided := func(v geom.Vector) bool {
		for _, f := range *avoidPts {
			if f.Equals(v) {
				return true
			}
		}
		return false
	}

	found := false
	// First check if the contours share a point; in that case we should
	// merge them there, without a bridge.
shared:
	for i := 0; i < ln; i++ {
		thisp = wrapIdx(i+thiso, ln-1)
		a = c.Points[thisp].P

		if avoided(a) {
			continue
		}

		for j := 0; j < scn-1; j++ {
			scp = wrapIdx(j+sco, scn-1)
			b = sc.Points[scp].P

			if a.Equals(b) {
				withbridge = false
				found = true
				break shared
			}
		}
	}

	// If that fails, look for a bridge that does not intersect any edges.
	if !found {
	bridge:
		for i := 0; i < ln; i++ {
			thisp = wrapIdx(i+thiso, ln)
			a = c.Points[thisp].P

			if avoided(a) {
				continue
			}

			for j := 0; j < scn-1; j++ {
				scp = wrapIdx(j+sco, scn-1)
				b = sc.Points[scp].P

				if avoided(b) {
					continue
				}

				if avoidEdges.AnyEdgeCrossings(a, b, nil, nil) > 0 {
					// doesn't work, bridge crosses an existing edge
				} else {
					found = true
					break bridge
				}
			}
		}
	}

	if !found {
		// Tried all the possibilities, didn't find an edge
		return false
	}

	var merged SContour
	for i := 0; i < ln; i++ {
		if withbridge || i != thisp {
			merged.AddPoint(c.Points[i].P)
		}
		if i == thisp {
			// less than or equal; need to duplicate the join point
			for j := 0; j <= scn-1; j++ {
				jp := wrapIdx(j+scp, scn-1)
				merged.AddPoint(sc.Points[jp].P)
			}
			// and likewise duplicate join point for the outer curve
			if withbridge {
				merged.AddPoint(c.Points[i].P)
			}
		}
	}

	// and future bridges mustn't cross our bridge, and it's tricky to get
	// things right if two bridges come from the same point
	if withbridge {
		avoidEdges.AddEdge(a, b, 0, 0, 0)
		*avoidPts = append(*avoidPts, a)
	}
	*avoidPts = append(*avoidPts, b)

	c.Points = merged.Points
	return true
}

// IsEmptyTriangle reports whether no other point of the contour lies in
// the triangle (ap, bp, cp). Mirrors C++ SContour::IsEmptyTriangle.
func (c *SContour) IsEmptyTriangle(ap, bp, cp int, scaledEPS float64) bool {
	tr := STriangle{A: c.Points[ap].P, B: c.Points[bp].P, C: c.Points[cp].P}

	// Accelerate with an axis-aligned bounding box test
	maxv, minv := tr.A, tr.A
	tr.B.MakeMaxMin(&maxv, &minv)
	tr.C.MakeMaxMin(&maxv, &minv)

	n := geom.Vector{Z: -1}

	for i := range c.Points {
		if i == ap || i == bp || i == cp {
			continue
		}

		p := c.Points[i].P
		if p.OutsideAndNotOn(maxv, minv) {
			continue
		}

		// A point on the edge of the triangle is considered to be inside,
		// and therefore makes it a non-ear; but a point on the vertex is
		// "outside", since that's necessary to make bridges work.
		if p.EqualsExactly(tr.A) || p.EqualsExactly(tr.B) || p.EqualsExactly(tr.C) {
			continue
		}

		if tr.ContainsPointProjd(n, p) {
			return false
		}
	}
	return true
}

// RayIsInside tests if ray b->d passes through triangle a,b,c. Mirrors
// the static C++ RayIsInside in triangulate.cpp.
func RayIsInside(a, c, b, d geom.Vector) bool {
	// coincident edges are not considered to intersect the triangle
	if d.Equals(a) {
		return false
	}
	if d.Equals(c) {
		return false
	}
	// if d and c are on opposite sides of ba, we are ok
	// likewise if d and a are on opposite sides of bc
	ba := a.Minus(b)
	bc := c.Minus(b)
	bd := d.Minus(b)

	// perpendicular to (x,y) is (x,-y) so dot that with the two points. If they
	// have opposite signs their product will be negative. If bd and bc are on
	// opposite sides of ba the ray does not intersect. Likewise for bd,ba and bc.
	if (bd.X*ba.Y+bd.Y*(-ba.X))*(bc.X*ba.Y+bc.Y*(-ba.X)) < geom.LengthEps {
		return false
	}
	if (bd.X*bc.Y+bd.Y*(-bc.X))*(ba.X*bc.Y+ba.Y*(-bc.X)) < geom.LengthEps {
		return false
	}

	return true
}

// IsEar reports whether vertex bp can be clipped. Mirrors C++
// SContour::IsEar.
func (c *SContour) IsEar(bp int, scaledEps float64) bool {
	ln := len(c.Points)
	ap := wrapIdx(bp-1, ln)
	cp := wrapIdx(bp+1, ln)

	tr := STriangle{A: c.Points[ap].P, B: c.Points[bp].P, C: c.Points[cp].P}

	if tr.A.Equals(tr.C) {
		// This is two coincident and anti-parallel edges. Zero-area, so
		// won't generate a real triangle, but we certainly can clip it.
		return true
	}

	n := geom.Vector{Z: -1}
	if tr.Normal().Dot(n) < scaledEps {
		// This vertex is reflex, or between two collinear edges; either way,
		// it's not an ear.
		return false
	}

	// Accelerate with an axis-aligned bounding box test
	maxv, minv := tr.A, tr.A
	tr.B.MakeMaxMin(&maxv, &minv)
	tr.C.MakeMaxMin(&maxv, &minv)

	for i := 0; i < ln; i++ {
		if i == ap || i == bp || i == cp {
			continue
		}

		p := c.Points[i].P
		if p.OutsideAndNotOn(maxv, minv) {
			continue
		}

		// A point on the edge of the triangle is considered to be inside,
		// and therefore makes it a non-ear; but a point on the vertex is
		// "outside", since that's necessary to make bridges work.
		if p.EqualsExactly(tr.A) {
			continue
		}
		if p.EqualsExactly(tr.C) {
			continue
		}
		// points coincident with bp have to be allowed for bridges but edges
		// from that other point must not cross through our triangle.
		if p.EqualsExactly(tr.B) {
			jp := c.Points[wrapIdx(i-1, ln)].P
			kp := c.Points[wrapIdx(i+1, ln)].P

			// two consecutive bridges (A,B,C) and later (C,B,A) are not an ear
			if jp.Equals(tr.C) && kp.Equals(tr.A) {
				return false
			}
			// check both edges from the point in question
			if !RayIsInside(tr.A, tr.C, p, jp) && !RayIsInside(tr.A, tr.C, p, kp) {
				continue
			}
		}

		if tr.ContainsPointProjd(n, p) {
			return false
		}
	}
	return true
}

// ClipEarInto adds the triangle at vertex bp to m (unless degenerate)
// and removes that vertex. Mirrors C++ SContour::ClipEarInto.
func (c *SContour) ClipEarInto(m *SMesh, bp int, scaledEps float64) {
	ln := len(c.Points)
	ap := wrapIdx(bp-1, ln)
	cp := wrapIdx(bp+1, ln)

	tr := STriangle{A: c.Points[ap].P, B: c.Points[bp].P, C: c.Points[cp].P}
	if tr.Normal().MagSquared() < scaledEps*scaledEps {
		// A vertex with more than two edges will cause us to generate
		// zero-area triangles, which must be culled.
	} else {
		m.AddTriangle(tr)
	}

	// By deleting the point at bp, we may change the ear-ness of the points
	// on either side.
	c.Points[ap].Ear = EarUnknown
	c.Points[cp].Ear = EarUnknown

	c.Points = append(c.Points[:bp], c.Points[bp+1:]...)
}

// removeTaggedPoints drops every point whose Tag is set.
func (c *SContour) removeTaggedPoints() {
	kept := c.Points[:0]
	for _, p := range c.Points {
		if p.Tag == 0 {
			kept = append(kept, p)
		}
	}
	c.Points = kept
}

// UvTriangulateInto ear-clips this single (hole-free) uv contour into
// m. Mirrors C++ SContour::UvTriangulateInto.
func (c *SContour) UvTriangulateInto(m *SMesh, srf *SSurface) {
	var tu, tv geom.Vector
	srf.TangentsAt(0.5, 0.5, &tu, &tv, true)
	s := math.Sqrt(tu.MagSquared() + tv.MagSquared())
	// We would like to apply our tolerances in xyz; but that would be a lot
	// of work, so at least scale the epsilon semi-reasonably. That's
	// perfect for square planes, less perfect for anything else.
	scaledEps := geom.LengthEps / s

	if len(c.Points) == 0 {
		return
	}
	// Clean the original contour by removing any zero-length edges.
	// initialize eartypes to unknown while we're going over them.
	for i := range c.Points {
		c.Points[i].Tag = 0
	}
	c.Points[0].Ear = EarUnknown
	for i := 1; i < len(c.Points); i++ {
		c.Points[i].Ear = EarUnknown
		if c.Points[i].P.Equals(c.Points[i-1].P) {
			c.Points[i].Tag = 1
		}
	}
	if c.Points[0].P.Equals(c.Points[len(c.Points)-1].P) {
		c.Points[len(c.Points)-1].Tag = 1
	}
	c.removeTaggedPoints()

	// Handle simple triangle fans all at once. This pass is optional.
	if srf.DegM == 1 && srf.DegN == 1 {
		for i := range c.Points {
			c.Points[i].Tag = 0
		}
		j := 0
		pstart := 0
		elen := -1.0
		oldspan := 0.0
		for i := 1; i < len(c.Points); i++ {
			ab := c.Points[i].P.Minus(c.Points[i-1].P)
			// first time just measure the segment
			if elen < 0.0 {
				elen = ab.Dot(ab)
				oldspan = elen
				j = 1
				continue
			}
			// check for consecutive segments of similar size which are also
			// ears and where the group forms a convex ear
			end := false
			ratio := ab.Dot(ab) / elen
			if ratio < 0.25 || ratio > 4.0 {
				end = true
			}

			slen := c.Points[pstart].P.Minus(c.Points[i].P).MagSquared()
			if slen < oldspan {
				end = true
			}

			if !c.IsEar(i-1, scaledEps) {
				end = true
			}
			if j > 0 && !c.IsEmptyTriangle(pstart, i-1, i, scaledEps) {
				end = true
			}
			// the new segment is valid so add to the fan
			if !end {
				j++
				oldspan = slen
			}
			// we need to stop at the end of polygon but may still
			if i == len(c.Points)-1 {
				end = true
			}
			if end { // triangulate the fan and tag the vertices
				if j > 3 {
					center := c.Points[pstart+1].P.Plus(c.Points[pstart+j-1].P).ScaledBy(0.5)
					for x := 0; x < j; x++ {
						m.AddTriangle(STriangle{A: center, B: c.Points[pstart+x].P, C: c.Points[pstart+x+1].P})
					}
					for x := 1; x < j; x++ {
						c.Points[pstart+x].Tag = 1
					}
					m.AddTriangle(STriangle{A: center, B: c.Points[pstart+j].P, C: c.Points[pstart].P})
				}
				pstart = i - 1
				elen = ab.Dot(ab)
				oldspan = elen
				j = 1
			}
		}
		c.removeTaggedPoints()
	} // end optional fan creation pass

	toggle := false
	for len(c.Points) > 3 {
		ln := len(c.Points)
		bestEar := -1
		bestChordTol := geom.VeryPositive
		// Alternate the starting position so we generate strip-like
		// triangulations instead of fan-like
		toggle = !toggle
		offset := 0
		if toggle {
			offset = -1
		}
		for i := 0; i < ln; i++ {
			ear := wrapIdx(i+offset, ln)
			if c.Points[ear].Ear == EarUnknown {
				if c.IsEar(ear, scaledEps) {
					c.Points[ear].Ear = EarIsEar
				} else {
					c.Points[ear].Ear = EarNotEar
				}
			}
			if c.Points[ear].Ear == EarIsEar {
				if srf.DegM == 1 && srf.DegN == 1 {
					// This is a plane; any ear is a good ear.
					bestEar = ear
					break
				}
				// If we are triangulating a curved surface, then try to
				// clip ears that have a small chord tolerance from the
				// surface.
				prev := c.Points[wrapIdx(i+offset-1, ln)].P
				next := c.Points[wrapIdx(i+offset+1, ln)].P
				tol := srf.ChordToleranceForEdge(prev, next)
				if tol < bestChordTol-scaledEps {
					bestEar = ear
					bestChordTol = tol
				}
				if bestChordTol < 0.1*ChordTolMm() {
					break
				}
			}
		}
		if bestEar < 0 {
			// couldn't find an ear! fail
			return
		}
		c.ClipEarInto(m, bestEar, scaledEps)
	}

	if len(c.Points) == 3 {
		c.ClipEarInto(m, 0, scaledEps) // add the last triangle
	}
}

// ChordToleranceForEdge returns how far the surface deviates from the
// straight xyz segment between the uv points a and b. Mirrors C++
// SSurface::ChordToleranceForEdge.
func (s *SSurface) ChordToleranceForEdge(a, b geom.Vector) float64 {
	as := s.PointAt(a.X, a.Y)
	bs := s.PointAt(b.X, b.Y)

	worst := -geom.VeryPositive
	for i := 1; i <= 3; i++ {
		p := a.Plus(b.Minus(a).ScaledBy(float64(i) / 4.0))
		ps := as.Plus(bs.Minus(as).ScaledBy(float64(i) / 4.0))

		pps := s.PointAt(p.X, p.Y)
		worst = geom.Max(worst, pps.Minus(ps).MagSquared())
	}
	return math.Sqrt(worst)
}

// PointAtMaybeSwapped mirrors C++ SSurface::PointAtMaybeSwapped.
func (s *SSurface) PointAtMaybeSwapped(u, v float64, swapped bool) geom.Vector {
	if swapped {
		return s.PointAt(v, u)
	}
	return s.PointAt(u, v)
}

// NormalAtMaybeSwapped mirrors C++ SSurface::NormalAtMaybeSwapped.
func (s *SSurface) NormalAtMaybeSwapped(u, v float64, swapped bool) geom.Vector {
	var du, dv geom.Vector
	if swapped {
		s.TangentsAt(v, u, &dv, &du, true)
	} else {
		s.TangentsAt(u, v, &du, &dv, true)
	}
	return du.Cross(dv).WithMagnitude(1.0)
}

// MakeTriangulationGridInto appends grid lines in (vs, vf] spaced to
// satisfy the chord tolerance. Mirrors C++
// SSurface::MakeTriangulationGridInto.
func (s *SSurface) MakeTriangulationGridInto(l *[]float64, vs, vf float64, swapped bool, depth int) {
	worst := 0.0

	// Try piecewise linearizing four curves, at u = 0, 1/3, 2/3, 1; choose
	// the worst chord tolerance of any of those.
	worstTwist := 1.0
	for i := 0; i <= 3; i++ {
		u := float64(i) / 3.0

		// This chord test should be identical to the one in SBezier::MakePwl
		// to make the piecewise linear edges line up with the grid more or
		// less.
		ps := s.PointAtMaybeSwapped(u, vs, swapped)
		pf := s.PointAtMaybeSwapped(u, vf, swapped)

		vm1 := (2*vs + vf) / 3
		vm2 := (vs + 2*vf) / 3

		pm1 := s.PointAtMaybeSwapped(u, vm1, swapped)
		pm2 := s.PointAtMaybeSwapped(u, vm2, swapped)

		// 0.999 is about 2.5 degrees of twist over the middle 1/3 V-span.
		// we don't check at the ends because the derivative may not be valid there.
		twist := 1.0
		if s.DegM == 1 {
			twist = s.NormalAtMaybeSwapped(u, vm1, swapped).Dot(
				s.NormalAtMaybeSwapped(u, vm2, swapped))
		}
		if twist < worstTwist {
			worstTwist = twist
		}

		worst = geom.Max(worst, pm1.DistanceToLine(ps, pf.Minus(ps)))
		worst = geom.Max(worst, pm2.DistanceToLine(ps, pf.Minus(ps)))
	}

	step := 1.0 / float64(GetMaxSegments())
	if ((vf-vs) < step || worst < ChordTolMm()) && (worstTwist > 0.999 || depth > 3) {
		*l = append(*l, vf)
	} else {
		s.MakeTriangulationGridInto(l, vs, (vs+vf)/2, swapped, depth+1)
		s.MakeTriangulationGridInto(l, (vs+vf)/2, vf, swapped, depth+1)
	}
}

// UvGridTriangulateInto covers the interior of the uv polygon with an
// adaptive grid of quads, cuts those out of the polygon, and
// ear-clips the rest. Mirrors C++ SPolygon::UvGridTriangulateInto.
func (p *SPolygon) UvGridTriangulateInto(mesh *SMesh, srf *SSurface) {
	var orig SEdgeList
	p.MakeEdgesInto(&orig)

	var holes SEdgeList

	p.Normal = geom.Vector{Z: 1}
	p.FixContourDirections()

	// Build a rectangular grid, with horizontal and vertical lines in the
	// uv plane. The spacing of these lines is adaptive, so calculate that.
	v := []float64{0.0, 0.25, 0.5, 0.75, 1.0}
	li := []float64{v[0]}
	srf.MakeTriangulationGridInto(&li, 0, 1, true, 0)
	lj := []float64{v[0]}
	srf.MakeTriangulationGridInto(&lj, 0, 1, false, 0)

	// force 2nd order grid to have at least 4 segments in each direction
	if len(li) < 5 && srf.DegM > 1 { // 4 segments minimum
		li = append([]float64(nil), v...)
	}
	if len(lj) < 5 && srf.DegN > 1 { // 4 segments minimum
		lj = append([]float64(nil), v...)
	}

	if len(li) > 3 && len(lj) > 3 {
		// Now iterate over each quad in the grid. If it's outside the polygon,
		// or if it intersects the polygon, then we discard it. Otherwise we
		// generate two triangles in the mesh, and cut it out of our polygon.
		// Quads around the perimeter would be rejected by AnyEdgeCrossings.
		bottom := make([]bool, len(lj)) // did we use this quad?
		var tu, tv geom.Vector
		nearTrim := gridNearTrim(srf, &orig, li, lj)
		for i := 1; i < len(li)-1; i++ {
			prevFlag := false
			for j := 1; j < len(lj)-1; j++ {
				thisFlag := true
				us, uf := li[i], li[i+1]
				vs, vf := lj[j], lj[j+1]

				a := geom.Vector{X: us, Y: vs}
				b := geom.Vector{X: us, Y: vf}
				c := geom.Vector{X: uf, Y: vf}
				d := geom.Vector{X: uf, Y: vs}

				//  |   d-----c
				//  |   |     |
				//  |   |     |
				//  |   a-----b
				//  |
				//  +-------------> j/v axis

				if i == len(li)-2 || j == len(lj)-2 ||
					orig.AnyEdgeCrossings(a, b, nil, nil) > 0 ||
					orig.AnyEdgeCrossings(b, c, nil, nil) > 0 ||
					orig.AnyEdgeCrossings(c, d, nil, nil) > 0 ||
					orig.AnyEdgeCrossings(d, a, nil, nil) > 0 {
					thisFlag = false
				}

				// There's no intersections, so it doesn't matter which point
				// we decide to test.
				if !p.ContainsPoint(a) {
					thisFlag = false
				}
				// A corner within the chord tolerance of the trim may lie
				// between a chord and the curve it stands for, on the far
				// side of the true boundary.
				if thisFlag && (nearTrim[i][j] || nearTrim[i][j+1] || nearTrim[i+1][j] || nearTrim[i+1][j+1]) {
					thisFlag = false
				}

				if thisFlag {
					// Add the quad to our mesh
					srf.TangentsAt(us, vs, &tu, &tv, true)
					if tu.Dot(tv) < geom.LengthEps {
						// Split "the other way" if angle>90
						// compare to LENGTH_EPS instead of zero to avoid alternating triangle
						// "orientations" when the tangents are orthogonal (revolve, lathe etc.)
						// this results in a higher quality mesh.
						mesh.AddTriangle(STriangle{A: a, B: b, C: c})
						mesh.AddTriangle(STriangle{A: a, B: c, C: d})
					} else {
						mesh.AddTriangle(STriangle{A: a, B: b, C: d})
						mesh.AddTriangle(STriangle{A: b, B: c, C: d})
					}
					if !prevFlag { // add our own left edge
						holes.AddEdge(d, a, 0, 0, 0)
					}
					if !bottom[j] { // add our own bottom edge
						holes.AddEdge(a, b, 0, 0, 0)
					}
				} else {
					if prevFlag { // add our left neighbors right edge
						holes.AddEdge(a, d, 0, 0, 0)
					}
					if bottom[j] { // add our bottom neighbors top edge
						holes.AddEdge(b, a, 0, 0, 0)
					}
				}
				prevFlag = thisFlag
				bottom[j] = thisFlag
			}
		}

		// Because no duplicate edges were created we do not need to cull them.
		var hp SPolygon
		holes.AssemblePolygon(&hp, nil, true)

		p.Contours = append(p.Contours, hp.Contours...)
	}
	p.UvTriangulateInto(mesh, srf)
}

// gridNearTrim tells, for each point of the triangulation grid li x lj,
// whether it lies, on the surface in xyz, within the sag of a trim
// edge of el (in uv): the gap between a chord of a curved trim and the
// curve it stands for, where a point inside the chords can lie beyond
// the true boundary. An edge's sag is taken from how far its curve
// turns at its ends, and is at least the chord tolerance.
func gridNearTrim(srf *SSurface, el *SEdgeList, li, lj []float64) [][]bool {
	type key struct{ x, y float64 }
	n := len(el.Edges)
	xyzA, xyzB := make([]geom.Vector, n), make([]geom.Vector, n)
	from, to := map[key]int{}, map[key]int{}
	for k, e := range el.Edges {
		xyzA[k], xyzB[k] = srf.PointAt(e.A.X, e.A.Y), srf.PointAt(e.B.X, e.B.Y)
		from[key{e.A.X, e.A.Y}] = k
		to[key{e.B.X, e.B.Y}] = k
	}
	// turn is the angle between edge k and the edge of the same curve
	// that meets it at its start (or its end), or 0 for none.
	turn := func(k, other int, ok bool) float64 {
		if !ok || el.Edges[other].AuxA != el.Edges[k].AuxA {
			return 0
		}
		d0, d1 := xyzB[k].Minus(xyzA[k]), xyzB[other].Minus(xyzA[other])
		c := d0.Dot(d1) / (d0.Magnitude() * d1.Magnitude())
		return math.Acos(math.Max(-1, math.Min(1, c)))
	}
	tol := make([]float64, n)
	for k, e := range el.Edges {
		prev, okp := to[key{e.A.X, e.A.Y}]
		next, okn := from[key{e.B.X, e.B.Y}]
		phi := (turn(k, prev, okp) + turn(k, next, okn)) / 2
		// A circular arc turning by phi per chord of length L sags by
		// about L phi / 8; take twice that.
		tol[k] = math.Max(ChordTolMm(), xyzB[k].Minus(xyzA[k]).Magnitude()*phi/4)
	}
	near := make([][]bool, len(li))
	for i := range li {
		near[i] = make([]bool, len(lj))
		for j := range lj {
			p := srf.PointAt(li[i], lj[j])
			for k := range el.Edges {
				if distanceToSegment(p, xyzA[k], xyzB[k]) < tol[k] {
					near[i][j] = true
					break
				}
			}
		}
	}
	return near
}

// TriangulateInto triangulates this planar xyz polygon into m. Mirrors
// C++ SPolygon::TriangulateInto.
func (p *SPolygon) TriangulateInto(m *SMesh) {
	n := p.Normal
	if n.Equals(geom.Vector{}) {
		n = p.ComputeNormal()
	}
	u := n.NormalDir(0)
	v := n.NormalDir(1)

	var tp SPolygon
	p.InverseTransformInto(&tp, u, v, n)

	srf := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	var pm SMesh
	tp.UvTriangulateInto(&pm, &srf)
	for _, st := range pm.Triangles {
		m.AddTriangle(st.Transform(u, v, n))
	}
}

// InverseTransformInto expresses every vertex in the (u, v, n) basis.
// Mirrors C++ SPolygon::InverseTransformInto.
func (p *SPolygon) InverseTransformInto(sp *SPolygon, u, v, n geom.Vector) {
	for i := range p.Contours {
		sc := &p.Contours[i]
		tsc := SContour{TimesEnclosed: sc.TimesEnclosed}
		for _, pt := range sc.Points {
			tsc.AddPoint(geom.Vector{X: pt.P.Dot(u), Y: pt.P.Dot(v), Z: pt.P.Dot(n)})
		}
		sp.Contours = append(sp.Contours, tsc)
	}
}

// EdgeCrosses reports whether the segment ea->eb crosses this edge.
// Sharing an endpoint is not a crossing; overlapping coincident
// segments are. Mirrors C++ SEdge::EdgeCrosses (polygon.cpp).
func (e *SEdge) EdgeCrosses(ea, eb geom.Vector, ppi *geom.Vector, spl *SPointList) bool {
	d := eb.Minus(ea)
	tEps := geom.LengthEps / d.Magnitude()

	dthis := e.B.Minus(e.A)
	tthisEps := geom.LengthEps / dthis.Magnitude()

	if (ea.Equals(e.A) && eb.Equals(e.B)) ||
		(eb.Equals(e.A) && ea.Equals(e.B)) {
		if ppi != nil {
			*ppi = e.A
		}
		if spl != nil {
			spl.Points = append(spl.Points, SPoint{P: e.A})
		}
		return true
	}

	// Can't just test if distance between d and a equals distance between d and b;
	// they could be on opposite sides, since we don't have the sign.
	m := math.Sqrt(d.Magnitude() * dthis.Magnitude())
	if math.Sqrt(math.Abs(d.Dot(dthis))) > m-geom.LengthEps {
		// The edges are parallel.
		if math.Abs(e.A.DistanceToLine(ea, d)) > geom.LengthEps {
			// and not coincident, so can't be intersecting
			return false
		}
		// The edges are coincident. Make sure that neither endpoint lies
		// on the other
		inters := false
		t := e.A.Minus(ea).DivProjected(d)
		if t > tEps && t < 1-tEps {
			inters = true
		}
		t = e.B.Minus(ea).DivProjected(d)
		if t > tEps && t < 1-tEps {
			inters = true
		}
		t = ea.Minus(e.A).DivProjected(dthis)
		if t > tthisEps && t < 1-tthisEps {
			inters = true
		}
		t = eb.Minus(e.A).DivProjected(dthis)
		if t > tthisEps && t < 1-tthisEps {
			inters = true
		}

		if inters {
			if ppi != nil {
				*ppi = e.A
			}
			if spl != nil {
				spl.Points = append(spl.Points, SPoint{P: e.A})
			}
			return true
		}
		// So coincident but disjoint, okay.
		return false
	}

	// Lines are not parallel, so look for an intersection.
	t, tthis := geom.ClosestPointBetweenLines(ea, d, e.A, dthis)
	pi := ea.Plus(d.ScaledBy(t))
	if !pi.Equals(e.A.Plus(dthis.ScaledBy(tthis))) {
		// skew
		return false
	}

	inOrEdge0 := t > -tEps && t < 1+tEps
	inOrEdge1 := tthis > -tthisEps && tthis < 1+tthisEps

	if inOrEdge0 && inOrEdge1 {
		if e.A.Equals(ea) || e.B.Equals(ea) ||
			e.A.Equals(eb) || e.B.Equals(eb) {
			// Not an intersection if we share an endpoint with an edge
			return false
		}
		// But it's an intersection if a vertex of one edge lies on the
		// inside of the other (or if they cross away from either's
		// vertex).
		if ppi != nil {
			*ppi = pi
		}
		if spl != nil {
			spl.Points = append(spl.Points, SPoint{P: pi})
		}
		return true
	}
	return false
}

// AnyEdgeCrossings counts the edges that ea->eb crosses. Mirrors C++
// SEdgeList::AnyEdgeCrossings.
func (el *SEdgeList) AnyEdgeCrossings(a, b geom.Vector, ppi *geom.Vector, spl *SPointList) int {
	cnt := 0
	for i := range el.Edges {
		if el.Edges[i].EdgeCrosses(a, b, ppi, spl) {
			cnt++
		}
	}
	return cnt
}

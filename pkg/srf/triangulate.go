package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
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
	if len(p.Contours) == 0 {
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
	for i := range scn - 1 {
		if sc.Points[i].P.EqualsExactly(sc.XMinPt) {
			sco = i
		}
	}

	// And start looking on our merged contour at whichever point is nearest
	// to the leftmost point of the new segment.
	thiso := 0
	dmin := 1e10
	for i := range ln - 1 {
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
	for i := range ln {
		thisp = wrapIdx(i+thiso, ln-1)
		a = c.Points[thisp].P

		if avoided(a) {
			continue
		}

		for j := range scn - 1 {
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
		for i := range ln {
			thisp = wrapIdx(i+thiso, ln)
			a = c.Points[thisp].P

			if avoided(a) {
				continue
			}

			for j := range scn - 1 {
				scp = wrapIdx(j+sco, scn-1)
				b = sc.Points[scp].P

				if avoided(b) {
					continue
				}

				// A bridge that crosses an existing edge doesn't work.
				if avoidEdges.AnyEdgeCrossings(a, b, nil, nil) == 0 {
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
	for i := range ln {
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

	for i := range ln {
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
	c.clipEarInto(m, bp, scaledEps, nil)
}

// clipEarInto is ClipEarInto on surface srf, nil for none. A triangle
// is culled when it is thin in uv: its height over its longest side
// below scaledEps (C++ compares its doubled area with the square of
// the epsilon, which also culls thin triangles that are real, such as
// the sliver between a fine chord of a curve and the next, and leaves a
// hole there). On a curved surface a triangle thin in uv may not be in
// xyz: three points along a seam between two patches of a face lie on
// a line in uv, the line that the seam is in the patch's parameters,
// but on a curve in xyz, and the next patch meets the chords between
// them; so it is kept unless it is thin in xyz too.
func (c *SContour) clipEarInto(m *SMesh, bp int, scaledEps float64, srf *SSurface) {
	ln := len(c.Points)
	ap := wrapIdx(bp-1, ln)
	cp := wrapIdx(bp+1, ln)

	tr := STriangle{A: c.Points[ap].P, B: c.Points[bp].P, C: c.Points[cp].P}
	thin := func(t STriangle, eps float64) bool {
		long := max(t.B.Minus(t.A).MagSquared(), t.C.Minus(t.B).MagSquared(), t.A.Minus(t.C).MagSquared())
		n := t.Normal().MagSquared()
		return n < eps*eps*long || n == 0
	}
	cull := thin(tr, scaledEps)
	if cull && srf != nil && (srf.DegM != 1 || srf.DegN != 1) && !tr.A.Equals(tr.C) {
		x := STriangle{A: srf.PointAt(tr.A.X, tr.A.Y), B: srf.PointAt(tr.B.X, tr.B.Y), C: srf.PointAt(tr.C.X, tr.C.Y)}
		cull = thin(x, geom.LengthEps)
	}
	// A vertex with more than two edges will cause us to generate
	// zero-area triangles, which must be culled.
	if !cull {
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
	if srf.flat() {
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
					for x := range j {
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

	// The sides of the patch folded to a pole: every point there is
	// one point on the surface, so grazing says nothing about them.
	var poles []int
	if !srf.flat() {
		for side := range 4 {
			if srf.foldsSide(side) {
				poles = append(poles, side)
			}
		}
	}
	// The contour as it was, by each point's place on it, for ears
	// whose chord passes points clipped before (cutOff.crosses): on a
	// patch ruled one way, a cone's or a cylinder's, where a trim can
	// run nearly along the rulings and the patch has no grid lines
	// across them. On faces curved both ways the test turned ears down
	// that the mesh needed, and the ears taken instead crossed.
	var cut *cutOff
	if !srf.flat() && (srf.DegM == 1) != (srf.DegN == 1) {
		cut = newCutOff(c)
	}
	toggle := false
	for len(c.Points) > 3 {
		ln := len(c.Points)
		bestEar := -1
		bestChordTol := geom.VeryPositive
		// The first folded ear, taken when there is no other.
		fallback := -1
		// The ear of least chord tolerance among those that graze a
		// point, taken before a folded one.
		bestGrazed, bestGrazedTol := -1, geom.VeryPositive
		// Alternate the starting position so we generate strip-like
		// triangulations instead of fan-like
		toggle = !toggle
		offset := 0
		if toggle {
			offset = -1
		}
		for i := range ln {
			ear := wrapIdx(i+offset, ln)
			if c.Points[ear].Ear == EarUnknown {
				if c.IsEar(ear, scaledEps) {
					c.Points[ear].Ear = EarIsEar
				} else {
					c.Points[ear].Ear = EarNotEar
				}
			}
			if c.Points[ear].Ear == EarIsEar {
				if srf.flat() {
					// This is a plane; any ear is a good ear.
					bestEar = ear
					break
				}
				// If we are triangulating a curved surface, then try to
				// clip ears that have a small chord tolerance from the
				// surface.
				prev := c.Points[wrapIdx(i+offset-1, ln)].P
				next := c.Points[wrapIdx(i+offset+1, ln)].P
				if folded(srf, prev, c.Points[ear].P, next) || srf.AvoidFlatEars && flatEar(srf, prev, c.Points[ear].P, next) ||
					poleSliver(srf, poles, prev, c.Points[ear].P, next) {
					if fallback < 0 {
						fallback = ear
					}
					continue
				}
				tol := srf.ChordToleranceForEdge(prev, next)
				if c.grazed(srf, poles, wrapIdx(i+offset-1, ln), wrapIdx(i+offset+1, ln)) || cut.crosses(srf, prev, next) {
					if tol < bestGrazedTol {
						bestGrazed, bestGrazedTol = ear, tol
					}
					continue
				}
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
			bestEar = bestGrazed
		}
		if bestEar < 0 {
			bestEar = fallback
		}
		eps := scaledEps
		if bestEar < 0 && srf.flat() && c.simple() {
			// No ear at the tolerance: what is left is a sliver, points
			// along a trim that runs nearly straight against the chord
			// that cut them off, every corner within the tolerance of
			// collinear (a chamfer's edge on a slot's flat wall). Its
			// corners are still convex or reflex, and it has ears: take
			// one at no tolerance, and keep its triangle, or the points
			// along the trim, which the face beyond keeps, are left out
			// and the mesh has a hole there.
			eps = math.SmallestNonzeroFloat64
			for i := range ln {
				c.Points[i].Ear = EarUnknown
				if c.IsEar(i, eps) {
					bestEar = i
					break
				}
			}
			for i := range ln {
				c.Points[i].Ear = EarUnknown
			}
		}
		if bestEar < 0 {
			// couldn't find an ear! fail
			return
		}
		c.clipEarInto(m, bestEar, eps, srf)
	}

	if len(c.Points) == 3 {
		c.clipEarInto(m, 0, scaledEps, srf) // add the last triangle
	}
}

// cutOff is a contour's points as they were before ear clipping, with
// each point's place among them, or -1 for a point the contour holds
// twice.
type cutOff struct {
	pts []geom.Vector
	at  map[geom.Vector]int
}

func newCutOff(c *SContour) *cutOff {
	co := &cutOff{pts: make([]geom.Vector, len(c.Points)), at: make(map[geom.Vector]int, len(c.Points))}
	for i, p := range c.Points {
		co.pts[i] = p.P
		if _, ok := co.at[p.P]; ok {
			co.at[p.P] = -1
		} else {
			co.at[p.P] = i
		}
	}
	return co
}

// crosses reports whether the chord from a to c, in uv, which clipping
// an ear between them would add, passes in xyz on the far side of a
// point of the contour as it was between them, one that ears clipped
// before have cut off (or the ear's own tip): in uv every such point
// lies on the tip's side of the chord, but in xyz the chord is straight
// where its image in uv bends by its sag, and where the contour runs
// nearly straight, as a trim along a cone's rulings does, the sag
// carries it past them. The face beyond keeps those points, and its
// triangles cross the chord's (a chamfer where a band meets a cone,
// fanned from a far point of the trim; roadmap 26).
func (co *cutOff) crosses(srf *SSurface, a, c geom.Vector) bool {
	if co == nil {
		return false
	}
	ia, ok := co.at[a]
	ic, okc := co.at[c]
	if !ok || !okc || ia < 0 || ic < 0 {
		return false
	}
	n := len(co.pts)
	ac := c.Minus(a)
	l2 := ac.X*ac.X + ac.Y*ac.Y
	if l2 == 0 {
		return false
	}
	pa, pc := srf.PointAt(a.X, a.Y), srf.PointAt(c.X, c.Y)
	chord := pc.Minus(pa)
	for k := wrapIdx(ia+1, n); k != ic; k = wrapIdx(k+1, n) {
		q := co.pts[k]
		su := ac.X*(q.Y-a.Y) - ac.Y*(q.X-a.X)
		// Only a point as near the chord in uv as grazed's: farther
		// off, the sag doesn't carry the chord past it.
		if su == 0 || su*su > grazeShare*grazeShare*l2*l2 {
			continue
		}
		nq := srf.NormalAt(q.X, q.Y)
		if nq.Magnitude() < lengthEps {
			continue
		}
		sx := chord.Cross(srf.PointAt(q.X, q.Y).Minus(pa)).Dot(nq)
		if sx*su <= 0 {
			return true
		}
	}
	return false
}

// simple reports whether no two of the contour's points are one: a
// contour with a bridge to a hole, or one that touches itself, has
// corners that are not ears however convex.
func (c *SContour) simple() bool {
	seen := make(map[geom.Vector]bool, len(c.Points))
	for _, p := range c.Points {
		if seen[p.P] {
			return false
		}
		seen[p.P] = true
	}
	return true
}

// poleSliver reports whether the ear a, b, c, in uv, has a corner on a
// side of the patch folded to a pole and is flat on the surface, as
// flatEar finds: the pole joined to two points of a trim running up a
// meridian into it, all three on that meridian, which lays a sliver in
// the meridian's plane over the face beyond it, as the face beyond
// lays its own (a half of a sphere bounded by meridians through its
// poles, as NIST_MBE_PMI_6 has).
func poleSliver(srf *SSurface, poles []int, a, b, c geom.Vector) bool {
	if len(poles) == 0 {
		return false
	}
	on := func(p geom.Vector) bool {
		for _, side := range poles {
			if math.Abs([]float64{p.X, p.X - 1, p.Y, p.Y - 1}[side]) < 1e-9 {
				return true
			}
		}
		return false
	}
	if !on(a) && !on(b) && !on(c) {
		return false
	}
	return flatEar(srf, a, b, c)
}

// grazeShare is how far off a chord in uv, as a share of its length, a
// point may be and still graze it.
const grazeShare = 0.02

// grazed reports whether the chord from ap to cp, which clipping the
// ear at bp would add, passes a point of the contour, the ear's own
// included, closer on the surface than twice the chord's sag. In uv the
// point is off the chord, but in xyz the chord dips below the surface,
// so the ear can be a sliver that turns over, or the edges from that
// point, on the other face that meets there, can cross its triangle: a
// cylinder's wall strip beside the point where a crossed cylinder's
// seam pierces it was clipped into a sliver the height of the wall
// (roadmap 24.1).
func (c *SContour) grazed(srf *SSurface, poles []int, ap, cp int) bool {
	a, b := c.Points[ap].P, c.Points[cp].P
	atPole := func(p geom.Vector) bool {
		for _, side := range poles {
			if math.Abs([]float64{p.X, p.X - 1, p.Y, p.Y - 1}[side]) < 1e-6 {
				return true
			}
		}
		return false
	}
	if atPole(a) || atPole(b) {
		return false
	}
	ab := b.Minus(a)
	l2 := ab.X*ab.X + ab.Y*ab.Y
	if l2 < geom.LengthEps*geom.LengthEps {
		return false
	}
	near := 2*srf.ChordToleranceForEdge(a, b) + geom.LengthEps
	pa, pb := srf.PointAt(a.X, a.Y), srf.PointAt(b.X, b.Y)
	for i := range c.Points {
		if i == ap || i == cp {
			continue
		}
		p := c.Points[i].P
		if p.EqualsExactly(a) || p.EqualsExactly(b) || atPole(p) {
			continue
		}
		t := ((p.X-a.X)*ab.X + (p.Y-a.Y)*ab.Y) / l2
		if t <= 0 || t >= 1 {
			continue
		}
		// Off the chord in uv by a small share of its length, which
		// leaves out points near a pole: far apart in uv, one in xyz.
		off := a.Plus(ab.ScaledBy(t)).Minus(p)
		if off.X*off.X+off.Y*off.Y > grazeShare*grazeShare*l2 {
			continue
		}
		// The chord's point at t, in xyz, against the point's own.
		q := pa.Plus(pb.Minus(pa).ScaledBy(t))
		if srf.PointAt(p.X, p.Y).Minus(q).Magnitude() < near {
			return true
		}
	}
	return false
}

// flatEar reports whether the triangle a, b, c, in uv, is an ear the
// mesher should not take on a surface trimmed by a planar curve
// (SSurface.AvoidFlatEars): one that is a sliver in xyz, far thinner
// than it is long, or that leans off the surface's tangent plane at its
// middle by more than 30 degrees. Three points along a planar trim make
// such an ear: it lies in the trim's plane, over the face on the trim's
// other side, and the chord test likes it, since the surface lies close
// to its long side.
func flatEar(srf *SSurface, a, b, c geom.Vector) bool {
	pa, pb, pc := srf.PointAt(a.X, a.Y), srf.PointAt(b.X, b.Y), srf.PointAt(c.X, c.Y)
	long := max(pb.Minus(pa).MagSquared(), pc.Minus(pb).MagSquared(), pa.Minus(pc).MagSquared())
	n := pb.Minus(pa).Cross(pc.Minus(pa))
	// Twice the area over the longest side is the height.
	if n.MagSquared() < 1e-6*long*long {
		return true
	}
	m := a.Plus(b).Plus(c).ScaledBy(1.0 / 3)
	sn := srf.NormalAt(m.X, m.Y)
	if sn.Magnitude() == 0 {
		return false
	}
	return math.Abs(n.WithMagnitude(1).Dot(sn.WithMagnitude(1))) < math.Cos(30*math.Pi/180)
}

// flat reports whether the surface is a plane: of degree 1 both ways,
// its four corners in one plane. A patch of degree 1 both ways whose
// corners are not, as a loft's side between two straight edges turned
// against each other, is a twisted ruled face, and is meshed as curved
// surfaces are: as a plane it would be two triangles whatever the
// tolerance.
func (s *SSurface) flat() bool {
	if s.DegM != 1 || s.DegN != 1 {
		return false
	}
	a, b, c, d := s.Ctrl[0][0], s.Ctrl[1][0], s.Ctrl[0][1], s.Ctrl[1][1]
	n := b.Minus(a).Cross(c.Minus(a))
	if n.Magnitude() < geom.LengthEps*geom.LengthEps {
		n = b.Minus(d).Cross(c.Minus(d))
		if n.Magnitude() < geom.LengthEps*geom.LengthEps {
			return true
		}
	}
	return math.Abs(d.Minus(a).Dot(n.WithMagnitude(1))) < geom.LengthEps
}

// folded reports whether the triangle a, b, c, in uv, turns over on the
// surface: whether in xyz it faces against the surface's normal at its
// middle, where the uv triangle's own turn says it should face along
// it. Three points of one curved trim, consecutive on it, can make
// such an ear, convex in uv where the curve bends one way and lying in
// xyz on the far side of the chord, over the face beyond the trim: a
// sphere's band cut by a plane would lay a sliver flat on the plane's
// face.
func folded(srf *SSurface, a, b, c geom.Vector) bool {
	uvTurn := b.Minus(a).Cross(c.Minus(a)).Z
	if math.Abs(uvTurn) < geom.LengthEps*geom.LengthEps {
		return false
	}
	pa, pb, pc := srf.PointAt(a.X, a.Y), srf.PointAt(b.X, b.Y), srf.PointAt(c.X, c.Y)
	n := pb.Minus(pa).Cross(pc.Minus(pa))
	if n.Magnitude() < geom.LengthEps*geom.LengthEps {
		return false
	}
	mid := a.Plus(b).Plus(c).ScaledBy(1.0 / 3)
	return n.Dot(srf.NormalAt(mid.X, mid.Y))*uvTurn < 0
}

// splitTrimDiagonals splits, at its middle in uv, edges inside the
// triangles from start on in m (in uv) that join two points of the
// trim el, with the two triangles on each, so that the mesh stays
// closed. An edge along one face beyond (its points on one curve, or
// on two meeting there) is split where the surface leaves it at all,
// by more than trimSplitOff: a curved face cut by a plane has its trim
// in that plane, so an edge between two points of that trim that are
// not neighbours on it lies in the plane's face, inside its outline,
// and the triangles that meet there cross the face's own (a small cap
// with too few grid points inside, or a band where the grid keeps
// clear of a trim near a pole; roadmap 24.1). On a patch folded to a
// pole, an edge between trim points, or points split here, is split
// where the surface leaves it by more than the chord tolerance, over
// trimSplitRounds rounds (roadmap 24.2).
func (s *SSurface) splitTrimDiagonals(shell *SShell, m *SMesh, start int, el *SEdgeList) {
	type key struct{ x, y float64 }
	type pair struct{ a, b key }
	// The faces beyond the curves each trim point is on: one, or two
	// at a corner. Two curves' ends at a corner may differ in uv by a
	// rounding, so the points are found to within a nanometre of uv.
	near := func(v geom.Vector) key {
		return key{math.Round(v.X * 1e9), math.Round(v.Y * 1e9)}
	}
	curves := make(map[key][2]int, 2*len(el.Edges))
	add := func(p geom.Vector, c int) {
		cs, ok := curves[near(p)]
		switch {
		case !ok:
			curves[near(p)] = [2]int{c, c}
		case cs[0] != c:
			cs[1] = c
			curves[near(p)] = cs
		}
	}
	beyond := map[int]int{}
	for _, e := range el.Edges {
		b, ok := beyond[e.AuxA]
		if !ok {
			b = -e.AuxA - 1
			if sc := shell.FindCurve(core.HSCurve{V: uint32(e.AuxA)}); sc != nil {
				switch s.H {
				case sc.SurfA:
					b = int(sc.SurfB.V)
				case sc.SurfB:
					b = int(sc.SurfA.V)
				}
			}
			beyond[e.AuxA] = b
		}
		add(e.A, b)
		add(e.B, b)
	}
	k := func(v geom.Vector) key { return key{v.X, v.Y} }
	// sameCurve is whether a and b are points of trim curves along one
	// face beyond.
	sameCurve := func(a, b geom.Vector) bool {
		ca, oka := curves[near(a)]
		cb, okb := curves[near(b)]
		if !oka || !okb {
			return false
		}
		for _, x := range ca {
			if x == cb[0] || x == cb[1] {
				return true
			}
		}
		return false
	}
	edge := func(a, b geom.Vector) pair {
		ka, kb := k(a), k(b)
		if kb.x < ka.x || (kb.x == ka.x && kb.y < ka.y) {
			ka, kb = kb, ka
		}
		return pair{ka, kb}
	}
	// The trim's edges in xyz, to keep new points clear of.
	trimXyz := make([][2]geom.Vector, len(el.Edges))
	for i, e := range el.Edges {
		trimXyz[i] = [2]geom.Vector{s.PointAt(e.A.X, e.A.Y), s.PointAt(e.B.X, e.B.Y)}
	}
	isClear := func(p geom.Vector, d float64) bool {
		for _, e := range trimXyz {
			if distanceToSegment(p, e[0], e[1]) < d {
				return false
			}
		}
		return true
	}
	// A point on a side of the patch folded to a pole has no one
	// parameter along it, so an edge from it is no straight line in uv
	// of the surface; such edges, rulings to a cone's apex, are left.
	var folds []int
	for side := range 4 {
		if s.foldsSide(side) {
			folds = append(folds, side)
		}
	}
	atPole := func(p geom.Vector) bool {
		for _, side := range folds {
			c := []float64{p.X, p.X - 1, p.Y, p.Y - 1}[side]
			if math.Abs(c) < 1e-9 {
				return true
			}
		}
		return false
	}
	// Points made here, which edges between trim points split.
	made := map[key]bool{}
	tol := ChordTolMm()
	for round := range trimSplitRounds {
		// Edges inside the mesh are in two of its triangles.
		count := map[pair]int{}
		for i := start; i < len(m.Triangles); i++ {
			t := &m.Triangles[i]
			for _, e := range [][2]geom.Vector{{t.A, t.B}, {t.B, t.C}, {t.C, t.A}} {
				if ok0, ok1 := curves[near(e[0])] != [2]int{} || made[k(e[0])], curves[near(e[1])] != [2]int{} || made[k(e[1])]; ok0 && ok1 {
					count[edge(e[0], e[1])]++
				}
			}
		}
		mids := map[pair]geom.Vector{}
		for e, n := range count {
			if n != 2 {
				continue
			}
			a, b := geom.Vector{X: e.a.x, Y: e.a.y}, geom.Vector{X: e.b.x, Y: e.b.y}
			if atPole(a) || atPole(b) {
				continue
			}
			mid := a.Plus(b).ScaledBy(0.5)
			pa, pb := s.PointAt(a.X, a.Y), s.PointAt(b.X, b.Y)
			pm := s.PointAt(mid.X, mid.Y)
			off := pm.Minus(pa.Plus(pb).ScaledBy(0.5)).Magnitude()
			// Along one face beyond, any rise splits the edge (it lies in
			// that face's plane). On a patch folded to a pole, which
			// has no grid along its straight lines, an edge that strays
			// from the surface by more than the chord tolerance, across
			// it, does too, where its middle is clear of the trim: a cone
			// cut through near its apex was meshed by chords across it.
			if (round == 0 && sameCurve(a, b) && off > trimSplitOff) || (len(folds) > 0 && off > tol && isClear(pm, tol)) {
				mids[e] = mid
			}
		}
		if len(mids) == 0 {
			return
		}
		for _, mid := range mids {
			made[k(mid)] = true
		}
		end := len(m.Triangles)
		for i := start; i < end; i++ {
			t := m.Triangles[i]
			v := [3]geom.Vector{t.A, t.B, t.C}
			var mid [3]geom.Vector
			var split [3]bool
			n := 0
			for j := range 3 {
				mid[j], split[j] = mids[edge(v[j], v[(j+1)%3])]
				if split[j] {
					n++
				}
			}
			if n == 0 {
				continue
			}
			// Turn so that edge 0 (v0 to v1) is split, and with two split,
			// edge 1 as well.
			for r := 0; r < 3 && !(split[0] && (n != 2 || split[1])); r++ {
				v = [3]geom.Vector{v[1], v[2], v[0]}
				mid = [3]geom.Vector{mid[1], mid[2], mid[0]}
				split = [3]bool{split[1], split[2], split[0]}
			}
			var out []STriangle
			switch n {
			case 1:
				out = []STriangle{{A: v[0], B: mid[0], C: v[2]}, {A: mid[0], B: v[1], C: v[2]}}
			case 2:
				out = []STriangle{{A: v[0], B: mid[0], C: v[2]}, {A: mid[0], B: v[1], C: mid[1]},
					{A: mid[0], B: mid[1], C: v[2]}}
			default:
				out = []STriangle{{A: v[0], B: mid[0], C: mid[2]}, {A: mid[0], B: v[1], C: mid[1]},
					{A: mid[2], B: mid[1], C: v[2]}, {A: mid[0], B: mid[1], C: mid[2]}}
			}
			m.Triangles[i] = out[0]
			for _, o := range out[1:] {
				m.AddTriangle(o)
			}
		}
	}
}

// trimSplitRounds is how many times splitTrimDiagonals goes over the
// edges it made.
const trimSplitRounds = 6

// trimSplitOff is how far, in mm, the surface may leave an edge
// between two trim points at its middle and the edge stay whole: a
// hundred times LengthEps, so that an edge the surface runs along
// within rounding, as a cylinder's along its axis, is left.
const trimSplitOff = 100 * geom.LengthEps

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
	for i := range 4 {
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

	step := 1.0 / float64(MaxSegments())
	if ((vf-vs) < step || worst < ChordTolMm()) && (worstTwist > 0.999 || depth >= gridMaxTwistDepth) {
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
	p.uvGridTriangulateInto(mesh, srf, nil)
}

// uvGridTriangulateInto is UvGridTriangulateInto with the trim's edges
// in uv as the surface made them, each with its curve in AuxA, which
// tell the chords of one curve from the corners between two; nil takes
// the polygon's edges, which don't.
func (p *SPolygon) uvGridTriangulateInto(mesh *SMesh, srf *SSurface, trim *SEdgeList) {
	var orig SEdgeList
	p.MakeEdgesInto(&orig)
	if trim == nil {
		trim = &orig
	}

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
	if srf.twisted() {
		li, lj = srf.refineTwistedGrid(li, lj)
	}

	if len(li) > 3 && len(lj) > 3 {
		// Now iterate over each quad in the grid. If it's outside the polygon,
		// or if it intersects the polygon, then we discard it. Otherwise we
		// generate two triangles in the mesh, and cut it out of our polygon.
		// Quads around the perimeter would be rejected by AnyEdgeCrossings.
		bottom := make([]bool, len(lj)) // did we use this quad?
		var tu, tv geom.Vector
		nearTrim := gridNearTrim(srf, trim, li, lj)
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

// twisted reports whether the surface is a twisted ruled face: straight
// one way, as an extrusion's sides are, but with its normal turning
// along those straight lines, as a cylinder's does not.
func (s *SSurface) twisted() bool {
	if (s.DegM != 1 && s.DegN != 1) || s.flat() {
		return false
	}
	for k := range 5 {
		t := float64(k) / 4
		var a, b geom.Vector
		if s.DegM == 1 {
			a, b = s.NormalAt(1.0/3, t), s.NormalAt(2.0/3, t)
		} else {
			a, b = s.NormalAt(t, 1.0/3), s.NormalAt(t, 2.0/3)
		}
		if a.Magnitude() < lengthEps || b.Magnitude() < lengthEps {
			continue
		}
		if a.WithMagnitude(1).Dot(b.WithMagnitude(1)) < 1-twistedMinTurn {
			return true
		}
	}
	return false
}

// twistedMinTurn is how far (1 - cos) a ruled face's normal must turn
// along its straight lines for twisted to take it for twisted.
const twistedMinTurn = 1e-6

// refineTwistedGrid adds lines to the grid li (in u) by lj (in v) of a
// twisted ruled surface, a loft's side between edges turned against
// each other, near its sides. The grid's first and last cells each way
// are left to the ear clipper, which fans them from the trim's points,
// and a side's straight edges have points at their ends only: on a
// twisted face such a fan, a cell wide and the side long, strays from
// the surface by the twist over the cell, and biased the volume by up
// to 1.2% (roadmap 24.4). Halving the outer cells leaves the fans a
// strip half a cell wide, and 0.55% at most; halving them again took
// that to 0.51% for a quarter more time. The lines are
// inside the face, so its edges, shared with the faces beside it, keep
// their points.
func (s *SSurface) refineTwistedGrid(li, lj []float64) ([]float64, []float64) {
	edges := func(l []float64) []float64 {
		// A face with no grid line across its rulings, as a swept
		// bar's sides, gets none here: four forced ones made three
		// threads' flanks cross each other.
		if len(l) < 3 {
			return l
		}
		for range twistEdgeHalvings {
			n := len(l)
			out := append([]float64{l[0], (l[0] + l[1]) / 2}, l[1:n-1]...)
			out = append(out, (l[n-2]+l[n-1])/2, l[n-1])
			l = out
		}
		return l
	}
	return edges(li), edges(lj)
}

// twistEdgeHalvings is how many times refineTwistedGrid halves the
// outer cells of a twisted ruled surface's grid.
const twistEdgeHalvings = 1

// gridMaxTwistDepth is how many times MakeTriangulationGridInto halves
// a span for twist, so a twisted face gets up to 32 rows. C++ stopped
// at 16 (depth > 3), which a ruled face turned by a third of a turn
// outruns: the rows along its trims were ear-clipped into triangles
// that cross (roadmap 13.1). Each level doubles the rows of every
// twisted face, and with them the time to mesh it.
const gridMaxTwistDepth = 5

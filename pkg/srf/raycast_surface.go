package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
)

// IsExtrusion reports whether this surface is an extrusion; if so and
// of is non-nil, the extruded curve and direction are returned.
func (s *SSurface) IsExtrusion(of *SBezier, along *geom.Vector) bool {
	if s.DegN != 1 {
		return false
	}
	dir := s.Ctrl[0][1].Minus(s.Ctrl[0][0])
	for i := 0; i <= s.DegM; i++ {
		if math.Abs(s.Weight[i][1]-s.Weight[i][0]) < geom.LengthEps &&
			s.Ctrl[i][1].Minus(s.Ctrl[i][0]).Equals(dir) {
			continue
		}
		return false
	}
	if of != nil {
		for i := 0; i <= s.DegM; i++ {
			of.Weight[i] = s.Weight[i][0]
			of.Ctrl[i] = s.Ctrl[i][0]
		}
		of.Degree = s.DegM
		*along = dir
	}
	return true
}

// IsCircle reports whether this curve is a circular arc about axis; if
// so its center and radius are returned.
func (b *SBezier) IsCircle(axis geom.Vector, center *geom.Vector, r *float64) bool {
	if b.Degree != 2 {
		return false
	}
	if b.Ctrl[1].DistanceToLine(b.Ctrl[0], b.Ctrl[2].Minus(b.Ctrl[0])) < geom.LengthEps {
		// This is almost a line segment. So it's a circle with very large
		// radius, which is likely to make code that tries to handle circles
		// blow up. So return false.
		return false
	}

	t0 := b.Ctrl[0].Minus(b.Ctrl[1])
	t2 := b.Ctrl[2].Minus(b.Ctrl[1])
	r0 := axis.Cross(t0)
	r2 := axis.Cross(t2)

	*center = geom.AtIntersectionOfLines(b.Ctrl[0], b.Ctrl[0].Plus(r0),
		b.Ctrl[2], b.Ctrl[2].Plus(r2))

	rd0 := center.Minus(b.Ctrl[0]).Magnitude()
	rd2 := center.Minus(b.Ctrl[2]).Magnitude()
	if math.Abs(rd0-rd2) > geom.LengthEps {
		return false
	}
	*r = rd0

	u := r0.WithMagnitude(1)
	v := axis.Cross(u).WithMagnitude(1)
	c2 := center.Project2d(u, v)
	pa2 := b.Ctrl[0].Project2d(u, v).Minus(c2)
	pb2 := b.Ctrl[2].Project2d(u, v).Minus(c2)

	thetaa := libm.Atan2(pa2.V, pa2.U) // in fact always zero due to csys
	thetab := libm.Atan2(pb2.V, pb2.U)
	dtheta := wrapNot0(thetab-thetaa, 2*math.Pi)
	if dtheta > math.Pi {
		// Not possible with a second order Bezier arc; so we must have
		// the points backwards.
		dtheta = 2*math.Pi - dtheta
	}

	return math.Abs(b.Weight[1]-libm.Cos(dtheta/2)) <= geom.LengthEps
}

// wrapNot0 clamps v to the range (0, n].
func wrapNot0(v, n float64) float64 {
	for v > n {
		v -= n
	}
	for v <= 0 {
		v += n
	}
	return v
}

// IsCylinder reports whether this surface is an extruded circular arc.
func (s *SSurface) IsCylinder(axis, center *geom.Vector, r *float64, start, finish *geom.Vector) bool {
	var sb SBezier
	if !s.IsExtrusion(&sb, axis) {
		return false
	}
	if !sb.IsCircle(*axis, center, r) {
		return false
	}
	*start = sb.Ctrl[0]
	*finish = sb.Ctrl[2]
	return true
}

// surfInter is a (u, v) line-surface intersection candidate.
// offPatchTol is how far from the line's own hit the closest point on a
// cylinder's patch may be and still count as that hit.
const offPatchTol = 1e3 * lengthEps

type surfInter struct {
	tag int
	p   geom.Point2d
}

// AllPointsIntersectingUntrimmed recursively subdivides the surface
// until each piece is nearly flat, then refines with Newton.
func (s *SSurface) AllPointsIntersectingUntrimmed(a, b geom.Vector, cnt, level *int,
	l *[]surfInter, asSegment bool, sorig *SSurface) {
	// Test if the line intersects our axis-aligned bounding box; if no, then
	// no possibility of an intersection
	if s.LineEntirelyOutsideBbox(a, b, asSegment) {
		return
	}
	if *cnt > 2000 {
		return
	}
	*cnt++

	// If we might intersect, and the surface is small, then switch to Newton
	// iterations.
	if s.DepartureFromCoplanar() < 0.2*s.Tol.ChordTolMm() {
		p := s.Ctrl[0][0].Plus(s.Ctrl[0][s.DegN]).Plus(s.Ctrl[s.DegM][0]).
			Plus(s.Ctrl[s.DegM][s.DegN]).ScaledBy(0.25)
		var inter surfInter
		sorig.ClosestPointTo(p, &inter.p, false)
		if sorig.PointIntersectingLine(a, b, &inter.p.U, &inter.p.V) {
			*l = append(*l, inter)
		}
		return
	}

	// But the surface is big, so split it, alternating by u and v
	var surf0, surf1 SSurface
	s.SplitInHalf((*level&1) == 0, &surf0, &surf1)

	nextLevel := *level + 1
	*level = nextLevel
	surf0.AllPointsIntersectingUntrimmed(a, b, cnt, level, l, asSegment, sorig)
	*level = nextLevel
	surf1.AllPointsIntersectingUntrimmed(a, b, cnt, level, l, asSegment, sorig)
}

// AllPointsIntersecting finds all points where the line through a and b
// intersects this surface and appends them to l. If asSegment, only
// intersections strictly inside the segment are reported. If trimmed,
// only intersections inside the trim region (per the classifying BSP)
// are reported.
func (s *SSurface) AllPointsIntersecting(a, b geom.Vector, l *[]SInter,
	asSegment, trimmed, inclTangent bool) {
	s.allPointsIntersecting(a, b, l, asSegment, trimmed, inclTangent, 0)
}

// allPointsIntersecting is AllPointsIntersecting. With nearCurved > 0,
// a trimmed search also reports, with OnEdge set, the hits within
// nearCurved of a curved trim's pwl, on whichever side of it the trim
// BSP puts them: there the BSP can be wrong about the true curve.
func (s *SSurface) allPointsIntersecting(a, b geom.Vector, l *[]SInter,
	asSegment, trimmed, inclTangent bool, nearCurved float64) {
	if s.LineEntirelyOutsideBbox(a, b, asSegment) {
		return
	}

	ba := b.Minus(a)
	bam := ba.Magnitude()

	var inters []surfInter

	// All the intersections between the line and the surface; either special
	// cases that we can quickly solve in closed form, or general numerical.
	var center, axis, start, finish geom.Vector
	var radius float64
	switch {
	case s.DegM == 1 && s.DegN == 1:
		// Against a plane, easy.
		n := s.NormalAt(0, 0).WithMagnitude(1)
		d := n.Dot(s.PointAt(0, 0))
		// Trim to line segment now if requested, don't generate points that
		// would just get discarded later.
		if !asSegment ||
			(n.Dot(a) > d+geom.LengthEps && n.Dot(b) < d-geom.LengthEps) ||
			(n.Dot(b) > d+geom.LengthEps && n.Dot(a) < d-geom.LengthEps) {
			p := geom.AtIntersectionOfPlaneAndLine(n, d, a, b)
			var inter surfInter
			s.ClosestPointTo(p, &inter.p, true)
			inters = append(inters, inter)
		}

	case s.IsCylinder(&axis, &center, &radius, &start, &finish):
		// This one can be solved in closed form too.
		ab := b.Minus(a)
		if axis.Cross(ab).Magnitude() < geom.LengthEps {
			// edge is parallel to axis of cylinder, no intersection points
			return
		}
		// A coordinate system centered at the center of the circle, with
		// the edge under test horizontal
		n := axis.WithMagnitude(1)
		u := ab.Minus(n.ScaledBy(ab.Dot(n))).WithMagnitude(1)
		v := n.Cross(u)
		ap := a.Minus(center).Project2d(u, v)
		bp := b.Minus(center).Project2d(u, v)
		sp := start.Minus(center).Project2d(u, v)
		fp := finish.Minus(center).Project2d(u, v)

		thetas := libm.Atan2(sp.V, sp.U)
		thetaf := libm.Atan2(fp.V, fp.U)

		var ip [2]geom.Point2d
		ipN := 0
		if math.Abs(math.Abs(ap.V)-radius) < geom.LengthEps {
			// tangent
			if inclTangent {
				ip[0] = geom.Point2d{U: 0, V: ap.V}
				ipN = 1
			}
		} else if math.Abs(ap.V) < radius {
			// two intersections
			xint := math.Sqrt(radius*radius - ap.V*ap.V)
			ip[0] = geom.Point2d{U: -xint, V: ap.V}
			ip[1] = geom.Point2d{U: xint, V: ap.V}
			ipN = 2
		}
		for i := range ipN {
			t := ip[i].Minus(ap).DivProjected(bp.Minus(ap))
			// This is a point on the circle; but is it on the arc?
			pp := ap.Plus(bp.Minus(ap).ScaledBy(t))
			theta := libm.Atan2(pp.V, pp.U)
			dp := WrapSymmetric(theta-thetas, 2*math.Pi)
			df := WrapSymmetric(thetaf-thetas, 2*math.Pi)
			tol := geom.LengthEps / radius

			if (df > 0 && ((dp < -tol) || (dp > df+tol))) ||
				(df < 0 && ((dp > tol) || (dp < df-tol))) {
				continue
			}

			p := a.Plus(b.Minus(a).ScaledBy(t))

			var inter surfInter
			s.ClosestPointTo(p, &inter.p, true)
			// A hit on the cylinder beyond the patch's straight sides
			// projects onto the nearest side, off the line, where the
			// trim may well take it in: a band's arc, a patch of a
			// cylinder across y, made a seed half a millimetre past
			// its side from a lathed wall's bottom circle (roadmap
			// 24.3).
			if s.PointAt(inter.p.U, inter.p.V).Minus(p).Magnitude() > offPatchTol {
				continue
			}
			inters = append(inters, inter)
		}

	default:
		// General numerical solution by subdivision, fallback; while
		// the surface has a tree of its pieces, by walking that.
		cnt, level := 0, 0
		if t := s.rayTreeFor(); t != nil {
			t.walk(&t.root, a, b, &cnt, &inters, asSegment, s)
		} else {
			s.AllPointsIntersectingUntrimmed(a, b, &cnt, &level, &inters, asSegment, s)
		}
	}

	// Remove duplicate intersection points
	for i := range inters {
		for j := i + 1; j < len(inters); j++ {
			if inters[i].p.EqualsTol(inters[j].p, geom.LengthEps) {
				inters[j].tag = 1
			}
		}
	}

	for i := range inters {
		if inters[i].tag != 0 {
			continue
		}
		puv := inters[i].p

		// Make sure the point lies within the finite line segment
		pxyz := s.PointAt(puv.U, puv.V)
		t := pxyz.Minus(a).DivProjected(ba)
		if asSegment && (t > 1-geom.LengthEps/bam || t < geom.LengthEps/bam) {
			continue
		}

		// And that it lies inside our trim region
		c := BspUvOutside
		if s.Bsp != nil {
			c = s.Bsp.ClassifyPoint(puv, geom.Point2d{}, s)
		}
		// A point outside the patch projects onto its boundary, off the
		// line; only a point really on the line is a hit.
		near := trimmed && nearCurved > 0 && distanceToEdges(pxyz, &s.curvedEdges) < nearCurved &&
			pxyz.DistanceToLine(a, ba) < lengthEps
		if trimmed && c == BspUvOutside && !near {
			continue
		}

		// It does, so generate the intersection
		*l = append(*l, SInter{
			P:          pxyz,
			SurfNormal: s.NormalAt(puv.U, puv.V),
			PInter:     puv,
			Srf:        s,
			OnEdge:     c != BspUvInside || near,
			nearTrim:   near && (c == BspUvInside || c == BspUvOutside),
		})
	}
}

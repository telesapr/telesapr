package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// cylinderOfExtrusion reports the axis (a point on it and its unit
// direction) and the radius of the cylinder an extrusion of a circular
// arc lies on, if of is one.
func cylinderOfExtrusion(of *SBezier, along geom.Vector) (o, dir geom.Vector, r float64, ok bool) {
	if along.Magnitude() < lengthEps {
		return geom.Vector{}, geom.Vector{}, 0, false
	}
	dir = along.WithMagnitude(1)
	if !of.IsCircle(dir, &o, &r) || r < lengthEps {
		return geom.Vector{}, geom.Vector{}, 0, false
	}
	return o, dir, r, true
}

// intersectEqualCylinders adds where two patches on cylinders of one
// radius whose axes cross meet, and reports whether it did: an
// equal-pipe tee or cross. The cylinders meet in two ellipses, in the
// planes through the axes' crossing that halve the angles between them
// (on both cylinders a point's distance from either axis is the radius,
// so its components along the two axes are equal or opposite), and the
// ellipses cross at the two points where the cylinders touch tangent.
// Marched, the curve turned back at those points, where the normals are
// parallel, and the Boolean failed whatever the operation (roadmap K3).
// Each ellipse is the affine image of the first cylinder's circle, so it
// is exact as rational quadratic arcs; it is cut at the patches' sides,
// their faces' outlines and the two touching points, and kept where it
// lies on both patches.
// equalCrossing is how two patches on cylinders of one radius whose axes
// cross lie: the crossing o, the axes a and b, e1 the unit direction from
// o to a point where the cylinders touch tangent (the other is opposite),
// e2 completing a frame about a, and the radius r.
type equalCrossing struct {
	o, a, b, e1, e2 geom.Vector
	r               float64
}

// equalCylindersOf reports how s and b cross, if they are patches on
// cylinders of one radius whose axes cross.
func equalCylindersOf(s, b *SSurface) (equalCrossing, bool) {
	var oft, ofb SBezier
	var alongt, alongb geom.Vector
	if !s.IsExtrusion(&oft, &alongt) || !b.IsExtrusion(&ofb, &alongb) {
		return equalCrossing{}, false
	}
	oa, a, ra, ok := cylinderOfExtrusion(&oft, alongt)
	if !ok {
		return equalCrossing{}, false
	}
	ob, bdir, rb, ok := cylinderOfExtrusion(&ofb, alongb)
	if !ok {
		return equalCrossing{}, false
	}
	if math.Abs(ra-rb) > lengthEps {
		return equalCrossing{}, false
	}
	cross := a.Cross(bdir)
	if cross.Magnitude() < 1e-6 {
		return equalCrossing{}, false // parallel: coaxial or apart, not crossing
	}
	ta, tb := geom.ClosestPointBetweenLines(oa, a, ob, bdir)
	pa, pb := oa.Plus(a.ScaledBy(ta)), ob.Plus(bdir.ScaledBy(tb))
	if pa.Minus(pb).Magnitude() > lengthEps {
		return equalCrossing{}, false // the axes pass apart: a curve the marcher follows
	}
	e1 := cross.WithMagnitude(1)
	return equalCrossing{
		o: pa.Plus(pb).ScaledBy(0.5), a: a, b: bdir,
		e1: e1, e2: a.Cross(e1).WithMagnitude(1), r: (ra + rb) / 2,
	}, true
}

// touchesOn appends to pts the points on the exact curve c where one of
// its surfaces, srfA or srfB, touches tangent a cylinder of sh of the
// same radius whose axis crosses its own: the operands' own edges that
// pass there, as a tee's branch's end circle does when the branch ends
// at the main pipe's axis, must have a vertex at the point, where the
// intersection's four branches meet, or their chords cross the other
// pipe's mesh beside it.
func touchesOn(pts *[]geom.Vector, c *SCurve, srfA, srfB *SSurface, sh *SShell) {
	if sh == nil || !c.IsExact {
		return
	}
	for _, own := range []*SSurface{srfA, srfB} {
		if own == nil || own.DegN != 1 {
			continue
		}
		for i := range sh.Surface {
			other := &sh.Surface[i]
			if other.DegN != 1 {
				continue
			}
			ec, ok := equalCylindersOf(own, other)
			if !ok {
				continue
			}
			for _, sgn := range []float64{1, -1} {
				p := ec.o.Plus(ec.e1.ScaledBy(sgn * ec.r))
				t, _ := c.Exact.ClosestPointTo(p, false)
				if c.Exact.PointAt(t).Minus(p).Magnitude() < lengthEps &&
					!slices.ContainsFunc(*pts, func(q geom.Vector) bool { return q.Equals(p) }) {
					*pts = append(*pts, p)
				}
			}
		}
	}
}

func (s *SSurface) intersectEqualCylinders(b *SSurface, agnstA, agnstB *SShell, into *curveSink) bool {
	ec, ok := equalCylindersOf(s, b)
	if !ok {
		return false
	}
	o, a, bdir, e1, e2, r := ec.o, ec.a, ec.b, ec.e1, ec.e2, ec.r
	angleOf := func(p geom.Vector) float64 {
		q := p.Minus(o)
		t := math.Atan2(q.Dot(e2), q.Dot(e1))
		if t < 0 {
			t += 2 * math.Pi
		}
		return t
	}
	onPatch := func(srf *SSurface, p geom.Vector) bool {
		const tol = 1e-6
		var puv geom.Point2d
		srf.ClosestPointTo(p, &puv, true)
		return puv.U > -tol && puv.U < 1+tol && puv.V > -tol && puv.V < 1+tol &&
			srf.PointAt(puv.U, puv.V).Minus(p).Magnitude() < s.Tol.ChordTolMm()/10 &&
			(srf.Bsp == nil || srf.Bsp.ClassifyPoint(puv, geom.Point2d{}, srf) != BspUvOutside)
	}
	for _, sign := range []float64{1, -1} {
		n := a.Plus(bdir.ScaledBy(sign))
		if n.Magnitude() < 1e-6 {
			continue
		}
		n = n.WithMagnitude(1)
		na := n.Dot(a)
		// The ellipse is the circle about a's axis through o, each point
		// moved along a onto the plane.
		mv := func(p geom.Vector) geom.Vector {
			return p.Minus(a.ScaledBy(n.Dot(p.Minus(o)) / na))
		}
		at := func(t float64) geom.Vector {
			return mv(o.Plus(e1.ScaledBy(r * math.Cos(t))).Plus(e2.ScaledBy(r * math.Sin(t))))
		}
		g := func(p geom.Vector) float64 { return n.Dot(p.Minus(o)) }

		// The touching points are on every such ellipse: cut there, so
		// that the four branches meet in one vertex.
		cuts := []float64{0, math.Pi}
		side := func(f func(t float64) geom.Vector) {
			if f(0).Minus(f(1)).Magnitude() < lengthEps && f(0.5).Minus(f(0)).Magnitude() < lengthEps {
				return
			}
			const steps = 32
			t0, g0 := 0.0, g(f(0))
			for i := 1; i <= steps; i++ {
				t1 := float64(i) / steps
				g1 := g(f(t1))
				if g0 == 0 {
					cuts = append(cuts, angleOf(f(t0)))
				} else if g0*g1 < 0 {
					lo, hi, glo := t0, t1, g0
					for range 60 {
						m := (lo + hi) / 2
						gm := g(f(m))
						if gm*glo <= 0 {
							hi = m
						} else {
							lo, glo = m, gm
						}
					}
					cuts = append(cuts, angleOf(f((lo+hi)/2)))
				}
				t0, g0 = t1, g1
			}
			if g0 == 0 {
				cuts = append(cuts, angleOf(f(1)))
			}
		}
		for _, srf := range []*SSurface{s, b} {
			side(func(t float64) geom.Vector { return srf.PointAt(t, 0) })
			side(func(t float64) geom.Vector { return srf.PointAt(t, 1) })
			side(func(t float64) geom.Vector { return srf.PointAt(0, t) })
			side(func(t float64) geom.Vector { return srf.PointAt(1, t) })
		}
		// The faces' outlines, where they are less than the patches.
		for _, pair := range []struct {
			srf *SSurface
			sh  *SShell
		}{{s, agnstA}, {b, agnstB}} {
			if pair.sh == nil {
				continue
			}
			for _, tr := range pair.srf.Trim {
				sc := pair.sh.FindCurve(tr.Curve)
				if sc == nil {
					continue
				}
				crosses := false
				for k := 1; k < len(sc.Points); k++ {
					g0, g1 := g(sc.Points[k-1].P), g(sc.Points[k].P)
					if g0 == 0 || g0*g1 < 0 {
						crosses = true
						break
					}
				}
				if !crosses {
					continue
				}
				if !sc.IsExact {
					return false
				}
				side(sc.Exact.PointAt)
			}
		}
		slices.Sort(cuts)
		cuts = slices.CompactFunc(cuts, func(x, y float64) bool { return math.Abs(x-y)*r < lengthEps })
		if len(cuts) > 1 && (cuts[0]+2*math.Pi-cuts[len(cuts)-1])*r < lengthEps {
			cuts = cuts[:len(cuts)-1]
		}
		for i, t0 := range cuts {
			t1 := cuts[(i+1)%len(cuts)]
			if t1 <= t0 {
				t1 += 2 * math.Pi
			}
			mid := at((t0 + t1) / 2)
			if !onPatch(s, mid) || !onPatch(b, mid) {
				continue
			}
			k := max(int(math.Ceil((t1-t0)/(math.Pi/2)*(1-1e-9))), 1)
			dt := (t1 - t0) / float64(k)
			w := math.Cos(dt / 2)
			for j := range k {
				s0 := t0 + float64(j)*dt
				s1 := s0 + dt
				sm := (s0 + s1) / 2
				p1 := mv(o.Plus(e1.ScaledBy(r / w * math.Cos(sm))).Plus(e2.ScaledBy(r / w * math.Sin(sm))))
				arc := SBezierFrom3(at(s0), p1, at(s1))
				arc.Weight[1] = w
				s.addExactIntersectionCurve(&arc, b, agnstA, agnstB, into)
			}
		}
	}
	return true
}

// hasEqualCrossing reports whether a and b hold patches on cylinders of
// one radius whose axes cross.
func hasEqualCrossing(a, b *SShell) bool {
	cyl := func(sh *SShell) []*SSurface {
		var out []*SSurface
		for i := range sh.Surface {
			s := &sh.Surface[i]
			var of SBezier
			var along geom.Vector
			if s.DegN == 1 && s.IsExtrusion(&of, &along) {
				if _, _, _, ok := cylinderOfExtrusion(&of, along); ok {
					out = append(out, s)
				}
			}
		}
		return out
	}
	cb := cyl(b)
	for _, sa := range cyl(a) {
		for _, sb := range cb {
			if _, ok := equalCylindersOf(sa, sb); ok {
				return true
			}
		}
	}
	return false
}

// failCrossingTouch marks the Boolean of a and b made into s failed if
// they hold equal cylinders crossing and its mesh crosses itself or has
// naked edges. A tee's difference keeps the branch's end, whose rim
// touches the main pipe tangent where the pipes do: within a chord
// tolerance's reach of those points it lies nearer the pipe than the
// pipe's chords, and the meshes can cross there. Regeneration checks no
// mesh, so the group would show a solid that isn't sound; failed, it
// says so.
func (s *SShell) failCrossingTouch(a, b *SShell) {
	if s.BooleanFailed || !hasEqualCrossing(a, b) {
		return
	}
	var m SMesh
	s.TriangulateInto(&m)
	var naked SEdgeList
	var inters, leaks bool
	SKdNodeFrom(&m).MakeCertainEdgesInto(&naked, EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
	if inters || leaks || len(naked.Edges) > 0 {
		s.BooleanFailed = true
	}
}

package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// sphereOfPatch reports the centre and radius of the sphere s lies on,
// if it does: curved both ways, and every point of a grid over it as far
// from the centre that four of its points give.
func sphereOfPatch(s *SSurface) (c geom.Vector, r float64, ok bool) {
	if s.DegM < 2 || s.DegN < 2 {
		return geom.Vector{}, 0, false
	}
	p0, p1 := s.PointAt(0.2, 0.2), s.PointAt(0.8, 0.2)
	p2, p3 := s.PointAt(0.2, 0.8), s.PointAt(0.7, 0.6)
	plane := func(q geom.Vector) (geom.Vector, float64) {
		n := q.Minus(p0).ScaledBy(2)
		return n, q.Dot(q) - p0.Dot(p0)
	}
	n1, d1 := plane(p1)
	n2, d2 := plane(p2)
	n3, d3 := plane(p3)
	c, parallel := geom.AtIntersectionOfThreePlanes(n1, d1, n2, d2, n3, d3)
	if parallel {
		return geom.Vector{}, 0, false
	}
	r = p0.Minus(c).Magnitude()
	if r <= lengthEps {
		return geom.Vector{}, 0, false
	}
	for u := 0.0; u <= 1; u += 0.25 {
		for v := 0.0; v <= 1; v += 0.25 {
			if math.Abs(s.PointAt(u, v).Minus(c).Magnitude()-r) > lengthEps {
				return geom.Vector{}, 0, false
			}
		}
	}
	return c, r, true
}

// isSpherePatch reports whether s lies on a sphere. The classification
// of edges on a sphere's patches allows for their chords dipping off it,
// which on other curved faces (threads, cylinders) moved results that
// were right.
func isSpherePatch(s *SSurface) bool {
	_, _, ok := sphereOfPatch(s)
	return ok
}

// intersectPlaneAndSphere adds where a plane meets a patch on a sphere as
// exact circular arcs, the circle cut where it crosses the patch's sides
// and its face's outline and kept where it lies on the face, and reports
// whether it did. A
// seat's rim cut by a plane through a ball is then the same exact curve
// in every Boolean that meets it, as a ball of the seat's radius united
// with the block (roadmap K3): marched, each Boolean followed other
// points, and the two faces' meshes did not meet there.
func (s *SSurface) intersectPlaneAndSphere(b *SSurface, agnstA, agnstB *SShell, into *curveSink) bool {
	splane, ssph := s, b
	if !(s.DegM == 1 && s.DegN == 1) {
		splane, ssph = b, s
	}
	c, r, ok := sphereOfPatch(ssph)
	if !ok {
		return false
	}
	n := splane.NormalAt(0, 0).WithMagnitude(1)
	d0 := n.Dot(splane.PointAt(0, 0))
	h := d0 - n.Dot(c)
	rho2 := r*r - h*h
	if rho2 <= 0 {
		return false
	}
	rho := math.Sqrt(rho2)
	if rho < 10*s.Tol.ChordTolMm() {
		// A circle about as small as the chord tolerance, where the plane
		// all but touches the sphere: the marcher's.
		return false
	}
	cc := c.Plus(n.ScaledBy(h))
	u := n.NormalDir(0).WithMagnitude(1)
	v := n.Cross(u).WithMagnitude(1)
	at := func(a float64) geom.Vector {
		return cc.Plus(u.ScaledBy(rho * math.Cos(a))).Plus(v.ScaledBy(rho * math.Sin(a)))
	}
	angleOf := func(p geom.Vector) float64 {
		q := p.Minus(cc)
		a := math.Atan2(q.Dot(v), q.Dot(u))
		if a < 0 {
			a += 2 * math.Pi
		}
		return a
	}

	// The circle's crossings of the patch's four sides.
	var cuts []float64
	side := func(f func(t float64) geom.Vector) {
		if f(0).Minus(f(1)).Magnitude() < lengthEps && f(0.5).Minus(f(0)).Magnitude() < lengthEps {
			return // a pole
		}
		g := func(t float64) float64 { return n.Dot(f(t)) - d0 }
		const steps = 32
		t0, g0 := 0.0, g(0)
		for i := 1; i <= steps; i++ {
			t1 := float64(i) / steps
			g1 := g(t1)
			if g0 == 0 {
				cuts = append(cuts, angleOf(f(t0)))
			} else if g0*g1 < 0 {
				lo, hi, glo := t0, t1, g0
				for range 60 {
					m := (lo + hi) / 2
					gm := g(m)
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
	side(func(t float64) geom.Vector { return ssph.PointAt(t, 0) })
	side(func(t float64) geom.Vector { return ssph.PointAt(t, 1) })
	side(func(t float64) geom.Vector { return ssph.PointAt(0, t) })
	side(func(t float64) geom.Vector { return ssph.PointAt(1, t) })
	// And of the face's own outline, where it is less than the patch (a
	// sphere read from STEP, its patch trimmed along a meridian). An
	// outline that isn't exact can't be crossed at the same point as its
	// split will be: the marcher's.
	sh := agnstA
	if ssph == b {
		sh = agnstB
	}
	if sh != nil {
		for _, tb := range ssph.Trim {
			sc := sh.FindCurve(tb.Curve)
			if sc == nil {
				continue
			}
			crosses := false
			for k := 1; k < len(sc.Points); k++ {
				g0 := n.Dot(sc.Points[k-1].P) - d0
				g1 := n.Dot(sc.Points[k].P) - d0
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
	cuts = slices.CompactFunc(cuts, func(a, b float64) bool { return math.Abs(a-b)*rho < lengthEps })
	if len(cuts) > 1 && (cuts[0]+2*math.Pi-cuts[len(cuts)-1])*rho < lengthEps {
		cuts = cuts[:len(cuts)-1]
	}

	onPatch := func(p geom.Vector) bool {
		const tol = 1e-6
		var puv geom.Point2d
		ssph.ClosestPointTo(p, &puv, true)
		return puv.U > -tol && puv.U < 1+tol && puv.V > -tol && puv.V < 1+tol &&
			ssph.PointAt(puv.U, puv.V).Minus(p).Magnitude() < s.Tol.ChordTolMm()/10 &&
			(ssph.Bsp == nil || ssph.Bsp.ClassifyPoint(puv, geom.Point2d{}, ssph) != BspUvOutside)
	}
	type span struct{ a0, a1 float64 }
	var spans []span
	switch len(cuts) {
	case 0:
		if !onPatch(at(0)) {
			return true // the plane meets the sphere, but not this patch
		}
		spans = []span{{0, 2 * math.Pi}}
	case 1:
		// The circle only touches a side: on the patch all round or not.
		if !onPatch(at(cuts[0] + math.Pi)) {
			return true
		}
		spans = []span{{cuts[0], cuts[0] + 2*math.Pi}}
	default:
		for i, a0 := range cuts {
			a1 := cuts[(i+1)%len(cuts)]
			if a1 <= a0 {
				a1 += 2 * math.Pi
			}
			if onPatch(at((a0 + a1) / 2)) {
				spans = append(spans, span{a0, a1})
			}
		}
	}
	for _, sp := range spans {
		k := int(math.Ceil((sp.a1 - sp.a0) / (math.Pi / 2) * (1 - 1e-9)))
		k = max(k, 1)
		da := (sp.a1 - sp.a0) / float64(k)
		w := math.Cos(da / 2)
		for i := range k {
			a0 := sp.a0 + float64(i)*da
			a1 := a0 + da
			p0, p2 := at(a0), at(a1)
			if i == 0 {
				p0 = at(sp.a0)
			}
			if i == k-1 {
				p2 = at(sp.a1)
			}
			am := (a0 + a1) / 2
			p1 := cc.Plus(u.ScaledBy(rho / w * math.Cos(am))).Plus(v.ScaledBy(rho / w * math.Sin(am)))
			arc := SBezierFrom3(p0, p1, p2)
			arc.Weight[1] = w
			s.addExactIntersectionCurve(&arc, b, agnstA, agnstB, into)
		}
	}
	return true
}

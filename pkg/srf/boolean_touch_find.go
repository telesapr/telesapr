package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A curve of one operand may touch a surface of the other tangent at a
// point, as a ring's inner circle touches a conical hole's circle lying
// in the ring's bottom (roadmap K3). There the two lie within LengthEps
// of each other for a long way either side, sqrt(2 R LengthEps) for a
// relative radius R (0.0045 at R = 10), and a split refined onto the
// curve and the surface lands anywhere in that stretch: the curve's
// chord crosses the surface twice a hair either side of the point, and
// each crossing is split and seeds a march, which stops at once at the
// other. The point itself is where the curve's tangent lies in the
// surface, a root that is well conditioned where the distance is not;
// found, it takes the crossings' place, and the Boolean's marches end
// there (touchSinks) and its other curves are split there
// (snapToKnownTouches).

// grazingTouch is the most the cosine between a curve and the normal of
// a surface it crosses may be for the crossing to be taken for a touch
// a little way along: past it the two cross.
const grazingTouch = 0.05

// exactTouch is the point where the exact curve c touches srf tangent,
// found from p, where c's chord from a to b crosses srf at a grazing
// angle: on c between a and b, where c's tangent lies in srf and c lies
// within LengthEps of it. It reports false where there is none: c is
// not exact or a line, crosses srf there, or the root runs off the
// segment or off the patch.
func exactTouch(c *SCurve, srf *SSurface, p, a, b geom.Vector) (geom.Vector, bool) {
	if !c.IsExact || c.Exact.Degree < 2 {
		return geom.Vector{}, false
	}
	ex := &c.Exact
	// g is the cosine between the curve at t and srf's normal at its
	// closest point there, the root sought.
	g := func(t float64) (float64, geom.Vector, float64) {
		q := ex.PointAt(t)
		var u, v float64
		srf.ClosestPointToUV(q, &u, &v, true)
		n := srf.NormalAt(u, v)
		d := ex.TangentAt(t)
		if n.Magnitude() < geom.LengthEps || d.Magnitude() < geom.LengthEps {
			return math.NaN(), q, math.Inf(1)
		}
		return d.WithMagnitude(1).Dot(n.WithMagnitude(1)), q, srf.PointAt(u, v).Minus(q).Magnitude()
	}
	t, _ := ex.ClosestPointTo(p, true)
	ta, _ := ex.ClosestPointTo(a, true)
	tb, _ := ex.ClosestPointTo(b, true)
	lo, hi := math.Min(ta, tb), math.Max(ta, tb)
	gt, _, _ := g(t)
	if !(math.Abs(gt) < grazingTouch) {
		return geom.Vector{}, false
	}
	const h = 1e-7
	for range 30 {
		g0, q, dist := g(t)
		if math.IsNaN(g0) {
			return geom.Vector{}, false
		}
		if math.Abs(g0) < 1e-12 {
			return touchAt(srf, q, dist, t, lo, hi)
		}
		g1, _, _ := g(t + h)
		dg := (g1 - g0) / h
		if dg == 0 || math.IsNaN(dg) {
			return geom.Vector{}, false
		}
		step := g0 / dg
		t -= step
		if t < lo-h || t > hi+h {
			return geom.Vector{}, false
		}
		if math.Abs(step) < 1e-14 {
			_, q, dist = g(t)
			return touchAt(srf, q, dist, t, lo, hi)
		}
	}
	return geom.Vector{}, false
}

// touchAt is q, the root exactTouch converged on at t, if it lies on
// srf's patch, within LengthEps of srf and on the segment's span of the
// curve, lo to hi.
func touchAt(srf *SSurface, q geom.Vector, dist, t, lo, hi float64) (geom.Vector, bool) {
	if dist > geom.LengthEps || t < lo-1e-9 || t > hi+1e-9 || offPatch(srf, q, geom.LengthEps) {
		return geom.Vector{}, false
	}
	return q, true
}

// addTouch puts p on the list of touching points found, unless it is
// there already.
func addTouch(list []geom.Vector, p geom.Vector) []geom.Vector {
	if slices.ContainsFunc(list, func(q geom.Vector) bool { return q.Equals(p) }) {
		return list
	}
	return append(list, p)
}

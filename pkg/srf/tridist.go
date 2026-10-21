package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// The nearest points of triangles, for MeshTree: the clearance between
// two parts is the least distance between their meshes' triangles.

// closestOnTriangle is the point of triangle abc nearest p, found by the
// region of the triangle's plane p projects into: a vertex, an edge or
// the inside.
func closestOnTriangle(p, a, b, c geom.Vector) geom.Vector {
	ab, ac, ap := b.Minus(a), c.Minus(a), p.Minus(a)
	d1, d2 := ab.Dot(ap), ac.Dot(ap)
	if d1 <= 0 && d2 <= 0 {
		return a
	}
	bp := p.Minus(b)
	d3, d4 := ab.Dot(bp), ac.Dot(bp)
	if d3 >= 0 && d4 <= d3 {
		return b
	}
	vc := d1*d4 - d3*d2
	if vc <= 0 && d1 >= 0 && d3 <= 0 {
		return a.Plus(ab.ScaledBy(d1 / (d1 - d3)))
	}
	cp := p.Minus(c)
	d5, d6 := ab.Dot(cp), ac.Dot(cp)
	if d6 >= 0 && d5 <= d6 {
		return c
	}
	vb := d5*d2 - d1*d6
	if vb <= 0 && d2 >= 0 && d6 <= 0 {
		return a.Plus(ac.ScaledBy(d2 / (d2 - d6)))
	}
	va := d3*d6 - d5*d4
	if va <= 0 && d4-d3 >= 0 && d5-d6 >= 0 {
		return b.Plus(c.Minus(b).ScaledBy((d4 - d3) / ((d4 - d3) + (d5 - d6))))
	}
	denom := va + vb + vc
	if denom <= 0 {
		// A triangle of no area: the nearest point of its sides.
		q := closestOnSegment(p, a, b)
		for _, r := range []geom.Vector{closestOnSegment(p, b, c), closestOnSegment(p, c, a)} {
			if r.Minus(p).MagSquared() < q.Minus(p).MagSquared() {
				q = r
			}
		}
		return q
	}
	v, w := vb/denom, vc/denom
	return a.Plus(ab.ScaledBy(v)).Plus(ac.ScaledBy(w))
}

// closestOnSegment is the point of segment ab nearest p.
func closestOnSegment(p, a, b geom.Vector) geom.Vector {
	ab := b.Minus(a)
	l := ab.MagSquared()
	if l == 0 {
		return a
	}
	t := min(max(p.Minus(a).Dot(ab)/l, 0), 1)
	return a.Plus(ab.ScaledBy(t))
}

// closestOnSegments are the nearest points of segments p0p1 and q0q1.
func closestOnSegments(p0, p1, q0, q1 geom.Vector) (geom.Vector, geom.Vector) {
	d1, d2, r := p1.Minus(p0), q1.Minus(q0), p0.Minus(q0)
	a, e, f := d1.MagSquared(), d2.MagSquared(), d2.Dot(r)
	var s, t float64
	switch {
	case a == 0 && e == 0:
		return p0, q0
	case a == 0:
		t = min(max(f/e, 0), 1)
	default:
		c := d1.Dot(r)
		if e == 0 {
			s = min(max(-c/a, 0), 1)
			break
		}
		b := d1.Dot(d2)
		if denom := a*e - b*b; denom > 0 {
			s = min(max((b*f-c*e)/denom, 0), 1)
		}
		t = (b*s + f) / e
		if t < 0 {
			t, s = 0, min(max(-c/a, 0), 1)
		} else if t > 1 {
			t, s = 1, min(max((b-c)/a, 0), 1)
		}
	}
	return p0.Plus(d1.ScaledBy(s)), q0.Plus(d2.ScaledBy(t))
}

// rayTriangle is where the line p + t·dir meets triangle abc, by t, and
// the cosine of the angle between dir and the triangle's normal (by its
// winding, b−a × c−a), positive where the ray leaves through a face
// whose normal points out; false where it misses or runs along the
// plane.
func rayTriangle(p, dir, a, b, c geom.Vector) (t, cos float64, ok bool) {
	e1, e2 := b.Minus(a), c.Minus(a)
	h := dir.Cross(e2)
	det := e1.Dot(h)
	if math.Abs(det) < 1e-14*e1.Magnitude()*e2.Magnitude()*dir.Magnitude() {
		return 0, 0, false
	}
	s := p.Minus(a)
	u := s.Dot(h) / det
	if u < 0 || u > 1 {
		return 0, 0, false
	}
	q := s.Cross(e1)
	v := dir.Dot(q) / det
	if v < 0 || u+v > 1 {
		return 0, 0, false
	}
	n := e1.Cross(e2)
	return e2.Dot(q) / det, n.Dot(dir) / (n.Magnitude() * dir.Magnitude()), true
}

// segmentCrossesTriangle is where segment p0p1 passes through triangle
// abc, if it does.
func segmentCrossesTriangle(p0, p1, a, b, c geom.Vector) (geom.Vector, bool) {
	d := p1.Minus(p0)
	t, _, ok := rayTriangle(p0, d, a, b, c)
	if !ok || t < 0 || t > 1 {
		return geom.Vector{}, false
	}
	return p0.Plus(d.ScaledBy(t)), true
}

// triangleDistance is the least distance between triangles t and o and
// the points of each where it is, 0 where they meet. Two triangles that
// don't meet are nearest at a vertex of one and the other, or at an
// edge of each; two that do have an edge of one through the other.
func triangleDistance(t, o *STriangle) (d float64, p, q geom.Vector) {
	tv := [3]geom.Vector{t.A, t.B, t.C}
	ov := [3]geom.Vector{o.A, o.B, o.C}
	for i := range 3 {
		if x, ok := segmentCrossesTriangle(tv[i], tv[(i+1)%3], o.A, o.B, o.C); ok {
			return 0, x, x
		}
		if x, ok := segmentCrossesTriangle(ov[i], ov[(i+1)%3], t.A, t.B, t.C); ok {
			return 0, x, x
		}
	}
	best := math.Inf(1)
	try := func(a, b geom.Vector) {
		if dd := a.Minus(b).MagSquared(); dd < best {
			best, p, q = dd, a, b
		}
	}
	for i := range 3 {
		try(tv[i], closestOnTriangle(tv[i], o.A, o.B, o.C))
		try(closestOnTriangle(ov[i], t.A, t.B, t.C), ov[i])
		for j := range 3 {
			try(closestOnSegments(tv[i], tv[(i+1)%3], ov[j], ov[(j+1)%3]))
		}
	}
	return math.Sqrt(best), p, q
}

package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A side the foot runs into may lean to the face the fillet rolls on, as
// a drafted plate's side does. The face is still widened past it along
// itself: the slab is the side swept across the face, square to the
// side's edge on it, so that the slab's top lies in the face. The box
// that cuts it back lies on the side's own plane, upward along it, and
// reaches above the face by the fillet's size measured along the plane.

// maxSlant is the most the side's normal may lean to the face's: a cosine
// of 0.8, a side some 53° off square. Past it the slab grows long and
// thin, the side's plane meeting the face at a sharp angle.
const maxSlant = 0.8

// slabAlong is the way the face is widened past a side of normal ns, for
// a face of normal n: across the face, square to the side's edge on it,
// outward. For a side square to the face it is ns itself, and slant is
// false. ok is false where the side leans more than maxSlant.
func slabAlong(ns, n geom.Vector) (along geom.Vector, slant, ok bool) {
	c := ns.Dot(n)
	if math.Abs(c) <= 1e-9 {
		return ns, false, true
	}
	if math.Abs(c) > maxSlant {
		return geom.Vector{}, false, false
	}
	return ns.Minus(n.ScaledBy(c)).Normal(), true, true
}

// slantUp is, for a side leaning to the face (normal n), the way up its
// own plane, square to its edge on the face, and how far along it the
// fillet's size r above the face reaches.
func slantUp(s *runOffSide, n geom.Vector, r float64) (up geom.Vector, reach float64) {
	up = n.Minus(s.n.ScaledBy(n.Dot(s.n))).Normal()
	return up, r / up.Dot(n)
}

// slabCorners is the corners of side s the box that cuts the slab of
// depth back must reach across: the side's own, and for a leaning side
// those of the slab's far end too, which lie lower or higher along the
// side's plane than the side by how far the slab runs.
func slabCorners(s *runOffSide, depth float64) []geom.Vector {
	if !s.slant {
		return s.poly
	}
	shift := s.along.ScaledBy(depth / s.along.Dot(s.n))
	out := slices.Clone(s.poly)
	for _, q := range s.poly {
		out = append(out, q.Plus(shift))
	}
	return out
}

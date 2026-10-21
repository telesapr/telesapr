package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
)

// RoundFace is a cylinder or a cone, as a face of a solid finds it: the
// surfaces a sketch can be wrapped onto (Wrap).
type RoundFace struct {
	// Point is a point of the axis; a cone's apex.
	Point geom.Vector
	// Axis is the unit axis; a cone opens along it from its apex.
	Axis geom.Vector
	// Radius is a cylinder's radius, 0 for a cone.
	Radius float64
	// HalfAngle is a cone's half angle, in radians, 0 for a cylinder.
	HalfAngle float64
	// Outward is +1 when the face's outward normal points away from the
	// axis, as a boss's does, and -1 when it points towards it, as a
	// bore's does.
	Outward float64
}

// IsCone reports whether f is a cone rather than a cylinder.
func (f RoundFace) IsCone() bool { return f.HalfAngle != 0 }

// roundFaceTol is how far, relative to the face's size, a sample of the
// patch may be from the cylinder or cone found for it.
const roundFaceTol = 1e-7

// RoundFaceOf finds the cylinder or cone patch s lies on: a patch
// straight one way, its lines parallel and as far from their axis (a
// cylinder) or meeting at one point and at one angle to their axis (a
// cone). False for any other patch, a plane included.
func RoundFaceOf(s *SSurface) (RoundFace, bool) {
	// The patch's lines at three places across it: along v at fixed u
	// when it is linear in v, else along u.
	var line func(w float64) (p, q geom.Vector)
	switch {
	case s.DegN == 1:
		line = func(w float64) (geom.Vector, geom.Vector) { return s.PointAt(w, 0), s.PointAt(w, 1) }
	case s.DegM == 1:
		line = func(w float64) (geom.Vector, geom.Vector) { return s.PointAt(0, w), s.PointAt(1, w) }
	default:
		return RoundFace{}, false
	}
	var ps, qs [3]geom.Vector
	size := 0.0
	for i, w := range []float64{0, 0.5, 1} {
		ps[i], qs[i] = line(w)
		size = max(size, qs[i].Minus(ps[i]).Magnitude(), ps[i].Minus(ps[0]).Magnitude())
	}
	if size < geom.LengthEps {
		return RoundFace{}, false
	}
	tol := roundFaceTol*size + geom.LengthEps/10
	d0 := qs[0].Minus(ps[0])
	parallel := true
	for i := 1; i < 3; i++ {
		di := qs[i].Minus(ps[i])
		if d0.Cross(di).Magnitude() > 1e-9*d0.Magnitude()*di.Magnitude() {
			parallel = false
		}
	}
	var f RoundFace
	if parallel {
		f.Axis = d0.WithMagnitude(1)
		// The lines' feet in the plane square to them through the first.
		var feet [3]geom.Vector
		for i := range 3 {
			feet[i] = ps[i].Minus(f.Axis.ScaledBy(ps[i].Minus(ps[0]).Dot(f.Axis)))
		}
		c, ok := circumcentre(feet[0], feet[1], feet[2])
		if !ok {
			return RoundFace{}, false
		}
		f.Point = c
		f.Radius = feet[0].Minus(c).Magnitude()
	} else {
		apex, ok := meetOfLines(ps, qs, tol)
		if !ok {
			return RoundFace{}, false
		}
		// A unit step along each line away from the apex: the three ends
		// lie on a circle about the axis.
		var ends [3]geom.Vector
		for i := range 3 {
			far := qs[i]
			if ps[i].Minus(apex).Magnitude() > far.Minus(apex).Magnitude() {
				far = ps[i]
			}
			g := far.Minus(apex)
			if g.Magnitude() < geom.LengthEps {
				return RoundFace{}, false
			}
			ends[i] = apex.Plus(g.WithMagnitude(1))
		}
		c, ok := circumcentre(ends[0], ends[1], ends[2])
		if !ok {
			return RoundFace{}, false
		}
		f.Point = apex
		f.Axis = c.Minus(apex).WithMagnitude(1)
		f.HalfAngle = libm.Acos(math.Max(-1, math.Min(1, ends[0].Minus(apex).Dot(f.Axis))))
		if f.HalfAngle < 1e-6 || f.HalfAngle > math.Pi/2-1e-6 {
			return RoundFace{}, false
		}
	}
	// Every sample of the patch lies on it.
	for i := range 5 {
		for j := range 5 {
			p := s.PointAt(float64(i)/4, float64(j)/4)
			if math.Abs(f.offset(p)) > tol {
				return RoundFace{}, false
			}
		}
	}
	p := s.PointAt(0.5, 0.5)
	n := s.NormalAt(0.5, 0.5)
	if n.Dot(f.normalAt(p)) < 0 {
		f.Outward = -1
	} else {
		f.Outward = 1
	}
	return f, true
}

// offset is how far p is from the surface, along its normal away from
// the axis.
func (f RoundFace) offset(p geom.Vector) float64 {
	rel := p.Minus(f.Point)
	along := rel.Dot(f.Axis)
	across := rel.Minus(f.Axis.ScaledBy(along)).Magnitude()
	if !f.IsCone() {
		return across - f.Radius
	}
	// The distance from the cone's line in the half plane through p.
	return across*libm.Cos(f.HalfAngle) - along*libm.Sin(f.HalfAngle)
}

// normalAt is the unit normal of the surface at p, pointing away from
// the axis (whatever Outward says).
func (f RoundFace) normalAt(p geom.Vector) geom.Vector {
	rel := p.Minus(f.Point)
	radial := rel.Minus(f.Axis.ScaledBy(rel.Dot(f.Axis)))
	if radial.Magnitude() < geom.LengthEps {
		return geom.Vector{}
	}
	radial = radial.WithMagnitude(1)
	if !f.IsCone() {
		return radial
	}
	return radial.ScaledBy(libm.Cos(f.HalfAngle)).Minus(f.Axis.ScaledBy(libm.Sin(f.HalfAngle)))
}

// circumcentre is the centre of the circle through a, b and c; false
// when they lie on a line.
func circumcentre(a, b, c geom.Vector) (geom.Vector, bool) {
	ab, ac := b.Minus(a), c.Minus(a)
	n := ab.Cross(ac)
	nn := n.MagSquared()
	if nn < 1e-18*ab.MagSquared()*ac.MagSquared() || nn == 0 {
		return geom.Vector{}, false
	}
	// a + (|ac|² (n × ab) + |ab|² (ac × n)) / (2 |n|²)
	v := n.Cross(ab).ScaledBy(ac.MagSquared()).Plus(ac.Cross(n).ScaledBy(ab.MagSquared()))
	return a.Plus(v.ScaledBy(1 / (2 * nn))), true
}

// meetOfLines is the point where the lines through ps[i] and qs[i] meet,
// within tol; false when they don't meet in one point.
func meetOfLines(ps, qs [3]geom.Vector, tol float64) (geom.Vector, bool) {
	d0 := qs[0].Minus(ps[0])
	closest := func(i int) (geom.Vector, bool) {
		di := qs[i].Minus(ps[i])
		n := d0.Cross(di)
		if n.Magnitude() < 1e-9*d0.Magnitude()*di.Magnitude() {
			return geom.Vector{}, false
		}
		// The point of line 0 on the plane through line i holding n.
		m := di.Cross(n)
		den := d0.Dot(m)
		if math.Abs(den) < 1e-18 {
			return geom.Vector{}, false
		}
		return ps[0].Plus(d0.ScaledBy(ps[i].Minus(ps[0]).Dot(m) / den)), true
	}
	p1, ok1 := closest(1)
	p2, ok2 := closest(2)
	if !ok1 || !ok2 || p1.Minus(p2).Magnitude() > tol*100 {
		return geom.Vector{}, false
	}
	return p1.Plus(p2).ScaledBy(0.5), true
}

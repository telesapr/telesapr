package geom

import "math"

// LengthEps and VeryPositive mirror C++ LENGTH_EPS and VERY_POSITIVE
// (solvespace/src/defs.h:7-8).
const (
	LengthEps    = 1e-6
	VeryPositive = 1e10
)

type Point2d struct {
	U, V float64
}

func NewPoint2d(u, v float64) Point2d {
	return Point2d{U: u, V: v}
}

func (p Point2d) Plus(b Point2d) Point2d {
	return Point2d{U: p.U + b.U, V: p.V + b.V}
}

func (p Point2d) Minus(b Point2d) Point2d {
	return Point2d{U: p.U - b.U, V: p.V - b.V}
}

func (p Point2d) ScaledBy(s float64) Point2d {
	return Point2d{U: p.U * s, V: p.V * s}
}

func (p Point2d) Equals(b Point2d) bool {
	return p == b
}

func (p Point2d) Dot(b Point2d) float64 {
	return p.U*b.U + p.V*b.V
}

func (p Point2d) MagSquared() float64 {
	return p.U*p.U + p.V*p.V
}

func (p Point2d) Magnitude() float64 {
	return math.Sqrt(p.U*p.U + p.V*p.V)
}

func (p Point2d) DistanceTo(b Point2d) float64 {
	return p.Minus(b).Magnitude()
}

// Normal rotates p by -90°, i.e. {v, -u}. Mirrors C++
// Point2d::Normal (util.cpp:985).
func (p Point2d) Normal() Point2d {
	return Point2d{U: p.V, V: -p.U}
}

// WithMagnitude mirrors C++ Point2d::WithMagnitude (util.cpp:934).
func (p Point2d) WithMagnitude(v float64) Point2d {
	m := p.Magnitude()
	if m < 1e-20 {
		return Point2d{U: v}
	}
	return Point2d{U: p.U * v / m, V: p.V * v / m}
}

// DistanceToLine returns the distance from p to the line p0 + t*dp,
// clamped to t in [0, 1] when asSegment. Mirrors C++
// Point2d::DistanceToLine (util.cpp:953).
func (p Point2d) DistanceToLine(p0, dp Point2d, asSegment bool) float64 {
	m := dp.MagSquared()
	if m < LengthEps*LengthEps {
		return VeryPositive
	}
	t := (dp.U*(p.U-p0.U) + dp.V*(p.V-p0.V)) / m
	if asSegment {
		if t < 0 {
			return p.DistanceTo(p0)
		}
		if t > 1 {
			return p.DistanceTo(p0.Plus(dp))
		}
	}
	return p.DistanceTo(p0.Plus(dp.ScaledBy(t)))
}

// DivProjected returns the parameter t such that delta*t is the
// projection of p onto delta. Mirrors C++ Point2d::DivProjected.
func (p Point2d) DivProjected(delta Point2d) float64 {
	return (p.U*delta.U + p.V*delta.V) / delta.MagSquared()
}

// EqualsTol mirrors C++ Point2d::Equals (util.cpp:989).
func (p Point2d) EqualsTol(b Point2d, tol float64) bool {
	du := b.U - p.U
	if du < -tol || du > tol {
		return false
	}
	dv := b.V - p.V
	if dv < -tol || dv > tol {
		return false
	}
	return p.Minus(b).MagSquared() < tol*tol
}

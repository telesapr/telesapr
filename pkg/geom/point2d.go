package geom

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/libm"
)

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

// Equals is EqualsTol with LENGTH_EPS (C++ Point2d::Equals's default).
func (p Point2d) Equals(b Point2d) bool {
	return p.EqualsTol(b, LengthEps)
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

// Point2dFromPolar is the point at radius r and angle a (C++
// Point2d::FromPolar).
func Point2dFromPolar(r, a float64) Point2d {
	return Point2d{U: r * libm.Cos(a), V: r * libm.Sin(a)}
}

// Angle is p's angle, in [0, 2 pi] (C++ Point2d::Angle).
func (p Point2d) Angle() float64 {
	a := libm.Atan2(p.V, p.U)
	return math.Pi + math.Remainder(a-math.Pi, 2*math.Pi)
}

// AngleTo is the angle of the direction from p to b (C++
// Point2d::AngleTo).
func (p Point2d) AngleTo(b Point2d) float64 {
	return b.Minus(p).Angle()
}

// DistanceToLineSigned is p's signed distance from the line p0 + t*dp,
// positive on the side of dp's normal; with asSegment, the distance to
// the nearer end past either (C++ Point2d::DistanceToLineSigned).
func (p Point2d) DistanceToLineSigned(p0, dp Point2d, asSegment bool) float64 {
	m := dp.U*dp.U + dp.V*dp.V
	if m < LengthEps*LengthEps {
		return VeryPositive
	}

	n := dp.Normal().WithMagnitude(1.0)
	dist := n.Dot(p) - n.Dot(p0)
	if asSegment {
		// Let our line be p = p0 + t*dp, for a scalar t from 0 to 1
		t := (dp.U*(p.U-p0.U) + dp.V*(p.V-p0.V)) / m
		sign := -1.0
		if dist > 0.0 {
			sign = 1.0
		}
		if t < 0.0 {
			return p.DistanceTo(p0) * sign
		}
		if t > 1.0 {
			return p.DistanceTo(p0.Plus(dp)) * sign
		}
	}

	return dist
}

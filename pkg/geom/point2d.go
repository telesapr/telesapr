package geom

import (
	"math"

	"github.com/telesapr/telesapr/pkg/libm"
)

// LengthEps is the tolerance lengths are compared with, and
// VeryPositive a length beyond any model.
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

// Equals is EqualsTol with LENGTH_EPS.
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

// Normal rotates p by -90°, i.e. {v, -u}.
func (p Point2d) Normal() Point2d {
	return Point2d{U: p.V, V: -p.U}
}

// WithMagnitude is the point scaled to the given length.
func (p Point2d) WithMagnitude(v float64) Point2d {
	m := p.Magnitude()
	if m < 1e-20 {
		return Point2d{U: v}
	}
	return Point2d{U: p.U * v / m, V: p.V * v / m}
}

// DistanceToLine returns the distance from p to the line p0 + t*dp,
// clamped to t in [0, 1] when asSegment.
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
// projection of p onto delta.
func (p Point2d) DivProjected(delta Point2d) float64 {
	return (p.U*delta.U + p.V*delta.V) / delta.MagSquared()
}

// EqualsTol reports whether the points are within tol of each other.
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

// Point2dFromPolar is the point at radius r and angle a.
func Point2dFromPolar(r, a float64) Point2d {
	return Point2d{U: r * libm.Cos(a), V: r * libm.Sin(a)}
}

// Angle is p's angle, in [0, 2 pi].
func (p Point2d) Angle() float64 {
	a := libm.Atan2(p.V, p.U)
	return math.Pi + math.Remainder(a-math.Pi, 2*math.Pi)
}

// AngleTo is the angle of the direction from p to b.
func (p Point2d) AngleTo(b Point2d) float64 {
	return b.Minus(p).Angle()
}

// DistanceToLineSigned is p's signed distance from the line p0 + t*dp,
// positive on the side of dp's normal; with asSegment, the distance to
// the nearer end past either.
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

// Pi is math.Pi as a variable. Go folds constant expressions exactly
// and rounds once, so 30*math.Pi/180 can differ in the last bit from
// the same expression rounded at each step, which the goldens record;
// expressions with Pi round at each step.
var Pi = math.Pi

package geom

import (
	"math"

	"github.com/telesapr/telesapr/pkg/libm"
)

var QuaternionIdentity = Quaternion{W: 1, X: 0, Y: 0, Z: 0}

type Quaternion struct {
	W, X, Y, Z float64
}

func NewQuaternion(w, x, y, z float64) Quaternion {
	return Quaternion{W: w, X: x, Y: y, Z: z}
}

// QuaternionFromAxisAngle is the rotation by dtheta about axis.
func QuaternionFromAxisAngle(axis Vector, dtheta float64) Quaternion {
	c, s := libm.Cos(dtheta/2), libm.Sin(dtheta/2)
	axis = axis.WithMagnitude(s)
	return Quaternion{W: c, X: axis.X, Y: axis.Y, Z: axis.Z}
}

// QuaternionFromBasis builds the quaternion that rotates the X axis
// onto u and the Y axis onto v. Caller must ensure u and v are
// perpendicular unit vectors.
func QuaternionFromBasis(u, v Vector) Quaternion {
	n := u.Cross(v)

	var q Quaternion
	tr := 1 + u.X + v.Y + n.Z
	if tr > 1e-4 {
		s := 2 * math.Sqrt(tr)
		q.W = s / 4
		q.X = (v.Z - n.Y) / s
		q.Y = (n.X - u.Z) / s
		q.Z = (u.Y - v.X) / s
	} else {
		switch {
		case u.X > v.Y && u.X > n.Z:
			s := 2 * math.Sqrt(1+u.X-v.Y-n.Z)
			q.W = (v.Z - n.Y) / s
			q.X = s / 4
			q.Y = (u.Y + v.X) / s
			q.Z = (n.X + u.Z) / s

		case v.Y > n.Z:
			s := 2 * math.Sqrt(1-u.X+v.Y-n.Z)
			q.W = (n.X - u.Z) / s
			q.X = (u.Y + v.X) / s
			q.Y = s / 4
			q.Z = (v.Z + n.Y) / s

		default:
			s := 2 * math.Sqrt(1-u.X-v.Y+n.Z)
			q.W = (u.Y - v.X) / s
			q.X = (n.X + u.Z) / s
			q.Y = (v.Z + n.Y) / s
			q.Z = s / 4
		}
	}

	return q.WithMagnitude(1)
}

func (q Quaternion) Plus(b Quaternion) Quaternion {
	return Quaternion{q.W + b.W, q.X + b.X, q.Y + b.Y, q.Z + b.Z}
}

func (q Quaternion) Minus(b Quaternion) Quaternion {
	return Quaternion{q.W - b.W, q.X - b.X, q.Y - b.Y, q.Z - b.Z}
}

func (q Quaternion) ScaledBy(s float64) Quaternion {
	return Quaternion{q.W * s, q.X * s, q.Y * s, q.Z * s}
}

func (q Quaternion) Magnitude() float64 {
	return math.Sqrt(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
}

func (q Quaternion) MagSquared() float64 {
	return q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z
}

// WithMagnitude is q scaled to magnitude s.
func (q Quaternion) WithMagnitude(s float64) Quaternion {
	return q.ScaledBy(s / q.Magnitude())
}

func (q Quaternion) Normal() Quaternion {
	mag := q.Magnitude()
	if mag < 1e-20 {
		return QuaternionIdentity
	}
	return q.ScaledBy(1.0 / mag)
}

// Times is the product q b.
func (q Quaternion) Times(b Quaternion) Quaternion {
	sa, sb := q.W, b.W
	va := Vector{q.X, q.Y, q.Z}
	vb := Vector{b.X, b.Y, b.Z}

	var r Quaternion
	r.W = sa*sb - va.Dot(vb)
	vr := vb.ScaledBy(sa).Plus(
		va.ScaledBy(sb).Plus(
			va.Cross(vb)))
	r.X, r.Y, r.Z = vr.X, vr.Y, vr.Z
	return r
}

// Inverse is the inverse rotation.
func (q Quaternion) Inverse() Quaternion {
	r := Quaternion{W: q.W, X: -q.X, Y: -q.Y, Z: -q.Z}
	return r.WithMagnitude(1) // not that the normalize should be reqd
}

// Mirror is the orientation with U and V reversed.
func (q Quaternion) Mirror() Quaternion {
	u, v := q.RotationU(), q.RotationV()
	u = u.ScaledBy(-1)
	v = v.ScaledBy(-1)
	return QuaternionFromBasis(u, v)
}

// ToThe is q, a unit quaternion, raised to the power p.
func (q Quaternion) ToThe(p float64) Quaternion {
	// Avoid division by zero, or arccos of something not in its domain
	if q.W >= (1 - 1e-6) {
		return Quaternion{W: 1}
	} else if q.W <= (-1 + 1e-6) {
		return Quaternion{W: -1}
	}

	axis := Vector{q.X, q.Y, q.Z}
	theta := libm.Acos(q.W) // okay, since magnitude is 1, so -1 <= w <= 1
	theta *= p
	var r Quaternion
	r.W = libm.Cos(theta)
	axis = axis.WithMagnitude(libm.Sin(theta))
	r.X, r.Y, r.Z = axis.X, axis.Y, axis.Z
	return r
}

// IsIdentity reports whether the quaternion is the identity (1, 0, 0, 0).
func (q Quaternion) IsIdentity() bool {
	return q.W == 1.0 && q.X == 0 && q.Y == 0 && q.Z == 0
}

// Rotate expresses v in the basis (RotationU, RotationV, RotationN).
// For a non-unit quaternion this scales by |q|^2.
func (q Quaternion) Rotate(v Vector) Vector {
	return q.RotationU().ScaledBy(v.X).Plus(
		q.RotationV().ScaledBy(v.Y)).Plus(
		q.RotationN().ScaledBy(v.Z))
}

func (q Quaternion) RotationU() Vector {
	return Vector{
		X: q.W*q.W + q.X*q.X - q.Y*q.Y - q.Z*q.Z,
		Y: 2*q.W*q.Z + 2*q.X*q.Y,
		Z: 2*q.X*q.Z - 2*q.W*q.Y,
	}
}

func (q Quaternion) RotationV() Vector {
	return Vector{
		X: 2*q.X*q.Y - 2*q.W*q.Z,
		Y: q.W*q.W - q.X*q.X + q.Y*q.Y - q.Z*q.Z,
		Z: 2*q.W*q.X + 2*q.Y*q.Z,
	}
}

func (q Quaternion) RotationN() Vector {
	return Vector{
		X: 2*q.W*q.Y + 2*q.X*q.Z,
		Y: 2*q.Y*q.Z - 2*q.W*q.X,
		Z: q.W*q.W - q.X*q.X - q.Y*q.Y + q.Z*q.Z,
	}
}

func (q Quaternion) Equals(b Quaternion) bool {
	if math.Abs(q.W-b.W) > 1e-8 {
		return false
	}
	if math.Abs(q.X-b.X) > 1e-8 {
		return false
	}
	if math.Abs(q.Y-b.Y) > 1e-8 {
		return false
	}
	if math.Abs(q.Z-b.Z) > 1e-8 {
		return false
	}
	return true
}

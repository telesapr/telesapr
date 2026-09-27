package geom

import "math"

var QuaternionIdentity = Quaternion{W: 1, X: 0, Y: 0, Z: 0}

type Quaternion struct {
	W, X, Y, Z float64
}

func NewQuaternion(w, x, y, z float64) Quaternion {
	return Quaternion{W: w, X: x, Y: y, Z: z}
}

func QuaternionFromAxisAngle(axis Vector, angle float64) Quaternion {
	s := math.Sin(angle / 2)
	c := math.Cos(angle / 2)
	n := axis.Normal()
	return Quaternion{
		W: c,
		X: n.X * s,
		Y: n.Y * s,
		Z: n.Z * s,
	}
}

// QuaternionFromBasis builds the quaternion that rotates the X axis
// onto u and the Y axis onto v. Caller must ensure u and v are
// perpendicular unit vectors. Mirrors C++ Quaternion::From(Vector u,
// Vector v) (util.cpp), including its branch choice and sign.
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
		if u.X > v.Y && u.X > n.Z {
			s := 2 * math.Sqrt(1+u.X-v.Y-n.Z)
			q.W = (v.Z - n.Y) / s
			q.X = s / 4
			q.Y = (u.Y + v.X) / s
			q.Z = (n.X + u.Z) / s
		} else if v.Y > n.Z {
			s := 2 * math.Sqrt(1-u.X+v.Y-n.Z)
			q.W = (n.X - u.Z) / s
			q.X = (u.Y + v.X) / s
			q.Y = s / 4
			q.Z = (v.Z + n.Y) / s
		} else {
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

func (q Quaternion) WithMagnitude(s float64) Quaternion {
	mag := q.Magnitude()
	if mag < 1e-20 {
		return QuaternionIdentity.ScaledBy(s)
	}
	return q.ScaledBy(s / mag)
}

func (q Quaternion) Normal() Quaternion {
	mag := q.Magnitude()
	if mag < 1e-20 {
		return QuaternionIdentity
	}
	return q.ScaledBy(1.0 / mag)
}

func (q Quaternion) Times(b Quaternion) Quaternion {
	return Quaternion{
		W: q.W*b.W - q.X*b.X - q.Y*b.Y - q.Z*b.Z,
		X: q.W*b.X + q.X*b.W + q.Y*b.Z - q.Z*b.Y,
		Y: q.W*b.Y - q.X*b.Z + q.Y*b.W + q.Z*b.X,
		Z: q.W*b.Z + q.X*b.Y - q.Y*b.X + q.Z*b.W,
	}
}

func (q Quaternion) Inverse() Quaternion {
	return Quaternion{W: q.W, X: -q.X, Y: -q.Y, Z: -q.Z}.Normal()
}

func (q Quaternion) Mirror() Quaternion {
	return Quaternion{W: -q.W, X: -q.X, Y: -q.Y, Z: -q.Z}
}

// ToThe returns q raised to a real power, used for parametric rotations.
func (q Quaternion) ToThe(p float64) Quaternion {
	if math.Abs(p) < 1e-12 {
		return QuaternionIdentity
	}
	norm := q.Normal()
	cosHalf := norm.W
	if cosHalf > 1 {
		cosHalf = 1
	}
	if cosHalf < -1 {
		cosHalf = -1
	}
	halfAngle := math.Acos(cosHalf)
	if norm.W < 0 {
		halfAngle -= math.Pi
	}
	sinHalf := math.Sin(halfAngle)
	if math.Abs(sinHalf) < 1e-12 {
		return norm
	}
	axis := Vector{X: norm.X / sinHalf, Y: norm.Y / sinHalf, Z: norm.Z / sinHalf}
	return QuaternionFromAxisAngle(axis, 2*halfAngle*p)
}

// IsIdentity reports whether the quaternion is the identity (1, 0, 0, 0).
func (q Quaternion) IsIdentity() bool {
	return q.W == 1.0 && q.X == 0 && q.Y == 0 && q.Z == 0
}

// Rotate expresses v in the basis (RotationU, RotationV, RotationN),
// as C++ Quaternion::Rotate does. For a non-unit quaternion this
// scales by |q|^2.
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

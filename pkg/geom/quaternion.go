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
// perpendicular unit vectors.
func QuaternionFromBasis(u, v Vector) Quaternion {
	w := u.Cross(v).Normal()
	// Build rotation matrix columns (u, v, w) and convert to quaternion.
	m00, m01, m02 := u.X, v.X, w.X
	m10, m11, m12 := u.Y, v.Y, w.Y
	m20, m21, m22 := u.Z, v.Z, w.Z
	trace := m00 + m11 + m22
	switch {
	case trace > 0:
		s := 0.5 / math.Sqrt(trace+1.0)
		return Quaternion{
			W: 0.25 / s,
			X: (m21 - m12) * s,
			Y: (m02 - m20) * s,
			Z: (m10 - m01) * s,
		}
	case m00 > m11 && m00 > m22:
		s := 2.0 * math.Sqrt(1.0+m00-m11-m22)
		return Quaternion{
			W: (m21 - m12) / s,
			X: 0.25 * s,
			Y: (m01 + m10) / s,
			Z: (m02 + m20) / s,
		}
	case m11 > m22:
		s := 2.0 * math.Sqrt(1.0+m11-m00-m22)
		return Quaternion{
			W: (m02 - m20) / s,
			X: (m01 + m10) / s,
			Y: 0.25 * s,
			Z: (m12 + m21) / s,
		}
	default:
		s := 2.0 * math.Sqrt(1.0+m22-m00-m11)
		return Quaternion{
			W: (m10 - m01) / s,
			X: (m02 + m20) / s,
			Y: (m12 + m21) / s,
			Z: 0.25 * s,
		}
	}
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

func (q Quaternion) Rotate(v Vector) Vector {
	qv := Quaternion{0, v.X, v.Y, v.Z}
	qInv := q.Inverse()
	r := q.Times(qv).Times(qInv)
	return Vector{r.X, r.Y, r.Z}
}

func (q Quaternion) RotationU() Vector {
	return q.Rotate(Vector{1, 0, 0})
}

func (q Quaternion) RotationV() Vector {
	return q.Rotate(Vector{0, 1, 0})
}

func (q Quaternion) RotationN() Vector {
	return q.Rotate(Vector{0, 0, 1})
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


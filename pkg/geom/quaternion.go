package geom

import "math"

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

func (q Quaternion) Normal() Quaternion {
	mag := q.Magnitude()
	if mag < 1e-20 {
		return Quaternion{1, 0, 0, 0}
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

func (q Quaternion) Rotate(v Vector) Vector {
	qv := Quaternion{0, v.X, v.Y, v.Z}
	qInv := Quaternion{q.W, -q.X, -q.Y, -q.Z}
	result := q.Times(qv).Times(qInv)
	return Vector{result.X, result.Y, result.Z}
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

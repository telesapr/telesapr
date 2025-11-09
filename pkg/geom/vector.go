package geom

import "math"

type Vector struct {
	X, Y, Z float64
}

func NewVector(x, y, z float64) Vector {
	return Vector{X: x, Y: y, Z: z}
}

func (v Vector) Plus(b Vector) Vector {
	return Vector{v.X + b.X, v.Y + b.Y, v.Z + b.Z}
}

func (v Vector) Minus(b Vector) Vector {
	return Vector{v.X - b.X, v.Y - b.Y, v.Z - b.Z}
}

func (v Vector) ScaledBy(s float64) Vector {
	return Vector{v.X * s, v.Y * s, v.Z * s}
}

func (v Vector) Dot(b Vector) float64 {
	return v.X*b.X + v.Y*b.Y + v.Z*b.Z
}

func (v Vector) Cross(b Vector) Vector {
	return Vector{
		v.Y*b.Z - v.Z*b.Y,
		v.Z*b.X - v.X*b.Z,
		v.X*b.Y - v.Y*b.X,
	}
}

func (v Vector) Magnitude() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

func (v Vector) MagSquared() float64 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

func (v Vector) WithMagnitude(s float64) Vector {
	mag := v.Magnitude()
	if mag < 1e-20 {
		return Vector{s, 0, 0}
	}
	return v.ScaledBy(s / mag)
}

func (v Vector) Normal() Vector {
	return v.WithMagnitude(1.0)
}

func (v Vector) DistanceTo(b Vector) float64 {
	return v.Minus(b).Magnitude()
}

func (v Vector) Equals(b Vector) bool {
	return v.DistanceTo(b) < 1e-8
}

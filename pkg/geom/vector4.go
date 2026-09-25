package geom

type Vector4 struct {
	X, Y, Z, W float64
}

func NewVector4(x, y, z, w float64) Vector4 {
	return Vector4{X: x, Y: y, Z: z, W: w}
}

func Vector4FromVector(v Vector, w float64) Vector4 {
	return Vector4{X: v.X, Y: v.Y, Z: v.Z, W: w}
}

func (v Vector4) Plus(b Vector4) Vector4 {
	return Vector4{X: v.X + b.X, Y: v.Y + b.Y, Z: v.Z + b.Z, W: v.W + b.W}
}

func (v Vector4) Minus(b Vector4) Vector4 {
	return Vector4{X: v.X - b.X, Y: v.Y - b.Y, Z: v.Z - b.Z, W: v.W - b.W}
}

func (v Vector4) ScaledBy(s float64) Vector4 {
	return Vector4{X: v.X * s, Y: v.Y * s, Z: v.Z * s, W: v.W * s}
}

func (v Vector4) Project3d() Vector {
	if v.W == 0 {
		return Vector{X: v.X, Y: v.Y, Z: v.Z}
	}
	iw := 1.0 / v.W
	return Vector{X: v.X * iw, Y: v.Y * iw, Z: v.Z * iw}
}

// Blend performs De Casteljau linear interpolation of two Vector4
// values. Used by SBezier::SplitAt.
func (v Vector4) Blend(b Vector4, t float64) Vector4 {
	return Vector4{
		X: v.X + (b.X-v.X)*t,
		Y: v.Y + (b.Y-v.Y)*t,
		Z: v.Z + (b.Z-v.Z)*t,
		W: v.W + (b.W-v.W)*t,
	}
}

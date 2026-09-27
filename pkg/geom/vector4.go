package geom

// Vector4 is a point in homogeneous coordinates (w*x, w*y, w*z, w)
// (C++ Vector4).
type Vector4 struct {
	X, Y, Z, W float64
}

// Vector4From is (x, y, z, w) (C++ Vector4::From(w, x, y, z)).
func Vector4From(w, x, y, z float64) Vector4 {
	return Vector4{X: x, Y: y, Z: z, W: w}
}

// Vector4FromWeighted is v with weight w: (w*v, w) (C++
// Vector4::From(w, v)).
func Vector4FromWeighted(w float64, v Vector) Vector4 {
	return Vector4From(w, w*v.X, w*v.Y, w*v.Z)
}

// Vector4Blend is a*(1 - t) + b*t (C++ Vector4::Blend).
func Vector4Blend(a, b Vector4, t float64) Vector4 {
	return a.ScaledBy(1 - t).Plus(b.ScaledBy(t))
}

func (v Vector4) Plus(b Vector4) Vector4 {
	return Vector4From(v.W+b.W, v.X+b.X, v.Y+b.Y, v.Z+b.Z)
}

func (v Vector4) Minus(b Vector4) Vector4 {
	return Vector4From(v.W-b.W, v.X-b.X, v.Y-b.Y, v.Z-b.Z)
}

func (v Vector4) ScaledBy(s float64) Vector4 {
	return Vector4From(v.W*s, v.X*s, v.Y*s, v.Z*s)
}

// PerspectiveProject is the point (x/w, y/w, z/w) (C++
// Vector4::PerspectiveProject).
func (v Vector4) PerspectiveProject() Vector {
	return Vector{X: v.X / v.W, Y: v.Y / v.W, Z: v.Z / v.W}
}

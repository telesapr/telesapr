package geom

import (
	"math"
	"testing"
)

func almostEq(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

func TestVectorDotCross(t *testing.T) {
	x := Vector{1, 0, 0}
	y := Vector{0, 1, 0}
	if x.Dot(y) != 0 {
		t.Fatalf("dot(x,y) should be 0")
	}
	z := x.Cross(y)
	if !almostEq(z.X, 0, 1e-12) || !almostEq(z.Y, 0, 1e-12) || !almostEq(z.Z, 1, 1e-12) {
		t.Fatalf("cross(x,y) = %v, want {0,0,1}", z)
	}
}

func TestVectorRotatedAbout(t *testing.T) {
	v := Vector{1, 0, 0}
	rotated := v.RotatedAbout(Vector{}, Vector{0, 0, 1}, math.Pi/2)
	if rotated.DistanceTo(Vector{0, 1, 0}) > 1e-9 {
		t.Fatalf("rotation about z by 90deg should give (0,1,0), got %v", rotated)
	}
}

func TestVectorProject2d(t *testing.T) {
	u := Vector{1, 0, 0}
	v := Vector{0, 1, 0}
	p := Vector{3, 4, 5}.Project2d(u, v)
	if !almostEq(p.U, 3, 1e-12) || !almostEq(p.V, 4, 1e-12) {
		t.Fatalf("project2d = %v, want {3,4}", p)
	}
}

func TestVectorBoundingBoxIntersectsLine(t *testing.T) {
	amax := Vector{2, 2, 2}
	amin := Vector{0, 0, 0}
	a := Vector{-1, 1, 1}
	b := Vector{3, 1, 1}
	if !a.BoundingBoxIntersectsLine(amax, amin, a, b) {
		t.Fatalf("expected line to intersect box")
	}
}

func TestVectorClosestPointOnLine(t *testing.T) {
	v := Vector{5, 5, 0}
	p0 := Vector{0, 0, 0}
	dp := Vector{10, 0, 0}
	closest := v.ClosestPointOnLine(p0, dp)
	if closest.DistanceTo(Vector{5, 0, 0}) > 1e-9 {
		t.Fatalf("closest = %v, want {5,0,0}", closest)
	}
}

func TestQuaternionFromBasis(t *testing.T) {
	u := Vector{1, 0, 0}
	v := Vector{0, 1, 0}
	q := QuaternionFromBasis(u, v)
	if q.RotationU().DistanceTo(u) > 1e-9 {
		t.Fatalf("RotationU = %v, want u", q.RotationU())
	}
	if q.RotationV().DistanceTo(v) > 1e-9 {
		t.Fatalf("RotationV = %v, want v", q.RotationV())
	}
}

func TestQuaternionToThe(t *testing.T) {
	q := QuaternionFromAxisAngle(Vector{0, 0, 1}, math.Pi/4)
	q2 := q.ToThe(2.0)
	if q2.RotationN().DistanceTo(q.RotationN()) > 1e-9 {
		t.Fatalf("ToThe not idempotent under composition")
	}
}

func TestQuaternionInverse(t *testing.T) {
	q := QuaternionFromAxisAngle(Vector{0, 1, 0}, math.Pi/3)
	v := Vector{1, 0, 0}
	rotated := q.Rotate(v)
	roundTrip := q.Inverse().Rotate(rotated)
	if roundTrip.DistanceTo(v) > 1e-9 {
		t.Fatalf("inverse not exact, got %v want %v", roundTrip, v)
	}
}

func TestPoint2d(t *testing.T) {
	p := Point2d{3, 4}.ScaledBy(2)
	if p.U != 6 || p.V != 8 {
		t.Fatalf("scaled = %v", p)
	}
}

func TestVector4Blend(t *testing.T) {
	a := Vector4{X: 0, Y: 0, Z: 0, W: 1}
	b := Vector4{X: 2, Y: 2, Z: 2, W: 1}
	mid := a.Blend(b, 0.5)
	if mid.X != 1 || mid.Y != 1 || mid.Z != 1 {
		t.Fatalf("blend at 0.5 = %v", mid)
	}
}

func TestVectorAtIntersectionOfPlanes(t *testing.T) {
	// Plane XY (z=0): n=(0,0,1), d=0
	// Plane XZ (y=0): n=(0,1,0), d=0
	// Plane YZ (x=0): n=(1,0,0), d=0
	// Intersection: origin
	p := AtIntersectionOfPlanes(
		Vector{0, 0, 1}, 0,
		Vector{0, 1, 0}, 0,
		Vector{1, 0, 0}, 0,
	)
	if p.Magnitude() > 1e-9 {
		t.Fatalf("intersection of 3 coordinate planes = %v, want {0,0,0}", p)
	}
}

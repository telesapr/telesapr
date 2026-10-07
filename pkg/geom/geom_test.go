package geom

import (
	"math"
	"testing"
)

func almostEq(a, b, eps float64) bool {
	return math.Abs(a-b) < eps
}

func TestVectorDotCross(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	v := Vector{1, 0, 0}
	rotated := v.RotatedAbout(Vector{}, Vector{0, 0, 1}, math.Pi/2)
	if rotated.DistanceTo(Vector{0, 1, 0}) > 1e-9 {
		t.Fatalf("rotation about z by 90deg should give (0,1,0), got %v", rotated)
	}
}

func TestVectorProject2d(t *testing.T) {
	t.Parallel()
	u := Vector{1, 0, 0}
	v := Vector{0, 1, 0}
	p := Vector{3, 4, 5}.Project2d(u, v)
	if !almostEq(p.U, 3, 1e-12) || !almostEq(p.V, 4, 1e-12) {
		t.Fatalf("project2d = %v, want {3,4}", p)
	}
}

func TestVectorBoundingBoxIntersectsLine(t *testing.T) {
	t.Parallel()
	amax := Vector{2, 2, 2}
	amin := Vector{0, 0, 0}
	a := Vector{-1, 1, 1}
	b := Vector{3, 1, 1}
	if !BoundingBoxIntersectsLine(amax, amin, a, b, true) {
		t.Fatalf("expected line to intersect box")
	}
}

func TestVectorClosestPointOnLine(t *testing.T) {
	t.Parallel()
	v := Vector{5, 5, 0}
	p0 := Vector{0, 0, 0}
	dp := Vector{10, 0, 0}
	closest := v.ClosestPointOnLine(p0, dp)
	if closest.DistanceTo(Vector{5, 0, 0}) > 1e-9 {
		t.Fatalf("closest = %v, want {5,0,0}", closest)
	}
}

func TestQuaternionFromBasis(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	q := QuaternionFromAxisAngle(Vector{0, 0, 1}, math.Pi/4)
	q2 := q.ToThe(2.0)
	if q2.RotationN().DistanceTo(q.RotationN()) > 1e-9 {
		t.Fatalf("ToThe not idempotent under composition")
	}
}

func TestQuaternionInverse(t *testing.T) {
	t.Parallel()
	q := QuaternionFromAxisAngle(Vector{0, 1, 0}, math.Pi/3)
	v := Vector{1, 0, 0}
	rotated := q.Rotate(v)
	roundTrip := q.Inverse().Rotate(rotated)
	if roundTrip.DistanceTo(v) > 1e-9 {
		t.Fatalf("inverse not exact, got %v want %v", roundTrip, v)
	}
}

func TestPoint2d(t *testing.T) {
	t.Parallel()
	p := Point2d{3, 4}.ScaledBy(2)
	if p.U != 6 || p.V != 8 {
		t.Fatalf("scaled = %v", p)
	}
}

func TestVector4Blend(t *testing.T) {
	t.Parallel()
	a := Vector4{X: 0, Y: 0, Z: 0, W: 1}
	b := Vector4{X: 2, Y: 2, Z: 2, W: 1}
	mid := Vector4Blend(a, b, 0.5)
	if mid.X != 1 || mid.Y != 1 || mid.Z != 1 {
		t.Fatalf("blend at 0.5 = %v", mid)
	}
}

func TestVectorAtIntersectionOfPlanes(t *testing.T) {
	t.Parallel()
	// Plane XY (z=0): n=(0,0,1), d=0
	// Plane XZ (y=0): n=(0,1,0), d=0
	// Plane YZ (x=0): n=(1,0,0), d=0
	// Intersection: origin
	p, parallel := AtIntersectionOfThreePlanes(
		Vector{0, 0, 1}, 0,
		Vector{0, 1, 0}, 0,
		Vector{1, 0, 0}, 0,
	)
	if parallel || p.Magnitude() > 1e-9 {
		t.Fatalf("intersection of 3 coordinate planes = %v, want {0,0,0}", p)
	}
}

// TestVectorRotatedAboutPoint verifies rotation around a point+axis.
func TestVectorRotatedAboutPoint(t *testing.T) {
	t.Parallel()
	v := Vector{X: 1, Y: 0}
	rot := v.RotatedAbout(Vector{}, Vector{Z: 1}, math.Pi/2)
	if math.Abs(rot.X) > 1e-9 || math.Abs(rot.Y-1) > 1e-9 {
		t.Errorf("90° rot of (1,0) around Z = %v, want (0,1)", rot)
	}
}

// TestVectorRotatedAroundAxis verifies axis-only rotation.
func TestVectorRotatedAroundAxis(t *testing.T) {
	t.Parallel()
	v := Vector{Y: 1}
	rot := v.RotatedAroundAxis(Vector{X: 1}, math.Pi)
	// 180° around X-axis flips Y to -1.
	if math.Abs(rot.Y+1) > 1e-9 {
		t.Errorf("180° rot of (0,1) around X = %v, want (0,-1)", rot)
	}
}

// TestVectorProjectXy verifies projection onto XY plane.
func TestVectorProjectXy(t *testing.T) {
	t.Parallel()
	v := Vector{X: 3, Y: 4, Z: 5}
	p := v.ProjectXy()
	if p.U != 3 || p.V != 4 {
		t.Errorf("ProjectXy = (%v, %v), want (3, 4)", p.U, p.V)
	}
}

// TestVectorProject4d verifies projection to Vector4 (homogeneous).
func TestVectorProject4d(t *testing.T) {
	t.Parallel()
	v := Vector{X: 1, Y: 2, Z: 3}
	v4 := v.Project4d()
	if v4.W != 1 {
		t.Errorf("Project4d.W = %v, want 1", v4.W)
	}
	if v4.X != 1 || v4.Y != 2 || v4.Z != 3 {
		t.Errorf("Project4d = %v, want (1,2,3,1)", v4)
	}
}

// TestVectorClampWithin verifies clamping.
func TestVectorClampWithin(t *testing.T) {
	t.Parallel()
	v := Vector{X: 5, Y: -5, Z: 0}
	c := v.ClampWithin(0, 2)
	if c.X != 2 || c.Y != 0 || c.Z != 0 {
		t.Errorf("ClampWithin = %v, want (2, 0, 0)", c)
	}
}

// TestVectorDirectionCosineWith verifies dot/(|a||b|).
func TestVectorDirectionCosineWith(t *testing.T) {
	t.Parallel()
	a := Vector{X: 1}
	b := Vector{Y: 1}
	cos := a.DirectionCosineWith(b)
	if math.Abs(cos) > 1e-9 {
		t.Errorf("DirectionCosine(orthogonal) = %v, want 0", cos)
	}
}

// TestVectorEqualsExactly vs Equals verifies exact-equality check.
func TestVectorEqualsExactly(t *testing.T) {
	t.Parallel()
	a := Vector{X: 1.0}
	b := Vector{X: 1.0 + 1e-15}
	if !a.Equals(b) {
		t.Errorf("close-but-not-exact vectors should Equals (tolerance 1e-8)")
	}
	if !a.EqualsExactly(a) {
		t.Errorf("self should EqualsExactly")
	}
	if a.EqualsExactly(b) {
		t.Errorf("different vectors should not EqualsExactly")
	}
}

// TestAtIntersectionOfPlaneAndLine verifies the plane/line intersection.
func TestAtIntersectionOfPlaneAndLine(t *testing.T) {
	t.Parallel()
	// Plane z = 5, line from (0,0,0) along (0,0,1).
	n := Vector{Z: 1}
	d := 5.0
	a := Vector{}
	b := Vector{Z: 1}
	p := AtIntersectionOfPlaneAndLine(n, d, a, b)
	if math.Abs(p.Z-5) > 1e-9 {
		t.Errorf("intersection = %v, want z=5", p)
	}
}

// TestClosestPointBetweenLines verifies line-line proximity.
func TestClosestPointBetweenLines(t *testing.T) {
	t.Parallel()
	// Two skew lines: a is x-axis, b is parallel at z=1.
	a0, da := Vector{}, Vector{X: 1}
	b0, db := Vector{Z: 1}, Vector{X: 1}
	ta, tb := ClosestPointBetweenLines(a0, da, b0, db)
	if math.Abs(ta) > 1e-9 || math.Abs(tb) > 1e-9 {
		t.Errorf("parallel lines ta=%v tb=%v, want 0", ta, tb)
	}
}

// TestVectorBoundingBoxesDisjoint verifies bbox overlap check.
func TestVectorBoundingBoxesDisjoint(t *testing.T) {
	t.Parallel()
	amax := Vector{X: 1, Y: 1, Z: 1}
	amin := Vector{X: 0, Y: 0, Z: 0}
	bmax := Vector{X: 2, Y: 2, Z: 2}
	bmin := Vector{X: -1, Y: -1, Z: -1}
	if amin.BoundingBoxesDisjoint(amax, amin, bmax, bmin) {
		t.Errorf("overlapping bboxes should not be disjoint")
	}
	c := Vector{X: 3, Y: 3, Z: 3}
	d := Vector{X: 2, Y: 2, Z: 2}
	if !amin.BoundingBoxesDisjoint(amax, amin, c, d) {
		t.Errorf("separated bboxes should be disjoint")
	}
}

// TestVectorOnLineSegment verifies point-on-segment check.
func TestVectorOnLineSegment(t *testing.T) {
	t.Parallel()
	mid := Vector{X: 0.5}
	if !mid.OnLineSegment(Vector{}, Vector{X: 1}, 1e-9) {
		t.Errorf("(0.5,0,0) should be on segment (0,0,0)-(1,0,0)")
	}
	off := Vector{X: 0.5, Y: 1}
	if off.OnLineSegment(Vector{}, Vector{X: 1}, 1e-9) {
		t.Errorf("(0.5,1,0) should not be on segment (0,0,0)-(1,0,0)")
	}
}

// TestVectorCsys checks DotInToCsys and ScaleOutOfCsys as C++ defines
// them: components along u, v, n, and the combination of u, v, n.
func TestVectorCsys(t *testing.T) {
	t.Parallel()
	u := Vector{Y: 2}
	w := Vector{Z: 1}
	n := Vector{X: 3}
	p := Vector{X: 1, Y: 2, Z: 3}
	if got := p.DotInToCsys(u, w, n); got != (Vector{X: 4, Y: 3, Z: 3}) {
		t.Errorf("DotInToCsys = %v, want (4, 3, 3)", got)
	}
	if got := p.ScaleOutOfCsys(u, w, n); got != (Vector{X: 9, Y: 2, Z: 2}) {
		t.Errorf("ScaleOutOfCsys = %v, want (9, 2, 2)", got)
	}
}

// TestVectorElement verifies Element accessor.
func TestVectorElement(t *testing.T) {
	t.Parallel()
	v := Vector{X: 1, Y: 2, Z: 3}
	if v.Element(0) != 1 || v.Element(1) != 2 || v.Element(2) != 3 {
		t.Errorf("Element = (%v, %v, %v), want (1, 2, 3)",
			v.Element(0), v.Element(1), v.Element(2))
	}
}

// TestVectorOutsideAndNotOn verifies AABB-miss check.
func TestVectorOutsideAndNotOn(t *testing.T) {
	t.Parallel()
	v := Vector{X: 5, Y: 5, Z: 5}
	if !v.OutsideAndNotOn(Vector{X: 10}, Vector{}) {
		t.Errorf("v inside should not be outside-and-not-on")
	}
}

// TestVectorBoundingBoxIntersectsLineThroughOrigin verifies bbox/line overlap.
func TestVectorBoundingBoxIntersectsLineThroughOrigin(t *testing.T) {
	t.Parallel()
	amax := Vector{X: 1, Y: 1, Z: 1}
	amin := Vector{}
	a := Vector{}
	b := Vector{X: 2, Y: 0, Z: 0}
	if !BoundingBoxIntersectsLine(amax, amin, a, b, true) {
		t.Errorf("line through origin should intersect unit bbox")
	}
}

// TestVectorRotatedAboutTranslation verifies rotation around a non-origin point.
func TestVectorRotatedAboutTranslation(t *testing.T) {
	t.Parallel()
	v := Vector{X: 1, Y: 0}
	rot := v.RotatedAbout(Vector{X: 5, Y: 5}, Vector{Z: 1}, math.Pi/2)
	// Rotate (1,0) around (5,5) by 90 degrees = (5, 6)
	if math.Abs(rot.X-10) > 1e-9 || math.Abs(rot.Y-1) > 1e-9 {
		t.Errorf("rotated = %v, want (10, 1, 0)", rot)
	}
}

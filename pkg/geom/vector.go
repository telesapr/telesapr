package geom

import "math"

type Vector struct {
	X, Y, Z float64
}

func NewVector(x, y, z float64) Vector {
	return Vector{X: x, Y: y, Z: z}
}

func VectorZero() Vector {
	return Vector{}
}

func (v Vector) Plus(b Vector) Vector {
	return Vector{v.X + b.X, v.Y + b.Y, v.Z + b.Z}
}

func (v Vector) Minus(b Vector) Vector {
	return Vector{v.X - b.X, v.Y - b.Y, v.Z - b.Z}
}

func (v Vector) Negated() Vector {
	return Vector{-v.X, -v.Y, -v.Z}
}

func (v Vector) ScaledBy(s float64) Vector {
	return Vector{v.X * s, v.Y * s, v.Z * s}
}

func (v Vector) Element(i int) float64 {
	switch i {
	case 0:
		return v.X
	case 1:
		return v.Y
	case 2:
		return v.Z
	}
	return 0
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

func (v Vector) DistanceToLine(p0, dp Vector) float64 {
	num := v.Minus(p0).Cross(dp).Magnitude()
	den := dp.Magnitude()
	if den < 1e-20 {
		return v.DistanceTo(p0)
	}
	return num / den
}

func (v Vector) DistanceToPlane(normal, origin Vector) float64 {
	n := normal.Normal()
	return v.Minus(origin).Dot(n)
}

func (v Vector) Equals(b Vector) bool {
	return v.DistanceTo(b) < 1e-8
}

func (v Vector) EqualsExactly(b Vector) bool {
	return v.X == b.X && v.Y == b.Y && v.Z == b.Z
}

func (v Vector) DirectionCosineWith(b Vector) float64 {
	ma := v.Magnitude()
	mb := b.Magnitude()
	if ma < 1e-20 || mb < 1e-20 {
		return 0
	}
	return v.Dot(b) / (ma * mb)
}

// ClosestPointOnLine returns the projection of v onto the line through
// p0 with direction dp.
func (v Vector) ClosestPointOnLine(p0, dp Vector) Vector {
	den := dp.Dot(dp)
	if den < 1e-20 {
		return p0
	}
	t := v.Minus(p0).Dot(dp) / den
	return p0.Plus(dp.ScaledBy(t))
}

// Project2d returns the 2-D coordinates of v expressed in the basis
// (u, v) of the workplane.
func (v Vector) Project2d(u, w Vector) Point2d {
	return Point2d{U: v.Dot(u), V: v.Dot(w)}
}

// ProjectXy returns the 2-D coordinates of v projected onto the XY plane.
func (v Vector) ProjectXy() Point2d {
	return Point2d{U: v.X, V: v.Y}
}

// Project4d returns the homogeneous Vector4 representation of v with
// unit weight.
func (v Vector) Project4d() Vector4 {
	return Vector4{X: v.X, Y: v.Y, Z: v.Z, W: 1}
}

// RotatedAbout rotates v around an axis through orig with direction
// axis by angle theta (radians). Right-hand rule.
func (v Vector) RotatedAbout(orig, axis Vector, theta float64) Vector {
	n := axis.Normal()
	q := QuaternionFromAxisAngle(n, theta)
	return q.Rotate(v.Minus(orig)).Plus(orig)
}

// RotatedAroundAxis is a shortcut for rotating v around an axis
// passing through the origin (saves the subtract/add of orig).
func (v Vector) RotatedAroundAxis(axis Vector, theta float64) Vector {
	return QuaternionFromAxisAngle(axis.Normal(), theta).Rotate(v)
}

// ClampWithin clamps each component of v to [minv, maxv].
func (v Vector) ClampWithin(minv, maxv Vector) Vector {
	clamp := func(x, lo, hi float64) float64 {
		if x < lo {
			return lo
		}
		if x > hi {
			return hi
		}
		return x
	}
	return Vector{
		X: clamp(v.X, minv.X, maxv.X),
		Y: clamp(v.Y, minv.Y, maxv.Y),
		Z: clamp(v.Z, minv.Z, maxv.Z),
	}
}

// OutsideAndNotOn returns true if v is outside the closed box
// [minv, maxv] OR on a face (not strictly inside).
func (v Vector) OutsideAndNotOn(maxv, minv Vector) bool {
	return v.X > maxv.X || v.X < minv.X ||
		v.Y > maxv.Y || v.Y < minv.Y ||
		v.Z > maxv.Z || v.Z < minv.Z
}

// BoundingBoxesDisjoint returns true if no point lies in both boxes.
func (v Vector) BoundingBoxesDisjoint(amax, amin, bmax, bmin Vector) bool {
	return amax.X < bmin.X || bmax.X < amin.X ||
		amax.Y < bmin.Y || bmax.Y < amin.Y ||
		amax.Z < bmin.Z || bmax.Z < amin.Z
}

// BoundingBoxIntersectsLine returns true if the line (a, b) crosses
// the closed box (amax/amin). Used by ray-cast pruning.
func (v Vector) BoundingBoxIntersectsLine(amax, amin, a, b Vector) bool {
	dir := b.Minus(a)
	if dir.Magnitude() < 1e-20 {
		return false
	}
	tmin, tmax := 0.0, 1.0
	axes := [3]float64{dir.X, dir.Y, dir.Z}
	orig := [3]float64{a.X, a.Y, a.Z}
	mins := [3]float64{amin.X, amin.Y, amin.Z}
	maxs := [3]float64{amax.X, amax.Y, amax.Z}
	for i := 0; i < 3; i++ {
		if axes[i] == 0 {
			if orig[i] < mins[i] || orig[i] > maxs[i] {
				return false
			}
			continue
		}
		t1 := (mins[i] - orig[i]) / axes[i]
		t2 := (maxs[i] - orig[i]) / axes[i]
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
		if tmin > tmax {
			return false
		}
	}
	return true
}

// DotInToCsys returns the dot product of v with the contravariant basis
// vector (u, w) of a coordinate system whose normal is n.
func (v Vector) DotInToCsys(u, w, n Vector) float64 {
	return v.Dot(n.Cross(u.Cross(n)).Normal().ScaledBy(u.Magnitude()))
}

// ScaleOutOfCsys is the inverse of DotInToCsys: given a coordinate
// value v projected onto u in (u, w, n) basis, return the
// corresponding 3-D vector. Used by Vector::ProjectInto.
func (v Vector) ScaleOutOfCsys(u, w, n Vector) Vector {
	return u.ScaledBy(v.Dot(n.Cross(w.Cross(n)).Normal()) / u.Magnitude())
}

// OnLineSegment returns true if v lies on segment [a, b] within tol.
func (v Vector) OnLineSegment(a, b Vector, tol float64) bool {
	ab := b.Minus(a)
	if ab.MagSquared() < 1e-20 {
		return v.Minus(a).MagSquared() < tol*tol
	}
	cross := v.Minus(a).Cross(ab)
	if cross.Magnitude() > tol*ab.Magnitude() {
		return false
	}
	t := v.Minus(a).Dot(ab) / ab.MagSquared()
	return t >= -tol && t <= 1+tol
}

// AtIntersectionOfLines returns the intersection of line (a0, a1)
// with line (b0, b1). Caller must ensure lines are not parallel.
func AtIntersectionOfLines(a0, a1, b0, b1 Vector) Vector {
	da := a1.Minus(a0)
	db := b1.Minus(b0)
	den := da.Cross(db).Magnitude()
	if den < 1e-20 {
		return a0
	}
	t := b0.Minus(a0).Cross(db).Dot(da.Cross(db)) / (den * den)
	return a0.Plus(da.ScaledBy(t))
}

// ClosestPointBetweenLines computes the parameters ta, tb on the two
// lines (a0, da) and (b0, db) such that a0 + ta*da is the closest
// point on the first line to the second. Lines need not be
// perpendicular but must not be parallel.
func ClosestPointBetweenLines(a0, da, b0, db Vector) (ta, tb float64) {
	dn := da.Cross(db)
	dna := dn.Cross(da)
	dnb := dn.Cross(db)
	tb = a0.Minus(b0).Dot(dna) / db.Dot(dna)
	ta = -a0.Minus(b0).Dot(dnb) / da.Dot(dnb)
	return ta, tb
}

// AtIntersectionOfPlaneAndLine returns the intersection of line
// (a, b) with plane (n, d) where n is the plane normal and d is the
// signed distance from the origin.
func AtIntersectionOfPlaneAndLine(n Vector, d float64, a, b Vector) Vector {
	dir := b.Minus(a)
	den := dir.Dot(n)
	if math.Abs(den) < 1e-20 {
		return a
	}
	t := (d - a.Dot(n)) / den
	return a.Plus(dir.ScaledBy(t))
}

// AtIntersectionOfPlanes returns the intersection of three planes
// (n1, d1), (n2, d2), (n3, d3). Caller must ensure planes are not
// parallel.
func AtIntersectionOfPlanes(n1 Vector, d1 float64, n2 Vector, d2 float64, n3 Vector, d3 float64) Vector {
	x := n2.Cross(n3).ScaledBy(d1)
	y := n3.Cross(n1).ScaledBy(d2)
	z := n1.Cross(n2).ScaledBy(d3)
	num := x.Plus(y).Plus(z)
	den := n1.Dot(n2.Cross(n3))
	if math.Abs(den) < 1e-20 {
		return Vector{}
	}
	return num.ScaledBy(1.0 / den)
}


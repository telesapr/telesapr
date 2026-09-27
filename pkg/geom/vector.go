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

// NormalDir returns one of two unit vectors perpendicular to v,
// selected by an arbitrary but well-defined pivot rule (which=0 or
// 1); which=1 is which=0 crossed with v, so the pair (Normal(0),
// Normal(1), v) forms a right-handed orthonormal basis. Mirrors the
// C++ `Vector::Normal(int)` (solvespace/src/util.cpp), used to build
// a coordinate system from a single direction vector, e.g.
// SShell::MakeFromExtrusionOf's cap-plane (u, v) basis.
func (v Vector) NormalDir(which int) Vector {
	var n Vector
	xa, ya, za := math.Abs(v.X), math.Abs(v.Y), math.Abs(v.Z)
	if v.Equals(Vector{X: 0, Y: 0, Z: 1}) {
		// Make DXFs exported in the XY plane work nicely...
		n = Vector{X: 1}
	} else if xa < ya && xa < za {
		n = Vector{X: 0, Y: v.Z, Z: -v.Y}
	} else if ya < za {
		n = Vector{X: -v.Z, Y: 0, Z: v.X}
	} else {
		n = Vector{X: v.Y, Y: -v.X, Z: 0}
	}
	if which == 1 {
		n = v.Cross(n)
	}
	return n.Normal()
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

// Equals mirrors C++ Vector::Equals with its default LENGTH_EPS
// tolerance (dsc.h:129).
func (v Vector) Equals(b Vector) bool {
	return v.EqualsTol(b, LengthEps)
}

// EqualsTol mirrors C++ Vector::Equals(v, tol).
func (v Vector) EqualsTol(b Vector, tol float64) bool {
	dx, dy, dz := v.X-b.X, v.Y-b.Y, v.Z-b.Z
	if math.Abs(dx) > tol || math.Abs(dy) > tol || math.Abs(dz) > tol {
		return false
	}
	return dx*dx+dy*dy+dz*dz < tol*tol
}

func (v Vector) EqualsExactly(b Vector) bool {
	return v.X == b.X && v.Y == b.Y && v.Z == b.Z
}

// MakeMaxMin expands `ptMax` and `ptMin` to include v. Used by
// SBezier::GetAxisAlignedBounding and SCurve::GetAxisAlignedBounding
// to compute a bounding box from a list of control points.
func (v Vector) MakeMaxMin(ptMax, ptMin *Vector) {
	if v.X > ptMax.X {
		ptMax.X = v.X
	}
	if v.Y > ptMax.Y {
		ptMax.Y = v.Y
	}
	if v.Z > ptMax.Z {
		ptMax.Z = v.Z
	}
	if v.X < ptMin.X {
		ptMin.X = v.X
	}
	if v.Y < ptMin.Y {
		ptMin.Y = v.Y
	}
	if v.Z < ptMin.Z {
		ptMin.Z = v.Z
	}
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

// DivProjected returns the scalar t such that
// v == origin + t * dir, i.e. (v - origin) / dir in the direction
// of dir. Mirrors Vector::DivProjected from solvespace's Vector.h.
func (v Vector) DivProjected(dir Vector) float64 {
	den := dir.MagSquared()
	if den < 1e-20 {
		return 0
	}
	return v.Dot(dir) / den
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

// OutsideAndNotOn reports whether v lies outside the box [minv, maxv]
// by more than LENGTH_EPS. Mirrors C++ Vector::OutsideAndNotOn.
func (v Vector) OutsideAndNotOn(maxv, minv Vector) bool {
	return (v.X > maxv.X+LengthEps) || (v.X < minv.X-LengthEps) ||
		(v.Y > maxv.Y+LengthEps) || (v.Y < minv.Y-LengthEps) ||
		(v.Z > maxv.Z+LengthEps) || (v.Z < minv.Z-LengthEps)
}

// BoundingBoxesDisjoint mirrors C++ Vector::BoundingBoxesDisjoint.
func (v Vector) BoundingBoxesDisjoint(amax, amin, bmax, bmin Vector) bool {
	for i := 0; i < 3; i++ {
		if amax.Element(i) < bmin.Element(i)-LengthEps {
			return true
		}
		if amin.Element(i) > bmax.Element(i)+LengthEps {
			return true
		}
	}
	return false
}

// BoundingBoxIntersectsLine reports whether the line (or, if
// asSegment, the segment) p0-p1 crosses a face of the box. Mirrors C++
// Vector::BoundingBoxIntersectsLine.
func BoundingBoxIntersectsLine(amax, amin, p0, p1 Vector, asSegment bool) bool {
	dp := p1.Minus(p0)
	lp := dp.Magnitude()
	dp = dp.ScaledBy(1.0 / lp)

	for i := 0; i < 3; i++ {
		j := (i + 1) % 3
		k := (i + 2) % 3
		if lp*math.Abs(dp.Element(i)) < LengthEps {
			continue // parallel to plane
		}

		for a := 0; a < 2; a++ {
			d := amin.Element(i)
			if a == 0 {
				d = amax.Element(i)
			}
			// n dot (p0 + t*dp) = d
			// (n dot p0) + t * (n dot dp) = d
			t := (d - p0.Element(i)) / dp.Element(i)
			p := p0.Plus(dp.ScaledBy(t))

			if asSegment && (t < -LengthEps || t > lp+LengthEps) {
				continue
			}

			if p.Element(j) > amax.Element(j)+LengthEps {
				continue
			}
			if p.Element(k) > amax.Element(k)+LengthEps {
				continue
			}

			if p.Element(j) < amin.Element(j)-LengthEps {
				continue
			}
			if p.Element(k) < amin.Element(k)-LengthEps {
				continue
			}

			return true
		}
	}

	return false
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
	// Mirrors C++ Vector::OnLineSegment (util.cpp:559).
	if v.EqualsTol(a, tol) || v.EqualsTol(b, tol) {
		return true
	}

	d := b.Minus(a)

	m := d.MagSquared()
	distsq := v.Minus(a).Cross(d).MagSquared() / m

	if distsq >= tol*tol {
		return false
	}

	t := v.Minus(a).DivProjected(d)
	// On-endpoint already tested
	if t < 0 || t > 1 {
		return false
	}
	return true
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

// AtIntersectionOfTwoPlanes returns a point on the line where planes
// (n1, d1) and (n2, d2) meet. Mirrors the two-plane overload of C++
// Vector::AtIntersectionOfPlanes (util.cpp:722).
func AtIntersectionOfTwoPlanes(n1 Vector, d1 float64, n2 Vector, d2 float64) Vector {
	det := n1.Dot(n1)*n2.Dot(n2) - n1.Dot(n2)*n1.Dot(n2)
	c1 := (d1*n2.Dot(n2) - d2*n1.Dot(n2)) / det
	c2 := (d2*n1.Dot(n1) - d1*n1.Dot(n2)) / det
	return n1.ScaledBy(c1).Plus(n2.ScaledBy(c2))
}

func det2(a1, b1, a2, b2 float64) float64 {
	return a1*b2 - b1*a2
}

func det3(a1, b1, c1, a2, b2, c2, a3, b3, c3 float64) float64 {
	return a1*det2(b2, c2, b3, c3) -
		b1*det2(a2, c2, a3, c3) +
		c1*det2(a2, b2, a3, b3)
}

// AtIntersectionOfThreePlanes returns the point where the three planes
// meet, reporting parallel when they don't meet in a point. Mirrors the
// three-plane overload of C++ Vector::AtIntersectionOfPlanes
// (util.cpp:812).
func AtIntersectionOfThreePlanes(na Vector, da float64, nb Vector, db float64, nc Vector, dc float64) (Vector, bool) {
	det := det3(na.X, na.Y, na.Z,
		nb.X, nb.Y, nb.Z,
		nc.X, nc.Y, nc.Z)
	if math.Abs(det) < 1e-10 { // arbitrary tolerance, not so good
		return Vector{}, true
	}

	detx := det3(da, na.Y, na.Z,
		db, nb.Y, nb.Z,
		dc, nc.Y, nc.Z)
	dety := det3(na.X, da, na.Z,
		nb.X, db, nb.Z,
		nc.X, dc, nc.Z)
	detz := det3(na.X, na.Y, da,
		nb.X, nb.Y, db,
		nc.X, nc.Y, dc)

	return Vector{X: detx / det, Y: dety / det, Z: detz / det}, false
}

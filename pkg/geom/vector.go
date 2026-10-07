package geom

import (
	"math"

	"github.com/telesapr/telesapr/pkg/libm"
)

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

// WithMagnitude is v scaled to magnitude s; a zero vector stays zero
// (C++ Vector::WithMagnitude).
func (v Vector) WithMagnitude(s float64) Vector {
	m := v.Magnitude()
	if m == 0 {
		// We can do a zero vector with zero magnitude, but not any other cases.
		return Vector{}
	}
	return v.ScaledBy(s / m)
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

// DistanceToLine is the distance from v to the line p0 + t*dp (C++
// Vector::DistanceToLine).
func (v Vector) DistanceToLine(p0, dp Vector) float64 {
	m := dp.Magnitude()
	return v.Minus(p0).Cross(dp).Magnitude() / m
}

// DistanceToPlane is v's signed distance from the plane through origin
// with normal (scaled by its magnitude, as C++ Vector::DistanceToPlane).
func (v Vector) DistanceToPlane(normal, origin Vector) float64 {
	return v.Dot(normal) - origin.Dot(normal)
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

// DirectionCosineWith is the cosine of the angle between v and b (C++
// Vector::DirectionCosineWith).
func (v Vector) DirectionCosineWith(b Vector) float64 {
	a := v.WithMagnitude(1)
	b = b.WithMagnitude(1)
	return a.Dot(b)
}

// ClosestPointOnLine is the point of the line p0 + t*dp closest to v
// (C++ Vector::ClosestPointOnLine).
func (v Vector) ClosestPointOnLine(p0, dp Vector) Vector {
	dp = dp.WithMagnitude(1)
	// this, p0, and (p0+dp) define a plane; the min distance is in
	// that plane, so calculate its normal
	pn := v.Minus(p0).Cross(dp)
	// The minimum distance line is in that plane, perpendicular
	// to the line
	n := pn.Cross(dp)

	// Calculate the actual distance
	d := dp.Cross(p0.Minus(v)).Magnitude()
	return v.Plus(n.WithMagnitude(d))
}

// DivProjected is v's length along delta, in units of delta (C++
// Vector::DivProjected).
func (v Vector) DivProjected(delta Vector) float64 {
	return (v.X*delta.X + v.Y*delta.Y + v.Z*delta.Z) /
		(delta.X*delta.X + delta.Y*delta.Y + delta.Z*delta.Z)
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

// RotatedAbout rotates v by theta about the axis through orig (C++
// Vector::RotatedAbout(orig, axis, theta)).
func (v Vector) RotatedAbout(orig, axis Vector, theta float64) Vector {
	r := v.Minus(orig)
	r = r.RotatedAroundAxis(axis, theta)
	return r.Plus(orig)
}

// RotatedAroundAxis rotates v by theta about the axis through the
// origin (C++ Vector::RotatedAbout(axis, theta)).
func (v Vector) RotatedAroundAxis(axis Vector, theta float64) Vector {
	c := libm.Cos(theta)
	s := libm.Sin(theta)

	axis = axis.WithMagnitude(1)

	var r Vector

	r.X = (v.X)*(c+(1-c)*(axis.X)*(axis.X)) +
		(v.Y)*((1-c)*(axis.X)*(axis.Y)-s*(axis.Z)) +
		(v.Z)*((1-c)*(axis.X)*(axis.Z)+s*(axis.Y))

	r.Y = (v.X)*((1-c)*(axis.Y)*(axis.X)+s*(axis.Z)) +
		(v.Y)*(c+(1-c)*(axis.Y)*(axis.Y)) +
		(v.Z)*((1-c)*(axis.Y)*(axis.Z)-s*(axis.X))

	r.Z = (v.X)*((1-c)*(axis.Z)*(axis.X)-s*(axis.Y)) +
		(v.Y)*((1-c)*(axis.Z)*(axis.Y)+s*(axis.X)) +
		(v.Z)*(c+(1-c)*(axis.Z)*(axis.Z))

	return r
}

// ClampWithin clamps each component to [minv, maxv] (C++
// Vector::ClampWithin).
// ClosestOrtho is the coordinate axis direction nearest v (C++
// Vector::ClosestOrtho).
func (v Vector) ClosestOrtho() Vector {
	mx, my, mz := math.Abs(v.X), math.Abs(v.Y), math.Abs(v.Z)
	sign := func(c float64) float64 {
		if c > 0 {
			return 1
		}
		return -1
	}
	switch {
	case mx > my && mx > mz:
		return Vector{X: sign(v.X)}
	case my > mz:
		return Vector{Y: sign(v.Y)}
	default:
		return Vector{Z: sign(v.Z)}
	}
}

func (v Vector) ClampWithin(minv, maxv float64) Vector {
	ret := v

	if ret.X < minv {
		ret.X = minv
	}
	if ret.Y < minv {
		ret.Y = minv
	}
	if ret.Z < minv {
		ret.Z = minv
	}

	if ret.X > maxv {
		ret.X = maxv
	}
	if ret.Y > maxv {
		ret.Y = maxv
	}
	if ret.Z > maxv {
		ret.Z = maxv
	}

	return ret
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
	for i := range 3 {
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

	for i := range 3 {
		j := (i + 1) % 3
		k := (i + 2) % 3
		if lp*math.Abs(dp.Element(i)) < LengthEps {
			continue // parallel to plane
		}

		for a := range 2 {
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

// DotInToCsys is v's components along u, w and n (C++
// Vector::DotInToCsys).
func (v Vector) DotInToCsys(u, w, n Vector) Vector {
	return Vector{v.Dot(u), v.Dot(w), v.Dot(n)}
}

// ScaleOutOfCsys takes v's components as coordinates in the (u, w, n)
// basis (C++ Vector::ScaleOutOfCsys).
func (v Vector) ScaleOutOfCsys(u, w, n Vector) Vector {
	return u.ScaledBy(v.X).Plus(w.ScaledBy(v.Y).Plus(n.ScaledBy(v.Z)))
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

// AtIntersectionOfLines is the point on line (a0, a1) closest to line
// (b0, b1), their intersection if they meet (C++
// Vector::AtIntersectionOfLines, without the skew and parameter
// outputs).
func AtIntersectionOfLines(a0, a1, b0, b1 Vector) Vector {
	da, db := a1.Minus(a0), b1.Minus(b0)

	pa, _ := ClosestPointBetweenLines(a0, da, b0, db)

	// And from either of those, we get the intersection point.
	return a0.Plus(da.ScaledBy(pa))
}

// AtIntersectionOfLinesSkew is AtIntersectionOfLines, also reporting
// whether the lines are skew: whether the closest points on them
// aren't coincident (C++ Vector::AtIntersectionOfLines with skew).
func AtIntersectionOfLinesSkew(a0, a1, b0, b1 Vector) (Vector, bool) {
	da, db := a1.Minus(a0), b1.Minus(b0)
	pa, pb := ClosestPointBetweenLines(a0, da, b0, db)
	pi := a0.Plus(da.ScaledBy(pa))
	// Check if the intersection points on each line are actually
	// coincident...
	return pi, !pi.Equals(b0.Plus(db.ScaledBy(pb)))
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

// AtIntersectionOfPlaneAndLine is where the line through p0 and p1
// meets the plane n . p = d; zero if they're parallel (C++
// Vector::AtIntersectionOfPlaneAndLine).
func AtIntersectionOfPlaneAndLine(n Vector, d float64, p0, p1 Vector) Vector {
	p, _ := AtIntersectionOfPlaneAndLineParallel(n, d, p0, p1)
	return p
}

// AtIntersectionOfPlaneAndLineParallel is AtIntersectionOfPlaneAndLine,
// also reporting whether the line and plane are parallel.
func AtIntersectionOfPlaneAndLineParallel(n Vector, d float64, p0, p1 Vector) (Vector, bool) {
	dp := p1.Minus(p0)

	if math.Abs(n.Dot(dp)) < LengthEps {
		return Vector{}, true
	}

	// n dot (p0 + t*dp) = d
	// (n dot p0) + t * (n dot dp) = d
	t := (d - n.Dot(p0)) / n.Dot(dp)

	return p0.Plus(dp.ScaledBy(t)), false
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

// Max is C++ std::max for doubles: b if a < b, else a. Unlike math.Max
// it keeps a when a and b are zeros of opposite sign (or either is
// NaN), and ported code relies on getting C++'s bits.
func Max(a, b float64) float64 {
	if a < b {
		return b
	}
	return a
}

// Min is C++ std::min for doubles: b if b < a, else a (see Max).
func Min(a, b float64) float64 {
	if b < a {
		return b
	}
	return a
}

// InPerspective takes v into the csys (u, v, n) at origin, and applies
// the perspective of cameraTan (C++ Vector::InPerspective). The csys is
// one where u cross v equals n, backwards from the display's.
func (v Vector) InPerspective(u, vv, n, origin Vector, cameraTan float64) Vector {
	r := v.Minus(origin)
	r = r.DotInToCsys(u, vv, n)
	// yes, minus; we are assuming a csys where u cross v equals n,
	// backwards from the display stuff
	w := 1 - r.Z*cameraTan
	r = r.ScaledBy(1 / w)

	return r
}

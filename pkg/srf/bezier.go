package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// lengthEps mirrors C++ LENGTH_EPS (defs.h:7).
const lengthEps = geom.LengthEps

// ratPolyEps mirrors C++ RATPOLY_EPS (ratpoly.cpp:16).
const ratPolyEps = lengthEps / 100

// veryPositive and veryNegative mirror C++ VERY_POSITIVE and
// VERY_NEGATIVE (defs.h).
const (
	veryPositive = 1e10
	veryNegative = -1e10
)

// isReasonable is C++ IsReasonable, which despite its name is true
// for NaN and huge values.
func isReasonable(x float64) bool {
	return math.IsNaN(x) || x > 1e11 || x < -1e11
}

// SBezier is a rational Bezier curve of degree 1..3 with up to 4
// weighted 3-D control points. Used for sketch curves (LineSegment,
// ArcOfCircle, Cubic), trim curves on shell surfaces, and for
// building SSurface extrusion/lathe profiles.
type SBezier struct {
	Degree int
	Ctrl   [MaxDegree + 1]geom.Vector // control points (only Ctrl[0..Degree] used)
	Weight [MaxDegree + 1]float64     // rational weights (1.0 = non-rational)

	Tag  int
	AuxA int
	AuxB int
	// Entity is the handle of the sketch entity (a line segment) the
	// curve came from, or 0 (C++ SBezier::entity).
	Entity uint32

	// Hint of which kind of curve this is, used by SShell trimming.
	IsCircleHint bool
	// Sheet is the sketch entity the curve is a piece of, any kind of
	// curve, or 0; the surfaces swept from it carry it (SSurface.Sheet).
	Sheet uint32
}

// SBezierFrom2 builds a line segment from a to b.
func SBezierFrom2(a, b geom.Vector) SBezier {
	return SBezierFromV4(a.Project4d(), b.Project4d())
}

// SBezierFrom3 builds a quadratic Bezier from three control points.
func SBezierFrom3(a, b, c geom.Vector) SBezier {
	return SBezierFromV4(a.Project4d(), b.Project4d(), c.Project4d())
}

// SBezierFrom4 builds a cubic Bezier from four control points.
func SBezierFrom4(a, b, c, d geom.Vector) SBezier {
	return SBezierFromV4(a.Project4d(), b.Project4d(), c.Project4d(), d.Project4d())
}

// Start returns the first control point.
func (b *SBezier) Start() geom.Vector {
	return b.Ctrl[0]
}

// Finish returns the last control point.
func (b *SBezier) Finish() geom.Vector {
	return b.Ctrl[b.Degree]
}

// IsRational reports whether a weight differs from 1 (C++
// SBezier::IsRational).
func (b *SBezier) IsRational() bool {
	for i := 0; i <= b.Degree; i++ {
		if mathAbs(b.Weight[i]-1.0) > lengthEps {
			return true
		}
	}
	return false
}

// Reverse reverses the curve in place (reverses control points and
// weights).
func (b *SBezier) Reverse() {
	for i, j := 0, b.Degree; i < j; i, j = i+1, j-1 {
		b.Ctrl[i], b.Ctrl[j] = b.Ctrl[j], b.Ctrl[i]
		b.Weight[i], b.Weight[j] = b.Weight[j], b.Weight[i]
	}
}

// ScaleSelfBy multiplies every control point by s.
func (b *SBezier) ScaleSelfBy(s float64) {
	for i := 0; i <= b.Degree; i++ {
		b.Ctrl[i] = b.Ctrl[i].ScaledBy(s)
	}
}

// TransformedBy applies (translation, rotation, scale) to every
// control point.
func (b *SBezier) TransformedBy(t geom.Vector, q geom.Quaternion, scale float64) {
	for i := 0; i <= b.Degree; i++ {
		b.Ctrl[i] = q.Rotate(b.Ctrl[i].ScaledBy(scale)).Plus(t)
	}
}

// IsInPlane reports whether every control point lies in the plane
// n . p = d (C++ SBezier::IsInPlane).
func (b *SBezier) IsInPlane(n geom.Vector, d float64) bool {
	for i := 0; i <= b.Degree; i++ {
		if mathAbs(b.Ctrl[i].Dot(n)-d) > lengthEps {
			return false
		}
	}
	return true
}

// PointAt evaluates the rational Bezier at parameter t.
func (b *SBezier) PointAt(t float64) geom.Vector {
	pt := geom.Vector{}
	d := 0.0

	for i := 0; i <= b.Degree; i++ {
		B := bernstein(i, b.Degree, t)
		pt = pt.Plus(b.Ctrl[i].ScaledBy(B * b.Weight[i]))
		d += b.Weight[i] * B
	}
	pt = pt.ScaledBy(1.0 / d)
	return pt
}

// TangentAt is the derivative of the curve at t (C++
// SBezier::TangentAt).
func (b *SBezier) TangentAt(t float64) geom.Vector {
	pt, ptP := geom.Vector{}, geom.Vector{}
	d, dP := 0.0, 0.0

	for i := 0; i <= b.Degree; i++ {
		B := bernstein(i, b.Degree, t)
		Bp := bernsteinDerivative(i, b.Degree, t)

		pt = pt.Plus(b.Ctrl[i].ScaledBy(B * b.Weight[i]))
		d += b.Weight[i] * B

		ptP = ptP.Plus(b.Ctrl[i].ScaledBy(Bp * b.Weight[i]))
		dP += b.Weight[i] * Bp
	}

	// quotient rule; f(t) = n(t)/d(t), so f' = (n'*d - n*d')/(d^2)
	ret := ptP.ScaledBy(d).Minus(pt.ScaledBy(dP))
	ret = ret.ScaledBy(1.0 / (d * d))
	return ret
}

// ClosestPointTo is the parameter t of the point on the curve closest
// to p: the closest of a few samples, refined by Newton's method. It
// reports whether that converged; C++ only complains when mustConverge
// (C++ SBezier::ClosestPointTo).
func (b *SBezier) ClosestPointTo(p geom.Vector, mustConverge bool) (float64, bool) {
	minDist := veryPositive
	t := 0.0
	res := 20.0
	if b.Degree <= 2 {
		res = 7.0
	}
	for i := 0; i < int(res); i++ {
		tryt := float64(i) / res

		tryp := b.PointAt(tryt)
		d := tryp.Minus(p).Magnitude()
		if d < minDist {
			t = tryt
			minDist = d
		}
	}

	iters := 5
	if mustConverge {
		iters = 15
	}
	for i := 0; i < iters; i++ {
		p0 := b.PointAt(t)
		if p0.EqualsTol(p, ratPolyEps) {
			return t, true
		}

		dp := b.TangentAt(t)
		pc := p.ClosestPointOnLine(p0, dp)
		t += pc.Minus(p0).DivProjected(dp)
	}
	return t, false
}

// SBezierFromV4 is the curve with homogeneous control points ct (C++
// SBezier::From(Vector4 ...)).
func SBezierFromV4(ct ...geom.Vector4) SBezier {
	ret := SBezier{Degree: len(ct) - 1}
	for i, c := range ct {
		ret.Weight[i] = c.W
		ret.Ctrl[i] = c.PerspectiveProject()
	}
	return ret
}

// SplitAt splits the curve at t into the part before and the part
// after, by de Casteljau's algorithm on the homogeneous control points
// (C++ SBezier::SplitAt).
func (b *SBezier) SplitAt(t float64, bef, aft *SBezier) {
	if b.Degree < 1 || b.Degree > MaxDegree {
		panic("Unexpected degree of spline")
	}
	var ct [MaxDegree + 1]geom.Vector4
	for i := 0; i <= b.Degree; i++ {
		ct[i] = geom.Vector4FromWeighted(b.Weight[i], b.Ctrl[i])
	}
	n := b.Degree
	var l, r [MaxDegree + 1]geom.Vector4
	l[0], r[n] = ct[0], ct[n]
	for k := 1; k <= n; k++ {
		for i := 0; i+k <= n; i++ {
			ct[i] = geom.Vector4Blend(ct[i], ct[i+1], t)
		}
		l[k], r[n-k] = ct[0], ct[n-k]
	}
	*bef = SBezierFromV4(l[:n+1]...)
	*aft = SBezierFromV4(r[:n+1]...)
}

// AllIntersectionsWith adds to spl the points where this curve crosses
// sbb: their pwls' crossings, refined onto both curves. It misses
// tangencies, and crossings where two pwl vertices meet (C++
// SBezier::AllIntersectionsWith).
func (b *SBezier) AllIntersectionsWith(sbb *SBezier, spl *SPointList) {
	var splRaw SPointList
	var sea, seb SEdgeList
	b.MakePwlInto(&sea, 0, 0)
	sbb.MakePwlInto(&seb, 0, 0)
	for k := range sea.Edges {
		se := &sea.Edges[k]
		// This isn't quite correct, since AnyEdgeCrossings doesn't count
		// the case where two pairs of line segments intersect at their
		// vertices. So this isn't robust, although that case isn't very
		// likely.
		seb.AnyEdgeCrossings(se.A, se.B, nil, &splRaw)
	}
	for _, sp := range splRaw.Points {
		p := sp.P
		if b.PointOnThisAndCurve(sbb, &p) {
			if !spl.ContainsPoint(p) {
				spl.Points = append(spl.Points, SPoint{P: p})
			}
		}
	}
}

// GetBoundingProjd extends (pmin, pmax) with the projection of every
// control point onto the axis (orig + t * u).
func (b *SBezier) GetBoundingProjd(u, orig geom.Vector, pmin, pmax *float64) {
	for i := 0; i <= b.Degree; i++ {
		v := b.Ctrl[i]
		proj := v.Minus(orig).Dot(u)
		if proj < *pmin {
			*pmin = proj
		}
		if proj > *pmax {
			*pmax = proj
		}
	}
}

// PointOnThisAndCurve finds a point that lies on both this bezier
// and the other bezier, by iteratively minimizing the closest
// distances. Returns the converged point via *p. Used during
// surface-surface intersection and curve trimming.
func (b *SBezier) PointOnThisAndCurve(other *SBezier, p *geom.Vector) bool {
	ta, _ := b.ClosestPointTo(*p, false)
	tb, _ := other.ClosestPointTo(*p, false)
	for i := 0; i < 20; i++ {
		pa := b.PointAt(ta)
		pb := other.PointAt(tb)
		da := b.TangentAt(ta)
		db := other.TangentAt(tb)
		if pa.EqualsTol(pb, ratPolyEps) {
			*p = pa
			return true
		}
		tta, ttb := geom.ClosestPointBetweenLines(pa, da, pb, db)
		ta += tta
		tb += ttb
	}
	return false
}

// PointOnNonparallelCurve is like PointOnThisAndCurve but rejects
// parallel-curve configurations that would converge to a degenerate
// solution. Used by curve-curve intersection where tangents at the
// candidate point should not be parallel.
func (b *SBezier) PointOnNonparallelCurve(other *SBezier, p *geom.Vector) bool {
	if b.Degree+other.Degree == 2 {
		d1 := b.Ctrl[1].Minus(b.Ctrl[0]).WithMagnitude(1)
		d2 := other.Ctrl[1].Minus(other.Ctrl[0]).WithMagnitude(1)
		if d1.Cross(d2).Magnitude() < lengthEps {
			return false
		}
	}
	if !b.PointOnThisAndCurve(other, p) {
		return false
	}
	ta, _ := b.ClosestPointTo(*p, false)
	tb, _ := other.ClosestPointTo(*p, false)
	da := b.TangentAt(ta).WithMagnitude(1)
	db := other.TangentAt(tb).WithMagnitude(1)
	if da.Cross(db).Magnitude() < lengthEps {
		return false
	}
	return true
}

// Equals returns true if two SBezier curves have identical degree,
// control points, and weights (within LENGTH_EPS tolerance).
func (b *SBezier) Equals(other *SBezier) bool {
	if b.Degree != other.Degree {
		return false
	}
	for i := 0; i <= b.Degree; i++ {
		if !b.Ctrl[i].Equals(other.Ctrl[i]) {
			return false
		}
		if math.Abs(b.Weight[i]-other.Weight[i]) > geom.LengthEps {
			return false
		}
	}
	return true
}

// InPerspective applies a perspective transformation, recomputing the
// rational weights. After this, points that previously projected to
// (x/w, y/w, z/w) now project to ((x/w)/(1-(z/w)*cameraTan), ...).
func (b *SBezier) InPerspective(u, v, n, origin geom.Vector, cameraTan float64) SBezier {
	q := geom.QuaternionFromBasis(u, v)
	qInv := q.Inverse()
	t := qInv.Rotate(origin).ScaledBy(-1)
	ret := *b
	ret.TransformedBy(t, qInv, 1.0)
	for i := 0; i <= ret.Degree; i++ {
		ct := geom.Vector4FromWeighted(ret.Weight[i], ret.Ctrl[i])
		// so the desired curve, before perspective, is
		//    (x/w, y/w, z/w)
		// and after perspective is
		//    ((x/w)/(1 - (z/w)*cameraTan, ...
		//  = (x/(w - z*cameraTan), ...
		// so we want to let w' = w - z*cameraTan
		ct.W = ct.W - ct.Z*cameraTan
		ret.Ctrl[i] = ct.PerspectiveProject()
		ret.Weight[i] = ct.W
	}
	return ret
}

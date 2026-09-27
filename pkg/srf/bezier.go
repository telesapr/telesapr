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
	Ctrl   [4]geom.Vector // control points (only Ctrl[0..Degree] used)
	Weight [4]float64     // rational weights (1.0 = non-rational)

	Tag  int
	AuxA int
	AuxB int
	// Entity is the handle of the sketch entity (a line segment) the
	// curve came from, or 0 (C++ SBezier::entity).
	Entity uint32

	// Hint of which kind of curve this is, used by SShell trimming.
	IsCircleHint bool
}

// SBezierFrom2 builds a line segment from a to b.
func SBezierFrom2(a, b geom.Vector) SBezier {
	return SBezier{
		Degree: 1,
		Ctrl:   [4]geom.Vector{a, b, {}, {}},
		Weight: [4]float64{1, 1, 1, 1},
	}
}

// SBezierFrom3 builds a quadratic Bezier from three control points.
func SBezierFrom3(a, b, c geom.Vector) SBezier {
	return SBezier{
		Degree: 2,
		Ctrl:   [4]geom.Vector{a, b, c, {}},
		Weight: [4]float64{1, 1, 1, 1},
	}
}

// SBezierFrom4 builds a cubic Bezier from four control points.
func SBezierFrom4(a, b, c, d geom.Vector) SBezier {
	return SBezier{
		Degree: 3,
		Ctrl:   [4]geom.Vector{a, b, c, d},
		Weight: [4]float64{1, 1, 1, 1},
	}
}

// Start returns the first control point.
func (b *SBezier) Start() geom.Vector {
	return b.Ctrl[0]
}

// Finish returns the last control point.
func (b *SBezier) Finish() geom.Vector {
	return b.Ctrl[b.Degree]
}

// IsRational returns true if any weight differs from 1.
func (b *SBezier) IsRational() bool {
	for i := 0; i <= b.Degree; i++ {
		if mathAbs(b.Weight[i]-1.0) > 1e-12 {
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

// IsInPlane returns true if every control point lies on the plane
// (n, d) within tol.
func (b *SBezier) IsInPlane(n geom.Vector, d, tol float64) bool {
	for i := 0; i <= b.Degree; i++ {
		if mathAbs(b.Ctrl[i].Dot(n)-d) > tol {
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

// ClosestPointTo finds the parameter t on the curve closest to point
// p, with the mustConverge flag controlling whether the result is
// guaranteed converged.
func (b *SBezier) ClosestPointTo(p geom.Vector, mustConverge bool) (float64, bool) {
	// Coarse sample: 7 samples for degree <=2, 20 for cubic.
	nSamples := 7
	if b.Degree == 3 {
		nSamples = 20
	}
	bestT := 0.0
	bestDist := mathInfPos
	for i := 0; i <= nSamples; i++ {
		t := float64(i) / float64(nSamples)
		d := b.PointAt(t).DistanceTo(p)
		if d < bestDist {
			bestDist = d
			bestT = t
		}
	}
	// Newton refinement.
	for iter := 0; iter < 25; iter++ {
		pt := b.PointAt(bestT)
		tan := b.TangentAt(bestT)
		tanMag2 := tan.MagSquared()
		if tanMag2 < 1e-20 {
			break
		}
		step := pt.Minus(p).Dot(tan) / tanMag2
		newT := bestT - step
		if newT < 0 {
			newT = 0
		}
		if newT > 1 {
			newT = 1
		}
		if mathAbs(newT-bestT) < 1e-12 {
			bestT = newT
			break
		}
		bestT = newT
	}
	if mustConverge && b.PointAt(bestT).DistanceTo(p) > 1e-6 {
		return bestT, false
	}
	return bestT, true
}

// SplitAt splits the Bezier at parameter t into two Beziers. The
// De Casteljau algorithm produces the new control points; weights
// are split using Vector4::Blend (De Casteljau in homogeneous
// coordinates).
func (b *SBezier) SplitAt(t float64, bef, aft *SBezier) {
	// Build Vector4 control points (homogeneous).
	cp := [4]geom.Vector4{}
	for i := 0; i <= b.Degree; i++ {
		cp[i] = geom.Vector4{
			X: b.Ctrl[i].X * b.Weight[i],
			Y: b.Ctrl[i].Y * b.Weight[i],
			Z: b.Ctrl[i].Z * b.Weight[i],
			W: b.Weight[i],
		}
	}
	// Standard De Casteljau split. After level j (j=1..degree),
	// `work[0..degree-j]` contains the level-j interpolation points.
	// Left control points: cp[0] at level 0, work[0] after level 1,
	// work[0] after level 2, ..., work[0] after level degree.
	// Right control points: work[0] after level degree (= P(t)),
	// then backwards from the other "diagonal": Q012, Q01, P0.
	work := [4]geom.Vector4{}
	copy(work[:], cp[:])
	left := [4]geom.Vector4{}
	right := [4]geom.Vector4{}
	left[0] = cp[0]
	for j := 1; j <= b.Degree; j++ {
		for i := 0; i <= b.Degree-j; i++ {
			work[i] = work[i].Blend(work[i+1], t)
		}
		left[j] = work[0]
	}
	right[b.Degree] = cp[b.Degree]
	right[0] = work[0]
	for j := 1; j < b.Degree; j++ {
		// Re-run De Casteljau to capture work[1] after each level.
		copy(work[:], cp[:])
		for k := 0; k < j; k++ {
			for i := 0; i <= b.Degree-k-1; i++ {
				work[i] = work[i].Blend(work[i+1], t)
			}
		}
		right[j] = work[1]
	}

	if bef != nil {
		*bef = SBezier{Degree: b.Degree, Tag: b.Tag, AuxA: b.AuxA, AuxB: b.AuxB}
		for i := 0; i <= b.Degree; i++ {
			p3 := left[i]
			if p3.W != 0 {
				bef.Ctrl[i] = geom.Vector{X: p3.X / p3.W, Y: p3.Y / p3.W, Z: p3.Z / p3.W}
			}
			bef.Weight[i] = p3.W
		}
	}
	if aft != nil {
		*aft = SBezier{Degree: b.Degree, Tag: b.Tag, AuxA: b.AuxA, AuxB: b.AuxB}
		for i := 0; i <= b.Degree; i++ {
			p3 := right[i]
			if p3.W != 0 {
				aft.Ctrl[i] = geom.Vector{X: p3.X / p3.W, Y: p3.Y / p3.W, Z: p3.Z / p3.W}
			}
			aft.Weight[i] = p3.W
		}
	}
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
		ct := geom.Vector4FromVector(ret.Ctrl[i], ret.Weight[i])
		ct.W = ct.W - ct.Z*cameraTan
		ret.Ctrl[i] = ct.Project3d()
		ret.Weight[i] = ct.W
	}
	return ret
}

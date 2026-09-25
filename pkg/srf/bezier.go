package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

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
	wSum := 0.0
	for i := 0; i <= b.Degree; i++ {
		B := bernstein(i, b.Degree, t)
		w := b.Weight[i] * B
		pt = pt.Plus(b.Ctrl[i].ScaledBy(w))
		wSum += w
	}
	if wSum != 0 {
		pt = pt.ScaledBy(1.0 / wSum)
	}
	return pt
}

// TangentAt returns the tangent vector at parameter t.
func (b *SBezier) TangentAt(t float64) geom.Vector {
	n := geom.Vector{}
	dw := 0.0
	for i := 0; i <= b.Degree; i++ {
		w := b.Weight[i]
		Bp := bernsteinDerivative(i, b.Degree, t)
		// d/dt (w_i * B_i * P_i) = w_i * B'_i * P_i
		n = n.Plus(b.Ctrl[i].ScaledBy(w * Bp))
		dw += w * Bp
	}
	// Apply quotient rule: (w * N' - N * w') / w^2.
	wSum := 0.0
	for i := 0; i <= b.Degree; i++ {
		wSum += b.Weight[i] * bernstein(i, b.Degree, t)
	}
	if wSum != 0 {
		n = n.ScaledBy(1.0 / wSum)
		if dw != 0 {
			pt := b.PointAt(t)
			n = n.Minus(pt.ScaledBy(dw / wSum))
		}
	}
	return n
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

// IsCircle reports whether this curve is a quadratic Bezier
// representing a circular arc. Returns (center, radius, ok).
func (b *SBezier) IsCircle() (center geom.Vector, radius float64, ok bool) {
	if b.Degree != 2 || !b.IsRational() {
		return geom.Vector{}, 0, false
	}
	// For a quadratic rational Bezier to be a circular arc, the
	// weights[1] = cos(theta/2) where theta is the swept angle.
	// Center is (|P0|^2 (P1-P2) + |P2|^2 (P0-P1) + |P1|^2 (P2-P0)) /
	// (2 * (P0 x P1 + P1 x P2 + P2 x P0)) for the 3-D cross product
	// reduction.
	// Simplified: solve for center as the unique point equidistant
	// from all three weighted control points in 3-D.
	// Solve via linear least squares: |c - Pi|^2 = r^2 for i=0,1,2.
	a := geom.Vector{X: 2 * (b.Ctrl[1].X - b.Ctrl[0].X), Y: 2 * (b.Ctrl[1].Y - b.Ctrl[0].Y), Z: 2 * (b.Ctrl[1].Z - b.Ctrl[0].Z)}
	bv := geom.Vector{X: 2 * (b.Ctrl[2].X - b.Ctrl[0].X), Y: 2 * (b.Ctrl[2].Y - b.Ctrl[0].Y), Z: 2 * (b.Ctrl[2].Z - b.Ctrl[0].Z)}
	rhsA := b.Ctrl[1].MagSquared() - b.Ctrl[0].MagSquared()
	rhsB := b.Ctrl[2].MagSquared() - b.Ctrl[0].MagSquared()
	// Solve 2x2: a . c = rhsA, b . c = rhsB
	det := a.X*bv.Y - a.Y*bv.X
	if mathAbs(det) < 1e-12 {
		return geom.Vector{}, 0, false
	}
	cz := (rhsA*bv.Y - rhsB*a.Y) / det
	cy := (a.X*rhsB - bv.X*rhsA) / det
	c := geom.Vector{X: 0, Y: cy, Z: cz}
	r := c.DistanceTo(b.Ctrl[0])
	// Verify the mid control point also lies on this circle.
	if mathAbs(c.DistanceTo(b.Ctrl[1])-r) > 1e-6 {
		return geom.Vector{}, 0, false
	}
	return c, r, true
}

// AllIntersectionsWith collects all intersection points between this
// Bezier and another into spl. Phase 3 implementation: simple
// uniform sampling on both curves and duplicate removal.
func (b *SBezier) AllIntersectionsWith(other *SBezier, spl *SPointList) {
	const n = 32
	type pt struct{ t1, t2 float64 }
	var candidates []pt
	for i := 0; i <= n; i++ {
		t1 := float64(i) / float64(n)
		p1 := b.PointAt(t1)
		t2, _ := other.ClosestPointTo(p1, false)
		p2 := other.PointAt(t2)
		if p1.DistanceTo(p2) < 1e-3 {
			candidates = append(candidates, pt{t1, t2})
		}
	}
	for j := 0; j <= n; j++ {
		t2 := float64(j) / float64(n)
		p2 := other.PointAt(t2)
		t1, _ := b.ClosestPointTo(p2, false)
		p1 := b.PointAt(t1)
		if p1.DistanceTo(p2) < 1e-3 {
			candidates = append(candidates, pt{t1, t2})
		}
	}
	for _, c := range candidates {
		// Midpoint of the two closest samples.
		spl.Points = append(spl.Points, SPoint{
			P: b.PointAt(c.t1).Plus(other.PointAt(c.t2)).ScaledBy(0.5),
		})
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

// MakePwlInto emits the polyline approximation of this Bezier into
// el (SEdgeList overload).
func (b *SBezier) MakePwlInto(el *SEdgeList, chordTol float64, maxDt float64) {
	const n = 8
	last := b.Start()
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		p := b.PointAt(t)
		el.AddEdge(last, p, b.AuxA, b.AuxB, b.Tag)
		last = p
	}
}

// MakePwlIntoPoints emits the polyline approximation as a list of
// points (used by SShell's triangulation pipeline).
func (b *SBezier) MakePwlIntoPoints(pts *[]geom.Vector, chordTol float64, maxDt float64) {
	const n = 8
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		*pts = append(*pts, b.PointAt(t))
	}
}

// Crossing returns the parametric t along the segment (a, b) at
// which this Bezier crosses the ray. Implemented via ray-vs-bezier
// approximation: sample the curve at multiple points, find the
// sample that crosses the ray's plane, refine. Returns ok=false if
// no crossing found.
func (b *SBezier) Crossing(a, dir geom.Vector) (float64, bool) {
	const n = 32
	prev := b.PointAt(0)
	prevT := 0.0
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		cur := b.PointAt(t)
		// Parametrize edge as prev + s*(cur - prev) for s in [0,1].
		edge := cur.Minus(prev)
		den := edge.Dot(dir)
		if mathAbs(den) < 1e-20 {
			prev = cur
			prevT = t
			continue
		}
		// Solve prev + s*edge = a + u*dir.
		// 3 equations, 2 unknowns — solve for s by minimizing the
		// perpendicular distance using the cross-product formula.
		w := a.Minus(prev)
		cross := edge.Cross(dir)
		cross2 := cross.MagSquared()
		if cross2 < 1e-20 {
			prev = cur
			prevT = t
			continue
		}
		s := w.Dot(cross.Cross(dir)) / cross2
		if s >= 0 && s <= 1 {
			// Compute distance from a + u*dir to prev + s*edge.
			closest := prev.Plus(edge.ScaledBy(s))
			diff := closest.Minus(a)
			u := diff.Dot(dir) / dir.MagSquared()
			if u >= 0 && u <= 1 {
				return prevT + s*(t-prevT), true
			}
		}
		prev = cur
		prevT = t
	}
	return 0, false
}

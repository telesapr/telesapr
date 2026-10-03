package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/libm"
)

// MaxDegree is the highest degree of a patch or a curve in either
// direction.
const MaxDegree = 5

// SSurface is a rational Bezier surface patch in (u, v). The
// surface is degree degm in u, degree degn in v, with up to 4x4
// weighted control points. The trim list specifies which (u, v)
// sub-region of the rectangle [0,1]x[0,1] is "inside" the surface.
type SSurface struct {
	Tag  int
	H    core.HSSurface
	NewH core.HSSurface

	Color core.RgbaColor
	Face  FaceID
	// Origin identifies a surface that no sketch entity names (Face is
	// 0), such as a thread's or a Boolean tool's: the group that made
	// it and its place among that group's surfaces (see
	// OriginFor). Copies keep it, so the faces it bounds can be named.
	Origin uint32
	// Sheet groups the patches swept from the pieces of one sketch
	// curve, such as the four quarters of an extruded circle: patches
	// of one Sheet that meet smoothly are one face of the solid, the
	// edge between them a seam (pkg/topo, Sheet). 0 for none.
	Sheet uint32

	DegM, DegN int
	Ctrl       [MaxDegree + 1][MaxDegree + 1]geom.Vector
	Weight     [MaxDegree + 1][MaxDegree + 1]float64

	Trim  []STrimBy
	Bsp   *SBspUv
	Edges SEdgeList
	// curvedEdges are the xyz pwl edges of the trims whose curve is not
	// straight, built with Edges. Within the chord tolerance of them the
	// trim BSP can put a point on the wrong side of the true curve, so
	// the raycast in ClassifyEdge does not trust a hit there.
	curvedEdges SEdgeList

	Cached geom.Point2d
	// noCache keeps ClosestPointTo off Cached while a loop run at once
	// projects points on this surface from several goroutines (see
	// ParallelFast).
	noCache bool

	// TrimBuilt is a Go-port-only marker (no C++ field mirrors it):
	// true when the surface's Trim list is authoritative, so an empty
	// one means "nothing to render", as in C++. Every SShell
	// constructor sets it; it is false only for a bare surface built
	// outside one (FromPlane etc.), which TriangulateInto renders
	// over its full parameter range instead.
	TrimBuilt bool
}

// STrimBy records which sub-range of an SCurve trims this surface.
// Used during boolean operations when a single trim curve becomes
// the boundary between two adjacent surfaces.
type STrimBy struct {
	Curve     core.HSCurve
	Start     float64 // parameter space: 0..1
	Finish    float64 // parameter space: 0..1
	Backwards bool
	// 3D endpoints, populated by STrimByEntireCurveFromShell.
	// Matches the C++ STrimBy::start / STrimBy::finish fields.
	StartP  geom.Vector
	FinishP geom.Vector
}

func STrimByEntireCurve(c core.HSCurve) STrimBy {
	return STrimBy{Curve: c, Start: 0, Finish: 1}
}

// trimIsClosedPolygon returns true if every consecutive pair of trim
// endpoints matches (StartP/FinishP) and the final entry's FinishP is
// the same as the first entry's StartP (within tol). Used by
// CopySurvesTrimAgainst's accurate path to decide whether the
// per-surface chain-finding succeeded. A non-closed (open) polygon
// is treated as a pipeline failure for that surface; the caller
// falls back to the bbox-keep heuristic.
func (s *SSurface) trimIsClosedPolygon() bool {
	const tol = 1e-7
	if len(s.Trim) < 3 {
		return false
	}
	firstStart := s.Trim[0].StartP
	for i := 0; i < len(s.Trim); i++ {
		cur := &s.Trim[i]
		next := &s.Trim[(i+1)%len(s.Trim)]
		if cur.FinishP.DistanceTo(next.StartP) > tol {
			return false
		}
	}
	if s.Trim[len(s.Trim)-1].FinishP.DistanceTo(firstStart) > tol {
		return false
	}
	return true
}

// Clear drops the trims (C++ SSurface::Clear).
func (s *SSurface) Clear() {
	s.Trim = nil
}

// Reverse flips the first parameter direction, which flips the normal,
// and reverses the trims to match. Mirrors C++ SSurface::Reverse.
func (s *SSurface) Reverse() {
	for i := 0; i < (s.DegM+1)/2; i++ {
		for j := 0; j <= s.DegN; j++ {
			ii := s.DegM - i
			s.Ctrl[i][j], s.Ctrl[ii][j] = s.Ctrl[ii][j], s.Ctrl[i][j]
			s.Weight[i][j], s.Weight[ii][j] = s.Weight[ii][j], s.Weight[i][j]
		}
	}

	for k := range s.Trim {
		stb := &s.Trim[k]
		stb.Backwards = !stb.Backwards
		stb.StartP, stb.FinishP = stb.FinishP, stb.StartP
	}
}

// ScaleSelfBy multiplies every control point by s.
func (s *SSurface) ScaleSelfBy(scale float64) {
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Ctrl[i][j] = s.Ctrl[i][j].ScaledBy(scale)
		}
	}
}

// PointAt evaluates the surface at parameter (u, v). Rational
// Bernstein product basis.
func (s *SSurface) PointAt(u, v float64) geom.Vector {
	num := geom.Vector{}
	den := 0.0

	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			Bi := bernstein(i, s.DegM, u)
			Bj := bernstein(j, s.DegN, v)

			num = num.Plus(s.Ctrl[i][j].ScaledBy(Bi * Bj * s.Weight[i][j]))
			den += s.Weight[i][j] * Bi * Bj
		}
	}
	num = num.ScaledBy(1.0 / den)
	return num
}

// PointAtUv is the Point2d variant.
func (s *SSurface) PointAtUv(puv geom.Point2d) geom.Vector {
	return s.PointAt(puv.U, puv.V)
}

// NormalAt returns the (unnormalized) surface normal at (u, v), the
// cross product of the tangents (C++ SSurface::NormalAt).
func (s *SSurface) NormalAt(u, v float64) geom.Vector {
	var tu, tv geom.Vector
	s.TangentsAt(u, v, &tu, &tv, true)
	return tu.Cross(tv)
}

// NormalAtUv is the Point2d variant.
func (s *SSurface) NormalAtUv(puv geom.Point2d) geom.Vector {
	return s.NormalAt(puv.U, puv.V)
}

// TangentsAt computes the partial derivatives dP/du and dP/dv at
// (u, v) (C++ SSurface::TangentsAt). A tangent is zero at
// singularities like the north pole; with retry, (u, v) is then moved
// a little toward the middle and the tangents recomputed.
func (s *SSurface) TangentsAt(u, v float64, tu, tv *geom.Vector, retry bool) {
	var num, numU, numV geom.Vector
	var den, denU, denV float64

	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			bi := bernstein(i, s.DegM, u)
			bj := bernstein(j, s.DegN, v)
			bip := bernsteinDerivative(i, s.DegM, u)
			bjp := bernsteinDerivative(j, s.DegN, v)

			num = num.Plus(s.Ctrl[i][j].ScaledBy(bi * bj * s.Weight[i][j]))
			den += s.Weight[i][j] * bi * bj

			numU = numU.Plus(s.Ctrl[i][j].ScaledBy(bip * bj * s.Weight[i][j]))
			denU += s.Weight[i][j] * bip * bj

			numV = numV.Plus(s.Ctrl[i][j].ScaledBy(bi * bjp * s.Weight[i][j]))
			denV += s.Weight[i][j] * bi * bjp
		}
	}
	// quotient rule; f(t) = n(t)/d(t), so f' = (n'*d - n*d')/(d^2)
	*tu = numU.ScaledBy(den).Minus(num.ScaledBy(denU))
	*tu = tu.ScaledBy(1.0 / (den * den))

	*tv = numV.ScaledBy(den).Minus(num.ScaledBy(denV))
	*tv = tv.ScaledBy(1.0 / (den * den))

	// Tangent is zero at sungularities like the north pole. Move away a bit and retry.
	if tv.Equals(geom.Vector{}) && retry {
		s.TangentsAt(u+(0.5-u)*0.00001, v, tu, tv, false)
	}
	if tu.Equals(geom.Vector{}) && retry {
		s.TangentsAt(u, v+(0.5-v)*0.00001, tu, tv, false)
	}
}

// GetAxisAlignedBounding extends (ptMin, ptMax) to include every
// control point of the surface.
func (s *SSurface) GetAxisAlignedBounding(ptMax, ptMin *geom.Vector) {
	*ptMax = geom.Vector{X: veryNegative, Y: veryNegative, Z: veryNegative}
	*ptMin = geom.Vector{X: veryPositive, Y: veryPositive, Z: veryPositive}

	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Ctrl[i][j].MakeMaxMin(ptMax, ptMin)
		}
	}
}

// FromExtrusionOf builds a surface by extruding a Bezier profile
// from t0 to t1. The degree is (deg, 1).
func FromExtrusionOf(spc *SBezier, t0, t1 geom.Vector) SSurface {
	s := SSurface{
		DegM:  spc.Degree,
		DegN:  1,
		Sheet: spc.Sheet,
	}
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Weight[i][j] = 1
		}
	}
	for i := 0; i <= s.DegM; i++ {
		w := spc.Weight[i]
		s.Ctrl[i][0] = spc.Ctrl[i].Plus(t0)
		s.Ctrl[i][1] = spc.Ctrl[i].Plus(t1)
		s.Weight[i][0] = w
		s.Weight[i][1] = w
	}
	return s
}

// FromPlane builds a planar surface of degree (1, 1) with normal
// u×v at point pt.
func FromPlane(pt, u, v geom.Vector) SSurface {
	s := SSurface{
		DegM: 1,
		DegN: 1,
	}
	s.Weight[0][0] = 1
	s.Weight[0][1] = 1
	s.Weight[1][0] = 1
	s.Weight[1][1] = 1
	// Matches C++ SSurface::FromPlane (surface.cpp:114): the m
	// (first) index runs along v and the n index along u.
	s.Ctrl[0][0] = pt
	s.Ctrl[0][1] = pt.Plus(u)
	s.Ctrl[1][0] = pt.Plus(v)
	s.Ctrl[1][1] = pt.Plus(v).Plus(u)
	return s
}

// FromRevolutionOf builds a surface by revolving a Bezier profile
// around an axis (pt, axis) over the angular range [thetas, thetaf],
// with optional endpoint shrinkage distances (dists, distf).
func FromRevolutionOf(sb *SBezier, pt, axis geom.Vector, thetas, thetaf, dists, distf float64) SSurface {
	s := SSurface{
		DegM:  sb.Degree,
		DegN:  2,
		Sheet: sb.Sheet,
	}
	dtheta := math.Abs(wrapSymmetric(thetaf-thetas, 2*math.Pi))
	w := libm.Cos(dtheta / 2)
	if w == 0 {
		w = 1e-12
	}
	for i := 0; i <= s.DegM; i++ {
		p := sb.Ctrl[i]
		ps := p.RotatedAbout(pt, axis, thetas)
		pf := p.RotatedAbout(pt, axis, thetaf)
		mid := ps.Plus(pf).ScaledBy(0.5)
		c0 := ps.ClosestPointOnLine(pt, axis)
		ct := mid.Minus(c0).ScaledBy(1 / (w * w)).Plus(c0)
		if ps.Equals(pf) {
			ps = c0
			ct = c0
			pf = c0
		}
		s.Ctrl[i][0] = ps.Plus(axis.ScaledBy(dists))
		s.Ctrl[i][1] = ct.Plus(axis.ScaledBy((dists + distf) / 2))
		s.Ctrl[i][2] = pf.Plus(axis.ScaledBy(distf))
		s.Weight[i][0] = sb.Weight[i]
		s.Weight[i][1] = sb.Weight[i] * w
		s.Weight[i][2] = sb.Weight[i]
	}
	return s
}

func wrapSymmetric(v, n float64) float64 {
	// Clamp it to the range (-n/2, n/2]. Mirrors C++ WRAP_SYMMETRIC.
	for v > n/2 {
		v -= n
	}
	for v <= -n/2 {
		v += n
	}
	return v
}

// OriginFor is the Origin of surface i of the surfaces group g made:
// bit 30 set, which no entity handle a sketch makes has, then the
// group's handle and i.
func OriginFor(g uint32, i int) uint32 {
	return 1<<30 | (g&0x3fff)<<16 | uint32(i)&0xffff
}

// FromTransformationOf produces a transformed copy of the surface.
func FromTransformationOf(a *SSurface, t geom.Vector, q geom.Quaternion, scale float64, includingTrims bool) SSurface {
	needRotate := !(q.X == 0 && q.Y == 0 && q.Z == 0 && q.W == 1)
	needTranslate := !(t.X == 0 && t.Y == 0 && t.Z == 0)
	needScale := scale != 1

	transform := func(v geom.Vector) geom.Vector {
		if needScale {
			v = v.ScaledBy(scale)
		}
		if needRotate {
			v = q.Rotate(v)
		}
		if needTranslate {
			v = v.Plus(t)
		}
		return v
	}

	s := SSurface{
		H:      a.H,
		Color:  a.Color,
		Face:   a.Face,
		Origin: a.Origin,
		Sheet:  a.Sheet,
		DegM:   a.DegM,
		DegN:   a.DegN,
	}
	for i := 0; i <= 3; i++ {
		for j := 0; j <= 3; j++ {
			s.Ctrl[i][j] = transform(a.Ctrl[i][j])
			s.Weight[i][j] = a.Weight[i][j]
		}
	}

	if includingTrims {
		for _, stb := range a.Trim {
			stb.StartP = transform(stb.StartP)
			stb.FinishP = transform(stb.FinishP)
			s.Trim = append(s.Trim, stb)
		}
		s.TrimBuilt = a.TrimBuilt
	}

	if scale < 0 {
		// If we mirror every surface of a shell, then it will end up inside
		// out. So fix that here.
		s.Reverse()
	}
	return s
}

// ClosestPointTo finds the (u, v) of the point on the surface closest to
// p (C++ SSurface::ClosestPointTo). With mustConverge the Newton
// refinement starts from the last result, is given more iterations,
// and stops once it reaches p.
func (s *SSurface) ClosestPointTo(p geom.Vector, puv *geom.Point2d, mustConverge bool) {
	s.ClosestPointToUV(p, &puv.U, &puv.V, mustConverge)
}

// ClosestPointToUV is ClosestPointTo returning u and v separately.
func (s *SSurface) ClosestPointToUV(p geom.Vector, u, v *float64, mustConverge bool) {
	// A few special cases first; when control points are coincident the
	// derivative goes to zero at the control points, and would result in
	// nonconvergence. We avoid that here, and also guarantee a consistent
	// (u, v) (of the infinitely many possible in one parameter).
	if p.Equals(s.Ctrl[0][0]) {
		*u, *v = 0, 0
		return
	}
	if p.Equals(s.Ctrl[s.DegM][0]) {
		*u, *v = 1, 0
		return
	}
	if p.Equals(s.Ctrl[s.DegM][s.DegN]) {
		*u, *v = 1, 1
		return
	}
	if p.Equals(s.Ctrl[0][s.DegN]) {
		*u, *v = 0, 1
		return
	}

	// And planes are trivial, so don't waste time iterating over those.
	if orig, bu, bv, ok := s.planeBasis(); ok {
		n := bu.Cross(bv)
		dp := p.Minus(orig)
		if math.Abs(bu.Dot(bv)) > 1e-12*bu.Magnitude()*bv.Magnitude() {
			// Axes not square to each other, as on the side of a leaning
			// prism: solve dp = u bu + v bv, through the vectors in the
			// plane square to each axis. (C++ assumes square axes.)
			tx, ty := bv.Cross(n), n.Cross(bu)
			*u = dp.Dot(tx) / bu.Dot(tx)
			*v = dp.Dot(ty) / bv.Dot(ty)
			return
		}
		ty := n.Cross(bu).ScaledBy(1.0 / bu.MagSquared())
		tx := bv.Cross(n).ScaledBy(1.0 / bv.MagSquared())

		*u = dp.Dot(bu) / tx.MagSquared()
		*v = dp.Dot(bv) / ty.MagSquared()
		return
	}

	// Try whatever the previous guess was. This is likely to do something
	// good if we're working our way along a curve or something else where
	// we project successive points that are close to each other; something
	// like a 20% speedup empirically.
	if mustConverge && !s.noCache {
		ut, vt := s.Cached.U, s.Cached.V
		if s.closestPointNewton(p, &ut, &vt, mustConverge) {
			*u, *v = ut, vt
			s.Cached = geom.Point2d{U: ut, V: vt}
			return
		}
	}

	// Search for a reasonable initial guess
	minDist := veryPositive
	res := 20
	if max(s.DegM, s.DegN) == 2 {
		res = 7
	}
	for i := 0; i < res; i++ {
		for j := 0; j < res; j++ {
			tryu, tryv := (float64(i)+0.5)/float64(res), (float64(j)+0.5)/float64(res)

			tryp := s.PointAt(tryu, tryv)
			d := tryp.Minus(p).Magnitude()
			if d < minDist {
				*u = tryu
				*v = tryv
				minDist = d
			}
		}
	}

	if s.closestPointNewton(p, u, v, mustConverge) {
		if !s.noCache {
			s.Cached = geom.Point2d{U: *u, V: *v}
		}
		return
	}

	// If we failed to converge, then at least don't return NaN.
	if isReasonable(*u) || isReasonable(*v) {
		*u, *v = 0, 0
	}
}

// closestPointNewton refines the guess (u, v) by Newton iteration,
// clamped to the unit square (C++ SSurface::ClosestPointNewton).
func (s *SSurface) closestPointNewton(p geom.Vector, u, v *float64, mustConverge bool) bool {
	iters := 5
	if mustConverge {
		iters = 25
	}
	for i := 0; i < iters; i++ {
		p0 := s.PointAt(*u, *v)
		if mustConverge {
			if p0.EqualsTol(p, ratPolyEps) {
				return true
			}
		}

		var tu, tv geom.Vector
		s.TangentsAt(*u, *v, &tu, &tv, true)
		n := tu.Cross(tv)
		// since tu and tv may not be orthogonal, use y in place of v.
		// |y| = |v|sin(theta) where theta is the angle between tu and tv.
		ty := n.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
		tx := tv.Cross(n).ScaledBy(1.0 / tv.MagSquared())

		// Project the point into a plane through p0, with basis tu, tv; a
		// second-order thing would converge faster but needs second
		// derivatives.
		dp := p.Minus(p0)
		du := dp.Dot(tx)
		dv := dp.Dot(ty)
		*u += du / tx.MagSquared()
		*v += dv / ty.MagSquared()

		if *u < 0.0 {
			*u = 0.0
		} else if *u > 1.0 {
			*u = 1.0
		}
		if *v < 0.0 {
			*v = 0.0
		} else if *v > 1.0 {
			*v = 1.0
		}
	}

	return false
}

// PointOnCurve refines (*up, *vp), an initial guess, to where curve
// pierces the surface, by alternating Newton iterations: each finds
// where the curve's tangent line pierces the surface's tangent plane,
// then updates both the surface (u, v) and the curve parameter. If
// it doesn't converge, *up and *vp are left alone. Faithful port of
// C++ `SSurface::PointOnCurve` (ratpoly.cpp).
func (s *SSurface) PointOnCurve(curve *SBezier, up, vp *float64) {
	u, v := *up, *vp
	ps := s.PointAt(u, v)
	// Get initial guesses for t on the curve
	tCurve, _ := curve.ClosestPointTo(ps, false)
	if tCurve < 0.0 {
		tCurve = 0.0
	}
	if tCurve > 1.0 {
		tCurve = 1.0
	}

	for i := 0; i < 30; i++ {
		// Approximate the surface by a plane
		ps := s.PointAt(u, v)
		var tu, tv geom.Vector
		s.TangentsAt(u, v, &tu, &tv, true)
		n := tu.Cross(tv).WithMagnitude(1)

		// point on curve and tangent line direction
		pc := curve.PointAt(tCurve)
		tc := curve.TangentAt(tCurve)

		if ps.EqualsTol(pc, ratPolyEps) {
			*up = u
			*vp = v
			return
		}

		// pi is where the curve tangent line intersects the surface tangent plane
		var pi geom.Vector
		d := tc.Dot(n)
		if math.Abs(d) < 1e-10 { // parallel line and plane, guess the average rather than fail
			pi = pc.Plus(ps).ScaledBy(0.5)
		} else {
			pi = pc.Minus(tc.ScaledBy(pc.Minus(ps).Dot(n) / d))
		}

		// project the point onto the tangent plane and line
		{
			n := tu.Cross(tv)
			ty := n.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
			tx := tv.Cross(n).ScaledBy(1.0 / tv.MagSquared())

			dp := pi.Minus(ps)
			du, dv := dp.Dot(tx), dp.Dot(ty)

			u += du / tx.MagSquared()
			v += dv / ty.MagSquared()
		}
		tCurve += pi.Minus(pc).Dot(tc) / tc.MagSquared()
		if tCurve < 0.0 {
			tCurve = 0.0
		}
		if tCurve > 1.0 {
			tCurve = 1.0
		}
	}
	// didn't converge (surface and curve intersecting)
}

// ContainsPlaneCurve returns true if the supplied exact curve lies
// entirely within the plane of this surface. Mirrors
// SSurface::ContainsPlaneCurve in srf/surfinter.cpp. Used by
// IntersectAgainst to detect trim curves that lie in a plane and
// can be added as exact intersection curves.
func (s *SSurface) ContainsPlaneCurve(sc *SCurve) bool {
	if s.DegM != 1 || s.DegN != 1 {
		return false
	}
	if !sc.IsExact {
		return false // we don't handle those (yet?)
	}

	p := s.Ctrl[0][0]
	n := s.NormalAt(0, 0).WithMagnitude(1)
	d := n.Dot(p)

	// check all control points on the curve
	for i := 0; i <= sc.Exact.Degree; i++ {
		if math.Abs(n.Dot(sc.Exact.Ctrl[i])-d) > lengthEps {
			return false
		}
	}
	return true
}

// CoincidentWithPlane reports whether the surface is a plane lying in
// the plane n . p = d (C++ SSurface::CoincidentWithPlane).
func (s *SSurface) CoincidentWithPlane(n geom.Vector, d float64) bool {
	if s.DegM != 1 || s.DegN != 1 {
		return false
	}
	if math.Abs(n.Dot(s.Ctrl[0][0])-d) > lengthEps {
		return false
	}
	if math.Abs(n.Dot(s.Ctrl[0][1])-d) > lengthEps {
		return false
	}
	if math.Abs(n.Dot(s.Ctrl[1][0])-d) > lengthEps {
		return false
	}
	if math.Abs(n.Dot(s.Ctrl[1][1])-d) > lengthEps {
		return false
	}

	return true
}

// LineEntirelyOutsideBbox returns true if the line (a, b) does not
// cross the surface's bounding box.
func (s *SSurface) LineEntirelyOutsideBbox(a, b geom.Vector, asSegment bool) bool {
	// Mirrors C++ SSurface::LineEntirelyOutsideBbox.
	var amax, amin geom.Vector
	s.GetAxisAlignedBounding(&amax, &amin)
	if !geom.BoundingBoxIntersectsLine(amax, amin, a, b, asSegment) {
		// The line segment could fail to intersect the bbox, but lie entirely
		// within it and intersect the surface.
		if a.OutsideAndNotOn(amax, amin) && b.OutsideAndNotOn(amax, amin) {
			return true
		}
	}
	return false
}

// WeightControlPoints multiplies each control point by its weight (C++
// SSurface::WeightControlPoints).
func (s *SSurface) WeightControlPoints() {
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Ctrl[i][j] = s.Ctrl[i][j].ScaledBy(s.Weight[i][j])
		}
	}
}

// UnWeightControlPoints divides each control point by its weight (C++
// SSurface::UnWeightControlPoints).
func (s *SSurface) UnWeightControlPoints() {
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Ctrl[i][j] = s.Ctrl[i][j].ScaledBy(1.0 / s.Weight[i][j])
		}
	}
}

// CopyRowOrCol copies row (or column) srcIJ of src into row thisIJ
// (C++ SSurface::CopyRowOrCol).
func (s *SSurface) CopyRowOrCol(row bool, thisIJ int, src *SSurface, srcIJ int) {
	if row {
		for j := 0; j <= s.DegN; j++ {
			s.Ctrl[thisIJ][j] = src.Ctrl[srcIJ][j]
			s.Weight[thisIJ][j] = src.Weight[srcIJ][j]
		}
	} else {
		for i := 0; i <= s.DegM; i++ {
			s.Ctrl[i][thisIJ] = src.Ctrl[i][srcIJ]
			s.Weight[i][thisIJ] = src.Weight[i][srcIJ]
		}
	}
}

// BlendRowOrCol sets row (or column) thisIJ to the average of row aIJ
// of a and row bIJ of b (C++ SSurface::BlendRowOrCol).
func (s *SSurface) BlendRowOrCol(row bool, thisIJ int, a *SSurface, aIJ int, b *SSurface, bIJ int) {
	if row {
		for j := 0; j <= s.DegN; j++ {
			c := a.Ctrl[aIJ][j].Plus(b.Ctrl[bIJ][j])
			w := a.Weight[aIJ][j] + b.Weight[bIJ][j]
			s.Ctrl[thisIJ][j] = c.ScaledBy(0.5)
			s.Weight[thisIJ][j] = w / 2
		}
	} else {
		for i := 0; i <= s.DegM; i++ {
			c := a.Ctrl[i][aIJ].Plus(b.Ctrl[i][bIJ])
			w := a.Weight[i][aIJ] + b.Weight[i][bIJ]
			s.Ctrl[i][thisIJ] = c.ScaledBy(0.5)
			s.Weight[i][thisIJ] = w / 2
		}
	}
}

// SplitInHalf splits the surface at u = 0.5 (byU) or v = 0.5 into sa
// and sb, by de Casteljau's algorithm in projective space (C++
// SSurface::SplitInHalf, for any degree). Only the control net of sa
// and sb is written.
func (s *SSurface) SplitInHalf(byU bool, sa, sb *SSurface) {
	sa.DegM, sb.DegM = s.DegM, s.DegM
	sa.DegN, sb.DegN = s.DegN, s.DegN

	deg, lines := s.DegN, s.DegM
	if byU {
		deg, lines = s.DegM, s.DegN
	}
	// Each line across the split is a curve of degree deg, worked on as
	// points (w*x, w*y, w*z) and weights w.
	var p [MaxDegree + 1]geom.Vector
	var w [MaxDegree + 1]float64
	for l := 0; l <= lines; l++ {
		for k := 0; k <= deg; k++ {
			i, j := l, k
			if byU {
				i, j = k, l
			}
			p[k] = s.Ctrl[i][j].ScaledBy(s.Weight[i][j])
			w[k] = s.Weight[i][j]
		}
		set := func(dst *SSurface, k int, q geom.Vector, wq float64) {
			i, j := l, k
			if byU {
				i, j = k, l
			}
			dst.Ctrl[i][j] = q.ScaledBy(1.0 / wq)
			dst.Weight[i][j] = wq
		}
		set(sa, 0, p[0], w[0])
		set(sb, deg, p[deg], w[deg])
		for r := 1; r <= deg; r++ {
			for k := 0; k+r <= deg; k++ {
				p[k] = p[k].Plus(p[k+1]).ScaledBy(0.5)
				w[k] = (w[k] + w[k+1]) / 2
			}
			set(sa, r, p[0], w[0])
			set(sb, deg-r, p[deg-r], w[deg-r])
		}
	}
}

// DepartureFromCoplanar is how far the control points stray from the
// plane through three of them, chosen far apart (C++
// SSurface::DepartureFromCoplanar).
func (s *SSurface) DepartureFromCoplanar() float64 {
	ib, jb, ic, jc := 0, 0, 0, 0

	// Grab three points to define a plane; first choose (0, 0) arbitrarily.
	ia, ja := 0, 0
	// Then the point farthest from pt a.
	best := veryNegative
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			if i == ia && j == ja {
				continue
			}

			dist := s.Ctrl[i][j].Minus(s.Ctrl[ia][ja]).Magnitude()
			if dist > best {
				best = dist
				ib, jb = i, j
			}
		}
	}
	// Then biggest magnitude of ab cross ac.
	best = veryNegative
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			if i == ia && j == ja {
				continue
			}
			if i == ib && j == jb {
				continue
			}

			mag := s.Ctrl[ia][ja].Minus(s.Ctrl[ib][jb]).Cross(
				s.Ctrl[ia][ja].Minus(s.Ctrl[i][j])).Magnitude()
			if mag > best {
				best = mag
				ic, jc = i, j
			}
		}
	}

	n := s.Ctrl[ia][ja].Minus(s.Ctrl[ib][jb]).Cross(
		s.Ctrl[ia][ja].Minus(s.Ctrl[ic][jc]))
	n = n.WithMagnitude(1)
	d := s.Ctrl[ia][ja].Dot(n)

	// Finally, calculate the deviation from each point to the plane.
	farthest := veryNegative
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			dist := math.Abs(n.Dot(s.Ctrl[i][j]) - d)
			if dist > farthest {
				farthest = dist
			}
		}
	}
	return farthest
}

// PointIntersectingLine refines (u, v) by Newton's method to where the
// line through p0 and p1 meets the surface, trying the surface's edge
// if that fails (C++ SSurface::PointIntersectingLine).
func (s *SSurface) PointIntersectingLine(p0, p1 geom.Vector, u, v *float64) bool {
	for i := 0; i < 20; i++ {
		var tu, tv geom.Vector
		p := s.PointAt(*u, *v)
		s.TangentsAt(*u, *v, &tu, &tv, true)

		n := tu.Cross(tv).WithMagnitude(1)
		d := p.Dot(n)

		pi, parallel := geom.AtIntersectionOfPlaneAndLineParallel(n, d, p0, p1)
		if parallel {
			break
		}

		// Check for convergence - success exits
		if pi.EqualsTol(p, ratPolyEps) {
			return true
		}

		n = tu.Cross(tv)
		ty := n.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
		tx := tv.Cross(n).ScaledBy(1.0 / tv.MagSquared())

		// Adjust our guess and iterate
		dp := pi.Minus(p)
		du, dv := dp.Dot(tx), dp.Dot(ty)
		*u += du / tx.MagSquared()
		*v += dv / ty.MagSquared()
	}
	// Lets try the edge of the surface against the line before giving up
	curve := SBezier{Degree: 1}
	curve.Ctrl[0], curve.Weight[0] = p0, 1.0
	curve.Ctrl[1], curve.Weight[1] = p1, 1.0
	return s.EdgeCurveIntersection(u, v, &curve)
}

// EdgeCurveIntersection is a fallback for PointIntersectingLine: tries
// to intersect the line with the boundary curve of the surface.
func (s *SSurface) EdgeCurveIntersection(u, v *float64, curve *SBezier) bool {
	p := s.PointAt(*u, *v)
	var edge SBezier
	degm := s.DegM
	degn := s.DegN
	if math.Abs(*u-0.5) > math.Abs(*v-0.5) {
		if *u < 0.5 {
			for n := 0; n <= degn; n++ {
				edge.Ctrl[n] = s.Ctrl[0][n]
				edge.Weight[n] = s.Weight[0][n]
			}
			edge.Degree = degn
			if edge.PointOnNonparallelCurve(curve, &p) {
				if tv, _ := edge.ClosestPointTo(p, true); true {
					*v = tv
					*u = 0.0
					return true
				}
			}
		} else {
			for n := 0; n <= degn; n++ {
				edge.Ctrl[n] = s.Ctrl[degm][n]
				edge.Weight[n] = s.Weight[degm][n]
			}
			edge.Degree = degn
			if edge.PointOnNonparallelCurve(curve, &p) {
				if tv, _ := edge.ClosestPointTo(p, true); true {
					*v = tv
					*u = 1.0
					return true
				}
			}
		}
	} else {
		if *v < 0.5 {
			for n := 0; n <= degm; n++ {
				edge.Ctrl[n] = s.Ctrl[n][0]
				edge.Weight[n] = s.Weight[n][0]
			}
			edge.Degree = degm
			if edge.PointOnNonparallelCurve(curve, &p) {
				if tv, _ := edge.ClosestPointTo(p, true); true {
					*u = tv
					*v = 0.0
					return true
				}
			}
		} else {
			for n := 0; n <= degm; n++ {
				edge.Ctrl[n] = s.Ctrl[n][degn]
				edge.Weight[n] = s.Weight[n][degn]
			}
			edge.Degree = degm
			if edge.PointOnNonparallelCurve(curve, &p) {
				if tv, _ := edge.ClosestPointTo(p, true); true {
					*u = tv
					*v = 1.0
					return true
				}
			}
		}
	}
	return false
}

func STrimByEntireCurveFromShell(shell *SShell, hsc core.HSCurve, backwards bool) STrimBy {
	stb := STrimBy{Curve: hsc, Backwards: backwards, Start: 0, Finish: 1}
	if shell == nil {
		return stb
	}
	sc := shell.FindCurve(hsc)
	if sc == nil || len(sc.Points) == 0 {
		return stb
	}
	if backwards {
		// Start = last point, Finish = first point.
		stb.StartP = sc.Points[len(sc.Points)-1].P
		stb.FinishP = sc.Points[0].P
	} else {
		stb.StartP = sc.Points[0].P
		stb.FinishP = sc.Points[len(sc.Points)-1].P
	}
	return stb
}

// uvStepToward returns the (du, dv) that moves srf's point at (u, v)
// by dp, to first order. Shared by the Newton iterations of C++
// PointOnSurfaces and ClosestPointOnThisAndSurface.
func uvStepToward(tu, tv, dp geom.Vector) (du, dv float64) {
	n := tu.Cross(tv)
	ty := n.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
	tx := tv.Cross(n).ScaledBy(1.0 / tv.MagSquared())
	return dp.Dot(tx) / tx.MagSquared(), dp.Dot(ty) / ty.MagSquared()
}

// PointOnSurfaces refines (*up, *vp) on s so that the point also lies
// on s1 and s2. Mirrors C++ SSurface::PointOnSurfaces (ratpoly.cpp).
func (s *SSurface) PointOnSurfaces(s1, s2 *SSurface, up, vp *float64) {
	u := [3]float64{*up, 0, 0}
	v := [3]float64{*vp, 0, 0}
	srf := [3]*SSurface{s, s1, s2}

	// Get initial guesses for (u, v) in the other surfaces
	p := s.PointAt(u[0], v[0])
	srf[1].ClosestPointToUV(p, &u[1], &v[1], false)
	srf[2].ClosestPointToUV(p, &u[2], &v[2], false)

	for i := 0; i < 20; i++ {
		// Approximate each surface by a plane
		var pj, tu, tv, n [3]geom.Vector
		var d [3]float64
		for j := 0; j < 3; j++ {
			pj[j] = srf[j].PointAt(u[j], v[j])
			srf[j].TangentsAt(u[j], v[j], &tu[j], &tv[j], true)
			n[j] = tu[j].Cross(tv[j]).WithMagnitude(1)
			d[j] = n[j].Dot(pj[j])
		}

		// If a = b and b = c, then does a = c? No, it doesn't.
		if pj[0].EqualsTol(pj[1], ratPolyEps) &&
			pj[1].EqualsTol(pj[2], ratPolyEps) &&
			pj[2].EqualsTol(pj[0], ratPolyEps) {
			*up = u[0]
			*vp = v[0]
			return
		}

		pi, parallel := geom.AtIntersectionOfThreePlanes(n[0], d[0], n[1], d[1], n[2], d[2])
		if parallel { // lets try something else for parallel planes
			pi = pj[0].Plus(pj[1]).Plus(pj[2]).ScaledBy(1.0 / 3.0)
		}

		for j := 0; j < 3; j++ {
			du, dv := uvStepToward(tu[j], tv[j], pi.Minus(pj[j]))
			u[j] += du
			v[j] += dv
		}
	}
	// didn't converge (three surfaces intersecting); C++ leaves *up,
	// *vp unchanged.
}

// ClosestPointOnThisAndSurface returns the point on the intersection
// of s and srf2 closest to p. Mirrors C++
// SSurface::ClosestPointOnThisAndSurface (ratpoly.cpp:627).
func (s *SSurface) ClosestPointOnThisAndSurface(srf2 *SSurface, p geom.Vector) geom.Vector {
	var puv [2]geom.Point2d
	srf := [2]*SSurface{s, srf2}

	for j := 0; j < 2; j++ {
		srf[j].ClosestPointTo(p, &puv[j], false)
	}

	for i := 0; i < 10; i++ {
		var tu, tv, cp, n [2]geom.Vector
		var d [2]float64

		for j := 0; j < 2; j++ {
			srf[j].TangentsAt(puv[j].U, puv[j].V, &tu[j], &tv[j], true)

			cp[j] = srf[j].PointAtUv(puv[j])

			n[j] = tu[j].Cross(tv[j]).WithMagnitude(1)
			d[j] = n[j].Dot(cp[j])
		}

		if cp[0].EqualsTol(cp[1], ratPolyEps) {
			break
		}

		p0 := geom.AtIntersectionOfTwoPlanes(n[0], d[0], n[1], d[1])
		dp := n[0].Cross(n[1])

		pc := p.ClosestPointOnLine(p0, dp)

		// Adjust our guess and iterate
		for j := 0; j < 2; j++ {
			du, dv := uvStepToward(tu[j], tv[j], pc.Minus(cp[j]))
			puv[j].U += du
			puv[j].V += dv
		}
	}

	// If this converged, then the two points are actually equal.
	return srf[0].PointAtUv(puv[0]).Plus(srf[1].PointAtUv(puv[1])).ScaledBy(0.5)
}

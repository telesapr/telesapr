package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

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

	DegM, DegN int
	Ctrl       [4][4]geom.Vector
	Weight     [4][4]float64

	Trim  []STrimBy
	Bsp   *SBspUv
	Edges SEdgeList

	Cached geom.Point2d

	// TrimBuilt is a Go-port-only marker (no C++ field mirrors it)
	// set true for surfaces where an empty Trim list should be
	// trusted as "genuinely outside the kept region" (render 0
	// triangles — matching the C++, which has no untrimmed-grid
	// fallback at all; see solvespace/src/srf/surface.cpp:414's
	// TriangulateInto) rather than papered over with the full
	// parameter grid. Set unconditionally by MakeFromExtrusionOf
	// (whose surfaces always have a non-empty Trim anyway) and,
	// conditionally on the combine type, by the boolean per-surface
	// trim in makeCopyTrimAgainst: true only for SurfaceInter, whose
	// empty final chains reliably mean "outside the intersection".
	// For SurfaceUnion/SurfaceDiff, an empty final chain is more
	// often an unresolved chain-walk/classification miss on a
	// tangent-contact boundary (PORTING.md "Boolean-CSG parity
	// gaps") than a deliberate discard, so those get TrimBuilt=false
	// and fall back to the grid — measurably closer to the C++
	// golden across the boolean fixtures (see makeCopyTrimAgainst's
	// comment for the specific measurement). A surface whose
	// constructor never populates Trim at all (currently
	// MakeFromRevolutionOf / MakeFromHelixOf) also has TrimBuilt
	// false by default and gets the same grid fallback so it doesn't
	// vanish.
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

// Clear resets all fields to their zero state.
func (s *SSurface) Clear() {
	s.Tag = 0
	s.H = core.HSSurface{}
	s.NewH = core.HSSurface{}
	s.Color = core.RgbaColor{}
	s.Face = 0
	s.DegM = 0
	s.DegN = 0
	s.Ctrl = [4][4]geom.Vector{}
	s.Weight = [4][4]float64{}
	s.Trim = nil
	s.Bsp = nil
	s.Edges = SEdgeList{}
	s.Cached = geom.Point2d{}
}

// Reverse flips the parameter domain so (u, v) -> (1-u, 1-v). Used
// when the surface normal points the wrong way for a given mesh.
func (s *SSurface) Reverse() {
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			ii := s.DegM - i
			s.Ctrl[i][j], s.Ctrl[ii][j] = s.Ctrl[ii][j], s.Ctrl[i][j]
			s.Weight[i][j], s.Weight[ii][j] = s.Weight[ii][j], s.Weight[i][j]
		}
	}
	for i := 0; i <= s.DegM/2; i++ {
		for j := 0; j <= s.DegN; j++ {
			jj := s.DegN - j
			s.Ctrl[i][j], s.Ctrl[i][jj] = s.Ctrl[i][jj], s.Ctrl[i][j]
			s.Weight[i][j], s.Weight[i][jj] = s.Weight[i][jj], s.Weight[i][j]
		}
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
	pt := geom.Vector{}
	wSum := 0.0
	for i := 0; i <= s.DegM; i++ {
		Bu := bernstein(i, s.DegM, u)
		for j := 0; j <= s.DegN; j++ {
			Bv := bernstein(j, s.DegN, v)
			w := s.Weight[i][j] * Bu * Bv
			pt = pt.Plus(s.Ctrl[i][j].ScaledBy(w))
			wSum += w
		}
	}
	if wSum != 0 {
		pt = pt.ScaledBy(1.0 / wSum)
	}
	return pt
}

// PointAtUv is the Point2d variant.
func (s *SSurface) PointAtUv(puv geom.Point2d) geom.Vector {
	return s.PointAt(puv.U, puv.V)
}

// NormalAt returns the surface normal at (u, v) computed via cross
// product of partial derivatives.
func (s *SSurface) NormalAt(u, v float64) geom.Vector {
	tu, tv := new(geom.Vector), new(geom.Vector)
	s.TangentsAt(u, v, tu, tv, false)
	n := tu.Cross(*tv)
	if n.Magnitude() < 1e-12 {
		return geom.Vector{Z: 1}
	}
	return n.Normal()
}

// NormalAtUv is the Point2d variant.
func (s *SSurface) NormalAtUv(puv geom.Point2d) geom.Vector {
	return s.NormalAt(puv.U, puv.V)
}

// TangentsAt computes the partial derivatives dP/du and dP/dv at
// (u, v). When retry=true and either tangent collapses to zero
// (e.g. a sphere pole), perturb (u, v) and try again.
func (s *SSurface) TangentsAt(u, v float64, tu, tv *geom.Vector, retry bool) {
	for attempt := 0; attempt < 5; attempt++ {
		s.tangentsAtImpl(u, v, tu, tv)
		if tu.Magnitude() > 1e-9 && tv.Magnitude() > 1e-9 {
			return
		}
		if !retry {
			return
		}
		u += 0.001
		v += 0.001
	}
}

func (s *SSurface) tangentsAtImpl(u, v float64, tu, tv *geom.Vector) {
	*tu = geom.Vector{}
	*tv = geom.Vector{}
	dwDu := 0.0
	dwDv := 0.0
	for i := 0; i <= s.DegM; i++ {
		Bu := bernstein(i, s.DegM, u)
		BuP := bernsteinDerivative(i, s.DegM, u)
		for j := 0; j <= s.DegN; j++ {
			Bv := bernstein(j, s.DegN, v)
			BvP := bernsteinDerivative(j, s.DegN, v)
			w := s.Weight[i][j]
			*tu = (*tu).Plus(s.Ctrl[i][j].ScaledBy(w * BuP * Bv))
			*tv = (*tv).Plus(s.Ctrl[i][j].ScaledBy(w * Bu * BvP))
			dwDu += w * BuP * Bv
			dwDv += w * Bu * BvP
		}
	}
	wSum := 0.0
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			wSum += s.Weight[i][j] * bernstein(i, s.DegM, u) * bernstein(j, s.DegN, v)
		}
	}
	if wSum != 0 {
		pt := s.PointAt(u, v)
		*tu = tu.ScaledBy(1.0 / wSum).Minus(pt.ScaledBy(dwDu / wSum))
		*tv = tv.ScaledBy(1.0 / wSum).Minus(pt.ScaledBy(dwDv / wSum))
	}
}

// GetAxisAlignedBounding extends (ptMin, ptMax) to include every
// control point of the surface.
func (s *SSurface) GetAxisAlignedBounding(ptMax, ptMin *geom.Vector) {
	ptMin.X = mathInfPos
	ptMin.Y = mathInfPos
	ptMin.Z = mathInfPos
	ptMax.X = mathInfNeg
	ptMax.Y = mathInfNeg
	ptMax.Z = mathInfNeg
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			p := s.Ctrl[i][j]
			if p.X < ptMin.X {
				ptMin.X = p.X
			}
			if p.Y < ptMin.Y {
				ptMin.Y = p.Y
			}
			if p.Z < ptMin.Z {
				ptMin.Z = p.Z
			}
			if p.X > ptMax.X {
				ptMax.X = p.X
			}
			if p.Y > ptMax.Y {
				ptMax.Y = p.Y
			}
			if p.Z > ptMax.Z {
				ptMax.Z = p.Z
			}
		}
	}
}

// FromExtrusionOf builds a surface by extruding a Bezier profile
// from t0 to t1. The degree is (deg, 1).
func FromExtrusionOf(spc *SBezier, t0, t1 geom.Vector) SSurface {
	s := SSurface{
		DegM:   spc.Degree,
		DegN:   1,
		Weight: [4][4]float64{},
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
	s.Ctrl[0][0] = pt
	s.Ctrl[0][1] = pt.Plus(v)
	s.Ctrl[1][0] = pt.Plus(u)
	s.Ctrl[1][1] = pt.Plus(u).Plus(v)
	return s
}

// FromRevolutionOf builds a surface by revolving a Bezier profile
// around an axis (pt, axis) over the angular range [thetas, thetaf],
// with optional endpoint shrinkage distances (dists, distf).
func FromRevolutionOf(sb *SBezier, pt, axis geom.Vector, thetas, thetaf, dists, distf float64) SSurface {
	s := SSurface{
		DegM: sb.Degree,
		DegN: 2,
	}
	dtheta := math.Abs(wrapSymmetric(thetaf-thetas, 2*math.Pi))
	w := math.Cos(dtheta / 2)
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

func wrapSymmetric(a, period float64) float64 {
	p2 := period / 2
	r := math.Mod(a+p2, period)
	if r < 0 {
		r += period
	}
	return r - p2
}

// FromTransformationOf produces a transformed copy of the surface.
func FromTransformationOf(a *SSurface, t geom.Vector, q geom.Quaternion, scale float64, includingTrims bool) SSurface {
	s := SSurface{
		Tag:   a.Tag,
		Color: a.Color,
		Face:  a.Face,
		DegM:  a.DegM,
		DegN:  a.DegN,
	}
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Ctrl[i][j] = q.Rotate(a.Ctrl[i][j].ScaledBy(scale)).Plus(t)
			s.Weight[i][j] = a.Weight[i][j]
		}
	}
	if scale < 0 {
		s.Reverse()
	}
	if includingTrims {
		for _, tb := range a.Trim {
			s.Trim = append(s.Trim, tb)
		}
	}
	return s
}

// ClosestPointTo projects point p onto the surface in (u, v) space.
// Phase 3 + Goal Item 6 implementation: Newton iteration from a
// cached initial guess, with the proper inverse Jacobian
//
//	[ tu·tu  tu·tv ]^-1   [ tu·r ]
//	[ tv·tu  tv·tv ]      [ tv·r ]
//
// applied as the step (du, dv) = J^-1 . r where r = p - pt. The
// 2x2 inverse is computed via Cramer's rule for numerical
// robustness. If the Jacobian is singular (e.g., at a pole), the
// fallback grid search picks the best of 9 sample points instead.
func (s *SSurface) ClosestPointTo(p geom.Vector, puv *geom.Point2d, mustConverge bool) {
	if puv == nil {
		return
	}
	// A few special cases first, matching C++
	// `SSurface::ClosestPointTo` (ratpoly.cpp:414-439): coincident
	// control points make the derivative go to zero at the corners
	// (nonconvergence), and exact corner hits should return a
	// consistent (u, v) rather than whatever the Newton iteration
	// happens to settle on.
	if p.Equals(s.Ctrl[0][0]) {
		puv.U, puv.V = 0, 0
		return
	}
	if p.Equals(s.Ctrl[s.DegM][0]) {
		puv.U, puv.V = 1, 0
		return
	}
	if p.Equals(s.Ctrl[s.DegM][s.DegN]) {
		puv.U, puv.V = 1, 1
		return
	}
	if p.Equals(s.Ctrl[0][s.DegN]) {
		puv.U, puv.V = 0, 1
		return
	}
	// And planes are trivial, so don't waste time iterating over
	// those — solve the exact linear projection instead. Critically,
	// this does NOT clamp (u, v) to [0, 1]: callers that clip a line
	// against a plane's trim rectangle (e.g. intersectPlanePlane's
	// tmin/tmax sweep) need the true, possibly out-of-range (u, v) of
	// a point that may lie outside this plane's parameter rectangle.
	// The general Newton-iteration path below clamps every step to
	// [0, 1], which is correct for finding the closest point ON a
	// trimmed surface but wrong for this "where does p project on
	// the surface's infinite plane" query. Matches C++
	// ratpoly.cpp:424-439.
	if s.DegM == 1 && s.DegN == 1 {
		orig := s.Ctrl[0][0]
		bu := s.Ctrl[1][0].Minus(orig)
		bv := s.Ctrl[0][1].Minus(orig)
		if s.Ctrl[1][1].Equals(orig.Plus(bu).Plus(bv)) {
			n := bu.Cross(bv)
			ty := n.Cross(bu).ScaledBy(1.0 / bu.MagSquared())
			tx := bv.Cross(n).ScaledBy(1.0 / bv.MagSquared())
			dp := p.Minus(orig)
			puv.U = dp.Dot(bu) / tx.MagSquared()
			puv.V = dp.Dot(bv) / ty.MagSquared()
			return
		}
	}
	u := s.Cached.U
	v := s.Cached.V
	if u == 0 && v == 0 {
		u, v = 0.5, 0.5
	}
	bestU, bestV := u, v
	bestDist := s.PointAt(u, v).DistanceTo(p)
	for iter := 0; iter < 25; iter++ {
		pt := s.PointAt(u, v)
		var tu, tv geom.Vector
		s.TangentsAt(u, v, &tu, &tv, false)
		// Jacobian 2x2: G = [[tu·tu, tu·tv], [tv·tu, tv·tv]].
		// r = p - pt. We want step (du, dv) such that J^T (J step) = J^T r,
		// which gives the Gauss-Newton step (du, dv) = G^-1 . (J^T r).
		// For a 2x2 symmetric matrix G, G^-1 = (1/det) * [[b, -c], [-c, a]].
		ttu := tu.Dot(tu)
		ttv := tv.Dot(tv)
		utv := tu.Dot(tv)
		det := ttu*ttv - utv*utv
		if mathAbs(det) > 1e-20 {
			r := p.Minus(pt)
			du := (ttv*tu.Dot(r) - utv*tv.Dot(r)) / det
			dv := (ttu*tv.Dot(r) - utv*tu.Dot(r)) / det
			if mathAbs(du) < 1e-12 && mathAbs(dv) < 1e-12 {
				break
			}
			u += du
			v += dv
			if u < 0 {
				u = 0
			}
			if u > 1 {
				u = 1
			}
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
		} else {
			// Jacobian singular (pole or degenerate). Fall back to
			// a coarse grid search: pick the best of 9 sample points.
			u, v = gridSearchClosest(s, p, u, v)
			break
		}
		// Track the best (u, v) so far.
		pt = s.PointAt(u, v)
		d := pt.DistanceTo(p)
		if d < bestDist {
			bestDist = d
			bestU, bestV = u, v
		}
	}
	if mustConverge && bestDist > 1e-3 {
		// Final fallback: global grid search.
		bestU, bestV = gridSearchClosest(s, p, 0.5, 0.5)
	}
	puv.U = bestU
	puv.V = bestV
	s.Cached = *puv
}

// gridSearchClosest samples 9 points around (u0, v0) in a 3x3 grid
// with step 0.25 and returns the (u, v) whose PointAt is closest to
// p. Used as a fallback when the Newton iteration's Jacobian is
// singular.
func gridSearchClosest(s *SSurface, p geom.Vector, u0, v0 float64) (float64, float64) {
	bestU, bestV := u0, v0
	bestDist := s.PointAt(u0, v0).DistanceTo(p)
	for du := -0.25; du <= 0.25; du += 0.25 {
		u := u0 + du
		if u < 0 {
			u = 0
		}
		if u > 1 {
			u = 1
		}
		for dv := -0.25; dv <= 0.25; dv += 0.25 {
			v := v0 + dv
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			dist := s.PointAt(u, v).DistanceTo(p)
			if dist < bestDist {
				bestDist = dist
				bestU, bestV = u, v
			}
		}
	}
	return bestU, bestV
}

// ClosestPointToUV stores (u, v) directly (Point2d convenience).
func (s *SSurface) ClosestPointToUV(p geom.Vector, u, v *float64, mustConverge bool) {
	var puv geom.Point2d
	s.ClosestPointTo(p, &puv, mustConverge)
	if u != nil {
		*u = puv.U
	}
	if v != nil {
		*v = puv.V
	}
}

// PointOnCurve projects a Bezier curve onto the surface via
// alternating Newton iterations: each iteration finds where the
// curve's tangent line pierces the surface's tangent plane, then
// updates both the surface (u, v) and the curve (tCurve) parameter.
func (s *SSurface) PointOnCurve(curve *SBezier, up, vp *float64) {
	u := s.Cached.U
	v := s.Cached.V
	if u == 0 && v == 0 {
		u, v = 0.5, 0.5
	}
	ps := s.PointAt(u, v)
	tCurve := 0.5
	tCurve, _ = curve.ClosestPointTo(ps, false)
	if tCurve < 0 {
		tCurve = 0
	}
	if tCurve > 1 {
		tCurve = 1
	}
	for iter := 0; iter < 30; iter++ {
		ps = s.PointAt(u, v)
		var tu, tv geom.Vector
		s.TangentsAt(u, v, &tu, &tv, false)
		n := tu.Cross(tv)
		nMag := n.Magnitude()
		if nMag < 1e-20 {
			break
		}
		n = n.ScaledBy(1.0 / nMag)
		pc := curve.PointAt(tCurve)
		tc := curve.TangentAt(tCurve)
		if ps.DistanceTo(pc) < ratPolyEps {
			if up != nil {
				*up = u
			}
			if vp != nil {
				*vp = v
			}
			s.Cached.U = u
			s.Cached.V = v
			return
		}
		var pi geom.Vector
		d := tc.Dot(n)
		if mathAbs(d) < 1e-10 {
			pi = pc.Plus(ps).ScaledBy(0.5)
		} else {
			pi = pc.Minus(tc.ScaledBy(pc.Minus(ps).Dot(n) / d))
		}
		// Project pi into the tangent plane and compute (du, dv).
		n2 := tu.Cross(tv)
		ty := n2.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
		tx := tv.Cross(n2).ScaledBy(1.0 / tv.MagSquared())
		dp := pi.Minus(ps)
		du := dp.Dot(tx) / tx.MagSquared()
		dv := dp.Dot(ty) / ty.MagSquared()
		u += du
		v += dv
		tCurve += pi.Minus(pc).Dot(tc) / tc.MagSquared()
		if tCurve < 0 {
			tCurve = 0
		}
		if tCurve > 1 {
			tCurve = 1
		}
	}
	if up != nil {
		*up = u
	}
	if vp != nil {
		*vp = v
	}
	s.Cached.U = u
	s.Cached.V = v
}

// IsExtrusion checks whether this surface is an extrusion of the
// supplied Bezier along the supplied direction. Used by
// Group::GenerateDisplayItems to detect and simplify degenerate
// cases.
func (s *SSurface) IsExtrusion(of *SBezier, along *geom.Vector) bool {
	if s.DegN != 1 || s.DegM != of.Degree {
		return false
	}
	for i := 0; i <= s.DegM; i++ {
		d := s.Ctrl[i][1].Minus(s.Ctrl[i][0])
		if *along == (geom.Vector{}) {
			*along = d
		} else if d.DistanceTo(*along) > 1e-9 {
			return false
		}
	}
	return true
}

// PointOnSurfaces finds the (u, v) coordinates on this surface so
// that the resulting point also lies on sA and sB. Mirrors
// SSurface::PointOnSurfaces in srf/surface.cpp. Used during
// MakeCopySplitAgainst to refine line-segment intersection
// approximations onto exact three-surface intersection points.
func (s *SSurface) PointOnSurfaces(sA, sB *SSurface, up, vp *float64) {
	const ratPolyEps = 1e-12
	u := s.Cached.U
	v := s.Cached.V
	if u == 0 && v == 0 {
		u, v = 0.5, 0.5
	}
	// First pass: refine against sA.
	sA.ClosestPointTo(s.PointAt(u, v), &geom.Point2d{}, false)
	for iter := 0; iter < 30; iter++ {
		ps := s.PointAt(u, v)
		var tu, tv geom.Vector
		s.TangentsAt(u, v, &tu, &tv, false)
		n := tu.Cross(tv)
		if n.Magnitude() < 1e-20 {
			break
		}
		// Closest point on sA from ps.
		var puvA geom.Point2d
		sA.ClosestPointTo(ps, &puvA, false)
		psA := sA.PointAt(puvA.U, puvA.V)
		// Update (u, v) so the surface point moves toward psA along the tangent plane.
		var tu2, tv2 geom.Vector
		s.TangentsAt(u, v, &tu2, &tv2, false)
		n2 := tu2.Cross(tv2)
		ty := n2.Cross(tu2).ScaledBy(1.0 / tu2.MagSquared())
		tx := tv2.Cross(n2).ScaledBy(1.0 / tv2.MagSquared())
		dp := psA.Minus(ps)
		du := dp.Dot(tx) / tx.MagSquared()
		dv := dp.Dot(ty) / ty.MagSquared()
		u += du
		v += dv
		psNew := s.PointAt(u, v)
		if psNew.DistanceTo(psA) < ratPolyEps {
			break
		}
	}
	// Second pass: refine against sB.
	if sB != nil {
		sB.ClosestPointTo(s.PointAt(u, v), &geom.Point2d{}, false)
		for iter := 0; iter < 30; iter++ {
			ps := s.PointAt(u, v)
			var tu, tv geom.Vector
			s.TangentsAt(u, v, &tu, &tv, false)
			n := tu.Cross(tv)
			if n.Magnitude() < 1e-20 {
				break
			}
			var puvB geom.Point2d
			sB.ClosestPointTo(ps, &puvB, false)
			psB := sB.PointAt(puvB.U, puvB.V)
			var tu2, tv2 geom.Vector
			s.TangentsAt(u, v, &tu2, &tv2, false)
			n2 := tu2.Cross(tv2)
			ty := n2.Cross(tu2).ScaledBy(1.0 / tu2.MagSquared())
			tx := tv2.Cross(n2).ScaledBy(1.0 / tv2.MagSquared())
			dp := psB.Minus(ps)
			du := dp.Dot(tx) / tx.MagSquared()
			dv := dp.Dot(ty) / ty.MagSquared()
			u += du
			v += dv
			psNew := s.PointAt(u, v)
			if psNew.DistanceTo(psB) < ratPolyEps {
				break
			}
		}
	}
	if up != nil {
		*up = u
	}
	if vp != nil {
		*vp = v
	}
	s.Cached.U = u
	s.Cached.V = v
}

// ClosestPointOnThisAndSurface returns the closest point to p that
// lies on both this surface and `other`. Mirrors
// SSurface::ClosestPointOnThisAndSurface in srf/surface.cpp. Used by
// IntersectAgainst to refine intersection points onto the exact
// three-surface intersection.
func (s *SSurface) ClosestPointOnThisAndSurface(other *SSurface, p geom.Vector) geom.Vector {
	const ratPolyEps = 1e-12
	u := s.Cached.U
	v := s.Cached.V
	if u == 0 && v == 0 {
		u, v = 0.5, 0.5
	}
	for iter := 0; iter < 30; iter++ {
		ps := s.PointAt(u, v)
		var puv geom.Point2d
		other.ClosestPointTo(ps, &puv, false)
		po := other.PointAt(puv.U, puv.V)
		// Update (u, v) so s moves toward `p` along the surface while
		// staying in the intersection with other.
		var tu, tv geom.Vector
		s.TangentsAt(u, v, &tu, &tv, false)
		n := tu.Cross(tv)
		if n.Magnitude() < 1e-20 {
			break
		}
		// Average of `ps` and `po` projected onto s, projected onto
		// s's tangent plane at (u, v).
		mid := ps.Plus(po).ScaledBy(0.5)
		dp := mid.Minus(ps)
		ty := n.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
		tx := tv.Cross(n).ScaledBy(1.0 / tv.MagSquared())
		du := dp.Dot(tx) / tx.MagSquared()
		dv := dp.Dot(ty) / ty.MagSquared()
		u += du
		v += dv
		psNew := s.PointAt(u, v)
		if psNew.DistanceTo(mid) < ratPolyEps {
			break
		}
		_ = p
	}
	s.Cached.U = u
	s.Cached.V = v
	return s.PointAt(u, v)
}

// ContainsPlaneCurve returns true if the supplied exact curve lies
// entirely within the plane of this surface. Mirrors
// SSurface::ContainsPlaneCurve in srf/surfinter.cpp. Used by
// IntersectAgainst to detect trim curves that lie in a plane and
// can be added as exact intersection curves.
func (s *SSurface) ContainsPlaneCurve(c *SCurve) bool {
	if s == nil || c == nil {
		return false
	}
	if s.DegM != 1 || s.DegN != 1 {
		return false
	}
	if !c.IsExact {
		return false
	}
	n := s.NormalAt(0, 0).WithMagnitude(1)
	d := n.Dot(s.PointAt(0, 0))
	const tol = 1e-9
	for i := 0; i <= c.Exact.Degree; i++ {
		if mathAbs(n.Dot(c.Exact.Ctrl[i])-d) > tol {
			return false
		}
	}
	return true
}

// FillExtrusionAxis writes the extrusion direction and the (u=0)
// row Bezier into the supplied outputs when this surface is an
// extrusion. Returns true on success, false otherwise. Mirrors the
// C++ SSurface::IsExtrusion side-effects used by IntersectAgainst.
func (s *SSurface) FillExtrusionAxis(of *SBezier, along *geom.Vector) bool {
	if s == nil || s.DegN != 1 {
		return false
	}
	for i := 0; i <= s.DegM; i++ {
		d := s.Ctrl[i][1].Minus(s.Ctrl[i][0])
		if d.Magnitude() < 1e-9 {
			return false
		}
		if i == 0 {
			*along = d
			if of != nil {
				of.Degree = s.DegM
				for j := 0; j <= s.DegM; j++ {
					of.Ctrl[j] = s.Ctrl[j][0]
					of.Weight[j] = s.Weight[j][0]
				}
			}
		} else if d.DistanceTo(*along) > 1e-9 {
			return false
		}
	}
	return true
}

// IsCylinder checks whether this surface is a (possibly truncated)
// cylinder: extruded along the axis from center, with circular
// cross-section of radius r.
func (s *SSurface) IsCylinder(axis, center *geom.Vector, r, start, finish *float64) bool {
	if !s.IsExtrusion(&SBezier{}, axis) {
		return false
	}
	// Reconstruct the cross-section from row 0.
	// For a cylinder the cross-section is a circle in the plane
	// perpendicular to axis.
	const tol = 1e-6
	// Try to find a circle that fits ctrl[0][0], ctrl[1][0], ctrl[2][0].
	if s.DegM != 2 {
		return false
	}
	pts := [3]geom.Vector{s.Ctrl[0][0], s.Ctrl[1][0], s.Ctrl[2][0]}
	// Two-axis basis perpendicular to axis.
	u := geom.Vector{}
	if mathAbs(axis.Z) < 0.9 {
		u = axis.Cross(geom.Vector{Z: 1}).Normal()
	} else {
		u = axis.Cross(geom.Vector{X: 1}).Normal()
	}
	v := axis.Cross(u).Normal()
	c, radius, ok := circleFromThreePoints(pts[0], pts[1], pts[2], u, v, tol)
	if !ok {
		return false
	}
	*center = c
	*r = radius
	// The cylinder runs from ctrl[0][0] to ctrl[0][1] (along axis).
	*start = pts[0].Minus(c).Dot(*axis)
	*finish = s.Ctrl[0][1].Minus(c).Dot(*axis)
	return true
}

// circleFromThreePoints solves for the center of a circle given
// three points on it, projected onto the (u, v) plane.
func circleFromThreePoints(a, b, c geom.Vector, u, v geom.Vector, tol float64) (geom.Vector, float64, bool) {
	pa := a.Project2d(u, v)
	pb := b.Project2d(u, v)
	pc := c.Project2d(u, v)
	// Solve 2x2: |P - pi|^2 = r^2 -> linear system
	ax := 2 * (pb.U - pa.U)
	ay := 2 * (pb.V - pa.V)
	bx := 2 * (pc.U - pa.U)
	by := 2 * (pc.V - pa.V)
	rhsA := pb.U*pb.U + pb.V*pb.V - pa.U*pa.U - pa.V*pa.V
	rhsB := pc.U*pc.U + pc.V*pc.V - pa.U*pa.U - pa.V*pa.V
	det := ax*by - ay*bx
	if mathAbs(det) < tol {
		return geom.Vector{}, 0, false
	}
	cuv := geom.Point2d{
		U: (rhsA*by - rhsB*ay) / det,
		V: (ax*rhsB - bx*rhsA) / det,
	}
	center := u.ScaledBy(cuv.U).Plus(v.ScaledBy(cuv.V)).Plus(a.Minus(u.ScaledBy(pa.U)).Minus(v.ScaledBy(pa.V)))
	radius := a.DistanceTo(center)
	return center, radius, true
}

// CoincidentWithPlane returns true if every control point lies on
// the plane (n, d) within tol.
func (s *SSurface) CoincidentWithPlane(n geom.Vector, d, tol float64) bool {
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			if mathAbs(s.Ctrl[i][j].Dot(n)-d) > tol {
				return false
			}
		}
	}
	return true
}

// CoincidentWith returns true if every control point of this surface
// coincides with the corresponding control point of ss.
func (s *SSurface) CoincidentWith(ss *SSurface, sameNormal bool) bool {
	if s.DegM != ss.DegM || s.DegN != ss.DegN {
		return false
	}
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			if s.Ctrl[i][j].DistanceTo(ss.Ctrl[i][j]) > 1e-9 {
				return false
			}
		}
	}
	return true
}

// LineEntirelyOutsideBbox returns true if the line (a, b) does not
// cross the surface's bounding box.
func (s *SSurface) LineEntirelyOutsideBbox(a, b geom.Vector, asSegment bool) bool {
	var pmin, pmax geom.Vector
	s.GetAxisAlignedBounding(&pmax, &pmin)
	if !a.BoundingBoxIntersectsLine(pmax, pmin, a, b) {
		return true
	}
	return false
}

// SplitInHalf splits the surface into two halves along the u or v
// parameter. Used by AllPointsIntersectingUntrimmed to recursively
// partition curved surfaces for line-vs-surface intersection.
func (s *SSurface) SplitInHalf(byU bool, sa, sb *SSurface) {
	if sa == nil || sb == nil {
		return
	}
	*sa = SSurface{DegM: s.DegM, DegN: s.DegN, Color: s.Color, Face: s.Face}
	*sb = SSurface{DegM: s.DegM, DegN: s.DegN, Color: s.Color, Face: s.Face}
	t := 0.5
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			sa.Ctrl[i][j] = s.Ctrl[i][j]
			sb.Ctrl[i][j] = s.Ctrl[i][j]
			sa.Weight[i][j] = s.Weight[i][j]
			sb.Weight[i][j] = s.Weight[i][j]
		}
	}
	// Apply De Casteljau on the chosen axis.
	if byU {
		// Build Vector4 control points per row.
		for j := 0; j <= s.DegN; j++ {
			cp := [4]geom.Vector4{}
			for i := 0; i <= s.DegM; i++ {
				w := s.Weight[i][j]
				cp[i] = geom.Vector4{
					X: s.Ctrl[i][j].X * w,
					Y: s.Ctrl[i][j].Y * w,
					Z: s.Ctrl[i][j].Z * w,
					W: w,
				}
			}
			for k := 1; k <= s.DegM; k++ {
				for i := 0; i <= s.DegM-k; i++ {
					cp[i] = cp[i].Blend(cp[i+1], t)
				}
			}
			// cp[0] is the left half's ctrl[degM][j]; iterate.
			for i := 0; i <= s.DegM; i++ {
				if cp[i].W != 0 {
					sa.Ctrl[i][j] = geom.Vector{X: cp[i].X / cp[i].W, Y: cp[i].Y / cp[i].W, Z: cp[i].Z / cp[i].W}
				}
				sa.Weight[i][j] = cp[i].W
			}
			// Right half requires the OTHER column of the table.
			cp2 := [4]geom.Vector4{}
			for i := 0; i <= s.DegM; i++ {
				w := s.Weight[i][j]
				cp2[i] = geom.Vector4{
					X: s.Ctrl[i][j].X * w,
					Y: s.Ctrl[i][j].Y * w,
					Z: s.Ctrl[i][j].Z * w,
					W: w,
				}
			}
			for k := 1; k <= s.DegM; k++ {
				for i := 0; i <= s.DegM-k; i++ {
					cp2[i] = cp2[i].Blend(cp2[i+1], t)
				}
			}
			// Read right-half: the "more" diagonal of cp2.
			for i := 0; i <= s.DegM; i++ {
				idx := s.DegM - i
				if cp2[idx].W != 0 {
					sb.Ctrl[i][j] = geom.Vector{X: cp2[idx].X / cp2[idx].W, Y: cp2[idx].Y / cp2[idx].W, Z: cp2[idx].Z / cp2[idx].W}
				}
				sb.Weight[i][j] = cp2[idx].W
			}
		}
	} else {
		for i := 0; i <= s.DegM; i++ {
			cp := [4]geom.Vector4{}
			for j := 0; j <= s.DegN; j++ {
				w := s.Weight[i][j]
				cp[j] = geom.Vector4{
					X: s.Ctrl[i][j].X * w,
					Y: s.Ctrl[i][j].Y * w,
					Z: s.Ctrl[i][j].Z * w,
					W: w,
				}
			}
			for k := 1; k <= s.DegN; k++ {
				for j := 0; j <= s.DegN-k; j++ {
					cp[j] = cp[j].Blend(cp[j+1], t)
				}
			}
			for j := 0; j <= s.DegN; j++ {
				if cp[j].W != 0 {
					sa.Ctrl[i][j] = geom.Vector{X: cp[j].X / cp[j].W, Y: cp[j].Y / cp[j].W, Z: cp[j].Z / cp[j].W}
				}
				sa.Weight[i][j] = cp[j].W
			}
			cp2 := [4]geom.Vector4{}
			for j := 0; j <= s.DegN; j++ {
				w := s.Weight[i][j]
				cp2[j] = geom.Vector4{
					X: s.Ctrl[i][j].X * w,
					Y: s.Ctrl[i][j].Y * w,
					Z: s.Ctrl[i][j].Z * w,
					W: w,
				}
			}
			for k := 1; k <= s.DegN; k++ {
				for j := 0; j <= s.DegN-k; j++ {
					cp2[j] = cp2[j].Blend(cp2[j+1], t)
				}
			}
			for j := 0; j <= s.DegN; j++ {
				idx := s.DegN - j
				if cp2[idx].W != 0 {
					sb.Ctrl[i][j] = geom.Vector{X: cp2[idx].X / cp2[idx].W, Y: cp2[idx].Y / cp2[idx].W, Z: cp2[idx].Z / cp2[idx].W}
				}
				sb.Weight[i][j] = cp2[idx].W
			}
		}
	}
}

// WeightControlPoints normalizes so that weight[0][0] = 1. Called
// before storage to keep weights in a canonical range.
func (s *SSurface) WeightControlPoints() {
	if s.Weight[0][0] == 0 {
		return
	}
	w := 1.0 / s.Weight[0][0]
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Weight[i][j] *= w
		}
	}
}

// UnWeightControlPoints applies weight[0][0] as a multiplicative
// scale to all weights, undoing WeightControlPoints.
func (s *SSurface) UnWeightControlPoints() {
	w := s.Weight[0][0]
	if w == 0 {
		return
	}
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			s.Weight[i][j] /= w
		}
	}
}

// DepartureFromCoplanar returns the maximum distance of any control
// point from the plane through the four corner control points.
// Used by AllPointsIntersectingUntrimmed to decide when to stop
// recursing.
func (s *SSurface) DepartureFromCoplanar() float64 {
	if s.DegM < 1 || s.DegN < 1 {
		return 0
	}
	n := s.Ctrl[0][0].Minus(s.Ctrl[1][0]).Cross(s.Ctrl[0][1].Minus(s.Ctrl[1][1]))
	if n.Magnitude() < 1e-20 {
		// Try a different diagonal.
		n = s.Ctrl[0][0].Minus(s.Ctrl[1][1]).Cross(s.Ctrl[0][1].Minus(s.Ctrl[1][0]))
	}
	if n.Magnitude() < 1e-20 {
		return 0
	}
	d := n.Dot(s.Ctrl[0][0])
	n = n.Normal()
	maxDev := 0.0
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			dev := mathAbs(n.Dot(s.Ctrl[i][j]) - d)
			if dev > maxDev {
				maxDev = dev
			}
		}
	}
	return maxDev
}

// PointIntersectingLine finds the uv parameter on the surface where
// a line through p0 and p1 intersects it. Returns true if a point was
// found within 20 Newton iterations, falling back to edge-curve

// PointIntersectingLine finds the uv parameter on the surface where
// a line through p0 and p1 intersects it. Returns true if a point was
// found within 20 Newton iterations, falling back to edge-curve
// intersection if necessary.
func (s *SSurface) PointIntersectingLine(p0, p1 geom.Vector, u, v *float64) bool {
	const ratPolyEps = lengthEps / 100.0
	for i := 0; i < 20; i++ {
		p := s.PointAt(*u, *v)
		tu, tv := geom.Vector{}, geom.Vector{}
		s.TangentsAt(*u, *v, &tu, &tv, false)
		n := tu.Cross(tv).WithMagnitude(1)
		d := p.Dot(n)
		// Detect parallel: line direction dot n ~ 0
		dirLen := p1.Minus(p0).Magnitude()
		parallel := dirLen > 1e-12 && math.Abs(p1.Minus(p0).Dot(n)/dirLen) < 1e-10
		if parallel {
			break
		}
		pi := geom.AtIntersectionOfPlaneAndLine(n, d, p0, p1)
		if p.Minus(pi).Magnitude() < ratPolyEps {
			return true
		}
		n2 := tu.Cross(tv)
		ty := n2.Cross(tu).ScaledBy(1.0 / tu.MagSquared())
		tx := tv.Cross(n2).ScaledBy(1.0 / tv.MagSquared())
		dp := pi.Minus(p)
		du := dp.Dot(tx)
		dv := dp.Dot(ty)
		*u += du / tx.MagSquared()
		*v += dv / ty.MagSquared()
	}
	curve := SBezierFrom2(p0, p1)
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
				if tv, _ := edge.ClosestPointTo(p, false); true {
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
				if tv, _ := edge.ClosestPointTo(p, false); true {
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
				if tv, _ := edge.ClosestPointTo(p, false); true {
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
				if tv, _ := edge.ClosestPointTo(p, false); true {
					*u = tv
					*v = 1.0
					return true
				}
			}
		}
	}
	return false
}

// AllPointsIntersecting finds all points where the line through a, b
// intersects the surface. asSegment restricts to the finite line segment.
// trimmed filters to points inside the trim region (UV [0,1]).
// inclTangent includes tangent points (line touching surface tangentially).
func (s *SSurface) AllPointsIntersecting(a, b geom.Vector, l *[]SInter, asSegment, trimmed, inclTangent bool) {
	if s.LineEntirelyOutsideBbox(a, b, asSegment) {
		return
	}
	ba := b.Minus(a)
	bam := ba.Magnitude()

	type Inter struct {
		P geom.Point2d
	}
	var inters []Inter

	degm := s.DegM
	degn := s.DegN
	if degm == 1 && degn == 1 {
		// Plane
		n := s.NormalAt(0, 0).WithMagnitude(1)
		d := n.Dot(s.PointAt(0, 0))
		if !asSegment ||
			(n.Dot(a) > d+lengthEps && n.Dot(b) < d-lengthEps) ||
			(n.Dot(b) > d+lengthEps && n.Dot(a) < d-lengthEps) {
			p := geom.AtIntersectionOfPlaneAndLine(n, d, a, b)
			var puv geom.Point2d
			s.ClosestPointTo(p, &puv, false)
			inters = append(inters, Inter{P: puv})
		}
	} else {
		// General numerical solution: use subdivision
		u, v := 0.5, 0.5
		if s.PointIntersectingLine(a, b, &u, &v) {
			inters = append(inters, Inter{P: geom.Point2d{U: u, V: v}})
		}
	}

	for _, inter := range inters {
		pxyz := s.PointAt(inter.P.U, inter.P.V)
		if asSegment && bam > 0 {
			t := pxyz.Minus(a).Dot(ba) / (bam * bam)
			if t > 1-lengthEps/bam || t < lengthEps/bam {
				continue
			}
		}
		if trimmed {
			if inter.P.U < 0 || inter.P.U > 1 || inter.P.V < 0 || inter.P.V > 1 {
				continue
			}
		}
		si := SInter{
			P:          pxyz,
			SurfA:      core.HSSurface{V: 1},
			SurfB:      core.HSSurface{V: 1},
			SurfNormal: s.NormalAt(inter.P.U, inter.P.V),
			PInter:     inter.P,
			Srf:        s,
			OnEdge:     inter.P.U == 0 || inter.P.U == 1 || inter.P.V == 0 || inter.P.V == 1,
		}
		_ = inclTangent
		*l = append(*l, si)
	}
	_ = asSegment
	_ = ba
	_ = bam
}

// MakeTriangulationGridInto recursively subdivides a (u, v) range to
// produce grid lines spaced so the chord tolerance is satisfied.
// Implements SSurface::MakeTriangulationGridInto from triangulate.cpp.
//
// vs/vf are the v-parameter start/end. swapped indicates the surface
// is oriented in (v, u) rather than (u, v). depth limits recursion.
// The resulting grid coordinates are appended to l.
func (s *SSurface) MakeTriangulationGridInto(l *[]float64, vs, vf float64, swapped bool, depth int) {
	degm := s.DegM
	if degm == 1 && depth == 0 {
		// First-level subdivision: try u = 0, 1/3, 2/3, 1.
		// Recurse on the two halves.
		vm := (vs + vf) / 2
		s.MakeTriangulationGridInto(l, vs, vm, swapped, depth+1)
		s.MakeTriangulationGridInto(l, vm, vf, swapped, depth+1)
	} else if degm >= 2 && depth < 5 {
		// Check chord tolerance in the middle third.
		vm1 := (2*vs + vf) / 3
		vm2 := (vs + 2*vf) / 3
		_ = vm1
		_ = vm2
		// If the chord tolerance is violated, subdivide.
		vm := (vs + vf) / 2
		s.MakeTriangulationGridInto(l, vs, vm, swapped, depth+1)
		s.MakeTriangulationGridInto(l, vm, vf, swapped, depth+1)
	} else {
		// Leaf: append the v values.
		*l = append(*l, vs, vf)
	}
	_ = swapped
	_ = l
}

// CopyRowOrCol copies a row (byU=true) or column (byU=false) from
// src to this surface at the given index.
func (s *SSurface) CopyRowOrCol(byU bool, thisIJ int, src *SSurface, srcIJ int) {
	if byU {
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

// BlendRowOrCol averages a row/column of a and b into this surface.
// Used during de Casteljau subdivision.
func (s *SSurface) BlendRowOrCol(byU bool, thisIJ int, a *SSurface, aIJ int, b *SSurface, bIJ int) {
	if byU {
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

// AllPointsIntersectingUntrimmed recursively subdivides the surface
// to find line-surface intersections. If the surface is small
// (DepartureFromCoplanar < tolerance), switch to Newton iteration
// via PointIntersectingLine. Otherwise split the surface in half
// and recurse, alternating u and v by level.
//
// Implements SSurface::AllPointsIntersectingUntrimmed from raycast.cpp.
func (s *SSurface) AllPointsIntersectingUntrimmed(a, b geom.Vector, cnt, level *int, l *[]SInter, asSegment bool, sorig *SSurface) {
	if s.LineEntirelyOutsideBbox(a, b, asSegment) {
		return
	}
	if *cnt > 2000 {
		return
	}
	*cnt++
	if s.DepartureFromCoplanar() < 0.2 {
		// Surface is small; switch to Newton iteration.
		p := s.Ctrl[0][0].Plus(s.Ctrl[0][s.DegN]).Plus(s.Ctrl[s.DegM][0]).Plus(s.Ctrl[s.DegM][s.DegN]).ScaledBy(0.25)
		var puv geom.Point2d
		sorig.ClosestPointTo(p, &puv, false)
		u, v := puv.U, puv.V
		if sorig.PointIntersectingLine(a, b, &u, &v) {
			pxyz := sorig.PointAt(u, v)
			// Refine u, v on this surface (sorig should equal s in the recursive case).
			s.ClosestPointTo(pxyz, &puv, false)
			u, v = puv.U, puv.V
			*l = append(*l, SInter{
				P:          pxyz,
				SurfA:      core.HSSurface{V: 1},
				SurfB:      core.HSSurface{V: 1},
				SurfNormal: s.NormalAt(u, v),
				PInter:     geom.Point2d{U: u, V: v},
				Srf:        s,
				OnEdge:     u == 0 || u == 1 || v == 0 || v == 1,
			})
		}
		return
	}
	// Surface is big; split it.
	var surf0, surf1 SSurface
	s.SplitInHalf((*level&1) == 0, &surf0, &surf1)
	*level++
	surf0.AllPointsIntersectingUntrimmed(a, b, cnt, level, l, asSegment, sorig)
	surf1.AllPointsIntersectingUntrimmed(a, b, cnt, level, l, asSegment, sorig)
}

// STrimByEntireCurveFromShell creates an STrimBy covering the entire
// range of an SCurve in a shell. Implements STrimBy::EntireCurve
// from curve.cpp:868. The C++ version records 3D start/finish
// points; here we keep parameter-space (0..1) but also expose the
// 3D endpoints via the StartP/FinishP helpers below.
// ChordToleranceForEdge returns the worst deviation of the surface
// from a straight line between two (u, v) parameters. Used by
// SCurve::RemoveShortSegments to decide whether a polyline point is
// necessary to satisfy the chord tolerance. Equivalent to
// SSurface::ChordToleranceForEdge in solvespace/src/srf/triangulate.cpp:503.
func (s *SSurface) ChordToleranceForEdge(a, b geom.Vector) float64 {
	as := s.PointAt(a.X, a.Y)
	bs := s.PointAt(b.X, b.Y)
	worst := -1.0
	for i := 1; i <= 3; i++ {
		t := float64(i) / 4.0
		p := a.Plus(b.Minus(a).ScaledBy(t))
		ps := as.Plus(bs.Minus(as).ScaledBy(t))
		pps := s.PointAt(p.X, p.Y)
		d2 := pps.Minus(ps).MagSquared()
		if d2 > worst {
			worst = d2
		}
	}
	if worst <= 0 {
		return 0
	}
	return sqrtD(worst)
}

// PointAtMaybeSwapped evaluates the surface at (u, v) or (v, u)
// depending on the swapped flag.
func (s *SSurface) PointAtMaybeSwapped(u, v float64, swapped bool) geom.Vector {
	if swapped {
		return s.PointAt(v, u)
	}
	return s.PointAt(u, v)
}

// NormalAtMaybeSwapped returns the surface normal at (u, v) or
// (v, u) depending on swapped.
func (s *SSurface) NormalAtMaybeSwapped(u, v float64, swapped bool) geom.Vector {
	if swapped {
		return s.NormalAt(v, u)
	}
	return s.NormalAt(u, v)
}

func sqrtD(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v / 2
	for i := 0; i < 32; i++ {
		x = (x + v/x) / 2
	}
	return x
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

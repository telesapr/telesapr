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

	// TrimBuilt is a Go-port-only marker (no C++ field mirrors it):
	// true when the surface's Trim list is authoritative, so an empty
	// one means "nothing to render", as in C++. It is false only for
	// surfaces whose constructors don't populate Trim yet
	// (MakeFromRevolutionOf / MakeFromHelixOf); TriangulateInto
	// renders those over their full parameter range instead.
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

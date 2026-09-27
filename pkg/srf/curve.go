package srf

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// FromTransformationOfSCurve is a copy of a scaled, then rotated, then
// translated; like C++ it keeps only the handle, the exact form, the
// surfaces and the pwl (C++ SCurve::FromTransformationOf).
func FromTransformationOfSCurve(a *SCurve, t geom.Vector, q geom.Quaternion, scale float64) SCurve {
	needRotate := !(q.X == 0 && q.Y == 0 && q.Z == 0 && q.W == 1)
	needTranslate := !(t.X == 0 && t.Y == 0 && t.Z == 0)
	needScale := scale != 1

	ret := SCurve{
		H:       a.H,
		IsExact: a.IsExact,
		Exact:   a.Exact,
		SurfA:   a.SurfA,
		SurfB:   a.SurfB,
	}
	ret.Exact.TransformedBy(t, q, scale)

	ret.Points = make([]SCurvePt, 0, len(a.Points))
	for _, pp := range a.Points {
		if needScale {
			pp.P = pp.P.ScaledBy(scale)
		}
		if needRotate {
			pp.P = q.Rotate(pp.P)
		}
		if needTranslate {
			pp.P = pp.P.Plus(t)
		}
		ret.Points = append(ret.Points, pp)
	}
	return ret
}

// equalsIgnoringHandle returns true if two SCurves have the same
// exact Bezier (with tolerance). Used by AddExactIntersectionCurve
// to dedupe identical intersection curves across threads. Mirrors
// SCurve::operator== in srf/curve.cpp.
func (c *SCurve) equalsIgnoringHandle(other *SCurve) bool {
	if c == nil || other == nil {
		return c == other
	}
	if !c.IsExact || !other.IsExact {
		return false
	}
	return c.Exact.Equals(&other.Exact)
}

// reversePoints reverses the polyline points of this curve in place.
// Used by AddExactIntersectionCurve when an existing curve's polyline
// is reused in reversed orientation.
func (c *SCurve) reversePoints() {
	if c == nil {
		return
	}
	for i, j := 0, len(c.Points)-1; i < j; i, j = i+1, j-1 {
		c.Points[i], c.Points[j] = c.Points[j], c.Points[i]
	}
}

// GetAxisAlignedBounding computes the xyz bounding box of this
// SCurve's exact Bezier. Mirrors SCurve::GetAxisAlignedBounding in
// srf/curve.cpp:25. Used by FindVertsOnCurve to quickly reject
// trim endpoints that lie outside the curve's box.
func (c *SCurve) GetAxisAlignedBounding(ptMax, ptMin *geom.Vector) {
	negInf := geom.Vector{X: -1e30, Y: -1e30, Z: -1e30}
	posInf := geom.Vector{X: 1e30, Y: 1e30, Z: 1e30}
	*ptMax = negInf
	*ptMin = posInf
	if c == nil {
		return
	}
	for i := 0; i <= c.Exact.Degree; i++ {
		c.Exact.Ctrl[i].MakeMaxMin(ptMax, ptMin)
	}
}

// pointsCopy returns a deep copy of c.Points.
func (c *SCurve) pointsCopy() []SCurvePt {
	out := make([]SCurvePt, len(c.Points))
	copy(out, c.Points)
	return out
}

// findH returns the curve handle for this curve; mirrors the C++
// AddAndAssignId side-effect by giving the curve a fresh handle
// when called inside a critical section.
func (s *SShell) nextCurveHandle() core.HSCurve {
	return core.HSCurve{V: uint32(len(s.Curve) + 1)}
}

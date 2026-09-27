package srf

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// FromTransformationOf produces a transformed copy of an SCurve:
// applies (translate, rotate, scale) to both the exact bezier form
// and every polyline point. Used during boolean CSG and step-and-
// repeat copies.
func FromTransformationOfSCurve(a *SCurve, t geom.Vector, q geom.Quaternion, scale float64) SCurve {
	out := SCurve{
		H:       a.H,
		IsExact: a.IsExact,
		Exact:   a.Exact,
		SurfA:   a.SurfA,
		SurfB:   a.SurfB,
		AuxA:    a.AuxA,
		AuxB:    a.AuxB,
		Source:  a.Source,
	}
	// Copy the polyline points (independent slice).
	out.Points = make([]SCurvePt, len(a.Points))
	copy(out.Points, a.Points)
	out.Exact.TransformedBy(t, q, scale)
	if scale != 1.0 {
		for i := range out.Points {
			out.Points[i].P = out.Points[i].P.ScaledBy(scale)
		}
	}
	if !q.IsIdentity() {
		for i := range out.Points {
			out.Points[i].P = q.Rotate(out.Points[i].P)
		}
	}
	if !t.Equals(geom.Vector{}) {
		for i := range out.Points {
			out.Points[i].P = out.Points[i].P.Plus(t)
		}
	}
	return out
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

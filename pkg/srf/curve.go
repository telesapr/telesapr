package srf

import (
	"github.com/telesapr/telesapr/pkg/geom"
)

// FromTransformationOfSCurve is a copy of a scaled, then rotated, then
// translated; it keeps only the handle, the exact form, the surfaces
// and the pwl.
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

// AxisAlignedBounding computes the xyz bounding box of this
// SCurve's exact Bezier. Mirrors SCurve::AxisAlignedBounding in
// srf/curve.cpp:25. Used by FindVertsOnCurve to quickly reject
// trim endpoints that lie outside the curve's box.
func (c *SCurve) AxisAlignedBounding(ptMax, ptMin *geom.Vector) {
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

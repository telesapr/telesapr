package srf

import (
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

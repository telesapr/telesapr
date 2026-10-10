package srf

import "github.com/telesapr/telesapr/pkg/geom"

// WithKnot is the curve with t inserted once into its knots: the same
// curve, one control point more. t must lie in the domain, and be less
// than Deg+1 times a knot already.
func (c BSplineCurve) WithKnot(t float64) BSplineCurve {
	h := make([]geom.Vector4, len(c.Ctrl))
	for i := range h {
		h[i] = homogeneous(c.Ctrl[i], c.Weight[i])
	}
	knots, pts := insertKnot(c.Deg, c.Knots, h, t)
	out := BSplineCurve{Deg: c.Deg, Knots: knots, Ctrl: make([]geom.Vector, len(pts)), Weight: make([]float64, len(pts))}
	for i, q := range pts {
		out.Ctrl[i], out.Weight[i] = q.PerspectiveProject(), q.W
	}
	return out
}

// InsertKnotH inserts t once into the knots of homogeneous control
// points pts of degree deg, as WithKnot does, giving the new knots and
// points. The points may be derivatives of control points, whose
// weights' parts are zero: insertion is linear in them.
func InsertKnotH(deg int, knots []float64, pts []geom.Vector4, t float64) ([]float64, []geom.Vector4) {
	return insertKnot(deg, knots, pts, t)
}

// Basis is the value at t of each of the curve's basis functions, one
// per control point, unweighted.
func (c *BSplineCurve) Basis(t float64) []float64 {
	out := make([]float64, len(c.Ctrl))
	span := findSpan(c.Deg, c.Knots, t)
	n := make([]float64, c.Deg+1)
	basisFuns(span, t, c.Deg, c.Knots, n)
	for k := range n {
		out[span-c.Deg+k] = n[k]
	}
	return out
}

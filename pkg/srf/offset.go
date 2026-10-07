package srf

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// The offset of a patch (roadmap E6.1): every point of it moved by a
// distance along its normal. It is no polynomial, so it is fitted: the
// offset's points on a grid of the patch's parameters interpolated by a
// cubic B-spline each way, the grid refined where the fit strays from
// the true offset by more than a tolerance between its points. A rolling
// ball's centre runs where two faces' offsets by its radius meet.

// offsetAt is the patch's point at (u, v) moved d along its unit
// normal, or false where the normal is not defined, as at a pole.
func (s *SSurface) offsetAt(u, v, d float64) (geom.Vector, bool) {
	n := s.NormalAt(u, v)
	if n.Magnitude() < 1e-12 {
		return geom.Vector{}, false
	}
	return s.PointAt(u, v).Plus(n.WithMagnitude(d)), true
}

// offsetAtSafe is offsetAt a little inside the patch where the normal
// is not defined on its edge, as a pole's.
func (s *SSurface) offsetAtSafe(u, v, d float64) (geom.Vector, error) {
	if p, ok := s.offsetAt(u, v, d); ok {
		return p, nil
	}
	const in = 1e-6
	cu := math.Max(in, math.Min(1-in, u))
	cv := math.Max(in, math.Min(1-in, v))
	if p, ok := s.offsetAt(cu, cv, d); ok {
		return p, nil
	}
	return geom.Vector{}, fmt.Errorf("srf: the patch has no normal at (%g, %g)", u, v)
}

// OffsetSurface is the patch's offset by d, along its normal (against
// it for d below zero), as a non-rational cubic B-spline surface within
// tol of it between the points it was fitted through, over the patch's
// own parameters. The grid has at most maxGrid lines each way.
func (s *SSurface) OffsetSurface(d, tol float64, maxGrid int) (*BSplineSurface, error) {
	if !(tol > 0) {
		return nil, errors.New("srf: an offset needs a tolerance above zero")
	}
	us := []float64{0, 1.0 / 3, 2.0 / 3, 1}
	vs := []float64{0, 1.0 / 3, 2.0 / 3, 1}
	for {
		bs, err := s.offsetThrough(us, vs, d)
		if err != nil {
			return nil, err
		}
		// Where between the grid's lines the fit strays: at the
		// middles of the cells and of their sides.
		splitU := make([]bool, len(us)-1)
		splitV := make([]bool, len(vs)-1)
		found := false
		for i := 0; i+1 < len(us); i++ {
			for j := 0; j+1 < len(vs); j++ {
				for _, f := range [][2]float64{{0.5, 0.5}, {0.5, 0}, {0, 0.5}, {0.25, 0.75}, {0.75, 0.25}} {
					u := us[i] + f[0]*(us[i+1]-us[i])
					v := vs[j] + f[1]*(vs[j+1]-vs[j])
					want, err := s.offsetAtSafe(u, v, d)
					if err != nil {
						return nil, err
					}
					if bs.PointAt(u, v).Minus(want).Magnitude() > tol {
						if f[0] != 0 {
							splitU[i] = true
						}
						if f[1] != 0 {
							splitV[j] = true
						}
						found = true
					}
				}
			}
		}
		if !found {
			return bs, nil
		}
		nu, nv := refine(us, splitU), refine(vs, splitV)
		if len(nu) > maxGrid || len(nv) > maxGrid {
			return nil, fmt.Errorf("srf: the offset by %g needs more than %d lines each way to stay within %g", d, maxGrid, tol)
		}
		us, vs = nu, nv
	}
}

// refine is the lines with each interval marked split halved.
func refine(lines []float64, split []bool) []float64 {
	out := []float64{lines[0]}
	for i := 0; i+1 < len(lines); i++ {
		if split[i] {
			out = append(out, (lines[i]+lines[i+1])/2)
		}
		out = append(out, lines[i+1])
	}
	return out
}

// offsetThrough is the cubic B-spline surface through the offset's
// points at the grid's lines, over the patch's parameters.
func (s *SSurface) offsetThrough(us, vs []float64, d float64) (*BSplineSurface, error) {
	// Along u first: each row of points at one v a curve; the rows
	// share their knots, the grid's.
	cols := make([][]geom.Vector4, len(vs))
	for j, v := range vs {
		cols[j] = make([]geom.Vector4, len(us))
		for i, u := range us {
			p, err := s.offsetAtSafe(u, v, d)
			if err != nil {
				return nil, err
			}
			cols[j][i] = homogeneous(p, 1)
		}
	}
	degU, knotsU, err := interpolate(us, cols)
	if err != nil {
		return nil, err
	}
	// Then across: each control point's column through the rows.
	across := make([][]geom.Vector4, len(us))
	for i := range us {
		across[i] = make([]geom.Vector4, len(vs))
		for j := range vs {
			across[i][j] = cols[j][i]
		}
	}
	degV, knotsV, err := interpolate(vs, across)
	if err != nil {
		return nil, err
	}
	out := &BSplineSurface{DegU: degU, DegV: degV, KnotsU: knotsU, KnotsV: knotsV}
	for i := range across {
		row := make([]geom.Vector, len(vs))
		w := make([]float64, len(vs))
		for j, q := range across[i] {
			row[j], w[j] = q.PerspectiveProject(), 1
		}
		out.Ctrl = append(out.Ctrl, row)
		out.Weight = append(out.Weight, w)
	}
	return out, out.Check()
}

package srf

import "github.com/telesapr/telesapr/pkg/geom"

// A Bezier's polynomial over another interval of its parameter is a
// Bezier too: control point i over [t0, t1] is the blossom of the
// original with t0 n-i times and t1 i times (de Casteljau's algorithm
// with a parameter of its own at each level). The interval may reach
// past [0, 1], which extends the curve or patch as its polynomial does,
// as a face that runs to the edge of its surface needs its patches to
// reach a little past it.

// blossom is the blossom of the homogeneous points pts at us, one
// parameter for each degree.
func blossom(pts []geom.Vector4, us []float64) geom.Vector4 {
	w := append([]geom.Vector4(nil), pts...)
	for k, u := range us {
		for i := 0; i < len(w)-1-k; i++ {
			w[i] = w[i].ScaledBy(1 - u).Plus(w[i+1].ScaledBy(u))
		}
	}
	return w[0]
}

// subPoints are homogeneous points pts over [t0, t1].
func subPoints(pts []geom.Vector4, t0, t1 float64) []geom.Vector4 {
	n := len(pts) - 1
	out := make([]geom.Vector4, n+1)
	us := make([]float64, n)
	for i := 0; i <= n; i++ {
		for k := range us {
			if k < n-i {
				us[k] = t0
			} else {
				us[k] = t1
			}
		}
		out[i] = blossom(pts, us)
	}
	return out
}

// Sub is the curve's polynomial over [t0, t1], run from 0 to 1: the
// piece between them, or reaching past its ends.
func (b *SBezier) Sub(t0, t1 float64) SBezier {
	pts := make([]geom.Vector4, b.Degree+1)
	for i := range pts {
		pts[i] = geom.Vector4FromWeighted(b.Weight[i], b.Ctrl[i])
	}
	out := *b
	for i, q := range subPoints(pts, t0, t1) {
		out.Ctrl[i], out.Weight[i] = q.PerspectiveProject(), q.W
	}
	return out
}

// Sub is the patch's polynomial over [u0, u1] by [v0, v1], run from 0
// to 1 each way. Its trims are dropped.
func (s *SSurface) Sub(u0, u1, v0, v1 float64) SSurface {
	out := SSurface{DegM: s.DegM, DegN: s.DegN, Color: s.Color, Face: s.Face, Origin: s.Origin, Sheet: s.Sheet}
	var h [MaxDegree + 1][MaxDegree + 1]geom.Vector4
	for i := 0; i <= s.DegM; i++ {
		row := make([]geom.Vector4, s.DegN+1)
		for j := range row {
			row[j] = geom.Vector4FromWeighted(s.Weight[i][j], s.Ctrl[i][j])
		}
		for j, q := range subPoints(row, v0, v1) {
			h[i][j] = q
		}
	}
	for j := 0; j <= s.DegN; j++ {
		col := make([]geom.Vector4, s.DegM+1)
		for i := range col {
			col[i] = h[i][j]
		}
		for i, q := range subPoints(col, u0, u1) {
			out.Ctrl[i][j], out.Weight[i][j] = q.PerspectiveProject(), q.W
		}
	}
	return out
}

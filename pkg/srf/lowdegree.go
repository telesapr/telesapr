package srf

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Degree reduction by approximation, for STEP files whose curves and
// surfaces are of a degree above MaxDegree (package 27.2): each span is
// replaced by Bezier pieces of degree MaxDegree that interpolate it, in
// homogeneous coordinates, at Chebyshev-Lobatto points, its ends among
// them, so that neighbouring pieces meet exactly; a piece that strays
// from the span by more than the tolerance is halved until none does.

// maxLowDegreeSpans bounds how finely a curve or surface is cut along
// one way before LowDegree gives up.
const maxLowDegreeSpans = 4096

// lobatto are the d+1 Chebyshev-Lobatto points of [0, 1], 0 and 1
// included.
func lobatto(d int) []float64 {
	out := make([]float64, d+1)
	for k := range out {
		out[k] = (1 - math.Cos(float64(k)*math.Pi/float64(d))) / 2
	}
	out[0], out[d] = 0, 1
	return out
}

// bernsteinInverse is the inverse of the matrix M[k][i] = B_i^d(t_k):
// the Bezier points of degree d through values at the points t.
func bernsteinInverse(d int, t []float64) [][]float64 {
	n := d + 1
	a := make([][]float64, n)
	for k := range a {
		a[k] = make([]float64, 2*n)
		for i := 0; i < n; i++ {
			a[k][i] = binomialF(d, i) * math.Pow(t[k], float64(i)) * math.Pow(1-t[k], float64(d-i))
		}
		a[k][n+k] = 1
	}
	for c := 0; c < n; c++ {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		a[c], a[p] = a[p], a[c]
		f := a[c][c]
		for j := range a[c] {
			a[c][j] /= f
		}
		for r := 0; r < n; r++ {
			if r == c || a[r][c] == 0 {
				continue
			}
			g := a[r][c]
			for j := range a[r] {
				a[r][j] -= g * a[c][j]
			}
		}
	}
	out := make([][]float64, n)
	for k := range out {
		out[k] = a[k][n:]
	}
	return out
}

func binomialF(n, k int) float64 {
	r := 1.0
	for i := 1; i <= k; i++ {
		r = r * float64(n-k+i) / float64(i)
	}
	return r
}

// bezierH is a homogeneous Bezier's point at t, by de Casteljau.
func bezierH(pts []geom.Vector4, t float64) geom.Vector4 {
	tmp := append([]geom.Vector4(nil), pts...)
	for r := 1; r < len(tmp); r++ {
		for i := 0; i < len(tmp)-r; i++ {
			tmp[i] = tmp[i].ScaledBy(1 - t).Plus(tmp[i+1].ScaledBy(t))
		}
	}
	return tmp[0]
}

// curveH is the curve's homogeneous point at t.
func (c *BSplineCurve) curveH(t float64) geom.Vector4 {
	span := findSpan(c.Deg, c.Knots, t)
	n := make([]float64, c.Deg+1)
	basisFuns(span, t, c.Deg, c.Knots, n)
	var h geom.Vector4
	for k := 0; k <= c.Deg; k++ {
		i := span - c.Deg + k
		h = h.Plus(homogeneous(c.Ctrl[i], c.Weight[i]).ScaledBy(n[k]))
	}
	return h
}

// surfaceH is the surface's homogeneous point at (u, v).
func (s *BSplineSurface) surfaceH(u, v float64) geom.Vector4 {
	su := findSpan(s.DegU, s.KnotsU, u)
	sv := findSpan(s.DegV, s.KnotsV, v)
	nu := make([]float64, s.DegU+1)
	nv := make([]float64, s.DegV+1)
	basisFuns(su, u, s.DegU, s.KnotsU, nu)
	basisFuns(sv, v, s.DegV, s.KnotsV, nv)
	var h geom.Vector4
	for a := 0; a <= s.DegU; a++ {
		i := su - s.DegU + a
		for b := 0; b <= s.DegV; b++ {
			j := sv - s.DegV + b
			h = h.Plus(homogeneous(s.Ctrl[i][j], s.Weight[i][j]).ScaledBy(nu[a] * nv[b]))
		}
	}
	return h
}

// fullKnotVector is a clamped knot vector of degree d with every break
// inside d times: Bezier pieces joined end to end.
func fullKnotVector(d int, breaks []float64) []float64 {
	var k []float64
	for i := 0; i <= d; i++ {
		k = append(k, breaks[0])
	}
	for _, b := range breaks[1 : len(breaks)-1] {
		for i := 0; i < d; i++ {
			k = append(k, b)
		}
	}
	for i := 0; i <= d; i++ {
		k = append(k, breaks[len(breaks)-1])
	}
	return k
}

// halved are the breaks with the spans marked split at their middles.
func halved(br []float64, split []bool) []float64 {
	var out []float64
	for i := 0; i+1 < len(br); i++ {
		out = append(out, br[i])
		if split[i] {
			out = append(out, (br[i]+br[i+1])/2)
		}
	}
	return append(out, br[len(br)-1])
}

// LowDegree is the curve, or one within tol of it of degree MaxDegree
// made of Bezier pieces, when its degree is above MaxDegree.
func (c *BSplineCurve) LowDegree(tol float64) (*BSplineCurve, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	if c.Deg <= MaxDegree {
		return c, nil
	}
	d := MaxDegree
	nodes := lobatto(d)
	inv := bernsteinInverse(d, nodes)
	br := c.Breaks()
	for {
		var pieces [][]geom.Vector4
		split := make([]bool, len(br)-1)
		any := false
		for s := 0; s+1 < len(br); s++ {
			a, b := br[s], br[s+1]
			vals := make([]geom.Vector4, d+1)
			for k, t := range nodes {
				vals[k] = c.curveH(a + (b-a)*t)
			}
			pts := make([]geom.Vector4, d+1)
			for i := range pts {
				for k := range vals {
					pts[i] = pts[i].Plus(vals[k].ScaledBy(inv[i][k]))
				}
			}
			pieces = append(pieces, pts)
			if !pieceWithin(pts, func(t float64) geom.Vector4 { return c.curveH(a + (b-a)*t) }, tol) {
				split[s], any = true, true
			}
		}
		if !any {
			out := &BSplineCurve{Deg: d, Knots: fullKnotVector(d, br)}
			for s, pts := range pieces {
				for i, q := range pts {
					if s > 0 && i == 0 {
						continue
					}
					out.Ctrl = append(out.Ctrl, q.PerspectiveProject())
					out.Weight = append(out.Weight, q.W)
				}
			}
			return out, out.Check()
		}
		br = halved(br, split)
		if len(br) > maxLowDegreeSpans {
			return nil, errors.New("srf: a curve of high degree that no pieces of degree 5 follow")
		}
	}
}

// pieceWithin reports whether a homogeneous Bezier lies within tol of
// the curve f over [0, 1], its weights all above zero.
func pieceWithin(pts []geom.Vector4, f func(t float64) geom.Vector4, tol float64) bool {
	for _, q := range pts {
		if !(q.W > 0) {
			return false
		}
	}
	const samples = 24
	for k := 0; k <= samples; k++ {
		t := float64(k) / samples
		if bezierH(pts, t).PerspectiveProject().Minus(f(t).PerspectiveProject()).Magnitude() > tol {
			return false
		}
	}
	return true
}

// LowDegree is the surface, or one within tol of it of degree at most
// MaxDegree each way, made of Bezier patches, when a degree is above
// MaxDegree. Along a way whose degree is not, the patches keep it and
// its spans, and match the surface exactly.
func (s *BSplineSurface) LowDegree(tol float64) (*BSplineSurface, error) {
	if err := s.Check(); err != nil {
		return nil, err
	}
	if s.DegU <= MaxDegree && s.DegV <= MaxDegree {
		return s, nil
	}
	du, dv := min(s.DegU, MaxDegree), min(s.DegV, MaxDegree)
	nu, nv := lobatto(du), lobatto(dv)
	invU, invV := bernsteinInverse(du, nu), bernsteinInverse(dv, nv)
	bu, bv := s.Breaks()
	for {
		// cells[a][b][i][j]: patch a, b's homogeneous points.
		cells := make([][][][]geom.Vector4, len(bu)-1)
		splitU := make([]bool, len(bu)-1)
		splitV := make([]bool, len(bv)-1)
		any := false
		for a := 0; a+1 < len(bu); a++ {
			cells[a] = make([][][]geom.Vector4, len(bv)-1)
			for b := 0; b+1 < len(bv); b++ {
				u0, u1, v0, v1 := bu[a], bu[a+1], bv[b], bv[b+1]
				f := func(x, y float64) geom.Vector4 { return s.surfaceH(u0+(u1-u0)*x, v0+(v1-v0)*y) }
				vals := make([][]geom.Vector4, du+1)
				for i, x := range nu {
					vals[i] = make([]geom.Vector4, dv+1)
					for j, y := range nv {
						vals[i][j] = f(x, y)
					}
				}
				// Along v, then along u.
				tmp := make([][]geom.Vector4, du+1)
				for i := range tmp {
					tmp[i] = make([]geom.Vector4, dv+1)
					for j := range tmp[i] {
						for k := range nv {
							tmp[i][j] = tmp[i][j].Plus(vals[i][k].ScaledBy(invV[j][k]))
						}
					}
				}
				pts := make([][]geom.Vector4, du+1)
				for i := range pts {
					pts[i] = make([]geom.Vector4, dv+1)
					for j := range pts[i] {
						for k := range nu {
							pts[i][j] = pts[i][j].Plus(tmp[k][j].ScaledBy(invU[i][k]))
						}
					}
				}
				cells[a][b] = pts
				if !patchWithin(pts, f, tol) {
					any = true
					if s.DegU > MaxDegree {
						splitU[a] = true
					}
					if s.DegV > MaxDegree {
						splitV[b] = true
					}
				}
			}
		}
		if !any {
			out := &BSplineSurface{DegU: du, DegV: dv, KnotsU: fullKnotVector(du, bu), KnotsV: fullKnotVector(dv, bv)}
			rows := (len(bu)-1)*du + 1
			cols := (len(bv)-1)*dv + 1
			out.Ctrl = make([][]geom.Vector, rows)
			out.Weight = make([][]float64, rows)
			for r := range out.Ctrl {
				out.Ctrl[r] = make([]geom.Vector, cols)
				out.Weight[r] = make([]float64, cols)
			}
			for a := range cells {
				for b := range cells[a] {
					for i := 0; i <= du; i++ {
						for j := 0; j <= dv; j++ {
							q := cells[a][b][i][j]
							out.Ctrl[a*du+i][b*dv+j] = q.PerspectiveProject()
							out.Weight[a*du+i][b*dv+j] = q.W
						}
					}
				}
			}
			return out, out.Check()
		}
		bu, bv = halved(bu, splitU), halved(bv, splitV)
		if len(bu) > maxLowDegreeSpans || len(bv) > maxLowDegreeSpans || len(bu)*len(bv) > 1<<16 {
			return nil, errors.New("srf: a surface of high degree that no patches of degree 5 follow")
		}
	}
}

// patchWithin reports whether a homogeneous Bezier patch lies within
// tol of the surface f over [0, 1] by [0, 1], its weights above zero.
func patchWithin(pts [][]geom.Vector4, f func(x, y float64) geom.Vector4, tol float64) bool {
	for i := range pts {
		for _, q := range pts[i] {
			if !(q.W > 0) {
				return false
			}
		}
	}
	const samples = 12
	col := make([]geom.Vector4, len(pts))
	for k := 0; k <= samples; k++ {
		y := float64(k) / samples
		for i := range pts {
			col[i] = bezierH(pts[i], y)
		}
		for m := 0; m <= samples; m++ {
			x := float64(m) / samples
			if bezierH(col, x).PerspectiveProject().Minus(f(x, y).PerspectiveProject()).Magnitude() > tol {
				return false
			}
		}
	}
	return true
}

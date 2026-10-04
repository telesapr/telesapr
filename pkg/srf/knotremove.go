package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Knot removal (Piegl and Tiller, A5.8), for STEP export (package
// 27.1): a face of several patches is written as one B-spline with a
// knot of full multiplicity between each two patches; where the patches
// join smoothly, the knot is taken out as many times as the surface
// lets it, so that readers see the surface as smooth as it is.

func len4(v geom.Vector4) float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z + v.W*v.W)
}

// removeKnotOnce takes knots[r], the last of its s copies, out of a
// curve of degree deg over homogeneous points pts once, when the curve
// moves no more than tol, homogeneous, for it; ok reports whether it
// did. knots and pts are not changed.
func removeKnotOnce(deg int, knots []float64, pts []geom.Vector4, r, s int, tol float64) (nk []float64, np []geom.Vector4, ok bool) {
	u := knots[r]
	ord := deg + 1
	first, last := r-deg, r-s
	if first < 1 || last+1 >= len(pts) {
		return nil, nil, false
	}
	off := first - 1
	temp := make([]geom.Vector4, last-off+2)
	temp[0] = pts[off]
	temp[last+1-off] = pts[last+1]
	i, j := first, last
	ii, jj := 1, last-off
	for j-i > 0 {
		di, dj := knots[i+ord]-knots[i], knots[j+ord]-knots[j]
		if di == 0 || dj == 0 {
			return nil, nil, false
		}
		ai := (u - knots[i]) / di
		aj := (u - knots[j]) / dj
		if ai == 0 || aj == 1 {
			return nil, nil, false
		}
		temp[ii] = pts[i].Minus(temp[ii-1].ScaledBy(1 - ai)).ScaledBy(1 / ai)
		temp[jj] = pts[j].Minus(temp[jj+1].ScaledBy(aj)).ScaledBy(1 / (1 - aj))
		i++
		ii++
		j--
		jj--
	}
	if j-i < 0 {
		ok = len4(temp[ii-1].Minus(temp[jj+1])) <= tol
	} else {
		di := knots[i+ord] - knots[i]
		if di == 0 {
			return nil, nil, false
		}
		ai := (u - knots[i]) / di
		ok = len4(pts[i].Minus(temp[ii+1].ScaledBy(ai).Plus(temp[ii-1].ScaledBy(1-ai)))) <= tol
	}
	if !ok {
		return nil, nil, false
	}
	np = append([]geom.Vector4(nil), pts...)
	i, j = first, last
	for j-i > 0 {
		np[i] = temp[i-off]
		np[j] = temp[j-off]
		i++
		j--
	}
	// The point the removal leaves over.
	fout := (2*r - s - deg) / 2
	np = append(np[:fout], np[fout+1:]...)
	nk = append(append([]float64(nil), knots[:r]...), knots[r+1:]...)
	return nk, np, true
}

// removalTol is the homogeneous distance that keeps a removal within
// d of the curve, for points whose weights are at least wmin and whose
// distances from the origin at most pmax (Piegl and Tiller, eq. 5.30).
func removalTol(d, wmin, pmax float64) float64 {
	return d * wmin / (1 + pmax)
}

// RemoveKnots takes out of the surface's knots inside its domain every
// copy it can without moving the surface more than tol, along u and
// then along v, and reports how many it took. A knot goes out of a
// surface only when it goes out of every row of control points at once.
func (s *BSplineSurface) RemoveKnots(tol float64) int {
	wmin, pmax := math.Inf(1), 0.0
	for i := range s.Ctrl {
		for j := range s.Ctrl[i] {
			wmin = min(wmin, s.Weight[i][j])
			pmax = max(pmax, s.Ctrl[i][j].Magnitude())
		}
	}
	h := removalTol(tol, wmin, pmax)
	// net[i][j] homogeneous, i along u.
	net := make([][]geom.Vector4, len(s.Ctrl))
	for i := range s.Ctrl {
		net[i] = make([]geom.Vector4, len(s.Ctrl[i]))
		for j := range s.Ctrl[i] {
			net[i][j] = homogeneous(s.Ctrl[i][j], s.Weight[i][j])
		}
	}
	removed := 0
	var n int
	s.KnotsU, net, n = removeAlong(s.DegU, s.KnotsU, net, h)
	removed += n
	tr := transpose4(net)
	s.KnotsV, tr, n = removeAlong(s.DegV, s.KnotsV, tr, h)
	removed += n
	net = transpose4(tr)
	s.Ctrl = make([][]geom.Vector, len(net))
	s.Weight = make([][]float64, len(net))
	for i := range net {
		s.Ctrl[i] = make([]geom.Vector, len(net[i]))
		s.Weight[i] = make([]float64, len(net[i]))
		for j, q := range net[i] {
			s.Ctrl[i][j], s.Weight[i][j] = q.PerspectiveProject(), q.W
		}
	}
	return removed
}

func transpose4(m [][]geom.Vector4) [][]geom.Vector4 {
	if len(m) == 0 {
		return nil
	}
	out := make([][]geom.Vector4, len(m[0]))
	for j := range out {
		out[j] = make([]geom.Vector4, len(m))
		for i := range m {
			out[j][i] = m[i][j]
		}
	}
	return out
}

// removeAlong removes the inner knots of the first index of net, the
// curves net[.][j] all at once, as often as each goes.
func removeAlong(deg int, knots []float64, net [][]geom.Vector4, h float64) ([]float64, [][]geom.Vector4, int) {
	removed := 0
	lo, hi := knots[deg], knots[len(knots)-1-deg]
	for r := deg + 1; r < len(knots)-deg-1; r++ {
		u := knots[r]
		if u <= lo || u >= hi || knots[r+1] == u {
			continue
		}
		// r is the last copy of u.
		for {
			s := 0
			for k := r; k >= 0 && knots[k] == u; k-- {
				s++
			}
			if s == 0 {
				break
			}
			cols := len(net[0])
			nk := knots
			next := make([][]geom.Vector4, cols)
			all := true
			for j := 0; j < cols && all; j++ {
				col := make([]geom.Vector4, len(net))
				for i := range net {
					col[i] = net[i][j]
				}
				var ok bool
				var k2 []float64
				k2, next[j], ok = removeKnotOnce(deg, knots, col, r, s, h)
				if !ok {
					all = false
					break
				}
				nk = k2
			}
			if !all {
				break
			}
			rows := len(net) - 1
			out := make([][]geom.Vector4, rows)
			for i := range out {
				out[i] = make([]geom.Vector4, cols)
				for j := range out[i] {
					out[i][j] = next[j][i]
				}
			}
			knots, net = nk, out
			removed++
			r--
			if knots[r] != u {
				// Every copy gone: the next knot is at r+1 now.
				break
			}
		}
	}
	return knots, net, removed
}

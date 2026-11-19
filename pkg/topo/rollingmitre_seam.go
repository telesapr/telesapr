package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A mitre at an inside corner crossing a seam of the face both chains
// share (rollingmitre_inside.go): where a pocket's corner stands near
// the seam between two patches of a curved face, as a lathe's quarters,
// and the two fillets meet in a mitre (two radii, or two feet at one
// depth), the mitre's point on the shared face lies on one patch while
// one chain's edge runs wholly along the other: that chain's foot
// crosses the seam a little before its end, onto a patch it was never
// looked for on, and its band was cut into the patch along its edge up
// to the mitre, past the patch's side. The seam, from the rim inside
// the bands, leaves them across that foot: the foot is cut where it
// crosses, its piece on to the mitre moved to the next patch, and the
// seam's piece back to the rim dropped, as a corner patch's side is
// carried across (rollingpivot_seam.go).

// seamParam is, for a seam of patch s along its points pts, how far past
// the seam a point lies in s's parameter across it, below 0 on s; false
// where the seam is not a side of s. The side is the one every point of
// the seam lies on: a seam's end may stand at a corner of the patch, as
// a straight seam of two points ending at a tube's end, where both
// parameters sit on a side.
func seamParam(s *srf.SSurface, pts []geom.Vector) (func(geom.Vector) float64, bool) {
	var on [2][2]int
	for _, p := range pts {
		mu, mv, _ := nearestOnWithin(s, p, 0.5)
		for i, x := range []float64{mu, mv} {
			for j, b := range []float64{0, 1} {
				if math.Abs(x-b) < 1e-6 {
					on[i][j]++
				}
			}
		}
	}
	k, side := -1, 0.0
	for i := range 2 {
		for j := range 2 {
			if on[i][j] == len(pts) {
				if k >= 0 {
					return nil, false
				}
				k, side = i, float64(j)
			}
		}
	}
	if k < 0 {
		return nil, false
	}
	return func(p geom.Vector) float64 {
		u, v, _ := nearestOnWithin(s, p, 0.5)
		x := []float64{u, v}[k]
		if side == 0 {
			return -x
		}
		return x - 1
	}, true
}

// seamBetween is the seam between faces f and g nearest at that ends at
// one of the vertices onRim, and the vertex; -1 where none does.
func (t *Topology) seamBetween(f, g int, onRim map[int]bool, at geom.Vector) (int, int) {
	best, end, near := -1, -1, math.Inf(1)
	for y := range t.Edges {
		fs := t.EdgeFaces(y)
		if !(fs[0] == f && fs[1] == g || fs[0] == g && fs[1] == f) {
			continue
		}
		for _, v := range t.Edges[y].V {
			if !onRim[v] {
				continue
			}
			if d := t.Vertices[v].P.Minus(at).Magnitude(); d < near {
				best, end, near = y, v, d
			}
		}
	}
	return best, end
}

// insidePatch reports whether p lies on surface s within its patch.
func insidePatch(s *srf.SSurface, p geom.Vector) bool {
	u, v, _ := nearestOnWithin(s, p, 0.5)
	const eps = 1e-9
	return u > -eps && u < 1+eps && v > -eps && v < 1+eps
}

// mitreSeams carries across a seam each inside mitre's foot that crosses
// one onto the mate's patch of the shared face, after the faces are cut
// back, and gives the points where the feet were cut, at which the
// fillets' patches are split.
func mitreSeams(sh *srf.SShell, bls []*rbBlend) ([]geom.Vector, error) {
	var out []geom.Vector
	for _, bl := range bls {
		rb := bl.rb
		if rb.closed {
			continue
		}
		for k, end := range rb.ends {
			if end == nil || !end.mitre || !end.concave || end.mate == nil || end.mate == rb {
				continue
			}
			c, ok, err := bl.mitreSeam(sh, bls, k)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

// mitreSeam carries end k's foot across the seam (mitreSeams), and gives
// where it was cut, if it crosses one.
func (bl *rbBlend) mitreSeam(sh *srf.SShell, bls []*rbBlend, k int) (geom.Vector, bool, error) {
	rb, t := bl.rb, bl.rb.t
	end := rb.ends[k]
	i := 1 - rb.creaseSide(k)
	if i > 1 {
		return geom.Vector{}, false, nil
	}
	sec := bl.endSec(k)
	m, f1 := sec.foot[i].p, sec.foot[i].f
	s1 := t.Surface(f1)
	if s1 == nil || insidePatch(s1, m) {
		return geom.Vector{}, false, nil
	}
	vp := t.Vertices[end.v].P
	refuse := func(why string) (geom.Vector, bool, error) {
		return geom.Vector{}, false, filletErr(m, "the fillets' mitre at the inside corner crosses a seam of the face they share: %s", why)
	}
	var mate *rbBlend
	for _, o := range bls {
		if o.rb == end.mate {
			mate = o
		}
	}
	if mate == nil {
		return refuse("its mate is not being made")
	}
	mi := 1 - mate.rb.creaseSide(end.mateK)
	if mi > 1 {
		return refuse("its mate has no crease")
	}
	f2 := mate.endSec(end.mateK).foot[mi].f
	s2 := t.Surface(f2)
	if f2 == f1 || s2 == nil || !insidePatch(s2, m) {
		return refuse("make them smaller")
	}
	onRim := map[int]bool{}
	for _, x := range []*rollingBall{rb, mate.rb} {
		for _, v := range x.verts {
			onRim[v] = true
		}
	}
	edge, sv := t.seamBetween(f1, f2, onRim, vp)
	if edge < 0 {
		return refuse("the seam doesn't reach the rim near it: make them smaller")
	}
	past, ok := seamParam(s1, t.Edges[edge].Points)
	if !ok {
		return refuse("the seam is not a side of the face's patch")
	}
	if sv == end.v && math.Abs(past(m)) < 1e-6 {
		// A seam leaving the vertex under the mitre through its point, as
		// at a corner the seam halves: split there before
		// (splitMitreSeams), each chain dropping its piece back to the
		// vertex.
		return geom.Vector{}, false, nil
	}
	sF1, sF2 := sh.FindSurface(t.Faces[f1].Surface), sh.FindSurface(t.Faces[f2].Surface)
	if sF1 == nil || sF2 == nil {
		return refuse("a patch of it is not in the shell")
	}
	path, near, c, err := footToSeam(sh, sF1, m, past)
	if err != nil {
		return refuse(err.Error())
	}
	last := path[len(path)-1]
	if err := splitCurve(sh, last.Curve, c); err != nil {
		return refuse("its foot can't be cut at the seam")
	}
	// The trims from the mitre to the cut, the last now two, go to the
	// next patch.
	moved := path[:len(path)-1]
	piece := slices.IndexFunc(sF1.Trim, func(tb srf.STrimBy) bool {
		return tb.StartP.Equals(near) && tb.FinishP.Equals(c) || tb.StartP.Equals(c) && tb.FinishP.Equals(near)
	})
	if piece < 0 {
		return refuse("its foot doesn't come to the cut")
	}
	moved = append(moved, sF1.Trim[piece])
	var kept []srf.STrimBy
	for _, tb := range sF1.Trim {
		if !trimIn(tb, moved) {
			kept = append(kept, tb)
		}
	}
	sF1.Trim = kept
	for _, tb := range moved {
		sF2.Trim = append(sF2.Trim, tb)
		if cv := sh.FindCurve(tb.Curve); cv != nil {
			cv.SurfA = sF2.H
		}
	}
	sEnd := t.Vertices[sv].P
	if err := splitNearest(sh, c, sEnd); err != nil {
		return refuse("its foot doesn't meet the seam: make them smaller")
	}
	for _, s := range []*srf.SSurface{sF1, sF2} {
		kept = kept[:0:0]
		for _, tb := range s.Trim {
			if tb.StartP.Equals(c) && tb.FinishP.Equals(sEnd) || tb.StartP.Equals(sEnd) && tb.FinishP.Equals(c) {
				continue
			}
			kept = append(kept, tb)
		}
		s.Trim = kept
		forget(s)
	}
	return c, true, nil
}

// trimIn reports whether tb is among trims.
func trimIn(tb srf.STrimBy, trims []srf.STrimBy) bool {
	for _, x := range trims {
		if x.Curve == tb.Curve && x.StartP.Equals(tb.StartP) && x.FinishP.Equals(tb.FinishP) {
			return true
		}
	}
	return false
}

// footToSeam walks s's trims from m, past the seam (past above 0), along
// the foot back to the trim it crosses the seam on: the trims walked,
// the last one's end on m's side, and the point on it where it crosses.
func footToSeam(sh *srf.SShell, s *srf.SSurface, m geom.Vector, past func(geom.Vector) float64) ([]srf.STrimBy, geom.Vector, geom.Vector, error) {
	var path []srf.STrimBy
	cur := m
	used := map[int]bool{}
	for range 64 {
		next := -1
		for j, tb := range s.Trim {
			if !used[j] && (tb.StartP.Equals(cur) || tb.FinishP.Equals(cur)) {
				next = j
				break
			}
		}
		if next < 0 {
			return nil, geom.Vector{}, geom.Vector{}, errSeamFoot("its foot doesn't reach the seam")
		}
		used[next] = true
		tb := s.Trim[next]
		path = append(path, tb)
		other := tb.FinishP
		if tb.FinishP.Equals(cur) {
			other = tb.StartP
		}
		if x := past(other); x < 0 {
			cv := sh.FindCurve(tb.Curve)
			if cv == nil || !cv.IsExact {
				return nil, geom.Vector{}, geom.Vector{}, errSeamFoot("its foot is not an exact curve")
			}
			sb := cv.Exact
			lo, hi := 0.0, 1.0
			if past(sb.Start()) < 0 {
				lo, hi = 1, 0
			}
			// lo on the far side of the seam, hi on this one.
			for range 60 {
				mid := (lo + hi) / 2
				if past(sb.PointAt(mid)) < 0 {
					hi = mid
				} else {
					lo = mid
				}
			}
			return path, cur, sb.PointAt((lo + hi) / 2), nil
		}
		cur = other
	}
	return nil, geom.Vector{}, geom.Vector{}, errSeamFoot("its foot runs on too long past the seam")
}

// errSeamFoot is why a mitre's foot can't be carried across a seam.
type errSeamFoot string

func (e errSeamFoot) Error() string { return string(e) }

// splitAtSeams splits the sides of the fillets' patches at the points
// where their feet were cut at a seam (mitreSeams).
func splitAtSeams(bls []*rbBlend, pts []geom.Vector, loops [][][][]srf.SBezier) {
	if len(pts) == 0 {
		return
	}
	for i := range bls {
		for r := range loops[i] {
			for b := range loops[i][r] {
				var out []srf.SBezier
				for _, c := range loops[i][r][b] {
					out = append(out, splitAtPoints(c, pts)...)
				}
				loops[i][r][b] = out
			}
		}
	}
}

package srf

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/par"
)

// Routines to merge multiple coincident surfaces (each with their own trim
// curves) into a single surface, with all of the trim curves. Port of
// solvespace/src/srf/merge.cpp.

// MergeCoincidentSurfaces merges coincident, same-color planes that
// share an edge into one surface carrying all of their trims.
//
// The planes are first put in classes, each the planes coincident with
// its first and of its colour, in the surfaces' order; a surface only
// ever merges with others of its class, so the classes are merged at
// once, each in order (roadmap 17.2). The curves that named a merged
// surface name the one it went into afterwards: making the edges of a
// surface reads its trims and the curves' points, never which surfaces
// a curve lies on, so the result is that of the serial loop.
func (s *SShell) MergeCoincidentSurfaces() {
	for i := range s.Surface {
		s.Surface[i].Tag = 0
	}
	curveAt := make(map[core.HSCurve]*SCurve, len(s.Curve))
	for i := range s.Curve {
		if _, dup := curveAt[s.Curve[i].H]; !dup {
			curveAt[s.Curve[i].H] = &s.Curve[i]
		}
	}

	// The classes, by index, each in order.
	var classes [][]int
	classed := make([]bool, len(s.Surface))
	for i := range s.Surface {
		si := &s.Surface[i]
		// Let someone else clean up the empty surfaces; we can certainly
		// merge them, but we don't know how to calculate a reasonable
		// bounding box. And for now we handle only coincident planes.
		if classed[i] || len(si.Trim) == 0 || si.DegM != 1 || si.DegN != 1 {
			continue
		}
		class := []int{i}
		classed[i] = true
		for j := i + 1; j < len(s.Surface); j++ {
			sj := &s.Surface[j]
			if !classed[j] && sj.CoincidentWith(si, true) && sj.Color.Equals(si.Color) {
				class = append(class, j)
				classed[j] = true
			}
		}
		if len(class) > 1 {
			classes = append(classes, class)
		}
	}

	replaced := make([][]surfaceReplaced, len(classes))
	par.For(len(classes), func(c int) {
		replaced[c] = s.mergeClass(classes[c], curveAt)
	})
	to := map[core.HSSurface]core.HSSurface{}
	for _, rs := range replaced {
		for _, r := range rs {
			to[r.from] = r.to
		}
	}
	if len(to) > 0 {
		for k := range s.Curve {
			sc := &s.Curve[k]
			if h, ok := to[sc.SurfA]; ok {
				sc.SurfA = h
			}
			if h, ok := to[sc.SurfB]; ok {
				sc.SurfB = h
			}
		}
	}

	kept := s.Surface[:0]
	for _, ss := range s.Surface {
		if ss.Tag == 0 {
			kept = append(kept, ss)
		}
	}
	s.Surface = kept
}

// surfaceReplaced is a surface merged into another.
type surfaceReplaced struct{ from, to core.HSSurface }

// edgesInto is MakeEdgesInto with the shell's curves by handle.
func (s *SSurface) edgesInto(curveAt map[core.HSCurve]*SCurve, sel *SEdgeList) {
	for i := range s.Trim {
		stb := &s.Trim[i]
		if sc := curveAt[stb.Curve]; sc != nil {
			s.makeTrimEdgesInto(sel, false, sc, stb)
		}
	}
}

// mergeClass merges the coincident planes of one class, the indices of
// its surfaces in order, as the serial loop does, and returns which
// surface went into which.
func (s *SShell) mergeClass(class []int, curveAt map[core.HSCurve]*SCurve) (replaced []surfaceReplaced) {
	for ci, i := range class {
		si := &s.Surface[i]
		if si.Tag != 0 {
			continue
		}

		var sel SEdgeList
		si.edgesInto(curveAt, &sel)

		merged := false
		for {
			mergedThisTime := false

			for _, j := range class[ci+1:] {
				sj := &s.Surface[j]
				if sj.Tag != 0 {
					continue
				}
				// But we do merge surfaces with different face entities,
				// since otherwise we'd hardly ever merge anything.

				// This surface is coincident. But let's not merge
				// coincident surfaces if they contain disjoint contours;
				// that just makes the bounding box tests less effective,
				// and possibly things less robust.
				var tel SEdgeList
				sj.edgesInto(curveAt, &tel)
				if !sel.ContainsEdgeFrom(&tel) {
					continue
				}

				sj.Tag = 1
				merged = true
				mergedThisTime = true
				sj.edgesInto(curveAt, &sel)
				sj.Trim = nil

				// All the references to this surface get replaced with
				// the new srf, once every class is merged.
				replaced = append(replaced, surfaceReplaced{sj.H, si.H})
			}

			// If this iteration merged a contour onto ours, then we have
			// to go through the surfaces again; that might have made a
			// new surface touch us.
			if !mergedThisTime {
				break
			}
		}

		if merged {
			sel.CullExtraneousEdges(true)
			si.Trim = nil
			si.trimFromEdgeList(&sel, false)

			// And we must choose control points such that all the trims lie
			// with u and v in [0, 1], so that the bbox tests work.
			var u, v geom.Vector
			si.TangentsAt(0.5, 0.5, &u, &v, true)
			u = u.WithMagnitude(1)
			n := si.NormalAt(0.5, 0.5).WithMagnitude(1)
			v = n.Cross(u).WithMagnitude(1)

			umax, umin := -geom.VeryPositive, geom.VeryPositive
			vmax, vmin := -geom.VeryPositive, geom.VeryPositive
			for _, se := range sel.Edges {
				ut, vt := se.A.Dot(u), se.A.Dot(v)
				umax = geom.Max(umax, ut)
				vmax = geom.Max(vmax, vt)
				umin = geom.Min(umin, ut)
				vmin = geom.Min(vmin, vt)
			}

			// An interesting problem here; the real curve could extend
			// slightly beyond the bounding box of the piecewise linear
			// bits. Not a problem for us, but some apps won't import STEP
			// in that case. So give a bit of extra room; in theory just
			// a chord tolerance, but more can't hurt.
			muv := geom.Max(umax-umin, vmax-vmin)
			tol := muv/50 + 3*s.Tol.ChordTolMm()
			umax += tol
			vmax += tol
			umin -= tol
			vmin -= tol

			// We move in the +v direction as v goes from 0 to 1, and in the
			// +u direction as u goes from 0 to 1. So our normal ends up
			// pointed the same direction.
			nt := si.Ctrl[0][0].Dot(n)
			si.Ctrl[0][0] = geom.Vector{X: umin, Y: vmin, Z: nt}.ScaleOutOfCsys(u, v, n)
			si.Ctrl[0][1] = geom.Vector{X: umin, Y: vmax, Z: nt}.ScaleOutOfCsys(u, v, n)
			si.Ctrl[1][1] = geom.Vector{X: umax, Y: vmax, Z: nt}.ScaleOutOfCsys(u, v, n)
			si.Ctrl[1][0] = geom.Vector{X: umax, Y: vmin, Z: nt}.ScaleOutOfCsys(u, v, n)
		}
	}
	return replaced
}

// CoincidentWith reports whether ss is a plane lying in the same plane
// as s, with the same (or, if !sameNormal, opposite) normal.
func (s *SSurface) CoincidentWith(ss *SSurface, sameNormal bool) bool {
	if s.DegM != 1 || s.DegN != 1 {
		return false
	}
	if ss.DegM != 1 || ss.DegN != 1 {
		return false
	}

	p := s.Ctrl[0][0]
	n := s.NormalAt(0, 0).WithMagnitude(1)
	d := n.Dot(p)

	if !ss.CoincidentWithPlane(n, d) {
		return false
	}

	n2 := ss.NormalAt(0, 0)
	if sameNormal {
		if n2.Dot(n) < 0 {
			return false
		}
	} else {
		if n2.Dot(n) > 0 {
			return false
		}
	}

	return true
}

// ContainsEdgeFrom reports whether any edge of el is also in sel (in
// either direction).
func (el *SEdgeList) ContainsEdgeFrom(sel *SEdgeList) bool {
	for i := range el.Edges {
		if sel.ContainsEdge(&el.Edges[i]) {
			return true
		}
	}
	return false
}

// ContainsEdge reports whether el contains set, in either direction.
func (el *SEdgeList) ContainsEdge(set *SEdge) bool {
	for i := range el.Edges {
		se := &el.Edges[i]
		if se.A.Equals(set.A) && se.B.Equals(set.B) {
			return true
		}
		if se.B.Equals(set.A) && se.A.Equals(set.B) {
			return true
		}
	}
	return false
}

// CullExtraneousEdges removes duplicate edges, and anti-parallel pairs
// (both of them if both, else one).
func (el *SEdgeList) CullExtraneousEdges(both bool) {
	for i := range el.Edges {
		el.Edges[i].Tag = 0
	}
	for i := range el.Edges {
		se := &el.Edges[i]
		for j := i + 1; j < len(el.Edges); j++ {
			set := &el.Edges[j]
			if set.A.Equals(se.A) && set.B.Equals(se.B) {
				// Two parallel edges exist; so keep only the first one.
				set.Tag = 1
			}
			if set.A.Equals(se.B) && set.B.Equals(se.A) {
				// Two anti-parallel edges exist; if both=true, keep neither,
				// otherwise keep only one.
				if both {
					se.Tag = 1
				}
				set.Tag = 1
			}
		}
	}
	kept := el.Edges[:0]
	for _, e := range el.Edges {
		if e.Tag == 0 {
			kept = append(kept, e)
		}
	}
	el.Edges = kept
}

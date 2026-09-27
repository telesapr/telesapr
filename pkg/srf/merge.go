package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// Routines to merge multiple coincident surfaces (each with their own trim
// curves) into a single surface, with all of the trim curves. Port of
// solvespace/src/srf/merge.cpp.

// MergeCoincidentSurfaces merges coincident, same-color planes that
// share an edge into one surface carrying all of their trims.
// Mirrors C++ SShell::MergeCoincidentSurfaces.
func (s *SShell) MergeCoincidentSurfaces() {
	for i := range s.Surface {
		s.Surface[i].Tag = 0
	}

	for i := range s.Surface {
		si := &s.Surface[i]
		if si.Tag != 0 {
			continue
		}
		// Let someone else clean up the empty surfaces; we can certainly merge
		// them, but we don't know how to calculate a reasonable bounding box.
		if len(si.Trim) == 0 {
			continue
		}
		// And for now we handle only coincident planes, so no sense wasting
		// time on other surfaces.
		if si.DegM != 1 || si.DegN != 1 {
			continue
		}

		var sel SEdgeList
		si.MakeEdgesInto(s, &sel, false, nil)

		merged := false
		for {
			mergedThisTime := false

			for j := i + 1; j < len(s.Surface); j++ {
				sj := &s.Surface[j]
				if sj.Tag != 0 {
					continue
				}
				if !sj.CoincidentWith(si, true) {
					continue
				}
				if !sj.Color.Equals(si.Color) {
					continue
				}
				// But we do merge surfaces with different face entities, since
				// otherwise we'd hardly ever merge anything.

				// This surface is coincident. But let's not merge coincident
				// surfaces if they contain disjoint contours; that just makes
				// the bounding box tests less effective, and possibly things
				// less robust.
				var tel SEdgeList
				sj.MakeEdgesInto(s, &tel, false, nil)
				if !sel.ContainsEdgeFrom(&tel) {
					continue
				}

				sj.Tag = 1
				merged = true
				mergedThisTime = true
				sj.MakeEdgesInto(s, &sel, false, nil)
				sj.Trim = nil

				// All the references to this surface get replaced with the
				// new srf
				for k := range s.Curve {
					sc := &s.Curve[k]
					if sc.SurfA == sj.H {
						sc.SurfA = si.H
					}
					if sc.SurfB == sj.H {
						sc.SurfB = si.H
					}
				}
			}

			// If this iteration merged a contour onto ours, then we have to
			// go through the surfaces again; that might have made a new
			// surface touch us.
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
				umax = math.Max(umax, ut)
				vmax = math.Max(vmax, vt)
				umin = math.Min(umin, ut)
				vmin = math.Min(vmin, vt)
			}

			// An interesting problem here; the real curve could extend
			// slightly beyond the bounding box of the piecewise linear
			// bits. Not a problem for us, but some apps won't import STEP
			// in that case. So give a bit of extra room; in theory just
			// a chord tolerance, but more can't hurt.
			muv := math.Max(umax-umin, vmax-vmin)
			tol := muv/50 + 3*ChordTolMm()
			umax += tol
			vmax += tol
			umin -= tol
			vmin -= tol

			// We move in the +v direction as v goes from 0 to 1, and in the
			// +u direction as u goes from 0 to 1. So our normal ends up
			// pointed the same direction.
			nt := si.Ctrl[0][0].Dot(n)
			si.Ctrl[0][0] = scaleOutOfCsys(geom.Vector{X: umin, Y: vmin, Z: nt}, u, v, n)
			si.Ctrl[0][1] = scaleOutOfCsys(geom.Vector{X: umin, Y: vmax, Z: nt}, u, v, n)
			si.Ctrl[1][1] = scaleOutOfCsys(geom.Vector{X: umax, Y: vmax, Z: nt}, u, v, n)
			si.Ctrl[1][0] = scaleOutOfCsys(geom.Vector{X: umax, Y: vmin, Z: nt}, u, v, n)
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

// CoincidentWith reports whether ss is a plane lying in the same plane
// as s, with the same (or, if !sameNormal, opposite) normal. Mirrors
// C++ SSurface::CoincidentWith.
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

	if !ss.CoincidentWithPlane(n, d, geom.LengthEps) {
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
// either direction). Mirrors C++ SEdgeList::ContainsEdgeFrom.
func (el *SEdgeList) ContainsEdgeFrom(sel *SEdgeList) bool {
	for i := range el.Edges {
		if sel.ContainsEdge(&el.Edges[i]) {
			return true
		}
	}
	return false
}

// ContainsEdge reports whether el contains set, in either direction.
// Mirrors C++ SEdgeList::ContainsEdge.
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
// (both of them if both, else one). Mirrors C++
// SEdgeList::CullExtraneousEdges.
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

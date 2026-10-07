package srf

import (
	"cmp"
	"slices"
	"sort"

	"github.com/telesapr/telesapr/pkg/geom"
)

// SnapTJunctions mends the T-junctions of a mesh made face by face:
// where one face's boundary has a point that the next face's boundary
// runs past, both edges are naked, though the surface is closed. Each
// triangle with a naked edge that has the end of another naked edge
// inside it is split there, in place, so the faces meet vertex to
// vertex; the new vertices' normals lie between their edge's. Edges
// that are not naked, and so the mesh of a closed solid, are left
// alone (roadmap 13.5).
func (m *SMesh) SnapTJunctions() {
	type edge struct{ a, b geom.Vector }
	have := make(map[edge]bool, 3*len(m.Triangles))
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		have[edge{tr.A, tr.B}] = true
		have[edge{tr.B, tr.C}] = true
		have[edge{tr.C, tr.A}] = true
	}
	var ends []geom.Vector
	seen := map[geom.Vector]bool{}
	for e := range have {
		if have[edge{e.b, e.a}] {
			continue
		}
		for _, p := range []geom.Vector{e.a, e.b} {
			if !seen[p] {
				seen[p] = true
				ends = append(ends, p)
			}
		}
	}
	if len(ends) == 0 {
		return
	}
	// Sorted by x, to find the ends near an edge quickly.
	slices.SortFunc(ends, func(a, b geom.Vector) int { return cmp.Compare(a.X, b.X) })
	inside := func(a, b geom.Vector) []geom.Vector {
		lo, hi := min(a.X, b.X)-geom.LengthEps, max(a.X, b.X)+geom.LengthEps
		k := sort.Search(len(ends), func(i int) bool { return ends[i].X >= lo })
		var on []geom.Vector
		for ; k < len(ends) && ends[k].X <= hi; k++ {
			p := ends[k]
			if !p.Equals(a) && !p.Equals(b) && p.OnLineSegment(a, b, geom.LengthEps) {
				on = append(on, p)
			}
		}
		d := b.Minus(a)
		slices.SortFunc(on, func(p, q geom.Vector) int { return cmp.Compare(p.Minus(a).Dot(d), q.Minus(a).Dot(d)) })
		return on
	}
	out := make([]STriangle, 0, len(m.Triangles))
	for _, tr := range m.Triangles {
		out = append(out, splitAtTJunctions(tr, func(a, b geom.Vector) []geom.Vector {
			if have[edge{b, a}] {
				return nil
			}
			return inside(a, b)
		})...)
	}
	m.Triangles = out
}

// splitAtTJunctions splits tr at the points on returns inside its first
// edge that has any, as a fan from the opposite corner, and splits the
// pieces again for the other edges.
func splitAtTJunctions(tr STriangle, on func(a, b geom.Vector) []geom.Vector) []STriangle {
	corners := [3]geom.Vector{tr.A, tr.B, tr.C}
	normals := [3]geom.Vector{tr.NA, tr.NB, tr.NC}
	for k := range 3 {
		a, b, c := corners[k], corners[(k+1)%3], corners[(k+2)%3]
		na, nb, nc := normals[k], normals[(k+1)%3], normals[(k+2)%3]
		pts := on(a, b)
		if len(pts) == 0 {
			continue
		}
		var out []STriangle
		pa, pn := a, na
		for _, p := range append(pts, b) {
			n := nb
			if !p.Equals(b) {
				n = normalBetween(a, b, na, nb, p)
			}
			piece := STriangle{Meta: tr.Meta, A: pa, B: p, C: c, NA: pn, NB: n, NC: nc}
			out = append(out, splitAtTJunctions(piece, on)...)
			pa, pn = p, n
		}
		return out
	}
	return []STriangle{tr}
}

// normalBetween is the vertex normal at v, on the edge from a to b
// whose ends have normals na and nb: between them, as far along as v
// is; none when they have none.
func normalBetween(a, b, na, nb, v geom.Vector) geom.Vector {
	if na.Equals(geom.Vector{}) || nb.Equals(geom.Vector{}) {
		return geom.Vector{}
	}
	t := v.Minus(a).Magnitude() / b.Minus(a).Magnitude()
	n := na.ScaledBy(1 - t).Plus(nb.ScaledBy(t))
	if n.Magnitude() < geom.LengthEps {
		return na
	}
	return n.WithMagnitude(1)
}

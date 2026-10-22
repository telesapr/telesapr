package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// MakeSectionPolygonInto puts the section of the mesh by the plane of
// h into dest: the loops where the plane cuts the triangles, to fill
// as the cap of a section view with the odd winding rule. dest's normal
// is h.N. It returns whether every loop closed, which it does for a
// watertight mesh; a leaky one gives open contours too.
//
// A vertex counts as inside when it is in h, its plane included, and a
// vertex within LengthEps of the plane is on it, laid onto it. A plane
// through a turned surface's axis passes along the seams of its
// quarters, whose vertices the meshes on either side place a few
// ulps apart, one just inside and the other just out: taken as they
// are, neither side's triangles give the seam's segment, and the
// section's contours come apart there.
func (m *SMesh) MakeSectionPolygonInto(h HalfSpace, dest *SPolygon) bool {
	dest.Clear()
	dest.Normal = h.N

	var el SEdgeList
	nn := h.N.Dot(h.N)
	eps := geom.LengthEps * math.Sqrt(nn)
	dist := func(p geom.Vector) float64 {
		if d := h.N.Dot(p) - h.D; math.Abs(d) >= eps {
			return d
		}
		return 0
	}
	// onPlane lays a point a vertex gave onto the plane; the same
	// vertex gives the same point in every triangle it belongs to.
	onPlane := func(p geom.Vector) geom.Vector {
		return p.Minus(h.N.ScaledBy((h.N.Dot(p) - h.D) / nn))
	}
	// cross is where the plane cuts edge a-b, computed from the edge's
	// ends in one order whichever triangle it belongs to, so that the
	// two triangles sharing it give the same point to the last bit.
	cross := func(a, b geom.Vector, da, db float64) geom.Vector {
		if b.X < a.X || (b.X == a.X && (b.Y < a.Y || (b.Y == a.Y && b.Z < a.Z))) {
			a, b, da, db = b, a, db, da
		}
		switch {
		case da == 0:
			return onPlane(a)
		case db == 0:
			return onPlane(b)
		}
		return a.Plus(b.Minus(a).ScaledBy(da / (da - db)))
	}
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		vs := [3]geom.Vector{tr.A, tr.B, tr.C}
		ds := [3]float64{dist(tr.A), dist(tr.B), dist(tr.C)}
		var entry, exit geom.Vector
		var haveEntry, haveExit bool
		for k := range 3 {
			j := (k + 1) % 3
			inK, inJ := ds[k] <= 0, ds[j] <= 0
			switch {
			case inK && !inJ:
				exit, haveExit = cross(vs[k], vs[j], ds[k], ds[j]), true
			case !inK && inJ:
				entry, haveEntry = cross(vs[k], vs[j], ds[k], ds[j]), true
			}
		}
		// Going round the triangle, the plane is entered once and left
		// once; a neighbour crosses their shared edge the other way, so
		// its entry is this exit, and the segments chain head to tail.
		if haveEntry && haveExit && !entry.Equals(exit) {
			el.AddEdge(entry, exit, 0, 0, 0)
		}
	}
	return assembleLoops(&el, dest)
}

// assembleLoops chains the directed edges head to tail into contours,
// closing each, as AssemblePolygon does with keepDir set, but finding
// the next edge by its start point in a map: a section of a fine mesh
// has thousands of edges. Only ends that don't meet exactly are looked
// for within LengthEps, edge by edge. A chain that doesn't close is
// grown backwards too, so that a gap leaves one open contour.
func assembleLoops(el *SEdgeList, dest *SPolygon) bool {
	starts := make(map[geom.Vector][]int, len(el.Edges))
	ends := make(map[geom.Vector][]int, len(el.Edges))
	for i := range el.Edges {
		starts[el.Edges[i].A] = append(starts[el.Edges[i].A], i)
		ends[el.Edges[i].B] = append(ends[el.Edges[i].B], i)
	}
	used := make([]bool, len(el.Edges))
	find := func(index map[geom.Vector][]int, p geom.Vector, at func(e *SEdge) geom.Vector) int {
		for _, i := range index[p] {
			if !used[i] {
				return i
			}
		}
		for i := range el.Edges {
			if !used[i] && at(&el.Edges[i]).Equals(p) {
				return i
			}
		}
		return -1
	}
	startOf := func(e *SEdge) geom.Vector { return e.A }
	endOf := func(e *SEdge) geom.Vector { return e.B }
	allClosed := true
	for i := range el.Edges {
		if used[i] {
			continue
		}
		used[i] = true
		first, last := el.Edges[i].A, el.Edges[i].B
		fwd := []geom.Vector{first, last}
		closed := false
		for {
			if last.Equals(first) {
				closed = true
				break
			}
			j := find(starts, last, startOf)
			if j < 0 {
				break
			}
			used[j] = true
			last = el.Edges[j].B
			fwd = append(fwd, last)
		}
		var back []geom.Vector
		if !closed {
			allClosed = false
			for {
				j := find(ends, first, endOf)
				if j < 0 {
					break
				}
				used[j] = true
				first = el.Edges[j].A
				back = append(back, first)
			}
		}
		var sc SContour
		for _, b := range slices.Backward(back) {
			sc.AddPoint(b)
		}
		for _, p := range fwd {
			sc.AddPoint(p)
		}
		dest.Contours = append(dest.Contours, sc)
	}
	return allClosed
}

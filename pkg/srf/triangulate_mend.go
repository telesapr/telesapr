package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// mendCrossings meshes again, by the constrained Delaunay mesher, each
// surface whose ear-clipped mesh strays from it (strays[i], its mesh
// ms[i]) or holds a sliver (earMeshSlivers) and is unsound in the
// shell's, an edge of its triangles crossing the shell's mesh or having
// no mate there, or an edge of another surface's crossing its triangles,
// and each whose triangles lie on another's
// (stackedFaces). The ear clipper fans long chords across a patch ruled
// one way or a trimmed corner, which pass a trim point on the wrong side
// and cross the face beyond (roadmap K4); on a plane it clips slivers
// along a straight trim, whose planes, turned anyhow, the face's own
// edges cross, and on a curved face fans slivers from a far grid point
// to a trim along a row of the grid; and where a trim runs up a
// meridian into a pole, or two faces fan from a corner along the curve
// they share, it joins the curve's points to the pole or corner on both
// faces, so slivers of both lie on each other, too thin to count as
// folded (K5). A surface whose edge runs along another's open edge,
// past its ends without stopping (runsAlongOpen), is taken too, flagged
// or not: where a trim lies within a sliver of the line joining two
// points of the surface's outline, the clipper joins them with one long
// edge and fans a strip over the face beyond. The CDT is sound there, at
// more triangles, so it takes only those surfaces, and a sound mesh is
// left as it is.
func (s *SShell) mendCrossings(ms []SMesh, strays []bool) {
	stacked := stackedFaces(ms)
	var all SMesh
	some := false
	for i := range ms {
		all.Triangles = append(all.Triangles, ms[i].Triangles...)
		some = some || strays[i] || stacked[i]
	}
	if !some {
		return
	}
	kd := SKdNodeFrom(&all)
	cnt := 0
	unsound := func(m *SMesh) bool {
		for i := range m.Triangles {
			tr := &m.Triangles[i]
			for j := range 3 {
				a, b := tr.vertex(j), tr.vertex((j+1)%3)
				cnt++
				var info EdgeOnInfo
				kd.FindEdgeOn(a, b, cnt, true, &info)
				if info.IntersectsMesh {
					return true
				}
				if info.Count != 1 {
					// Coincident edges run both ways in equal numbers.
					cnt++
					var par EdgeOnInfo
					kd.FindEdgeOn(b, a, cnt, true, &par)
					if info.Count != par.Count {
						return true
					}
				}
			}
		}
		return false
	}
	// pierced tells whether an edge of another surface's mesh passes
	// through a triangle of ms[i]: a sliver the ear clipper fans across a
	// cone from its base to a point on its top trim cuts through the
	// solid, and its own edges, each ending on a face it meets, cross
	// nothing; the flat face beyond, its fan reaching that point, does.
	pierced := func(i int) bool {
		own := SMesh{Triangles: slices.Clone(ms[i].Triangles)}
		ki := SKdNodeFrom(&own)
		for j := range ms {
			if j == i {
				continue
			}
			for k := range ms[j].Triangles {
				tr := &ms[j].Triangles[k]
				for e := range 3 {
					cnt++
					var info EdgeOnInfo
					ki.FindEdgeOn(tr.vertex(e), tr.vertex((e+1)%3), cnt, true, &info)
					if info.IntersectsMesh {
						return true
					}
				}
			}
		}
		return false
	}
	// open is, once asked, the shell's edges with no mate: a T-junction
	// leaves them where a face's edge runs along another's trim points
	// without stopping at them.
	var open [][2]geom.Vector
	openDone := false
	openEdges := func() [][2]geom.Vector {
		if !openDone {
			openDone = true
			open = unmatedEdges(&all, kd, &cnt)
		}
		return open
	}
	for i := range ms {
		bad := stacked[i] || strays[i] && (unsound(&ms[i]) || pierced(i))
		// A face whose edge runs along another's open edge is the one to
		// mesh again, flagged or not: the long edge's own triangles are
		// whole, and only the face beyond, left open, was flagged.
		if !bad && !runsAlongOpen(&ms[i], openEdges(), s.Surface[i].Tol.ChordTolMm()) {
			continue
		}
		var m SMesh
		s.Surface[i].triangulate(s, &m, true)
		ms[i] = m
	}
}

// stackedFaces tells which meshes have a triangle on the same three
// points as another of the shell's, either way round: two faces' meshes
// lying on each other there, which close and mate edge for edge, but
// fill no volume. A sound mesh has none; where solids meet along an edge
// (a knife edge), faces share the edge, not a triangle, and are left
// alone. Points that are one differ in their last digits, so they are
// compared on a grid of lengthEps; a pair split by the grid is missed,
// which costs only a face not mended.
func stackedFaces(ms []SMesh) []bool {
	type point [3]int64
	snap := func(p geom.Vector) point {
		return point{int64(math.Round(p.X / lengthEps)), int64(math.Round(p.Y / lengthEps)), int64(math.Round(p.Z / lengthEps))}
	}
	first := map[[3]point]int{}
	out := make([]bool, len(ms))
	for i := range ms {
		for j := range ms[i].Triangles {
			tr := &ms[i].Triangles[j]
			key := [3]point{snap(tr.A), snap(tr.B), snap(tr.C)}
			slices.SortFunc(key[:], func(p, q point) int { return slices.Compare(p[:], q[:]) })
			if f, ok := first[key]; ok {
				out[i], out[f] = true, true
			} else {
				first[key] = i
			}
		}
	}
	return out
}

// unmatedEdges are the edges of m (its k-d tree kd) that have no mate:
// as many edges run b -> a there as a -> b on coincident ones. cnt tags
// triangles seen, as FindEdgeOn wants.
func unmatedEdges(m *SMesh, kd *SKdNode, cnt *int) [][2]geom.Vector {
	var out [][2]geom.Vector
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		for j := range 3 {
			a, b := tr.vertex(j), tr.vertex((j+1)%3)
			*cnt++
			var info EdgeOnInfo
			kd.FindEdgeOn(a, b, *cnt, false, &info)
			if info.Count == 1 {
				continue
			}
			*cnt++
			var par EdgeOnInfo
			kd.FindEdgeOn(b, a, *cnt, false, &par)
			if info.Count != par.Count {
				out = append(out, [2]geom.Vector{a, b})
			}
		}
	}
	return out
}

// runsAlongOpen tells whether an edge of m runs past an end of an open
// edge of the shell's mesh (open), within tol of it but not ending
// there: a T-junction. A chamfer's foot along a rod, its trim within a
// sliver of the line joining two points of the rod's end rings, leaves
// the ear clipper's long edge between those points running along the
// foot's points without stopping at them, and its strip of triangles
// over the chamfer's; every edge of the rod's mesh has its mate, so
// only the foot's edges, left open, show it.
func runsAlongOpen(m *SMesh, open [][2]geom.Vector, tol float64) bool {
	if len(open) == 0 {
		return false
	}
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		for j := range 3 {
			a, b := tr.vertex(j), tr.vertex((j+1)%3)
			for _, e := range open {
				for _, p := range e {
					if p.Minus(a).Magnitude() > lengthEps && p.Minus(b).Magnitude() > lengthEps &&
						distanceToSegment(p, a, b) < tol {
						return true
					}
				}
			}
		}
	}
	return false
}

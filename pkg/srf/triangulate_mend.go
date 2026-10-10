package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// mendCrossings meshes again, by the constrained Delaunay mesher, each
// surface whose ear-clipped mesh strays from it (strays[i], its mesh
// ms[i]) and is unsound in the shell's, an edge of its triangles
// crossing the shell's mesh or having no mate there, and each whose
// triangles lie on another's (stackedFaces). The ear clipper fans long chords across a patch ruled one
// way or a trimmed corner, which pass a trim point on the wrong side and
// cross the face beyond (roadmap K4); and where a trim runs up a
// meridian into a pole, or two faces fan from a corner along the curve
// they share, it joins the curve's points to the pole or corner on both
// faces, so slivers of both lie on each other, too thin to count as
// folded (K5). The CDT is sound there, at more triangles, so it
// takes only those surfaces, and a sound mesh is left as it is.
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
	for i := range ms {
		if !stacked[i] && (!strays[i] || !unsound(&ms[i])) {
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

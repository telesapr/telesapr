package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// mendCrossings meshes again by the ear clipper each face the
// constrained Delaunay mesher meshed that crosses the shell's mesh
// (mendDelaunayCrossings); then, by the constrained Delaunay mesher, each
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
	s.mendDelaunayCrossings(ms)
	s.paced()
	stacked := stackedFaces(ms)
	s.paced()
	some := false
	for i := range ms {
		some = some || strays[i] || stacked[i]
	}
	if !some {
		return
	}
	mc := newMeshCheck(ms, s.pace)
	// open is, once asked, the shell's edges with no mate: a T-junction
	// leaves them where a face's edge runs along another's trim points
	// without stopping at them.
	var open [][2]geom.Vector
	openDone := false
	openEdges := func() [][2]geom.Vector {
		if !openDone {
			openDone = true
			open = unmatedEdges(&mc.all, mc.kd, &mc.cnt)
		}
		return open
	}
	for i := range ms {
		s.paced()
		bad := stacked[i] || strays[i] && (mc.unsound(&ms[i]) || mc.pierced(ms, i))
		// A face whose edge runs along another's open edge is the one to
		// mesh again, flagged or not: the long edge's own triangles are
		// whole, and only the face beyond, left open, was flagged.
		if !bad && !runsAlongOpen(&ms[i], openEdges(), s.Surface[i].Tol.ChordTolMm()) {
			continue
		}
		var m SMesh
		s.Surface[i].triangulate(s, &m, meshCDT)
		ms[i] = m
	}
}

// mendDelaunayCrossings meshes again by the ear clipper each face the
// constrained Delaunay mesher meshed whose mesh crosses the shell's,
// or is crossed by it. Two faces that come within the chord tolerance
// of each other, as the sides of a triangle lofted a third of a turn
// round near its corners, have meshes each within it of its own surface
// that may cross there; the ear clipper's rows, one per point of the
// trims, keep clear of each other (TestLoftSweep). A face meshed again
// is checked again against the faces beside it, and they against it,
// for a few rounds.
func (s *SShell) mendDelaunayCrossings(ms []SMesh) {
	byCDT := make([]bool, len(ms))
	some := false
	for i := range ms {
		byCDT[i] = s.Surface[i].byCDT()
		some = some || byCDT[i]
	}
	if !some || len(ms) < 2 {
		return
	}
	for range 3 {
		mc := newMeshCheck(ms, s.pace)
		var bad []int
		for i := range ms {
			s.paced()
			if byCDT[i] && mc.unsound(&ms[i]) {
				bad = append(bad, i)
			}
		}
		if len(bad) == 0 {
			return
		}
		for _, i := range bad {
			var m SMesh
			s.Surface[i].triangulate(s, &m, meshEar)
			ms[i] = m
			byCDT[i] = false
		}
	}
}

// meshCheck is a shell's meshes joined and their k-d tree, to tell
// whether one face's mesh crosses the rest.
type meshCheck struct {
	all SMesh
	kd  *SKdNode
	// cnt tags the triangles seen, as FindEdgeOn wants.
	cnt int
	// pace, when not nil, is called every meshCheckPaceEvery triangles
	// checked, as the shell's pace is (SShell.PaceBy): one face may
	// hold most of a shell's triangles.
	pace func()
}

// meshCheckPaceEvery is how many triangles a meshCheck checks between
// paces.
const meshCheckPaceEvery = 1024

// newMeshCheck joins ms, to be checked with pace called now and then
// (nil for never).
func newMeshCheck(ms []SMesh, pace func()) *meshCheck {
	mc := &meshCheck{pace: pace}
	for i := range ms {
		mc.all.Triangles = append(mc.all.Triangles, ms[i].Triangles...)
	}
	mc.kd = SKdNodeFromPaced(&mc.all, pace)
	return mc
}

// unsound reports whether an edge of m's triangles crosses the joined
// mesh, or has no mate in it.
func (mc *meshCheck) unsound(m *SMesh) bool {
	for i := range m.Triangles {
		if mc.pace != nil && i%meshCheckPaceEvery == meshCheckPaceEvery-1 {
			mc.pace()
		}
		tr := &m.Triangles[i]
		for j := range 3 {
			a, b := tr.vertex(j), tr.vertex((j+1)%3)
			mc.cnt++
			var info EdgeOnInfo
			mc.kd.FindEdgeOn(a, b, mc.cnt, true, &info)
			if info.IntersectsMesh {
				return true
			}
			if info.Count != 1 {
				// Coincident edges run both ways in equal numbers.
				mc.cnt++
				var par EdgeOnInfo
				mc.kd.FindEdgeOn(b, a, mc.cnt, true, &par)
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
// cone from its base to a point on its top trim cuts through the solid,
// and its own edges, each ending on a face it meets, cross nothing; the
// flat face beyond, its fan reaching that point, does.
func (mc *meshCheck) pierced(ms []SMesh, i int) bool {
	own := SMesh{Triangles: slices.Clone(ms[i].Triangles)}
	ki := SKdNodeFrom(&own)
	for j := range ms {
		if j == i {
			continue
		}
		for k := range ms[j].Triangles {
			tr := &ms[j].Triangles[k]
			for e := range 3 {
				mc.cnt++
				var info EdgeOnInfo
				ki.FindEdgeOn(tr.vertex(e), tr.vertex((e+1)%3), mc.cnt, true, &info)
				if info.IntersectsMesh {
					return true
				}
			}
		}
	}
	return false
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

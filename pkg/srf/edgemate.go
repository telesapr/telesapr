package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A mateIndex holds a mesh's triangles and finds, for one of its edges,
// the edges that run the other way: the other side of the same surface
// patch boundary, or the other side of a wall. Listing a mesh's outlines
// is mostly this search, and a kd-tree of the triangles does it badly on
// a solid, whose triangles are large enough to straddle the tree's
// splitting planes: each of those is then held by several leaves, and
// every edge query reads it again in each. On the assembly this was
// written for that is a million triangle reads a frame, where the index
// below reads a few tens of thousands.
//
// The index is a grid over the mesh: every triangle is in the cells of
// its three vertices. A query reads the cells around one end of its
// edge, and looks for the triangle that has that edge from there: the
// other end of a matching edge is within lengthEps, which puts it in the
// same cell or the next one.

// mateIndex is a mesh's triangles, in the grid cells of their vertices.
type mateIndex struct {
	cells map[[3]int32][]*STriangle
	// cell is how fine the grid is, in the mesh's units.
	cell float64
	// tag marks the triangles read for one edge, in STriangle.Tag.
	tag int
}

// cellOf is the cell that p falls in.
func (ix *mateIndex) cellOf(p geom.Vector) [3]int32 {
	return [3]int32{
		int32(math.Floor(p.X / ix.cell)),
		int32(math.Floor(p.Y / ix.cell)),
		int32(math.Floor(p.Z / ix.cell)),
	}
}

// newMateIndex indexes the triangles.
func newMateIndex(tris []*STriangle) *mateIndex {
	// The stamp starts past the tags the triangles come with, which the
	// tree leaves at one, and only rises from there.
	ix := &mateIndex{cells: map[[3]int32][]*STriangle{}, tag: 1233}
	// Cells a fraction of the size of the edges, so that the cells
	// around one hold few vertices.
	ix.cell = max(meanEdgeLength(tris)/8, 16*lengthEps)
	for _, tr := range tris {
		for _, v := range [3]geom.Vector{tr.A, tr.B, tr.C} {
			c := ix.cellOf(v)
			ix.cells[c] = append(ix.cells[c], tr)
		}
	}
	return ix
}

// find fills in what info learns about the edges that run b -> a, the
// other side of the edge a -> b, counting them.
func (ix *mateIndex) find(a, b geom.Vector, info *EdgeOnInfo) {
	*info = EdgeOnInfo{}
	ix.tag++
	cb := ix.cellOf(b)
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				c := [3]int32{cb[0] + int32(i) - 1, cb[1] + int32(j) - 1, cb[2] + int32(k) - 1}
				for _, tr := range ix.cells[c] {
					// A triangle with more than one vertex in the cells
					// around b is read from each of them; read it once.
					if tr.Tag == ix.tag {
						continue
					}
					tr.Tag = ix.tag
					countMate(tr, a, b, info)
				}
			}
		}
	}
}

// countMate counts the edge b -> a of tr, the other side of a -> b.
func countMate(tr *STriangle, a, b geom.Vector, info *EdgeOnInfo) {
	// Test if this triangle matches up with the given edge.
	if (a.Equals(tr.B) && b.Equals(tr.A)) ||
		(a.Equals(tr.C) && b.Equals(tr.B)) ||
		(a.Equals(tr.A) && b.Equals(tr.C)) {
		info.Count++
		// Record whether this triangle is front- or back-facing.
		info.FrontFacing = tr.Normal().Z > lengthEps
		// And the triangle, and which of its vertices a and b are.
		info.Tr = tr
		info.Ai = vertexIndex(tr, a)
		info.Bi = vertexIndex(tr, b)
	}
}

// meanEdgeLength is how long a typical edge of the triangles is.
func meanEdgeLength(tris []*STriangle) float64 {
	total, n := 0.0, 0
	for _, tr := range tris {
		for j := range 3 {
			total += tr.vertex(j).DistanceTo(tr.vertex((j + 1) % 3))
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return total / float64(n)
}

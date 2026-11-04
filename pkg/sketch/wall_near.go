package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The corners of the pairs of triangles rays joined (wall_corners.go)
// leave out a wall between two triangles no measured ray reached, as
// under a dimple narrower than the points measured are apart, whose
// own rays run sideways into the part. Any wall thinner than the
// thinnest found lies between two triangles that face each other within
// that thickness: a ray of length t from a point of one to the other
// puts them within t. So the pairs within it of each triangle, facing
// it, are taken as the rays' pairs are, their corners measured where
// the wall there could be thinner, and the corners' rays walked on as
// before.
//
// The triangles a ray from triangle T can reach lie in the prism T
// sweeps into the part. A box round the whole prism, slanting across
// the mesh's axes, takes in a good part of a round part (a ball of 16k
// triangles gave 2.7M candidates, 900k past the cheap tests), so the
// tree is walked along the segment T's centre sweeps, and a pair
// counts only where the one triangle, seen along the ray, covers some
// of the other. (The least corner of a pair isn't taken to bound the
// prisms of the triangles after it: at a vertex two triangles share,
// as the faces of a pit at its point, it is no wall but rounding, a
// hundred-millionth or so, and would end the search.)

// wallNearMost is how many pairs of triangles of a part, each covering
// some of the other, the search for facing triangles measures; past it
// the rest are left and the report says so (WallReport.NearCut).
const wallNearMost = 4 << 20

// appendPrismNear appends to dst the triangles of the tree that may meet
// the prism triangle t sweeps along in to depth: those near the segment
// its centre sweeps, within the reach of its corners from the centre.
func appendPrismNear(dst []int, tree *srf.MeshTree, t *srf.STriangle, in geom.Vector, depth float64) []int {
	c := t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3)
	reach := max(t.A.Minus(c).Magnitude(), t.B.Minus(c).Magnitude(), t.C.Minus(c).Magnitude())
	return tree.AppendNearSegment(dst, c, c.Plus(in.ScaledBy(depth)), reach)
}

// measureNear measures the corners of the pairs of triangles of the
// part that face each other within the thinnest found, or below where
// that is more, and that no ray found joined, and adds what they found
// to found.
func (r *WallReport) measureNear(w *clearancePart, below float64, cw *cornerWalk, found []wallSample) []wallSample {
	limit := max(below, cw.least)
	if math.IsInf(limit, 1) {
		return found
	}
	n := w.tree.Len()
	normals := make([]geom.Vector, n)
	for i := range n {
		if nv := w.tree.Triangle(i).Normal(); nv.Magnitude() >= 1e-12 {
			normals[i] = nv.WithMagnitude(1)
		}
	}
	// A wall no thinner than the thinnest by more than the meshes'
	// tolerance tells nothing the thinnest doesn't, as across a ball,
	// where every pair of triangles facing each other measures within it
	// of its diameter.
	cutoff := max(below, cw.least-2*w.chord-geom.LengthEps)
	looked := 0
	var seeds []wallSample
	var near []int
	for i := range n {
		t := w.tree.Triangle(i)
		f, ok := newWallFrame(t)
		if !ok {
			continue
		}
		near = appendPrismNear(near[:0], w.tree, t, f.in, limit)
		for _, j := range near {
			// Only a triangle the rays leave through, that comes nearer
			// t's plane than the cutoff and isn't wholly behind it, and
			// that covers some of t seen along the rays, can hold a
			// corner of a wall thinner than it.
			h := w.tree.Triangle(j)
			if j == i || normals[j].Dot(f.in) < 1e-9 || cw.seen[[2]int{i, j}] {
				continue
			}
			da, db, dc := h.A.Minus(t.A).Dot(f.in), h.B.Minus(t.A).Dot(f.in), h.C.Minus(t.A).Dot(f.in)
			if min(da, db, dc) >= cutoff || max(da, db, dc) <= -geom.LengthEps {
				continue
			}
			corners := f.corners(h)
			if len(corners) == 0 {
				continue
			}
			if looked++; looked > wallNearMost {
				r.NearCut = true
				return r.walkCorners(w, below, cw, found, seeds)
			}
			least := math.Inf(1)
			for _, c := range corners {
				if c.t > -geom.LengthEps {
					least = min(least, c.t)
				}
			}
			if least < cutoff {
				seeds = append(seeds, wallSample{spot: ThinSpot{Thickness: least}, tri: i, hit: j})
			}
		}
	}
	return r.walkCorners(w, below, cw, found, seeds)
}

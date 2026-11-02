package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
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
// before. The pairs are found in the mesh's box tree, about as many as
// the triangles have neighbours within the thinnest; a part whose
// thinnest wall is a good part of its size has many, and past
// wallNearMost pairs looked at the rest are left.

// wallNearMost is how many pairs of triangles of a part the search for
// facing triangles looks at.
const wallNearMost = 4 << 20

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
	cutoff := func() float64 { return max(below, cw.least-2*w.chord-geom.LengthEps) }
	looked := 0
	var seeds []wallSample
	var near []int
	for i := range n {
		t := w.tree.Triangle(i)
		f, ok := newWallFrame(t)
		if !ok {
			continue
		}
		// The prism t sweeps into the part as far as the limit holds
		// every triangle a ray from t meets within it.
		deep := f.in.ScaledBy(limit)
		lo, hi := t.A, t.A
		for _, v := range []geom.Vector{t.A, t.B, t.C, t.A.Plus(deep), t.B.Plus(deep), t.C.Plus(deep)} {
			v.MakeMaxMin(&hi, &lo)
		}
		near = w.tree.AppendInBox(near[:0], lo, hi)
		for _, j := range near {
			// Only a triangle the rays leave through, and that comes
			// nearer t's plane than the cutoff, can hold a corner of a
			// wall thinner than it.
			h := w.tree.Triangle(j)
			if j == i || normals[j].Dot(f.in) < 1e-9 ||
				min(h.A.Minus(t.A).Dot(f.in), h.B.Minus(t.A).Dot(f.in), h.C.Minus(t.A).Dot(f.in)) >= cutoff() ||
				cw.seen[[2]int{i, j}] {
				continue
			}
			if looked++; looked > wallNearMost {
				return r.walkCorners(w, below, cw, found, seeds)
			}
			least := math.Inf(1)
			for _, c := range f.corners(h) {
				if c.t > -geom.LengthEps {
					least = min(least, c.t)
				}
			}
			// A wall no thinner than the thinnest found by more than the
			// meshes' tolerance tells nothing the thinnest doesn't, as
			// across a ball, where every pair of triangles facing each
			// other measures within it of its diameter.
			if least < cutoff() {
				seeds = append(seeds, wallSample{spot: ThinSpot{Thickness: least}, tri: i, hit: j})
			}
		}
	}
	return r.walkCorners(w, below, cw, found, seeds)
}

package check

import (
	"cmp"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Manifold is what one pass over a mesh's sorted half-edges finds, as
// a closed two-manifold has none of: each undirected edge should be
// used by exactly two triangles, once each way.
type Manifold struct {
	// Unmatched counts edges used once: the mesh is open there.
	Unmatched int
	// Excess counts edges used by more than two triangles.
	Excess int
	// Misoriented counts edges used twice the same way: two triangles
	// there face opposite ways.
	Misoriented int
	// Degenerate counts triangles with two corners at one place.
	Degenerate int
}

// Closed reports whether every edge is used twice, once each way, by
// triangles that aren't degenerate.
func (m Manifold) Closed() bool { return m == Manifold{} }

// halfEdge is a triangle's edge from a to b, with its undirected key.
type halfEdge struct {
	lo, hi  geom.Vector
	forward bool
}

// lessVec orders points by x, then y, then z.
func lessVec(a, b geom.Vector) int {
	if c := cmp.Compare(a.X, b.X); c != 0 {
		return c
	}
	if c := cmp.Compare(a.Y, b.Y); c != 0 {
		return c
	}
	return cmp.Compare(a.Z, b.Z)
}

// MeshManifold sorts the mesh's half-edges by their ends, so that the
// uses of each edge lie together, and counts what is wrong with each
// run: O(n log n). The kernel's meshes meet at T-junctions, a corner of
// one triangle on another's side, so a copy is first made vertex to
// vertex (srf.SKdNode.SnapToMesh); its triangles then share their
// corners exactly.
func MeshManifold(mesh *srf.SMesh) Manifold {
	var m Manifold
	var c, snapped srf.SMesh
	c.MakeFromCopyOf(mesh)
	root := srf.SKdNodeFrom(&c)
	root.SnapToMesh(&c)
	root.MakeMeshInto(&snapped)
	hs := make([]halfEdge, 0, 3*len(snapped.Triangles))
	for i := range snapped.Triangles {
		tr := &snapped.Triangles[i]
		if tr.A == tr.B || tr.B == tr.C || tr.C == tr.A {
			m.Degenerate++
			continue
		}
		for _, e := range [3][2]geom.Vector{{tr.A, tr.B}, {tr.B, tr.C}, {tr.C, tr.A}} {
			if lessVec(e[0], e[1]) < 0 {
				hs = append(hs, halfEdge{e[0], e[1], true})
			} else {
				hs = append(hs, halfEdge{e[1], e[0], false})
			}
		}
	}
	slices.SortFunc(hs, func(a, b halfEdge) int {
		if c := lessVec(a.lo, b.lo); c != 0 {
			return c
		}
		return lessVec(a.hi, b.hi)
	})
	for i := 0; i < len(hs); {
		j, fwd := i, 0
		for ; j < len(hs) && hs[j].lo == hs[i].lo && hs[j].hi == hs[i].hi; j++ {
			if hs[j].forward {
				fwd++
			}
		}
		switch n := j - i; {
		case n == 1:
			m.Unmatched++
		case n > 2:
			m.Excess++
		case fwd != 1:
			m.Misoriented++
		}
		i = j
	}
	return m
}

// RayReport is what the ray-cast oracle finds of a group's solid,
// against the kernel's.
type RayReport struct {
	// Measure is the oracle's volume and centroid.
	Measure RayMeasure
	// MeshVolume is the volume the kernel's mesh encloses.
	MeshVolume float64
	// Disagree are the rays along which the oracle and the mesh differ
	// by more than twice the chord tolerance.
	Disagree []RayDiff
	// Rays is how many rays were compared.
	Rays int
	// Manifold is what the mesh's half-edges say.
	Manifold Manifold
}

// RayCheck measures group g's solid by the ray-cast oracle, compares it
// ray by ray with the kernel's mesh along a grid of n rays a side in
// each direction, and checks the mesh's half-edges. The volume is
// refined until the oracle's three grids agree within 0.1%, or 256
// rays a side. ErrUnsupported, wrapped, says which group the oracle
// can't build; a group with no solid gives a report with no volume.
func RayCheck(sk *sketch.Sketch, g *sketch.Group, n int) (RayReport, error) {
	var r RayReport
	o, err := NewOracle(sk, g.H)
	if err != nil {
		return r, err
	}
	g.GenerateDisplayItems(sk)
	mesh := &g.DisplayMesh
	rep := Solid(nil, mesh)
	r.MeshVolume = rep.Volume
	r.Manifold = MeshManifold(mesh)
	if o == nil {
		return r, nil
	}
	r.Measure = o.Measure(1e-3, 256)
	r.Rays = 3 * n * n
	r.Disagree = o.Diff(mesh, n, 2*sk.Tol().ChordTolMm())
	return r, nil
}

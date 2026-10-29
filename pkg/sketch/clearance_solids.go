package sketch

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A part's pieces, its surfaces joined by their trims or its triangles
// by their corners, are not its solids: a hollow's inside is a piece of
// its own, closed and facing in, and with the piece round it is one
// solid. Taken apart, the hollow's inside would be a gap to the solid
// it is in, and a wall's ray from the inside would find no way out.

// solidsOf groups pieces, by their meshes, into solids: each piece
// that encloses a negative volume, a hollow's inside, goes with the
// smallest piece of positive volume it lies in. A piece lying in none,
// as the inside of an open mesh, is a solid of its own.
func solidsOf(pieces []srf.SMesh) [][]int {
	vols := make([]float64, len(pieces))
	trees := make([]*srf.MeshTree, len(pieces))
	for i := range pieces {
		vols[i] = pieces[i].CalculateVolume()
	}
	var out [][]int
	at := make([]int, len(pieces))
	for i := range pieces {
		at[i] = -1
		if vols[i] >= 0 {
			at[i] = len(out)
			out = append(out, []int{i})
		}
	}
	for i := range pieces {
		if vols[i] >= 0 {
			continue
		}
		best := -1
		for j := range pieces {
			if vols[j] < 0 || (best >= 0 && vols[j] >= vols[best]) {
				continue
			}
			if trees[j] == nil {
				trees[j] = srf.NewMeshTree(&pieces[j])
			}
			if pieceInside(&pieces[i], trees[j]) {
				best = j
			}
		}
		if best < 0 {
			out = append(out, []int{i})
			continue
		}
		out[at[best]] = append(out[at[best]], i)
	}
	return out
}

// pieceInside reports whether a hollow's inside, the piece in, lies in
// the solid of tree: a ray from one of its triangles, into the
// material round it, leaves that solid first through a face that faces
// out.
func pieceInside(in *srf.SMesh, tree *srf.MeshTree) bool {
	for i := range in.Triangles {
		tr := &in.Triangles[i]
		n := tr.Normal()
		if n.Magnitude() < 1e-12 {
			continue
		}
		mid := tr.A.Plus(tr.B).Plus(tr.C).ScaledBy(1.0 / 3)
		h, ok := tree.Ray(mid, n.WithMagnitude(-1), 0, -1)
		return ok && h.Cos > 0
	}
	return false
}

// shellSolids takes a part's shell apart into its solids
// (srf.SShell.Pieces, solidsOf): each a part of the mesh p holds, its
// triangles still tagged by their places in the whole shell. A shell
// of one solid gives nil.
func shellSolids(p *clearancePart) []*clearancePart {
	pieces := p.shell.Pieces()
	if len(pieces) < 2 {
		return nil
	}
	of := map[core.HSSurface]int{}
	for k, sh := range pieces {
		for i := range sh.Surface {
			of[sh.Surface[i].H] = k
		}
	}
	meshes := make([]srf.SMesh, len(pieces))
	for _, tr := range p.mesh.Triangles {
		k := of[p.shell.Surface[tr.Tag].H]
		meshes[k].Triangles = append(meshes[k].Triangles, tr)
	}
	return p.solidsFrom(meshes)
}

// meshSolids takes a part that is a mesh apart into its solids, by its
// pieces (meshPieces, solidsOf); a mesh of one solid gives nil.
func meshSolids(p *clearancePart) []*clearancePart {
	pieces := meshPieces(&p.mesh)
	if len(pieces) < 2 {
		return nil
	}
	return p.solidsFrom(pieces)
}

// solidsFrom makes a part of each solid of pieces, the part p's pieces;
// nil where they are one solid.
func (p *clearancePart) solidsFrom(pieces []srf.SMesh) []*clearancePart {
	solids := solidsOf(pieces)
	if len(solids) < 2 {
		return nil
	}
	out := make([]*clearancePart, len(solids))
	for k, s := range solids {
		sp := &clearancePart{g: p.g, solid: k + 1, shell: p.shell, chord: p.chord}
		for _, i := range s {
			sp.mesh.Triangles = append(sp.mesh.Triangles, pieces[i].Triangles...)
		}
		sp.tree = srf.NewMeshTree(&sp.mesh)
		out[k] = sp
	}
	return out
}

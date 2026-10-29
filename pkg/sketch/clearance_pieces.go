package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// meshPieces are the pieces of a mesh: its triangles joined by the
// corners they share, within LengthEps, each piece in the order its
// triangles come in the mesh. A mesh imported as one part, as an STL
// of an assembly, is taken apart so, as a shell is by its surfaces'
// trims (srf.SShell.Pieces). A mesh of one piece is one.
func meshPieces(m *srf.SMesh) []srf.SMesh {
	n := len(m.Triangles)
	up := make([]int, n)
	for i := range up {
		up[i] = i
	}
	root := func(i int) int {
		for up[i] != i {
			up[i] = up[up[i]]
			i = up[i]
		}
		return i
	}
	// Two corners within LengthEps of each other along an axis fall in
	// one cell of a grid twice that size, or of the same grid moved by
	// half a cell along it: so each corner is put in the eight grids so
	// moved, and joined to what is in its cell of each.
	type cell [4]int64
	at := map[cell]int{}
	size := 2 * geom.LengthEps
	key := func(v geom.Vector, g int, dx, dy, dz float64) cell {
		return cell{int64(g), int64(math.Floor(v.X/size + dx)), int64(math.Floor(v.Y/size + dy)), int64(math.Floor(v.Z/size + dz))}
	}
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		for _, v := range []geom.Vector{tr.A, tr.B, tr.C} {
			for g := range 8 {
				k := key(v, g, float64(g&1)/2, float64(g>>1&1)/2, float64(g>>2&1)/2)
				if j, ok := at[k]; ok {
					up[root(i)] = root(j)
				} else {
					at[k] = i
				}
			}
		}
	}
	index := map[int]int{}
	var out []srf.SMesh
	for i := range m.Triangles {
		r := root(i)
		k, ok := index[r]
		if !ok {
			k = len(out)
			index[r] = k
			out = append(out, srf.SMesh{})
		}
		out[k].Triangles = append(out[k].Triangles, m.Triangles[i])
	}
	return out
}

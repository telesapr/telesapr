package export

import (
	"fmt"
	"strings"

	"github.com/telesapr/telesapr/pkg/srf"
)

// stepBody is a body to write: its outside, and the cavities inside it.
type stepBody struct {
	outer *srf.SShell
	voids []*srf.SShell
}

// writeBodies writes shell's faces, scaled by scale, as solids named
// name: a MANIFOLD_SOLID_BREP for each body, each of its own CLOSED_SHELL,
// as a closed shell must be connected. A shell of bodies apart (a mirror
// copy beside the original, a pattern's copies) went into one
// CLOSED_SHELL before, which OpenCASCADE and FreeCAD split and a stricter
// reader may refuse. A body with cavities is a BREP_WITH_VOIDS: each
// cavity, a piece whose inside faces in, is a void of the body round it,
// an ORIENTED_CLOSED_SHELL turning back the closed shell of the
// cavity's faces turned out, as ISO 10303-42 has a void. It returns the
// solids' ids.
func (w *stepFileWriter) writeBodies(shell *srf.SShell, scale float64, name string) []int {
	var breps []int
	for k, b := range stepBodies(shell) {
		if k > 0 {
			// Each body stands alone: no point or edge is shared.
			w.resetAliases()
		}
		outer, ok := w.writeClosedShell(b.outer, scale)
		if !ok {
			continue
		}
		var voids []int
		for _, v := range b.voids {
			w.turned = true
			cs, ok := w.writeClosedShell(v, scale)
			w.turned = false
			if !ok {
				continue
			}
			w.printf("#%d=ORIENTED_CLOSED_SHELL('',*,#%d,.F.);\n", w.id, cs)
			voids = append(voids, w.id)
			w.id++
		}
		if len(voids) == 0 {
			w.printf("#%d=MANIFOLD_SOLID_BREP(%s,#%d);\n", w.id, name, outer)
		} else {
			w.printf("#%d=BREP_WITH_VOIDS(%s,#%d,(%s));\n", w.id, name, outer, stepRefs(voids...))
		}
		breps = append(breps, w.id)
		w.id++
	}
	return breps
}

// writeClosedShell writes the faces of shell, scaled by scale, as a
// CLOSED_SHELL and returns its id; false when it has no face to write.
func (w *stepFileWriter) writeClosedShell(shell *srf.SShell, scale float64) (int, bool) {
	w.advancedFaces = w.advancedFaces[:0]
	w.exportShellFaces(shell, scale)
	if len(w.advancedFaces) == 0 {
		return 0, false
	}
	w.printf("#%d=CLOSED_SHELL('',(%s));\n", w.id, stepRefs(w.advancedFaces...))
	w.id++
	return w.id - 1, true
}

// stepBodies are shell's pieces as bodies: each piece whose mesh
// encloses a positive volume a body, and each that encloses a negative
// one, a cavity, a void of the smallest body round it. Where a cavity
// lies in no body, or no piece is a body, the shell is one body as it
// was written before cavities were voids: a CLOSED_SHELL of every face.
func stepBodies(shell *srf.SShell) []stepBody {
	pieces := shell.Pieces()
	if len(pieces) == 1 {
		return []stepBody{{outer: shell}}
	}
	meshes := make([]srf.SMesh, len(pieces))
	vols := make([]float64, len(pieces))
	var bodies []stepBody
	at := make(map[int]int) // a piece's body
	for i, p := range pieces {
		p.TriangulateInto(&meshes[i])
		vols[i] = meshes[i].CalculateVolume()
		if vols[i] > 0 {
			at[i] = len(bodies)
			bodies = append(bodies, stepBody{outer: p})
		}
	}
	whole := []stepBody{{outer: shell}}
	if len(bodies) == 0 {
		return whole
	}
	for i, p := range pieces {
		if vols[i] > 0 {
			continue
		}
		if len(meshes[i].Triangles) == 0 {
			return whole
		}
		// A point of the cavity's surface lies inside the body round it,
		// clear of the body's own surface.
		pt := meshes[i].Triangles[0].A
		round := -1
		for j := range pieces {
			if vols[j] > 0 && meshes[j].WindingNumber(pt) > 0.5 &&
				(round < 0 || vols[j] < vols[round]) {
				round = j
			}
		}
		if round < 0 {
			return whole
		}
		b := &bodies[at[round]]
		b.voids = append(b.voids, p)
	}
	return bodies
}

// stepRefs lists ids as a STEP list's items, "#1,#2".
func stepRefs(ids ...int) string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("#%d", id)
	}
	return strings.Join(out, ",")
}

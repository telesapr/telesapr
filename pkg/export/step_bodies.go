package export

import (
	"fmt"
	"strings"

	"github.com/telesapr/telesapr/pkg/srf"
)

// writeBodies writes shell's faces, scaled by scale, as solids named
// name: a MANIFOLD_SOLID_BREP for each body, each of its own CLOSED_SHELL,
// as a closed shell must be connected. A shell of bodies apart (a mirror
// copy beside the original, a pattern's copies) went into one
// CLOSED_SHELL before, which OpenCASCADE and FreeCAD split and a stricter
// reader may refuse. A shell with a cavity, whose inner piece faces in,
// is still written as one CLOSED_SHELL: its cavity would be a void of
// the body round it (BREP_WITH_VOIDS), not a body. It returns the
// solids' ids.
func (w *stepFileWriter) writeBodies(shell *srf.SShell, scale float64, name string) []int {
	pieces := shell.Pieces()
	if len(pieces) > 1 && hasCavity(pieces) {
		pieces = []*srf.SShell{shell}
	}
	var breps []int
	for k, p := range pieces {
		if k > 0 {
			// Each body stands alone: no point or edge is shared.
			w.resetAliases()
		}
		w.advancedFaces = w.advancedFaces[:0]
		w.exportShellFaces(p, scale)
		if len(w.advancedFaces) == 0 {
			continue
		}
		faces := make([]string, len(w.advancedFaces))
		for i, af := range w.advancedFaces {
			faces[i] = fmt.Sprintf("#%d", af)
		}
		w.printf("#%d=CLOSED_SHELL('',(%s));\n", w.id, strings.Join(faces, ","))
		w.printf("#%d=MANIFOLD_SOLID_BREP(%s,#%d);\n", w.id+1, name, w.id)
		breps = append(breps, w.id+1)
		w.id += 2
	}
	return breps
}

// hasCavity tells whether one of a shell's pieces faces in: the inside
// of a hollow solid, whose mesh encloses a negative volume.
func hasCavity(pieces []*srf.SShell) bool {
	for _, p := range pieces {
		var m srf.SMesh
		p.TriangulateInto(&m)
		if m.CalculateVolume() < 0 {
			return true
		}
	}
	return false
}

// stepRefs lists ids as a STEP list's items, "#1,#2".
func stepRefs(ids ...int) string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = fmt.Sprintf("#%d", id)
	}
	return strings.Join(out, ",")
}

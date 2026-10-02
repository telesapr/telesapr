package slvs

import (
	"fmt"
	"os"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/persist/link"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// This file ports the .slvs half of linking in solvespace/src/file.cpp:
// LoadEntitiesFromSlvs, and the mesh and shell SaveToFile writes for
// files that link the saved one. The rest is in pkg/persist/link.

// isShellLine reports whether line belongs to the mesh or shell a file
// saves for linking.
func isShellLine(line string) bool {
	for _, prefix := range []string{"Triangle ", "Surface ", "SCtrl ", "TrimBy ", "Curve ", "CCtrl ", "CurvePt "} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return line == "AddSurface" || line == "AddCurve"
}

// ErrCancelled is LoadFile's error when the user cancelled loading at
// a missing linked file.
var ErrCancelled = link.ErrCancelled

// reloadAllLinked loads what the sketch links (C++
// SolveSpaceUI::ReloadAllLinked), reading linked .slvs files too, as
// the C++ comparison tests need. It is false when the user cancelled,
// which canCancel allows.
func (d *Document) reloadAllLinked(saveFile string, canCancel bool) bool {
	return link.ReloadAll(d.Sketch, saveFile, link.Options{CanCancel: canCancel, Fallback: loadEntitiesFromSlvs})
}

// loadEntitiesFromSlvs reads the saved entities, mesh and shell of a
// linked .slvs file, and imports its styles that the sketch doesn't
// have (C++ SolveSpaceUI::LoadEntitiesFromSlvs).
func loadEntitiesFromSlvs(path string, sk *sketch.Sketch, le *[]core.Entity, m *srf.SMesh, sh *srf.SShell) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	linked := &Document{Sketch: sketch.NewSketch(), groupRemap: make(map[core.HGroup][]RemapEntry)}
	if err := linked.parse(data); err != nil || linked.Empty {
		return false
	}
	*le = append((*le)[:0], linked.Sketch.Entities...)
	for _, st := range linked.Sketch.Styles {
		// Linked file contains a style that we don't have yet,
		// so import it.
		if sk.GetStyle(st.H) == nil {
			sk.Styles = append(sk.Styles, st)
		}
	}
	return parseShellLines(linked.shellLines, m, sh) == nil
}

// parseShellLines reads a saved mesh and shell (the Triangle, Surface
// and Curve records of C++ LoadEntitiesFromSlvs).
func parseShellLines(lines []string, m *srf.SMesh, sh *srf.SShell) error {
	var sf srf.SSurface
	var crv srf.SCurve
	for _, line := range lines {
		f := strings.Fields(line)
		num := func(k int) float64 { return parseFloat(f[k]) }
		vec := func(k int) geom.Vector { return geom.Vector{X: num(k), Y: num(k + 1), Z: num(k + 2)} }
		switch {
		case f[0] == "Triangle" && len(f) == 12:
			m.AddTriangle3(srf.STriMeta{
				Face:      srf.FaceID(parseHexUint32(f[1])),
				RgbaColor: core.RgbaColorFromPackedInt(parseHexUint32(f[2])),
			}, vec(3), vec(6), vec(9))
		case f[0] == "Surface" && len(f) == 6:
			sf.H = core.HSSurface{V: parseHexUint32(f[1])}
			sf.Color = core.RgbaColorFromPackedInt(parseHexUint32(f[2]))
			sf.Face = srf.FaceID(parseHexUint32(f[3]))
			sf.DegM, sf.DegN = parseInt(f[4]), parseInt(f[5])
		case f[0] == "SCtrl" && len(f) == 8:
			i, j := parseInt(f[1]), parseInt(f[2])
			sf.Ctrl[i][j] = vec(3)
			sf.Weight[i][j] = num(7)
		case f[0] == "TrimBy" && len(f) == 9:
			sf.Trim = append(sf.Trim, srf.STrimBy{
				Curve:     core.HSCurve{V: parseHexUint32(f[1])},
				Backwards: parseInt(f[2]) != 0,
				StartP:    vec(3),
				FinishP:   vec(6),
			})
		case f[0] == "AddSurface":
			sf.TrimBuilt = true
			sh.Surface = append(sh.Surface, sf)
			sf = srf.SSurface{}
		case f[0] == "Curve" && len(f) == 6:
			crv.H = core.HSCurve{V: parseHexUint32(f[1])}
			crv.IsExact = parseInt(f[2]) != 0
			crv.Exact.Degree = parseInt(f[3])
			crv.SurfA = core.HSSurface{V: parseHexUint32(f[4])}
			crv.SurfB = core.HSSurface{V: parseHexUint32(f[5])}
		case f[0] == "CCtrl" && len(f) == 7:
			i := parseInt(f[1])
			crv.Exact.Ctrl[i] = vec(2)
			crv.Exact.Weight[i] = num(6)
		case f[0] == "CurvePt" && len(f) == 5:
			crv.Points = append(crv.Points, srf.SCurvePt{Vertex: parseInt(f[1]) != 0, P: vec(2)})
		case f[0] == "AddCurve":
			sh.Curve = append(sh.Curve, crv)
			crv = srf.SCurve{}
		default:
			return fmt.Errorf("unexpected shell record %q", line)
		}
	}
	return nil
}

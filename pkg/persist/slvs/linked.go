package slvs

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// This file ports the linking half of solvespace/src/file.cpp:
// ReloadAllLinked, LoadEntitiesFromSlvs, and the mesh and shell
// SaveToFile writes for files that link the saved one.

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

// linkPath resolves a group's link path, which the file stores
// relative to itself (C++ 'P' fields).
func linkPath(saveFile, link string) string {
	if filepath.IsAbs(link) {
		return link
	}
	return filepath.Join(filepath.Dir(saveFile), filepath.FromSlash(link))
}

// reloadAllLinked loads the entities, mesh and shell of every file the
// sketch links (C++ SolveSpaceUI::ReloadAllLinked). C++ asks the user
// to find a missing file; here it counts as declined, so the geometry
// that depends on it is pruned when the sketch is regenerated. Only
// .slvs files can be linked; STL and IDF import aren't ported.
func (d *Document) reloadAllLinked(saveFile string) {
	for i := range d.Sketch.Groups {
		g := &d.Sketch.Groups[i]
		if g.Type != sketch.GroupLinked {
			continue
		}

		g.ImpEntity = nil
		g.ImpMesh.Clear()
		g.ImpShell.Clear()

		if loadEntitiesFromSlvs(linkPath(saveFile, g.LinkFile), d.Sketch, &g.ImpEntity, &g.ImpMesh, &g.ImpShell) {
			// C++ also loads the linked file's images here, which only
			// matter for drawing.
			if g.IsTriangleMeshAssembly() {
				g.ForceToMesh = true
			}
		}
	}

	for i := range d.Sketch.Requests {
		r := &d.Sketch.Requests[i]
		if r.Type != sketch.RequestImage {
			continue
		}
		reloadLinkedImage(d.Sketch, saveFile, r.File)
	}
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
	if err := linked.parse(data); err != nil {
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

// writeShell writes a mesh and shell as C++ SaveToFile does, for files
// that link this one.
func writeShell(b *strings.Builder, m *srf.SMesh, s *srf.SShell) {
	co := func(v geom.Vector) string { return fmt.Sprintf("%.20f %.20f %.20f", v.X, v.Y, v.Z) }
	for _, tr := range m.Triangles {
		fmt.Fprintf(b, "Triangle %08x %08x %s  %s  %s\n",
			uint32(tr.Meta.Face), tr.Meta.RgbaColor.ToPackedInt(), co(tr.A), co(tr.B), co(tr.C))
	}

	for i := range s.Surface {
		sf := &s.Surface[i]
		fmt.Fprintf(b, "Surface %08x %08x %08x %d %d\n",
			sf.H.V, sf.Color.ToPackedInt(), uint32(sf.Face), sf.DegM, sf.DegN)
		for i := 0; i <= sf.DegM; i++ {
			for j := 0; j <= sf.DegN; j++ {
				fmt.Fprintf(b, "SCtrl %d %d %s Weight %20.20f\n", i, j, co(sf.Ctrl[i][j]), sf.Weight[i][j])
			}
		}

		for _, stb := range sf.Trim {
			backwards := 0
			if stb.Backwards {
				backwards = 1
			}
			fmt.Fprintf(b, "TrimBy %08x %d %s  %s\n", stb.Curve.V, backwards, co(stb.StartP), co(stb.FinishP))
		}

		b.WriteString("AddSurface\n")
	}
	for i := range s.Curve {
		sc := &s.Curve[i]
		exact := 0
		if sc.IsExact {
			exact = 1
		}
		fmt.Fprintf(b, "Curve %08x %d %d %08x %08x\n", sc.H.V, exact, sc.Exact.Degree, sc.SurfA.V, sc.SurfB.V)

		if sc.IsExact {
			for i := 0; i <= sc.Exact.Degree; i++ {
				fmt.Fprintf(b, "CCtrl %d %s Weight %.20f\n", i, co(sc.Exact.Ctrl[i]), sc.Exact.Weight[i])
			}
		}
		for _, scpt := range sc.Points {
			vertex := 0
			if scpt.Vertex {
				vertex = 1
			}
			fmt.Fprintf(b, "CurvePt %d %s\n", vertex, co(scpt.P))
		}

		b.WriteString("AddCurve\n")
	}
}

// reloadLinkedImage loads the size of image file, named relative to
// saveFile, into the sketch's images (C++
// SolveSpaceUI::ReloadLinkedImage). C++ asks the user to find a
// missing or unreadable file; here that counts as declined, and the
// file is recorded as absent.
func reloadLinkedImage(sk *sketch.Sketch, saveFile, file string) {
	if file == "" {
		return
	}
	if _, ok := sk.Images[file]; ok {
		return
	}
	if sk.Images == nil {
		sk.Images = make(map[string]*sketch.Image)
	}

	var img *sketch.Image
	if f, err := os.Open(linkPath(saveFile, file)); err == nil {
		if cfg, err := png.DecodeConfig(f); err == nil {
			img = &sketch.Image{Width: cfg.Width, Height: cfg.Height}
		}
		f.Close()
	}
	sk.Images[file] = img
}

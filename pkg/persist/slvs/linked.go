package slvs

import (
	"errors"
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

// LocateResponse is what the user answered when a linked file was
// missing (C++ LocateImportedFile and the file dialog after it).
type LocateResponse int

const (
	// LocateNo: the file stays missing; what depends on it is pruned.
	LocateNo LocateResponse = iota
	// LocateYes: the file was found at the path returned with it.
	LocateYes
	// LocateCancel: stop loading.
	LocateCancel
	// LocateGaveUp: the user wanted to find the file but chose none,
	// where cancelling isn't allowed; nothing is remembered about it.
	LocateGaveUp
)

// LocateMissingFile, when set, asks the user to find a linked file or
// image that couldn't be read, named relative to the sketch's file,
// and returns where it is now (C++ LocateImportedFile and the file
// dialogs of ReloadAllLinked and ReloadLinkedImage). Without it a
// missing file counts as declined. canCancel says whether loading can
// be given up; it can when a file is being opened, not when the sketch
// is regenerated or undone.
var LocateMissingFile func(name string, isImage, canCancel bool) (path string, resp LocateResponse)

// ErrCancelled is LoadFile's error when the user cancelled loading at
// a missing linked file.
var ErrCancelled = errors.New("slvs: loading cancelled")

// relativeLink names a file the user chose as the sketch's file links
// it: relative to the sketch's folder when it can be, as C++ writes
// its 'P' fields; else absolute.
func relativeLink(saveFile, chosen string) string {
	abs, err := filepath.Abs(chosen)
	if err != nil {
		return chosen
	}
	if saveFile == "" {
		return abs
	}
	dir, err := filepath.Abs(filepath.Dir(saveFile))
	if err != nil {
		return abs
	}
	rel, err := filepath.Rel(dir, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

// reloadAllLinked loads the entities, mesh and shell of every file the
// sketch links (C++ SolveSpaceUI::ReloadAllLinked), asking the user to
// find a missing one through LocateMissingFile. It is false when the
// user cancelled, which canCancel allows.
func (d *Document) reloadAllLinked(saveFile string, canCancel bool) bool {
	// If we prompted for this specific file before, don't ask again.
	linkMap := make(map[string]string)

	for i := range d.Sketch.Groups {
		g := &d.Sketch.Groups[i]
		if g.Type != sketch.GroupLinked {
			continue
		}

		g.ImpEntity = nil
		g.ImpMesh.Clear()
		g.ImpShell.Clear()

		if found, ok := linkMap[g.LinkFile]; ok {
			g.LinkFile = found
		}

		for {
			linkFile := linkPath(saveFile, g.LinkFile)
			if loadEntitiesFromFile(linkFile, d.Sketch, &g.ImpEntity, &g.ImpMesh, &g.ImpShell) {
				// We loaded the data, good. Now import its dependencies
				// as well.
				for j := range g.ImpEntity {
					e := &g.ImpEntity[j]
					if e.Type != core.EntityImage {
						continue
					}
					if !reloadLinkedImage(d.Sketch, linkFile, &e.File, canCancel) {
						return false
					}
				}
				if g.IsTriangleMeshAssembly() {
					g.ForceToMesh = true
				}
				break
			}
			if _, asked := linkMap[g.LinkFile]; asked {
				// User was already asked to and refused to locate a
				// missing linked file.
				break
			}
			// The file was moved; prompt the user for its new location.
			path, resp := locateMissingFile(g.LinkFile, false, canCancel)
			if resp == LocateYes {
				found := relativeLink(saveFile, path)
				linkMap[g.LinkFile] = found
				g.LinkFile = found
				continue
			}
			if resp == LocateCancel {
				return false
			}
			if resp == LocateNo {
				// Geometry will be pruned by GenerateAll().
				linkMap[g.LinkFile] = ""
			}
			break
		}
	}

	for i := range d.Sketch.Requests {
		r := &d.Sketch.Requests[i]
		if r.Type != sketch.RequestImage {
			continue
		}
		if !reloadLinkedImage(d.Sketch, saveFile, &r.File, canCancel) {
			return false
		}
	}
	return true
}

// locateMissingFile asks LocateMissingFile, or declines without one.
func locateMissingFile(name string, isImage, canCancel bool) (string, LocateResponse) {
	if LocateMissingFile == nil {
		return "", LocateNo
	}
	return LocateMissingFile(name, isImage, canCancel)
}

// loadEntitiesFromFile reads the entities, mesh and shell of a linked
// file: an IDF board, an STL mesh, or else a .slvs file (C++
// SolveSpaceUI::LoadEntitiesFromFile).
func loadEntitiesFromFile(path string, sk *sketch.Sketch, le *[]core.Entity, m *srf.SMesh, sh *srf.SShell) bool {
	switch pathExtension(path) {
	case "emn", "EMN":
		return linkIDF(path, sk, le, m, sh)
	case "stl", "STL":
		return linkStl(path, sk, le, m)
	default:
		return loadEntitiesFromSlvs(path, sk, le, m, sh)
	}
}

// pathExtension is what follows the last dot of the file name (C++
// Platform::Path::Extension).
func pathExtension(path string) string {
	if dot := strings.LastIndexByte(path, '.'); dot >= 0 {
		return path[dot+1:]
	}
	return ""
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

// readImage reads a PNG image file, or nil if it can't.
func readImage(path string) *sketch.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	pixels, err := png.Decode(f)
	if err != nil {
		return nil
	}
	b := pixels.Bounds()
	return &sketch.Image{Width: b.Dx(), Height: b.Dy(), Pixels: pixels}
}

// reloadLinkedImage loads the image *file, named relative to saveFile,
// into the sketch's images (C++ SolveSpaceUI::ReloadLinkedImage),
// asking the user to find it through LocateMissingFile when it can't be
// read, and renaming *file to where it was found. It is false when the
// user cancelled, which canCancel allows.
func reloadLinkedImage(sk *sketch.Sketch, saveFile string, file *string, canCancel bool) bool {
	if *file == "" {
		return true
	}
	if _, ok := sk.Images[*file]; ok {
		return true
	}
	if sk.Images == nil {
		sk.Images = make(map[string]*sketch.Image)
	}

	img := readImage(linkPath(saveFile, *file))
	if img == nil {
		// The file was moved; prompt the user for its new location.
		switch path, resp := locateMissingFile(*file, true, canCancel); resp {
		case LocateYes:
			*file = relativeLink(saveFile, path)
			if _, ok := sk.Images[*file]; ok {
				return true
			}
			img = readImage(linkPath(saveFile, *file))
			if img == nil && LocateMissingFile != nil {
				// C++: Error("The image '%s' is corrupted.")
				ImageCorrupted(*file)
			}
		case LocateCancel:
			return false
		case LocateGaveUp:
			if canCancel {
				return false
			}
		}
		// We don't know where the file is, record it as absent.
	}
	sk.Images[*file] = img
	return true
}

// ImageCorrupted, when set, tells the user an image they chose can't be
// read (C++ Error("The image '%s' is corrupted.")).
var ImageCorrupted = func(file string) {}

// ReloadAllLinked reloads the linked files and images of sk, relative
// to saveFile, the file the sketch is saved in or "" (C++
// SolveSpaceUI::ReloadAllLinked).
func ReloadAllLinked(sk *sketch.Sketch, saveFile string) {
	(&Document{Sketch: sk}).reloadAllLinked(saveFile, false)
}

// LoadLinkedImage loads image file, named relative to saveFile, into
// the sketch's images unless it is there already, and reports whether
// the sketch now has its pixels.
func LoadLinkedImage(sk *sketch.Sketch, saveFile, file string) bool {
	if _, ok := sk.Images[file]; !ok {
		if sk.Images == nil {
			sk.Images = make(map[string]*sketch.Image)
		}
		sk.Images[file] = readImage(linkPath(saveFile, file))
	}
	return sk.Images[file] != nil
}

// RebaseLinks rewrites the linked files and images of sk, named relative
// to the file the sketch was in (from, or "" for a new sketch), relative
// to the file it is saved in next (to), as C++ writes its 'P' fields
// relative to the file being saved. It fails when a link can't be made
// relative, such as across Windows volumes, and then changes nothing.
func RebaseLinks(sk *sketch.Sketch, from, to string) (badLink string, err error) {
	rebase := func(link string) (string, error) {
		if link == "" {
			return "", nil
		}
		abs, err := filepath.Abs(linkPath(from, link))
		if err != nil {
			return "", err
		}
		dir, err := filepath.Abs(filepath.Dir(to))
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(dir, abs)
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(rel), nil
	}

	groups := make([]string, len(sk.Groups))
	for i := range sk.Groups {
		rel, err := rebase(sk.Groups[i].LinkFile)
		if err != nil {
			return sk.Groups[i].LinkFile, err
		}
		groups[i] = rel
	}
	requests := make([]string, len(sk.Requests))
	for i := range sk.Requests {
		rel, err := rebase(sk.Requests[i].File)
		if err != nil {
			return sk.Requests[i].File, err
		}
		requests[i] = rel
	}

	for i := range sk.Groups {
		sk.Groups[i].LinkFile = groups[i]
	}
	for i := range sk.Requests {
		old := sk.Requests[i].File
		if old == requests[i] {
			continue
		}
		sk.Requests[i].File = requests[i]
		if img, ok := sk.Images[old]; ok {
			sk.Images[requests[i]] = img
		}
		for j := range sk.Entities {
			if sk.Entities[j].File == old {
				sk.Entities[j].File = requests[i]
			}
		}
	}
	return "", nil
}

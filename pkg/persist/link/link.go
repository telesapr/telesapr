// Package link loads what a sketch links: the files of its link groups
// and the pictures of its image requests. It knows no document format
// of its own: a format registers a Loader for its extension, as
// pkg/persist/tlspr does for .tlspr. IDF boards (.emn), STL and OBJ
// meshes (.stl, .obj) and STEP files (.step, .stp, through pkg/persist/step) are
// built in.
//
// It ports the linking half of solvespace/src/file.cpp:
// ReloadAllLinked, ReloadLinkedImage and LocateImportedFile.
package link

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Loader reads the entities, mesh and shell of a linked file at path
// into le, m and sh, and may add the file's styles to sk. It is false
// when the file can't be read.
type Loader func(path string, sk *sketch.Sketch, le *[]core.Entity, m *srf.SMesh, sh *srf.SShell) bool

// Report is what a loader says about a file it read, for the group
// that links it (sketch.Group.LinkInfo and LinkProblem).
type Report struct {
	Info    []string
	Problem string
}

// ReportingLoader is a Loader that also describes the file, and says
// what of it it could not read.
type ReportingLoader func(path string, sk *sketch.Sketch, le *[]core.Entity, m *srf.SMesh, sh *srf.SShell, rep *Report) bool

var (
	loadersMu sync.RWMutex
	loaders   = map[string]ReportingLoader{
		"emn": quiet(linkIDF),
		"stl": func(path string, sk *sketch.Sketch, le *[]core.Entity, m *srf.SMesh, _ *srf.SShell, _ *Report) bool {
			return linkStl(path, sk, le, m)
		},
		"obj": func(path string, sk *sketch.Sketch, le *[]core.Entity, m *srf.SMesh, _ *srf.SShell, _ *Report) bool {
			return linkObj(path, sk, le, m)
		},
		"step": linkStep,
		"stp":  linkStep,
	}
)

// quiet makes a Loader a ReportingLoader that reports nothing.
func quiet(l Loader) ReportingLoader {
	if l == nil {
		return nil
	}
	return func(path string, sk *sketch.Sketch, le *[]core.Entity, m *srf.SMesh, sh *srf.SShell, _ *Report) bool {
		return l(path, sk, le, m, sh)
	}
}

// Register makes l the loader of linked files with extension ext,
// given without its dot and matched ignoring case.
func Register(ext string, l Loader) {
	RegisterReporting(ext, quiet(l))
}

// RegisterReporting makes l the loader of linked files with extension
// ext, as Register does.
func RegisterReporting(ext string, l ReportingLoader) {
	loadersMu.Lock()
	defer loadersMu.Unlock()
	loaders[strings.ToLower(ext)] = l
}

// loaderFor is the registered loader of path's extension, or nil.
func loaderFor(path string) ReportingLoader {
	loadersMu.RLock()
	defer loadersMu.RUnlock()
	return loaders[strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))]
}

// Path resolves a link, which the file holding it names relative to
// itself (C++ 'P' fields).
func Path(saveFile, link string) string {
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

// ImageCorrupted, when set, tells the user an image they chose can't be
// read (C++ Error("The image '%s' is corrupted.")).
var ImageCorrupted = func(file string) {}

// ErrCancelled is a loader's error when the user cancelled loading at
// a missing linked file.
var ErrCancelled = errors.New("loading cancelled")

// RelativeLink names a file the user chose as the sketch's file links
// it: relative to the sketch's folder when it can be, as C++ writes
// its 'P' fields; else absolute.
func RelativeLink(saveFile, chosen string) string {
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

// Options say how ReloadAll treats what it can't read.
type Options struct {
	// CanCancel lets the user give up loading at a missing file.
	CanCancel bool
	// Quiet treats a missing file as declined without asking through
	// LocateMissingFile, as a batch conversion must.
	Quiet bool
	// Fallback loads a link whose extension has no registered loader;
	// without one such a link counts as unreadable.
	Fallback Loader
}

// ReloadAll loads the entities, mesh and shell of every file the
// sketch links (C++ SolveSpaceUI::ReloadAllLinked), and the pictures of
// its image requests, asking the user to find a missing one through
// LocateMissingFile. It is false when the user cancelled.
func ReloadAll(sk *sketch.Sketch, saveFile string, o Options) bool {
	canCancel := o.CanCancel
	locate := locateMissingFile
	if o.Quiet {
		locate = func(string, bool, bool) (string, LocateResponse) { return "", LocateNo }
	}
	// If we prompted for this specific file before, don't ask again.
	linkMap := make(map[string]string)

	for i := range sk.Groups {
		g := &sk.Groups[i]
		if g.Type != sketch.GroupLinked {
			continue
		}

		g.ImpEntity = nil
		g.ImpMesh.Clear()
		g.ImpShell.Clear()
		g.LinkInfo, g.LinkProblem, g.LinkParts = nil, "", nil

		if found, ok := linkMap[g.LinkFile]; ok {
			g.LinkFile = found
		}

		for {
			linkFile := Path(saveFile, g.LinkFile)
			load := loaderFor(linkFile)
			if load == nil {
				load = quiet(o.Fallback)
			}
			solid := g.LinkAsSolid && IsStep(linkFile)
			if solid {
				load = linkStepSolid
			}
			var rep Report
			if load != nil && cachedLoadAs(load, linkFile, solid, sk, &g.ImpEntity, &g.ImpMesh, &g.ImpShell, &rep) {
				g.LinkInfo, g.LinkProblem = rep.Info, rep.Problem
				g.LinkParts = LinkedParts(saveFile, linkFile)
				// We loaded the data, good. Now import its dependencies
				// as well.
				for j := range g.ImpEntity {
					e := &g.ImpEntity[j]
					if e.Type != core.EntityImage {
						continue
					}
					if !reloadLinkedImage(sk, linkFile, &e.File, canCancel, locate) {
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
			path, resp := locate(g.LinkFile, false, canCancel)
			if resp == LocateYes {
				found := RelativeLink(saveFile, path)
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

	for i := range sk.Requests {
		r := &sk.Requests[i]
		if r.Type != sketch.RequestImage {
			continue
		}
		if !reloadLinkedImage(sk, saveFile, &r.File, canCancel, locate) {
			return false
		}
	}
	return true
}

// ReloadAllLinked reloads the linked files and images of sk, relative
// to saveFile, the file the sketch is saved in or "", without the
// choice to cancel (C++ SolveSpaceUI::ReloadAllLinked). Undo and
// regeneration call it.
func ReloadAllLinked(sk *sketch.Sketch, saveFile string) {
	ReloadAll(sk, saveFile, Options{})
}

// locateMissingFile asks LocateMissingFile, or declines without one.
func locateMissingFile(name string, isImage, canCancel bool) (string, LocateResponse) {
	if LocateMissingFile == nil {
		return "", LocateNo
	}
	return LocateMissingFile(name, isImage, canCancel)
}

// readImage reads a PNG image file, or nil if it can't.
func readImage(path string) *sketch.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
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
func reloadLinkedImage(sk *sketch.Sketch, saveFile string, file *string, canCancel bool,
	locate func(name string, isImage, canCancel bool) (string, LocateResponse)) bool {
	if *file == "" {
		return true
	}
	if _, ok := sk.Images[*file]; ok {
		return true
	}
	if sk.Images == nil {
		sk.Images = make(map[string]*sketch.Image)
	}

	img := readImage(Path(saveFile, *file))
	if img == nil {
		// The file was moved; prompt the user for its new location.
		switch path, resp := locate(*file, true, canCancel); resp {
		case LocateYes:
			*file = RelativeLink(saveFile, path)
			if _, ok := sk.Images[*file]; ok {
				return true
			}
			img = readImage(Path(saveFile, *file))
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

// LoadLinkedImage loads image file, named relative to saveFile, into
// the sketch's images unless it is there already, and reports whether
// the sketch now has its pixels.
func LoadLinkedImage(sk *sketch.Sketch, saveFile, file string) bool {
	if _, ok := sk.Images[file]; !ok {
		if sk.Images == nil {
			sk.Images = make(map[string]*sketch.Image)
		}
		sk.Images[file] = readImage(Path(saveFile, file))
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
		abs, err := filepath.Abs(Path(from, link))
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

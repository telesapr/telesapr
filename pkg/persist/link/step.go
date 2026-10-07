package link

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/step"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A STEP file links as a reference mesh: its faces tessellated, with
// a point entity at each vertex and at the centre of each circular
// edge, so constraints can attach to it.

// maxStepPoints bounds the point entities of one STEP file, as STL
// links bound theirs.
const maxStepPoints = 15000

// stepKey names a tessellation of a file: undo and regeneration reload
// links, and a large STEP file takes long to read.
type stepKey struct {
	path  string
	size  int64
	mtime time.Time
	pct   float64
}

type stepEntry struct {
	res *step.Result
	err error
	use int
}

var (
	stepCacheMu sync.Mutex
	stepCache   = map[stepKey]*stepEntry{}
	stepUses    int
)

// stepCacheSize is how many tessellations are kept.
const stepCacheSize = 4

// readStep reads a STEP file, or takes its tessellation from the cache
// when the file and the chord tolerance are unchanged, in memory or on
// disk (stepdisk.go).
func readStep(path string, pct float64) (*step.Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	key := stepKey{path: abs, size: info.Size(), mtime: info.ModTime(), pct: pct}
	stepCacheMu.Lock()
	defer stepCacheMu.Unlock()
	stepUses++
	if e, ok := stepCache[key]; ok {
		e.use = stepUses
		return e.res, e.err
	}
	res, kept := readStepDisk(key)
	if !kept {
		res, err = step.ReadFile(path, step.Options{ChordTolPct: pct})
		if errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		if err == nil {
			writeStepDisk(key, res)
		}
	}
	stepCache[key] = &stepEntry{res: res, err: err, use: stepUses}
	if len(stepCache) > stepCacheSize {
		keys := make([]stepKey, 0, len(stepCache))
		for k := range stepCache {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b stepKey) int { return cmp.Compare(stepCache[a].use, stepCache[b].use) })
		delete(stepCache, keys[0])
	}
	return res, err
}

// linkStep reads a STEP file into mesh m and point entities el. A
// file that is missing or can't be opened counts as missing; one that
// can't be parsed, or holds nothing this reader supports, links as an
// empty mesh, with the reason in the report.
func linkStep(path string, sk *sketch.Sketch, el *[]core.Entity, m *srf.SMesh, _ *srf.SShell, rep *Report) bool {
	*el = (*el)[:0]
	res, err := readStep(path, sk.ChordTolPct)
	if err != nil {
		if _, ok := errors.AsType[*fs.PathError](err); ok {
			return false
		}
		rep.Problem = fmt.Sprintf("the file can't be read: %v", err)
		return true
	}
	m.MakeFromCopyOf(&res.Mesh)
	r := &res.Report
	shapes := plural(r.Shapes, "shape")
	if r.Placements != r.Shapes {
		shapes += fmt.Sprintf(" placed %s", plural(r.Placements, "time"))
	}
	rep.Info = []string{
		fmt.Sprintf("STEP %s, in %s", step.Protocol(r.Schema), r.Unit),
		fmt.Sprintf("%s, %s, %s", shapes, plural(r.Faces, "face"), plural(r.Triangles, "triangle")),
		fmt.Sprintf("chord tolerance %.3g mm", r.ChordTol),
	}
	rep.Problem = r.Problem()

	id := 1
	origin := newPoint(el, &id, geom.Vector{})
	newNormal(el, &id, geom.QuaternionFromBasis(geom.Vector{X: 1}, geom.Vector{Y: 1}), origin)
	newNormal(el, &id, geom.QuaternionFromBasis(geom.Vector{Y: 1}, geom.Vector{Z: 1}), origin)
	newNormal(el, &id, geom.QuaternionFromBasis(geom.Vector{Z: 1}, geom.Vector{X: 1}), origin)
	n := 0
	for _, pts := range [][]geom.Vector{res.Vertices, res.Centers} {
		for _, p := range pts {
			if n >= maxStepPoints {
				break
			}
			newPoint(el, &id, p)
			n++
		}
	}
	if n < len(res.Vertices)+len(res.Centers) {
		rep.Info = append(rep.Info, fmt.Sprintf("points for the first %d of %d vertices and centres",
			n, len(res.Vertices)+len(res.Centers)))
	}
	return true
}

// plural is "1 face" or "n faces".
func plural(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}

// IsStep reports whether path names a STEP file, by its extension.
func IsStep(path string) bool {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "step", "stp":
		return true
	}
	return false
}

// linkStepSolid reads a STEP file as a solid (roadmap D3): its points
// as linkStep gives them, and its shapes as one exact shell; a file
// that can't be read as one links as linkStep links it, with the reason
// in the report.
func linkStepSolid(path string, sk *sketch.Sketch, el *[]core.Entity, m *srf.SMesh, sh *srf.SShell, rep *Report) bool {
	if !linkStep(path, sk, el, m, sh, rep) {
		return false
	}
	// At the tolerance the mesh was made at: from the part's own size
	// and the sketch's chord tolerance, so that it is the same whenever
	// the part is read.
	res, err := readStep(path, sk.ChordTolPct)
	if err != nil {
		return false
	}
	solid, faces, err := step.SolidFile(path, srf.Tol(res.Report.ChordTol, sk.SegmentLimit()))
	if err != nil {
		why := fmt.Sprintf("read as a mesh, not a solid: %v", err)
		if rep.Problem != "" {
			why = rep.Problem + "; " + why
		}
		rep.Problem = why
		return true
	}
	m.Clear()
	*sh = *solid
	rep.Info = append(rep.Info, fmt.Sprintf("read as a solid, %s exact", plural(faces, "face")))
	return true
}

package link

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// What a loader read, kept for the next time the same file is linked
// (roadmap 17.4, G4): a part linked many times is read once, and the
// reloads of undo and regeneration read nothing that hasn't changed. A
// file is the same while its size and modification time are; a STEP
// file's tessellation also depends on the chord tolerance. Each group
// gets its own copy, since a group changes what it holds. A file that
// can't be read is never kept, so that it is looked for again.

type loadKey struct {
	path  string
	size  int64
	mtime time.Time
	pct   float64
	// solid is set for a STEP file read as a solid.
	solid bool
}

type loadEntry struct {
	ok     bool
	ents   []core.Entity
	mesh   srf.SMesh
	shell  srf.SShell
	rep    Report
	styles []core.Style
	use    uint64
}

var (
	loadMu    sync.Mutex
	loadCache = map[loadKey]*loadEntry{}
	loadUses  uint64
	// filesRead counts the files a loader read, not taken from the
	// cache; the tests read it.
	filesRead atomic.Int64
)

// loadCacheSize is how many files are kept.
const loadCacheSize = 64

// cachedLoadAs is load(path, ...), or what it gave for the file before,
// a file read as a solid kept apart from the same file read as a mesh.
func cachedLoadAs(load ReportingLoader, path string, solid bool, sk *sketch.Sketch, le *[]core.Entity, m *srf.SMesh, sh *srf.SShell, rep *Report) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return load(path, sk, le, m, sh, rep)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	key := loadKey{path: abs, size: info.Size(), mtime: info.ModTime(), pct: sk.ChordTolPct, solid: solid}

	loadMu.Lock()
	loadUses++
	e, hit := loadCache[key]
	if hit {
		e.use = loadUses
	}
	loadMu.Unlock()
	if hit {
		*le = append((*le)[:0], e.ents...)
		m.Triangles = append(m.Triangles[:0], e.mesh.Triangles...)
		*sh = e.shell.Clone()
		*rep = Report{Info: append([]string(nil), e.rep.Info...), Problem: e.rep.Problem}
		for _, st := range e.styles {
			if sk.Style(st.H) == nil {
				sk.Styles = append(sk.Styles, st)
			}
		}
		return e.ok
	}

	filesRead.Add(1)
	before := len(sk.Styles)
	ok := load(path, sk, le, m, sh, rep)
	if !ok {
		return false
	}
	e = &loadEntry{ok: ok, ents: append([]core.Entity(nil), (*le)...),
		rep:    Report{Info: append([]string(nil), rep.Info...), Problem: rep.Problem},
		styles: append([]core.Style(nil), sk.Styles[before:]...)}
	e.mesh.Triangles = append([]srf.STriangle(nil), m.Triangles...)
	e.shell = sh.Clone()

	loadMu.Lock()
	defer loadMu.Unlock()
	e.use = loadUses
	loadCache[key] = e
	if len(loadCache) > loadCacheSize {
		keys := make([]loadKey, 0, len(loadCache))
		for k := range loadCache {
			keys = append(keys, k)
		}
		slices.SortFunc(keys, func(a, b loadKey) int { return cmp.Compare(loadCache[a].use, loadCache[b].use) })
		delete(loadCache, keys[0])
	}
	return true
}

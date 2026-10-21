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
// file's tessellation also depends on the chord tolerance, and a model
// regenerated with values a link passes into it on those values and on
// the files the model links in turn. Each group
// gets its own copy, since a group changes what it holds. A file that
// can't be read is never kept, so that it is looked for again.

type loadKey struct {
	path  string
	size  int64
	mtime time.Time
	pct   float64
	// solid is set for a STEP file read as a solid.
	solid bool
	// overrides is what a link passes into a model it regenerates
	// (sketch.Group.LinkOverridesKey), "" for a file read as saved.
	overrides string
}

// stamp is a file as the cache saw it: the same while its size and
// modification time are.
type stamp struct {
	path  string
	size  int64
	mtime time.Time
}

func stampOf(path string) (stamp, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return stamp{}, false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return stamp{path: abs, size: info.Size(), mtime: info.ModTime()}, true
}

// current reports whether the file is as it was.
func (s stamp) current() bool {
	now, ok := stampOf(s.path)
	return ok && now == s
}

type loadEntry struct {
	ok     bool
	ents   []core.Entity
	mesh   srf.SMesh
	shell  srf.SShell
	rep    Report
	styles []core.Style
	// deps are the files a regenerated model linked in turn, which its
	// result depends on as well as on its own file.
	deps []stamp
	use  uint64
}

// loadFunc is a loader that also notes the other files what it read
// depends on, in deps.
type loadFunc func(path string, sk *sketch.Sketch, m *srf.SMesh, sh *srf.SShell, rep *Report, deps *[]stamp) ([]core.Entity, bool)

// plain is a loader of a file that depends on no other.
func plain(l ReportingLoader) loadFunc {
	return func(path string, sk *sketch.Sketch, m *srf.SMesh, sh *srf.SShell, rep *Report, _ *[]stamp) ([]core.Entity, bool) {
		return l(path, sk, m, sh, rep)
	}
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

// cachedLoadAs is load(path, ...), or what it gave for the file before:
// a file read as a solid is kept apart from the same file read as a
// mesh, and a model regenerated with values passed into it (overrides)
// apart from the model as saved and with other values. The files the
// result depends on are added to ctx's.
func cachedLoadAs(load loadFunc, path string, solid bool, overrides string, sk *sketch.Sketch, m *srf.SMesh, sh *srf.SShell, rep *Report, ctx *loadCtx) ([]core.Entity, bool) {
	st, ok := stampOf(path)
	if !ok {
		return load(path, sk, m, sh, rep, nil)
	}
	key := loadKey{path: st.path, size: st.size, mtime: st.mtime, pct: sk.ChordTolPct, solid: solid, overrides: overrides}

	loadMu.Lock()
	loadUses++
	e, hit := loadCache[key]
	if hit && !slices.ContainsFunc(e.deps, func(d stamp) bool { return !d.current() }) {
		e.use = loadUses
	} else {
		hit = false
	}
	loadMu.Unlock()
	if hit {
		ents := append([]core.Entity(nil), e.ents...)
		m.Triangles = append(m.Triangles[:0], e.mesh.Triangles...)
		*sh = e.shell.Clone()
		*rep = e.rep.clone()
		for _, st := range e.styles {
			if sk.Style(st.H) == nil {
				sk.Styles = append(sk.Styles, st)
			}
		}
		ctx.depend(st, e.deps)
		return ents, e.ok
	}

	filesRead.Add(1)
	before := len(sk.Styles)
	var deps []stamp
	ents, ok := load(path, sk, m, sh, rep, &deps)
	if !ok {
		return nil, false
	}
	e = &loadEntry{ok: ok, ents: append([]core.Entity(nil), ents...),
		rep:    rep.clone(),
		styles: append([]core.Style(nil), sk.Styles[before:]...),
		deps:   deps}
	e.mesh.Triangles = append([]srf.STriangle(nil), m.Triangles...)
	e.shell = sh.Clone()
	ctx.depend(st, deps)

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
	return ents, true
}

// depend notes that what is being read depends on the file st and on
// the files deps.
func (c *loadCtx) depend(st stamp, deps []stamp) {
	if c == nil || c.deps == nil {
		return
	}
	*c.deps = append(*c.deps, st)
	*c.deps = append(*c.deps, deps...)
}

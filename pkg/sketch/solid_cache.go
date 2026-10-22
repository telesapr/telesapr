package sketch

import (
	"container/list"
	"encoding/binary"
	"hash/maphash"
	"math"
	"reflect"
	"sync"
	"unsafe"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Solids kept by what they were made from (item 17 of
// doc/notes/modelling_upgrades.md). A regeneration makes each group's
// solid from the group's definition, its entities and params, those of
// the groups before it, the solids before it and the tolerance. Those
// inputs are hashed, group by group in order, into a key; the solid made
// is kept under its key, and a group whose key is one kept takes a copy
// of that solid in place of making it again. An undo, a redo, an edit
// taken back or a configuration chosen again then costs the solve and
// the copies, not the Booleans; so does the full regeneration a
// ShellJob runs for the window, whose copy of the sketch shares the
// sketch's cache.
//
// A key is truthful: a group's solidKey names the inputs of the solid
// it holds, and its prefixKey the definitions, entities and params of
// it and the groups before it as they were when that was computed. A
// key that can't be known is zero, and nothing is kept or looked up
// under it.

// solidKey identifies what a solid was made from.
type solidKey [2]uint64

// known reports whether k names inputs at all.
func (k solidKey) known() bool { return k != solidKey{} }

// solidResult is what making a group's solid leaves on the group.
type solidResult struct {
	thisShell, runningShell srf.SShell
	thisMesh, runningMesh   srf.SMesh
	booleanFailed           bool
	problem                 GroupProblem
	tol                     srf.Tolerance
	fittedPath              *topo.SweepPath
	sweepClosed             bool
	// size is about how many bytes the copy holds.
	size int64
}

// result is a copy of the solid g holds, sharing nothing a later
// operation on it could change.
func (g *Group) result() *solidResult {
	r := &solidResult{
		thisShell:     g.ThisShell.ExactCopy(),
		runningShell:  g.RunningShell.ExactCopy(),
		thisMesh:      g.ThisMesh.ExactCopy(),
		runningMesh:   g.RunningMesh.ExactCopy(),
		booleanFailed: g.BooleanFailed,
		problem:       g.Problem,
		tol:           g.solidTol,
		fittedPath:    g.fittedPath,
		sweepClosed:   g.sweepClosed,
	}
	r.size = shellSize(&r.thisShell) + shellSize(&r.runningShell) +
		int64(len(r.thisMesh.Triangles)+len(r.runningMesh.Triangles))*int64(unsafe.Sizeof(srf.STriangle{}))
	return r
}

// setResult gives g a copy of the solid r holds, as making it would
// have left it.
func (g *Group) setResult(r *solidResult) {
	g.ThisShell = r.thisShell.ExactCopy()
	g.RunningShell = r.runningShell.ExactCopy()
	g.ThisMesh = r.thisMesh.ExactCopy()
	g.RunningMesh = r.runningMesh.ExactCopy()
	g.BooleanFailed = r.booleanFailed
	g.Problem = r.problem
	g.solidTol = r.tol
	g.fittedPath = r.fittedPath
	g.sweepClosed = r.sweepClosed
	g.forgetTopology()
	g.DisplayDirty = true
}

// shellSize is about how many bytes a shell holds.
func shellSize(s *srf.SShell) int64 {
	n := int64(len(s.Surface))*int64(unsafe.Sizeof(srf.SSurface{})) + int64(len(s.Curve))*int64(unsafe.Sizeof(srf.SCurve{}))
	for i := range s.Surface {
		n += int64(len(s.Surface[i].Trim)) * int64(unsafe.Sizeof(srf.STrimBy{}))
	}
	for i := range s.Curve {
		n += int64(len(s.Curve[i].Points)) * int64(unsafe.Sizeof(srf.SCurvePt{}))
	}
	return n
}

// DefaultSolidCacheBytes is about how much a sketch's solid cache holds
// before it forgets the solids used longest ago.
const DefaultSolidCacheBytes = 256 << 20

// maxSolidEntries is the most solids the cache holds, however small.
const maxSolidEntries = 4096

// solidCache holds solids by key, the most recently used last. A
// ShellJob's copy of the sketch shares it from another goroutine, so it
// locks; what it holds is never changed, only copied in and out.
type solidCache struct {
	mu           sync.Mutex
	seedA, seedB maphash.Seed
	max, bytes   int64
	entries      map[solidKey]*list.Element
	maxEntries   int
	order        list.List
	hits, misses int
}

// cacheEntry is a solid in the cache's order.
type cacheEntry struct {
	key    solidKey
	result *solidResult
}

func newSolidCache(maxBytes int64) *solidCache {
	return &solidCache{seedA: maphash.MakeSeed(), seedB: maphash.MakeSeed(), max: maxBytes,
		maxEntries: maxSolidEntries, entries: make(map[solidKey]*list.Element)}
}

// get is the solid kept under k.
func (c *solidCache) get(k solidKey) (*solidResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[k]
	if !ok {
		c.misses++
		return nil, false
	}
	c.hits++
	c.order.MoveToBack(e)
	return e.Value.(*cacheEntry).result, true
}

// put keeps r under k, forgetting the solids used longest ago while it
// holds more than its bytes; a solid larger than all of them isn't kept.
func (c *solidCache) put(k solidKey, r *solidResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if r.size > c.max {
		return
	}
	if e, ok := c.entries[k]; ok {
		c.bytes -= e.Value.(*cacheEntry).result.size
		c.order.Remove(e)
	}
	c.entries[k] = c.order.PushBack(&cacheEntry{key: k, result: r})
	c.bytes += r.size
	for c.bytes > c.max || len(c.entries) > c.maxEntries {
		e := c.order.Front()
		old := e.Value.(*cacheEntry)
		c.order.Remove(e)
		delete(c.entries, old.key)
		c.bytes -= old.result.size
	}
}

// solidCache is the sketch's cache, made on first use; nil when
// NoSolidCache is set.
func (sk *Sketch) solidCache() *solidCache {
	if sk.NoSolidCache {
		return nil
	}
	if sk.solids == nil {
		sk.solids = newSolidCache(DefaultSolidCacheBytes)
	}
	return sk.solids
}

// SolidCacheStats are how many solids were taken from the cache and how
// many were looked for there and made.
func (sk *Sketch) SolidCacheStats() (hits, misses int) {
	c := sk.solids
	if c == nil {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses
}

// makeSolid makes g's solid, made from what key names, or takes a copy
// of the one kept under key. A sketch's solid is the solid so far,
// passed through: it is made, not kept.
func (sk *Sketch) makeSolid(g *Group, key solidKey) {
	c := sk.solidCache()
	if g.Type == GroupDrawing3D || g.Type == GroupDrawingWorkplane {
		c = nil
	}
	if c != nil && key.known() {
		if g.solidKey == key && !g.ShellsStale {
			// It holds that solid already.
			return
		}
		if r, ok := c.get(key); ok {
			g.setResult(r)
			g.solidKey = key
			return
		}
	}
	g.GenerateShellAndMesh(sk)
	g.solidKey = key
	if c != nil && key.known() {
		c.put(key, g.result())
	}
}

// keyHash gathers what a key is made of, then hashes it into two
// independent 64-bit hashes, which together name it.
type keyHash struct {
	seedA, seedB maphash.Seed
	buf          []byte
}

func (sk *Sketch) newKeyHash() *keyHash {
	c := sk.solidCache()
	return &keyHash{seedA: c.seedA, seedB: c.seedB, buf: make([]byte, 0, 1024)}
}

func (h *keyHash) u64(v uint64) { h.buf = binary.LittleEndian.AppendUint64(h.buf, v) }

func (h *keyHash) f64(v float64) { h.u64(math.Float64bits(v)) }

func (h *keyHash) boolean(v bool) {
	if v {
		h.u64(1)
	} else {
		h.u64(0)
	}
}

func (h *keyHash) str(s string) {
	h.u64(uint64(len(s)))
	h.buf = append(h.buf, s...)
}

func (h *keyHash) key(k solidKey) { h.u64(k[0]); h.u64(k[1]) }

// slice hashes a slice by where its elements are and how many: an
// empty one is the same wherever its array is.
func (h *keyHash) slice(data unsafe.Pointer, n int) {
	h.u64(uint64(n))
	if n > 0 {
		h.u64(uint64(uintptr(data)))
	}
}

func (h *keyHash) sum() solidKey {
	k := solidKey{maphash.Bytes(h.seedA, h.buf), maphash.Bytes(h.seedB, h.buf)}
	if !k.known() {
		k[0] = 1 // zero means unknown
	}
	return k
}

// value hashes any value by what it holds, field by field, so that a
// field added to a group's definition is in its key without more.
// Pointers, functions and channels are hashed by address: two that
// differ may hold the same, which costs a solid made again, never a
// wrong one taken.
func (h *keyHash) value(v reflect.Value) {
	switch v.Kind() {
	case reflect.Bool:
		h.boolean(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		h.u64(uint64(v.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		h.u64(v.Uint())
	case reflect.Float32, reflect.Float64:
		h.f64(v.Float())
	case reflect.Complex64, reflect.Complex128:
		h.f64(real(v.Complex()))
		h.f64(imag(v.Complex()))
	case reflect.String:
		h.str(v.String())
	case reflect.Array:
		for i := range v.Len() {
			h.value(v.Index(i))
		}
	case reflect.Slice:
		h.u64(uint64(v.Len()))
		for i := range v.Len() {
			h.value(v.Index(i))
		}
	case reflect.Struct:
		for _, f := range v.Fields() {
			h.value(f)
		}
	case reflect.Map:
		// In no order: the entries' own hashes, summed.
		h.u64(uint64(v.Len()))
		var sa, sb uint64
		e := &keyHash{seedA: h.seedA, seedB: h.seedB}
		it := v.MapRange()
		for it.Next() {
			e.buf = e.buf[:0]
			e.value(it.Key())
			e.value(it.Value())
			sa += maphash.Bytes(e.seedA, e.buf)
			sb += maphash.Bytes(e.seedB, e.buf)
		}
		h.u64(sa)
		h.u64(sb)
	case reflect.Interface:
		if v.IsNil() {
			h.u64(0)
			return
		}
		h.str(v.Elem().Type().String())
		h.value(v.Elem())
	case reflect.Pointer, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		h.u64(uint64(v.Pointer()))
	default:
		h.str(v.Kind().String())
	}
}

// entity hashes what an entity is, but its tag and whether its box on
// the screen is up to date, which say nothing of it.
func (h *keyHash) entity(e *core.Entity) {
	h.u64(uint64(e.H.V))
	h.u64(uint64(e.Type))
	h.u64(uint64(e.Group.V))
	h.u64(uint64(e.Workplane.V))
	for _, p := range e.Point {
		h.u64(uint64(p.V))
	}
	h.u64(uint64(e.ExtraPoints))
	h.u64(uint64(e.Normal.V))
	h.u64(uint64(e.Distance.V))
	for _, p := range e.Param {
		h.u64(uint64(p.V))
	}
	h.f64(e.NumPoint.X)
	h.f64(e.NumPoint.Y)
	h.f64(e.NumPoint.Z)
	h.f64(e.NumNormal.W)
	h.f64(e.NumNormal.X)
	h.f64(e.NumNormal.Y)
	h.f64(e.NumNormal.Z)
	h.f64(e.NumDistance)
	h.str(e.Str)
	h.str(e.Font)
	h.str(e.File)
	h.f64(e.AspectRatio)
	h.u64(uint64(e.TimesApplied))
	h.boolean(e.ForceHidden)
	h.f64(e.ActPoint.X)
	h.f64(e.ActPoint.Y)
	h.f64(e.ActPoint.Z)
	h.f64(e.ActNormal.W)
	h.f64(e.ActNormal.X)
	h.f64(e.ActNormal.Y)
	h.f64(e.ActNormal.Z)
	h.f64(e.ActDistance)
	h.boolean(e.ActVisible)
	h.u64(uint64(e.Style.V))
	h.boolean(e.Construction)
	h.f64(e.EdgesChordTol)
}

// globalKey is the start of the keys: what every solid depends on but
// the groups and the tolerance.
func (sk *Sketch) globalKey() solidKey {
	if sk.solidCache() == nil {
		return solidKey{}
	}
	h := sk.newKeyHash()
	h.boolean(sk.CheckClosedContour)
	h.u64(uint64(uintptr(unsafe.Pointer(sk.Fonts))))
	h.u64(uint64(reflect.ValueOf(sk.Images).Pointer()))
	return h.sum()
}

// firstSolidKey is the key of the solid before the first group's:
// none.
func (sk *Sketch) firstSolidKey() solidKey {
	if sk.solidCache() == nil {
		return solidKey{}
	}
	return solidKey{1}
}

// chainKey is the key the groups after g take for the solid so far at
// g: its solidKey, but the same for every group that holds no solid at
// all, as a sketch before any solid, whatever tolerance it was made at.
func (sk *Sketch) chainKey(g *Group) solidKey {
	if g.solidKey.known() && len(g.ThisShell.Surface) == 0 && len(g.RunningShell.Surface) == 0 &&
		len(g.ThisMesh.Triangles) == 0 && len(g.RunningMesh.Triangles) == 0 && !g.ShellsStale {
		return sk.firstSolidKey()
	}
	return g.solidKey
}

// prefixKey is the key of group g's definition, entities and the values
// of params, after prev, the key of the groups before it; zero when
// prev is.
func (sk *Sketch) prefixKey(prev solidKey, g *Group, params []core.HParam) solidKey {
	if !prev.known() || sk.solidCache() == nil {
		return solidKey{}
	}
	h := sk.newKeyHash()
	h.key(prev)
	def := g.defining()
	// What the solid made last left in the definition is not an input.
	def.BooleanFailed = false
	h.value(reflect.ValueOf(&def).Elem())
	h.u64(uint64(g.SuppressedBy.V))
	h.str(g.faceProblem)
	h.u64(uint64(g.PolyError.How))
	if g.PolyError.How != int(core.PolyGood) {
		// Where a profile is bad says so; a good one keeps what was
		// found when it last wasn't.
		h.value(reflect.ValueOf(g.PolyError.NotClosed))
		h.value(reflect.ValueOf(g.PolyError.ErrorPoint))
	}
	h.u64(uint64(g.Solved.How))
	// What a link read, by where it is: reading it again makes another.
	h.slice(unsafe.Pointer(unsafe.SliceData(g.ImpShell.Surface)), len(g.ImpShell.Surface))
	h.slice(unsafe.Pointer(unsafe.SliceData(g.ImpShell.Curve)), len(g.ImpShell.Curve))
	h.slice(unsafe.Pointer(unsafe.SliceData(g.ImpMesh.Triangles)), len(g.ImpMesh.Triangles))
	h.slice(unsafe.Pointer(unsafe.SliceData(g.ImpEntity)), len(g.ImpEntity))
	h.str(g.LinkLoaded)
	h.str(g.LinkProblem)
	h.u64(uint64(len(g.LinkInfo)))
	for _, s := range g.LinkInfo {
		h.str(s)
	}
	for i := range sk.Entities {
		if e := &sk.Entities[i]; e.Group == g.H {
			h.entity(e)
		}
	}
	for _, ph := range params {
		h.u64(uint64(ph.V))
		if p := sk.Param(ph); p != nil {
			h.f64(p.Val)
		}
	}
	return h.sum()
}

// solidKeyOf is the key of the solid of a group whose prefix key is
// prefix, made at the sketch's tolerance after the solid prev names;
// zero when either is unknown.
func (sk *Sketch) solidKeyOf(prefix, prev solidKey) solidKey {
	if !prefix.known() || !prev.known() || sk.solidCache() == nil {
		return solidKey{}
	}
	h := sk.newKeyHash()
	h.key(prefix)
	h.key(prev)
	h.value(reflect.ValueOf(sk.tol))
	return h.sum()
}

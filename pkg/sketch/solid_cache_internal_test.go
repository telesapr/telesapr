package sketch

import (
	"reflect"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// result is a solid of n triangles, about n triangles' size.
func result(n int) *solidResult {
	r := &solidResult{runningMesh: srf.SMesh{Triangles: make([]srf.STriangle, n)}}
	r.size = int64(n) * 100
	return r
}

// The cache forgets the solids used longest ago while it holds more
// than its bytes or its count, keeps none larger than all of them, and
// counts what it found and didn't.
func TestSolidCacheEviction(t *testing.T) {
	t.Parallel()
	c := newSolidCache(1000)
	c.maxEntries = 3
	for i := range 4 {
		c.put(solidKey{uint64(i + 1)}, result(3))
	}
	if _, ok := c.get(solidKey{1}); ok {
		t.Error("the solid used first is kept past the bytes")
	}
	if _, ok := c.get(solidKey{2}); !ok {
		t.Error("a solid within the bytes is forgotten")
	}
	c.put(solidKey{5}, result(1))
	// 2 was used last of the first ones; 3 goes for the count.
	if _, ok := c.get(solidKey{3}); ok {
		t.Error("the solid used longest ago is kept past the count")
	}
	for _, k := range []uint64{2, 4, 5} {
		if _, ok := c.get(solidKey{k}); !ok {
			t.Errorf("solid %d is forgotten", k)
		}
	}
	c.put(solidKey{9}, result(11))
	if _, ok := c.get(solidKey{9}); ok {
		t.Error("a solid larger than the cache is kept")
	}
	c.put(solidKey{4}, result(2))
	if c.bytes != 600 || len(c.entries) != 3 {
		t.Errorf("after one put again: %d bytes in %d solids, want 600 in 3", c.bytes, len(c.entries))
	}
	if c.hits != 4 || c.misses != 3 {
		t.Errorf("%d hits and %d misses, want 4 and 3", c.hits, c.misses)
	}
}

// A solid is taken from the cache as a copy, which the group may change
// without changing the one kept.
func TestSolidCacheCopies(t *testing.T) {
	t.Parallel()
	var g Group
	g.RunningMesh.Triangles = []srf.STriangle{{A: geom.Vector{X: 1}}}
	g.RunningShell.Surface = []srf.SSurface{{DegM: 1, Trim: []srf.STrimBy{{Start: 0.5}}}}
	r := g.result()
	g.RunningMesh.Triangles[0].A.X = 2
	g.RunningShell.Surface[0].Trim[0].Start = 0.25
	if r.runningMesh.Triangles[0].A.X != 1 || r.runningShell.Surface[0].Trim[0].Start != 0.5 {
		t.Error("the solid kept changed with the group's")
	}
	var h Group
	h.setResult(r)
	h.RunningMesh.Triangles[0].A.X = 3
	if r.runningMesh.Triangles[0].A.X != 1 || !h.DisplayDirty {
		t.Error("the solid taken shares the one kept, or isn't to be drawn again")
	}
	if r.size == 0 {
		t.Error("a solid of a triangle and a surface holds nothing")
	}
}

// A key hashes a map's entries whatever their order, and every field of
// an entity but its tag and its box on the screen: a field added to
// core.Entity has to be put in keyHash.entity, or here with why not.
func TestSolidKeyHashing(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sum := func(fn func(h *keyHash)) solidKey {
		h := sk.newKeyHash()
		fn(h)
		return h.sum()
	}
	m1, m2 := map[int]string{}, map[int]string{}
	for i := range 50 {
		m1[i] = "x"
		m2[49-i] = "x"
	}
	if sum(func(h *keyHash) { h.value(reflect.ValueOf(m1)) }) != sum(func(h *keyHash) { h.value(reflect.ValueOf(m2)) }) {
		t.Error("the same map hashes differently")
	}
	m2[3] = "y"
	if sum(func(h *keyHash) { h.value(reflect.ValueOf(m1)) }) == sum(func(h *keyHash) { h.value(reflect.ValueOf(m2)) }) {
		t.Error("maps that differ hash alike")
	}

	unhashed := map[string]bool{"Tag": true, "ScreenBBoxValid": true}
	var e core.Entity
	base := sum(func(h *keyHash) { h.entity(&e) })
	fields := 0
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		for i := range v.NumField() {
			f := v.Type().Field(i)
			if f.Anonymous {
				walk(v.Field(i))
				continue
			}
			fields++
			was := reflect.New(f.Type).Elem()
			was.Set(v.Field(i))
			switch f.Type.Kind() {
			case reflect.Bool:
				v.Field(i).SetBool(true)
			case reflect.Int, reflect.Int64:
				v.Field(i).SetInt(7)
			case reflect.Float64:
				v.Field(i).SetFloat(7)
			case reflect.String:
				v.Field(i).SetString("7")
			case reflect.Struct, reflect.Array:
				// The first number in it.
				w := v.Field(i)
				for w.Kind() == reflect.Struct || w.Kind() == reflect.Array {
					if w.Kind() == reflect.Struct {
						w = w.Field(0)
					} else {
						w = w.Index(0)
					}
				}
				switch w.Kind() {
				case reflect.Uint32:
					w.SetUint(7)
				case reflect.Float64:
					w.SetFloat(7)
				default:
					t.Fatalf("field %s: set what in %s?", f.Name, w.Kind())
				}
			case reflect.Uint32:
				v.Field(i).SetUint(7)
			default:
				t.Fatalf("field %s of kind %s: say how to change it here", f.Name, f.Type.Kind())
			}
			if changed := sum(func(h *keyHash) { h.entity(&e) }) != base; changed == unhashed[f.Name] {
				t.Errorf("entity field %s: in the key %v, want %v", f.Name, changed, !unhashed[f.Name])
			}
			v.Field(i).Set(was)
		}
	}
	walk(reflect.ValueOf(&e).Elem())
	if fields < 25 {
		t.Errorf("only %d fields walked", fields)
	}
}

package api

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"
)

// parts draws a ring turned about z, then n-1 small plates in a row
// far apart, so that the model grows, and its chord tolerance with it:
// the ring, curved both ways, is meshed by the tolerance.
func parts(d *Doc, at Entity, from, n int) {
	for i := from; i < from+n; i++ {
		d.Features.SketchAt(at, V(1, 0, 0), V(0, 0, 1), "")
		if i == 0 {
			d.Sketch.Circle(V(30, 0, 10), 8)
			axis := d.Sketch.Line(V(0, 0, 0), V(0, 0, 20))
			axis.SetConstruction(true)
			d.Features.Lathe(axis, "part0").SetCombine(Assemble)
			continue
		}
		x := float64(i) * 300
		d.Sketch.Polyline(true, V(x, 0, 0), V(x+10, 0, 0), V(x+10, 0, 10), V(x, 0, 10))
		d.Features.Extrude(5, fmt.Sprint("part", i)).SetCombine(Assemble)
	}
}

// readBack saves d and opens it again.
func readBack(t *testing.T, d *Doc) *Doc {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plates.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// TestAddedGroupMeshedAsRead adds groups that grow the model. Adding
// one regenerates it and what follows, not the groups before, unless
// the chord tolerance they were made at has changed: the model in
// memory is meshed as the file read back is.
func TestAddedGroupMeshedAsRead(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.Sketch3D("datums")
	at := d.Sketch.Point(V(0, 0, 0))
	d.Constrain.Lock(at)
	parts(d, at, 0, 5)
	// Measured before the save, which regenerates everything.
	a := d.Active().Analyze()
	triangles, volume := a.Triangles(), a.Volume()
	r := readBack(t, d).Active().Analyze()
	if triangles != r.Triangles() || volume != r.Volume() {
		t.Errorf("made %d triangles, %g mm³; read back %d, %g", triangles, volume, r.Triangles(), r.Volume())
	}
}

// TestAddGroupMidHistory adds a part after an earlier one: the
// groups after it are made again once the last is active.
func TestAddGroupMidHistory(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.Sketch3D("datums")
	at := d.Sketch.Point(V(0, 0, 0))
	d.Constrain.Lock(at)
	parts(d, at, 0, 3)
	last := d.Active()
	d.Activate(d.Group("part1"))
	parts(d, at, 3, 1)
	d.Activate(last)
	a := last.Analyze()
	if lo, hi := a.Bounds(); lo.X > -37 || hi.X < 910 {
		t.Errorf("bounds %v %v", lo, hi)
	}
	triangles, volume := a.Triangles(), a.Volume()
	r := readBack(t, d).Group(last.Name()).Analyze()
	if triangles != r.Triangles() || volume != r.Volume() {
		t.Errorf("made %d triangles, %g mm³; read back %d, %g", triangles, volume, r.Triangles(), r.Volume())
	}
}

// TestEditRemakesStaleMeshes lengthens a plate after a ring until the
// model is many times larger. The edit regenerates the plate, not the
// ring, whose solid was made at the tolerance of a small model; what
// measures the mesh makes it again first, so that it is meshed as when
// the file is read back.
func TestEditRemakesStaleMeshes(t *testing.T) {
	t.Parallel()
	d := New()
	d.Features.Sketch3D("datums")
	at := d.Sketch.Point(V(0, 0, 0))
	d.Constrain.Lock(at)
	parts(d, at, 0, 1)
	d.Features.SketchAt(at, V(1, 0, 0), V(0, 0, 1), "")
	sides := d.Sketch.Polyline(true, V(60, 0, 0), V(70, 0, 0), V(70, 0, 10), V(60, 0, 10))
	length := d.Constrain.Length(sides[0], 10)
	d.Features.Extrude(5, "plate").SetCombine(Assemble)
	length.Set(1000)
	a := d.Active().Analyze()
	if lo, hi := a.Bounds(); hi.X-lo.X < 1000 {
		t.Fatalf("the model is %g wide", hi.X-lo.X)
	}
	triangles, volume := a.Triangles(), a.Volume()
	r := readBack(t, d).Active().Analyze()
	if triangles != r.Triangles() || volume != r.Volume() {
		t.Errorf("made %d triangles, %g mm³; read back %d, %g", triangles, volume, r.Triangles(), r.Volume())
	}
}

// TestSolidMadeOnceWithItsCombine adds an extrusion and sets how it
// combines before anything reads it: it is made once, with that mode,
// as many regenerations as an extrusion read at once costs. A change of
// another group makes it first.
func TestSolidMadeOnceWithItsCombine(t *testing.T) {
	t.Parallel()
	regenerations := func(combine bool) uint64 {
		d := New()
		d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
		sk := d.Host().Sketch()
		before := sk.Generation
		g := d.Features.Extrude(5, "")
		if combine {
			g.SetCombine(Assemble)
			g.SetColor(10, 20, 30)
		}
		if v := g.Analyze().Volume(); math.Abs(v-500) > 1e-6 {
			t.Errorf("volume %g", v)
		}
		return sk.Generation - before
	}
	if alone, with := regenerations(false), regenerations(true); with != alone {
		t.Errorf("%d regenerations with the combine mode set, %d without", with, alone)
	}

	d := New()
	sketch := d.Active()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	g := d.Features.Extrude(5, "")
	sketch.SetVisible(false)
	if p := d.Host().Sketch().AddedPending(); p.V != 0 {
		t.Errorf("group %d still to be made after a change of another", p.V)
	}
	if v := g.Analyze().Volume(); math.Abs(v-500) > 1e-6 {
		t.Errorf("volume %g", v)
	}
}

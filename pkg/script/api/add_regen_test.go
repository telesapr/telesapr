package api

import (
	"fmt"
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

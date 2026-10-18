package api

import (
	"path/filepath"
	"strings"
	"testing"
)

// screwHead draws a shank and, on its end face, the sketch of a head
// with a socket: a sketch whose workplane the default view sees edge
// on, as a screw lying along y has.
func screwHead(t *testing.T) *Doc {
	t.Helper()
	d := New()
	d.Features.Sketch3D("datums")
	shankAt := d.Sketch.Point(V(100, -78.5, 40))
	d.Constrain.Lock(shankAt)
	headAt := d.Sketch.Point(V(100, -88.5, 40))
	d.Constrain.Lock(headAt)
	d.Features.SketchAt(shankAt, V(0, 0, -1), V(1, 0, 0), "")
	d.Sketch.Circle(V(100, -78.5, 40), 1.5)
	d.Features.Extrude(10, "shank")
	d.Features.SketchAt(headAt, V(1, 0, 0), V(0, 0, 1), "")
	d.Sketch.Circle(V(100, -88.5, 40), 2.76)
	d.Sketch.Circle(V(100, -88.5, 40), 1.2)
	return d
}

// TestExtrudeSeededDepth extrudes a sketch seen edge on onto the face
// of the solid so far. Solved from the view's guess, its first solid
// was a sheet in its own plane, whose Boolean panicked; given its depth
// before it is first made, it is the solid a file read back makes.
func TestExtrudeSeededDepth(t *testing.T) {
	t.Parallel()
	d := screwHead(t)
	g := d.Features.Extrude(3, "head")
	if p := g.Problem(); p != "" {
		t.Fatalf("problem %q", p)
	}
	volume, triangles := g.Analyze().Volume(), g.Analyze().Triangles()
	if lo, hi := g.Analyze().Bounds(); !nearVec(lo, V(97.24, -91.5, 37.24)) || !nearVec(hi, V(102.76, -78.5, 42.76)) {
		t.Errorf("bounds %v %v", lo, hi)
	}
	path := filepath.Join(t.TempDir(), "screw.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	read, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	r := read.Group("head").Analyze()
	if r.Volume() != volume || r.Triangles() != triangles {
		t.Errorf("made %g mm³ of %d triangles, read back %g of %d", volume, triangles, r.Volume(), r.Triangles())
	}
}

// TestExtrudeNoDepthOnFace extrudes the head by nothing: a sheet on the
// shank's end face, where the faces meet in curves of no length. The
// Boolean fails and the group says so; it does not panic.
func TestExtrudeNoDepthOnFace(t *testing.T) {
	t.Parallel()
	d := screwHead(t)
	g := d.Features.Extrude(3, "head")
	for i := range 3 {
		g.Param(i).Set(0)
	}
	if p := g.Problem(); !strings.Contains(p, "Boolean") {
		t.Errorf("problem %q", p)
	}
}

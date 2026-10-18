package api

import (
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

package api

import (
	"math"
	"strings"
	"testing"
)

// A plate with a hole: the ray-cast check's volume is the plate's less
// the hole's, every ray agrees with the mesh, and a probe across the
// hole finds the plate on either side of it.
func TestRayCheck(t *testing.T) {
	t.Parallel()
	d := New()
	d.Rect(V(0, 0, 0), V(40, 40, 0))
	d.Extrude(10, "")
	d.SketchIn(d.Workplane("XY"), "")
	d.Circle(V(20, 20, 0), 5)
	hole := d.Extrude(10, "hole")
	hole.SetCombine(Difference)
	r := hole.RayCheck()
	want := 40*40*10 - math.Pi*25*10
	if math.Abs(r.Measure.Volume-want) > 2e-3*want || len(r.Disagree) != 0 || !r.Manifold.Closed() {
		t.Errorf("%+v, want volume %g", r, want)
	}
	in := hole.Probe(V(-10, 20, 5), V(1, 0, 0))
	if len(in) != 2 || math.Abs(in[0].From-10) > 1e-9 || math.Abs(in[0].To-25) > 1e-6 ||
		math.Abs(in[1].From-35) > 1e-6 || math.Abs(in[1].To-50) > 1e-9 {
		t.Errorf("probe %v", in)
	}
	if in := d.Groups()[0].Probe(V(0, 0, 0), V(1, 0, 0)); len(in) != 0 {
		t.Errorf("no solid: %v", in)
	}
}

// A solid the oracle can't build fails with the group's name.
func TestRayCheckUnsupported(t *testing.T) {
	t.Parallel()
	d := New()
	d.Rect(V(0, 0, 0), V(10, 10, 0))
	ex := d.Extrude(10, "")
	ch := d.ChamferSolid(ex.Solid().Edges()[:1], 1, "")
	failsWith(t, "chamfer", func() { ch.RayCheck() })
	failsWith(t, "chamfer", func() { ch.Probe(V(0, 0, 0), V(1, 0, 0)) })
	if !strings.Contains(ch.String(), "chamfer") {
		t.Skip("named otherwise")
	}
}

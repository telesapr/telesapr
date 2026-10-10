package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// An assembly of three blocks (roadmap 16.10): the first made by an
// extrusion, the second assembled overlapping it by 2 x 10 x 10, the
// third assembled touching the second face to face. Only the first two
// interfere, by their shells, with the volume they share.
func TestInterferences(t *testing.T) {
	d := api.New()
	block(d, 0, 0, 10, 10, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(8, 0, 0), api.V(18, 10, 0))
	b := d.Features.Extrude(10, "second")
	b.SetCombine(api.Assemble)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(18, 0, 0), api.V(28, 10, 0))
	c := d.Features.Extrude(10, "third")
	c.SetCombine(api.Assemble)

	found, meshOnly := d.Assembly.Interferences()
	if len(meshOnly) != 0 || len(found) != 1 {
		t.Fatalf("%d interferences, %d mesh-only parts", len(found), len(meshOnly))
	}
	if f := found[0]; f.B.H != b.H || math.Abs(f.Volume-200) > 1e-6 {
		t.Errorf("%s and %s share %g mm³", f.A, f.B, f.Volume)
	}
	// A part linked twice, at one place: all its volume, though every
	// face lies on another.
	l := api.New()
	one, err := l.Assembly.Link("../../testdata/solid/chamfer-parameter.tlspr", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Assembly.Link("../../testdata/solid/chamfer-parameter.tlspr", ""); err != nil {
		t.Fatal(err)
	}
	want := one.Analyze().Check().Volume
	if found, _ := l.Assembly.Interferences(); len(found) != 1 || math.Abs(found[0].Volume-want) > 1e-3*want {
		t.Errorf("the part linked twice: %+v, want %g", found, want)
	}

	// Without the overlapping block, nothing.
	b.Delete()
	if found, _ := d.Assembly.Interferences(); len(found) != 0 {
		t.Errorf("%d interferences without the second block", len(found))
	}
}

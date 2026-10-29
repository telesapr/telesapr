package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A notch's corner: the block 20 by 20 by 10 with the 10 by 10 block cut
// from its far corner, its two top edges at the inside corner rounded at
// 1 and 2, convex, and the upright below them at 1, concave. The upright
// is rounded first and the top edges go on round the arc it leaves, the
// radius going from one's to the other's (pkg/topo, fillet_vary.go); one
// undo step, back the same on redo and reopened.
func TestFilletMixedCornerRadii(t *testing.T) {
	t.Parallel()
	d := api.New()
	fineChord(d, 0.002)
	block(d, 0, 0, 20, 20, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(10, 10, 0), api.V(20, 20, 0))
	cut := d.Features.Extrude(10, "")
	cut.SetCombine(sketch.CombineDifference)
	if p := cut.Problem(); p != "" {
		t.Fatal(p)
	}
	before := cut.Analyze().Check().Volume
	step(d)
	saveBefore := saved(t, d)
	f := d.Features.Fillet([]api.Entity{
		lineAlong(t, cut, api.V(10, 10, 10), api.V(10, 20, 10)),
		lineAlong(t, cut, api.V(10, 10, 10), api.V(20, 10, 10)),
		lineAlong(t, cut, api.V(10, 10, 0), api.V(10, 10, 10)),
	}, 1, "")
	f.AsFillet().SetEdgeRadius(1, 2)
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	r := f.Analyze().Check()
	// Along their 10 mm the convex fillets take (1 - π/4) r² each, 2.1
	// and 8.6 mm³, and the concave one adds 2.1; the corner's share is
	// a small part of it.
	took, want := before-r.Volume, (1-math.Pi/4)*10*(1+4-1)
	if !r.Sound() || math.Abs(took-want) > 0.05*want {
		t.Fatalf("%s, took %g, the edges alone %g", r.String(), took, want)
	}
	undoRedo(t, d, saveBefore)
	// A file keeps no chord tolerance of a test's: reopened at the
	// default.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	d.Edit.Regen()
	r = f.Analyze().Check()
	back := roundTrip(t, d)
	if br := back.Group(f.Name()).Analyze().Check(); !br.Sound() || math.Abs(br.Volume-r.Volume) > 1e-6*r.Volume {
		t.Fatalf("reopened: %s, want volume %g", br.String(), r.Volume)
	}
}

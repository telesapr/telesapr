package api

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestEqualPipeTee unites, and cuts, a pipe of radius 5 with a branch of
// the same radius from its axis out square to it: the two cylinders
// touch tangent where their axes' plane is square to the branch, and the
// Booleans of such a pair all failed (roadmap K3). The shared part is
// half the Steinmetz solid, 8 r³ / 3.
func TestEqualPipeTee(t *testing.T) {
	t.Parallel()
	const r = 5.0
	half := 8 * r * r * r / 3
	pipe, branch := math.Pi*r*r*40, math.Pi*r*r*20
	for _, c := range []struct {
		how  sketch.CombineAs
		want float64
	}{
		{sketch.CombineUnion, pipe + branch - half},
		{sketch.CombineDifference, pipe - half},
	} {
		d := New()
		d.Edit.SetChordTolerance(0.01)
		d.Features.SketchIn(d.Workplane("XY"), "pipe")
		d.Sketch.Circle(V(0, 0, 0), r)
		d.Features.Extrude(40, "")
		d.Features.SketchIn(d.Workplane("YZ"), "branch")
		d.Sketch.Circle(V(0, 0, 20), r)
		g := d.Features.Extrude(20, "")
		g.SetCombine(c.how)
		if p := g.Problem(); p != "" {
			t.Fatalf("%v: %s", c.how, p)
		}
		rep := g.Analyze().Check()
		if !rep.Sound() || math.Abs(rep.Volume-c.want) > 0.002*c.want {
			t.Errorf("%v: sound %v, volume %.1f, want %.1f", c.how, rep.Sound(), rep.Volume, c.want)
		}
	}
}

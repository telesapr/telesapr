package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A narrow conical dimple drilled from a block's top, 0.3 across its
// mouth, its point 0.2 over the floor: the floor is two triangles,
// measured at points about a step of the model's size apart, none of
// whose rays meets the dimple, and the dimple's rays run sideways into
// the block, so no measured ray joins the floor to the dimple's
// triangles. The pairs of triangles that face each other within the
// thinnest found are measured too, and find the point.
func TestWallThicknessUnderADimple(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, -20, 40, 20, 10)
	const cx, cy = 23.9, 0
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(cx, cy, -2), api.V(cx, cy, 22))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(cx, cy, 10), api.V(cx+0.3, cy, 10), api.V(cx, cy, 0.2))
	cut := d.Features.Lathe(axis, "dimple")
	cut.SetCombine(api.Difference)
	if p := cut.Problem(); p != "" {
		t.Fatal(p)
	}
	r := cut.Analyze().WallThickness(0)
	if math.Abs(r.Thinnest.Thickness-0.2) > 1e-6 {
		t.Fatalf("thinnest %g at %v, want 0.2 under the dimple's point (step %g)", r.Thinnest.Thickness, r.Thinnest.From, r.Step)
	}
}

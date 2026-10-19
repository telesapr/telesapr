package sketch_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A plate after a disc, lengthened until the model is many times
// larger, leaves the disc's solid at the tolerance of the small model:
// RemakeStaleMeshes regenerates the model once, and then has nothing
// to do.
func TestRemakeStaleMeshes(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Circle(api.V(0, 0, 0), 20)
	d.Features.Extrude(5, "disc")
	d.Features.SketchIn(d.Workplane("XY"), "")
	sides := d.Sketch.Polyline(true, api.V(30, 0, 0), api.V(40, 0, 0), api.V(40, 10, 0), api.V(30, 10, 0))
	length := d.Constrain.Length(sides[0], 10)
	d.Features.Extrude(5, "plate").SetCombine(api.Assemble)
	length.Set(2000)
	sk := d.Host().Sketch()
	sk.Refresh()
	if !sk.RemakeStaleMeshes() {
		t.Error("nothing was made again after the model grew")
	}
	if sk.RemakeStaleMeshes() {
		t.Error("made again a second time")
	}
}

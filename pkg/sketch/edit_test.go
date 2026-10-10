package sketch_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A point drawn in a 3d sketch is free in 3d as its file reads back,
// so a save, a reload and a save agree.
func TestRequestIn3DRoundTrips(t *testing.T) {
	d := api.New()
	d.Features.Sketch3D("")
	d.Sketch.Point(api.V(1, 2, 3))
	d.Sketch.Line(api.V(0, 0, 0), api.V(4, 5, 6))
	roundTrip(t, d)
}

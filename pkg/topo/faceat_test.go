package topo_test

import (
	"math"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A point on the seam between two of a hole's quarter patches lies on
// both, and FaceAt finds one of them, the same one each time.
func TestFaceAtSeam(t *testing.T) {
	t.Parallel()
	tp := build(t, plateWithHole(5, 10, srf.Tolerance{}))
	at := func(deg float64) geom.Vector {
		r := deg * math.Pi / 180
		return v(5*math.Cos(r), 5*math.Sin(r), 5)
	}
	var quarters []int
	for _, a := range []float64{45, 135, 225, 315} {
		f, d := tp.FaceAt(at(a))
		if f < 0 || d > 1e-9 {
			t.Fatalf("at %g°, off the seams: face %d at %g", a, f, d)
		}
		quarters = append(quarters, f)
	}
	for _, a := range []float64{0, 90, 180, 270} {
		f, d := tp.FaceAt(at(a))
		if f < 0 || d > 1e-9 || !slices.Contains(quarters, f) {
			t.Errorf("at %g°: face %d at %g, want one of the hole's %v", a, f, d, quarters)
			continue
		}
		if g, _ := tp.FaceAt(at(a)); g != f {
			t.Errorf("at %g°: face %d, then %d", a, f, g)
		}
	}
}

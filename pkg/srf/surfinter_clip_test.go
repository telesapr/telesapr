package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A quarter of a cylinder of radius 1 along x, from x 0 to 5, meets a
// plane slanted to its axis in an ellipse arc: whole where the plane
// crosses the patch between its ends, the piece up to an end where it
// crosses that end, nothing where it misses the patch.
func TestSpansWithinExtrusion(t *testing.T) {
	t.Parallel()
	arc := SBezierFrom3(geom.Vector{Y: 1}, geom.Vector{Y: 1, Z: 1}, geom.Vector{Z: 1})
	arc.Weight[1] = math.Sqrt2 / 2
	along := geom.Vector{X: 5}
	// The plane x = x0 + k y.
	plane := func(x0, k float64) (geom.Vector, float64) {
		n := geom.Vector{X: 1, Y: -k}
		m := n.Magnitude()
		return n.ScaledBy(1 / m), x0 / m
	}
	for _, c := range []struct {
		name  string
		x0, k float64
		want  int
	}{
		{"across", 2, 0.5, 1},
		{"past the start", -0.5, 1, 1},
		{"past the far end", 4.5, 1, 1},
		{"corner", 0, 1, 1},
		{"missing", 10, 1, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			n, d := plane(c.x0, c.k)
			spans := spansWithinExtrusion(&arc, along, n, d)
			if len(spans) != c.want {
				t.Fatalf("spans %v, want %d of them", spans, c.want)
			}
			for _, sp := range spans {
				for _, end := range sp {
					p := arc.PointAt(end)
					x := c.x0 + c.k*p.Y
					// Each end is the arc's end or on the patch's.
					if end != 0 && end != 1 && math.Abs(x) > 1e-12 && math.Abs(x-5) > 1e-12 {
						t.Errorf("span %v ends at x %g, off the patch's ends", sp, x)
					}
					if x < -1e-12 || x > 5+1e-12 {
						t.Errorf("span %v reaches x %g, past the patch", sp, x)
					}
				}
			}
			if c.name == "across" || c.name == "corner" {
				if spans[0] != [2]float64{0, 1} {
					t.Errorf("a curve within the patch is cut to %v", spans[0])
				}
			}
		})
	}
}

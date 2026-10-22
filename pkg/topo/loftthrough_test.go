package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A loft held square to a profile between its ends: three circles, the
// middle one moved aside, the loft passing it straight up. Its pieces
// meet there tangent, both square to the circle's plane, and its
// horizontal sections are the circle moved, so its volume is the area
// times the height.
func TestSmoothLoftThrough(t *testing.T) {
	t.Parallel()
	loops := [][]srf.SBezier{circleAt(5, 0, 0, 0), circleAt(5, 8, 0, 10), circleAt(5, 0, 0, 20)}
	for _, ends := range []topo.LoftEnds{{}, {Start: 1, End: 0.5}} {
		bss, err := topo.SmoothLoftThroughSurfaces(loops, 0, ends, topo.LoftSides{}, []float64{1})
		if err != nil {
			t.Fatal(err)
		}
		if len(bss) != 2 {
			t.Fatalf("%d pieces, want 2", len(bss))
		}
		for _, u := range []float64{0, 0.3, 0.6} {
			a := bss[0].PointAt(u, 1).Minus(bss[0].PointAt(u, 1-1e-6))
			b := bss[1].PointAt(u, 1e-6).Minus(bss[1].PointAt(u, 0))
			for _, d := range []struct {
				name string
				x, y float64
				mag  float64
			}{{"reaching", a.X, a.Y, a.Magnitude()}, {"leaving", b.X, b.Y, b.Magnitude()}} {
				if math.Hypot(d.x, d.y) > 1e-4*d.mag {
					t.Errorf("at u %g, %s the middle circle the loft isn't square to it: %v, %v", u, d.name, a, b)
				}
			}
			if !bss[0].PointAt(u, 1).Equals(bss[1].PointAt(u, 0)) {
				t.Errorf("at u %g the pieces don't meet", u)
			}
		}
		sh, err := topo.MakeSmoothLoftThrough(loops, 0, ends, topo.LoftSides{}, []float64{1},
			core.NewRGBA(1, 1, 1, 255), 1<<30, srf.Tol(0.002, 30))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := sound(t, sh), math.Pi*25*20; math.Abs(got-want) > 0.003*want {
			t.Errorf("volume %g, want %g", got, want)
		}
	}
	// None set is the smooth loft, one surface.
	if bss, err := topo.SmoothLoftThroughSurfaces(loops, 0, topo.LoftEnds{}, topo.LoftSides{}, []float64{0}); err != nil || len(bss) != 1 {
		t.Errorf("no middle tangency gave %d surfaces, %v", len(bss), err)
	}
	if _, err := topo.SmoothLoftThroughSurfaces(loops, 0, topo.LoftEnds{}, topo.LoftSides{}, []float64{1, 1}); err == nil {
		t.Error("two middle tangencies for one profile between the ends")
	}
	if _, err := topo.SmoothLoftThroughSurfaces(loops, 0, topo.LoftEnds{}, topo.LoftSides{}, []float64{math.Inf(1)}); err == nil {
		t.Error("an infinite tangency was taken")
	}
}

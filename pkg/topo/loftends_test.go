package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A loft square to its end profiles (roadmap 28.3): two circles, the
// second moved aside, joined by an S that leaves the first and reaches
// the second along their planes' normal; its horizontal sections are
// the circle moved, so its volume is the area times the height.
func TestSmoothLoftEnds(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.002, 20)
	loops := [][]srf.SBezier{circleAt(5, 0, 0, 0), circleAt(5, 10, 0, 20)}
	bs, err := topo.SmoothLoftSurfaceEnds(loops, 0, topo.LoftEnds{Start: 1, End: 1})
	if err != nil {
		t.Fatal(err)
	}
	u0, u1, v0, v1 := bs.Domain()
	for _, u := range []float64{u0, (u0 + u1) / 3, (u0 + u1) / 2} {
		for _, end := range []struct{ v, dv float64 }{{v0, 1e-5}, {v1, -1e-5}} {
			d := bs.PointAt(u, end.v+end.dv).Minus(bs.PointAt(u, end.v))
			if math.Abs(d.X) > 1e-6*d.Magnitude()*100 || math.Abs(d.Y) > 1e-6*d.Magnitude()*100 {
				t.Errorf("at u %g, v %g the loft leaves along %v, not square to the profile", u, end.v, d)
			}
		}
	}
	sh, err := topo.MakeSmoothLoftEnds(loops, 0, topo.LoftEnds{Start: 1, End: 1}, core.NewRGBA(1, 1, 1, 255), 1<<30, tol)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := sound(t, sh), math.Pi*25*20; math.Abs(got-want) > 0.003*want {
		t.Errorf("volume %g, want %g", got, want)
	}
	// A square to a circle: sections of other weights.
	sq := polyLoop(v(-4, -4, 0), v(4, -4, 0), v(4, 4, 0), v(-4, 4, 0))
	sh, err = topo.MakeSmoothLoftEnds([][]srf.SBezier{sq, circleAt(4, 3, 0, 15)}, 0, topo.LoftEnds{Start: 1, End: 1},
		core.NewRGBA(1, 1, 1, 255), 1<<30, tol)
	if err != nil {
		t.Fatal(err)
	}
	sound(t, sh)
	// One end set: the other is as the free loft leaves it.
	if _, err := topo.SmoothLoftSurfaceEnds(loops, 0, topo.LoftEnds{End: 0.5}); err != nil {
		t.Error(err)
	}
	if _, err := topo.SmoothLoftSurfaceEnds(loops, 0, topo.LoftEnds{Start: math.NaN()}); err == nil {
		t.Error("a NaN tangency was taken")
	}
}

// TestSmoothLoftEndsSweep lofts circles and blobs through two or three
// profiles with ends set and free, and requires sound solids whose
// volume is the sections' area times the height. The rate allowed was
// fixed at none before the ends were written.
func TestSmoothLoftEndsSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("profiles", 2, 3),
			sweep.Floats("start", 0, 0.5, 1.5),
			sweep.Floats("end", 0, 1),
			sweep.Floats("aside", 0, 6),
		},
		MaxFailRate: 0,
		Sample:      12,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		n := int(p.Float("profiles"))
		aside := p.Float("aside")
		var loops [][]srf.SBezier
		for i := range n {
			f := float64(i) / float64(n-1)
			loops = append(loops, circleAt(4, aside*f*f, 0, 15*f))
		}
		sh, err := topo.MakeSmoothLoftEnds(loops, 0, topo.LoftEnds{Start: p.Float("start"), End: p.Float("end")},
			core.NewRGBA(1, 1, 1, 255), 1<<30, srf.Tol(0.002, 20))
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		r := check.Solid(sh, &m)
		if !r.Sound() {
			return fmt.Errorf("unsound: %s", r.String())
		}
		got := r.Volume
		want := math.Pi * 16 * 15
		if math.Abs(got-want) > 0.003*want {
			return fmt.Errorf("volume %g, want %g", got, want)
		}
		return nil
	}).Require(t)
}

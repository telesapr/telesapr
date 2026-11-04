package check_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A block with narrow slots whose walls, square to the axes, fall
// between the grids' rays: with every ray through its cell's middle, a
// column along a wall counted wholly in or out, and at 256 rays a side
// the grids still disagreed by 0.14%. The slots' widths and
// places are no fraction of the block's that a grid of 2^k rays would
// land on.
func TestOracleWallsAlongRays(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	d.Features.Extrude(10, "")
	want := 30.0 * 20 * 10
	for _, x := range []float64{4.13, 11.71, 19.37} {
		d.Features.SketchIn(d.Workplane("XY"), "")
		d.Sketch.Rect(api.V(x, 2.29, 0), api.V(x+0.61, 17.43, 0))
		d.Features.Extrude(7.07, "").SetCombine(api.Difference)
		want -= 0.61 * (17.43 - 2.29) * 7.07
	}
	m := oracleOf(t, d).Measure(1e-3, 256)
	if !near(m.Volume, want, 1e-3) || m.Spread() > 1e-3 {
		t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, want, m.Spread(), m.Rays)
	}
}

// A square swept round a closed skew pentagon, mitred at its corners:
// the ray-cast check's volume is the square's area times the path's
// length, as the kernel's mesh is. Its straight pieces' walls lie along
// the grids' rays; through the cells' middles the check settled 0.27%
// over.
func TestRayCheckSkewSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("the ray-cast check of a sweep round a skew pentagon")
	}
	pts := []api.Vec{api.V(0, 0, -5), api.V(0, 0, 5), api.V(10, 2, 6), api.V(12, 9, -1), api.V(3, 7, -6)}
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.01 })
	d.Features.Sketch3D("path")
	var path []api.Entity
	length := 0.0
	for i := range pts {
		next := pts[(i+1)%len(pts)]
		path = append(path, d.Sketch.Line(pts[i], next))
		length += next.Minus(pts[i]).Magnitude()
	}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-1, -1, 0), api.V(1, 1, 0))
	if p := d.Features.Sweep(path, "").Problem(); p != "" {
		t.Fatal(p)
	}
	sk := d.Host().Sketch()
	sk.GenerateAll()
	r, err := check.RayCheck(sk, sk.Group(sk.ActiveGroup()), 8)
	want := 4 * length
	if err != nil || !near(r.Measure.Volume, want, 1e-3) || !near(r.MeshVolume, want, 1e-3) || r.Measure.Spread() > 1e-3 {
		t.Errorf("%v: ray-cast %g (spread %g at %d rays), mesh %g, want %g", err, r.Measure.Volume, r.Measure.Spread(), r.Measure.Rays, r.MeshVolume, want)
	}
}

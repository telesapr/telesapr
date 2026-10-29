package check_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// slantCorner draws a path up 30 from the origin and on at 60 degrees
// from the vertical for 25 in ZX, with a corner between them, and a
// cubic on from there out of the plane when curved; a 10 by 8
// rectangle on the origin is swept along it.
func slantCorner(d *api.Doc, curved bool) api.SweepGroup {
	d.Features.Sketch3D("path")
	top := api.V(0, 0, 30)
	dir := api.V(math.Sin(math.Pi/3), 0, math.Cos(math.Pi/3))
	end := top.Plus(dir.ScaledBy(25))
	path := []api.Entity{d.Sketch.Line(api.V(0, 0, 0), top), d.Sketch.Line(top, end)}
	if curved {
		c := slantCubic(end, dir)
		path = append(path, d.Sketch.Cubic(c[0], c[1], c[2], c[3]))
	}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-5, -4, 0), api.V(5, 4, 0))
	return d.Features.Sweep(path, "bent").AsSweep()
}

// slantCubic is the cubic on from end along dir, bending gently out of
// the plane.
func slantCubic(end, dir api.Vec) [4]api.Vec {
	return [4]api.Vec{end, end.Plus(dir.ScaledBy(15)), end.Plus(api.V(25, 8, 8)), end.Plus(api.V(35, 14, 6))}
}

// Held to the corner's axis, a sweep along two lines meeting at a
// corner is mitred as a rigid one is: the oracle's prisms for the
// fitted one and its steps for the rigid one give the same solid, ray
// by ray.
func TestOracleFittedCornerAsRigid(t *testing.T) {
	t.Parallel()
	rigid, held := api.New(), api.New()
	slantCorner(rigid, false)
	slantCorner(held, false).SetSweepUp(api.V(0, 1, 0))
	if p := held.Active().Problem(); p != "" {
		t.Fatalf("the kernel refused it: %s", p)
	}
	a, b := oracleOf(t, rigid), oracleOf(t, held)
	for _, r := range []struct{ from, dir api.Vec }{
		{api.V(-20, 0, 29), api.V(1, 0, 0)},
		{api.V(-20, 3, 33), api.V(1, 0, 0.2)},
		{api.V(-4, -3.5, -10), api.V(0.05, 0, 1)},
		{api.V(-10, 0, 40), api.V(1, 0, -0.6)},
		{api.V(10, -20, 37), api.V(0, 1, 0)},
	} {
		ia, ib := a.Probe(r.from, r.dir), b.Probe(r.from, r.dir)
		same := len(ia) == len(ib)
		for i := 0; same && i < len(ia); i++ {
			same = math.Abs(ia[i].From-ib[i].From) < 1e-4 && math.Abs(ia[i].To-ib[i].To) < 1e-4
		}
		if !same || len(ia) == 0 {
			t.Errorf("from %v along %v: rigid %v, held %v", r.from, r.dir, ia, ib)
		}
	}
}

// A sweep along two lines meeting at a corner and then a cubic, its
// frames turning least: the mitre gives what it takes, so the volume is
// the profile's area times the path's length, and the kernel's mesh
// agrees with the oracle's prisms and moved profiles, ray by ray.
func TestOracleFittedCorner(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("rays through a fitted sweep, tens of seconds")
	}
	d := api.New()
	d.Edit.SetChordTolerance(0.01)
	slantCorner(d, true)
	if p := d.Active().Problem(); p != "" {
		t.Fatalf("the kernel refused it: %s", p)
	}
	top := api.V(0, 0, 30)
	end := top.Plus(api.V(math.Sin(math.Pi/3), 0, math.Cos(math.Pi/3)).ScaledBy(25))
	dir := end.Minus(top).WithMagnitude(1)
	want := 80 * (55 + cubicLength(slantCubic(end, dir)))
	o := oracleOf(t, d)
	if m := o.Measure(1e-3, 256); !near(m.Volume, want, 2e-3) {
		t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, want, m.Spread(), m.Rays)
	}
	if diffs := o.Diff(meshOf(d), 16, 0.05); len(diffs) != 0 {
		t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
	}
}

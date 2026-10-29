package check_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// cubicLength is the length of the cubic Bézier through p by Simpson's
// rule on a fine grid, the test's own.
func cubicLength(p [4]api.Vec) float64 {
	speed := func(u float64) float64 {
		a, b, c := (1-u)*(1-u), 2*u*(1-u), u*u
		d := p[1].Minus(p[0]).ScaledBy(3 * a).Plus(p[2].Minus(p[1]).ScaledBy(3 * b)).Plus(p[3].Minus(p[2]).ScaledBy(3 * c))
		return d.Magnitude()
	}
	const n = 4096
	sum := speed(0) + speed(1)
	for i := 1; i < n; i++ {
		w := 2.0
		if i%2 == 1 {
			w = 4
		}
		sum += w * speed(float64(i)/n)
	}
	return sum / (3 * n)
}

// fittedBar sweeps an a by a square centred on the origin in XY along
// the path draw makes, and returns the sweep.
func fittedBar(d *api.Doc, a float64, draw func(d *api.Doc) []api.Entity) api.SweepGroup {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	path := draw(d)
	d.Features.SketchIn(d.Workplane("XY"), "square")
	d.Sketch.Rect(api.V(-a/2, -a/2, 0), api.V(a/2, a/2, 0))
	return d.Features.Sweep(path, "bar").AsSweep()
}

// Fitted sweeps, placed along their paths by the oracle's own frames: a
// square square to its path with its centre on it sweeps its area times
// the path's length, however the frames turn about the path (Pappus), so
// twisted, held to a direction or along a cubic, in a plane or out of
// it, the volume is known. Each agrees with the kernel's fitted mesh,
// ray by ray.
func TestOracleFittedSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("rays through a fitted sweep, tens of seconds")
	}
	line := func(d *api.Doc) []api.Entity { return []api.Entity{d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 60))} }
	flat := [4]api.Vec{api.V(0, 0, 0), api.V(0, 0, 20), api.V(30, 0, 20), api.V(30, 0, 45)}
	skew := [4]api.Vec{api.V(0, 0, 0), api.V(0, 0, 20), api.V(20, 15, 25), api.V(30, 10, 45)}
	cubic := func(p [4]api.Vec) func(d *api.Doc) []api.Entity {
		return func(d *api.Doc) []api.Entity {
			// Drawn in 3d: in the workplane a point off it lands on it.
			d.Features.Sketch3D("path")
			return []api.Entity{d.Sketch.Cubic(p[0], p[1], p[2], p[3])}
		}
	}
	bend := func(d *api.Doc) []api.Entity {
		up := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 30))
		arc := d.Sketch.Arc(api.V(25, 0, 30), api.V(0, 0, 30), api.V(25, 0, 55))
		along := d.Sketch.Line(api.V(25, 0, 55), api.V(65, 0, 55))
		d.Constrain.Coincident(up.End(), arc.Start())
		d.Constrain.Coincident(arc.End(), along.Start())
		d.Constrain.Tangent(up, arc)
		d.Constrain.Tangent(arc, along)
		return []api.Entity{up, arc, along}
	}
	for name, c := range map[string]struct {
		a     float64
		draw  func(d *api.Doc) []api.Entity
		twist float64
		up    api.Vec
		want  float64
	}{
		"twisted line":  {12, line, 90, api.Vec{}, 144 * 60},
		"twisted bend":  {12, bend, 45, api.Vec{}, 144 * (70 + 25*math.Pi/2)},
		"cubic":         {6, cubic(flat), 0, api.Vec{}, 36 * cubicLength(flat)},
		"cubic out":     {6, cubic(skew), 0, api.Vec{}, 36 * cubicLength(skew)},
		"cubic held":    {6, cubic(skew), 0, api.V(1, 0, 0), 36 * cubicLength(skew)},
		"cubic twisted": {6, cubic(skew), 120, api.Vec{}, 36 * cubicLength(skew)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			d.Edit.SetChordTolerance(0.01)
			sw := fittedBar(d, c.a, c.draw)
			if c.twist != 0 {
				sw.SetTwist(c.twist)
			}
			if c.up.Magnitude() > 0 {
				sw.SetSweepUp(c.up)
			}
			if p := d.Active().Problem(); p != "" {
				t.Fatalf("the kernel refused it: %s", p)
			}
			o := oracleOf(t, d)
			if m := o.Measure(1e-3, 128); !near(m.Volume, c.want, 2e-3) {
				t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, c.want, m.Spread(), m.Rays)
			}
			if diffs := o.Diff(meshOf(d), 16, 0.05); len(diffs) != 0 {
				t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
			}
		})
	}
}

// Held to the normal of its path's plane, a sweep along lines and arcs
// turns as one moved rigidly by its steps does: the frames the oracle
// holds and the steps the oracle moves by give the same solid, ray by
// ray.
func TestOracleFittedHeldAsRigid(t *testing.T) {
	t.Parallel()
	rigid := api.New()
	bar(rigid)
	held := api.New()
	bar(held).AsSweep().SetSweepUp(api.V(0, 1, 0))
	if p := held.Active().Problem(); p != "" {
		t.Fatalf("the kernel refused it: %s", p)
	}
	a, b := oracleOf(t, rigid), oracleOf(t, held)
	for _, r := range []struct{ from, dir api.Vec }{
		{api.V(-20, 0, 20), api.V(1, 0, 0)},
		{api.V(-20, 3, 45), api.V(1, 0, 0.3)},
		{api.V(10, -5, -10), api.V(0, 0.1, 1)},
		{api.V(17, 2, 47), api.V(1, 0, -1)},
		{api.V(50, -20, 52), api.V(0, 1, 0)},
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

// A twisted bar along a line: its inertia about the line is V a² / 6,
// across it V (a² + L²) / 12, as an untwisted bar's, since a square's
// second moment is the same about every axis through its centre.
func TestOracleFittedSweepInertia(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("rays through a fitted sweep, tens of seconds")
	}
	d := api.New()
	fittedBar(d, 12, func(d *api.Doc) []api.Entity {
		return []api.Entity{d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 60))}
	}).SetTwist(90)
	m := oracleOf(t, d).Measure(1e-3, 128)
	v := 144.0 * 60
	axial, across := v*144/6, v*(144+3600)/12
	if !near(m.Inertia[2][2], axial, 3e-3) || !near(m.Inertia[0][0], across, 3e-3) || !near(m.Inertia[1][1], across, 3e-3) {
		t.Errorf("%v, want %g across and %g about the line", m.Inertia, across, axial)
	}
	if c := m.CenterOfMass; c.Minus(api.V(0, 0, 30)).Magnitude() > 0.01 {
		t.Errorf("centre %v", c)
	}
}

// A ring twisted a whole turn, fitted round its closed path, whose
// square comes back where it started: each section is the square turned
// about its centre, symmetric through it, with the same second moment
// about every axis through it, so Pappus gives the volume, 2 pi R A, and
// the moment about the ring's axis, 2 pi (A R³ + 3 R a⁴ / 12), however
// far it has turned.
func TestOracleFittedClosedSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("rays through a fitted sweep, tens of seconds")
	}
	d := api.New()
	d.Edit.SetChordTolerance(0.01)
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	path := d.Sketch.Circle(api.V(10, 0, 0), 10)
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-2, -2, 0), api.V(2, 2, 0))
	d.Features.Sweep([]api.Entity{path}, "").AsSweep().SetTwist(360)
	if p := d.Active().Problem(); p != "" {
		t.Fatalf("the kernel refused it: %s", p)
	}
	o := oracleOf(t, d)
	m := o.Measure(1e-3, 128)
	v, axial := 16*2*math.Pi*10, 2*math.Pi*(16*1000+3*10*256/12.0)
	// The ring lies in ZX about (10, 0, 0): its axis is y.
	if !near(m.Volume, v, 2e-3) || !near(m.Inertia[1][1], axial, 3e-3) {
		t.Errorf("volume %g, want %g; inertia %v, want %g about y (spread %g)", m.Volume, v, m.Inertia, axial, m.Spread())
	}
	if diffs := o.Diff(meshOf(d), 16, 0.05); len(diffs) != 0 {
		t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
	}
}

package check_test

import (
	"errors"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// helixRing draws a 10 by 10 square from 10 to 20 mm off a construction
// axis along y, in the plane z = 0, and returns the axis.
func helixRing(d *api.Doc) api.Entity {
	axis := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 10, 0))
	axis.SetConstruction(true)
	d.Sketch.Rect(api.V(10, 0, 0), api.V(20, 10, 0))
	return axis
}

// A helix of a square in a plane through its axis: Pappus gives the
// volume, area times angle times the centre's radius, however far it
// moves along the axis; turned both ways about the drawing, the same.
// Its rays agree with the kernel's mesh within 0.03 mm at a chord
// tolerance of 0.01%. Quadratic patches whose rise followed the arc's
// rational parameter, not its angle, were 2% of a quarter turn's rise
// off the true helix, and hundreds of rays disagreed here by more; a ray
// grazing the inner wall still can, where the oracle samples the
// crossing.
func TestOracleHelix(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		degrees, length float64
		twoSided        bool
	}{
		"three quarters": {270, 30, false},
		"two turns":      {720, 30, false},
		"backwards":      {-400, 25, false},
		"two sides":      {540, 36, true},
		"steep":          {720, 60, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			d.Edit.SetChordTolerance(0.01)
			g := d.Features.Helix(helixRing(d), c.degrees, c.length, "")
			g.SetTwoSided(c.twoSided)
			want := 100 * 15 * math.Abs(c.degrees) * math.Pi / 180
			o := oracleOf(t, d)
			if m := o.Measure(1e-3, 128); !near(m.Volume, want, 3e-3) {
				t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, want, m.Spread(), m.Rays)
			}
			if diffs := o.Diff(meshOf(d), 24, 0.03); len(diffs) != 0 {
				t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
			}
		})
	}
	// A line segment's direction runs from its end to its start, so the
	// helix about one drawn up y moves down it: where the square starts,
	// where it has turned a quarter (from x towards z) and moved a
	// quarter of the length, and three quarters.
	d := api.New()
	d.Features.Helix(helixRing(d), 360, 40, "")
	o := oracleOf(t, d)
	for _, c := range []struct {
		p  api.Vec
		in bool
	}{
		{api.V(15, 5, 0), true},
		{api.V(15, 15, 0), false},
		{api.V(0, -5, 15), true},
		{api.V(0, 5, 15), false},
		{api.V(0, -25, -15), true},
		{api.V(0, -5, -15), false},
		{api.V(5, 5, 0), false},
	} {
		if got := o.Inside(c.p); got != c.in {
			t.Errorf("inside %v: %t, want %t", c.p, got, c.in)
		}
	}
}

// bar draws the path up 30 along z from the origin, a quarter arc of
// radius 25 over to x, and 40 along x, and a 12 by 12 square on the
// origin to sweep along it.
func bar(d *api.Doc) api.Group {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	up := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 30))
	bend := d.Sketch.Arc(api.V(25, 0, 30), api.V(0, 0, 30), api.V(25, 0, 55))
	along := d.Sketch.Line(api.V(25, 0, 55), api.V(65, 0, 55))
	d.Constrain.Coincident(up.End(), bend.Start())
	d.Constrain.Coincident(bend.End(), along.Start())
	d.Constrain.Tangent(up, bend)
	d.Constrain.Tangent(bend, along)
	d.Features.SketchIn(d.Workplane("XY"), "square")
	d.Sketch.Rect(api.V(-6, -6, 0), api.V(6, 6, 0))
	return d.Features.Sweep([]api.Entity{up, bend, along}, "bar")
}

// corner draws a path up 30 along z from the origin and 40 along x from
// there, meeting at a right angle, and a 10 by 6 rectangle on the
// origin to sweep along it.
func corner(d *api.Doc) api.Group {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	up := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 30))
	over := d.Sketch.Line(api.V(0, 0, 30), api.V(40, 0, 30))
	d.Constrain.Coincident(up.End(), over.Start())
	d.Features.SketchIn(d.Workplane("XY"), "rectangle")
	d.Sketch.Rect(api.V(-5, -3, 0), api.V(5, 3, 0))
	return d.Features.Sweep([]api.Entity{up, over}, "corner")
}

// Sweeps along lines and arcs: a bar along a line, an arc and a line,
// whose volume Pappus gives, area times the centre's path; a mitred
// corner, whose two runs gain and lose the same wedges, so its volume is
// the area times the path's length; and the corner rounded with a
// radius. Each agrees with the kernel's mesh, ray by ray.
func TestOracleSweep(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		build func(d *api.Doc)
		want  float64
	}{
		"bar":    {func(d *api.Doc) { bar(d) }, 144 * (70 + 25*math.Pi/2)},
		"mitred": {func(d *api.Doc) { corner(d) }, 60 * 70},
		"rounded": {func(d *api.Doc) { corner(d).AsSweep().SetCornerRadius(8) },
			60 * (70 - 2*8 + 8*math.Pi/2)},
		// Round closed paths, started where the profile crosses them.
		"ring":  {func(d *api.Doc) { closedFrame(d, true) }, 4 * 2 * math.Pi * 10},
		"frame": {func(d *api.Doc) { closedFrame(d, false) }, 4 * 60},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			c.build(d)
			if p := d.Active().Problem(); p != "" {
				t.Fatalf("the kernel refused it: %s", p)
			}
			o := oracleOf(t, d)
			if m := o.Measure(1e-4, 256); !near(m.Volume, c.want, 2e-3) {
				t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, c.want, m.Spread(), m.Rays)
			}
			if diffs := o.Diff(meshOf(d), 24, 0.05); len(diffs) != 0 {
				t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
			}
		})
	}
	// The mitre: at the corner's outside, a point past the first run's
	// end but before the plane halving the corner is inside.
	d := api.New()
	corner(d)
	o := oracleOf(t, d)
	if !o.Inside(api.V(-4.5, 0, 34.4)) || o.Inside(api.V(-4.5, 0, 35.5)) || !o.Inside(api.V(4.9, 0, 25.2)) {
		t.Error("the corner is not mitred")
	}
}

// What the oracle doesn't build it refuses by name: a loft, whose
// surfaces between its sections are the kernel's own construction.
func TestOracleRefusesLoft(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("XY"), "foot")
	d.Sketch.Rect(api.V(-10, -10, 0), api.V(10, 10, 0))
	foot := d.Active()
	d.Features.Sketch3D("")
	o := d.Sketch.Point(api.V(0, 0, 20))
	top := d.Features.SketchAt(o, api.V(1, 0, 0), api.V(0, 1, 0), "")
	d.Sketch.Circle(api.V(0, 0, 20), 6)
	d.Features.Loft(foot, top, "")
	sk := d.Host().Sketch()
	sk.GenerateAll()
	if _, err := check.NewOracle(sk, sk.ActiveGroup()); !errors.Is(err, check.ErrUnsupported) {
		t.Errorf("a loft: %v", err)
	}
}

// TestOracleRefusesArcCorner holds the oracle to refusing a sharp corner
// beside an arc, which the sweep joins along curves its leaves can't cut.
func TestOracleRefusesArcCorner(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	top := api.V(0, 0, 30)
	path := []api.Entity{
		d.Sketch.Line(api.V(0, 0, 0), top),
		d.Sketch.Arc(api.V(0, 0, 42), top, api.V(12, 0, 42)),
	}
	d.Features.SketchIn(d.Workplane("XY"), "bar")
	d.Sketch.Circle(api.V(0, 0, 0), 2)
	hook := d.Features.Sweep(path, "hook")
	if p := hook.Problem(); p != "" {
		t.Fatal(p)
	}
	sk := d.Host().Sketch()
	sk.GenerateAll()
	if _, err := check.NewOracle(sk, sk.ActiveGroup()); !errors.Is(err, check.ErrUnsupported) {
		t.Errorf("a corner beside an arc: %v", err)
	}
	// Held to the arc's axis the sweep is fitted, and the corner joined
	// as the rigid sweep's: refused the same.
	hook.AsSweep().SetSweepUp(api.V(0, 1, 0))
	if p := hook.Problem(); p != "" {
		t.Fatal(p)
	}
	sk.GenerateAll()
	if _, err := check.NewOracle(sk, sk.ActiveGroup()); !errors.Is(err, check.ErrUnsupported) {
		t.Errorf("a fitted sweep's corner beside an arc: %v", err)
	}
}

// closedFrame sweeps a 2 by 2 square about the origin in XY round a
// circle of radius 10 in ZX through the origin, with ring, or round a
// 20 by 10 rectangle with its left side on the z axis about z = 0.
func closedFrame(d *api.Doc, ring bool) {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	var path []api.Entity
	if ring {
		path = []api.Entity{d.Sketch.Circle(api.V(10, 0, 0), 10)}
	} else {
		p := []api.Vec{api.V(0, 0, -5), api.V(0, 0, 5), api.V(20, 0, 5), api.V(20, 0, -5)}
		for i := range p {
			path = append(path, d.Sketch.Line(p[i], p[(i+1)%4]))
		}
	}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-1, -1, 0), api.V(1, 1, 0))
	d.Features.Sweep(path, "")
}

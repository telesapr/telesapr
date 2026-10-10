package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// pipePath draws in ZX a line up from the origin by up, a quarter turn
// of radius r towards +x, and a line along x by along.
func pipePath(d *api.Doc, up, r, along float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	return []api.Entity{
		d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, up)),
		d.Sketch.Arc(api.V(r, 0, up), api.V(0, 0, up), api.V(r, 0, up+r)),
		d.Sketch.Line(api.V(r, 0, up+r), api.V(r+along, 0, up+r)),
	}
}

func TestSweepPipe(t *testing.T) {
	d := api.New()
	path := pipePath(d, 10, 8, 12)
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Circle(api.V(0, 0, 0), 2)
	step(d)
	before := saved(t, d)
	// The path is given in any order.
	s := d.Features.Sweep([]api.Entity{path[2], path[0], path[1]}, "")
	if p := s.Problem(); p != "" {
		t.Fatal(p)
	}
	r := s.Analyze().Check()
	want := math.Pi * 4 * (10 + 8*math.Pi/2 + 12)
	if !r.Sound() || math.Abs(r.Volume-want) > 0.01*want {
		t.Fatalf("pipe: %s, want volume %g", r.String(), want)
	}
	if lo, hi := s.Analyze().Bounds(); math.Abs(hi.X-20) > 1e-6 || math.Abs(hi.Z-20) > 1e-6 || math.Abs(lo.Z) > 1e-6 {
		t.Errorf("pipe bounds %v %v", lo, hi)
	}
	// The end cap's copy of the circle is at the end of the path,
	// turned to face along x.
	found := false
	for _, e := range s.Entities() {
		if e.Kind() == "circle" {
			if c := e.Center().Pos(); c.Minus(api.V(20, 0, 18)).Magnitude() < 1e-9 {
				found = true
				if n := e.Normal().Dir(); math.Abs(math.Abs(n.X)-1) > 1e-9 {
					t.Errorf("the end circle's normal is %v", n)
				}
			}
		}
	}
	if !found {
		t.Errorf("no copy of the profile at the end of the path")
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
}

func TestSweepProblems(t *testing.T) {
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	up := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 10))
	corner := d.Sketch.Line(api.V(0, 0, 10), api.V(10, 0, 10))
	apart := d.Sketch.Line(api.V(0, 0, 20), api.V(0, 0, 30))
	flat := d.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	tight := d.Sketch.Arc(api.V(1, 0, 10), api.V(0, 0, 10), api.V(1, 0, 11))
	branch := d.Sketch.Line(api.V(0, 0, 10), api.V(0, 0, 20))
	back := d.Sketch.Line(api.V(0, 0, 10), api.V(0, 0, 5))
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	circle := d.Sketch.Circle(api.V(0, 0, 0), 2)
	for name, c := range map[string]struct {
		path []api.Entity
		want string
	}{
		// A corner between two lines is mitred (roadmap 28.4); one
		// between a line and the tight arc is refused.
		"corner":    {[]api.Entity{up, back}, "turns back"},
		"apart":     {[]api.Entity{up, apart}, "not one chain"},
		"flat":      {[]api.Entity{flat}, "along the profile's plane"},
		"tight":     {[]api.Entity{up, tight}, "too tight"},
		"circle":    {[]api.Entity{circle}, "not a line segment, an arc or a cubic"},
		"no path":   {nil, "no path"},
		"branching": {[]api.Entity{up, corner, branch}, "branches"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			d.Features.Sweep(c.path, "")
		}()
	}
}

// TestSweepGrid sweeps a circle of radius rho along a line of length
// up (down, for a negative one), a turn of radius r by angle degrees
// and a straight tail, and requires a sound solid of the volume
// Pappus gives. The rate allowed was fixed at none before the sweep
// was written; see sweepGridMaxFailRate for what it is.
func TestSweepGrid(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("rho", 0.5, 2),
			sweep.Floats("r", 5, 10, 30),
			sweep.Floats("angle", 30, 90, 150),
			sweep.Floats("up", 10, -10),
			sweep.Floats("tilt", 0, 25),
		},
		MaxFailRate: sweepGridMaxFailRate,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		rho, r, up := p.Float("rho"), p.Float("r"), p.Float("up")
		th := p.Float("angle") * math.Pi / 180
		tail := 5.0
		d := api.New()
		fineChord(d)
		// The path, in a workplane through the z axis, turned by tilt
		// about it.
		tilt := p.Float("tilt") * math.Pi / 180
		dir := api.V(math.Cos(tilt), math.Sin(tilt), 0)
		d.Sketch.Anywhere3D()
		wp := d.Sketch.NewWorkplane(api.V(0, 0, 0), api.V(0, 0, 1), dir)
		d.Features.SketchIn(wp, "path")
		sgn := 1.0
		if up < 0 {
			sgn = -1
		}
		at := func(x, z float64) api.Vec { return dir.ScaledBy(x).Plus(api.V(0, 0, z)) }
		c := at(r, up)
		// The turn bends towards +dir, starting along z.
		end := c.Plus(at(-r*math.Cos(th), sgn*r*math.Sin(th)).Minus(api.V(0, 0, 0)))
		tangent := at(math.Sin(th), sgn*math.Cos(th))
		var arc api.Entity
		if up > 0 {
			arc = d.Sketch.Arc(c, at(0, up), end)
		} else {
			arc = d.Sketch.Arc(c, end, at(0, up))
		}
		path := []api.Entity{
			d.Sketch.Line(at(0, 0), at(0, up)),
			arc,
			d.Sketch.Line(end, end.Plus(tangent.ScaledBy(tail))),
		}
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Circle(api.V(0, 0, 0), rho)
		s := d.Features.Sweep(path, "")
		res := s.Analyze().Check()
		want := math.Pi * rho * rho * (math.Abs(up) + r*th + tail)
		switch {
		case s.Problem() != "":
			return fmt.Errorf("problem: %s", s.Problem())
		case !res.Sound():
			return fmt.Errorf("unsound: %s", res.String())
		case math.Abs(res.Volume-want) > 5e-3*want:
			return fmt.Errorf("volume %g, want %g", res.Volume, want)
		}
		return nil
	}).Require(t)
}

// sweepGridMaxFailRate is the fraction of TestSweepGrid's grid allowed
// to fail: none, as the whole grid gave in the review of 2026-10-03.
// When the sweep was written, earlier that day, 7 of 72 failed, all
// with the thinnest tube (0.5 about bends of 5 and 10): the solid
// check found edges of the turn's triangulation, the grid
// triangulation of thin tori, crossing a neighbouring row by about
// 2e-4 mm.
const sweepGridMaxFailRate = 0

// cubicLength is the length of the cubic Bezier p0 p1 p2 p3, by
// Simpson's rule on its speed.
func cubicLength(p0, p1, p2, p3 api.Vec) float64 {
	speed := func(t float64) float64 {
		s := 1 - t
		d := p1.Minus(p0).ScaledBy(3 * s * s).Plus(p2.Minus(p1).ScaledBy(6 * s * t)).Plus(p3.Minus(p2).ScaledBy(3 * t * t))
		return d.Magnitude()
	}
	const n = 2000
	sum := speed(0) + speed(1)
	for k := 1; k < n; k++ {
		w := 2.0
		if k%2 == 1 {
			w = 4
		}
		sum += w * speed(float64(k)/n)
	}
	return sum / (3 * n)
}

// A circle swept up a line and on along a cubic that leaves its plane
// (roadmap 20.7): its surface is fitted, and the profile, centred on
// the path, keeps its area times the path's length by the centroid
// theorem. The end's copy of the profile faces along the path's end.
func TestSweepAlongCubic(t *testing.T) {
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.005 })
	d.Features.Sketch3D("path")
	c := []api.Vec{api.V(0, 0, 10), api.V(0, 0, 20), api.V(6, 8, 28), api.V(16, 10, 30)}
	path := []api.Entity{d.Sketch.Line(api.V(0, 0, 0), c[0]), d.Sketch.Cubic(c[0], c[1], c[2], c[3])}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Circle(api.V(0, 0, 0), 2)
	step(d)
	before := saved(t, d)
	s := d.Features.Sweep(path, "")
	if p := s.Problem(); p != "" {
		t.Fatal(p)
	}
	want := math.Pi * 4 * (10 + cubicLength(c[0], c[1], c[2], c[3]))
	r := s.Analyze().Check()
	if !r.Sound() || math.Abs(r.Volume-want) > 0.005*want {
		t.Fatalf("along a cubic: %s, want volume %g", r.String(), want)
	}
	endDir := c[3].Minus(c[2]).WithMagnitude(1)
	found := false
	for _, e := range s.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Minus(c[3]).Magnitude() < 1e-9 {
			found = true
			if n := e.Normal().Dir(); math.Abs(math.Abs(n.Dot(endDir))-1) > 1e-6 {
				t.Errorf("the end circle's normal is %v, not along %v", n, endDir)
			}
		}
	}
	if !found {
		t.Errorf("no copy of the profile at the end of the path")
	}
	undoRedo(t, d, before)
	// A file holds the curves' points at the chord tolerance, which it
	// doesn't hold: the default's, for the round trip.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	d.Edit.Regen()
	roundTrip(t, d)
}

// A square swept along a line and twisted keeps its volume, its end
// turned by the twist; a circle swept along a quarter of a helix with
// its frames held to the axis keeps its area times the path's length,
// as it does when they turn least, and both are saved.
func TestSweepTwistAndUp(t *testing.T) {
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.005 })
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	line := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 30))
	d.Features.SketchIn(d.Workplane("XY"), "square")
	d.Sketch.Rect(api.V(-2, -2, 0), api.V(2, 2, 0))
	bar := d.Features.Sweep([]api.Entity{line}, "bar")
	bar.AsSweep().SetTwist(45)
	// Its sides are twisted ruled faces, which the mesher grids with
	// rows but no columns: as a twisted loft's (TestLoftSweep), the
	// diagonals of the quads bias the volume, here by 1.6%.
	if r := bar.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-480) > 0.02*480 || bar.AsSweep().Twist() != 45 {
		t.Fatalf("a twisted bar: %s, twist %g", r.String(), bar.AsSweep().Twist())
	}
	for _, e := range bar.Entities() {
		if e.Kind() != "point" {
			continue
		}
		// The corners' copies at the end, not the copy of the sketch's
		// origin on the axis.
		if p := e.Pos(); math.Abs(p.Z-30) < 1e-9 && math.Hypot(p.X, p.Y) > 1e-9 {
			if math.Abs(math.Hypot(p.X, p.Y)-math.Sqrt(8)) > 1e-6 || math.Min(math.Abs(p.X), math.Abs(p.Y)) > 1e-6 {
				t.Errorf("an end corner of the bar at %v, not a corner turned by 45 degrees", p)
			}
		}
	}

	// A quarter of a helix of radius 20 rising 5, as a cubic.
	h := api.New()
	h.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.005 })
	const rad, rise = 20.0, 5.0
	k := 4.0 / 3 * math.Tan(math.Pi/8)
	c := []api.Vec{api.V(rad, 0, 0), api.V(rad, rad*k, rise/3), api.V(rad*k, rad, 2*rise/3), api.V(0, rad, rise)}
	h.Features.Sketch3D("path")
	helix := h.Sketch.Cubic(c[0], c[1], c[2], c[3])
	t0 := c[1].Minus(c[0]).WithMagnitude(1)
	o := h.Sketch.Point(c[0])
	h.Features.SketchAt(o, api.V(1, 0, 0), t0.Cross(api.V(1, 0, 0)), "profile")
	h.Sketch.Circle(c[0], 2)
	s := h.Features.Sweep([]api.Entity{helix}, "spring")
	want := math.Pi * 4 * cubicLength(c[0], c[1], c[2], c[3])
	least := s.Analyze().Check()
	s.AsSweep().SetSweepUp(api.V(0, 0, 3))
	held := s.Analyze().Check()
	for name, r := range map[string]struct {
		Sound  bool
		Volume float64
	}{"turning least": {least.Sound(), least.Volume}, "held up": {held.Sound(), held.Volume}} {
		if !r.Sound || math.Abs(r.Volume-want) > 0.005*want {
			t.Errorf("%s: sound %v, volume %g, want %g", name, r.Sound, r.Volume, want)
		}
	}
	if up := s.AsSweep().SweepUp(); up != api.V(0, 0, 1) {
		t.Errorf("held to %v", up)
	}
	h.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	h.Edit.Regen()
	if back := roundTrip(t, h).Group("spring"); back.AsSweep().SweepUp() != api.V(0, 0, 1) {
		t.Errorf("reopened held to %v", back.AsSweep().SweepUp())
	}
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	d.Edit.Regen()
	if back := roundTrip(t, d).Group("bar"); back.AsSweep().Twist() != 45 {
		t.Errorf("reopened twisted by %g", back.AsSweep().Twist())
	}
	// Along a line held up its own way, the frames have no direction.
	bar.AsSweep().SetSweepUp(api.V(0, 0, 1))
	if bar.Problem() == "" {
		t.Error("a sweep held up along its path has no problem")
	}
}

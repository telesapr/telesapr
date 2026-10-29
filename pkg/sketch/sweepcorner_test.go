package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// cornerPath draws, in the ZX workplane, up from the origin by up, then
// at angle degrees from the vertical for along; a corner between them.
func cornerPath(d *api.Doc, up, along, angle float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	th := angle * math.Pi / 180
	top := api.V(0, 0, up)
	return []api.Entity{
		d.Sketch.Line(api.V(0, 0, 0), top),
		d.Sketch.Line(top, top.Plus(api.V(math.Sin(th), 0, math.Cos(th)).ScaledBy(along))),
	}
}

// A sweep along a path with a corner (roadmap 28.4): the pieces are
// mitred, each cut by the plane halving the corner, so the volume is the
// profile's area times the path's length (the mitre takes from one
// piece what it gives the other).
func TestSweepCorner(t *testing.T) {
	t.Parallel()
	d := api.New()
	path := cornerPath(d, 10, 12, 90)
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
	step(d)
	before := saved(t, d)
	s := d.Features.Sweep(path, "")
	if p := s.Problem(); p != "" {
		t.Fatal(p)
	}
	r := s.Analyze().Check()
	if want := 8.0 * 22; !r.Sound() || math.Abs(r.Volume-want) > 1e-6*want {
		t.Fatalf("an L: %s, want %g", r.String(), want)
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	// A round profile, mitred: elliptical joints.
	d2 := api.New()
	d2.Edit.SetChordTolerance(0.002)
	path2 := cornerPath(d2, 10, 12, 60)
	d2.Features.SketchIn(d2.Workplane("XY"), "profile")
	d2.Sketch.Circle(api.V(0, 0, 0), 2)
	s2 := d2.Features.Sweep(path2, "")
	r2 := s2.Analyze().Check()
	if want := math.Pi * 4 * 22; s2.Problem() != "" || !r2.Sound() || math.Abs(r2.Volume-want) > 0.005*want {
		t.Fatalf("a round elbow: %q %s, want %g", s2.Problem(), r2.String(), want)
	}
}

func TestSweepCornerRefused(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		path func(d *api.Doc) []api.Entity
		want string
	}{
		"too short": {func(d *api.Doc) []api.Entity {
			d.Features.SketchIn(d.Workplane("ZX"), "path")
			return []api.Entity{
				d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 10)),
				d.Sketch.Line(api.V(0, 0, 10), api.V(0.5, 0, 10)),
				d.Sketch.Line(api.V(0.5, 0, 10), api.V(0.5, 0, 0)),
			}
		}, "too short"},
	} {
		d := api.New()
		path := c.path(d)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			d.Features.Sweep(path, "")
		}()
	}
}

// A sweep along edges of the solid by name (roadmap 28.4): a bead along
// the top edges of a block, round its corner, by their names.
func TestSweepSolidEdges(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 20, 10, 5)
	s := d.Active().Solid()
	front := s.EdgeAt(api.V(10, 0, 5))
	right := s.EdgeAt(api.V(20, 5, 5))
	// The profile: a small square on the block's left end, across the
	// front top edge's start.
	d.Features.SketchIn(d.Workplane("YZ"), "profile")
	d.Sketch.Rect(api.V(0, -0.5, 4.5), api.V(0, 0.5, 5.5))
	step(d)
	before := saved(t, d)
	g := d.Features.SweepEdges([]api.SolidEdge{right, front}, nil, "")
	if p := g.Problem(); p != "" {
		t.Fatal(p)
	}
	if len(g.SolidEdges()) != 2 {
		t.Errorf("the path's edges: %v", g.SolidEdges())
	}
	r := g.Analyze().Check()
	// The block and the bead, mitred at the corner: the bead lies a
	// quarter in the block along each edge, a quarter of its 1 by 1
	// section, so it adds three quarters of a square mm per mm, along
	// 30 mm of path less the overlap at the corner (a quarter square's
	// worth either way).
	if !r.Sound() || r.Volume < 1000+0.75*30-1 || r.Volume > 1000+0.75*30+1 {
		t.Fatalf("a bead along two edges: %s", r.String())
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
}

// A corner beside an arc with no corner radius: joined where the
// profile's paths along the line and the arc cross, the outside of the
// turn filled by both carried on, sound, one step of undo, and back from
// the file. The kernel's tests hold its volume to the slices'; here it
// is near the area times the path's length.
func TestSweepCornerBesideArc(t *testing.T) {
	t.Parallel()
	for _, way := range []float64{1, -1} {
		// At the default chord tolerance, which the file doesn't hold, so
		// that it comes back the same.
		d := api.New()
		d.Features.SketchIn(d.Workplane("ZX"), "path")
		// Up 10, then round the circle of radius 5 about (0, 0, 15), from
		// the line's top at a right angle: three quarters of a turn one way
		// (the corner turning with the arc), one quarter the other.
		path := []api.Entity{
			d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 10)),
			d.Sketch.Arc(api.V(0, 0, 15), api.V(0, 0, 10), api.V(way*5, 0, 15)),
		}
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		step(d)
		before := saved(t, d)
		s := d.Features.Sweep(path, "")
		if p := s.Problem(); p != "" {
			t.Fatalf("way %g: %s", way, p)
		}
		turn := 3 * math.Pi / 2
		if way < 0 {
			turn = math.Pi / 2
		}
		want := 8 * (10 + 5*turn)
		if r := s.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.03*want {
			t.Errorf("way %g: %s, want about %g", way, r.String(), want)
		}
		undoRedo(t, d, before)
		roundTrip(t, d)
	}
}

// A corner beside an arc that the kernel can't join is a problem on the
// group naming the corner and the corner radius, which then rounds it:
// up 10, then back down at 150° to an arc of radius 10 turning on the
// way, whose outside the line's, carried back, never meets.
func TestSweepCornerBesideArcRefused(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	th := 150 * math.Pi / 180
	top := api.V(0, 0, 10)
	c := top.Plus(api.V(-math.Cos(th), 0, math.Sin(th)).ScaledBy(10))
	end := c.Plus(top.Minus(c).RotatedAroundAxis(api.V(0, 1, 0), 1))
	path := []api.Entity{
		d.Sketch.Line(api.V(0, 0, 0), top),
		d.Sketch.Arc(c, end, top),
	}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
	s := d.Features.Sweep(path, "")
	if p := s.Problem(); !strings.Contains(p, "can't be joined") || !strings.Contains(p, "corner radius") {
		t.Fatalf("unrounded: %q", p)
	}
	s.AsSweep().SetCornerRadius(2)
	if p := s.Problem(); p != "" {
		t.Errorf("rounded: %q", p)
	}
}

// A closed path whose corners are beside an arc: a line closed by half
// a circle that meets it square at both ends, a D, swept from the
// line's middle; both corners joined, the ring sound.
func TestSweepClosedCornersBesideArc(t *testing.T) {
	t.Parallel()
	for name, area := range map[string]float64{"circle": math.Pi, "rect": 4} {
		d := api.New()
		d.Features.SketchIn(d.Workplane("ZX"), "path")
		path := []api.Entity{
			d.Sketch.Line(api.V(-4, 0, -6), api.V(-4, 0, 6)),
			d.Sketch.Arc(api.V(-4, 0, 0), api.V(-4, 0, 6), api.V(-4, 0, -6)),
		}
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		if name == "circle" {
			d.Sketch.Circle(api.V(-4, 0, 0), 1)
		} else {
			d.Sketch.Rect(api.V(-5, -1, 0), api.V(-3, 1, 0))
		}
		s := d.Features.Sweep(path, "")
		want := area * (12 + 6*math.Pi)
		if r := s.Analyze().Check(); s.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 0.02*want {
			t.Errorf("%s: %q %s, want about %g", name, s.Problem(), r.String(), want)
		}
	}
}

// A sweep along a cubic takes sharp corners between lines, mitred as a
// rigid sweep's are: the volume is the profile's area times the path's
// length. Twisted, its corner needs a radius, and held to a direction
// it turns off the corner's axis, too; a corner beside the cubic is
// refused as it was.
func TestSweepFittedCorner(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.005 })
	d.Features.Sketch3D("path")
	c := []api.Vec{api.V(0, 0, 0), api.V(0, 0, 6), api.V(4, 0, 10), api.V(8, 0, 12)}
	a := c[3].Plus(c[3].Minus(c[2]).WithMagnitude(8))
	b := a.Plus(api.V(1, 0, -2).WithMagnitude(10))
	path := []api.Entity{d.Sketch.Cubic(c[0], c[1], c[2], c[3]), d.Sketch.Line(c[3], a), d.Sketch.Line(a, b)}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-1, -1.5, 0), api.V(1, 1.5, 0))
	step(d)
	before := saved(t, d)
	s := d.Features.Sweep(path, "bent")
	if p := s.Problem(); p != "" {
		t.Fatal(p)
	}
	want := 6 * (cubicLength(c[0], c[1], c[2], c[3]) + 18)
	if r := s.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("a mitred corner after a cubic: %s, want %g", r.String(), want)
	}
	undoRedo(t, d, before)
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	d.Edit.Regen()
	roundTrip(t, d)
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.005 })
	d.Edit.Regen()

	sw := s.AsSweep()
	sw.SetTwist(30)
	if p := s.Problem(); !strings.Contains(p, "twisted") || !strings.Contains(p, "corner radius") {
		t.Errorf("a twisted sweep's corner: %q", p)
	}
	sw.SetCornerRadius(3)
	if r := s.Analyze().Check(); s.Problem() != "" || !r.Sound() {
		t.Errorf("a twisted sweep's corner rounded: %q %s", s.Problem(), r.String())
	}
	sw.SetTwist(0)
	sw.SetCornerRadius(0)
	sw.SetSweepUp(api.V(1, 1, 0))
	if p := s.Problem(); !strings.Contains(p, "held to a direction") {
		t.Errorf("held off the corner's axis: %q", p)
	}
	sw.SetSweepUp(api.V(0, 1, 0))
	if r := s.Analyze().Check(); s.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Errorf("held to the corner's axis: %q %s, want %g", s.Problem(), r.String(), want)
	}

	// A corner straight off the cubic.
	e := api.New()
	e.Features.Sketch3D("path")
	kinked := []api.Entity{e.Sketch.Cubic(c[0], c[1], c[2], c[3]), e.Sketch.Line(c[3], c[3].Plus(api.V(8, 0, 0)))}
	e.Features.SketchIn(e.Workplane("XY"), "profile")
	e.Sketch.Rect(api.V(-1, -1.5, 0), api.V(1, 1.5, 0))
	defer func() {
		err, ok := recover().(error)
		if !ok || !strings.Contains(err.Error(), "next to a curve") {
			t.Errorf("a corner beside a cubic: %v", err)
		}
	}()
	e.Features.Sweep(kinked, "")
}

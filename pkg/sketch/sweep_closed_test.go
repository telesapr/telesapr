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

// framePath draws in ZX a rectangle w along x by h along z, its left
// side on the z axis, about z = 0.
func framePath(d *api.Doc, w, h float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	a, b := api.V(0, 0, -h/2), api.V(0, 0, h/2)
	c, e := api.V(w, 0, h/2), api.V(w, 0, -h/2)
	return []api.Entity{d.Sketch.Line(a, b), d.Sketch.Line(b, c), d.Sketch.Line(c, e), d.Sketch.Line(e, a)}
}

// stadiumPath draws in ZX two lines up and down by h, joined by half
// circles of radius r, the left line on the z axis, about z = 0.
func stadiumPath(d *api.Doc, h, r float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	a, b := api.V(0, 0, -h/2), api.V(0, 0, h/2)
	c, e := api.V(2*r, 0, h/2), api.V(2*r, 0, -h/2)
	return []api.Entity{
		d.Sketch.Line(a, b),
		d.Sketch.Arc(api.V(r, 0, h/2), b, c),
		d.Sketch.Line(c, e),
		d.Sketch.Arc(api.V(r, 0, -h/2), e, a),
	}
}

// squareProfile draws a square of side a about the origin in XY.
func squareProfile(d *api.Doc, a float64) {
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	squareAt(d, api.V(0, 0, 0), a)
}

// squareAt draws a square of side a about c in the active sketch, its
// sides along x and y.
func squareAt(d *api.Doc, c api.Vec, a float64) {
	h := a / 2
	p := []api.Vec{c.Plus(api.V(-h, -h, 0)), c.Plus(api.V(h, -h, 0)), c.Plus(api.V(h, h, 0)), c.Plus(api.V(-h, h, 0))}
	for i := range p {
		d.Sketch.Line(p[i], p[(i+1)%4])
	}
}

// Sweeps round closed paths: a ring round a circle, a frame round a
// rectangle, mitred at its corners and rounded by a corner radius, a
// stadium of lines and half circles, a twisted ring; each sound, with
// no caps, of the volume the centroid theorem gives, one undo step, the
// same after a save and load.
func TestSweepClosed(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		make func(d *api.Doc) api.Group
		want float64
	}{
		{"ring", func(d *api.Doc) api.Group {
			d.Features.SketchIn(d.Workplane("ZX"), "path")
			ring := d.Sketch.Circle(api.V(10, 0, 0), 10)
			d.Features.SketchIn(d.Workplane("XY"), "profile")
			d.Sketch.Circle(api.V(0, 0, 0), 2)
			step(d)
			return d.Features.Sweep([]api.Entity{ring}, "")
		}, 2 * math.Pi * math.Pi * 10 * 4},
		{"frame", func(d *api.Doc) api.Group {
			path := framePath(d, 20, 10)
			squareProfile(d, 2)
			step(d)
			return d.Features.Sweep(path, "")
		}, 4 * 60},
		{"rounded frame", func(d *api.Doc) api.Group {
			path := framePath(d, 20, 10)
			squareProfile(d, 2)
			step(d)
			s := d.Features.Sweep(path, "")
			s.AsSweep().SetCornerRadius(2)
			return s
		}, 4 * (60 - 4*(4-math.Pi)/2*2)},
		{"stadium", func(d *api.Doc) api.Group {
			path := stadiumPath(d, 10, 6)
			d.Features.SketchIn(d.Workplane("XY"), "profile")
			d.Sketch.Circle(api.V(0, 0, 0), 1.5)
			step(d)
			return d.Features.Sweep(path, "")
		}, math.Pi * 2.25 * (20 + 2*6*math.Pi)},
		{"twisted ring", func(d *api.Doc) api.Group {
			d.Features.SketchIn(d.Workplane("ZX"), "path")
			ring := d.Sketch.Circle(api.V(10, 0, 0), 10)
			squareProfile(d, 2)
			step(d)
			s := d.Features.Sweep([]api.Entity{ring}, "")
			s.AsSweep().SetTwist(360)
			return s
		}, 4 * 2 * math.Pi * 10},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			fineChord(d, 0.002)
			s := c.make(d)
			if p := s.Problem(); p != "" {
				t.Fatal(p)
			}
			r := s.Analyze().Check()
			if !r.Sound() || math.Abs(r.Volume-c.want) > 0.005*c.want {
				t.Fatalf("%s, want volume %g", r.String(), c.want)
			}
			for _, e := range s.Entities() {
				if e.Kind() == "face" {
					t.Errorf("a closed sweep has a cap face")
				}
			}
			// The chord tolerance isn't saved.
			d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
			roundTrip(t, d)
		})
	}
}

// A closed sweep is one undo step, and comes back identical on redo.
func TestSweepClosedUndo(t *testing.T) {
	t.Parallel()
	d := api.New()
	path := framePath(d, 20, 10)
	squareProfile(d, 2)
	step(d)
	before := saved(t, d)
	d.Features.Sweep(path, "")
	undoRedo(t, d, before)
}

func TestSweepClosedProblems(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		make func(d *api.Doc) api.Group
		want string
	}{
		"at a corner": {func(d *api.Doc) api.Group {
			path := framePath(d, 20, 10)
			d.Sketch.Anywhere3D()
			d.Features.SketchIn(d.Sketch.NewWorkplane(api.V(0, 0, 5), api.V(1, 0, 0), api.V(0, 1, 0)), "profile")
			squareAt(d, api.V(0, 0, 5), 2)
			return d.Features.Sweep(path, "")
		}, "corner of the closed path"},
		"no crossing": {func(d *api.Doc) api.Group {
			path := framePath(d, 20, 10)
			d.Sketch.Anywhere3D()
			d.Features.SketchIn(d.Sketch.NewWorkplane(api.V(0, 0, 20), api.V(1, 0, 0), api.V(0, 1, 0)), "far")
			squareAt(d, api.V(0, 0, 20), 2)
			return d.Features.Sweep(path, "")
		}, "doesn't cross"},
		"part twist": {func(d *api.Doc) api.Group {
			d.Features.SketchIn(d.Workplane("ZX"), "path")
			ring := d.Sketch.Circle(api.V(10, 0, 0), 10)
			squareProfile(d, 2)
			s := d.Features.Sweep([]api.Entity{ring}, "")
			s.AsSweep().SetTwist(90)
			return s
		}, "whole turns"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			var msg string
			func() {
				defer func() {
					if err, ok := recover().(error); ok {
						msg = err.Error()
					}
				}()
				msg = c.make(d).Problem()
			}()
			if !strings.Contains(msg, c.want) {
				t.Errorf("%q, want a problem saying %q", msg, c.want)
			}
		})
	}
}

// TestSweepClosedSweep sweeps a square or a circle round a ring, a
// mitred frame, a frame rounded at its corners, and a stadium, the
// profile drawn at the start of a straight side or part way up it, and
// requires a sound solid of the volume the centroid theorem gives. The
// rate allowed was fixed at none before the feature was written.
func TestSweepClosedSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("a sweep round closed paths")
	}
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("path", "ring", "frame", "rounded", "stadium"),
			sweep.Strings("profile", "square", "circle"),
			sweep.Floats("size", 1, 2.5),
			// Within the straight of the left side the rounded corners
			// leave, |z| < 2.
			sweep.Floats("up", 0, 1.5),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		d.Edit.SetChordTolerance(0.002)
		up, size := p.Float("up"), p.Float("size")
		var path []api.Entity
		length := 0.0
		switch p.String("path") {
		case "ring":
			// A ring's profile is across it only in a plane through its
			// axis.
			up = 0
			d.Features.SketchIn(d.Workplane("ZX"), "path")
			path = []api.Entity{d.Sketch.Circle(api.V(10, 0, 0), 10)}
			length = 20 * math.Pi
		case "frame", "rounded":
			path = framePath(d, 20, 10)
			length = 60
		case "stadium":
			path = stadiumPath(d, 10, 6)
			length = 20 + 12*math.Pi
		}
		d.Sketch.Anywhere3D()
		d.Features.SketchIn(d.Sketch.NewWorkplane(api.V(0, 0, up), api.V(1, 0, 0), api.V(0, 1, 0)), "profile")
		area := size * size
		tol := 1e-6
		if p.String("profile") == "square" {
			squareAt(d, api.V(0, 0, up), size)
		} else {
			d.Sketch.Circle(api.V(0, 0, up), size/2)
			area = math.Pi * size * size / 4
		}
		if p.String("profile") == "circle" || p.String("path") != "frame" {
			tol = 0.005
		}
		s := d.Features.Sweep(path, "")
		if p.String("path") == "rounded" {
			s.AsSweep().SetCornerRadius(3)
			length = 60 - 4*(6-3*math.Pi/2)
		}
		r := s.Analyze().Check()
		want := area * length
		switch {
		case s.Problem() != "":
			return fmt.Errorf("problem: %s", s.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case math.Abs(r.Volume-want) > tol*want:
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}

// BenchmarkSweepClosed regenerates a square swept round a circle and a
// circle round a mitred frame.
func BenchmarkSweepClosed(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		d.Features.SketchIn(d.Workplane("ZX"), "ring")
		ring := d.Sketch.Circle(api.V(10, 0, 0), 10)
		squareProfile(d, 2)
		d.Features.Sweep([]api.Entity{ring}, "")
		path := framePath(d, 20, 10)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Circle(api.V(0, 0, 0), 1)
		d.Features.Sweep(path, "")
	})
}

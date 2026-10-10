package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// patternPath draws, in the XY workplane, a straight run of length
// run from the origin along x, then, if turn is not zero, a bend of
// radius r through turn degrees and another straight run.
func patternPath(d *api.Doc, run, r, turn float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("XY"), "path")
	first := d.Sketch.Line(api.V(0, 0, 0), api.V(run, 0, 0))
	if turn == 0 {
		return []api.Entity{first}
	}
	th := turn * math.Pi / 180
	c := api.V(run, r, 0)
	end := api.V(run+r*math.Sin(th), r-r*math.Cos(th), 0)
	bend := d.Sketch.Arc(c, api.V(run, 0, 0), end)
	tail := d.Sketch.Line(end, end.Plus(api.V(math.Cos(th), math.Sin(th), 0).ScaledBy(run)))
	return []api.Entity{first, bend, tail}
}

// TestPathPattern repeats a post along a straight run, a bend and
// another run: n copies of its volume when they stand apart, the last
// at the path's far end; and after a change of count, undo, redo and a
// round trip.
func TestPathPattern(t *testing.T) {
	d := api.New()
	path := patternPath(d, 20, 10, 90)
	d.Features.SketchIn(d.Workplane("XY"), "post-sketch")
	d.Sketch.Circle(api.V(0, 0, 0), 1)
	post := d.Features.Extrude(5, "post")
	one := post.Analyze().Check().Volume
	p := d.Features.PatternAlongPath(path, 5, "")
	r := p.Analyze().Check()
	if p.Problem() != "" || !r.Sound() || math.Abs(r.Volume-5*one) > 1e-6*one {
		t.Fatalf("five posts: %s, %s, want %g", p.Problem(), r.String(), 5*one)
	}
	// The far end of the path is (30, 30, 0).
	if math.Abs(r.Max.X-31) > 1e-6 || math.Abs(r.Max.Y-31) > 1e-6 {
		t.Errorf("the last copy: bounds up to %v", r.Max)
	}
	step(d)
	before := saved(t, d)
	p.AsRepeat().SetCopies(3)
	if p.AsRepeat().Copies() != 3 || !near(p.Analyze().Check().Volume, 3*one) {
		t.Errorf("three copies: %d, volume %g", p.AsRepeat().Copies(), p.Analyze().Check().Volume)
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	if p.Edges()[0] != path[0] {
		t.Error("the path")
	}
}

// TestPathPatternRefused: one copy, no path, a corner in the path.
func TestPathPatternRefused(t *testing.T) {
	for name, fn := range map[string]func(d *api.Doc, path []api.Entity){
		"one copy": func(d *api.Doc, path []api.Entity) { d.Features.PatternAlongPath(path, 1, "") },
		"no path":  func(d *api.Doc, path []api.Entity) { d.Features.PatternAlongPath(nil, 3, "") },
		"corner": func(d *api.Doc, path []api.Entity) {
			d.Features.SketchIn(d.Workplane("XY"), "")
			a := d.Sketch.Line(api.V(0, 0, 0), api.V(5, 0, 0))
			b := d.Sketch.Line(api.V(5, 0, 0), api.V(5, 5, 0))
			d.Activate(d.Group("post"))
			d.Features.PatternAlongPath([]api.Entity{a, b}, 3, "")
		},
	} {
		d := api.New()
		path := patternPath(d, 20, 0, 0)
		d.Features.SketchIn(d.Workplane("XY"), "")
		d.Sketch.Circle(api.V(0, 0, 0), 1)
		d.Features.Extrude(5, "post")
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn(d, path)
		}()
	}
}

// TestPathPatternSweep repeats a post along runs and bends of several
// sizes, apart and overlapping: a sound solid each time, of n posts'
// volume when they stand apart. The rate allowed, none, was set before.
func TestPathPatternSweep(t *testing.T) {
	sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("copies", 2, 3, 7),
			sweep.Floats("radius", 0.5, 2),
			sweep.Floats("turn", 0, 45, 180),
			sweep.Floats("bend", 6, 30),
		},
		MaxFailRate: 0,
		Sample:      12,
	}.Run(func(p sweep.Point) error {
		n, rad := int(p.Float("copies")), p.Float("radius")
		d := api.New()
		path := patternPath(d, 10, p.Float("bend"), p.Float("turn"))
		d.Features.SketchIn(d.Workplane("XY"), "")
		d.Sketch.Circle(api.V(0, 0, 0), rad)
		post := d.Features.Extrude(4, "")
		one := post.Analyze().Check().Volume
		g := d.Features.PatternAlongPath(path, n, "")
		r := g.Analyze().Check()
		switch {
		case g.Problem() != "":
			return fmt.Errorf("problem: %s", g.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case r.Volume > float64(n)*one*(1+1e-6) || r.Volume < one:
			return fmt.Errorf("volume %g for %d posts of %g", r.Volume, n, one)
		}
		return nil
	}).Require(t)
}

// A pattern round a circle (roadmap 28.5): the copies share the circle
// out, each turned with it, the one after the last being the first.
func TestPathPatternCircle(t *testing.T) {
	d := api.New()
	d.Features.SketchIn(d.Workplane("XY"), "ring")
	ring := d.Sketch.Circle(api.V(0, 0, 0), 20)
	d.Features.SketchIn(d.Workplane("XY"), "post-sketch")
	// A square post, which shows its turn: its sides along x and y.
	d.Sketch.Rect(api.V(19, -1, 0), api.V(21, 1, 0))
	post := d.Features.Extrude(5, "post")
	one := post.Analyze().Check().Volume
	p := d.Features.PatternAlongPath([]api.Entity{ring}, 4, "")
	r := p.Analyze().Check()
	if p.Problem() != "" || !r.Sound() || math.Abs(r.Volume-4*one) > 1e-6*one {
		t.Fatalf("four posts round a ring: %q, %s, want %g", p.Problem(), r.String(), 4*one)
	}
	// At 0, 90, 180 and 270 degrees, each turned a quarter more: the
	// whole stays inside the square of half side 21.
	for _, c := range []struct{ got, want float64 }{{r.Max.X, 21}, {r.Max.Y, 21}, {r.Min.X, -21}, {r.Min.Y, -21}} {
		if math.Abs(c.got-c.want) > 1e-6 {
			t.Errorf("bounds %v to %v, want ±21", r.Min, r.Max)
			break
		}
	}
	step(d)
	before := saved(t, d)
	p.AsRepeat().SetCopies(6)
	if !near(p.Analyze().Check().Volume, 6*one) {
		t.Errorf("six posts: %g", p.Analyze().Check().Volume)
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
}

// A pattern round a rounded rectangle, a closed chain of lines and
// arcs: the copies are spaced by length all round it.
func TestPathPatternClosedChain(t *testing.T) {
	d := api.New()
	d.Features.SketchIn(d.Workplane("XY"), "track")
	sides := d.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	for _, s := range sides {
		d.Sketch.TangentArc(s.Start(), 5, true)
	}
	var path []api.Entity
	for _, e := range d.Active().Entities() {
		if k := e.Kind(); k == "line" || k == "arc" {
			path = append(path, e)
		}
	}
	d.Features.SketchIn(d.Workplane("XY"), "post-sketch")
	d.Sketch.Circle(api.V(20, 0, 0), 1)
	post := d.Features.Extrude(3, "post")
	one := post.Analyze().Check().Volume
	// The perimeter: 2*(30+10) of straight runs and a circle of 5.
	perimeter := 2*(30+10) + 2*math.Pi*5
	n := 8
	p := d.Features.PatternAlongPath(path, n, "")
	r := p.Analyze().Check()
	if p.Problem() != "" || !r.Sound() || math.Abs(r.Volume-float64(n)*one) > 1e-6*float64(n)*one {
		t.Fatalf("posts round a track: %q, %s, want %g", p.Problem(), r.String(), float64(n)*one)
	}
	// The posts lie on the track: within the rectangle grown by a post's
	// radius, and touching each side of it.
	if r.Max.X > 41+1e-6 || r.Min.X < -1-1e-6 || r.Max.Y > 21+1e-6 || r.Min.Y < -1-1e-6 {
		t.Errorf("posts off the track: %v to %v (perimeter %g)", r.Min, r.Max, perimeter)
	}
}

// A closed path with a corner, and a circle with another piece.
func TestPathPatternClosedRefused(t *testing.T) {
	d := api.New()
	d.Features.SketchIn(d.Workplane("XY"), "square")
	sq := d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	d.Features.SketchIn(d.Workplane("XY"), "ring")
	ring := d.Sketch.Circle(api.V(0, 0, 0), 20)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 1)
	d.Features.Extrude(5, "post")
	for name, c := range map[string]struct {
		path []api.Entity
		want string
	}{
		"corner":           {sq, "corner"},
		"circle and lines": {append([]api.Entity{ring}, sq[0]), "closes on itself"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			d.Features.PatternAlongPath(c.path, 3, "")
		}()
	}
}

// TestPathPatternClosedSweep repeats a post round circles and rounded
// rectangles of several sizes and counts, and requires a sound solid of
// n posts' volume. The rate allowed was fixed at none before the
// closed path was written.
func TestPathPatternClosedSweep(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("copies", 2, 3, 5, 12),
			sweep.Floats("size", 6, 20, 100),
			sweep.Strings("path", "circle", "track"),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		n, size := int(p.Float("copies")), p.Float("size")
		d := api.New()
		d.Features.SketchIn(d.Workplane("XY"), "path")
		var path []api.Entity
		var at api.Vec
		if p.String("path") == "circle" {
			path = []api.Entity{d.Sketch.Circle(api.V(0, 0, 0), size)}
			at = api.V(0, -size, 0)
		} else {
			sides := d.Sketch.Rect(api.V(0, 0, 0), api.V(2*size, size, 0))
			for _, s := range sides {
				d.Sketch.TangentArc(s.Start(), size/4, true)
			}
			for _, e := range d.Active().Entities() {
				if k := e.Kind(); k == "line" || k == "arc" {
					path = append(path, e)
				}
			}
			at = api.V(size/3, 0, 0)
		}
		d.Features.SketchIn(d.Workplane("XY"), "post-sketch")
		d.Sketch.Circle(at, 1)
		one := d.Features.Extrude(2, "post").Analyze().Check().Volume
		g := d.Features.PatternAlongPath(path, n, "")
		r := g.Analyze().Check()
		switch {
		case g.Problem() != "":
			return fmt.Errorf("problem: %s", g.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case math.Abs(r.Volume-float64(n)*one) > 1e-6*float64(n)*one:
			return fmt.Errorf("volume %g, want %g", r.Volume, float64(n)*one)
		}
		return nil
	}).Require(t)
}

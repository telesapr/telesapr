package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/internal/sweep"
	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// chordLoss bounds how much less a mesh holds than a fillet's exact
// solid: an arc of angle a and radius r drawn with at least 8 chords
// loses at most r^2 a^3 / (12 * 64) of its section, along length l.
func chordLoss(r, a, l float64) float64 { return r * r * a * a * a / 768 * l }

// filletLoss is the volume a fillet of radius r takes from a straight
// edge of length l whose faces meet at interior angle beta.
func filletLoss(r, beta, l float64) float64 {
	s := r / math.Tan(beta/2)
	return l * (r*s - r*r*(math.Pi-beta)/2)
}

// fineChordFillet sets a fine chord tolerance, so volumes are near the
// formula's: the arc's chords cut inside by up to the tolerance.
func fineChordFillet(d *api.Doc) {
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
}

func TestFilletOneEdge(t *testing.T) {
	d := api.New()
	fineChordFillet(d)
	ex := block(d, 0, 0, 10, 10, 5)
	top := lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))
	step(d)
	before := saved(t, d)
	f := d.Fillet([]api.Entity{top}, 1, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	r := f.Check()
	want := 500 - filletLoss(1, math.Pi/2, 10)
	tol := func(r float64) float64 { return chordLoss(r, math.Pi/2, 10) }
	if !r.Sound() || math.Abs(r.Volume-want) > tol(1)+1e-9 || f.Kind() != "fillet" {
		t.Fatalf("filleted block: %s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, before)
	f.SetRadius(2)
	if v := f.Check().Volume; math.Abs(v-(500-filletLoss(2, math.Pi/2, 10))) > tol(2)+1e-9 || f.Radius() != 2 {
		t.Fatalf("at radius 2 the volume is %g", v)
	}
	if err := f.SetExpr(sketch.GroupPitch, "1+0.5"); err != nil {
		t.Fatal(err)
	}
	d.Regen()
	if f.Radius() != 1.5 {
		t.Fatalf("radius %g", f.Radius())
	}
	if err := f.SetExpr(sketch.GroupPitch, "0"); err == nil {
		t.Fatal("a radius of 0 was taken")
	}
	// The file holds the solid's curves at the sketch's tolerance, which
	// a reopened file has at its default.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	back := roundTrip(t, d)
	bf := back.Group(f.Name())
	if bf.Kind() != "fillet" || len(bf.Edges()) != 1 || bf.Expr(sketch.GroupPitch) != "1+0.5" {
		t.Fatalf("reopened: %s, %d edges, %q", bf.Kind(), len(bf.Edges()), bf.Expr(sketch.GroupPitch))
	}
	// The fillet's own edges have names.
	s := f.Solid()
	if len(s.Problems()) != 0 || s.Euler() != 2 {
		t.Fatalf("topology: %v, Euler %d", s.Problems(), s.Euler())
	}
	for _, e := range s.Edges() {
		if e.Name() == "" {
			t.Fatal("an edge with no name")
		}
	}
}

// A Boolean's edge, by name; and two parallel edges at once.
func TestFilletSolidEdges(t *testing.T) {
	d, err := api.Open("../../testdata/topo/slot.spcs")
	if err != nil {
		t.Fatal(err)
	}
	fineChordFillet(d)
	s := d.Group("slot").Solid()
	upright := s.EdgeAt(api.V(15, 0, 5))
	f := d.FilletSolid([]api.SolidEdge{upright}, 2, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	r := f.Check()
	want := 10000 - filletLoss(2, math.Pi/2, 10)
	if !r.Sound() || math.Abs(r.Volume-want) > chordLoss(2, math.Pi/2, 10) {
		t.Fatalf("%s, want %g", r.String(), want)
	}
	if len(f.SolidEdges()) != 1 {
		t.Fatal("the fillet's edges")
	}
	// It follows the notch.
	if err := d.SetParameter("slot", "8"); err != nil {
		t.Fatal(err)
	}
	if p := f.Problem(); p != "" {
		t.Fatalf("after the notch moved: %s", p)
	}
	a, b := f.SolidEdges()[0].Ends()
	if math.Abs(a.X-8) > 1e-9 || math.Abs(b.X-8) > 1e-9 {
		t.Fatalf("the edge is at x = %g, %g", a.X, b.X)
	}
}

func TestFilletRefused(t *testing.T) {
	d := api.New()
	d.Polyline(true, api.V(0, 0, 0), api.V(20, 0, 0), api.V(20, 5, 0), api.V(5, 5, 0), api.V(5, 20, 0), api.V(0, 20, 0))
	ex := d.Extrude(5, "")
	for name, c := range map[string]struct {
		fn   func()
		want string
	}{
		"concave":           {func() { d.Fillet([]api.Entity{lineAlong(t, ex, api.V(5, 5, 0), api.V(5, 5, 5))}, 1, "") }, "concave"},
		"no edges":          {func() { d.Fillet(nil, 1, "") }, "no edges"},
		"zero":              {func() { d.Fillet([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(20, 0, 5))}, 0, "") }, "more than zero"},
		"too large":         {func() { d.Fillet([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(20, 0, 5))}, 50, "") }, "wider than a face"},
		"radius of extrude": {func() { ex.Radius() }, "fillet"},
		"bad radius": {func() {
			d.Fillet([]api.Entity{lineAlong(t, ex, api.V(0, 20, 5), api.V(5, 20, 5))}, 1, "").SetRadius(-1)
		}, "more than zero"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			c.fn()
		}()
	}
}

// A fillet that stops fitting, as when the solid is made thinner, is a
// problem on the group, and the solid passes through.
func TestFilletProblem(t *testing.T) {
	d := api.New()
	sides := d.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	width := d.Length(sides[1], 10)
	ex := d.Extrude(5, "")
	f := d.Fillet([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))}, 2, "")
	width.Set(1)
	if p := f.Problem(); !strings.Contains(p, "wider than a face") {
		t.Fatalf("problem %q", p)
	}
	if v := f.Check().Volume; math.Abs(v-1*10*5) > 1e-6 {
		t.Fatalf("with a problem the fillet took %g away", 50-v)
	}
}

// TestFilletSweep rounds the edge at a corner of angle angle of a
// triangular prism of length length, at a fraction k of the largest
// radius that fits, and requires a sound solid of the formula's
// volume. The rate allowed, 0, was fixed before the group was built.
func TestFilletSweep(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("angle", 20, 45, 60, 90, 120, 150, 165),
			sweep.Floats("k", 0.05, 0.4, 0.8),
			sweep.Floats("length", 0.5, 5, 40),
		},
		MaxFailRate: 0,
		Sample:      20,
	}
	grid.Run(func(p sweep.Point) error {
		th := p.Float("angle") * math.Pi / 180
		length := p.Float("length")
		r := p.Float("k") * 10 * math.Tan(th/2)
		d := api.New()
		fineChordFillet(d)
		d.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(10*math.Cos(th), 10*math.Sin(th), 0))
		ex := d.Extrude(length, "")
		f := d.Fillet([]api.Entity{lineAlong(t, ex, api.V(0, 0, 0), api.V(0, 0, length))}, r, "")
		rep := f.Check()
		want := 50*math.Sin(th)*length - filletLoss(r, th, length)
		allow := chordLoss(r, math.Pi-th, length) + 1e-9*want
		switch {
		case f.Problem() != "":
			return fmt.Errorf("problem: %s", f.Problem())
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case math.Abs(rep.Volume-want) > allow:
			return fmt.Errorf("volume %g, want %g", rep.Volume, want)
		}
		return nil
	}).Require(t)
}

// A fillet's faces join later Booleans: a boss on the top face over the
// rounded edge, and a cut through the round.
func TestFilletThenBoolean(t *testing.T) {
	for _, cut := range []bool{false, true} {
		d := api.New()
		ex := block(d, 0, 0, 20, 10, 10)
		d.Fillet([]api.Entity{lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))}, 2, "")
		d.SketchIn(d.Workplane("XY"), "")
		var g api.Group
		if cut {
			d.Rect(api.V(5, -1, 0), api.V(8, 4, 0))
			g = d.Extrude(20, "")
			g.SetCombine(api.Difference)
		} else {
			d.Rect(api.V(5, 1, 0), api.V(8, 4, 0))
			g = d.Extrude(14, "")
		}
		if r := g.Check(); !r.Sound() || g.Problem() != "" {
			t.Errorf("cut %v: %s %s", cut, r.String(), g.Problem())
		}
	}
}

// boxCornerLoss is what fillets of radius r take off a box: each edge its
// cross section times its length, less at a corner where two meet the
// part both count, and at one where three meet, the corner cube they
// count three times less the sphere's eighth (as OpenCASCADE gives,
// pkg/topo's TestFilletCorners).
func boxCornerLoss(r, length float64, mitres, spheres int) float64 {
	a := r * r * (1 - math.Pi/4)
	return a*length - float64(mitres)*r*r*r*(5.0/3-math.Pi/2) + float64(spheres)*r*r*r*(7*math.Pi/12-2)
}

// Edges that meet at a block's corner: two (a mitre) and three (a
// sphere), through the group, undone and redone and reopened.
func TestFilletCorners(t *testing.T) {
	for _, three := range []bool{false, true} {
		d := api.New()
		fineChordFillet(d)
		ex := block(d, 0, 0, 20, 10, 10)
		edges := []api.Entity{lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10)),
			lineAlong(t, ex, api.V(20, 0, 10), api.V(20, 10, 10))}
		length, mitres, spheres := 30.0, 1, 0
		if three {
			edges = append(edges, lineAlong(t, ex, api.V(20, 0, 0), api.V(20, 0, 10)))
			length, mitres, spheres = 40, 0, 1
		}
		step(d)
		before := saved(t, d)
		f := d.Fillet(edges, 2, "")
		if p := f.Problem(); p != "" {
			t.Fatal(p)
		}
		r := f.Check()
		loss := boxCornerLoss(2, length, mitres, spheres)
		want := 2000 - loss
		if !r.Sound() || math.Abs(r.Volume-want) > 0.01*loss {
			t.Fatalf("three %v: %s, want volume %g", three, r.String(), want)
		}
		undoRedo(t, d, before)
		d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
		back := roundTrip(t, d)
		if br := back.Group(f.Name()).Check(); !br.Sound() {
			t.Fatalf("three %v reopened: %s", three, br.String())
		}
	}
}

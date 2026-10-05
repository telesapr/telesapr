package sketch_test

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
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
	d, err := api.Open("../../testdata/topo/slot.tlspr")
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
		"convex and concave": {func() {
			d.Fillet([]api.Entity{lineAlong(t, ex, api.V(5, 5, 0), api.V(5, 5, 5)), lineAlong(t, ex, api.V(5, 5, 5), api.V(20, 5, 5))}, 1, "")
		}, "a convex edge's fillet meets a concave"},
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

// A cut through a fillet loses no more to its chords than the fillet's
// own mesh does over what is left of it. At the default tolerance it
// lost 0.34 mm³ more, a fiftieth of a percent, which a fine one takes
// away: the chords of the arcs the cut makes on the cylinder, not a
// fault of the Boolean (roadmap 13.3).
func TestFilletCutChords(t *testing.T) {
	d := api.New()
	fineChordFillet(d)
	ex := block(d, 0, 0, 20, 10, 10)
	f := d.Fillet([]api.Entity{lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))}, 2, "")
	a := 4 * (1 - math.Pi/4)
	filletErr := f.Check().Volume - (2000 - 20*a)
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(api.V(5, -1, 0), api.V(8, 4, 0))
	g := d.Extrude(20, "")
	g.SetCombine(api.Difference)
	cutErr := g.Check().Volume - (2000 - 20*a - (120 - 3*a))
	if extra := cutErr - filletErr*17/20; math.Abs(extra) > 0.005 {
		t.Errorf("the cut loses %g mm³ more than the fillet's chords (fillet %g, cut %g)", extra, filletErr, cutErr)
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

// circleOf is the circle entity of an extrusion of a circle at height z.
func circleOf(t *testing.T, ex api.Group, z float64) api.Entity {
	t.Helper()
	for _, e := range ex.Entities() {
		if e.Kind() == "circle" && math.Abs(e.Center().Pos().Z-z) < 1e-9 {
			return e
		}
	}
	t.Fatalf("no circle of %s at z = %g", ex.Name(), z)
	return api.Entity{}
}

// spandrelLoss is what a fillet of radius r takes from a circular edge
// of radius rad, by Pappus: the area between the corner and the arc,
// turned about the axis by its centroid, inside the circle for a boss
// and outside it for a hole.
func spandrelLoss(rad, r float64, hole bool) float64 {
	a := r * r * (1 - math.Pi/4)
	c := r * (10 - 3*math.Pi) / (12 - 3*math.Pi)
	if hole {
		return a * 2 * math.Pi * (rad + c)
	}
	return a * 2 * math.Pi * (rad - c)
}

// A cylinder's top circle rounded and chamfered, by its entity and by
// one of its solid edges, through the groups; the loss by Pappus.
func TestFilletCircle(t *testing.T) {
	for _, flat := range []bool{false, true} {
		for _, byName := range []bool{false, true} {
			d := api.New()
			d.SetChordTolerance(0.005)
			d.Circle(api.V(0, 0, 0), 5)
			ex := d.Extrude(10, "")
			before := ex.Check().Volume
			step(d)
			saveBefore := saved(t, d)
			var g api.Group
			switch {
			case flat && byName:
				g = d.ChamferSolid([]api.SolidEdge{ex.Solid().EdgeAt(api.V(5, 0, 10))}, 2, "")
			case flat:
				g = d.Chamfer([]api.Entity{circleOf(t, ex, 10)}, 2, "")
			case byName:
				g = d.FilletSolid([]api.SolidEdge{ex.Solid().EdgeAt(api.V(5, 0, 10))}, 2, "")
			default:
				g = d.Fillet([]api.Entity{circleOf(t, ex, 10)}, 2, "")
			}
			if p := g.Problem(); p != "" {
				t.Fatalf("flat %v by name %v: %s", flat, byName, p)
			}
			r := g.Check()
			want := spandrelLoss(5, 2, false)
			if flat {
				want = 2 * 2 / 2.0 * 2 * math.Pi * (5 - 2/3.0)
			}
			if !r.Sound() || math.Abs(before-r.Volume-want) > 0.02*want {
				t.Fatalf("flat %v by name %v: %s, took %g, want %g", flat, byName, r.String(), before-r.Volume, want)
			}
			undoRedo(t, d, saveBefore)
			// A reopened file meshes at the default tolerance.
			d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
			d.Regen()
			r = g.Check()
			back := roundTrip(t, d)
			if br := back.Group(g.Name()).Check(); !br.Sound() || math.Abs(br.Volume-r.Volume) > 1e-6*r.Volume {
				t.Fatalf("flat %v by name %v reopened: %s", flat, byName, br.String())
			}
		}
	}
}

// A boss on a block: its top circle and a top edge of the block
// chamfered in one group, the circle by a cone, the edge by a wedge.
func TestChamferCircleAndEdge(t *testing.T) {
	d := api.New()
	d.SetChordTolerance(0.005)
	block(d, -10, -10, 10, 10, 5)
	d.SketchAt(above(d, 5), api.V(1, 0, 0), api.V(0, 1, 0), "")
	d.Circle(api.V(0, 0, 5), 4)
	boss := d.Extrude(5, "")
	before := boss.Check().Volume
	edge := boss.Solid().EdgeAt(api.V(0, -10, 5))
	c := d.ChamferSolid([]api.SolidEdge{boss.Solid().EdgeAt(api.V(4, 0, 10)), edge}, 1, "")
	if p := c.Problem(); p != "" {
		t.Fatal(p)
	}
	r := c.Check()
	want := 1.0/2*2*math.Pi*(4-1/3.0) + 1.0/2*20
	if !r.Sound() || math.Abs(before-r.Volume-want) > 0.02*want {
		t.Fatalf("%s, took %g, want %g", r.String(), before-r.Volume, want)
	}
}

// Edges that go in, filled through the group: a square boss's foot on a
// block, its four edges by name meeting in mitres outside its corners,
// and a round boss's foot by its circle entity, a torus by Pappus.
func TestFilletConcave(t *testing.T) {
	for _, round := range []bool{false, true} {
		d := api.New()
		d.SetChordTolerance(0.005)
		block(d, -10, -10, 10, 10, 5)
		d.SketchAt(above(d, 5), api.V(1, 0, 0), api.V(0, 1, 0), "")
		if round {
			d.Circle(api.V(0, 0, 5), 4)
		} else {
			d.Rect(api.V(-4, -4, 5), api.V(4, 4, 5))
		}
		boss := d.Extrude(5, "")
		before := boss.Check().Volume
		step(d)
		saveBefore := saved(t, d)
		var f api.Group
		var want float64
		if round {
			f = d.Fillet([]api.Entity{circleOf(t, boss, 5)}, 1.5, "")
			a := 1.5 * 1.5 * (1 - math.Pi/4)
			c := 1.5 * (10 - 3*math.Pi) / (12 - 3*math.Pi)
			want = a * 2 * math.Pi * (4 + c)
		} else {
			var edges []api.SolidEdge
			for _, p := range []api.Vec{api.V(0, -4, 5), api.V(4, 0, 5), api.V(0, 4, 5), api.V(-4, 0, 5)} {
				edges = append(edges, boss.Solid().EdgeAt(p))
			}
			f = d.FilletSolid(edges, 1.5, "")
			// The strips run on past the boss's corners to the mitres: as a
			// box's mitres, counted the other way.
			want = boxCornerLoss(1.5, 32, -4, 0)
		}
		if p := f.Problem(); p != "" {
			t.Fatalf("round %v: %s", round, p)
		}
		r := f.Check()
		if !r.Sound() || math.Abs(r.Volume-before-want) > 0.02*want {
			t.Fatalf("round %v: %s, added %g, want %g", round, r.String(), r.Volume-before, want)
		}
		undoRedo(t, d, saveBefore)
		d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
		d.Regen()
		r = f.Check()
		back := roundTrip(t, d)
		if br := back.Group(f.Name()).Check(); !br.Sound() || math.Abs(br.Volume-r.Volume) > 1e-6*r.Volume {
			t.Fatalf("round %v reopened: %s", round, br.String())
		}
	}
}

// A rectangle with its corners rounded by tangent arcs, extruded, and
// one top line picked: the fillet runs on round the outline, through the
// group, undone and redone and reopened; the loss along the centroid's
// path.
func TestFilletRoundedOutline(t *testing.T) {
	d := api.New()
	d.SetChordTolerance(0.005)
	sides := d.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	for _, s := range sides {
		d.TangentArc(s.Start(), 4, true)
	}
	ex := d.Extrude(10, "")
	before := ex.Check().Volume
	step(d)
	saveBefore := saved(t, d)
	f := d.Fillet([]api.Entity{lineAlong(t, ex, api.V(4, 0, 10), api.V(26, 0, 10))}, 2, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	a := 2 * 2 * (1 - math.Pi/4)
	c := 2 * (10 - 3*math.Pi) / (12 - 3*math.Pi)
	want := a * (2*22 + 2*12 + 2*math.Pi*(4-c))
	r := f.Check()
	if !r.Sound() || math.Abs(before-r.Volume-want) > 0.02*want {
		t.Fatalf("%s, took %g, want %g", r.String(), before-r.Volume, want)
	}
	undoRedo(t, d, saveBefore)
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	d.Regen()
	r = f.Check()
	back := roundTrip(t, d)
	if br := back.Group(f.Name()).Check(); !br.Sound() || math.Abs(br.Volume-r.Volume) > 1e-6*r.Volume {
		t.Fatalf("reopened: %s", br.String())
	}
}

// A ramp: a trapezoid drawn upright and extruded, its top's front edge
// and its edge along the slope rounded, meeting in an uneven mitre (the
// slope leans, the front doesn't); the volume as OpenCASCADE gives, whose
// curves are exact here.
func TestFilletUnevenMitre(t *testing.T) {
	d := api.New()
	d.SetChordTolerance(0.005)
	d.SketchIn(d.Workplane("ZX"), "")
	d.Polyline(true, api.V(0, 0, 0), api.V(20, 0, 0), api.V(15, 0, 10), api.V(0, 0, 10))
	ex := d.Extrude(10, "")
	s := ex.Solid()
	front, slope := s.EdgeAt(api.V(7, 0, 10)), s.EdgeAt(api.V(15, 5, 10))
	if a, b := slope.Ends(); a.Y < -1 || b.Y < -1 {
		// The extrusion runs to -y.
		slope = s.EdgeAt(api.V(15, -5, 10))
	}
	step(d)
	saveBefore := saved(t, d)
	f := d.FilletSolid([]api.SolidEdge{front, slope}, 2, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	r := f.Check()
	if want := 1734.655169; !r.Sound() || math.Abs(r.Volume-want) > 0.5 {
		t.Fatalf("%s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, saveBefore)
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	back := roundTrip(t, d)
	if br := back.Group(f.Name()).Check(); !br.Sound() {
		t.Fatalf("reopened: %s", br.String())
	}
}

// A bracket's rib, its foot filled all round: each side's fillet runs
// along the floor, round the bend as a torus and up the wall, and at
// the rib's four corners meets the fillet across an end of the rib's
// top below an edge of the top that is left sharp and sticks out, where
// the two go in. They meet in an uneven mitre outside the rib, the top
// growing out to it. What the corners hold beyond the sides' fillets
// and the ends', each made alone, is the least of the two fillets'
// heights less the side's fillet as the top's plane cuts it, summed
// over the floor beside the rib (pkg/topo's TestFilletRibFootSweep sums
// the like): 0.0600158 mm³ a corner at 2 mm under a top at 35 degrees.
func TestFilletRibFoot(t *testing.T) {
	volume := func(sides, ends bool) float64 {
		d := api.New()
		d.SetChordTolerance(0.001)
		f := filletRib(d, 2, sides, ends)
		if p := f.Problem(); p != "" {
			t.Fatalf("sides %v, ends %v: %s", sides, ends, p)
		}
		r := f.Check()
		if !r.Sound() {
			t.Fatalf("sides %v, ends %v: %s", sides, ends, r.String())
		}
		return r.Volume
	}
	d := api.New()
	d.SetChordTolerance(0.001)
	rib, edges := ribbed(d)
	bare := rib.Volume()
	step(d)
	saveBefore := saved(t, d)
	f := d.FilletSolid(edges(rib.Solid(), true, true), 2, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	r := f.Check()
	corners := r.Volume - volume(true, false) - volume(false, true) + bare
	if want := 4 * 0.0600158; !r.Sound() || math.Abs(corners-want) > 0.1*want {
		t.Fatalf("%s: the corners hold %g, want %g", r.String(), corners, want)
	}
	// The ends' fillets alone, by their section: the top meets the floor
	// and the wall at 145 degrees outside the rib.
	if got, want := volume(false, true)-bare, 2*filletLoss(2, 145*math.Pi/180, 10); math.Abs(got-want) > 0.01 {
		t.Fatalf("the ends' fillets hold %g, want %g", got, want)
	}
	undoRedo(t, d, saveBefore)

	// A slot cut down through a side's fillet where it ends on the top,
	// which grew there: the Boolean finds the grown face.
	step(d)
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(api.V(24, 10.5, 0), api.V(25, 14, 0))
	slot := d.Extrude(30, "slot")
	slot.SetTwoSided(true)
	slot.SetCombine(api.Difference)
	d.Regen()
	if sr := slot.Check(); slot.Problem() != "" || !sr.Sound() || sr.Volume > r.Volume-1 {
		t.Fatalf("the slot: %s %s", sr.String(), slot.Problem())
	}
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	d.Regen()
	sr := slot.Check()
	back := roundTrip(t, d)
	if br := back.Group("slot").Check(); !br.Sound() || math.Abs(br.Volume-sr.Volume) > 1e-6*sr.Volume {
		t.Fatalf("reopened: %s", br.String())
	}
}

// A frustum turned by a lathe from a trapezoid in the ZX plane, its
// top's circle, where the flat top meets the cone, rounded by name
// through the group; the volume by Pappus.
func TestFilletCone(t *testing.T) {
	d := api.New()
	d.SetChordTolerance(0.005)
	d.SketchIn(d.Workplane("ZX"), "")
	axis := d.Line(api.V(0, 0, -2), api.V(0, 0, 12))
	axis.SetConstruction(true)
	d.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(6, 0, 8), api.V(0, 0, 8))
	lathe := d.Lathe(axis, "")
	before := lathe.Check().Volume
	step(d)
	saveBefore := saved(t, d)
	f := d.FilletSolid([]api.SolidEdge{lathe.Solid().EdgeAt(api.V(6*math.Cos(1), 6*math.Sin(1), 8))}, 1.5, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	// In the half plane through the axis: the top runs in, the cone out
	// and down; the section's spandrel turned about the axis.
	beta := math.Pi - math.Atan2(8, 4)
	s := 1.5 / math.Tan(beta/2)
	g := [2]float64{4 / math.Hypot(4, 8), -8 / math.Hypot(4, 8)}
	bis := [2]float64{-1 + g[0], g[1]}
	l := math.Hypot(bis[0], bis[1])
	c := [2]float64{6 + bis[0]/l*1.5/math.Sin(beta/2), bis[1] / l * 1.5 / math.Sin(beta/2)}
	kiteX := ((6 + 6 - s + c[0]) + (6 + c[0] + 6 + s*g[0])) / 6
	alpha := (math.Pi - beta) / 2
	sectorX := c[0] - bis[0]/l*2*1.5*math.Sin(alpha)/(3*alpha)
	want := 2 * math.Pi * (1.5*s*kiteX - alpha*1.5*1.5*sectorX)
	r := f.Check()
	if !r.Sound() || math.Abs(before-r.Volume-want) > 0.03*want {
		t.Fatalf("%s, took %g, want %g", r.String(), before-r.Volume, want)
	}
	undoRedo(t, d, saveBefore)
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	d.Regen()
	r = f.Check()
	back := roundTrip(t, d)
	if br := back.Group(f.Name()).Check(); !br.Sound() || math.Abs(br.Volume-r.Volume) > 1e-6*r.Volume {
		t.Fatalf("reopened: %s", br.String())
	}
}

// One fillet group, edges at radii of their own: a block's two top
// long edges at the group's radius, its foot's front edge at one of its
// own, and its foot's back edge from a parameter; undone, redone,
// reopened, and the parameter changed.
func TestFilletEdgeRadii(t *testing.T) {
	d := api.New()
	fineChordFillet(d)
	if err := d.SetParameter("r", "1.5"); err != nil {
		t.Fatal(err)
	}
	ex := block(d, 0, 0, 20, 10, 10)
	f := d.Fillet([]api.Entity{
		lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10)),
		lineAlong(t, ex, api.V(0, 10, 10), api.V(20, 10, 10)),
		lineAlong(t, ex, api.V(0, 0, 0), api.V(20, 0, 0)),
		lineAlong(t, ex, api.V(0, 10, 0), api.V(20, 10, 0)),
	}, 2, "")
	step(d)
	before := saved(t, d)
	f.SetEdgeRadius(2, 1)
	if err := f.SetEdgeRadiusExpr(3, "r*2"); err != nil {
		t.Fatal(err)
	}
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	check := func(what string, r3 float64) {
		t.Helper()
		loss := 2*filletLoss(2, math.Pi/2, 20) + filletLoss(1, math.Pi/2, 20) + filletLoss(r3, math.Pi/2, 20)
		want := 2000 - loss
		if r := f.Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.01*loss {
			t.Fatalf("%s: %s, want volume %g", what, r.String(), want)
		}
		if got := f.EdgeRadius(3); got != r3 || f.EdgeRadius(0) != 2 || f.EdgeRadius(2) != 1 {
			t.Fatalf("%s: radii %g %g %g", what, f.EdgeRadius(0), f.EdgeRadius(2), got)
		}
	}
	check("set", 3)
	if err := d.SetParameter("r", "1"); err != nil {
		t.Fatal(err)
	}
	check("parameter changed", 2)
	if err := f.SetEdgeRadiusExpr(3, "-r"); err == nil {
		t.Error("a radius below zero is taken")
	}
	undoRedo(t, d, before)
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	back := roundTrip(t, d)
	bf := back.Group(f.Name())
	if br := bf.Check(); bf.EdgeRadius(2) != 1 || bf.EdgeRadius(3) != 2 || !br.Sound() {
		t.Fatalf("reopened: radii %g %g", bf.EdgeRadius(2), bf.EdgeRadius(3))
	}
	// In inches, the sizes stay, and the expression gains the factor;
	// reopened, the same (a plain length may move by an ulp on its first
	// reload in inches, as any does).
	d.SetUnits(core.UnitInches)
	if math.Abs(f.EdgeRadius(3)-2) > 1e-9 || math.Abs(f.EdgeRadius(2)-1) > 1e-9 {
		t.Fatalf("in inches: radii %g %g", f.EdgeRadius(2), f.EdgeRadius(3))
	}
	file := filepath.Join(t.TempDir(), "inches.tlspr")
	if err := d.Save(file); err != nil {
		t.Fatal(err)
	}
	inches, err := api.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	if bi := inches.Group(f.Name()); math.Abs(bi.EdgeRadius(3)-2) > 1e-9 || math.Abs(bi.EdgeRadius(2)-1) > 1e-9 {
		t.Fatalf("reopened in inches: radii %g %g", bi.EdgeRadius(2), bi.EdgeRadius(3))
	}
	d.SetUnits(core.UnitMM)
	// Back to the group's radius.
	f.SetEdgeRadius(2, 0)
	if f.EdgeRadius(2) != 2 {
		t.Errorf("edge 2 at %g after its own radius is taken off", f.EdgeRadius(2))
	}
	// Two that meet must have one radius.
	d = api.New()
	ex = block(d, 0, 0, 20, 10, 10)
	g := d.Fillet([]api.Entity{
		lineAlong(t, ex, api.V(0, 0, 10), api.V(0, 10, 10)),
		lineAlong(t, ex, api.V(0, 0, 0), api.V(0, 0, 10)),
	}, 1, "")
	g.SetEdgeRadius(1, 2)
	if p := g.Problem(); !strings.Contains(p, "radius 1 and 2 meet") {
		t.Errorf("a corner of two radii: %q", p)
	}
}

// The lines where a fillet meets the faces it rounds are drawn, though
// the faces meet smoothly there: their triangles carry other faces
// (roadmap 14.1).
func TestFilletBordersDrawn(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	d.Fillet([]api.Entity{lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))}, 2, "")
	var top, front float64
	d.With(func(sk *sketch.Sketch) {
		g := sk.GetGroup(sk.Active)
		g.GenerateDisplayItems(sk)
		for _, o := range g.DisplayOutlines.Outlines {
			if o.Tag == 0 {
				continue
			}
			l := o.B.Minus(o.A).Magnitude()
			switch {
			case near(o.A.Y, 2) && near(o.B.Y, 2) && near(o.A.Z, 10) && near(o.B.Z, 10):
				top += l
			case near(o.A.Y, 0) && near(o.B.Y, 0) && near(o.A.Z, 8) && near(o.B.Z, 8):
				front += l
			}
		}
	})
	if !near(top, 20) || !near(front, 20) {
		t.Errorf("drawn along the fillet's borders: %g on the top, %g on the front; want 20 and 20", top, front)
	}
}

// near reports whether a and b agree to a millionth of a millimetre.
func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// A fillet whose radius is the rounded outline's corner radius can't go
// on round the corner, and says so, rather than naming a curved face at
// the edge's end (roadmap 15.7).
func TestFilletStoppedAtRoundedCorner(t *testing.T) {
	d := api.New()
	sides := d.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	for _, s := range sides {
		d.TangentArc(s.Start(), 4, true)
	}
	ex := d.Extrude(10, "")
	defer func() {
		err, _ := recover().(error)
		if err == nil || !strings.Contains(err.Error(), "stops at a rounded corner") ||
			!strings.Contains(err.Error(), "smaller than the corner's radius") {
			t.Errorf("refused with %v", err)
		}
	}()
	d.Fillet([]api.Entity{lineAlong(t, ex, api.V(4, 0, 10), api.V(26, 0, 10))}, 4, "")
}

// spandrelRevolved is the volume a fillet of radius r takes from (or
// adds to) a corner at p, in the half plane through an axis as
// (radius, height), between directions g0 and g1 along its two faces:
// the kite p, the two tangent points and the arc's centre, less the
// arc's sector, turned about the axis (Pappus).
func spandrelRevolved(p, g0, g1 [2]float64, r float64) float64 {
	norm := func(a [2]float64) [2]float64 { l := math.Hypot(a[0], a[1]); return [2]float64{a[0] / l, a[1] / l} }
	g0, g1 = norm(g0), norm(g1)
	beta := math.Acos(g0[0]*g1[0] + g0[1]*g1[1])
	s := r / math.Tan(beta/2)
	bis := norm([2]float64{g0[0] + g1[0], g0[1] + g1[1]})
	c := [2]float64{p[0] + bis[0]*r/math.Sin(beta/2), p[1] + bis[1]*r/math.Sin(beta/2)}
	t0 := [2]float64{p[0] + s*g0[0], p[1] + s*g0[1]}
	t1 := [2]float64{p[0] + s*g1[0], p[1] + s*g1[1]}
	tri := r * s / 2
	kiteX := tri*(p[0]+t0[0]+c[0])/3 + tri*(p[0]+c[0]+t1[0])/3
	phi := math.Pi - beta
	sectorA := r * r * phi / 2
	// The sector's centroid lies from c towards p.
	sectorX := c[0] - bis[0]*2*r*math.Sin(phi/2)/(3*phi/2)
	return 2 * math.Pi * (kiteX - sectorA*sectorX)
}

// A circle where a cone meets a cylinder about one axis (roadmap 22.2),
// rounded by a torus: a frustum on a shaft, whose rim sticks out, and a
// shaft from a cone's top, whose foot goes in; against Pappus, undone,
// redone and reopened.
func TestFilletConeOnCylinder(t *testing.T) {
	for _, c := range []struct {
		name    string
		profile []api.Vec
		edge    api.Vec
		corner  [2]float64
		g0, g1  [2]float64
		convex  bool
	}{
		{"frustum on a shaft", []api.Vec{api.V(0, 0, 0), api.V(10, 0, 0), api.V(10, 0, 20), api.V(5, 0, 30), api.V(0, 0, 30)},
			api.V(10*math.Cos(1), 10*math.Sin(1), 20), [2]float64{10, 20}, [2]float64{0, -1}, [2]float64{-5, 10}, true},
		{"shaft from a cone", []api.Vec{api.V(0, 0, 0), api.V(10, 0, 0), api.V(5, 0, 10), api.V(5, 0, 20), api.V(0, 0, 20)},
			api.V(5*math.Cos(1), 5*math.Sin(1), 10), [2]float64{5, 10}, [2]float64{5, -10}, [2]float64{0, 1}, false},
	} {
		d := api.New()
		d.SetChordTolerance(0.001)
		d.SketchIn(d.Workplane("ZX"), "")
		axis := d.Line(api.V(0, 0, -2), api.V(0, 0, 32))
		axis.SetConstruction(true)
		d.Polyline(true, c.profile...)
		lathe := d.Lathe(axis, "")
		before := lathe.Check().Volume
		step(d)
		saveBefore := saved(t, d)
		f := d.FilletSolid([]api.SolidEdge{lathe.Solid().EdgeAt(c.edge)}, 3, "")
		if p := f.Problem(); p != "" {
			t.Fatalf("%s: %s", c.name, p)
		}
		want := spandrelRevolved(c.corner, c.g0, c.g1, 3)
		if c.convex {
			want = -want
		}
		r := f.Check()
		if !r.Sound() || math.Abs(r.Volume-before-want) > 0.03*math.Abs(want) {
			t.Fatalf("%s: %s, changed by %g, want %g", c.name, r.String(), r.Volume-before, want)
		}
		undoRedo(t, d, saveBefore)
		d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
		d.Regen()
		r = f.Check()
		back := roundTrip(t, d)
		if br := back.Group(f.Name()).Check(); !br.Sound() || math.Abs(br.Volume-r.Volume) > 1e-6*r.Volume {
			t.Fatalf("%s: reopened: %s", c.name, br.String())
		}
		// The same edge chamfered: a cone's band in place of the torus.
		f.Delete()
		ch := d.ChamferSolid([]api.SolidEdge{lathe.Solid().EdgeAt(c.edge)}, 3, "")
		if r := ch.Check(); ch.Problem() != "" || !r.Sound() {
			t.Fatalf("%s: chamfered: %s %s", c.name, r.String(), ch.Problem())
		}
	}
}

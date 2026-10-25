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

// One side of a block drafted (roadmap 28.2): only the front leans, the
// rest stay upright; undo, redo, a round trip; all sides again.
func TestDraftChosenSides(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	ex.AsExtrude().SetDraft(10)
	front := lineAlong(t, ex, api.V(0, 0, 0), api.V(20, 0, 0))
	step(d)
	before := saved(t, d)
	ex.AsExtrude().SetDraftFaces([]api.Entity{front})
	tan := math.Tan(10 * math.Pi / 180)
	want := 20 * (100 - 50*tan)
	r := ex.Analyze().Check()
	if ex.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 1e-6*want {
		t.Fatalf("one side drafted: %q, %s, want %g", ex.Problem(), r.String(), want)
	}
	if fs := ex.AsExtrude().DraftFaces(); len(fs) != 1 || fs[0].Group().H == ex.H {
		t.Errorf("the draft's sides: %v", fs)
	}
	// The back and the ends stay upright: the bounds are the block's.
	if math.Abs(r.Max.Y-10) > 1e-9 || math.Abs(r.Max.X-20) > 1e-9 || math.Abs(r.Min.X) > 1e-9 {
		t.Errorf("bounds %v to %v", r.Min, r.Max)
	}
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	if g := back.Group(ex.Name()); len(g.AsExtrude().DraftFaces()) != 1 || math.Abs(g.Analyze().Check().Volume-want) > 1e-6*want {
		t.Errorf("after a round trip: %d sides, volume %g", len(g.AsExtrude().DraftFaces()), g.Analyze().Check().Volume)
	}
	// By the side's face, and two sides.
	right := ex.Solid().FaceAt(api.V(20, 5, 5)).Entity()
	ex.AsExtrude().SetDraftFaces([]api.Entity{front, right})
	want2 := 0.0
	for i := range 1001 {
		z := 10 * float64(i) / 1000
		w := 1.0
		if i > 0 && i < 1000 {
			w = float64(2 + 2*(i%2))
		}
		want2 += w * (20 - z*tan) * (10 - z*tan)
	}
	want2 *= 10.0 / 1000 / 3
	if v := ex.Analyze().Check().Volume; math.Abs(v-want2) > 1e-6*want2 {
		t.Errorf("two sides: volume %g, want %g", v, want2)
	}
	ex.AsExtrude().SetDraftFaces(nil)
	if len(ex.AsExtrude().DraftFaces()) != 0 {
		t.Error("all sides again")
	}
}

// An extrusion of a D, its diameter meeting its arc at sharp corners:
// the arc's side a cone, the diameter's a plane, meeting in conics.
func TestDraftSharpArc(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.002)
	d.Sketch.Arc(api.V(0, 0, 0), api.V(10, 0, 0), api.V(-10, 0, 0))
	d.Sketch.Line(api.V(-10, 0, 0), api.V(10, 0, 0))
	ex := d.Features.Extrude(8, "")
	ex.AsExtrude().SetDraft(6)
	tan := math.Tan(6 * math.Pi / 180)
	seg := func(r, c float64) float64 { return r*r*math.Acos(c/r) - c*math.Sqrt(r*r-c*c) }
	want := 0.0
	const n = 400
	for i := 0; i <= n; i++ {
		z := 8 * float64(i) / n
		w := 1.0
		if i > 0 && i < n {
			w = float64(2 + 2*(i%2))
		}
		x := z * tan
		want += w * seg(10-x, x)
	}
	want *= 8.0 / n / 3
	r := ex.Analyze().Check()
	if ex.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 0.003*want {
		t.Fatalf("a drafted D: %q, %s, want %g", ex.Problem(), r.String(), want)
	}
}

// A lens, two arcs meeting at sharp corners, drafted (roadmap 28.2):
// the cones meet in curves of degree four; the lens at height z is
// where two discs of radius 5 - z tan 4° overlap. Undo and a round trip
// keep it.
func TestDraftSharpArcs(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.002)
	d.Sketch.Arc(api.V(0, 0, 0), api.V(5, 0, 0), api.V(0, 5, 0))
	d.Sketch.Arc(api.V(5, 5, 0), api.V(0, 5, 0), api.V(5, 0, 0))
	ex := d.Features.Extrude(6, "")
	step(d)
	before := saved(t, d)
	ex.AsExtrude().SetDraft(4)
	overlap := func(r, dist float64) float64 {
		x := dist / 2
		return 2 * (r*r*math.Acos(x/r) - x*math.Sqrt(r*r-x*x))
	}
	tan := math.Tan(4 * math.Pi / 180)
	want := 0.0
	const n = 400
	for i := 0; i <= n; i++ {
		z := 6 * float64(i) / n
		w := 1.0
		if i > 0 && i < n {
			w = float64(2 + 2*(i%2))
		}
		want += w * overlap(5-z*tan, 5*math.Sqrt2)
	}
	want *= 6.0 / n / 3
	r := ex.Analyze().Check()
	if ex.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 0.003*want {
		t.Fatalf("a drafted lens: %q, %s, want %g", ex.Problem(), r.String(), want)
	}
	undoRedo(t, d, before)
	// The file keeps no chord tolerance.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	d.Edit.Regen()
	roundTrip(t, d)
}

// The neutral plane elsewhere: at the far end the block narrows towards
// its foot; in the middle it narrows both ways; off the extrusion it is
// a problem.
func TestDraftNeutral(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	ex.AsExtrude().SetDraft(5)
	x := 10 * math.Tan(5*math.Pi/180)
	area := func(s float64) float64 { return (20 - 2*s) * (10 - 2*s) }
	whole := 10.0 / 6 * (area(0) + 4*area(x/2) + area(x))
	step(d)
	before := saved(t, d)
	ex.AsExtrude().SetDraftNeutral(10)
	r := ex.Analyze().Check()
	if ex.Problem() != "" || !r.Sound() || math.Abs(r.Volume-whole) > 1e-6*whole {
		t.Fatalf("from the far end: %q, %s, want %g", ex.Problem(), r.String(), whole)
	}
	// The foot is the smaller end now.
	for _, v := range ex.Solid().Vertices() {
		if p := v.Pos(); math.Abs(p.Z) < 1e-9 && (p.X < x-1e-9 || p.X > 20-x+1e-9) {
			t.Errorf("from the far end: a corner of the foot at %v", p)
		}
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	ex.AsExtrude().SetDraftNeutral(5)
	half := 5.0 / 6 * (area(0) + 4*area(x/4) + area(x/2))
	if r := ex.Analyze().Check(); ex.Problem() != "" || !r.Sound() || math.Abs(r.Volume-2*half) > 1e-6*half {
		t.Errorf("from the middle: %q, %s, want %g", ex.Problem(), r.String(), 2*half)
	}
	if err := ex.SetExpr(sketch.GroupPitch, "2*2"); err != nil || ex.AsExtrude().DraftNeutral() != 4 {
		t.Errorf("by an expression: %g, %v", ex.AsExtrude().DraftNeutral(), err)
	}
	ex.AsExtrude().SetDraftNeutral(20)
	if p := ex.Problem(); !strings.Contains(p, "neutral plane") {
		t.Errorf("off the extrusion: %q", p)
	}
}

func TestDraftSidesMisuse(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	ex.AsExtrude().SetDraft(5)
	d.Features.SketchIn(d.Workplane("XY"), "other")
	other := d.Sketch.Line(api.V(0, 30, 0), api.V(5, 30, 0))
	for name, c := range map[string]struct {
		fn   func()
		want string
	}{
		"another sketch's line": {func() { ex.AsExtrude().SetDraftFaces([]api.Entity{other}) }, "not a line, arc or circle of the extruded sketch"},
		"not an extrusion":      {func() { d.Group("other").AsExtrude().SetDraftFaces(nil) }, "extrude"},
		"neutral of a sketch":   {func() { d.Group("other").AsExtrude().SetDraftNeutral(1) }, "extrude"},
		"not a number":          {func() { ex.AsExtrude().SetDraftNeutral(math.NaN()) }, "distance"},
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

// TestDraftSidesSweep drafts one, two or all sides of a block and a D
// from planes along it, and requires sound solids of the right volume.
// The rate allowed was fixed at none before the sides were written.
func TestDraftSidesSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("sides", "front", "front and right", "all"),
			sweep.Floats("degrees", -8, 3, 12),
			sweep.Floats("neutral", 0, 4, 10),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		deg, s0 := p.Float("degrees"), p.Float("neutral")
		tan := math.Tan(deg * math.Pi / 180)
		d := api.New()
		ex := block(d, 0, 0, 20, 10, 10)
		ex.AsExtrude().SetDraft(deg)
		ex.AsExtrude().SetDraftNeutral(s0)
		front := lineAlong(t, ex, api.V(0, 0, 0), api.V(20, 0, 0))
		right := lineAlong(t, ex, api.V(20, 0, 0), api.V(20, 10, 0))
		var area func(x float64) float64
		switch p.String("sides") {
		case "front":
			ex.AsExtrude().SetDraftFaces([]api.Entity{front})
			area = func(x float64) float64 { return 20 * (10 - x) }
		case "front and right":
			ex.AsExtrude().SetDraftFaces([]api.Entity{front, right})
			area = func(x float64) float64 { return (20 - x) * (10 - x) }
		default:
			area = func(x float64) float64 { return (20 - 2*x) * (10 - 2*x) }
		}
		want := 0.0
		const n = 200
		for i := 0; i <= n; i++ {
			z := 10 * float64(i) / n
			w := 1.0
			if i > 0 && i < n {
				w = float64(2 + 2*(i%2))
			}
			want += w * area(math.Abs(z-s0)*tan)
		}
		want *= 10.0 / n / 3
		r := ex.Analyze().Check()
		switch {
		case ex.Problem() != "":
			return fmt.Errorf("problem: %s", ex.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case math.Abs(r.Volume-want) > 1e-6*want:
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}

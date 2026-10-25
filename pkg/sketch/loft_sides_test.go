package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// draftedPost is a circle of radius 5 extruded 10 up with a draft of
// draft degrees, its top circle drawn as the loft's first profile, and a
// circle of radius 2 at height 20 the last. It returns the two profiles
// and the top's radius.
func draftedPost(d *api.Doc, draft float64) (api.Group, api.Group, float64) {
	return draftedPostOf(d, draft, false)
}

// draftedPostOf is draftedPost, of a square 10 across, and its top and
// tip squares, with square.
func draftedPostOf(d *api.Doc, draft float64, square bool) (api.Group, api.Group, float64) {
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	shape := func(z, r float64) {
		if square {
			d.Sketch.Rect(api.V(-r, -r, z), api.V(r, r, z))
		} else {
			d.Sketch.Circle(api.V(0, 0, z), r)
		}
	}
	d.Features.SketchIn(d.Workplane("XY"), "base")
	shape(0, 5)
	ex := d.Features.Extrude(10, "post")
	if draft != 0 {
		ex.AsExtrude().SetDraft(draft)
	}
	rTop := 5 - 10*math.Tan(draft*math.Pi/180)
	bottom := d.Features.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "top")
	shape(10, rTop)
	top := d.Features.SketchAt(above(d, 20), api.V(1, 0, 0), api.V(0, 1, 0), "tip")
	shape(20, 2)
	return bottom, top, rTop
}

// loftRadius is how far out along x the loft's own surfaces are at
// height z.
func loftRadius(t *testing.T, d *api.Doc, g api.Group, z float64) float64 {
	t.Helper()
	r := math.NaN()
	d.With(func(sk *sketch.Sketch) {
		var inters []srf.SInter
		sh := &sk.Group(g.H).ThisShell
		sh.AllPointsIntersecting(geom.Vector{Z: z}, geom.Vector{X: 20, Z: z}, &inters, true, true, true)
		for _, in := range inters {
			if math.IsNaN(r) || in.P.X > r {
				r = in.P.X
			}
		}
	})
	if math.IsNaN(r) {
		t.Fatalf("nothing of the loft at height %g", z)
	}
	return r
}

// A loft drawn from the top of a drafted post leaves along its sides:
// its radius just above the top shrinks at the draft's slope, where
// square to the top it doesn't change at first. On a post of straight
// sides the two are the same loft.
func TestLoftAlongSides(t *testing.T) {
	t.Parallel()
	slope := func(along bool, draft float64) (float64, api.Group, *api.Doc) {
		d := api.New()
		bottom, top, _ := draftedPost(d, draft)
		step(d)
		before := saved(t, d)
		l := d.Features.Loft(bottom, top, "")
		lg := l.AsLoft()
		lg.SetTangency(0.5, 0)
		if along {
			lg.SetAlongSides(true, false)
		}
		if p := l.Problem(); p != "" {
			t.Fatal(p)
		}
		if r := l.Analyze().Check(); !r.Sound() {
			t.Fatalf("along %v, draft %g: %s", along, draft, r.String())
		}
		if along {
			_ = before
		}
		const h = 0.005
		return (loftRadius(t, d, l, 10+2*h) - loftRadius(t, d, l, 10+h)) / h, l, d
	}
	const draft = 10
	want := -math.Tan(draft * math.Pi / 180)
	if s, _, _ := slope(true, draft); math.Abs(s-want) > 0.02 {
		t.Errorf("along the drafted sides the loft leaves at slope %.4f, want %.4f", s, want)
	}
	if s, _, _ := slope(false, draft); math.Abs(s) > 0.02 {
		t.Errorf("square to the top the loft leaves at slope %.4f, want 0", s)
	}
	a, la, da := slope(true, 0)
	b, _, _ := slope(false, 0)
	if math.Abs(a-b) > 1e-6 {
		t.Errorf("on straight sides the loft along them leaves at %g, square to the top at %g", a, b)
	}
	if s, e := la.AsLoft().AlongSides(); !s || e {
		t.Errorf("AlongSides %v %v", s, e)
	}
	da.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	roundTrip(t, da)
}

// Setting a loft along the sides is one undo step, and comes back
// identical on redo.
func TestLoftAlongSidesUndo(t *testing.T) {
	t.Parallel()
	d := api.New()
	bottom, top, _ := draftedPost(d, 10)
	l := d.Features.Loft(bottom, top, "")
	step(d)
	before := saved(t, d)
	l.AsLoft().SetAlongSides(true, false)
	undoRedo(t, d, before)
}

func TestLoftAlongSidesProblems(t *testing.T) {
	t.Parallel()
	// No solid under the first profile's edge.
	d := api.New()
	bottom, top, _ := draftedPost(d, 10)
	d.Features.SketchAt(above(d, 30), api.V(1, 0, 0), api.V(0, 1, 0), "far")
	d.Sketch.Circle(api.V(0, 0, 30), 1)
	l := d.Features.Loft(top, d.Active(), "")
	l.AsLoft().SetAlongSides(true, false)
	if p := l.Problem(); !strings.Contains(p, "no edge of the solid") {
		t.Errorf("%q, want a problem saying the profile is on no edge", p)
	}
	_ = bottom
}

// TestLoftAlongSidesSweep lofts from the top of posts round and square,
// drafted in and out, along their sides, by a tangency of 1 (left at 0)
// and of 0.5, and requires a sound solid joined to the post, leaving the
// top at the draft's slope. The rate allowed was fixed at none before
// the feature was written.
func TestLoftAlongSidesSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("a sweep of lofts along drafted sides")
	}
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("profile", "circle", "square"),
			sweep.Floats("draft", -8, 0, 5, 15),
			sweep.Floats("tangency", 0, 0.5),
		},
		MaxFailRate: 0,
		Sample:      8,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		bottom, top, _ := draftedPostOf(d, p.Float("draft"), p.String("profile") == "square")
		l := d.Features.Loft(bottom, top, "")
		l.AsLoft().SetTangency(p.Float("tangency"), 0)
		l.AsLoft().SetAlongSides(true, false)
		if msg := l.Problem(); msg != "" {
			return fmt.Errorf("problem: %s", msg)
		}
		if r := l.Analyze().Check(); !r.Sound() {
			return fmt.Errorf("unsound: %s", r.String())
		}
		const h = 0.005
		slope := (loftRadius(t, d, l, 10+2*h) - loftRadius(t, d, l, 10+h)) / h
		if want := -math.Tan(p.Float("draft") * math.Pi / 180); math.Abs(slope-want) > 0.02 {
			return fmt.Errorf("leaves at slope %.4f, want %.4f", slope, want)
		}
		return nil
	}).Require(t)
}

// BenchmarkLoftAlongSides regenerates a loft along a drafted post's
// sides.
func BenchmarkLoftAlongSides(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		bottom, top, _ := draftedPost(d, 10)
		d.Features.Loft(bottom, top, "").AsLoft().SetAlongSides(true, false)
	})
}

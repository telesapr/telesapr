package check_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// rect draws a w by h rectangle with its corner at the origin and
// extrudes it by depth.
func rect(d *api.Doc, w, h, depth float64) api.Group {
	d.Sketch.Rect(api.V(0, 0, 0), api.V(w, h, 0))
	return d.Features.Extrude(depth, "")
}

// A drafted block is a frustum: its volume the closed form, a probe
// across its top inset by the draft, and the kernel's mesh agreeing ray
// by ray; leaning out, it grows; a draft at which the sides meet leaves
// the block upright, as the kernel makes it.
func TestOracleDraftedBlock(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := rect(d, 20, 10, 10)
	ex.AsExtrude().SetDraft(5)
	x := 10 * math.Tan(5*math.Pi/180)
	area := func(z float64) float64 { s := z / 10 * x; return (20 - 2*s) * (10 - 2*s) }
	want := 10.0 / 6 * (area(0) + 4*area(5) + area(10))
	o := oracleOf(t, d)
	if m := o.Measure(1e-4, 256); !near(m.Volume, want, 2e-3) {
		t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, want, m.Spread(), m.Rays)
	}
	if iv := o.Probe(api.V(-5, 5, 9.99), api.V(1, 0, 0)); len(iv) != 1 || math.Abs(iv[0].From-5-x) > 1e-3 || math.Abs(iv[0].To-25+x) > 1e-3 {
		t.Errorf("probe near the top %v, want from %g", iv, 5+x)
	}
	if o.Inside(api.V(0.01, 5, 9.9)) || !o.Inside(api.V(0.01, 5, 0.01)) {
		t.Error("the side doesn't lean")
	}
	if diffs := o.Diff(meshOf(d), 24, 0.01); len(diffs) != 0 {
		t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
	}
	// The draft lost from the mesh: its rays through the sides disagree.
	ex.AsExtrude().SetDraft(0)
	oracleOf(t, d) // regenerates the block upright
	if diffs := o.Diff(meshOf(d), 24, 0.01); len(diffs) == 0 {
		t.Error("an upright block agrees with the drafted one")
	}

	ex.AsExtrude().SetDraft(-5)
	o = oracleOf(t, d)
	if !o.Inside(api.V(-x/2, 5, 9.9)) || o.Max.X < 20+x-1e-9 {
		t.Errorf("leaning out: box to %v", o.Max)
	}
	if diffs := o.Diff(meshOf(d), 24, 0.01); len(diffs) != 0 {
		t.Errorf("leaning out: %d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
	}

	ex.AsExtrude().SetDraft(40)
	if m := oracleOf(t, d).Measure(1e-9, 64); !near(m.Volume, 2000, 1e-9) {
		t.Errorf("a refused draft: volume %g", m.Volume)
	}
}

// A round boss with a hole: the outside narrows and the hole widens,
// two cones whose frustums give the volume.
func TestOracleDraftedRound(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Circle(api.V(0, 0, 0), 10)
	d.Sketch.Circle(api.V(0, 0, 0), 4)
	d.Features.Extrude(10, "").AsExtrude().SetDraft(3)
	x := 10 * math.Tan(3*math.Pi/180)
	frustum := func(r0, r1 float64) float64 { return math.Pi * 10 / 3 * (r0*r0 + r0*r1 + r1*r1) }
	want := frustum(10, 10-x) - frustum(4, 4+x)
	o := oracleOf(t, d)
	if m := o.Measure(1e-4, 256); !near(m.Volume, want, 2e-3) {
		t.Errorf("volume %g, want %g (spread %g at %d rays)", m.Volume, want, m.Spread(), m.Rays)
	}
	if o.Inside(api.V(4.1, 0, 9.9)) || !o.Inside(api.V(4.1, 0, 0.1)) || o.Inside(api.V(9.9, 0, 9.9)) {
		t.Error("the cones don't lean")
	}
	if diffs := o.Diff(meshOf(d), 24, 0.1); len(diffs) != 0 {
		t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
	}
}

// Drafts the kernel builds in other ways: one side leaning from a
// neutral plane at the middle of a two-sided extrusion, an arc meeting a
// line at sharp corners, and two arcs meeting at a lens's points. The
// oracle's volume is the kernel's, and their rays agree.
func TestOracleDraftVariants(t *testing.T) {
	t.Parallel()
	for name, build := range map[string]func(t *testing.T, d *api.Doc){
		"one side, neutral": func(t *testing.T, d *api.Doc) {
			d.Sketch.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
			ex := d.Features.Extrude(10, "")
			ex.SetTwoSided(true)
			ex.AsExtrude().SetDraft(8)
			ex.AsExtrude().SetDraftNeutral(3)
			for _, e := range d.Group(d.Groups()[len(d.Groups())-2].Name()).Entities() {
				if e.Kind() == "line" && e.Start().Pos().Y == 0 && e.End().Pos().Y == 0 {
					ex.AsExtrude().SetDraftFaces([]api.Entity{e})
				}
			}
			if len(ex.AsExtrude().DraftFaces()) != 1 {
				t.Fatal("no side to lean")
			}
		},
		"sharp arc": func(_ *testing.T, d *api.Doc) {
			d.Sketch.Arc(api.V(0, 0, 0), api.V(10, 0, 0), api.V(-10, 0, 0))
			d.Sketch.Line(api.V(-10, 0, 0), api.V(10, 0, 0))
			d.Features.Extrude(8, "").AsExtrude().SetDraft(6)
		},
		"lens": func(_ *testing.T, d *api.Doc) {
			d.Sketch.Arc(api.V(0, 0, 0), api.V(5, 0, 0), api.V(0, 5, 0))
			d.Sketch.Arc(api.V(5, 5, 0), api.V(0, 5, 0), api.V(5, 0, 0))
			d.Features.Extrude(6, "").AsExtrude().SetDraft(4)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			build(t, d)
			g := d.Group(d.Groups()[len(d.Groups())-1].Name())
			if p := g.Problem(); p != "" {
				t.Fatalf("the kernel refused it: %s", p)
			}
			kernel := g.Analyze().Check()
			o := oracleOf(t, d)
			if m := o.Measure(1e-4, 256); !near(m.Volume, kernel.Volume, 3e-3) {
				t.Errorf("volume %g, the kernel's %g (spread %g at %d rays)", m.Volume, kernel.Volume, m.Spread(), m.Rays)
			}
			if diffs := o.Diff(meshOf(d), 24, 0.05); len(diffs) != 0 {
				t.Errorf("%d rays disagree with the mesh, as %+v", len(diffs), diffs[0])
			}
		})
	}
}

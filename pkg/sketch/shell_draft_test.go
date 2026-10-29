package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// frustum is the volume of a frustum of height h between radii r1 and
// r2.
func frustum(h, r1, r2 float64) float64 { return math.Pi * h / 3 * (r1*r1 + r1*r2 + r2*r2) }

// A drafted cylinder, its side a cone, hollowed open at its top: the
// inside is the cone moved in square to itself, 1/cos α across, from
// the bottom moved up by 1. The shell is one undo step and comes back
// from the file; a thickness the top can't take is a problem.
func TestShellDraftedCylinder(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	d.Sketch.Circle(api.V(0, 0, 0), 10)
	ex := d.Features.Extrude(10, "")
	ex.AsExtrude().SetDraft(5)
	if p := ex.Problem(); p != "" {
		t.Fatal(p)
	}
	step(d)
	before := saved(t, d)
	tan, sec := math.Tan(5*math.Pi/180), 1/math.Cos(5*math.Pi/180)
	top := ex.Solid().FaceAt(api.V(1, 1, 10))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	want := frustum(10, 10, 10-10*tan) - frustum(9, 10-sec-tan, 10-sec-10*tan)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled drafted cylinder: %s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, before)
	// The file holds the solid's curves at the sketch's tolerance, which
	// a reopened file has at its default.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0; sk.ScheduleGenerateAll() })
	back := roundTrip(t, d)
	if br := back.Group(sh.Name()).Analyze().Check(); !br.Sound() || math.Abs(br.Volume-want) > 0.01*want {
		t.Fatalf("reopened: %s", br.String())
	}
	sh.AsShell().SetThickness(10)
	if p := sh.Problem(); !strings.Contains(p, "thickness") {
		t.Errorf("walls that meet: problem %q", p)
	}
}

// A drafted ring, a cylinder with a hole, both cones leaning into the
// material, open at its foot: the outer cone narrows going up, the
// hole widens, and the inside between them keeps 1 from each and from
// the top.
func TestShellDraftedRing(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	d.Sketch.Circle(api.V(0, 0, 0), 10)
	d.Sketch.Circle(api.V(0, 0, 0), 4)
	ex := d.Features.Extrude(8, "")
	ex.AsExtrude().SetDraft(3)
	if p := ex.Problem(); p != "" {
		t.Fatal(p)
	}
	foot := ex.Solid().FaceAt(api.V(7, 0, 0))
	sh := d.Features.ShellSolid([]api.SolidFace{foot}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	tan, sec := math.Tan(3*math.Pi/180), 1/math.Cos(3*math.Pi/180)
	solid := frustum(8, 10, 10-8*tan) - frustum(8, 4, 4+8*tan)
	inside := frustum(7, 10-sec, 10-sec-7*tan) - frustum(7, 4+sec, 4+sec+7*tan)
	want := solid - inside
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled drafted ring: %s, want volume %g", r.String(), want)
	}
}

// A slot, two lines and two half circles, extruded both ways from its
// sketch with a draft, hollowed open at its top: the inside is two
// drafted prisms from the neutral plane, joined, as the solid is. Its
// side is refused as a face to take away.
func TestShellDraftedSlotTwoSided(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	a := d.Sketch.Line(api.V(-5, -4, 0), api.V(5, -4, 0))
	b := d.Sketch.Line(api.V(5, 4, 0), api.V(-5, 4, 0))
	d.Sketch.Arc(api.V(5, 0, 0), a.End().Pos(), b.Start().Pos())
	d.Sketch.Arc(api.V(-5, 0, 0), b.End().Pos(), a.Start().Pos())
	ex := d.Features.Extrude(12, "")
	ex.SetTwoSided(true)
	ex.AsExtrude().SetDraft(4)
	if p := ex.Problem(); p != "" {
		t.Fatal(p)
	}
	r0 := ex.Analyze().Check()
	if !r0.Sound() {
		t.Fatalf("drafted slot: %s", r0.String())
	}
	side := ex.Solid().FaceAt(api.V(0, -4, 0))
	refusedWith(t, "only at its ends", func() { d.Features.ShellSolid([]api.SolidFace{side}, 1, "") })
	top := ex.Solid().FaceAt(api.V(0, 0, 6))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	// Each half of the solid, z from 0 to 6, is a slot of half-width w
	// and straight length 10 narrowing by z tan α: its area
	// 20w + πw². The inside's is w - sec α less, from z = 0 to 5 above
	// and to 5 below (the bottom kept, moved in too).
	tan, sec := math.Tan(4*math.Pi/180), 1/math.Cos(4*math.Pi/180)
	half := func(w0, h float64) float64 {
		// ∫0^h 20(w0 - z tan) + π(w0 - z tan)² dz
		w1 := w0 - h*tan
		return 20*h*(w0+w1)/2 + math.Pi*h/3*(w0*w0+w0*w1+w1*w1)
	}
	want := 2*half(4, 6) - half(4-sec, 6) - half(4-sec, 5)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled drafted slot: %s, want volume %g", r.String(), want)
	}
}

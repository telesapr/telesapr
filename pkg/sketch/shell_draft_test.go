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
// flat side is refused as a face to take away: it runs on smoothly into
// the round ends, and the wall would part there.
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
	refusedWith(t, "runs on smoothly", func() { d.Features.ShellSolid([]api.SolidFace{side}, 1, "") })
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

// A drafted half disc, open at its round side: the inside's cone runs
// out past the solid's, and its flat side is moved in square to itself,
// so what is left is the top, the bottom and a wall along the flat
// side, leaning as it does.
func TestShellDraftedOpenAtCone(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	l := d.Sketch.Line(api.V(0, 10, 0), api.V(0, -10, 0))
	d.Sketch.Arc(api.V(0, 0, 0), l.End().Pos(), l.Start().Pos())
	ex := d.Features.Extrude(10, "")
	ex.AsExtrude().SetDraft(5)
	if p := ex.Problem(); p != "" {
		t.Fatal(p)
	}
	tan, sec := math.Tan(5*math.Pi/180), 1/math.Cos(5*math.Pi/180)
	r5 := 10 - 5*tan
	// The cone, in halves either side of the x axis.
	var cone []api.SolidFace
	for _, a := range []float64{0.6, -0.6} {
		cone = append(cone, ex.Solid().FaceAt(api.V(r5*math.Cos(a), r5*math.Sin(a), 5)))
	}
	sh := d.Features.ShellSolid(cone, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	// Of the circle of radius r, what lies at x ≥ c.
	seg := func(r, c float64) float64 { return r*r*math.Acos(c/r) - c*math.Sqrt(r*r-c*c) }
	const n = 20000
	want := 0.0
	for i := range n {
		z := 10 * (float64(i) + 0.5) / n
		r, c := 10-z*tan, z*tan
		a := seg(r, c)
		if z > 1 && z < 9 {
			a -= seg(r, c+sec)
		}
		want += a * 10 / n
	}
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("drafted half disc open at its cone: %s, want volume %g", r.String(), want)
	}
}

// A block with a drafted round boss standing up through its top from
// its bottom, open at the boss's top: the boss's cone is a wall above
// the block, moved in square to itself, and its inside reaches down into
// the block's.
func TestShellBlockAndDraftedBoss(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	block(d, -10, -10, 10, 10, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 4)
	boss := d.Features.Extrude(16, "")
	boss.AsExtrude().SetDraft(5)
	if p := boss.Problem(); p != "" {
		t.Fatal(p)
	}
	step(d)
	before := saved(t, d)
	top := boss.Solid().FaceAt(api.V(1, 1, 16))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	tan, sec := math.Tan(5*math.Pi/180), 1/math.Cos(5*math.Pi/180)
	solid := 4000 + frustum(6, 4-10*tan, 4-16*tan)
	want := solid - (18*18*8 + frustum(7, 4-sec-9*tan, 4-sec-16*tan))
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled block and drafted boss: %s, want volume %g", r.String(), want)
	}
	undoRedo(t, d, before)
}

// A drafted cylinder with a square post standing up through its top
// from its bottom, open at the post's top: the cylinder's cone and top
// are walls, moved in, and the post's inside meets the cylinder's.
func TestShellDraftedCylinderAndBlock(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	d.Sketch.Circle(api.V(0, 0, 0), 10)
	cyl := d.Features.Extrude(10, "")
	cyl.AsExtrude().SetDraft(5)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(-3, -3, 0), api.V(3, 3, 0))
	post := d.Features.Extrude(16, "")
	if p := post.Problem(); p != "" {
		t.Fatal(p)
	}
	top := post.Solid().FaceAt(api.V(1, 1, 16))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	tan, sec := math.Tan(5*math.Pi/180), 1/math.Cos(5*math.Pi/180)
	solid := frustum(10, 10, 10-10*tan) + 36*6
	want := solid - (frustum(8, 10-sec-tan, 10-sec-9*tan) + 16*7)
	if r := sh.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 0.002*want {
		t.Fatalf("shelled drafted cylinder and post: %s, want volume %g", r.String(), want)
	}
}

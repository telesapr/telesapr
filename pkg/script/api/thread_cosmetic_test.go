package api

import (
	"math"
	"strings"
	"testing"
)

// A cosmetic thread is one group on a rod of its size, reads back as
// cosmetic, edits to a modelled thread and back in one step of undo
// each, comes back the same on redo, and is written to a script as one
// call.
func TestThreadCosmetic(t *testing.T) {
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 5)
	d.Features.Extrude(30, "rod")
	g := d.Features.Thread(rod, ThreadOptions{Designation: "M10", Length: 20, Cosmetic: true})
	if !g.IsThread() || g.Kind() != "sketch-in-plane" {
		t.Fatalf("the thread is kept by a %s, IsThread %v", g.Kind(), g.IsThread())
	}
	th := g.AsThread()
	if o := th.Options(); !o.Cosmetic || o.Designation != "M10" || o.Length != 20 {
		t.Errorf("options %+v", o)
	}
	if gs := th.Groups(); len(gs) != 1 || th.Finished().H != g.H {
		t.Errorf("groups %v, finished %v", gs, th.Finished())
	}
	cylinder := math.Pi * 25 * 30
	volume := func() float64 { return d.Active().Analyze().Check().Volume }
	if v := volume(); math.Abs(v-cylinder) > 0.01*cylinder {
		t.Errorf("volume %.2f, want the rod's %.2f", v, cylinder)
	}
	before := len(d.Groups())

	d.Host().(*SketchHost).BeginEval()
	o := th.Options()
	o.Cosmetic = false
	modelled := th.Edit(o)
	if modelled.g.H == g.H || len(modelled.Groups()) != 4 || modelled.Options().Cosmetic {
		t.Fatalf("made modelled: kept by %v, groups %v", modelled.g, modelled.Groups())
	}
	if modelled.Groups()[0].H != g.H {
		t.Error("the thread's sketch didn't keep its handle")
	}
	turned := volume()
	if turned > cylinder-200 {
		t.Errorf("modelled volume %.2f, not less than the rod's %.2f", turned, cylinder)
	}
	if r := modelled.Finished().Analyze().Check(); !r.Sound() {
		t.Errorf("modelled: %s", r.String())
	}

	d.Host().(*SketchHost).BeginEval()
	back := modelled.Edit(ThreadOptions{Cosmetic: true})
	if back.g.H != g.H || len(d.Groups()) != before {
		t.Errorf("cosmetic again: kept by %v, %d groups, want %v and %d", back.g, len(d.Groups()), g, before)
	}

	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	if v := volume(); math.Abs(v-turned) > 1e-6*turned || len(d.Groups()) != before+3 {
		t.Errorf("undo: volume %.3f, %d groups; want the modelled %.3f and %d", v, len(d.Groups()), turned, before+3)
	}
	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	if !g.AsThread().Options().Cosmetic || len(d.Groups()) != before {
		t.Error("undo didn't bring the cosmetic thread back")
	}
	if !d.Edit.Redo() {
		t.Fatal("nothing to redo")
	}
	if v := volume(); math.Abs(v-turned) > 1e-6*turned {
		t.Errorf("redo: volume %.3f, want %.3f", v, turned)
	}
	if !d.Edit.Redo() {
		t.Fatal("nothing to redo")
	}

	src, notes := d.Script()
	if !strings.Contains(src, `doc.Features.Thread(`) || !strings.Contains(src, "Cosmetic: true") || len(notes) > 0 {
		t.Errorf("script %s\nnotes %v", src, notes)
	}
}

// A nut's cosmetic thread bores the part to the minor diameter, and a
// hole already that wide is left as it is.
func TestThreadCosmeticNut(t *testing.T) {
	d := New()
	c := d.Sketch.Circle(V(0, 0, 0), 8)
	d.Features.Extrude(10, "block")
	block := d.Active().Analyze().Check().Volume
	o := ThreadOptions{Designation: "M8", Length: 10, Through: true, Internal: true, Cosmetic: true}
	g := d.Features.Thread(c, o)
	if gs := g.AsThread().Groups(); len(gs) != 2 || gs[1].Kind() != "lathe" {
		t.Fatalf("groups %v", gs)
	}
	bored := d.Active().Analyze().Check().Volume
	// M8's minor diameter is about 6.65.
	if hole := block - bored; math.Abs(hole-math.Pi*3.32*3.32*10) > 10 {
		t.Errorf("bored %.2f mm³ out", hole)
	}
	again := d.Features.Thread(c, o)
	if gs := again.AsThread().Groups(); len(gs) != 1 {
		t.Errorf("on the hole: groups %v", gs)
	}
	if v := d.Active().Analyze().Check().Volume; math.Abs(v-bored) > 1e-6*bored {
		t.Errorf("on the hole: volume %.3f, was %.3f", v, bored)
	}
}

// Cosmetic threads picked on the model: a shaft of the thread's size
// and a tapped hole at its tap drill are left as they are, a pilot hole
// narrower than the minor diameter is bored to it, and a cosmetic
// thread on a face edits to a modelled one and back.
func TestThreadCosmeticOnFace(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Circle(V(0, 0, 0), 5)
	rod := d.Features.Extrude(30, "shaft")
	shaft := d.Active().Analyze().Check().Volume
	g := d.Features.ThreadOnFace(rod.Solid().FaceAt(V(5, 0, 5)), ThreadOptions{Length: 12, Cosmetic: true})
	if o := g.AsThread().Options(); o.Designation != "M10" || !o.Cosmetic || o.Internal {
		t.Errorf("shaft: options %+v", o)
	}
	if gs := g.AsThread().Groups(); len(gs) != 1 {
		t.Errorf("shaft: groups %v", gs)
	}
	if v := d.Active().Analyze().Check().Volume; math.Abs(v-shaft) > 1e-6*shaft {
		t.Errorf("shaft: volume %.3f, was %.3f", v, shaft)
	}
	if p := threadPlace(t, d, g); p.At.Frame.Origin.Z < 29 {
		t.Errorf("shaft: thread from z = %g, want the free end", p.At.Frame.Origin.Z)
	}
	o := g.AsThread().Options()
	o.Cosmetic = false
	m := g.AsThread().Edit(o)
	if r := m.Finished().Analyze().Check(); !r.Sound() || len(m.Groups()) != 4 {
		t.Errorf("made modelled on the face: %d groups, %s", len(m.Groups()), r.String())
	}
	o.Cosmetic = true
	if c := m.Edit(o); c.g.H != g.H {
		t.Errorf("cosmetic again kept by %v, want %v", c.g, g)
	}

	for _, tc := range []struct {
		dia   float64
		bored bool
	}{{8.5, false}, {6, true}} {
		d, face := plateWithHole(t, tc.dia)
		plate := d.Active().Analyze().Check().Volume
		g := d.Features.ThreadOnFace(face, ThreadOptions{Designation: "M10", Cosmetic: true})
		o := g.AsThread().Options()
		if !o.Internal || !o.Through || !o.FullLength {
			t.Errorf("hole %g: options %+v", tc.dia, o)
		}
		v := d.Active().Analyze().Check().Volume
		if bored := v < plate-1; bored != tc.bored {
			t.Errorf("hole %g: volume %.2f from %.2f, bored %v", tc.dia, v, plate, bored)
		}
		if r := d.Active().Analyze().Check(); !r.Sound() {
			t.Errorf("hole %g: %s", tc.dia, r.String())
		}
	}
}

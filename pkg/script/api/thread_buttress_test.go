package api

import (
	"math"
	"path/filepath"
	"testing"
)

// buttressNut saves an S20x4 nut, a Ø40 disc 16 mm high with its hole
// threaded through from the bottom up, its load flank turned back or
// not, and returns its path.
func buttressNut(t *testing.T, back bool) string {
	t.Helper()
	nut := New()
	disc := nut.Sketch.Circle(V(0, 0, 0), 20)
	nut.Constrain.Coincident(disc.Center(), nut.Origin())
	nut.Features.Extrude(16, "disc")
	nut.Features.SketchIn(nut.Workplane("XY"), "hole")
	hole := nut.Sketch.Circle(V(0, 0, 0), 10)
	nut.Constrain.Coincident(hole.Center(), nut.Origin())
	nut.Features.Thread(hole, ThreadOptions{Designation: "S20x4", Internal: true, Through: true, Length: 16,
		Clearance: 0.1, LoadFlankBack: back})
	path := filepath.Join(t.TempDir(), "nut.tlspr")
	if err := nut.File.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// buttressRod is a Ø20 rod 48 mm long threaded S20x4 down from its
// top, and the circle there that the thread is on.
func buttressRod(d *Doc) Entity {
	shank := d.Sketch.Circle(V(0, 0, 0), 10)
	d.Constrain.Coincident(shank.Center(), d.Origin())
	rod := d.Features.Extrude(48, "rod")
	var end Entity
	for _, e := range rod.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 24 {
			end = e
		}
	}
	d.Features.Thread(end, ThreadOptions{Designation: "S20x4", Length: 48, Reverse: true, Through: true, Clearance: 0.1})
	return end
}

// buttressLowest is g's circle of the hole lowest down.
func buttressLowest(g Group) Entity {
	var best Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && math.Abs(e.Radius()-10) < 1e-6 &&
			(best.H.V == 0 || e.Center().Pos().Z < best.Center().Pos().Z) {
			best = e
		}
	}
	return best
}

// TestScrewFitsButtressThreads screws a buttress nut onto a buttress
// rod. The rod's thread runs down and the nut's up, so their load
// flanks face the same way and the screw mate refuses them; the nut
// made with its load flank back fits: the mate moves it to where its
// ridges lie in the rod's grooves, leaning the same way, and it fits at
// every turn after, where half a pitch off it collides.
func TestScrewFitsButtressThreads(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("interferences of threads by their meshes, a few seconds")
	}
	d := New()
	end := buttressRod(d)
	g, err := d.Assembly.Link(buttressNut(t, false), "ahead")
	if err != nil {
		t.Fatal(err)
	}
	d.Sketch.Anywhere3D()
	mustFail(t, "load flanks the same way", "load flanks face the same way", func() {
		d.Assembly.Screw(end, buttressLowest(g), 0)
	})

	d = New()
	end = buttressRod(d)
	if g, err = d.Assembly.Link(buttressNut(t, true), "back"); err != nil {
		t.Fatal(err)
	}
	link := g.AsLink()
	o := link.Offset()
	link.Move(V(-o.X, -o.Y, 14-o.Z))
	d.Sketch.Anywhere3D()
	mate := d.Assembly.Screw(end, buttressLowest(g), 0)
	if lead, _ := d.Params.Get(mate.Lead); math.Abs(lead.Value-4) > 1e-9 {
		t.Errorf("lead %v, want the S20x4's 4", lead.Value)
	}
	for _, turn := range []string{"0", "90", "-200"} {
		if err := d.Params.Set(mate.Turn, turn); err != nil {
			t.Fatal(err)
		}
		if found, _ := d.Assembly.Interferences(); len(found) != 0 {
			t.Errorf("turned %s: %+v", turn, found)
		}
	}
	// Half a turn on, a lead of 4 and half a pitch: the ridges collide.
	if err := d.Params.Set(mate.Lead, "0"); err != nil {
		t.Fatal(err)
	}
	if err := d.Params.Set(mate.Turn, "180"); err != nil {
		t.Fatal(err)
	}
	if found, _ := d.Assembly.Interferences(); len(found) != 1 {
		t.Errorf("half a pitch off: %+v, want the nut to collide", found)
	}
}

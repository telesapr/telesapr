package api

import (
	"path/filepath"
	"testing"
)

// An M10 rod threaded along its length, and a nut of a Ø20 disc 8 mm
// high threaded through, both of clearance 0, the nut linked onto the
// rod: the threads start in phase at whole pitches from the rod's end,
// where the flanks touch and nothing interferes, and half a pitch on
// from there the ridges collide, as Assembly.Interferences says by the
// parts' meshes, the Boolean of their shells failing on the flanks
// lying on each other.
func TestInterferencesOfANutOnARod(t *testing.T) {
	t.Parallel()
	nut := New()
	disc := nut.Sketch.Circle(V(0, 0, 0), 10)
	nut.Constrain.Coincident(disc.Center(), nut.Origin())
	nut.Features.Extrude(8, "disc")
	nut.Features.SketchIn(nut.Workplane("XY"), "hole")
	hole := nut.Sketch.Circle(V(0, 0, 0), 5)
	nut.Constrain.Coincident(hole.Center(), nut.Origin())
	nut.Features.Thread(hole, ThreadOptions{Designation: "M10", Internal: true, Through: true, Length: 8})
	path := filepath.Join(t.TempDir(), "nut.tlspr")
	if err := nut.File.Save(path); err != nil {
		t.Fatal(err)
	}

	d := New()
	shank := d.Sketch.Circle(V(0, 0, 0), 5)
	d.Constrain.Coincident(shank.Center(), d.Origin())
	rod := d.Features.Extrude(30, "rod")
	var end Entity
	for _, e := range rod.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 15 {
			end = e
		}
	}
	d.Features.Thread(end, ThreadOptions{Designation: "M10", Length: 30, Reverse: true, Through: true})
	g, err := d.Assembly.Link(path, "nut")
	if err != nil {
		t.Fatal(err)
	}
	link := g.AsLink()
	at := func(z float64) []Interference {
		o := link.Offset()
		link.Move(V(-o.X, -o.Y, z-o.Z))
		found, _ := d.Assembly.Interferences()
		return found
	}
	if found := at(6); len(found) != 0 {
		t.Errorf("four pitches up the rod: %+v", found)
	}
	// The nut's bore is taken a little past its hole (thread on a face),
	// so the Boolean may succeed; by it or by the meshes, about 39 mm³.
	found := at(6.75)
	if len(found) != 1 || found[0].B.H != g.H || found[0].Failed || found[0].Volume < 20 || found[0].Volume > 60 {
		t.Errorf("half a pitch on: %+v, want about 39 mm³", found)
	}
}

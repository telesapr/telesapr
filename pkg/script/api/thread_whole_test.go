package api

import (
	"math"
	"slices"
	"testing"
)

// groupNames are the names of the document's groups, in history order.
func groupNames(d *Doc) []string {
	var names []string
	for _, g := range d.Groups() {
		names = append(names, g.Name())
	}
	return names
}

// A thread's groups are found from any of them, and the thread is
// shown, moved, suppressed and deleted as one, as the window's group
// list does with its row.
func TestThreadWhole(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Circle(V(0, 0, 0), 4)
	ext := d.Features.Extrude(30, "rod")
	rodVolume := ext.Analyze().Volume()
	other := d.Features.Sketch3D("other")
	d.Activate(ext)
	// On the extrusion's top circle, down into it: the thread is drawn
	// on the extrusion.
	var top Entity
	for _, e := range ext.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 15 {
			top = e
		}
	}
	th := d.Features.Thread(top, ThreadOptions{Designation: "M8", Length: 12, Reverse: true}).AsThread()
	d.Activate(other)

	for _, g := range th.Groups() {
		got, ok := g.ThreadOf()
		if !ok || got.Group().H != th.Group().H {
			t.Errorf("%s: ThreadOf gave %v, %v", g.Name(), got.Group(), ok)
		}
	}
	for _, g := range []Group{ext, other} {
		if _, ok := g.ThreadOf(); ok {
			t.Errorf("%s is not one of the thread's groups", g.Name())
		}
	}

	th.SetVisible(false)
	if th.Visible() || slices.ContainsFunc(th.Groups(), Group.Visible) {
		t.Errorf("hidden: Visible %v", th.Visible())
	}
	th.SetVisible(true)
	if !th.Visible() {
		t.Error("shown again: not Visible")
	}

	// The thread was made after the extrusion, before the other sketch;
	// it can't go before the extrusion it is drawn on, and goes after
	// the other sketch as one.
	if err := th.MoveBefore(ext); err == nil {
		t.Error("the thread moved before the extrusion it is drawn on")
	}
	before := groupNames(d)
	if err := th.MoveBefore(other); err != nil {
		t.Fatal(err)
	}
	if got := groupNames(d); !slices.Equal(got, before) {
		t.Errorf("a move to where it is changed the history: %v", got)
	}
	last := d.Features.Sketch3D("last")
	if err := th.MoveBefore(last); err != nil {
		t.Fatal(err)
	}
	names := groupNames(d)
	if want := []string{"sketch-in-plane", "rod", "other", "M8-blank", "M8-core", "M8-profile", "M8-thread", "last"}; !slices.Equal(names, want) {
		t.Errorf("after the move: %v, want %v", names, want)
	}

	th.SetSuppressed(true)
	if !th.Suppressed() {
		t.Error("not Suppressed")
	}
	for _, g := range th.Groups() {
		if g.LeftOut() == "" {
			t.Errorf("%s is in the model, the thread suppressed", g.Name())
		}
	}
	th.SetSuppressed(false)
	if th.Suppressed() || th.Finished().LeftOut() != "" {
		t.Error("brought back: still suppressed")
	}
	if r := th.Finished().Analyze().Check(); !r.Sound() {
		t.Errorf("brought back: %s", r.String())
	}

	th.Delete()
	names = groupNames(d)
	if want := []string{"sketch-in-plane", "rod", "other", "last"}; !slices.Equal(names, want) {
		t.Errorf("after the delete: %v", names)
	}
	if v := ext.Analyze().Volume(); math.Abs(v-rodVolume) > 1e-9*rodVolume {
		t.Errorf("the rod's volume %g, want %g as before the thread", v, rodVolume)
	}
}

// Every group of a thread of level starts is one of its, as is the
// datum of one made on a face.
func TestThreadOfLevelStartsAndFace(t *testing.T) {
	t.Parallel()
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 8)
	d.Features.Extrude(40, "rod")
	th := d.Features.Thread(rod, ThreadOptions{Designation: "M16xPh3P1.5", Length: 12, LevelStarts: true}).AsThread()
	if n := len(th.Groups()); n != 6 {
		t.Fatalf("%d groups, want 6", n)
	}
	for _, g := range th.Groups() {
		if got, ok := g.ThreadOf(); !ok || got.Group().H != th.Group().H {
			t.Errorf("%s: not the thread's", g.Name())
		}
	}

	f := New()
	f.Sketch.Circle(V(0, 0, 0), 5)
	shaft := f.Features.Extrude(30, "shaft")
	face := shaft.Solid().FaceAt(V(5*math.Sqrt2/2, 5*math.Sqrt2/2, 15))
	ft := f.Features.ThreadOnFace(face, ThreadOptions{Length: 12}).AsThread()
	groups := f.Groups()
	datum := groups[slices.IndexFunc(groups, func(g Group) bool { return g.H == ft.Groups()[0].H })-1]
	if got, ok := datum.ThreadOf(); !ok || got.Group().H != ft.Group().H {
		t.Errorf("the datum %s is not the face thread's", datum.Name())
	}
	ft.Delete()
	if names := groupNames(f); !slices.Equal(names, []string{"sketch-in-plane", "shaft"}) {
		t.Errorf("deleting the face thread left %v", names)
	}
}

// A chamfered thread's sketch and lathe, and a cosmetic thread's
// sketch, are its groups too; a cosmetic thread is deleted whole.
func TestThreadOfChamferAndCosmetic(t *testing.T) {
	t.Parallel()
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 4)
	d.Features.Extrude(30, "rod")
	th := d.Features.Thread(rod, ThreadOptions{Designation: "M8", Length: 12, Chamfer: true}).AsThread()
	if n := len(th.Groups()); n != 6 {
		t.Fatalf("%d groups, want 6", n)
	}
	for _, g := range th.Groups() {
		if got, ok := g.ThreadOf(); !ok || got.Group().H != th.Group().H {
			t.Errorf("%s: not the chamfered thread's", g.Name())
		}
	}
	th.Delete()

	c := New()
	cr := c.Sketch.Circle(V(0, 0, 0), 5)
	c.Features.Extrude(30, "shaft")
	ct := c.Features.Thread(cr, ThreadOptions{Designation: "M10", Length: 12, Cosmetic: true}).AsThread()
	for _, g := range ct.Groups() {
		if got, ok := g.ThreadOf(); !ok || got.Group().H != ct.Group().H {
			t.Errorf("%s: not the cosmetic thread's", g.Name())
		}
	}
	ct.SetSuppressed(true)
	if !ct.Suppressed() {
		t.Error("the cosmetic thread is not suppressed")
	}
	ct.SetSuppressed(false)
	ct.Delete()
	if names := groupNames(c); !slices.Equal(names, []string{"sketch-in-plane", "shaft"}) {
		t.Errorf("deleting the cosmetic thread left %v", names)
	}
}

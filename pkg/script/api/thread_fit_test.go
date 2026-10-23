package api

import (
	"math"
	"path/filepath"
	"testing"
)

// threadedNut saves an M10 nut, a Ø20 disc 8 mm high with its hole
// threaded through, of the class given, and returns its path.
func threadedNut(t *testing.T, class string) string {
	t.Helper()
	nut := New()
	disc := nut.Sketch.Circle(V(0, 0, 0), 10)
	nut.Constrain.Coincident(disc.Center(), nut.Origin())
	nut.Features.Extrude(8, "disc")
	nut.Features.SketchIn(nut.Workplane("XY"), "hole")
	hole := nut.Sketch.Circle(V(0, 0, 0), 5)
	nut.Constrain.Coincident(hole.Center(), nut.Origin())
	nut.Features.Thread(hole, ThreadOptions{Designation: "M10", Internal: true, Through: true, Length: 8, Class: class})
	path := filepath.Join(t.TempDir(), "nut.tlspr")
	if err := nut.File.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// threadedRod is an M10 rod 30 mm long threaded along all of it, of
// the class given, and the circle at its top that the thread is on.
func threadedRod(d *Doc, class string) Entity {
	shank := d.Sketch.Circle(V(0, 0, 0), 5)
	d.Constrain.Coincident(shank.Center(), d.Origin())
	rod := d.Features.Extrude(30, "rod")
	var end Entity
	for _, e := range rod.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 15 {
			end = e
		}
	}
	d.Features.Thread(end, ThreadOptions{Designation: "M10", Length: 30, Reverse: true, Through: true, Class: class})
	return end
}

// lowestCircle is g's circle lowest down: the nut's hole at its
// bottom, the other rim from the one its thread is on.
func lowestCircle(g Group) Entity {
	var best Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && math.Abs(e.Radius()-5) < 1e-6 &&
			(best.H.V == 0 || e.Center().Pos().Z < best.Center().Pos().Z) {
			best = e
		}
	}
	return best
}

// TestScrewFitsThreads screws a linked nut onto a threaded rod, of
// classes 6H and 6g: placed half a pitch from where it fits, it
// collides, and the screw mate, picked on the other rim of
// the nut's hole and with the threads' own lead, moves it to fit, and it
// fits at every turn after.
func TestScrewFitsThreads(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("interferences of threads by their meshes, a few seconds")
	}
	path := threadedNut(t, "6H")
	d := New()
	end := threadedRod(d, "6g")
	g, err := d.Assembly.Link(path, "nut")
	if err != nil {
		t.Fatal(err)
	}
	link := g.AsLink()
	o := link.Offset()
	link.Move(V(-o.X, -o.Y, 6.75-o.Z))
	if found, _ := d.Assembly.Interferences(); len(found) != 1 {
		t.Fatalf("half a pitch off: %+v, want the nut to collide", found)
	}

	d.Sketch.Anywhere3D()
	mate := d.Assembly.Screw(end, lowestCircle(g), 0)
	if lead, _ := d.Params.Get(mate.Lead); math.Abs(lead.Value-1.5) > 1e-9 {
		t.Errorf("lead %v, want the M10's 1.5", lead.Value)
	}
	for _, turn := range []string{"0", "90", "-200", "450"} {
		if err := d.Params.Set(mate.Turn, turn); err != nil {
			t.Fatal(err)
		}
		if found, _ := d.Assembly.Interferences(); len(found) != 0 {
			t.Errorf("turned %s: %+v", turn, found)
		}
	}
}

// TestScrewThreadsRefused checks a screw mate of threads that don't
// mate, and of a circle without one: a lead of 0 needs a thread.
func TestScrewThreadsRefused(t *testing.T) {
	t.Parallel()
	d := New()
	end := threadedRod(d, "")
	d.Features.SketchIn(d.Workplane("XY"), "other")
	other := d.Sketch.Circle(V(40, 0, 0), 5)
	ex := d.Features.Extrude(5, "other")
	var rim Entity
	for _, e := range ex.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 2 {
			rim = e
		}
	}
	d.Sketch.Anywhere3D()
	mustFail(t, "no thread", "unless a thread on its circles gives it", func() { d.Assembly.Screw(other, rim, 0) })
	// Two screws' threads.
	d.Features.Thread(rim, ThreadOptions{Designation: "M10", Length: 5, Reverse: true})
	d.Sketch.Anywhere3D()
	mustFail(t, "two screws", "both screws'", func() { d.Assembly.Screw(end, rim, 0) })
}

// TestScrewFitsFaceThreads fits a nut threaded on its hole's face, of
// class 6H, to a bolt threaded on its shaft's face, of class 6g: a
// screw mate of the plate's hole rim and the rod's tip, with the
// threads' lead, puts the nut where nothing interferes, at a turn and
// a quarter as well, where half a pitch off the ridges collide.
func TestScrewFitsFaceThreads(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("interferences of threads, a few seconds")
	}
	nut, face := plateWithHole(t, 10)
	nut.Features.ThreadOnFace(face, ThreadOptions{Designation: "M10", FullLength: true, Class: "6H"})
	path := filepath.Join(t.TempDir(), "plate.tlspr")
	if err := nut.File.Save(path); err != nil {
		t.Fatal(err)
	}

	d := New()
	shank := d.Sketch.Circle(V(0, 0, 0), 5)
	d.Constrain.Coincident(shank.Center(), d.Origin())
	rod := d.Features.Extrude(30, "rod")
	d.Features.ThreadOnFace(rod.Solid().FaceAt(V(5/math.Sqrt2, 5/math.Sqrt2, 15)),
		ThreadOptions{Designation: "M10", FullLength: true, Class: "6g"})
	var tip Entity
	for _, e := range rod.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 15 {
			tip = e
		}
	}
	g, err := d.Assembly.Link(path, "plate")
	if err != nil {
		t.Fatal(err)
	}
	link := g.AsLink()
	o := link.Offset()
	link.Move(V(-o.X, -o.Y, 10-o.Z))
	d.Sketch.Anywhere3D()
	mate := d.Assembly.Screw(tip, lowestCircle(g), 0)
	for _, turn := range []string{"0", "450"} {
		if err := d.Params.Set(mate.Turn, turn); err != nil {
			t.Fatal(err)
		}
		if found, _ := d.Assembly.Interferences(); len(found) != 0 {
			t.Errorf("turned %s: %+v", turn, found)
		}
	}
	// Half a turn with no travel along: half a pitch out of phase.
	if err := d.Params.Set(mate.Turn, "180"); err != nil {
		t.Fatal(err)
	}
	if err := d.Params.Set(mate.Lead, "0"); err != nil {
		t.Fatal(err)
	}
	if found, _ := d.Assembly.Interferences(); len(found) != 1 {
		t.Errorf("half a turn without travel: %+v, want the plate to collide", found)
	}
}

// A thread's class sets its clearance, which Options gives with the
// class; a phase and a class edited in one step of undo come back on
// undo and redo; a class of the other side and a phase with a run-out
// are refused.
func TestThreadClassAndPhase(t *testing.T) {
	t.Parallel()
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 5)
	d.Features.Extrude(30, "rod")
	g := d.Features.Thread(rod, ThreadOptions{Designation: "M10", Length: 15, Class: "6g"})
	th := g.AsThread()
	o := th.Options()
	if o.Class != "6g" || math.Abs(o.Clearance-0.0491) > 0.0005 {
		t.Errorf("options %+v, want 6g's clearance, 0.049", o)
	}
	if got := ThreadClasses("M10", false); len(got) == 0 || got[0] != "6g" {
		t.Errorf("classes of an M10 screw %v, the default first", got)
	}
	if got := ThreadClasses("1/4 BSW", true); len(got) != 0 {
		t.Errorf("classes of a Whitworth nut %v, want none", got)
	}

	d.Host().(*SketchHost).BeginEval()
	o.Class, o.Phase = "4h", 90
	th.Edit(o)
	if e := th.Options(); e.Class != "4h" || e.Phase != 90 || e.Clearance >= o.Clearance {
		t.Errorf("edited %+v, want 4h, tighter than 6g, and a phase of 90", e)
	}
	if r := g.Analyze().Check(); !r.Sound() {
		t.Errorf("the phased thread is not sound: %s", r.String())
	}
	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	if e := th.Options(); e.Class != "6g" || e.Phase != 0 {
		t.Errorf("undo left %+v", e)
	}
	if !d.Edit.Redo() {
		t.Fatal("nothing to redo")
	}
	if e := th.Options(); e.Class != "4h" || e.Phase != 90 {
		t.Errorf("redo left %+v", e)
	}

	o = th.Options()
	o.Class = "6H"
	mustFail(t, "a nut's class", "class of an internal thread", func() { th.Edit(o) })
	o.Class, o.RunOut = "", 2
	mustFail(t, "a phase and a run-out", "can't be phased", func() { th.Edit(o) })
}

// A cosmetic thread keeps its class and phase as a record: the rod is
// left at its nominal size and nothing is turned; made modelled, the
// class sets the clearance and the phase moves the ridges. A screw mate
// takes a cosmetic thread's lead and leaves the part where it is.
func TestThreadCosmeticClassAndPhase(t *testing.T) {
	t.Parallel()
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 5)
	d.Features.Extrude(30, "rod")
	th := d.Features.Thread(rod, ThreadOptions{Designation: "M10", Length: 20, Cosmetic: true, Class: "6g", Phase: 90}).AsThread()
	o := th.Options()
	if o.Class != "6g" || o.Phase != 90 || o.Clearance != 0 || len(th.Groups()) != 1 {
		t.Errorf("cosmetic: %+v in %d groups, want the class and phase kept and the rod not turned", o, len(th.Groups()))
	}
	cylinder := math.Pi * 25 * 30
	if v := d.Active().Analyze().Check().Volume; math.Abs(v-cylinder) > 0.01*cylinder {
		t.Errorf("volume %.2f, want the rod's %.2f", v, cylinder)
	}

	o.Cosmetic = false
	modelled := th.Edit(o)
	if m := modelled.Options(); m.Class != "6g" || m.Phase != 90 || math.Abs(m.Clearance-0.0491) > 0.0005 {
		t.Errorf("modelled: %+v, want 6g's clearance and the phase", m)
	}
	if r := modelled.Finished().Analyze().Check(); !r.Sound() {
		t.Errorf("modelled: %s", r.String())
	}
}

// A screw mate of a cosmetic bolt and a modelled nut takes the
// threads' lead and leaves the nut where it is: a cosmetic thread has no
// ridges to fit.
func TestScrewCosmeticThread(t *testing.T) {
	t.Parallel()
	path := threadedNut(t, "6H")
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
	d.Features.Thread(end, ThreadOptions{Designation: "M10", Length: 30, Reverse: true, Through: true, Cosmetic: true})
	g, err := d.Assembly.Link(path, "nut")
	if err != nil {
		t.Fatal(err)
	}
	link := g.AsLink()
	o := link.Offset()
	link.Move(V(-o.X, -o.Y, 6.3-o.Z))
	d.Sketch.Anywhere3D()
	mate := d.Assembly.Screw(end, lowestCircle(g), 0)
	if lead, _ := d.Params.Get(mate.Lead); math.Abs(lead.Value-1.5) > 1e-9 {
		t.Errorf("lead %v, want the M10's 1.5", lead.Value)
	}
	if z := lowestCircle(g).Center().Pos().Z; math.Abs(z-6.3) > 1e-6 {
		t.Errorf("the nut moved to %g from 6.3", z)
	}
}

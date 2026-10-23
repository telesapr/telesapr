package api

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// threadPlace is where the thread whose helix group is g was made on a
// face.
func threadPlace(t *testing.T, d *Doc, g Group) sketch.ThreadPlace {
	t.Helper()
	p, ok := d.Host().Sketch().ThreadPlaceOf(g.H)
	if !ok {
		t.Fatalf("%s was not made on a face", g.Name())
	}
	return p
}

// A thread on a rod's face is a screw's, from the upper end; Reverse
// starts it at the other end, as does the rim there. Each is one step
// of undo and comes back the same on redo.
func TestThreadOnFace(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Circle(V(0, 0, 0), 4)
	rod := d.Features.Extrude(30, "rod")
	face := rod.Solid().FaceAt(V(4, 0, 5))
	groups := len(d.Groups())

	// Each call made as a script's step, which is one step of undo.
	step := d.Host().(*SketchHost).BeginEval
	step()
	g := d.Features.ThreadOnFace(face, ThreadOptions{Length: 12})
	if o := g.AsThread().Options(); o.Designation != "M8" || o.Internal || o.Length != 12 {
		t.Errorf("options %+v", o)
	}
	if r := g.Analyze().Check(); !r.Sound() {
		t.Errorf("not sound: %s", r.String())
	}
	top := threadPlace(t, d, g).At.Frame.Origin.Z
	if n := len(d.Groups()); n != groups+5 {
		t.Errorf("%d groups, want %d: a datum and the thread's four", n, groups+5)
	}

	if !d.Edit.Undo() || len(d.Groups()) != groups {
		t.Fatalf("undo left %d groups", len(d.Groups()))
	}
	if !d.Edit.Redo() || len(d.Groups()) != groups+5 {
		t.Fatalf("redo gave %d groups", len(d.Groups()))
	}
	if o := d.Groups()[len(d.Groups())-1].AsThread().Options(); o.Designation != "M8" || o.Length != 12 {
		t.Errorf("redone %+v", o)
	}
	d.Edit.Undo()

	step()
	g = d.Features.ThreadOnFace(face, ThreadOptions{Length: 12, Reverse: true})
	if z := threadPlace(t, d, g).At.Frame.Origin.Z; math.Abs(z-top) < 1 {
		t.Errorf("reversed, the thread starts at z = %g too", z)
	}
	d.Edit.Undo()
	rim := rod.Solid().EdgeAt(V(0, 4, 0))
	step()
	g = d.Features.ThreadOnEdge(rim, ThreadOptions{Length: 12})
	if p := threadPlace(t, d, g); math.Abs(p.At.Frame.Origin.Z) > 1e-9 || p.At.Frame.N.Z < 0.999 {
		t.Errorf("from the foot's rim, the thread starts at %v along %v", p.At.Frame.Origin, p.At.Frame.N)
	}
	if r := g.Analyze().Check(); !r.Sound() {
		t.Errorf("from the rim, not sound: %s", r.String())
	}
}

// plateWithHole is a 30 x 30 x 10 plate with a hole of diameter dia
// through it, and the hole's face.
func plateWithHole(t *testing.T, dia float64) (*Doc, SolidFace) {
	t.Helper()
	d := New()
	d.Sketch.Rect(V(-15, -15, 0), V(15, 15, 0))
	d.Features.Extrude(10, "plate")
	d.Features.SketchIn(d.Workplane("XY"), "hole")
	d.Sketch.Circle(V(0, 0, 0), dia/2)
	hole := d.Features.Extrude(12, "drill")
	hole.SetCombine(sketch.CombineDifference)
	// Off the seams between the hole's quarters, where FaceAt finds none.
	c := dia / 2 / math.Sqrt2
	return d, hole.Solid().FaceAt(V(c, c, 5))
}

// A hole's face carries a nut's thread, sized for the hole as drawn at
// its tap drill, as long as the plate is thick and through it.
func TestThreadOnHole(t *testing.T) {
	t.Parallel()
	d, face := plateWithHole(t, 8.5)
	g := d.Features.ThreadOnFace(face, ThreadOptions{})
	o := g.AsThread().Options()
	if o.Designation != "M10" || !o.Internal || !o.Through || !o.FullLength || math.Abs(o.Length-10) > 1e-9 {
		t.Errorf("options %+v", o)
	}
	if r := g.Analyze().Check(); !r.Sound() {
		t.Errorf("not sound: %s", r.String())
	}
	// An edit keeps it a nut's, whatever the options say.
	o.Internal, o.Clearance = false, 0.1
	g.AsThread().Edit(o)
	if o := g.AsThread().Options(); !o.Internal || o.Clearance != 0.1 {
		t.Errorf("edited %+v", o)
	}
}

// A hole drawn at the thread's nominal size, its root, is threaded
// sound, though the boring would lie on it; one wider is refused, as is
// a shaft thinner than the thread's root.
func TestThreadOnHoleAtRoot(t *testing.T) {
	t.Parallel()
	d, face := plateWithHole(t, 10)
	g := d.Features.ThreadOnFace(face, ThreadOptions{})
	if o := g.AsThread().Options(); o.Designation != "M10" {
		t.Errorf("designation %s", o.Designation)
	}
	if r := g.Analyze().Check(); !r.Sound() {
		t.Errorf("not sound: %s", r.String())
	}
	d, face = plateWithHole(t, 10.4)
	failsWith(t, "wider than the thread's root", func() { d.Features.ThreadOnFace(face, ThreadOptions{Designation: "M10"}) })
	d = New()
	d.Sketch.Circle(V(0, 0, 0), 3)
	rod := d.Features.Extrude(30, "rod")
	failsWith(t, "thinner than the thread's root", func() {
		d.Features.ThreadOnFace(rod.Solid().FaceAt(V(3, 0, 5)), ThreadOptions{Designation: "M8"})
	})
}

// A thread made on a face is saved and read back with its datum, and
// follows the face when the part changes after.
func TestThreadOnFaceRoundTrip(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Circle(V(0, 0, 0), 4)
	rod := d.Features.Extrude(30, "rod")
	g := d.Features.ThreadOnFace(rod.Solid().FaceAt(V(4, 0, 5)), ThreadOptions{FullLength: true, Designation: "M8x1"})
	want := g.AsThread().Options()
	path := filepath.Join(t.TempDir(), "rod.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	back, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	gb := back.Groups()[len(back.Groups())-1]
	if o := gb.AsThread().Options(); o != want {
		t.Errorf("read back %+v, want %+v", o, want)
	}
	if r := gb.Analyze().Check(); !r.Sound() {
		t.Errorf("read back, not sound: %s", r.String())
	}
	if p := threadPlace(t, back, gb); p.Face != threadPlace(t, d, g).Face {
		t.Errorf("read back on face %x", uint32(p.Face))
	}
}

// What can't carry a thread is refused with an error that says why.
func TestThreadOnFaceMisuse(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Circle(V(0, 0, 0), 4)
	s := d.Features.Extrude(30, "rod").Solid()
	groups := len(d.Groups())
	failsWith(t, "cylindrical face", func() { d.Features.ThreadOnFace(s.FaceAt(V(0, 0, 0)), ThreadOptions{}) })
	failsWith(t, "shaft's", func() { d.Features.ThreadOnFace(s.FaceAt(V(4, 0, 5)), ThreadOptions{Internal: true}) })
	failsWith(t, "thread length", func() { d.Features.ThreadOnFace(s.FaceAt(V(4, 0, 5)), ThreadOptions{Length: 1}) })
	if n := len(d.Groups()); n != groups {
		t.Errorf("%d groups after the refusals, want %d", n, groups)
	}
}

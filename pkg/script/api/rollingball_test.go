package api

import (
	"math"
	"path/filepath"
	"testing"
)

// A fillet where a branch pipe meets a pipe, two cylinders along a
// closed edge no circle or line is: the rolling ball (roadmap 22.2),
// through the script path, sound and of OpenCASCADE's volume; one step
// of undo; and the same after saving and loading.
func TestFilletBetweenCylinders(t *testing.T) {
	d := New()
	d.Edit.SetChordTolerance(0.002)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(V(0, 0, 0), 10)
	d.Features.Extrude(40, "pipe")
	d.Features.SketchIn(d.Workplane("YZ"), "")
	d.Sketch.Circle(V(0, 0, 20), 5)
	branch := d.Features.Extrude(25, "branch")
	before := branch.Analyze().Check().Volume
	joint := branch.Solid().EdgeAt(V(math.Sqrt(91), 3, 24))
	fill := d.Features.FilletSolid([]SolidEdge{joint}, 2, "joint")
	if p := fill.Problem(); p != "" {
		t.Fatal(p)
	}
	r := fill.Analyze().Check()
	// OpenCASCADE adds 15.769; the mesh's chords at the edge count for
	// about a tenth of a mm³.
	if added := r.Volume - before; !r.Sound() || math.Abs(added-15.769) > 0.3 {
		t.Fatalf("the fillet: %s, added %g", r.Summary(), r.Volume-before)
	}
	path := filepath.Join(t.TempDir(), "tee.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	if !d.Edit.Undo() {
		t.Fatal("no undo")
	}
	if g := d.Groups(); len(g) == 0 || g[len(g)-1].Name() == "joint" {
		t.Fatal("the fillet is still there after undo")
	}
	e := New()
	if err := e.File.Load(path); err != nil {
		t.Fatal(err)
	}
	// The tolerance is the session's, not the file's.
	e.Edit.SetChordTolerance(0.002)
	g := e.Groups()
	again := g[len(g)-1]
	if p := again.Problem(); p != "" {
		t.Fatal(p)
	}
	if r2 := again.Analyze().Check(); !r2.Sound() || math.Abs(r2.Volume-r.Volume) > 1e-6*r.Volume {
		t.Errorf("loaded: %s, volume %g, saved %g", r2.Summary(), r2.Volume, r.Volume)
	}
}

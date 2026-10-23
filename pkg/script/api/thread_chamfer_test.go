package api

import (
	"strings"
	"testing"
)

// A chamfered thread is two groups more, the last of which holds the
// finished part: sound, less than the thread without the chamfer. The
// chamfer reads back, edits off and on in one step of undo each, comes
// back on redo, is written to a script, and is refused on a rounded
// crest.
func TestThreadChamfer(t *testing.T) {
	t.Parallel()
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 4)
	d.Features.Extrude(30, "rod")
	g := d.Features.Thread(rod, ThreadOptions{Designation: "M8", Length: 12, Chamfer: true})
	th := g.AsThread()
	if o := th.Options(); !o.Chamfer {
		t.Errorf("options %+v", o)
	}
	gs := th.Groups()
	if len(gs) != 6 || gs[5].Kind() != "lathe" || th.Finished().H != gs[5].H || d.Active().H != gs[5].H {
		t.Fatalf("groups %v, finished %v, active %v", gs, th.Finished(), d.Active())
	}
	r := th.Finished().Analyze().Check()
	if !r.Sound() {
		t.Fatalf("not sound: %s", r.String())
	}
	chamfered := r.Volume

	d.Host().(*SketchHost).BeginEval()
	o := th.Options()
	o.Chamfer = false
	th = th.Edit(o)
	if gs := th.Groups(); len(gs) != 4 {
		t.Fatalf("without the chamfer: groups %v", gs)
	}
	plain := th.Finished().Analyze().Check().Volume
	if !(plain > chamfered) {
		t.Errorf("volume %.3f without the chamfer, %.3f with it", plain, chamfered)
	}

	d.Host().(*SketchHost).BeginEval()
	o.Chamfer = true
	th = th.Edit(o)
	if gs := th.Groups(); len(gs) != 6 {
		t.Fatalf("chamfered again: groups %v", gs)
	}
	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	if gs := g.AsThread().Groups(); len(gs) != 4 || g.AsThread().Options().Chamfer {
		t.Errorf("undo left groups %v", gs)
	}
	if !d.Edit.Redo() {
		t.Fatal("nothing to redo")
	}
	if gs := g.AsThread().Groups(); len(gs) != 6 || !g.AsThread().Options().Chamfer {
		t.Errorf("redo left groups %v", gs)
	}

	if src, _ := d.Script(); !strings.Contains(src, "Chamfer: true") {
		t.Errorf("script %s", src)
	}

	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !strings.Contains(err.Error(), "rounded crest") {
				t.Errorf("a rounded crest chamfered: %v", err)
			}
		}()
		th.Edit(ThreadOptions{Crest: "rounded", Chamfer: true})
	}()
}

// A cosmetic thread's chamfer cuts the rod's end, which it otherwise
// leaves as it is, by a lathe after its sketch.
func TestThreadChamferCosmetic(t *testing.T) {
	t.Parallel()
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 5)
	d.Features.Extrude(30, "rod")
	whole := d.Active().Analyze().Check().Volume
	g := d.Features.Thread(rod, ThreadOptions{Designation: "M10", Length: 20, Cosmetic: true, Chamfer: true})
	gs := g.AsThread().Groups()
	if len(gs) != 2 || gs[1].Kind() != "lathe" {
		t.Fatalf("groups %v", gs)
	}
	r := d.Active().Analyze().Check()
	if !r.Sound() || !(r.Volume < whole-20) {
		t.Errorf("volume %.2f, the rod's %.2f: %s", r.Volume, whole, r.String())
	}
}

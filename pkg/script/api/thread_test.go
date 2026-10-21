package api

import (
	"strings"
	"testing"
)

// A thread is read back and edited as one: its options come back as
// made, an edit changes them in one step of undo, and a length can
// follow a parameter.
func TestThreadEdit(t *testing.T) {
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 4)
	ext := d.Features.Extrude(30, "rod")
	g := d.Features.Thread(rod, ThreadOptions{Designation: "M8", Length: 12})
	if !g.IsThread() || ext.IsThread() {
		t.Fatalf("IsThread: helix %v, extrusion %v", g.IsThread(), ext.IsThread())
	}
	th := g.AsThread()
	want := ThreadOptions{Designation: "M8", Length: 12, Crest: "standard"}
	if o := th.Options(); o != want {
		t.Errorf("options %+v, want %+v", o, want)
	}
	groups := th.Groups()
	if len(groups) != 4 || groups[3].H != g.H || groups[1].Kind() != "lathe" {
		t.Fatalf("groups %v", groups)
	}

	// Made as a script's step, which is one step of undo.
	d.Host().(*SketchHost).BeginEval()
	if err := d.Params.Set("len", "15"); err != nil {
		t.Fatal(err)
	}
	th.Edit(ThreadOptions{Designation: "M8x1", Crest: "truncated", LengthExpr: "len", Clearance: 0.1})
	want = ThreadOptions{Designation: "M8x1", Length: 15, LengthExpr: "len", Clearance: 0.1, Crest: "truncated"}
	if o := th.Options(); o != want {
		t.Errorf("edited %+v, want %+v", o, want)
	}
	if r := g.Analyze().Check(); !r.Sound() {
		t.Errorf("the edited thread is not sound: %s", r.String())
	}
	if !strings.HasPrefix(g.Name(), "M8x1-") {
		t.Errorf("the edited thread is named %q", g.Name())
	}
	// A designation and a length left out are the thread's own.
	th.Edit(ThreadOptions{Crest: "rounded"})
	if o := th.Options(); o.Designation != "M8x1" || o.LengthExpr != "len" || o.Crest != "rounded" {
		t.Errorf("an edit of the crest alone gave %+v", o)
	}
	if err := d.Params.Set("len", "18"); err != nil {
		t.Fatal(err)
	}
	if o := th.Options(); o.Length != 18 {
		t.Errorf("length %g after the parameter changed, want 18", o.Length)
	}

	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	if o := d.Groups()[len(d.Groups())-1].AsThread().Options(); o.Designation != "M8" || o.Crest != "standard" {
		t.Errorf("undo left %+v", o)
	}
}

// What can't be done to a thread panics with an error that says why.
func TestThreadEditMisuse(t *testing.T) {
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 4)
	ext := d.Features.Extrude(30, "rod")
	th := d.Features.Thread(rod, ThreadOptions{Length: 12}).AsThread()
	for name, fn := range map[string]func(){
		"not a thread":    func() { ext.AsThread() },
		"crest":           func() { th.Edit(ThreadOptions{Crest: "pointed"}) },
		"designation":     func() { th.Edit(ThreadOptions{Designation: "Q9"}) },
		"unknown names":   func() { th.Edit(ThreadOptions{LengthExpr: "nothing+1"}) },
		"too short":       func() { th.Edit(ThreadOptions{Length: 1}) },
		"thread crest":    func() { d.Features.Thread(d.Sketch.Circle(V(20, 0, 0), 4), ThreadOptions{Crest: "flat"}) },
		"thread unknowns": func() { d.Features.Thread(d.Sketch.Circle(V(20, 0, 0), 4), ThreadOptions{LengthExpr: "x"}) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
}

package api

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A thread is read back and edited as one: its options come back as
// made, an edit changes them in one step of undo, and a length can
// follow a parameter.
func TestThreadEdit(t *testing.T) {
	t.Parallel()
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
	if f := th.Finished(); f.H != g.H {
		t.Errorf("one helix: finished in %v, want the helix group %v", f, g)
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

// With level starts, Finished is the last start's helix, which holds
// the whole thread, and the group that is left active.
func TestThreadFinished(t *testing.T) {
	t.Parallel()
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 6)
	d.Features.Extrude(20, "lead")
	th := d.Features.Thread(rod, ThreadOptions{Designation: "M12xPh3P1", Length: 14, LevelStarts: true}).AsThread()
	groups := th.Groups()
	f := th.Finished()
	if len(groups) != 8 || f.H != groups[7].H || f.H != d.Active().H {
		t.Fatalf("finished in %v of %v, active %v", f, groups, d.Active())
	}
	first := groups[3].Analyze().Volume()
	if whole := f.Analyze().Volume(); whole <= first {
		t.Errorf("the finished solid has %g mm³, the first start's %g", whole, first)
	}
}

// What can't be done to a thread panics with an error that says why.
func TestThreadEditMisuse(t *testing.T) {
	t.Parallel()
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

// A thread that doesn't fit its part is refused, saying why, and one
// whose part is made shorter afterwards says so as its problem.
func TestThreadPastPart(t *testing.T) {
	t.Parallel()
	d := New()
	rod := d.Sketch.Circle(V(0, 0, 0), 4)
	ext := d.Features.Extrude(30, "rod")
	groups := len(d.Groups())
	for name, c := range map[string]struct {
		o    ThreadOptions
		want string
	}{
		"longer":    {ThreadOptions{Designation: "M8", Length: 40}, "thread is longer than its part, which reaches 30 mm along it"},
		"other way": {ThreadOptions{Designation: "M8", Length: 12, Reverse: true}, "no part along the thread"},
		"thick":     {ThreadOptions{Designation: "M12", Length: 12}, "part is thinner than the thread's root"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want %q", name, err, c.want)
				}
			}()
			d.Features.Thread(rod, c.o)
		}()
	}
	if n := len(d.Groups()); n != groups {
		t.Errorf("refused threads left %d groups, want %d", n, groups)
	}

	th := d.Features.Thread(rod, ThreadOptions{Designation: "M8", Length: 12})
	if p := th.Problem(); p != "" {
		t.Errorf("problem %q", p)
	}
	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !strings.Contains(err.Error(), "longer than its part") {
				t.Errorf("an edit past the part: %v", err)
			}
		}()
		th.AsThread().Edit(ThreadOptions{Length: 31})
	}()
	if err := ext.SetExpr(sketch.GroupExtent, "10"); err != nil {
		t.Fatal(err)
	}
	if p := th.Problem(); !strings.Contains(p, "longer than its part, which reaches 10 mm") {
		t.Errorf("a part shorter than its thread: problem %q", p)
	}
}

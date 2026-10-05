package ui

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// scriptConsole runs lines on the application as a console does, and
// returns what it showed.
func scriptConsole(t *testing.T, ss *Application, lines ...string) string {
	t.Helper()
	var out bytes.Buffer
	host := ss.ScriptHost()
	host.Messages = &out
	e, err := script.New(script.Options{Host: host, Stdout: &out, Stderr: &out})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.REPL(context.Background(), strings.NewReader(strings.Join(lines, "\n")), &out, false); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// A script draws on the application's sketch; each of its lines is a
// step of undo, and the windows follow.
func TestScriptHostDraws(t *testing.T) {
	ss, p := newTestApp(t)
	got := scriptConsole(t, ss,
		`sides := doc.Rect(V(0, 0, 0), V(30, 20, 0))`,
		`doc.Length(sides[0], 40)`,
		`math.Round(sides[0].Length())`,
		`ex := doc.Extrude(10, "box")`,
		`doc.Active()`,
		`math.Round(ex.Volume())`,
	)
	if want := "c009-pt-pt-distance = 40\n40\ng003-box\n8000\n"; got != want {
		t.Errorf("the console showed %q, want %q", got, want)
	}
	sk := ss.SK
	if got := sk.Undo.Cnt; got != 3 {
		t.Errorf("%d steps of undo, want 3: the rectangle, its length and the extrusion", got)
	}
	if g := sk.GetGroup(sk.Active); g == nil || g.Name != "box" || g.Type != sketch.GroupExtrude {
		t.Errorf("the active group is %+v, want the extrusion", g)
	}
	if !sk.Unsaved {
		t.Error("the sketch isn't marked as changed")
	}
	// Nothing is left for the event loop to regenerate, and no dialog
	// was shown: the headless one would have panicked.
	if sk.RegenerationPending() {
		t.Error("a regeneration is still pending after the script")
	}
	if ss.messageSink != nil {
		t.Error("messages still go to the script after it ended")
	}
	p.RunDueTimers(100)

	ss.UndoUndo()
	if g := ss.SK.GetGroup(ss.SK.Active); g == nil || g.Type != sketch.GroupDrawingWorkplane {
		t.Errorf("after undo the active group is %+v, want the sketch", g)
	}
	if n := len(ss.SK.Groups); n != 2 {
		t.Errorf("after undo there are %d groups, want 2", n)
	}
}

// What the application would say in a dialog goes to the script: as
// the error of the call that failed, or else to its output.
func TestScriptHostMessages(t *testing.T) {
	ss, _ := newTestApp(t)
	dir := t.TempDir()
	got := scriptConsole(t, ss,
		`doc.Export("`+filepath.Join(dir, "none", "x.stl")+`")`,
		`doc.Circle(V(0, 0, 0), 5)`,
		`doc.Extrude(3, "disc")`,
		`doc.Export("`+filepath.Join(dir, "disc.xyz")+`")`,
		`doc.Export("`+filepath.Join(dir, "disc.stl")+`")`,
		`doc.Save("`+filepath.Join(dir, "disc.tlspr")+`")`,
		`doc.Unsaved()`,
		`doc.Load("`+filepath.Join(dir, "none.tlspr")+`")`,
	)
	want := "Active group mesh is empty; nothing to export.\n" +
		"circle r004-circle, radius 5.000\n" +
		"g003-disc\n" +
		"can't tell what to export from the name '" + filepath.Join(dir, "disc.xyz") + "'\n" +
		"nil\nnil\nfalse\n" +
		"Couldn't read from file '" + filepath.Join(dir, "none.tlspr") + "'\n"
	if got != want {
		t.Errorf("the console showed:\n%s\nwant:\n%s", got, want)
	}
	// The file that couldn't be loaded left the sketch as it was.
	if ss.SaveFile != filepath.Join(dir, "disc.tlspr") || len(ss.SK.Groups) != 3 {
		t.Errorf("the sketch's file is %q, with %d groups", ss.SaveFile, len(ss.SK.Groups))
	}
	if got := scriptConsole(t, ss, `doc.Load("`+filepath.Join(dir, "disc.tlspr")+`")`, `doc.Groups()`); got !=
		"nil\n0: g002-sketch-in-plane\n1: g003-disc\n" {
		t.Errorf("loading the saved sketch showed %q", got)
	}
	if ss.ExportMode {
		t.Error("the application is still in export mode after the script's export")
	}
}

func TestScriptHostSelection(t *testing.T) {
	ss, _ := newTestApp(t)
	got := scriptConsole(t, ss,
		`a := doc.Line(V(0, 0, 0), V(10, 0, 0))`,
		`b := doc.Line(V(0, 0, 0), V(0, 10, 0))`,
		`len(doc.Selection())`,
		`doc.Select(a, b.End())`,
		`doc.Selection()`,
	)
	want := "0\n0: line r004-line-segment\n1: point (0.000, 10.000, 0.000) of r005-line-segment\n"
	if got != want {
		t.Errorf("the console showed %q, want %q", got, want)
	}
	if n := len(ss.GW.selection); n != 2 {
		t.Errorf("%d things are selected in the graphics window, want 2", n)
	}
}

// loopPlatform is a headless platform with an event loop on another
// goroutine, as the windowed one has.
type loopPlatform struct {
	*headless.Platform
	posted chan func()
	done   chan struct{}
}

func (p *loopPlatform) PostToLoop(fn func())      { p.posted <- fn }
func (p *loopPlatform) LoopDone() <-chan struct{} { return p.done }

// A script on a goroutine of its own works on the sketch through the
// event loop, where a call made from inside another runs in place and
// a panic comes back to the script.
func TestScriptHostRunsOnLoop(t *testing.T) {
	p := &loopPlatform{Platform: headless.New(), posted: make(chan func(), 16), done: make(chan struct{})}
	stopped := make(chan struct{})
	var ss *Application
	go func() {
		defer close(stopped)
		for fn := range p.posted {
			fn()
		}
	}()
	// The application is made on the loop too.
	made := make(chan struct{})
	p.PostToLoop(func() {
		ss = New(p)
		ss.Init()
		close(made)
	})
	<-made

	host := ss.ScriptHost()
	doc := api.NewDoc(host)
	var onLoop, nested int64
	host.Run(func() {
		onLoop = goid()
		host.Run(func() { nested = goid() })
	})
	if onLoop == goid() || nested != onLoop {
		t.Fatalf("Run ran on goroutine %d, nested on %d; the test is on %d", onLoop, nested, goid())
	}

	lines := doc.Rect(api.V(0, 0, 0), api.V(4, 3, 0))
	if got := lines[1].Length(); got < 2.999 || got > 3.001 {
		t.Errorf("the rectangle's side is %g long, want 3", got)
	}
	func() {
		defer func() {
			if r := recover(); r == nil || !strings.Contains(r.(error).Error(), "isn't a point") {
				t.Errorf("a failed call panicked with %v in the script's goroutine", r)
			}
		}()
		lines[0].Pos()
	}()
	// The loop survived the panic.
	if !lines[0].Valid() {
		t.Error("the line is gone after a failed call")
	}

	close(p.posted)
	<-stopped
	close(p.done)
	func() {
		defer func() {
			if r := recover(); r != ErrLoopEnded {
				t.Errorf("a call after the loop ended panicked with %v", r)
			}
		}()
		p.posted = make(chan func(), 1)
		lines[0].Valid()
	}()
}

// A script's chord tolerance is the application's setting, which the
// regenerations after it keep: set on the sketch alone, the application
// put its own back at the next one.
func TestScriptHostChordTolerance(t *testing.T) {
	ss, _ := newTestApp(t)
	got := scriptConsole(t, ss,
		`doc.SetChordTolerance(0.01)`,
		`doc.Rect(V(0, 0, 0), V(30, 20, 0))`,
		`doc.Extrude(10, "")`,
		`doc.ChordTolerance()`,
	)
	if want := "0.01\n"; !strings.HasSuffix(got, want) || ss.ChordTol != 0.01 || ss.SK.ChordTolPct != 0.01 {
		t.Errorf("the console showed %q; the setting is %g, the sketch's %g", got, ss.ChordTol, ss.SK.ChordTolPct)
	}
}

// Deleting the sketch of the active extrusion takes the extrusion with
// it; the group before them becomes active, not the first sketch, so
// that new groups come at the end. A tangent arc in a step is part of
// the step's undo.
func TestScriptHostDeleteActiveAndUndo(t *testing.T) {
	ss, _ := newTestApp(t)
	got := scriptConsole(t, ss,
		`first := doc.Active()`,
		`doc.Rect(V(0, 0, 0), V(10, 10, 0)); box := doc.Extrude(5, "box")`,
		`second := doc.SketchIn(first.Workplane(), "second"); doc.Circle(V(20, 20, 0), 3); doc.Extrude(5, "pin")`,
		`second.Delete()`,
		`doc.Active() == box`,
		`doc.SketchIn(first.Workplane(), "third"); ls := doc.Polyline(false, V(0, 0, 0), V(20, 0, 0), V(20, 20, 0)); doc.TangentArc(ls[0].End(), 5, true)`,
	)
	if !strings.HasSuffix(got, "Choose Edit -> Undo to undelete all elements.\ntrue\n") {
		t.Fatalf("the console showed %q", got)
	}
	n := ss.SK.Undo.Cnt
	if n != 4 {
		t.Errorf("%d steps of undo, want 4: the box, the pin, the delete and the rounded corner", n)
	}
	ss.UndoUndo()
	if g := ss.SK.GetGroup(ss.SK.Active); g == nil || g.Name != "box" {
		t.Fatalf("one undo after the rounded corner leaves %+v active, want the box", g)
	}
}

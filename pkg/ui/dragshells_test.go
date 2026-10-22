package ui

import (
	"math"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Dragging the geometry makes the solids on the interface's own thread,
// not in a shell job: a job takes a copy of the sketch and gives its
// solids back only if the sketch is defined the same way as when it was
// made, and a drag changes the sketch on every frame, so the job's work
// would be taken back every time and the parts would sit still while the
// pointer moved. The solid keeps its edges through the drag, as any other
// regeneration.
func TestDragMakesSolidsHere(t *testing.T) {
	ss, _ := newTestApp(t)
	sk := ss.SK
	ss.BackgroundShells = true
	doc := api.NewDoc(ss.ScriptHost())
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	block := doc.Features.Extrude(5, "")
	corner := sides[1].End()

	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 800, 600
	ss.GW.ZoomToFit(false, false)
	w.Render()

	p := ss.GW.ProjectPoint(corner.Pos())
	// A regeneration rebuilds the groups, so each one is looked up again.
	ex := func() *sketch.Group { return ss.SK.Group(block.H) }
	outlines := func() int { return len(ex().DisplayOutlines.Outlines) }
	if outlines() == 0 {
		t.Fatal("the block has no outlines to begin with")
	}

	// Press on the corner and start dragging it. What the hit test picks
	// under the corner is the extrusion's copy of it, so the window is
	// told to drag the sketch's own point, as it would where that copy
	// isn't in the way.
	ss.GW.MouseLeftDown(p.U, p.V, false, false)
	ss.GW.hover.Clear()
	ss.GW.pending.operation = pendingDraggingPoints
	ss.GW.pending.points = []core.HEntity{corner.H}
	before, edges := meshX(t, ex()), outlines()

	ss.GW.orig.mouse = p
	ss.GW.dragPoints(p.U+60, p.V, false, false)
	ss.Refresh()
	if ex().ShellsStale || ss.shellJobs.job != nil {
		t.Fatalf("a drag made its solids in a job: stale %v, job %v",
			ex().ShellsStale, ss.shellJobs.job != nil)
	}
	w.Render()
	if after := meshX(t, ex()); math.Abs(after-before) < 1e-6 {
		t.Errorf("the solid did not follow the drag: x %g then %g", before, after)
	}
	if n := outlines(); n != edges {
		t.Errorf("the solid has %d outlines after the drag, want the %d it had",
			n, edges)
	}
	if v := block.Analyze().Check().Volume; v <= 1000 {
		t.Errorf("volume %g, want the dragged block to be bigger than 1000", v)
	}

	// Letting go: the regeneration the last frame of the drag scheduled,
	// which has not been made yet, is made here, so that the solids are
	// the ones the drag ended with.
	sk.MarkGroupDirtyByEntity(corner.H)
	ss.GW.MouseLeftUp(p.U+60, p.V, false, false)
	if ex().ShellsStale || ss.shellJobs.job != nil {
		t.Errorf("the drag ended with its solids out of date: stale %v, job %v",
			ex().ShellsStale, ss.shellJobs.job != nil)
	}
	w.Render()
	if outlines() == 0 {
		t.Error("the outlines were dropped when the drag ended")
	}
}

// meshX is where a group's display mesh is, which moves when the solid
// does.
func meshX(t *testing.T, g *sketch.Group) float64 {
	t.Helper()
	if len(g.DisplayMesh.Triangles) == 0 {
		t.Fatal("the group has no display mesh")
	}
	x := 0.0
	for _, tr := range g.DisplayMesh.Triangles {
		x += tr.A.X + tr.B.X + tr.C.X
	}
	return x
}

// With DragSolidsLate, a drag of the geometry leaves its solids to a
// shell job, and the frames after don't stop it: when it ends its solids
// go in, those of where the corner was when it started, still marked out
// of date, and the next job makes them for where the corner is then.
// When the pointer is let go, the last job makes the solids of where the
// drag ended.
func TestDragSolidsLate(t *testing.T) {
	ss, p := newTestApp(t)
	ss.BackgroundShells = true
	ss.DragSolidsLate = true
	doc := api.NewDoc(ss.ScriptHost())
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	block := doc.Features.Extrude(5, "")
	corner := sides[1].End()

	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 800, 600
	ss.GW.ZoomToFit(false, false)
	w.Render()
	ex := func() *sketch.Group { return ss.SK.Group(block.H) }

	at := ss.GW.ProjectPoint(corner.Pos())
	ss.GW.MouseLeftDown(at.U, at.V, false, false)
	ss.GW.hover.Clear()
	ss.GW.pending.operation = pendingDraggingPoints
	ss.GW.pending.points = []core.HEntity{corner.H}
	before := meshX(t, ex())

	// The first frame starts a job and leaves the solid as it was.
	ss.GW.orig.mouse = at
	ss.GW.dragPoints(at.U+40, at.V, false, false)
	ss.Refresh()
	w.Render()
	first := ss.shellJobs.job
	if first == nil || !ex().ShellsStale {
		t.Fatalf("the drag's frame made no job: stale %v", ex().ShellsStale)
	}
	if x := meshX(t, ex()); x != before {
		t.Fatalf("the solid moved before its job ended: x %g then %g", before, x)
	}

	// The second frame moves on; the first job runs on.
	ss.GW.dragPoints(at.U+80, at.V, false, false)
	ss.Refresh()
	if ss.shellJobs.job != first {
		t.Fatal("the next frame stopped the drag's job")
	}

	// The first job ends: its solid goes in, a step behind, and the
	// next job starts.
	for range 5000 {
		if ss.shellJobs.job != first {
			break
		}
		// One round of the timers at a time, so that the next job is
		// looked at only on the next round.
		p.RunDueTimers(1)
		time.Sleep(time.Millisecond)
	}
	w.Render()
	late := meshX(t, ex())
	if late == before {
		t.Error("the first job's solid didn't go in")
	}
	if !ex().ShellsStale || ss.shellJobs.job == nil {
		t.Errorf("after the first job: stale %v, job %v", ex().ShellsStale, ss.shellJobs.job != nil)
	}

	// Letting go: the last job makes the solid where the drag ended.
	ss.GW.MouseLeftUp(at.U+80, at.V, false, false)
	waitShells(t, ss, p)
	w.Render()
	if ex().ShellsStale {
		t.Error("the solid is out of date after the drag's jobs")
	}
	if x := meshX(t, ex()); x == late || x == before {
		t.Errorf("the solid didn't follow to where the drag ended: x %g, %g, then %g", before, late, x)
	}
	if v := block.Analyze().Check().Volume; v <= 1000 {
		t.Errorf("volume %g, want the dragged block to be bigger than 1000", v)
	}
}

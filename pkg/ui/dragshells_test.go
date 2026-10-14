package ui

import (
	"math"
	"testing"

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

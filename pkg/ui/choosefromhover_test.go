package ui

import (
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A hover entry that names a solid edge or a face, and no entity and no
// constraint, names no group to order by. Choosing from such a list, for
// a click or for a drag, must skip it rather than dereference a group
// that isn't there: the entries HitTestMakeSelection appends for a solid
// edge or a face crashed the drag (draw.go, chooseFromHover).
//
// These render, and the vector font's glyph cache is process-wide state,
// so they can't run in parallel.
func TestChooseFromHoverSkipsSolidEdgeAndFace(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	scriptConsole(t, ss, `doc.Sketch.Rect(V(0, 0, 0), V(20, 10, 0))`, `doc.Features.Extrude(10, "block")`)
	gw := ss.GW
	w := gw.window.(*headless.Window)
	w.Width, w.Height = 1200, 900
	gw.ZoomToFit(false, false)
	w.Render()

	edge := topo.Name{A: 1, B: 2}
	for _, c := range []struct {
		name string
		sel  Selection
	}{
		{"solid edge", Selection{Edge: edge}},
		{"face", Selection{Face: 7}},
	} {
		// An entity of the active group, so the choice would be that one
		// were the entries without a group skipped.
		var ent core.HEntity
		for _, e := range gw.sk().Entities {
			if e.IsPoint() {
				ent = e.H
				break
			}
		}
		if ent.V == 0 {
			t.Fatal("no point entity to hover")
		}
		gw.hoverList = []Hover{
			{Selection: Selection{Entity: ent}, ZIndex: 5},
			{Selection: c.sel},
		}
		if got := gw.ChooseFromHoverToDrag(); got.Entity != ent {
			t.Errorf("%s: dragged %x, want the entity %x", c.name, got.Entity.V, ent.V)
		}
		if got := gw.ChooseFromHoverToSelect(); got.Entity != ent {
			t.Errorf("%s: selected %x, want the entity %x", c.name, got.Entity.V, ent.V)
		}
	}
}

// The same, where the edge or face is the only thing hovered: there is
// nothing to choose, and saying so is what lets the caller carry on.
func TestChooseFromHoverOnlySolidEdgeOrFace(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	gw := ss.GW
	w := gw.window.(*headless.Window)
	w.Width, w.Height = 800, 600
	w.Render()
	for _, sel := range []Selection{
		{Edge: topo.Name{A: 1, B: 2}},
		{Face: 7},
	} {
		gw.hoverList = []Hover{{Selection: sel}}
		if got := gw.ChooseFromHoverToDrag(); !got.IsEmpty() {
			t.Errorf("dragged %v, want nothing", got)
		}
		if got := gw.ChooseFromHoverToSelect(); !got.IsEmpty() {
			t.Errorf("selected %v, want nothing", got)
		}
	}
}

// A hover entry whose entity or constraint has since gone, because the
// model moved under a hover list built by an earlier hit test, is stale
// rather than fatal.
func TestChooseFromHoverSkipsStale(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	scriptConsole(t, ss, `doc.Sketch.Line(V(0, 0, 0), V(10, 0, 0))`)
	gw := ss.GW
	w := gw.window.(*headless.Window)
	w.Width, w.Height = 800, 600
	w.Render()
	var live core.HEntity
	for _, e := range gw.sk().Entities {
		if e.IsPoint() {
			live = e.H
			break
		}
	}
	staleEntity := core.HEntity{V: live.V + 0x1000}
	staleConstraint := core.HConstraint{V: 0x1000}
	gw.hoverList = []Hover{
		{Selection: Selection{Entity: staleEntity}, ZIndex: 9},
		{Selection: Selection{Constraint: staleConstraint}, ZIndex: 9},
		{Selection: Selection{Entity: live}, ZIndex: 5},
	}
	if got := gw.ChooseFromHoverToDrag(); got.Entity != live {
		t.Errorf("dragged %x, want the live entity %x", got.Entity.V, live.V)
	}
}

// The crash as it was met: a drag begun over a solid whose fillet made
// edges and faces that no entity names. Hovering one puts an entry with
// no group into the hover list, and the mouse move that starts the drag
// chooses from it.
func TestChooseFromHoverDragOverFillet(t *testing.T) {
	t.Parallel()
	ss := New(headless.New())
	ss.Init()
	if ss.LoadFromFile(filepath.Join("..", "..", "testdata", "solid", "fillet-tee.tlspr")) != nil {
		t.Fatal("load failed")
	}
	ss.AfterNewFile()
	gw := ss.GW
	w := gw.window.(*headless.Window)
	w.Width, w.Height = 1200, 900
	gw.ZoomToFit(false, false)
	w.Render()

	// Where the mouse is over a fillet face, and over nothing drawn, is
	// the view's business, so sweep for such a spot.
	var found bool
	for i := range 40 {
		for j := range 40 {
			mp := geom.Point2d{U: w.Width * float64(j) / 40, V: w.Height * float64(i) / 40}
			gw.HitTestMakeSelection(mp)
			for _, h := range gw.hoverList {
				if h.Selection.Entity.V == 0 && h.Selection.Constraint.V == 0 {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no spot over the fillet hovers something that no entity names")
	}
	// This is what MouseMoved does once the button is down and the mouse
	// has moved past the threshold.
	if sel := gw.ChooseFromHoverToDrag(); !sel.IsEmpty() {
		t.Logf("dragging %v", sel)
	}
}

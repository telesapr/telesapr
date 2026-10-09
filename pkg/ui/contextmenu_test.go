package ui

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// contextMenuFor selects he alone, opens the context menu and returns
// its labels and the menu, triggering nothing.
func contextMenuFor(t *testing.T, ss *Application, p *headless.Platform, he core.HEntity) ([]string, *headless.Menu) {
	t.Helper()
	gw := ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Entity: he})
	var shown *headless.Menu
	p.PopUpHook = func(m *headless.Menu) *headless.MenuItem { shown = m; return nil }
	gw.showContextMenu(0, 0)
	if shown == nil {
		t.Fatal("no context menu")
	}
	var labels []string
	for _, it := range shown.Items {
		if !it.Separator {
			labels = append(labels, it.Label)
		}
	}
	return labels, shown
}

func TestContextMenuSolid(t *testing.T) {
	ss, p := newTestApp(t)
	if err := ss.LoadFromFile(filepath.Join("..", "..", "testdata", "topo", "slot.tlspr")); err != nil {
		t.Fatal(err)
	}
	sk := ss.SK
	var sketchLine, extrudedLine core.HEntity
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if e.Type != core.EntityLineSegment {
			continue
		}
		switch sk.Group(e.Group).Type {
		case sketch.GroupDrawingWorkplane:
			if sketchLine.V == 0 {
				sketchLine = e.H
			}
		case sketch.GroupExtrude:
			if extrudedLine.V == 0 {
				extrudedLine = e.H
			}
		}
	}
	if sketchLine.V == 0 || extrudedLine.V == 0 {
		t.Fatalf("slot.tlspr: sketch line %v, extruded line %v", sketchLine, extrudedLine)
	}

	labels, _ := contextMenuFor(t, ss, p, extrudedLine)
	for _, want := range []string{"Fillet Edges", "Chamfer Edges", "Measure Selection", "Group"} {
		if !slices.Contains(labels, want) {
			t.Errorf("an extruded edge's menu lacks %q: %v", want, labels)
		}
	}

	labels, menu := contextMenuFor(t, ss, p, sketchLine)
	if slices.Contains(labels, "Fillet Edges") {
		t.Errorf("a sketch's line offers Fillet Edges: %v", labels)
	}
	hg := sk.Entity(sketchLine).Group
	group := menu.Find("Group")
	if group == nil || group.SubMenu == nil {
		t.Fatalf("no Group submenu: %v", labels)
	}
	hide := group.SubMenu.Find("Hide")
	if hide == nil {
		t.Fatal("no Hide in the Group submenu")
	}
	hide.OnTrigger()
	if sk.Group(hg).Visible {
		t.Error("Hide left the group shown")
	}

	// Leaving the group out of the model is one undo step.
	_, menu = contextMenuFor(t, ss, p, extrudedLine)
	he := sk.Entity(extrudedLine).Group
	menu.Find("Group").SubMenu.Find("Suppress with What Uses It").OnTrigger()
	if !sk.Group(he).SuppressAll {
		t.Fatal("the group was not suppressed")
	}
	ss.UndoUndo()
	if sk.Group(he).SuppressAll {
		t.Error("undo did not bring the group back")
	}
}

func TestContextMenuEditValue(t *testing.T) {
	ss, p := newTestApp(t)
	if err := ss.LoadFromFile(filepath.Join("..", "..", "testdata", "topo", "slot.tlspr")); err != nil {
		t.Fatal(err)
	}
	var hc core.HConstraint
	for _, c := range ss.SK.Constraints {
		if c.HasLabel() && !c.Reference && c.Type != core.ConstraintComment {
			hc = c.H
			break
		}
	}
	if hc.V == 0 {
		t.Skip("slot.tlspr has no dimension")
	}
	gw := ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Constraint: hc})
	var shown *headless.Menu
	p.PopUpHook = func(m *headless.Menu) *headless.MenuItem { shown = m; return nil }
	gw.showContextMenu(0, 0)
	it := shown.Find("Edit Value...")
	if it == nil {
		t.Fatal("no Edit Value... for a dimension")
	}
	it.OnTrigger()
	if gw.constraintBeingEdited != hc {
		t.Errorf("editing %v, want %v", gw.constraintBeingEdited, hc)
	}
}

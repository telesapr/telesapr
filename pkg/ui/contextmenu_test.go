package ui

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
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
	t.Parallel()
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
	t.Parallel()
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

// The spline entries of the context menu reach the sketch's entry
// points: a point added to the cubic of spline_line.tlspr and taken
// away again, the coincidence of the line's start with its end deleted,
// each one step of undo.
func TestContextMenuSpline(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	if err := ss.LoadFromFile(filepath.Join("..", "..", "testdata", "ui", "spline_line.tlspr")); err != nil {
		t.Fatal(err)
	}
	sk := ss.SK
	var cubic *sketch.Request
	for i := range sk.Requests {
		if sk.Requests[i].Type == sketch.RequestCubic {
			cubic = &sk.Requests[i]
		}
	}
	if cubic == nil {
		t.Fatal("no cubic in spline_line.tlspr")
	}
	hr := cubic.H
	extra := cubic.ExtraPoints
	trigger := func(he core.HEntity, label string) {
		t.Helper()
		labels, menu := contextMenuFor(t, ss, p, he)
		it := menu.Find(label)
		if it == nil {
			t.Fatalf("no %q in %v", label, labels)
		}
		it.OnTrigger()
		ss.GenerateAll(sketch.GenerateDirty)
	}

	trigger(hr.Entity(0), "Add Spline Point")
	if got := sk.Request(hr).ExtraPoints; got != extra+1 {
		t.Fatalf("%d extra points after one was added, want %d", got, extra+1)
	}
	trigger(sk.Entity(hr.Entity(0)).Point[2], "Remove Spline Point")
	if got := sk.Request(hr).ExtraPoints; got != extra {
		t.Fatalf("%d extra points after one was removed, want %d", got, extra)
	}
	ss.UndoUndo()
	if got := sk.Request(hr).ExtraPoints; got != extra+1 {
		t.Errorf("undo of the removal: %d extra points", got)
	}

	end := sk.Entity(hr.Entity(0)).Point[extra+4]
	if !sk.PointIsCoincident(end) {
		t.Fatal("the cubic's end is coincident with nothing")
	}
	trigger(end, "Delete Point-Coincident Constraint")
	if sk.PointIsCoincident(end) {
		t.Error("the coincidence is still there")
	}
	ss.UndoUndo()
	if !sk.PointIsCoincident(end) {
		t.Error("undo did not bring the coincidence back")
	}
}

// The context menu of one face of the solid gives that face an
// appearance of its own, as View.SetFaceAppearance does for a script,
// and takes it back with the group's own.
func TestContextMenuFaceAppearance(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	if err := ss.LoadFromFile(filepath.Join("..", "..", "testdata", "topo", "slot.tlspr")); err != nil {
		t.Fatal(err)
	}
	sk := ss.SK
	var faceEntity core.HEntity
	for i := range sk.Entities {
		if e := &sk.Entities[i]; e.IsFace() && sk.Group(e.Group).Type == sketch.GroupExtrude {
			faceEntity = e.H
		}
	}
	if faceEntity.V == 0 {
		t.Fatal("the extrusion has no face entity")
	}
	labels, menu := contextMenuFor(t, ss, p, faceEntity)
	appearance := menu.Find("Appearance")
	if appearance == nil || appearance.SubMenu == nil {
		t.Fatalf("the face's menu has no Appearance submenu: %v", labels)
	}
	glossy := appearance.SubMenu.Find("glossy")
	if glossy == nil {
		t.Fatalf("no glossy in the Appearance submenu: %v", appearance.SubMenu.Items)
	}
	glossy.OnTrigger()
	if got := sk.AppearanceOf(srf.FaceID(faceEntity.V)); got != "glossy" {
		t.Errorf("the face's appearance is %q, want glossy", got)
	}
	// Taking it back gives the face its group's again.
	_, menu = contextMenuFor(t, ss, p, faceEntity)
	menu.Find("Appearance").SubMenu.Find("The group's own").OnTrigger()
	if got := sk.AppearanceOf(srf.FaceID(faceEntity.V)); got == "glossy" {
		t.Error("the face keeps the appearance it was given")
	}
	ss.UndoUndo()
	if got := sk.AppearanceOf(srf.FaceID(faceEntity.V)); got != "glossy" {
		t.Errorf("undo took away the appearance, and left %q", got)
	}
}

package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The context menu offers what applies to what was right-clicked: the
// sketch's entities and constraints as SolveSpace had them, and the
// features that take edges and faces of the solid, the group a thing
// belongs to, and a dimension's value. Each entry runs the handler its
// main-menu entry does, so the two can't differ.

// showContextMenu shows the context menu for a right click at x, y,
// selecting what is hovered first.
func (gw *graphicsWindow) showContextMenu(x, y float64) {
	ss := gw.ss
	sk := gw.sk()

	// The current mouse location
	scale := gw.view.Scale
	v := gw.view.Offset.ScaledBy(-1)
	v = v.Plus(gw.view.ProjRight.ScaledBy(x / scale))
	v = v.Plus(gw.view.ProjUp.ScaledBy(y / scale))

	menu := ss.Platform.CreateMenu()
	gw.context.active = true

	if !gw.hover.IsEmpty() {
		gw.MakeSelected(&gw.hover)
		ss.ScheduleShowTW()
	}
	gw.GroupSelection()
	gs := gw.gs

	itemsSelected := gs.n > 0 || gs.constraints > 0
	// Edges and faces of a solid picked on the model are selected too,
	// though no entity draws them.
	anyPicked := itemsSelected || len(gs.solidEdge) > 0 || len(gs.solidFace) > 0
	if itemsSelected {
		if gs.stylables > 0 {
			styleMenu := menu.AddSubMenu("Assign to Style")
			empty := true
			for _, s := range sk.Styles {
				if s.H.V < core.StyleFirstCustom {
					continue
				}
				hv := s.H.V
				styleMenu.AddItem(s.DescriptionString(), func() { ss.AssignSelectionToStyle(hv) }, true)
				empty = false
			}
			if !empty {
				styleMenu.AddSeparator()
			}
			styleMenu.AddItem("No Style", func() { ss.AssignSelectionToStyle(0) }, true)
			styleMenu.AddItem("Newly Created Custom Style...", func() {
				vs := ss.CreateCustomStyle(true)
				ss.AssignSelectionToStyle(vs)
				gw.ForceTextWindowShown()
			}, true)
		}
		if gs.n+gs.constraints == 1 {
			menu.AddItem("Group Info", func() {
				var hg core.HGroup
				switch {
				case gs.entities == 1:
					hg = sk.Entity(gs.entity[0]).Group
				case gs.points == 1:
					hg = sk.Entity(gs.point[0]).Group
				case gs.constraints == 1:
					hg = sk.Constraint(gs.constraint[0]).Group
				default:
					return
				}
				gw.ClearSelection()

				ss.TW.GoToScreen(screenGroupInfo)
				ss.TW.shown.group = hg
				ss.ScheduleShowTW()
				gw.ForceTextWindowShown()
			}, true)
		}
		if gs.n+gs.constraints == 1 && gs.stylables == 1 {
			menu.AddItem("Style Info", func() {
				var hs core.HStyle
				switch {
				case gs.entities == 1:
					hs = sk.StyleForEntity(gs.entity[0])
				case gs.points == 1:
					hs = sk.StyleForEntity(gs.point[0])
				case gs.constraints == 1:
					hs = constraintStyle(sk.Constraint(gs.constraint[0]))
				default:
					return
				}
				gw.ClearSelection()

				ss.TW.GoToScreen(screenStyleInfo)
				ss.TW.shown.style = hs
				ss.ScheduleShowTW()
				gw.ForceTextWindowShown()
			}, true)
		}
		if gs.withEndpoints > 0 {
			menu.AddItem("Select Edge Chain", func() { gw.MenuEdit(cmdSelectChain) }, true)
		}
		if gs.constraints == 1 && gs.n == 0 {
			c := sk.Constraint(gs.constraint[0])
			if c.HasLabel() && c.Type != core.ConstraintComment {
				menu.AddItem("Toggle Reference Dimension", func() { ss.MenuConstrain(cmdReference) }, true)
			}
			if c.Type == core.ConstraintAngle || c.Type == core.ConstraintEqualAngle {
				menu.AddItem("Other Supplementary Angle", func() { ss.MenuConstrain(cmdOtherAngle) }, true)
			}
		}
		if gs.constraintLabels > 0 || gs.points > 0 {
			menu.AddItem("Snap to Grid", func() { gw.MenuEdit(cmdSnapToGrid) }, true)
		}

		if gs.points == 1 && gs.point[0].IsFromRequest() {
			r := sk.Request(gs.point[0].Request())
			index := r.IndexOfPoint(gs.point[0])
			if (r.Type == sketch.RequestCubic && index > 1 && index < r.ExtraPoints+2) ||
				r.Type == sketch.RequestCubicPeriodic {
				hp := gs.point[0]
				menu.AddItem("Remove Spline Point", func() {
					ss.editModel("Can't remove the spline point", func() error { return sk.RemoveSplinePoint(hp) })
				}, true)
			}
		}
		if gs.entities == 1 && gs.entity[0].IsFromRequest() {
			r := sk.Request(gs.entity[0].Request())
			if r.Type == sketch.RequestCubic || r.Type == sketch.RequestCubicPeriodic {
				e := sk.Entity(gs.entity[0])
				camera := gw.Camera()
				// The piece of the curve nearest the click; an open
				// spline's first point is its start's control point.
				after := gw.entityPositionOfPoint(e, &camera, geom.Point2d{U: x, V: y})
				if r.Type == sketch.RequestCubic {
					after++
				}
				he := gs.entity[0]
				menu.AddItem("Add Spline Point", func() {
					ss.applyEngineSettings()
					ss.editModel("Can't add a spline point", func() error { return sk.AddSplinePoint(he, after, v) })
					ss.afterGenerate()
				}, true)
			}
		}
		if gs.entities == gs.n {
			menu.AddItem("Toggle Construction", func() { gw.MenuSketch(cmdConstruction) }, true)
		}

		if gs.points == 1 && sk.PointIsCoincident(gs.point[0]) {
			hp := gs.point[0]
			menu.AddItem("Delete Point-Coincident Constraint", func() {
				ss.editModel("Can't delete the coincidence", func() error { return sk.DeletePointCoincidences(hp) })
			}, true)
		}
		menu.AddSeparator()
		if gw.LockedInWorkplane() {
			menu.AddItem("Cut", func() { gw.MenuClipboard(cmdCut) }, true)
			menu.AddItem("Copy", func() { gw.MenuClipboard(cmdCopy) }, true)
		}
	} else if !anyPicked {
		menu.AddItem("Select All", func() { gw.MenuEdit(cmdSelectAll) }, true)
	}
	gw.addModelItems(menu, &gs, itemsSelected)

	if (len(sk.Clipboard.Requests) > 0 || len(sk.Clipboard.Constraints) > 0) && gw.LockedInWorkplane() {
		menu.AddItem("Paste", func() { gw.MenuClipboard(cmdPaste) }, true)
		menu.AddItem("Paste Transformed...", func() { gw.MenuClipboard(cmdPasteTransform) }, true)
	}

	if itemsSelected {
		menu.AddItem("Delete", func() { gw.MenuClipboard(cmdDelete) }, true)
	}
	if anyPicked {
		menu.AddSeparator()
		menu.AddItem("Unselect All", func() { gw.MenuEdit(cmdUnselectAll) }, true)
	}
	// If only one item is selected, then it must be the one that we just
	// selected from the hovered item; in which case unselect all and
	// hovered are equivalent.
	if !gw.hover.IsEmpty() && len(gw.selection) > 1 {
		menu.AddItem("Unselect Hovered", func() {
			if !gw.hover.IsEmpty() {
				gw.MakeUnselected(&gw.hover, true)
			}
		}, true)
	}

	if anyPicked {
		menu.AddSeparator()
		menu.AddItem("Zoom to Fit", func() { gw.MenuView(cmdZoomToFit) }, true)
	}

	menu.PopUp()

	gw.context.active = false
	ss.ScheduleShowTW()
}

// editModel makes an edit of the model one undo step, and says why when
// it can't be made, with what before the reason.
func (ss *Application) editModel(what string, edit func() error) {
	if err := ss.SK.UndoRememberFor(edit); err != nil {
		ss.Error("%s: %s.", what, err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.GW.ClearSelection()
}

// addModelItems adds the entries for the solid's edges and faces, for a
// dimension, and for the group of the one thing selected; after says
// whether entries come before them, to be set apart.
func (gw *graphicsWindow) addModelItems(menu platform.Menu, gs *groupedSelection, after bool) {
	ss := gw.ss
	sk := gw.sk()
	edges := len(gs.solidEdge) > 0 || gw.selectionIsSolidEdges(gs)
	faces := gs.faces > 0 || len(gs.solidFace) > 0
	if (edges || faces) && after {
		menu.AddSeparator()
	}
	if edges {
		menu.AddItem("Fillet Edges", func() { ss.MenuFeatures(cmdGroupFillet, "") }, true)
		menu.AddItem("Chamfer Edges", func() { ss.MenuFeatures(cmdGroupChamfer, "") }, true)
		menu.AddItem("Sweep Along Edges", func() { ss.MenuFeatures(cmdGroupSweep, "") }, true)
	}
	if faces {
		menu.AddItem("Shell Solid Open Here", func() { ss.MenuFeatures(cmdGroupShell, "") }, true)
		if g := sk.Group(gw.activeGroupH()); g != nil && g.Type == sketch.GroupExtrude && (g.ValC != 0 || g.ExprC != "") {
			menu.AddItem("Draft Selected Sides", func() { ss.draftSelectedSides() }, true)
		}
		if gs.n == 1 && gs.faces == 1 && len(gs.solidFace) == 0 {
			menu.AddItem("Mirror in This Plane", func() { ss.MenuFeatures(cmdGroupMirror, "") }, true)
		}
		// One face of the solid, whether it was picked by its identity or
		// is the entity that drew it, may be sketched on and given an
		// appearance of its own, as a whole group's is on the group's
		// screen.
		face, oneFace := gs.onlyFace()
		menu.AddItem("Select Face Edges", func() { ss.SelectFaceEdges() }, true)
		if oneFace {
			menu.AddItem("Sketch On Face", func() { ss.MenuFeatures(cmdGroupSketchOnFace, "") }, true)
			if gw.LockedInWorkplane() {
				menu.AddItem("Wrap Sketch Onto This Face", func() { ss.MenuFeatures(cmdGroupWrap, "") }, true)
			}
		}
		if oneFace {
			appearance := menu.AddSubMenu("Appearance")
			appearance.AddItem("The group's own", func() { ss.SetFaceAppearance(face, "") }, true)
			for i, name := range render.MaterialNames {
				appearance.AddItem(name, func() { ss.SetFaceAppearance(face, render.MaterialNames[i]) }, true)
			}
		}
	}
	if ss.threadSelected() {
		menu.AddItem("Screw Thread...", func() { ss.MenuThread() }, true)
	}
	if edges || faces || gs.n > 0 {
		menu.AddItem("Measure Selection", func() { ss.MenuMeasure() }, true)
	}

	if gs.constraints == 1 && gs.n == 0 {
		hc := gs.constraint[0]
		if c := sk.Constraint(hc); c != nil && c.HasLabel() && !c.Reference && c.Type != core.ConstraintComment {
			menu.AddItem("Edit Value...", func() { gw.EditConstraint(hc) }, true)
		}
	}

	if hg, ok := gw.selectedGroup(gs); ok {
		gw.addGroupMenu(menu.AddSubMenu("Group"), hg)
	}
}

// SetFaceAppearance gives one face of the model an appearance of its
// own, as one step of undo, as View.SetFaceAppearance does for a script.
func (ss *Application) SetFaceAppearance(face srf.FaceID, name string) {
	ss.UndoRemember()
	if err := ss.SK.SetFaceAppearance(face, name); err != nil {
		ss.Report(err)
	}
	ss.GW.Invalidate(false)
}

// selectionIsSolidEdges reports whether the entities selected are all
// edges a solid's group made, as an extrusion's, so that the features
// that take edges apply to them; a sketch's own lines are not.
func (gw *graphicsWindow) selectionIsSolidEdges(gs *groupedSelection) bool {
	sk := gw.sk()
	if gs.entities == 0 || gs.points > 0 || gs.constraints > 0 {
		return false
	}
	for _, he := range gs.entity {
		e := sk.Entity(he)
		if e == nil || e.IsFace() || e.IsWorkplane() || e.IsNormal() {
			return false
		}
		g := sk.Group(e.Group)
		if g == nil || g.Type == sketch.GroupDrawingWorkplane || g.Type == sketch.GroupDrawing3D {
			return false
		}
	}
	return true
}

// selectedGroup is the group of the one entity, point, face or
// constraint selected.
func (gw *graphicsWindow) selectedGroup(gs *groupedSelection) (core.HGroup, bool) {
	sk := gw.sk()
	if gs.n+gs.constraints != 1 {
		return core.HGroup{}, false
	}
	var hg core.HGroup
	switch {
	case gs.entities == 1:
		hg = sk.Entity(gs.entity[0]).Group
	case gs.points == 1:
		hg = sk.Entity(gs.point[0]).Group
	case gs.constraints == 1:
		hg = sk.Constraint(gs.constraint[0]).Group
	}
	if hg.V == 0 || hg.V == core.HGroupReferences {
		return core.HGroup{}, false
	}
	return hg, true
}

// addGroupMenu adds what the group list does for one group: activate,
// show or hide, leave out of the model with what uses it, delete.
func (gw *graphicsWindow) addGroupMenu(m platform.Menu, hg core.HGroup) {
	ss := gw.ss
	sk := gw.sk()
	g := sk.Group(hg)
	if hg != gw.activeGroupH() {
		m.AddItem("Activate", func() {
			gw.ClearSelection()
			ss.TW.ScreenActivateGroup(0, hg.V)
		}, true)
	}
	show := "Hide"
	if !g.Visible {
		show = "Show"
	}
	m.AddItem(show, func() {
		gw.ClearSelection()
		ss.TW.ScreenToggleGroupShown(0, hg.V)
	}, true)
	suppress := "Suppress with What Uses It"
	if g.SuppressAll {
		suppress = "Bring Back"
	}
	m.AddItem(suppress, func() {
		gw.ClearSelection()
		ss.UndoRemember()
		if err := sk.SetSuppressAll(hg, !sk.Group(hg).SuppressAll); err != nil {
			ss.Report(err)
		}
		ss.GenerateAll(sketch.GenerateDirty)
	}, true)
	m.AddSeparator()
	m.AddItem("Delete Group", func() {
		gw.ClearSelection()
		ss.UndoRemember()
		ss.deleteGroup(hg)
	}, true)
}

package ui

import "github.com/telesapr/telesapr/pkg/sketch"

// selectionForEngine is the selection as the engine takes it.
func (gw *GraphicsWindow) selectionForEngine() []sketch.Selection {
	out := make([]sketch.Selection, 0, len(gw.selection))
	for _, s := range gw.selection {
		out = append(out, sketch.Selection{Entity: s.Entity, Constraint: s.Constraint})
	}
	return out
}

// afterEngineEdit updates what an engine edit that may have remembered
// an undo state leaves stale: the unsaved flag and the undo menus.
func (ss *Application) afterEngineEdit(undoBefore int) {
	if ss.SK.Undo.Write != undoBefore {
		ss.SK.Unsaved = true
	}
	ss.UndoEnableMenus()
}

// MakeTangentArc rounds the selected corner.
func (gw *GraphicsWindow) MakeTangentArc() {
	ss := gw.ss
	if !gw.LockedInWorkplane() {
		ss.Error("Must be sketching in workplane to create tangent arc.")
		return
	}
	hpt := gw.gs.point[0]
	gw.ClearSelection()
	undo := ss.SK.Undo.Write
	err := ss.SK.MakeTangentArc(hpt, sketch.TangentArcSettings{
		Manual: ss.TangentArcManual,
		Radius: ss.TangentArcRadius,
		Modify: ss.TangentArcModify,
	})
	ss.afterEngineEdit(undo)
	if err != nil {
		ss.Report(err)
	}
}

// SplitLinesOrCurves splits the selected curves where they cross,
// taking the crossing nearest the mouse.
func (gw *GraphicsWindow) SplitLinesOrCurves() {
	ss := gw.ss
	undo := ss.SK.Undo.Write
	err := ss.SK.SplitLinesOrCurves(gw.selectionForEngine(), gw.currentMousePosition)
	ss.afterEngineEdit(undo)
	if err != nil {
		ss.Report(err)
		return
	}
	// All done, clean up and regenerate.
	gw.ClearSelection()
}

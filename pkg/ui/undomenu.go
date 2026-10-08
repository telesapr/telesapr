package ui

import "github.com/telesapr/telesapr/pkg/core"

// UndoRemember records the state for undo.
func (ss *Application) UndoRemember() {
	ss.SK.Unsaved = true
	ss.SK.UndoRemember()
	ss.UndoEnableMenus()
}

// UndoUndo restores the last remembered state.
func (ss *Application) UndoUndo() {
	if !ss.SK.UndoUndo() {
		return
	}
	ss.afterUndoPop()
	ss.UndoEnableMenus()
}

// UndoRedo restores the last undone state.
func (ss *Application) UndoRedo() {
	if !ss.SK.UndoRedo() {
		return
	}
	ss.afterUndoPop()
	ss.UndoEnableMenus()
}

// afterUndoPop resets the rest of the program after the sketch changed
// a lot (the engine already reloaded the linked files and regenerated).
func (ss *Application) afterUndoPop() {
	ss.ViewUnits = ss.SK.Units
	ss.afterGenerate()
	ss.GW.ClearSuper()
	ss.TW.ClearSuper()
	ss.ScheduleShowTW()
	// Activate the group that was active before.
	if g := ss.SK.Group(ss.GW.activeGroupH()); g != nil {
		ss.groupActivate(g)
	}
}

// UndoEnableMenus greys out Undo and Redo when there's nothing to do.
func (ss *Application) UndoEnableMenus() {
	if ss.GW.undoMenuItem == nil {
		return
	}
	ss.GW.undoMenuItem.SetEnabled(ss.SK.Undo.Cnt > 0)
	ss.GW.redoMenuItem.SetEnabled(ss.SK.Redo.Cnt > 0)
}

// GroupsInOrder reports whether group before comes before group after.
func (ss *Application) GroupsInOrder(before, after core.HGroup) bool {
	return ss.SK.GroupsInOrder(before, after)
}

// addConstraint adds a constraint.
func (ss *Application) addConstraint(c *core.Constraint, rememberForUndo bool) core.HConstraint {
	if rememberForUndo {
		ss.UndoRemember()
	}
	return ss.SK.AddConstraintRemember(c, false)
}

// constrain adds a constraint in the active group and workplane.
func (ss *Application) constrain(typ core.ConstraintType, ptA, ptB, entityA core.HEntity) {
	ss.SK.Constrain(typ, ptA, ptB, entityA, core.HEntity{}, false, false)
}

// constrainCoincident makes two points coincident.
func (ss *Application) constrainCoincident(ptA, ptB core.HEntity) {
	ss.SK.ConstrainCoincident(ptA, ptB)
}

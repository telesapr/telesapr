package ui

import "github.com/telesapr/telesapr/pkg/core"

// UndoRemember records the state for undo (C++
// SolveSpaceUI::UndoRemember).
func (ss *SolveSpaceUI) UndoRemember() {
	ss.SK.Unsaved = true
	ss.SK.UndoRemember()
	ss.UndoEnableMenus()
}

// UndoUndo restores the last remembered state.
func (ss *SolveSpaceUI) UndoUndo() {
	if !ss.SK.UndoUndo() {
		return
	}
	ss.afterUndoPop()
	ss.UndoEnableMenus()
}

// UndoRedo restores the last undone state.
func (ss *SolveSpaceUI) UndoRedo() {
	if !ss.SK.UndoRedo() {
		return
	}
	ss.afterUndoPop()
	ss.UndoEnableMenus()
}

// afterUndoPop resets the rest of the program after the sketch changed
// a lot (the end of C++ PopOntoCurrentFrom; the engine already reloaded
// the linked files and regenerated).
func (ss *SolveSpaceUI) afterUndoPop() {
	ss.ViewUnits = ss.SK.Units
	ss.afterGenerate()
	ss.GW.ClearSuper()
	ss.TW.ClearSuper()
	ss.ScheduleShowTW()
	// Activate the group that was active before.
	if g := ss.SK.GetGroup(ss.GW.activeGroupH()); g != nil {
		ss.groupActivate(g)
	}
}

// UndoEnableMenus greys out Undo and Redo when there's nothing to do.
func (ss *SolveSpaceUI) UndoEnableMenus() {
	if ss.GW.undoMenuItem == nil {
		return
	}
	ss.GW.undoMenuItem.SetEnabled(ss.SK.Undo.Cnt > 0)
	ss.GW.redoMenuItem.SetEnabled(ss.SK.Redo.Cnt > 0)
}

// GroupsInOrder reports whether group before comes before group after
// (C++ SolveSpaceUI::GroupsInOrder).
func (ss *SolveSpaceUI) GroupsInOrder(before, after core.HGroup) bool {
	return ss.SK.GroupsInOrder(before, after)
}

// addConstraint adds a constraint (C++ Constraint::AddConstraint).
func (ss *SolveSpaceUI) addConstraint(c *core.Constraint, rememberForUndo bool) core.HConstraint {
	if rememberForUndo {
		ss.UndoRemember()
	}
	return ss.SK.AddConstraintRemember(c, false)
}

// constrain adds a constraint in the active group and workplane (C++
// Constraint::Constrain).
func (ss *SolveSpaceUI) constrain(typ core.ConstraintType, ptA, ptB, entityA core.HEntity) core.HConstraint {
	return ss.SK.Constrain(typ, ptA, ptB, entityA, core.HEntity{}, false, false)
}

// constrainFull is Constraint::Constrain with all of its arguments.
func (ss *SolveSpaceUI) constrainFull(typ core.ConstraintType, ptA, ptB, entityA, entityB core.HEntity,
	other, other2 bool) core.HConstraint {
	return ss.SK.Constrain(typ, ptA, ptB, entityA, entityB, other, other2)
}

// tryConstrain adds a constraint unless it would be redundant (C++
// Constraint::TryConstrain).
func (ss *SolveSpaceUI) tryConstrain(typ core.ConstraintType, ptA, ptB, entityA core.HEntity) core.HConstraint {
	return ss.SK.TryConstrain(typ, ptA, ptB, entityA, core.HEntity{}, false, false)
}

// constrainCoincident makes two points coincident.
func (ss *SolveSpaceUI) constrainCoincident(ptA, ptB core.HEntity) core.HConstraint {
	return ss.SK.ConstrainCoincident(ptA, ptB)
}

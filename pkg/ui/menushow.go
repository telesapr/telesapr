package ui

import (
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The View > Show menu holds what the icons at the top of the property
// browser show and hide, so that the menu, the command search and
// scripts reach them too. Both change the same fields of the window.

// showFlag is the field of the window a check item of View > Show
// toggles, or nil for a command that is not one.
func (gw *graphicsWindow) showFlag(cmd Command) *bool {
	switch cmd {
	case cmdShowWorkplanes:
		return &gw.show.Workplanes
	case cmdShowNormals:
		return &gw.show.Normals
	case cmdShowPoints:
		return &gw.show.Points
	case cmdShowConstruction:
		return &gw.show.Construction
	case cmdShowShaded:
		return &gw.showShaded
	case cmdShowFaces:
		return &gw.showFaces
	case cmdShowEdges:
		return &gw.show.Edges
	case cmdShowOutlines:
		return &gw.show.Outlines
	case cmdShowMesh:
		return &gw.showMesh
	}
	return nil
}

// showRadio is the mode a radio item of View > Show stands for, and
// whether cmd is one.
func showRadio(cmd Command) (constraints showConstraintMode, occluded DrawOccludedAs, isConstraints, ok bool) {
	switch cmd {
	case cmdShowConstraintsAll:
		return sCMShowAll, 0, true, true
	case cmdShowConstraintsDim:
		return sCMShowDim, 0, true, true
	case cmdShowConstraintsNone:
		return sCMNoShow, 0, true, true
	case cmdShowOccludedInvisible:
		return 0, occludedInvisible, false, true
	case cmdShowOccludedStippled:
		return 0, occludedStippled, false, true
	case cmdShowOccludedVisible:
		return 0, occludedVisible, false, true
	}
	return 0, 0, false, false
}

// isShowCommand reports whether cmd is an entry of View > Show.
func (gw *graphicsWindow) isShowCommand(cmd Command) bool {
	_, _, _, radio := showRadio(cmd)
	return radio || gw.showFlag(cmd) != nil
}

// MenuShow runs an entry of View > Show.
func (gw *graphicsWindow) MenuShow(cmd Command) {
	if v := gw.showFlag(cmd); v != nil {
		gw.ToggleBool(v)
		gw.refreshShowMenu()
		return
	}
	constraints, occluded, isConstraints, ok := showRadio(cmd)
	if !ok {
		return
	}
	if isConstraints {
		gw.showConstraints = constraints
	} else {
		gw.drawOccludedAs = occluded
	}
	gw.ss.GenerateAll(sketch.GenerateDirty)
	gw.Invalidate(false)
	gw.ss.ScheduleShowTW()
	gw.refreshShowMenu()
}

// refreshShowMenu sets the marks of View > Show from the window.
func (gw *graphicsWindow) refreshShowMenu() {
	for cmd, item := range gw.showMenuItems {
		item.SetActive(gw.showActive(cmd))
	}
}

// showActive reports whether the entry cmd of View > Show is marked.
func (gw *graphicsWindow) showActive(cmd Command) bool {
	if v := gw.showFlag(cmd); v != nil {
		return *v
	}
	constraints, occluded, isConstraints, _ := showRadio(cmd)
	if isConstraints {
		return gw.showConstraints == constraints
	}
	return gw.drawOccludedAs == occluded
}

// addShowMenuItem keeps an item of View > Show to mark it later.
func (gw *graphicsWindow) addShowMenuItem(cmd Command, item platform.MenuItem) {
	if gw.showMenuItems == nil {
		gw.showMenuItems = map[Command]platform.MenuItem{}
	}
	gw.showMenuItems[cmd] = item
	item.SetActive(gw.showActive(cmd))
}

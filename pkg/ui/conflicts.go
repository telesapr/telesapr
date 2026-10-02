package ui

import (
	"github.com/spacesolve/spacesolve/pkg/core"
)

// This file is an extension: C++ SolveSpace lists the constraints that
// keep a group from solving only in the text window. Here they are also
// drawn in their own colour, and the list can show each one.

// colorConflict is the colour of a constraint the active group's
// failed solve blames.
var colorConflict = rgbi(255, 60, 60)

// isConflicting reports whether constraint c is one the active group's
// failed solve blames: one of those that could be removed to fix it.
func (ss *SolveSpaceUI) isConflicting(c *core.Constraint) bool {
	if cppComparison {
		return false
	}
	g := ss.SK.GetGroup(ss.GW.activeGroupH())
	if g == nil || c.Group != g.H || g.IsSolvedOkay() {
		return false
	}
	for _, hc := range g.Solved.Remove {
		if hc == c.H {
			return true
		}
	}
	return false
}

// ScreenShowConstraint selects constraint v and zooms to it and what
// it constrains, from the list of a failed solve.
func (tw *TextWindow) ScreenShowConstraint(link int, v uint32) {
	ss := tw.ss
	gw := ss.GW
	sk := ss.SK
	c := sk.GetConstraint(core.HConstraint{V: v})
	if c == nil {
		return
	}
	gw.ClearSelection()
	gw.MakeSelectedConstraint(c.H)
	for _, he := range []core.HEntity{c.PtA, c.PtB, c.EntityA, c.EntityB, c.EntityC, c.EntityD} {
		if he.V != 0 && sk.GetEntity(he) != nil {
			gw.MakeSelectedEntity(he)
		}
	}
	gw.ZoomToFit(false, true)
	gw.ClearSelection()
	gw.MakeSelectedConstraint(c.H)
	gw.Invalidate(false)
	ss.ScheduleShowTW()
}

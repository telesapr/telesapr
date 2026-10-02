package ui

import (
	"strings"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// This file is an extension: C++ SolveSpace lists the constraints that
// keep a group from solving, and measures the selection, only in the
// text window. Here the constraints are also drawn in their own colour,
// and the list can show each one; and a measure mode labels the
// selection's measure on the canvas.

// colorConflict is the colour of a constraint the active group's
// failed solve blames.
var colorConflict = rgbi(255, 60, 60)

// isConflicting reports whether constraint c is one the active group's
// failed solve blames: one of those that could be removed to fix it.
func (ss *SolveSpaceUI) isConflicting(c *core.Constraint) bool {
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

// MenuMeasure turns the measure mode on or off: while it is on, the
// canvas labels what the selection measures, as the description of the
// selection does, without adding a constraint (an extension).
func (ss *SolveSpaceUI) MenuMeasure() {
	gw := ss.GW
	gw.measuring = !gw.measuring
	if gw.measureMenuItem != nil {
		gw.measureMenuItem.SetActive(gw.measuring)
	}
	gw.Invalidate(false)
}

// measureLabel is the label of what the selection measures, and where
// it goes; false when the mode is off or the selection measures
// nothing.
func (ss *SolveSpaceUI) measureLabel() (string, geom.Vector, bool) {
	gw := ss.GW
	if !gw.measuring {
		return "", geom.Vector{}, false
	}
	var hs []core.HEntity
	for _, s := range gw.selection {
		if s.Entity.V != 0 {
			hs = append(hs, s.Entity)
		}
	}
	if len(hs) == 0 {
		return "", geom.Vector{}, false
	}
	m, err := ss.SK.Measure(hs...)
	if err != nil {
		return "", geom.Vector{}, false
	}
	v := ss.MmToString(m.Value, false)
	if m.Kind == sketch.MeasureAngle {
		v = ss.DegreeToString(m.Value) + "°"
	}
	return m.What + " " + v, m.At, true
}

// SetDocumentUnits makes u the document's unit, which lengths are shown
// and typed in, keeping every size, as one undo step; new documents
// start in it too. When lengths are computed from parameters, the user
// is told that those now count in u.
func (ss *SolveSpaceUI) SetDocumentUnits(u core.Unit) {
	defer func() {
		ss.ViewUnits = ss.SK.Units
		ss.ScheduleShowTW()
		ss.GW.EnsureValidActives()
	}()
	ss.DefaultUnits = u
	if ss.SK.Units == u {
		return
	}
	var check []string
	_ = ss.SK.UndoRememberFor(func() error {
		check = ss.SK.SetUnits(u)
		return nil
	})
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	if len(check) > 0 {
		ss.ViewUnits = u
		ss.Message("The document's lengths are now in %s, and keep their sizes. "+
			"Lengths computed from these parameters now count them in %s; "+
			"check their values:\n\n    %s", ss.UnitName(), ss.UnitName(), strings.Join(check, ", "))
	}
}

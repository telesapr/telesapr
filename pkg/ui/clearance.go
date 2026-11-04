package ui

import (
	"fmt"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The clearance between parts and the thickness of walls (item 14 of
// doc/dev-notes/modelling_upgrades.md, Sketch.Clearances and
// Sketch.WallThickness). Its screen takes the gap and the wall
// thickness to look for less than, checks the model up to the active
// group, lists what it found, and marks each gap and thin wall on the
// model in the error colour (clearance_draw.go).

// clearanceCheck is the screen's state.
type clearanceCheck struct {
	gap, wall float64
	// bySolid has the gaps between the solids of a part of several, as
	// a linked assembly, checked too (Sketch.ClearancesWithin).
	bySolid    bool
	clearances []sketch.Clearance
	walls      *sketch.WallReport
	checked    bool
	err        string
}

// maxWallLines is how many thin spots are drawn: a thin wall is thin at
// each of its triangles.
const maxWallLines = 2000

// MenuClearance shows the screen to check the clearance between parts
// and the thickness of walls, looking for less than 1 mm of each.
func (ss *Application) MenuClearance() {
	tw := ss.TW
	c := &tw.clearance
	if c.gap == 0 {
		c.gap, c.wall = 1, 1
	}
	ss.GW.ClearSelection()
	tw.GoToScreen(screenClearance)
	ss.GW.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// ShowClearance is the screen of the clearance and wall check.
func (tw *textWindow) ShowClearance() {
	ss := tw.ss
	c := &tw.clearance
	tw.Printf(true, "%FtCLEARANCE AND WALLS%E")
	tw.Printf(false, "%Ba   %Ftgaps less than%E   %s %Fl%Lg%f[change]%E", ss.MmToString(c.gap, true), lf(tw.ScreenChangeClearance))
	tw.Printf(false, "%Bd   %Ftwalls thinner than%E %s %Fl%Lw%f[change]%E", ss.MmToString(c.wall, true), lf(tw.ScreenChangeClearance))
	// The box is at the end of the row, so that the rows below stay put.
	tw.Printf(true, "   %Fl%Lc%f[check gaps]%E   %Fl%Lt%f[check walls]%E   %Fl%Ls%f%s solids within parts%E",
		lf(tw.ScreenChangeClearance), lf(tw.ScreenChangeClearance), lf(tw.ScreenChangeClearance), checkMark(c.bySolid))
	if c.err != "" {
		tw.Printf(true, "   %Fx%s%E", c.err)
	}
	if c.checked {
		tw.showClearances()
	}
	if c.walls != nil {
		tw.showWalls()
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}

// showClearances lists the gaps found.
func (tw *textWindow) showClearances() {
	ss := tw.ss
	c := &tw.clearance
	if len(c.clearances) == 0 {
		tw.Printf(true, "   No two parts are nearer than %s.", ss.MmToString(c.gap, true))
		return
	}
	tw.Printf(true, "%Ft   gap            parts%E")
	for i, cl := range c.clearances {
		bg := byte('a')
		if i%2 == 1 {
			bg = 'd'
		}
		tw.Printf(false, "%Bp   %s %s and %s", bg, pad(ss.gapString(cl), 14), solidName(ss.SK, cl.A, cl.SolidA), solidName(ss.SK, cl.B, cl.SolidB))
	}
	tol := 0.0
	for _, cl := range c.clearances {
		tol = max(tol, cl.Tolerance)
	}
	tw.Printf(false, "   Gaps are within %s; parts that touch may overlap:", ss.MmToString(tol, true))
	tw.Printf(false, "   Show Interfering Parts tells.")
}

// showWalls says what the wall check found.
func (tw *textWindow) showWalls() {
	ss := tw.ss
	r := tw.clearance.walls
	if r.Samples == r.Unmeasured {
		tw.Printf(true, "   No wall could be measured.")
		return
	}
	tw.Printf(true, "   The thinnest wall is %s, in %s.", ss.MmToString(r.Thinnest.Thickness, true),
		ss.SK.GroupName(r.Thinnest.Group))
	if len(r.Thin) > 0 {
		tw.Printf(false, "   %Fx%d of %d points are thinner than %s.%E", len(r.Thin), r.Samples,
			ss.MmToString(tw.clearance.wall, true))
	} else {
		tw.Printf(false, "   No wall is thinner than %s.", ss.MmToString(tw.clearance.wall, true))
	}
	tw.Printf(false, "   Each is within %s, measured at points no more", ss.MmToString(r.Tolerance, true))
	tw.Printf(false, "   than %s apart, closer about the thinnest.", ss.MmToString(r.Step, true))
	if r.Unmeasured > 0 {
		tw.Printf(false, "   %d points could not be measured: the mesh is", r.Unmeasured)
		tw.Printf(false, "   open there, or parts stand in each other.")
	}
	if r.NearCut {
		tw.Printf(false, "   A part has more triangles facing each other within")
		tw.Printf(false, "   its thinnest wall than are measured: a thinner")
		tw.Printf(false, "   wall between two of those left may be missed.")
	}
}

// gapString is a gap as the list and the mark on the model give it.
func (ss *Application) gapString(cl sketch.Clearance) string {
	if cl.Gap > 0 {
		return ss.MmToString(cl.Gap, true)
	}
	return "touch"
}

// solidName is a part in the list of gaps: its group, and which of its
// solids where the gaps between them were checked.
func solidName(sk *sketch.Sketch, hg core.HGroup, solid int) string {
	if solid == 0 {
		return sk.GroupName(hg)
	}
	return fmt.Sprintf("%s solid %d", sk.GroupName(hg), solid)
}

// ScreenChangeClearance edits the gap ('g') or the wall thickness ('w')
// to look for, checks the gaps ('c') or the walls ('t'), or turns the
// gaps between the solids of a part on or off ('s').
func (tw *textWindow) ScreenChangeClearance(link int, v uint32) {
	ss := tw.ss
	c := &tw.clearance
	switch link {
	case 'g':
		tw.ShowEditControl(23, ss.MmToString(c.gap, false))
		tw.edit.meaning = editClearanceGap
	case 'w':
		tw.ShowEditControl(23, ss.MmToString(c.wall, false))
		tw.edit.meaning = editClearanceWall
	case 'c':
		ss.checkClearances()
	case 't':
		ss.checkWalls()
	case 's':
		c.bySolid = !c.bySolid
		ss.ScheduleShowTW()
	}
}

// checkClearances checks the gaps between the parts up to the active
// group, and marks each on the model (clearance_draw.go).
func (ss *Application) checkClearances() {
	c := &ss.TW.clearance
	sk := ss.SK
	sk.RemakeStaleMeshes()
	if c.bySolid {
		c.clearances = sk.ClearancesWithin(ss.GW.activeGroupH(), c.gap)
	} else {
		c.clearances = sk.Clearances(ss.GW.activeGroupH(), c.gap)
	}
	c.checked, c.walls, c.err = true, nil, ""
	ss.GW.nakedEdges.Clear()
	ss.GW.clearClearanceMarks()
	for _, cl := range c.clearances {
		ss.GW.gapMarks = append(ss.GW.gapMarks, gapMark{A: cl.PA, B: cl.PB, Label: ss.gapString(cl)})
	}
	ss.GW.Invalidate(false)
	ss.ScheduleShowTW()
}

// checkWalls measures the walls up to the active group, marks the
// thinnest with its thickness and draws a line across each thin spot
// (clearance_draw.go).
func (ss *Application) checkWalls() {
	c := &ss.TW.clearance
	sk := ss.SK
	sk.RemakeStaleMeshes()
	c.checked, c.walls, c.err = false, nil, ""
	ss.GW.nakedEdges.Clear()
	ss.GW.clearClearanceMarks()
	r, err := sk.WallThickness(ss.GW.activeGroupH(), c.wall)
	if err != nil {
		c.err = capitalize(err.Error()) + "."
	} else {
		c.walls = &r
		if r.Samples > r.Unmeasured {
			ss.GW.gapMarks = append(ss.GW.gapMarks, gapMark{A: r.Thinnest.From, B: r.Thinnest.To,
				Label: ss.MmToString(r.Thinnest.Thickness, true)})
		}
		for _, s := range r.Thin[:min(len(r.Thin), maxWallLines)] {
			ss.GW.thinSpots.AddEdge(s.From, s.To, 0, 0, 0)
		}
	}
	ss.GW.Invalidate(false)
	ss.ScheduleShowTW()
}

// editClearance takes an edit of the screen, and reports whether the
// edit was one of those.
func (tw *textWindow) editClearance(s string) bool {
	ss := tw.ss
	c := &tw.clearance
	if tw.edit.meaning != editClearanceGap && tw.edit.meaning != editClearanceWall {
		return false
	}
	e := ss.exprFromString(strings.TrimSpace(s))
	if e == nil {
		return true
	}
	v := ss.ExprToMm(e)
	if v <= 0 {
		ss.Error("The gap and the wall thickness to look for are more than zero.")
		return true
	}
	if tw.edit.meaning == editClearanceGap {
		c.gap = v
	} else {
		c.wall = v
	}
	ss.ScheduleShowTW()
	return true
}

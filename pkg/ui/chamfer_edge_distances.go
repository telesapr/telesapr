package ui

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// showEdgeDistances shows, under a chamfer's edge i, the distance and
// second distance of its own that it has, each with links to change it
// or give it the chamfer's again, and links to give it those it hasn't.
func (tw *textWindow) showEdgeDistances(g *sketch.Group, i int, bg byte) {
	val, expr, own := g.EdgeRadius(i)
	val2, expr2, own2 := g.EdgeSecond(i)
	if own {
		tw.Printf(false, "%Bp       %Ftdistance%E %s%s %Fl%Ll%f%D[change]%E %Fl%Ll%f%D[the chamfer's]%E", bg,
			tw.ss.MmToString(val, false), exprNote(expr), lf(tw.ScreenChangeEdgeDistance), uint32(i),
			lf(tw.ScreenResetEdgeDistance), uint32(i))
	}
	if own2 {
		tw.Printf(false, "%Bp       %Ftsecond distance%E %s%s %Fl%Ll%f%D[change]%E %Fl%Ll%f%D[the chamfer's]%E", bg,
			tw.ss.MmToString(val2, false), exprNote(expr2), lf(tw.ScreenChangeEdgeSecond), uint32(i),
			lf(tw.ScreenResetEdgeSecond), uint32(i))
	}
	switch {
	case !own && !own2:
		tw.Printf(false, "%Bp       %Fl%Ll%f%D[own distance]%E %Fl%Ll%f%D[own second distance]%E", bg,
			lf(tw.ScreenChangeEdgeDistance), uint32(i), lf(tw.ScreenChangeEdgeSecond), uint32(i))
	case !own:
		tw.Printf(false, "%Bp       %Fl%Ll%f%D[own distance]%E", bg, lf(tw.ScreenChangeEdgeDistance), uint32(i))
	case !own2:
		tw.Printf(false, "%Bp       %Fl%Ll%f%D[own second distance]%E", bg, lf(tw.ScreenChangeEdgeSecond), uint32(i))
	}
}

// ScreenChangeEdgeDistance edits the own distance of the shown
// chamfer's edge v.
func (tw *textWindow) ScreenChangeEdgeDistance(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)
	val, text, _ := g.EdgeRadius(int(v))
	tw.editEdgeDistance(g, int(v), val, text, editEdgeDistance)
}

// ScreenChangeEdgeSecond edits the own second distance of the shown
// chamfer's edge v. With none, the edit starts at the distance the
// edge's second face has now.
func (tw *textWindow) ScreenChangeEdgeSecond(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)
	val, text, _ := g.EdgeSecond(int(v))
	if val == 0 && text == "" {
		val, text, _ = g.EdgeRadius(int(v))
	}
	tw.editEdgeDistance(g, int(v), val, text, editEdgeSecond)
}

// editEdgeDistance opens the edit box for a chamfer's edge i's own
// distance, meaning which one, at val or its expression text.
func (tw *textWindow) editEdgeDistance(g *sketch.Group, i int, val float64, text string, meaning Edit) {
	if text == "" {
		text = fmt.Sprintf("%.8f", val/tw.ss.MmPerUnit())
	}
	tw.ShowEditControl(9, text)
	tw.edit.meaning = meaning
	tw.edit.group = g.H
	tw.edit.i = i
}

// ScreenResetEdgeDistance gives the shown chamfer's edge v the
// chamfer's distance again.
func (tw *textWindow) ScreenResetEdgeDistance(link int, v uint32) {
	tw.ss.setEdgeDistanceText(tw.shown.group, int(v), false, "")
}

// ScreenResetEdgeSecond gives the shown chamfer's edge v the chamfer's
// second distance again.
func (tw *textWindow) ScreenResetEdgeSecond(link int, v uint32) {
	tw.ss.setEdgeDistanceText(tw.shown.group, int(v), true, "")
}

// setEdgeDistanceText gives a chamfer's edge its own distance, or with
// second its own second distance, from typed text, or "" the
// chamfer's, as one step of undo.
func (ss *Application) setEdgeDistanceText(hg core.HGroup, i int, second bool, s string) {
	set := ss.SK.SetEdgeDistanceText
	if second {
		set = ss.SK.SetEdgeSecondText
	}
	err := ss.SK.UndoRememberFor(func() error { return set(hg, i, s) })
	if err != nil {
		ss.Report(err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.GW.Invalidate(false)
}

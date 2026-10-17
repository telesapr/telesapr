package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// buildGroupEditDoc is an extrusion and a rotate of it, made by script
// on a new application.
func buildGroupEditDoc(t *testing.T) (*Application, api.Group, api.Group) {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(4, -1, 0), api.V(6, 1, 0))
	ex := doc.Features.Extrude(2, "")
	rot := doc.Features.Rotate(2, doc.Origin(), doc.Workplane("XY").Normal(), 90, "")
	return ss, ex, rot
}

// The group screen's times-repeated edit and the script's SetCopies
// reach Sketch.SetGroupCopies: both set the count, the window re-spaces
// a rotate over one turn and the script keeps the angle it gave.
func TestGroupCopiesMatchScript(t *testing.T) {
	win, _, rot := buildGroupEditDoc(t)
	win.TW.edit.meaning, win.TW.edit.group = editTimesRepeated, rot.H
	win.TW.EditControlDone("5")

	scr, _, rot2 := buildGroupEditDoc(t)
	rot2.AsRepeat().SetCopies(5)

	angle := func(ss *Application, g api.Group) float64 { return ss.SK.MustParam(g.H.Param(3)).Val }
	if got, want := angle(win, rot), math.Pi/10; math.Abs(got-want) > 1e-12 {
		t.Errorf("the window's angle guess %g, want %g", got, want)
	}
	if got, want := angle(scr, rot2), math.Pi/8; math.Abs(got-want) > 1e-12 {
		t.Errorf("the script's angle guess %g, want the %g it gave", got, want)
	}
	for _, c := range []struct {
		ss *Application
		g  api.Group
	}{{win, rot}, {scr, rot2}} {
		if g := c.ss.SK.Group(c.g.H); g.ValA != 5 {
			t.Errorf("%g copies, want 5", g.ValA)
		}
	}
}

// The group screen's combine mode turns an unconstrained extrusion
// round, which a script's SetCombine does not; the mode is the same.
func TestGroupCombineWindowFlips(t *testing.T) {
	win, ex, _ := buildGroupEditDoc(t)
	g := win.SK.Group(ex.H)
	before := g.ExtrusionGetVector(win.SK).Z
	win.TW.shown.group = ex.H
	win.TW.ScreenChangeGroupOption('c', uint32(sketch.CombineDifference))
	if got := g.ExtrusionGetVector(win.SK).Z; got != -before || g.MeshCombine != sketch.CombineDifference {
		t.Errorf("vector z %g, mode %d; want %g and difference", got, g.MeshCombine, -before)
	}

	scr, ex2, _ := buildGroupEditDoc(t)
	ex2.SetCombine(sketch.CombineDifference)
	g2 := scr.SK.Group(ex2.H)
	if got := g2.ExtrusionGetVector(scr.SK).Z; got != before || g2.MeshCombine != sketch.CombineDifference {
		t.Errorf("script: vector z %g, mode %d; want %g and difference", got, g2.MeshCombine, before)
	}
}

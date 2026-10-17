package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// optionFields are the fields of a group the options set.
type optionFields struct {
	subtype    sketch.GroupSubtype
	skipFirst  bool
	color      core.RGBA
	combine    sketch.CombineAs
	suppress   bool
	appearance string
	copiesExpr string
	copies     float64
}

func fieldsOf(g *sketch.Group) optionFields {
	return optionFields{subtype: g.Subtype, skipFirst: g.SkipFirst, color: g.Color, combine: g.MeshCombine,
		suppress: g.Suppress, appearance: g.Appearance, copiesExpr: g.ExprA, copies: g.ValA}
}

// The group screen's skewed, start-with-copy-#1, colour, opacity and
// suppress choices and the script's methods for them reach the same
// Sketch entry points, so they leave the same group.
func TestGroupOptionsMatchScript(t *testing.T) {
	win, ex, rot := buildGroupEditDoc(t)
	win.TW.shown.group = ex.H
	win.TW.ScreenChangeGroupOption('w', 0)
	win.TW.ScreenChangeGroupOption('P', 0)
	win.TW.edit.meaning, win.TW.edit.group = editGroupColor, ex.H
	win.TW.EditControlDone("1, 0.2, 0")
	win.TW.edit.meaning, win.TW.edit.group = editGroupOpacity, ex.H
	win.TW.EditControlDone("0.25")
	win.TW.shown.group = rot.H
	win.TW.ScreenChangeGroupOption('k', 0)

	scr, ex2, rot2 := buildGroupEditDoc(t)
	ex2.AsExtrude().SetSkewed(true)
	ex2.SetSuppress(api.SuppressSolid)
	ex2.SetColor(255, 51, 0)
	ex2.SetOpacity(0.25)
	rot2.AsRepeat().SetSkipFirst(true)

	for _, c := range []struct {
		name     string
		win, scr *sketch.Group
	}{
		{"extrusion", win.SK.Group(ex.H), scr.SK.Group(ex2.H)},
		{"rotate", win.SK.Group(rot.H), scr.SK.Group(rot2.H)},
	} {
		if a, b := fieldsOf(c.win), fieldsOf(c.scr); a != b {
			t.Errorf("%s: the window left %+v, the script %+v", c.name, a, b)
		}
	}
	if g := win.SK.Group(ex.H); g.Subtype != sketch.GroupOneSkewed || !g.Suppress || g.Color.Alpha != 63 {
		t.Errorf("the extrusion: subtype %d, suppress %v, alpha %d", g.Subtype, g.Suppress, g.Color.Alpha)
	}
	if !win.SK.Group(rot.H).SkipFirst {
		t.Error("the rotate still starts with the original")
	}
}

// A refused edit of a group that has no such option is reported, and
// changes nothing.
func TestGroupOptionsRefused(t *testing.T) {
	ss, p := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(4, -1, 0), api.V(6, 1, 0))
	doc.Features.Extrude(2, "")
	rot := doc.Features.Rotate(2, doc.Origin(), doc.Workplane("XY").Normal(), 90, "")
	p.DialogResponses = append(p.DialogResponses, platform.ResponseOK, platform.ResponseOK)

	ss.TW.shown.group = rot.H
	before := fieldsOf(ss.SK.Group(rot.H))
	ss.TW.edit.meaning, ss.TW.edit.group = editGroupOpacity, rot.H
	ss.TW.EditControlDone("0.5")
	ss.TW.ScreenChangeGroupOption('w', 0)
	if after := fieldsOf(ss.SK.Group(rot.H)); after != before {
		t.Errorf("a refused edit changed the group: %+v to %+v", before, after)
	}
	if len(p.DialogsShown) != 2 {
		t.Errorf("%d refusals reported, want 2", len(p.DialogsShown))
	}
}

// The group screen offers the options of the kinds of group that have
// them and no others: a step and repeat shows no combine mode, colour
// or opacity, an extrusion all of them.
func TestGroupScreenOptionsByKind(t *testing.T) {
	ss, ex, rot := buildGroupEditDoc(t)
	tw := ss.TW
	show := func(h core.HGroup) {
		tw.shown.screen, tw.shown.group = screenGroupInfo, h
		tw.Show()
	}
	show(ex.H)
	for _, want := range []string{"solid model as", "appearance", "opacity", "suppress this group's solid model"} {
		if !shows(ss, want) {
			t.Errorf("the extrusion's screen doesn't show %q", want)
		}
	}
	show(rot.H)
	for _, no := range []string{"solid model as", "opacity"} {
		if shows(ss, no) {
			t.Errorf("the rotate's screen shows %q", no)
		}
	}
	for _, want := range []string{"appearance", "suppress this group's solid model"} {
		if !shows(ss, want) {
			t.Errorf("the rotate's screen doesn't show %q", want)
		}
	}
}

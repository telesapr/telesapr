package ui

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
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
	density    float64
}

func fieldsOf(g *sketch.Group) optionFields {
	return optionFields{subtype: g.Subtype, skipFirst: g.SkipFirst, color: g.Color, combine: g.MeshCombine,
		suppress: g.Suppress, appearance: g.Appearance, copiesExpr: g.ExprA, copies: g.ValA, density: g.Density}
}

// The group screen's skewed, start-with-copy-#1, colour, opacity,
// density and suppress choices and the script's methods for them reach the same
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
	win.TW.edit.meaning, win.TW.edit.group = editGroupDensity, ex.H
	win.TW.EditControlDone("7.85")
	win.TW.shown.group = rot.H
	win.TW.ScreenChangeGroupOption('k', 0)

	scr, ex2, rot2 := buildGroupEditDoc(t)
	ex2.AsExtrude().SetSkewed(true)
	ex2.SetSuppress(api.SuppressSolid)
	ex2.SetColor(255, 51, 0)
	ex2.SetOpacity(0.25)
	ex2.SetDensity(7.85)
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
	p.DialogResponses = append(p.DialogResponses, platform.ResponseOK, platform.ResponseOK, platform.ResponseOK)

	ss.TW.shown.group = rot.H
	before := fieldsOf(ss.SK.Group(rot.H))
	ss.TW.edit.meaning, ss.TW.edit.group = editGroupOpacity, rot.H
	ss.TW.EditControlDone("0.5")
	ss.TW.edit.meaning, ss.TW.edit.group = editGroupDensity, rot.H
	ss.TW.EditControlDone("2")
	ss.TW.ScreenChangeGroupOption('w', 0)
	if after := fieldsOf(ss.SK.Group(rot.H)); after != before {
		t.Errorf("a refused edit changed the group: %+v to %+v", before, after)
	}
	if len(p.DialogsShown) != 3 {
		t.Errorf("%d refusals reported, want 3", len(p.DialogsShown))
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
	for _, want := range []string{"solid model as", "appearance", "opacity", "density", "suppress this group's solid model"} {
		if !shows(ss, want) {
			t.Errorf("the extrusion's screen doesn't show %q", want)
		}
	}
	show(rot.H)
	for _, no := range []string{"solid model as", "opacity", "density"} {
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

// The density field takes an expression, clears with nothing, refuses
// a negative, and is one step of undo; Measure Volume then gives the
// mass.
func TestGroupScreenDensity(t *testing.T) {
	ss, ex, _ := buildGroupEditDoc(t)
	tw := ss.TW
	set := func(text string) {
		tw.shown.group = ex.H
		tw.ScreenDensity(0, 0)
		tw.EditControlDone(text)
	}
	set("2*4")
	if d := ss.SK.Group(ex.H).Density; d != 8 {
		t.Fatalf("density %g, want 8", d)
	}
	if msg := ss.massText(ss.GW.activeGroupH()); !strings.Contains(msg, "The mass of the model is") {
		t.Errorf("Measure Volume says nothing of the mass:%s", msg)
	}
	set("")
	if d := ss.SK.Group(ex.H).Density; d != 0 {
		t.Errorf("an empty field left %g", d)
	}
	if msg := ss.massText(ss.GW.activeGroupH()); msg != "" {
		t.Errorf("with no density set, Measure Volume says %q", msg)
	}
	ss.UndoUndo()
	if d := ss.SK.Group(ex.H).Density; d != 8 {
		t.Errorf("after undo %g, want 8", d)
	}
	p := ss.Platform.(*headless.Platform)
	p.DialogResponses = append(p.DialogResponses, platform.ResponseOK)
	set("-1")
	if d := ss.SK.Group(ex.H).Density; d != 8 || len(p.DialogsShown) != 1 {
		t.Errorf("a negative density: %g, %d dialogs", d, len(p.DialogsShown))
	}
}

// Measure Volume lists an assembly's parts with their masses, and says
// where a density is taken for want of one.
func TestMeasureVolumeParts(t *testing.T) {
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	doc.Features.Extrude(10, "base")
	doc.Features.SketchIn(doc.Workplane("XY"), "")
	doc.Sketch.Rect(api.V(20, 0, 0), api.V(30, 10, 0))
	block := doc.Features.Extrude(10, "block")
	block.SetCombine(api.Assemble)
	block.SetDensity(7.85)
	msg := ss.massText(block.H)
	for _, want := range []string{"The parts:", "7.85 g at 7.85 g/cm³", "base  1 g at 1 g/cm³", "no density is set"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Measure Volume doesn't say %q:%s", want, msg)
		}
	}
}

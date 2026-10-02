package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/headless"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// shows reports whether the text window shows s.
func shows(ss *SolveSpaceUI, s string) bool {
	tw := ss.TW
	for r := 0; r < tw.rows; r++ {
		if strings.Contains(string(tw.lines[r].text[:]), s) {
			return true
		}
	}
	return false
}

// The parameters screen is reached from the group list, and shows what
// could not be evaluated; its edits are undo steps, and a refused one
// is none.
func TestParametersScreen(t *testing.T) {
	asProduct(t)
	ss, p := newTestApp(t)
	tw := ss.TW
	w := tw.window.(*headless.Window)
	tw.Show()
	clickText(t, ss, "parameters")
	tw.Show()
	if tw.shown.screen != ScreenParameters {
		t.Fatalf("screen %d after the link", tw.shown.screen)
	}
	clickText(t, ss, "[add a parameter]")
	w.FinishEditing("a")
	ss.SK.Refresh()
	tw.Show()
	if p := ss.SK.Parameter("a"); p == nil || p.Expr != "0" {
		t.Fatalf("a name alone made %+v", p)
	}
	undo := ss.SK.Undo.Cnt
	clickText(t, ss, "[rename]")
	p.DialogResponses = append(p.DialogResponses, platform.ResponseOK)
	w.FinishEditing("9")
	if len(p.DialogsShown) == 0 || !strings.Contains(p.DialogsShown[len(p.DialogsShown)-1].Message, "can't name a parameter") {
		t.Errorf("dialogs %v", p.DialogsShown)
	}
	if ss.SK.Undo.Cnt != undo || ss.SK.Parameter("a") == nil {
		t.Fatal("a refused rename changed something")
	}

	// A table and an expression broken by hand, as a file can hold them.
	ss.SK.Parameters = append(ss.SK.Parameters, sketch.Parameter{Name: "b", Expr: "zz"})
	ss.SK.GetGroup(ss.SK.GroupOrder[1]).ExprA = "nope"
	ss.SK.GenerateAll()
	tw.Show()
	for _, want := range []string{`! unknown parameter "zz"`, "EXPRESSIONS THAT COULD NOT BE EVALUATED",
		`copies = nope: unknown parameter "nope"`, "in group sketch-in-plane"} {
		if !shows(ss, want) {
			t.Errorf("the screen doesn't show %q", want)
		}
	}
	clickText(t, ss, "in group sketch-in-plane")
	tw.Show()
	if tw.shown.screen != ScreenGroupInfo {
		t.Errorf("screen %d after the group link", tw.shown.screen)
	}
	ss.MenuParameters()
	tw.Show()
	clickText(t, ss, "[delete]")
	if ss.SK.Parameter("a") != nil {
		t.Error("delete left a")
	}
	ss.UndoUndo()
	if ss.SK.Parameter("a") == nil {
		t.Error("undo didn't bring a back")
	}
}

// The active group's entities that their constraints don't fix are
// drawn in their own colours, when the setting is on.
func TestColorByFreedom(t *testing.T) {
	asProduct(t)
	path := filepath.Join("..", "..", "testdata", "request", "line_segment", "normal.slvs")
	ss, _ := newTestApp(t)
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ss.LoadSketchForTest(doc.Sketch, path)
	draw := func() string {
		var b strings.Builder
		rc := newRecordingCanvas(&b)
		rc.camera = ss.GW.GetCamera()
		rc.SetBitmapFont(ss.GW.bitmapFont())
		ss.GW.Draw(rc)
		return b.String()
	}
	var line *core.Entity
	for i := range ss.SK.Entities {
		if e := &ss.SK.Entities[i]; e.Type == core.EntityLineSegment && e.Group == ss.GW.activeGroupH() {
			line = e
		}
	}
	if line == nil {
		t.Fatal("no line in the active group")
	}
	f := ss.SK.EntityFreedom(line.H)
	c, ok := ss.freedomColor(line)
	if f == sketch.FreedomFixed || !ok || (c != colorFree && c != colorPartlyFree) {
		t.Fatalf("line is %v, colour %v %v", f, c, ok)
	}
	ss.ColorByFreedom = true
	on := draw()
	ss.ColorByFreedom = false
	off := draw()
	if on == off {
		t.Error("the setting changes nothing drawn")
	}
	if _, ok := ss.freedomColor(line); ok {
		t.Error("a colour with the setting off")
	}
	ss.TW.ScreenChangeColorByFreedom(0, 0)
	if !ss.ColorByFreedom {
		t.Error("the configuration link didn't turn it on")
	}
}

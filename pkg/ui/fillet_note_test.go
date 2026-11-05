package ui

import (
	"testing"
)

// A fillet made at a size other than the one asked for, the nearest the
// kernel can cut (topo.FilletPicksMade), is marked "(note)" in the list
// of groups and says why on its screen.
func TestFilletNoteShown(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("a fillet round a rod through a plate; tens of seconds")
	}
	ss, _ := newTestApp(t)
	scriptConsole(t, ss,
		`doc.Edit.SetChordTolerance(0.003)`,
		`doc.Features.SketchIn(doc.Workplane("XY"), "")`,
		`doc.Sketch.Rect(V(-6, -15, 0), V(6, 15, 0))`,
		`doc.Features.Extrude(4, "plate")`,
		`doc.Features.Sketch3D("under")`,
		`doc.Features.SketchIn(doc.Sketch.NewWorkplane(V(0, 0, -6), V(1, 0, 0), V(0, 1, 0)), "")`,
		`doc.Sketch.Circle(V(0, 0, -6), 4)`,
		`rod := doc.Features.Extrude(16, "rod")`,
		`f := doc.Features.FilletSolid([]SolidEdge{rod.Solid().EdgeAt(V(4, 0, 4))}, 2.001, "round")`,
	)
	sk := ss.SK
	g := sk.Group(sk.Active)
	if g == nil || g.Name != "round" {
		t.Fatalf("the active group is %+v, want the fillet", g)
	}
	if g.Problem.Msg != "" || g.Note == "" {
		t.Fatalf("problem %q, notice %q: want the fillet made, with a notice", g.Problem.Msg, g.Note)
	}
	ss.TW.GoToScreen(screenListOfGroups)
	ss.TW.Show()
	if !shows(ss, "round (note)") {
		t.Error("the fillet's row in the list of groups isn't marked (note)")
	}
	ss.TW.shown.screen, ss.TW.shown.group = screenGroupInfo, g.H
	ss.TW.Show()
	if !shows(ss, "Note: "+g.Note) {
		t.Errorf("the fillet's screen doesn't say %q", g.Note)
	}
}

package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/script/api"
)

func buildDiameterDoc(t *testing.T) (*Application, api.Constraint) {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	circle := doc.Sketch.Circle(api.V(0, 0, 0), 5)
	return ss, doc.Constrain.Diameter(circle, 10)
}

// The text window's "show as radius" and the script's SetOther reach
// Sketch.SetConstraintShowAsRadius: the same file results, and the
// window's step is one undo that redo brings back.
func TestShowAsRadiusMatchesScript(t *testing.T) {
	win, wc := buildDiameterDoc(t)
	win.TW.ScreenConstraintShowAsRadius(0, wc.H.V)

	scr, sc := buildDiameterDoc(t)
	sc.SetOther(true)

	got := string(tlspr.Encode(win.SK, tlspr.Options{}))
	want := string(tlspr.Encode(scr.SK, tlspr.Options{}))
	if got != want {
		t.Errorf("the window and the script save different files, at %s", firstDiffLine(got, want))
	}
	if !win.SK.Constraint(wc.H).Other {
		t.Fatal("the window didn't show the diameter as a radius")
	}
	win.UndoUndo()
	if win.SK.Constraint(wc.H).Other {
		t.Errorf("undo left the radius")
	}
	win.UndoRedo()
	if !win.SK.Constraint(wc.H).Other {
		t.Errorf("redo lost the radius")
	}
}

// Dragging a dimension's label moves it by the mouse's travel, through
// the same sketch call a script's SetLabelOffset reaches.
func TestDragLabelUsesSketchCall(t *testing.T) {
	ss, c := buildDiameterDoc(t)
	before := c.LabelOffset()
	gw := ss.GW
	gw.pending.constraint = c.H
	gw.orig.mouse = geom.Point2d{U: 100, V: 100}
	gw.dragConstraint(130, 100)
	want := before.Plus(gw.view.ProjRight.ScaledBy(30 / gw.view.Scale))
	if got := ss.SK.Constraint(c.H).Disp.Offset; !got.Equals(want) {
		t.Errorf("the label is at %v after the drag, want %v", got, want)
	}
	if got := c.LabelOffset(); !got.Equals(want) {
		t.Errorf("the script reads %v, want %v", got, want)
	}
}

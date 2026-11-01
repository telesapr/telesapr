package ui

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// The drawing screen chooses which views the sheet shows and where the
// views, sections and details go, through the same Drawing calls a
// script reaches, each change one step of undo.
func TestDrawingScreenViews(t *testing.T) {
	t.Parallel()
	ss, doc := drawingModel(t)
	tw := ss.TW
	ss.MenuDrawingSheet()
	if ss.SK.Drawing != nil && len(ss.SK.Drawing.Views) != 0 {
		t.Fatalf("a new drawing has %d views placed", len(ss.SK.Drawing.Views))
	}
	// Choosing the views places the standard four, which can be moved.
	tw.ScreenChangeDrawing('V', 0)
	d := ss.SK.Drawing
	if len(d.Views) != 4 || d.ViewIndex("front") < 0 {
		t.Fatalf("the views chosen: %+v", d.Views)
	}
	// Hide the iso view and show the back one.
	tw.ScreenChangeDrawing('v', 6)
	tw.ScreenChangeDrawing('v', 1)
	d = ss.SK.Drawing
	if d.ViewIndex("iso") >= 0 || d.ViewIndex("back") < 0 {
		t.Errorf("after toggling: %+v", d.Views)
	}
	// Move the front view by typing where it goes.
	tw.ScreenChangeDrawing('M', 0)
	if !tw.editDrawing("111 222") {
		t.Fatal("the place edit was not taken")
	}
	if f := ss.SK.Drawing.Views[ss.SK.Drawing.ViewIndex("front")]; f.X != 111 || f.Y != 222 {
		t.Errorf("the front view is at %g, %g", f.X, f.Y)
	}
	// A section and a detail are placed by their numbers.
	tw.edit.meaning = editDrawingSection
	tw.editDrawing("top 0")
	tw.edit.meaning = editDrawingDetail
	tw.editDrawing("front 0 0 10 2")
	tw.ScreenChangeDrawing('y', 0)
	tw.editDrawing("300 100")
	tw.ScreenChangeDrawing('Y', 0)
	tw.editDrawing("300 50")
	d = ss.SK.Drawing
	if s := d.Sections[0]; s.X != 300 || s.Y != 100 {
		t.Errorf("the section is at %g, %g", s.X, s.Y)
	}
	if dt := d.Details[0]; dt.X != 300 || dt.Y != 50 {
		t.Errorf("the detail is at %g, %g", dt.X, dt.Y)
	}

	// A script reaches the same drawing, and what it sets is what the
	// screen shows.
	got := doc.Drawing.Get()
	doc.Drawing.Set(got)
	if again := doc.Drawing.Get(); len(again.Views) != len(got.Views) || again.Sections[0] != got.Sections[0] ||
		again.Details[0] != got.Details[0] {
		t.Errorf("the script's drawing differs: %+v, %+v", again, got)
	}

	// Toggling every view leaves at least one: the last cannot go.
	for k := range uint32(7) {
		tw.ScreenChangeDrawing('v', k)
	}
	if len(ss.SK.Drawing.Views) < 1 {
		t.Error("toggling every view left none")
	}
	tw.ScreenChangeDrawing('N', 0)
	if len(ss.SK.Drawing.Views) != 0 {
		t.Error("back to the standard views left views")
	}
}

// The drawing screen adds sections and details through the calls a
// script's Drawing.AddSection and AddDetail reach: one that would show
// nothing of the solid is refused, and an addition or a deletion is one
// step of undo that comes back on redo.
func TestDrawingScreenSections(t *testing.T) {
	t.Parallel()
	ss, doc := drawingModel(t)
	tw := ss.TW
	ss.MenuDrawingSheet()
	extras := func() (int, int) {
		if ss.SK.Drawing == nil {
			return 0, 0
		}
		return len(ss.SK.Drawing.Sections), len(ss.SK.Drawing.Details)
	}
	p := ss.Platform.(*headless.Platform)
	var said string
	p.OnMessageDialog = func(d *headless.MessageDialog) { said = d.Message }
	// The block is 40 deep seen from the front, 120 wide.
	for _, c := range []struct {
		meaning   Edit
		text, say string
	}{{editDrawingSection, "front 20", "within 20 mm"}, {editDrawingSection, "iso 0", "No section of"},
		{editDrawingDetail, "front 61 0 5 2", "within 60 mm"}, {editDrawingDetail, "front 0 0 -5 2", "radius and scale"}} {
		p.DialogResponses = []platform.Response{platform.ResponseOK}
		said = ""
		tw.edit.meaning = c.meaning
		tw.editDrawing(c.text)
		if s, d := extras(); s+d != 0 || !strings.Contains(said, c.say) {
			t.Fatalf("%q: %d sections and %d details, said %q", c.text, s, d, said)
		}
	}
	tw.ScreenChangeDrawing('a', 0)
	tw.editDrawing("front 10")
	tw.ScreenChangeDrawing('d', 0)
	tw.editDrawing("top -50 0 8 2")
	if s, d := extras(); s != 1 || d != 1 {
		t.Fatalf("%d sections and %d details", s, d)
	}
	ss.UndoUndo()
	if s, d := extras(); s != 1 || d != 0 {
		t.Errorf("after undo: %d sections and %d details", s, d)
	}
	ss.UndoRedo()
	if s, d := extras(); s != 1 || d != 1 {
		t.Errorf("after redo: %d sections and %d details", s, d)
	}
	// The script reads the same drawing.
	other := doc.Drawing.Get()
	if other.Sections[0] != ss.SK.Drawing.Sections[0] || other.Details[0] != ss.SK.Drawing.Details[0] {
		t.Errorf("the script reads %+v", other)
	}
	tw.ScreenChangeDrawing('x', 0)
	if s, d := extras(); s != 0 || d != 1 {
		t.Errorf("after the section's deletion: %d sections and %d details", s, d)
	}
	tw.ScreenChangeDrawing('X', 0)
	if s, d := extras(); s != 0 || d != 0 {
		t.Errorf("after the detail's deletion: %d sections and %d details", s, d)
	}
	ss.UndoUndo()
	if s, d := extras(); s != 0 || d != 1 {
		t.Errorf("after undoing the detail's deletion: %d sections and %d details", s, d)
	}
}

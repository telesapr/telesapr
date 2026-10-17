package ui

import "testing"

// The z-index edit of a style takes a whole number and refuses anything
// else, with a message and no undo step.
func TestStyleZIndexEdit(t *testing.T) {
	ss, _ := newTestApp(t)
	var msgs []string
	ss.messageSink = func(text string, _ bool) { msgs = append(msgs, text) }
	h := ss.SK.NewCustomStyle()
	ss.SK.Unsaved = false
	start := ss.SK.StyleGet(h).ZIndex
	tw := ss.TW

	for _, bad := range []string{"x", "4a", "", "1.5"} {
		tw.edit.meaning, tw.edit.style = editStyleZIndex, h
		if !tw.EditControlDoneForStyles(bad) {
			t.Fatalf("%q: edit not taken as a style's", bad)
		}
		if z := ss.SK.StyleGet(h).ZIndex; z != start || ss.SK.Unsaved {
			t.Errorf("%q: z-index %d, unsaved %v; want it left alone", bad, z, ss.SK.Unsaved)
		}
	}
	if len(msgs) != 4 {
		t.Errorf("%d messages for 4 bad values: %q", len(msgs), msgs)
	}

	tw.edit.meaning, tw.edit.style = editStyleZIndex, h
	tw.EditControlDoneForStyles(" 0 ")
	if z := ss.SK.StyleGet(h).ZIndex; z != 0 || !ss.SK.Unsaved {
		t.Errorf("z-index %d, unsaved %v after 0", z, ss.SK.Unsaved)
	}
}

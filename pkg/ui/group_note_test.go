package ui

import "testing"

// A group whose solid was made, but not quite as asked, shows its note
// on its screen; one with a problem shows the problem instead.
func TestGroupScreenNote(t *testing.T) {
	t.Parallel()
	ss, ex, _ := buildGroupEditDoc(t)
	tw := ss.TW
	g := ss.SK.Group(ex.H)
	g.Note = "the wall under a flat face stops up to 0.04 short"
	tw.shown.screen, tw.shown.group = screenGroupInfo, ex.H
	tw.Show()
	if !shows(ss, "Note: the wall under a flat face stops") {
		t.Error("the group's screen doesn't show its note")
	}
	g.Problem.Msg = "no solid"
	tw.Show()
	if shows(ss, "Note:") {
		t.Error("the group's screen shows a note beside a problem")
	}
}

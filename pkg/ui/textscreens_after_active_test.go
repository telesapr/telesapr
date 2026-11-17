package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// TestWindowGroupsAfterActiveLeftAsBefore changes a group before
// others in the window: the regeneration goes only as far as the active
// group, so the group after it still holds what it was made as (the
// slot made a cut, then a union). The list of groups says "old" for it,
// not its degrees of freedom then, and its screen says it was not made
// since and offers to make it; the link makes it as it now is, the
// active group left as it was.
func TestWindowGroupsAfterActiveLeftAsBefore(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	box := func(x0, y0, x1, y1 float64, name string) api.Group {
		doc.Features.SketchAt(doc.Origin(), api.V(1, 0, 0), api.V(0, 1, 0), name+"-outline")
		doc.Sketch.Rect(api.V(x0, y0, 0), api.V(x1, y1, 0))
		return doc.Features.Extrude(10, name)
	}
	block := box(0, 0, 20, 20, "block")
	hole := box(2, 2, 6, 6, "hole")
	hole.SetCombine(api.Difference)
	// Made through the hole, so that only the slot's change leaves it old.
	if v := hole.Analyze().Volume(); math.Abs(v-(4000-160)) > 1e-6 {
		t.Fatalf("the hole reads %g mm³, want %d", v, 4000-160)
	}
	doc.Activate(block)
	slot := box(18, 10, 24, 14, "slot")
	slot.SetCombine(api.Difference)
	// A group added before others makes them all; a change after that
	// goes only as far as the active group.
	ss.ScriptHost().Regenerate()
	slot.SetCombine(api.Union)
	ss.ScriptHost().Regenerate()

	sk := ss.SK
	for _, g := range []api.Group{block, slot} {
		if sk.LeftAsBefore(g.H) {
			t.Errorf("%s, at or before the active group, is left as before", g.Name())
		}
	}
	if !sk.LeftAsBefore(hole.H) {
		t.Fatal("the hole after the changed slot is not left as before")
	}

	tw := ss.TW
	screen := func(s Screen) string {
		tw.shown.screen = s
		tw.Show()
		var b strings.Builder
		tw.dumpRows(&b)
		return b.String()
	}
	rowOf := func(text, name string) string {
		for line := range strings.SplitSeq(text, "\n") {
			if strings.HasPrefix(line, "row ") && strings.Contains(line, name) {
				return line
			}
		}
		return ""
	}
	list := screen(screenListOfGroups)
	if row := rowOf(list, "-hole"); !strings.Contains(row, " old ") {
		t.Errorf("the hole's row in the list says no \"old\": %q", row)
	}
	if row := rowOf(list, "-block"); strings.Contains(row, " old ") {
		t.Errorf("the block's row in the list says \"old\": %q", row)
	}
	tw.shown.group = hole.H
	info := screen(screenGroupInfo)
	if !strings.Contains(info, "Not made since the last change") || !strings.Contains(info, "make it now") {
		t.Fatalf("the hole's screen says nothing of being old:\n%s", info)
	}

	tw.ScreenMakeGroupNow(0, hole.H.V)
	ss.settleShells()
	if sk.LeftAsBefore(hole.H) {
		t.Fatal("make it now left the hole as before")
	}
	if a := doc.Active(); a.H != slot.H {
		t.Errorf("make it now activated %s", a.Name())
	}
	if sk.MakeThrough(hole.H) {
		t.Error("a read after make it now made the groups again")
	}
	// 80 mm³ of the slot is in the block, 160 out.
	if v := hole.Analyze().Volume(); math.Abs(v-(4000-160+160)) > 1e-6 {
		t.Errorf("the hole made now reads %g mm³, want %d", v, 4000-160+160)
	}
	if info := screen(screenGroupInfo); strings.Contains(info, "Not made since") {
		t.Error("the hole's screen still says it is old")
	}
}

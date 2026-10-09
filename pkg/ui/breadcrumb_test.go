package ui

import (
	"strings"
	"testing"
)

func TestBreadcrumb(t *testing.T) {
	ss, _ := newTestApp(t)
	tw := ss.TW
	g := ss.SK.Group(ss.GW.activeGroupH())
	for _, c := range []struct {
		screen Screen
		want   string
	}{
		{screenListOfGroups, ""},
		{screenGroupInfo, " › " + groupDescription(g)},
		{screenGroupSolveInfo, " › " + groupDescription(g) + " › solve"},
		{screenParameters, " › parameters"},
		{screenConfiguration, " › preferences"},
	} {
		tw.shown.screen, tw.shown.group = c.screen, g.H
		if got := tw.breadcrumb(); got != c.want {
			t.Errorf("screen %d: %q, want %q", c.screen, got, c.want)
		}
	}
	// Every screen but the list of groups says where it is.
	for s := screenConfiguration; s <= screenDrawing; s++ {
		tw.shown.screen = s
		if !strings.HasPrefix(tw.breadcrumb(), " › ") {
			t.Errorf("screen %d has no breadcrumb", s)
		}
	}

	// A group's actions say it is active, or offer to make it so.
	format, _ := tw.groupActions(g)
	if !strings.Contains(format, "active%E") || strings.Contains(format, "activate") {
		t.Errorf("the active group's actions: %q", format)
	}
	other := ss.SK.Group(ss.SK.GroupOrder[0])
	if format, args := tw.groupActions(other); !strings.Contains(format, "activate") || len(args) != 9 {
		t.Errorf("another group's actions: %q, %d args", format, len(args))
	}
}

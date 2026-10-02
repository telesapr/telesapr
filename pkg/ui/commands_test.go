package ui

import (
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/platform/headless"
	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func TestFindCommands(t *testing.T) {
	labels := CommandLabels()
	if len(labels) < 100 || labels[0] != "File > New" {
		t.Fatalf("%d labels, first %q", len(labels), labels[0])
	}
	for _, l := range labels {
		if strings.Contains(l, "&") || strings.Contains(l, "Open Recent") || strings.Contains(l, "Link Recent") {
			t.Errorf("label %q", l)
		}
	}
	if got := api.Commands(); len(got) != len(labels) {
		t.Errorf("api.Commands has %d, want %d", len(got), len(labels))
	}
	names := func(q string) string {
		var out []string
		for _, e := range findCommands(q) {
			out = append(out, e.label)
		}
		return strings.Join(out, "; ")
	}
	if got := names("zoom fit"); got != "View > Zoom To Fit" {
		t.Errorf("zoom fit: %s", got)
	}
	if got := names("EXTRUDE"); got != "New Group > Extrude" {
		t.Errorf("EXTRUDE: %s", got)
	}
	// An entry whose name starts with the words comes first.
	if got := names("sel"); !strings.HasPrefix(got, "Edit > Select Edge Chain; Edit > Select All") {
		t.Errorf("sel: %s", got)
	}
	if got := names("  "); got != "" {
		t.Errorf("blank: %s", got)
	}
	if got := names("no such thing"); got != "" {
		t.Errorf("nothing: %s", got)
	}
}

func TestRunCommand(t *testing.T) {
	ss, _ := newTestApp(t)
	if err := ss.RunCommand("view > dimension units > dimensions in inches"); err != nil {
		t.Fatal(err)
	}
	if ss.MmPerUnit() != 25.4 {
		t.Errorf("units %v", ss.MmPerUnit())
	}
	if err := ss.RunCommand("Dimensions in Millimeters"); err != nil {
		t.Fatal(err)
	}
	if ss.MmPerUnit() != 1 {
		t.Errorf("units %v", ss.MmPerUnit())
	}
	if err := ss.RunCommand("Frobnicate"); err == nil || !strings.Contains(err.Error(), "no command") {
		t.Errorf("unknown: %v", err)
	}
	// A name two entries share is ambiguous.
	saved := menuTable
	t.Cleanup(func() { menuTable = saved })
	menuTable = append(append([]menuEntry(nil), saved...),
		menuEntry{0, "&Extra", CmdNone, 0, kn, nil},
		menuEntry{1, "Zoom To &Fit", CmdZoomToFit, 0, kn, mView})
	if err := ss.RunCommand("zoom to fit"); err == nil || !strings.Contains(err.Error(), "View > Zoom To Fit or Extra > Zoom To Fit") {
		t.Errorf("ambiguous: %v", err)
	}
	if err := ss.RunCommand("Extra > Zoom To Fit"); err != nil {
		t.Error(err)
	}
}

// The search screen: Enter lists, Enter again runs the first; a single
// match runs at once.
func TestCommandSearchScreen(t *testing.T) {
	ss, _ := newTestApp(t)
	tw := ss.TW
	w := tw.window.(*headless.Window)
	ss.MenuFindCommand()
	if tw.shown.screen != ScreenCommands || !w.EditorVisible {
		t.Fatalf("screen %d, editor %v", tw.shown.screen, w.EditorVisible)
	}
	w.FinishEditing("feet")
	tw.Show()
	if ss.MmPerUnit() != 25.4 || tw.shown.screen == ScreenCommands {
		t.Fatalf("a single match didn't run: units %v, screen %d", ss.MmPerUnit(), tw.shown.screen)
	}

	ss.MenuFindCommand()
	w.FinishEditing("dimensions in")
	tw.Show()
	if !w.EditorVisible || !shows(ss, "View > Dimension Units > Dimensions in Millimeters") || !shows(ss, "(Enter)") {
		t.Fatal("the matches are not listed with the editor open")
	}
	w.FinishEditing("dimensions in")
	tw.Show()
	if ss.MmPerUnit() != 1 {
		t.Errorf("the second Enter didn't run the first match: units %v", ss.MmPerUnit())
	}

	ss.MenuFindCommand()
	w.FinishEditing("qqq")
	tw.Show()
	if !shows(ss, "No command matches 'qqq'.") {
		t.Error("no message for no match")
	}
	w.FinishEditing("dimensions in")
	tw.Show()
	// The first click closes the editor, as everywhere in the window.
	clickText(t, ss, "View > Dimension Units > Dimensions in Inches")
	clickText(t, ss, "View > Dimension Units > Dimensions in Inches")
	if ss.MmPerUnit() != 25.4 {
		t.Errorf("a click didn't run it: units %v", ss.MmPerUnit())
	}
	ss.MenuFindCommand()
	tw.Show()
	if !shows(ss, "Enter again runs the first.") {
		t.Error("no help on an empty search")
	}
}

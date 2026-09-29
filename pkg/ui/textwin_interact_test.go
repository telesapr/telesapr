package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/headless"
)

// clickText clicks the first place the text window shows s, as the
// mouse would; it fails the test if s isn't shown.
func clickText(t *testing.T, ss *SolveSpaceUI, s string) {
	t.Helper()
	tw := ss.TW
	for r := 0; r < tw.rows; r++ {
		row := string(tw.lines[r].text[:])
		i := strings.Index(row, s)
		if i < 0 {
			continue
		}
		col := len([]rune(row[:i]))
		x := float64(leftMargin + col*charWidth + charWidth/2)
		y := float64((tw.lines[r].top-tw.scrollPos)*(lineHeight/2) + lineHeight/2)
		w := tw.window.(*headless.Window)
		w.Mouse(platform.MouseEvent{Type: platform.MouseMotion, X: x, Y: y})
		w.Mouse(platform.MouseEvent{Type: platform.MousePress, X: x, Y: y, Button: platform.ButtonLeft})
		return
	}
	t.Fatalf("text window doesn't show %q", s)
}

// TestTextWindowLinks follows links in the property browser, and edits
// a group's name through the edit control.
func TestTextWindowLinks(t *testing.T) {
	ss, _ := newTestApp(t)
	if !ss.Load(filepath.Join("..", "..", "testdata", "group", "translate_nd", "normal.slvs")) {
		t.Fatal("load failed")
	}
	tw := ss.TW
	w := tw.window.(*headless.Window)
	if w.Title != "Property Browser" {
		t.Errorf("text window title %q", w.Title)
	}

	clickText(t, ss, "g002-sketch-in-plane")
	if tw.shown.screen != ScreenGroupInfo || tw.shown.group.V != 2 {
		t.Fatalf("after clicking the group: screen %d group %v", tw.shown.screen, tw.shown.group)
	}

	clickText(t, ss, "rename")
	if !w.EditorVisible || w.EditorText != "sketch-in-plane" {
		t.Fatalf("editor visible %v with %q", w.EditorVisible, w.EditorText)
	}
	w.FinishEditing("renamed")
	if name := ss.SK.GetGroup(tw.shown.group).Name; name != "renamed" {
		t.Errorf("group name %q", name)
	}
	if w.EditorVisible {
		t.Error("editor still visible")
	}
	ss.TW.Show()

	clickText(t, ss, "home")
	tw.Show()
	if tw.shown.screen != ScreenListOfGroups {
		t.Fatalf("screen %d after home", tw.shown.screen)
	}
	clickText(t, ss, "g002-renamed")

	// The configuration takes a new chord tolerance.
	clickText(t, ss, "home")
	tw.Show()
	clickText(t, ss, "configuration")
	clickText(t, ss, "[change]; ")
	chordTol := ss.ChordTol
	w.FinishEditing("0.25")
	if ss.ChordTol != 0.25 {
		t.Errorf("chord tolerance %v, was %v", ss.ChordTol, chordTol)
	}

	// Rendering doesn't fail.
	w.Width, w.Height = 400, 500
	w.Render()
	if w.Frame == nil {
		t.Fatal("no frame")
	}
}

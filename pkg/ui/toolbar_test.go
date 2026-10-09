package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// The toolbar is laid out from its table: SolveSpace's two columns when
// they fit below the menu bar, three when the window is too short.
func TestToolbarLayout(t *testing.T) {
	rows2, spacers := toolbarRows(2)
	rows3, _ := toolbarRows(3)
	if spacers != 4 || rows3 >= rows2 {
		t.Errorf("rows %d in two columns, %d in three, %d spacers", rows2, rows3, spacers)
	}
	if got := toolbarHeight(2); got != rows2*toolbarPitch+spacers*toolbarSpacer+toolbarFudge {
		t.Errorf("height %d", got)
	}
	tall := toolbarHeight(2) + toolbarTop
	if toolbarColumns(tall) != 2 || toolbarColumns(tall-1) != 3 {
		t.Errorf("columns at %d: %d, at %d: %d", tall, toolbarColumns(tall), tall-1, toolbarColumns(tall-1))
	}
	if toolbarWidth(2) != 66 {
		t.Errorf("two columns are %d wide, SolveSpace's 66", toolbarWidth(2))
	}
	for _, icon := range toolbar {
		if icon.name != "" && icon.tooltip == "" {
			t.Errorf("button %s has no tooltip", icon.name)
		}
	}
}

// Every button is hit at its place, in either layout, and runs its own
// command.
func TestToolbarHitTest(t *testing.T) {
	ss, _ := newTestApp(t)
	gw := ss.GW
	win := gw.window.(*headless.Window)
	for _, height := range []int{toolbarHeight(2) + toolbarTop + 40, toolbarHeight(3) + toolbarTop} {
		win.Height = float64(height)
		cols := toolbarColumns(height)
		seen := map[Command]bool{}
		for y := 0; y < height; y += 4 {
			for x := 0; x < toolbarWidth(cols); x += 4 {
				if _, cmd, _, _ := gw.toolbarDrawOrHitTest(x, y, nil); cmd != cmdNone {
					seen[cmd] = true
				}
			}
		}
		for _, icon := range toolbar {
			if icon.name != "" && !seen[icon.command] {
				t.Errorf("button %s is never hit in a window %d high (%d columns)", icon.name, height, cols)
			}
		}
	}
}

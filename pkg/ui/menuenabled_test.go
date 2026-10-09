package ui

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
)

func TestMenuEnablement(t *testing.T) {
	ss, p := newTestApp(t)
	gw := ss.GW
	enabled := func(cmd Command) bool {
		t.Helper()
		it, ok := gw.enablementItems[cmd]
		if !ok {
			t.Fatalf("no menu item for command %d", cmd)
		}
		return it.(*headless.MenuItem).Enabled
	}
	for _, cmd := range enablementCommands {
		if enabled(cmd) {
			t.Errorf("command %d enabled in a new file: %s", cmd, gw.commandUnavailable(cmd))
		}
	}

	// Ctrl+V with nothing to paste does nothing: no message.
	gw.KeyboardEvent(platform.KeyboardEvent{Type: platform.KeyPress, Key: platform.KeyCharacter,
		Chr: 'v', ControlDown: true})
	if len(p.DialogsShown) != 0 {
		t.Errorf("Ctrl+V on an empty clipboard: %d dialogs", len(p.DialogsShown))
	}
	if err := ss.RunCommand("Edit > Paste"); err == nil || !strings.Contains(err.Error(), "clipboard is empty") {
		t.Errorf("RunCommand(Paste) = %v", err)
	}

	gw.selection = append(gw.selection, Selection{Entity: ss.SK.Entities[0].H})
	gw.Invalidate(false)
	for _, cmd := range []Command{cmdCopy, cmdCut, cmdDelete, cmdSnapToGrid} {
		if !enabled(cmd) {
			t.Errorf("command %d not enabled with a selection", cmd)
		}
	}
	if enabled(cmdPaste) {
		t.Error("Paste enabled with an empty clipboard")
	}
	gw.ClearSelection()
	if enabled(cmdCopy) {
		t.Error("Copy still enabled after the selection was cleared")
	}

	// The command search puts what can't run after what can, and says why.
	found := ss.findCommands("select")
	last := found[len(found)-1]
	if gw.commandUnavailable(last.cmd) == "" || gw.commandUnavailable(found[0].cmd) != "" {
		t.Errorf("search order: first %q, last %q", found[0].label, last.label)
	}
}

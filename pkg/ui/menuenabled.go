package ui

import "github.com/telesapr/telesapr/pkg/platform"

// An entry of the menu that has nothing to work on is greyed out, its
// key does nothing and the command search says why. Only what the state
// of the window decides is greyed: an entry that needs a particular
// selection, as Fillet Edges does, stays enabled, since the message it
// gives on a wrong one says what to select.

// commandUnavailable says why cmd can't run now, or "" when it can.
func (gw *graphicsWindow) commandUnavailable(cmd Command) string {
	ss := gw.ss
	switch cmd {
	case cmdCut, cmdCopy, cmdDelete, cmdSelectChain, cmdSnapToGrid:
		if len(gw.selection) == 0 {
			return "nothing is selected"
		}
	case cmdPaste, cmdPasteTransform:
		if len(ss.SK.Clipboard.Requests) == 0 && len(ss.SK.Clipboard.Constraints) == 0 {
			return "the clipboard is empty"
		}
	case cmdStopTracing:
		if gw.traced.Point.V == 0 {
			return "no point is traced"
		}
	case cmdScriptStop:
		if ss.console == nil || !ss.console.Busy() {
			return "no script is running"
		}
	}
	return ""
}

// enablementCommands are the commands commandUnavailable can refuse.
var enablementCommands = []Command{
	cmdCut, cmdCopy, cmdDelete, cmdSelectChain, cmdSnapToGrid,
	cmdPaste, cmdPasteTransform, cmdStopTracing, cmdScriptStop,
}

// refreshEnabled greys out the entries that can't run, and enables
// those that can, telling the platform only of a change.
func (gw *graphicsWindow) refreshEnabled() {
	for _, cmd := range enablementCommands {
		item, ok := gw.enablementItems[cmd]
		if !ok {
			continue
		}
		enabled := gw.commandUnavailable(cmd) == ""
		if was, known := gw.enabledState[cmd]; known && was == enabled {
			continue
		}
		if gw.enabledState == nil {
			gw.enabledState = map[Command]bool{}
		}
		gw.enabledState[cmd] = enabled
		item.SetEnabled(enabled)
	}
}

// addEnablementItem keeps a menu item whose command can be refused.
func (gw *graphicsWindow) addEnablementItem(cmd Command, item platform.MenuItem) {
	for _, c := range enablementCommands {
		if c == cmd {
			if gw.enablementItems == nil {
				gw.enablementItems = map[Command]platform.MenuItem{}
			}
			gw.enablementItems[cmd] = item
			delete(gw.enabledState, cmd)
			return
		}
	}
}

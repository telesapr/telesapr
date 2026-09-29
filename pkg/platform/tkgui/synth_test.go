package tkgui

import (
	"testing"

	"github.com/msorc/takigo/event"
	tkplatform "github.com/msorc/takigo/platform"

	"github.com/spacesolve/spacesolve/pkg/platform"
)

func synthButton(button uint, x, y int) *event.Event {
	return &event.Event{Type: event.ButtonPressType, Button: button, X: x, Y: y}
}

func synthKey(sym tkplatform.KeySym, state uint) *event.Event {
	return &event.Event{Type: event.KeyPressType, KeySym: sym, State: state}
}

// TestShiftedSymbolAccelerators checks that a key whose symbol Shift
// made is offered to the application as GTK's accelerators would take
// it: the symbol alone, and for the digit row the digit with Shift.
func TestShiftedSymbolAccelerators(t *testing.T) {
	w := &Window{}
	var seen []platform.KeyboardEvent
	// Zoom In takes '+', Show Snap Grid '>', Sketch In 3d Shift+3.
	w.events.OnKeyboardEvent = func(e platform.KeyboardEvent) bool {
		seen = append(seen, e)
		switch {
		case e.Chr == '+' || e.Chr == '>':
			return !e.ShiftDown
		case e.Chr == '3':
			return e.ShiftDown
		}
		return false
	}
	press := func(sym tkplatform.KeySym, keycode uint) bool {
		return w.key(platform.KeyPress, &event.Event{Type: event.KeyPressType, KeySym: sym,
			State: tkplatform.ShiftMask, KeyCode: keycode})
	}
	if !press('+', 21) || len(seen) != 2 || seen[1].Chr != '+' || seen[1].ShiftDown {
		t.Errorf("Shift+= : %+v", seen)
	}
	seen = nil
	if !press('>', 60) || len(seen) != 2 || seen[1].Chr != '>' || seen[1].ShiftDown {
		t.Errorf("Shift+. : %+v", seen)
	}
	seen = nil
	if !press('#', 12) || len(seen) != 3 || seen[2].Chr != '3' || !seen[2].ShiftDown {
		t.Errorf("Shift+3 : %+v", seen)
	}
	seen = nil
	// A letter with Shift is just that letter with Shift.
	if press('A', 38) || len(seen) != 1 || seen[0].Chr != 'a' || !seen[0].ShiftDown {
		t.Errorf("Shift+a : %+v", seen)
	}
}

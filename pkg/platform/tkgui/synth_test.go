package tkgui

import (
	"github.com/msorc/takigo/event"
	tkplatform "github.com/msorc/takigo/platform"
)

func synthButton(button uint, x, y int) *event.Event {
	return &event.Event{Type: event.ButtonPressType, Button: button, X: x, Y: y}
}

func synthKey(sym tkplatform.KeySym, state uint) *event.Event {
	return &event.Event{Type: event.KeyPressType, KeySym: sym, State: state}
}

package web

import (
	"math"

	"github.com/telesapr/telesapr/pkg/platform"
)

// The GTK backend's double-click time and distance, which tkgui also
// uses.
const (
	doubleClickMs       = 400
	doubleClickDistance = 5
)

// clickTracker turns a second quick press of a button near the first
// into a double press. The browser has its own dblclick, but it comes
// after the press and release and would leave the press to run twice.
type clickTracker struct {
	button platform.MouseButton
	at     float64 // milliseconds
	x, y   float64
}

// press returns MouseDblPress for the second of two quick clicks and
// MousePress otherwise; a third click is a press again.
func (c *clickTracker) press(ms float64, button platform.MouseButton, x, y float64) platform.MouseEventType {
	if c.button == button && button != platform.ButtonNone && ms-c.at < doubleClickMs &&
		math.Abs(x-c.x) <= doubleClickDistance && math.Abs(y-c.y) <= doubleClickDistance {
		*c = clickTracker{}
		return platform.MouseDblPress
	}
	*c = clickTracker{button: button, at: ms, x: x, y: y}
	return platform.MousePress
}

// A KeyboardEvent.deltaMode.
const (
	wheelPixels = iota
	wheelLines
	wheelPages
)

// wheelDelta is a wheel turn as the application reads it: one for each
// notch, positive up, and fractions for a smooth wheel. A notch of a
// mouse is 100 pixels in Chrome, three lines in Firefox.
func wheelDelta(deltaY float64, mode int) float64 {
	switch mode {
	case wheelLines:
		return -deltaY / 3
	case wheelPages:
		return -deltaY
	}
	return -deltaY / 100
}

// buttonFromBrowser is a platform button from PointerEvent.button (0
// left, 1 middle, 2 right).
func buttonFromBrowser(b int) platform.MouseButton {
	switch b {
	case 0:
		return platform.ButtonLeft
	case 1:
		return platform.ButtonMiddle
	case 2:
		return platform.ButtonRight
	}
	return platform.ButtonNone
}

// buttonFromMask is the button a motion event reports, from
// PointerEvent.buttons (1 left, 2 right, 4 middle), the left one first
// as the other backends choose.
func buttonFromMask(mask int) platform.MouseButton {
	switch {
	case mask&1 != 0:
		return platform.ButtonLeft
	case mask&4 != 0:
		return platform.ButtonMiddle
	case mask&2 != 0:
		return platform.ButtonRight
	}
	return platform.ButtonNone
}

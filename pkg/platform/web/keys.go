package web

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/telesapr/telesapr/pkg/platform"
)

// namedKeys are the browser's names (KeyboardEvent.key) of the keys that
// the platform reports as characters.
var namedKeys = map[string]rune{
	"Backspace": '\b',
	"Tab":       '\t',
	"Enter":     '\r',
	"Escape":    '\x1b',
	"Delete":    '\x7f',
	" ":         ' ',
}

var arrowNames = map[string]int{
	"ArrowLeft":  platform.ArrowLeft,
	"ArrowRight": platform.ArrowRight,
	"ArrowUp":    platform.ArrowUp,
	"ArrowDown":  platform.ArrowDown,
}

// keyCandidates maps a browser key event to the events the application
// is offered, in order, until it takes one; nil for a key that is not
// the application's (modifiers, Home, a key with Alt held). The first is
// the key as it is: a character lowercase with Shift reported
// separately, a function key, an arrow. As the GTK backend does for the
// symbols a shifted key makes, a character that is not a letter or
// digit is offered also without Shift ('+' on Shift+'=' is Zoom In's
// key), and, when it is on the top row of digits, as that digit with
// Shift (Shift+'3' is '#' on one layout, '£' on another). Alt is left to
// the browser, which also reports AltGr as Control and Alt together,
// but for Ctrl+Alt with a key the browser keeps Ctrl's own of
// (browserCtrlKey): that is offered as Ctrl and the key, the way to
// reach New, Paste Transformed, Show Naked Edges and Trace Point in a
// page. Command on a Mac counts as Control. The events' Type is KeyPress.
func keyCandidates(key, code string, shift, ctrl, alt, meta bool) []platform.KeyboardEvent {
	ctrl = ctrl || meta
	if alt && (!ctrl || !browserCtrlKey(key)) {
		return nil
	}
	k := platform.KeyboardEvent{Type: platform.KeyPress, ShiftDown: shift, ControlDown: ctrl}
	if chr, ok := namedKeys[key]; ok {
		k.Key, k.Chr = platform.KeyCharacter, chr
		return []platform.KeyboardEvent{k}
	}
	if n, ok := arrowNames[key]; ok {
		k.Key, k.Num = platform.KeyArrow, n
		return []platform.KeyboardEvent{k}
	}
	if rest, ok := strings.CutPrefix(key, "F"); ok && len(key) > 1 {
		n, err := strconv.Atoi(rest)
		if err != nil || n < 1 || n > 12 {
			return nil
		}
		k.Key, k.Num = platform.KeyFunction, n
		return []platform.KeyboardEvent{k}
	}
	chr, size := utf8.DecodeRuneInString(key)
	if chr == utf8.RuneError || size != len(key) {
		return nil
	}
	k.Key, k.Chr = platform.KeyCharacter, unicode.ToLower(chr)
	out := []platform.KeyboardEvent{k}
	if shift && !unicode.IsLetter(chr) && !unicode.IsDigit(chr) && chr > ' ' {
		unshifted := k
		unshifted.ShiftDown = false
		out = append(out, unshifted)
		if digit, ok := digitRowKey(code); ok {
			asDigit := k
			asDigit.Chr = digit
			out = append(out, asDigit)
		}
	}
	return out
}

// browserCtrlKey reports whether a browser keeps Ctrl and the key for
// itself, with or without Shift, so that a page never sees it: a new
// window, a new tab, closing the tab (Ctrl+N, Ctrl+T, Ctrl+W). The
// application's accelerators on them are reached with Ctrl+Alt instead,
// and with Ctrl alone while the page is full screen, where web.js asks
// the browser for those keys (navigator.keyboard.lock).
func browserCtrlKey(key string) bool {
	switch strings.ToLower(key) {
	case "n", "t", "w":
		return true
	}
	return false
}

// digitRowKey is the digit of a key of the keyboard's top row by its
// physical name (KeyboardEvent.code), whatever the layout's symbols.
func digitRowKey(code string) (rune, bool) {
	d, ok := strings.CutPrefix(code, "Digit")
	if !ok || len(d) != 1 || d[0] < '0' || d[0] > '9' {
		return 0, false
	}
	return rune(d[0]), true
}

// browserKey reports whether a key is one of the browser's own that is left to it:
// reload, full screen and the developer tools.
func browserKey(key string, shift, ctrl bool) bool {
	switch key {
	case "F5", "F11", "F12":
		return true
	}
	if ctrl && shift {
		switch strings.ToLower(key) {
		case "i", "j", "c", "r", "delete":
			return true
		}
	}
	return false
}

// takesKey reports whether the page should keep the browser from acting
// on a key event, because the application takes it: whether it uses the
// key is known only when its goroutine gets to it, so this is every key
// that maps to an event, but those the browser keeps for itself.
func takesKey(key, code string, shift, ctrl, alt, meta bool) bool {
	if browserKey(key, shift, ctrl || meta) {
		return false
	}
	return len(keyCandidates(key, code, shift, ctrl, alt, meta)) > 0
}

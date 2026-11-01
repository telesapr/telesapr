package web

import (
	"sync"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
)

func TestQueueOrderAndNestedLoop(t *testing.T) {
	t.Parallel()
	q := newEventQueue()
	var got []string
	finished := false
	q.post(func() {
		got = append(got, "outer")
		// A modal dialog: the nested loop runs what is queued until
		// the answer comes.
		q.post(func() { got = append(got, "in dialog") })
		q.post(func() { finished = true })
		q.run(func() bool { return finished })
		got = append(got, "after dialog")
	})
	q.post(func() { got = append(got, "later") })
	quit := false
	q.post(func() { quit = true })
	if !q.run(func() bool { return quit }) {
		t.Fatal("run reported a closed queue")
	}
	// The nested loop takes the queue in order: what was posted before
	// the dialog opened runs while it is open.
	want := []string{"outer", "later", "in dialog", "after dialog"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestQueueFromOtherGoroutines(t *testing.T) {
	t.Parallel()
	q := newEventQueue()
	const n = 200
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { q.post(func() {}) })
	}
	count := 0
	done := make(chan struct{})
	go func() { wg.Wait(); q.post(func() { close(done) }) }()
	q.post(func() { count++ })
	stop := false
	go func() { <-done; q.post(func() { stop = true }) }()
	q.run(func() bool { return stop })
	if count != 1 {
		t.Errorf("count %d", count)
	}
}

func TestQueueCloseEndsLoopsAndDropsPosts(t *testing.T) {
	t.Parallel()
	q := newEventQueue()
	ran := false
	go func() { q.close() }()
	if q.run(func() bool { return false }) {
		t.Error("run returned true on a closed queue")
	}
	q.post(func() { ran = true })
	if q.pending() != 0 || ran {
		t.Error("a function was kept after close")
	}
}

func TestQueueSurvivesPanic(t *testing.T) {
	t.Parallel()
	q := newEventQueue()
	after := false
	q.post(func() { panic("bug in a handler") })
	q.post(func() { after = true })
	q.run(func() bool { return after })
	if !after {
		t.Error("the loop stopped at a panic")
	}
}

func TestKeyCandidates(t *testing.T) {
	t.Parallel()
	ch := func(r rune, shift, ctrl bool) platform.KeyboardEvent {
		return platform.KeyboardEvent{Key: platform.KeyCharacter, Chr: r, ShiftDown: shift, ControlDown: ctrl}
	}
	tests := []struct {
		name                   string
		key, code              string
		shift, ctrl, alt, meta bool
		want                   []platform.KeyboardEvent
	}{
		{name: "letter", key: "a", code: "KeyA", want: []platform.KeyboardEvent{ch('a', false, false)}},
		{name: "upper case letter is lower with shift", key: "S", code: "KeyS", shift: true, ctrl: true,
			want: []platform.KeyboardEvent{ch('s', true, true)}},
		{name: "enter", key: "Enter", want: []platform.KeyboardEvent{ch('\r', false, false)}},
		{name: "escape", key: "Escape", want: []platform.KeyboardEvent{ch('\x1b', false, false)}},
		{name: "space", key: " ", code: "Space", want: []platform.KeyboardEvent{ch(' ', false, false)}},
		{name: "delete", key: "Delete", want: []platform.KeyboardEvent{ch('\x7f', false, false)}},
		{name: "arrow", key: "ArrowUp", want: []platform.KeyboardEvent{{Key: platform.KeyArrow, Num: platform.ArrowUp}}},
		{name: "function", key: "F10", want: []platform.KeyboardEvent{{Key: platform.KeyFunction, Num: 10}}},
		{name: "shift plus offers the plain key", key: "+", code: "Equal", shift: true,
			want: []platform.KeyboardEvent{ch('+', true, false), ch('+', false, false)}},
		{name: "digit row", key: "#", code: "Digit3", shift: true,
			want: []platform.KeyboardEvent{ch('#', true, false), ch('#', false, false), ch('3', true, false)}},
		{name: "shifted digit is not repeated", key: "3", code: "Digit3", shift: true,
			want: []platform.KeyboardEvent{ch('3', true, false)}},
		{name: "alt is the browser's", key: "f", alt: true},
		{name: "AltGr", key: "@", ctrl: true, alt: true},
		{name: "modifier", key: "Shift", shift: true},
		{name: "home", key: "Home"},
		{name: "F13", key: "F13"},
		{name: "meta counts as control", key: "z", code: "KeyZ", meta: true, want: []platform.KeyboardEvent{ch('z', false, true)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := keyCandidates(tc.key, tc.code, tc.shift, tc.ctrl, tc.alt, tc.meta)
			for i := range tc.want {
				tc.want[i].Type = platform.KeyPress
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("candidate %d: got %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestTakesKey(t *testing.T) {
	t.Parallel()
	if !takesKey("s", "KeyS", false, true, false, false) {
		t.Error("Ctrl+S should be the application's")
	}
	for _, k := range []string{"F5", "F11", "F12"} {
		if takesKey(k, "", false, false, false, false) {
			t.Errorf("%s should be left to the browser", k)
		}
	}
	if takesKey("I", "KeyI", true, true, false, false) {
		t.Error("Ctrl+Shift+I opens the developer tools")
	}
	if takesKey("f", "KeyF", false, false, true, false) {
		t.Error("Alt+F is the browser's")
	}
	if !takesKey("z", "KeyZ", false, false, false, true) {
		t.Error("Cmd+Z is Ctrl+Z on a Mac")
	}
	if takesKey("Shift", "ShiftLeft", true, false, false, false) {
		t.Error("a modifier is nobody's")
	}
}

func TestClickTracker(t *testing.T) {
	t.Parallel()
	var c clickTracker
	steps := []struct {
		ms     float64
		b      platform.MouseButton
		x, y   float64
		want   platform.MouseEventType
		reason string
	}{
		{0, platform.ButtonLeft, 10, 10, platform.MousePress, "first"},
		{200, platform.ButtonLeft, 12, 11, platform.MouseDblPress, "quick and near"},
		{300, platform.ButtonLeft, 12, 11, platform.MousePress, "a third click is a press"},
		{900, platform.ButtonLeft, 12, 11, platform.MousePress, "too slow"},
		{1000, platform.ButtonLeft, 40, 11, platform.MousePress, "too far"},
		{1100, platform.ButtonRight, 40, 11, platform.MousePress, "another button"},
		{1200, platform.ButtonRight, 40, 11, platform.MouseDblPress, "right double"},
	}
	for _, s := range steps {
		if got := c.press(s.ms, s.b, s.x, s.y); got != s.want {
			t.Errorf("%s: got %v, want %v", s.reason, got, s.want)
		}
	}
}

func TestWheelDelta(t *testing.T) {
	t.Parallel()
	if got := wheelDelta(100, wheelPixels); got != -1 {
		t.Errorf("a notch down is %v", got)
	}
	if got := wheelDelta(-3, wheelLines); got != 1 {
		t.Errorf("three lines up is %v", got)
	}
	if got := wheelDelta(-10, wheelPixels); got != 0.1 {
		t.Errorf("a trackpad's 10 pixels up is %v", got)
	}
	if got := wheelDelta(1, wheelPages); got != -1 {
		t.Errorf("a page down is %v", got)
	}
}

func TestButtons(t *testing.T) {
	t.Parallel()
	if buttonFromBrowser(2) != platform.ButtonRight || buttonFromBrowser(1) != platform.ButtonMiddle ||
		buttonFromBrowser(0) != platform.ButtonLeft || buttonFromBrowser(7) != platform.ButtonNone {
		t.Error("buttonFromBrowser")
	}
	if buttonFromMask(3) != platform.ButtonLeft || buttonFromMask(6) != platform.ButtonMiddle ||
		buttonFromMask(2) != platform.ButtonRight || buttonFromMask(0) != platform.ButtonNone {
		t.Error("buttonFromMask")
	}
}

type mapStore map[string]string

func (m mapStore) get(k string) (string, bool) { v, ok := m[k]; return v, ok }
func (m mapStore) set(k, v string)             { m[k] = v }

func TestSettings(t *testing.T) {
	t.Parallel()
	m := mapStore{}
	s := kvSettings{m}
	if s.Has("A") || s.ThawInt("A", 7) != 7 || s.ThawFloat("A", 1.5) != 1.5 || s.ThawString("A", "d") != "d" {
		t.Error("defaults")
	}
	s.FreezeInt("A", 4000000000)
	s.FreezeFloat("F", 0.1)
	s.FreezeString("S", "x y")
	if !s.Has("A") || s.ThawInt("A", 0) != 4000000000 || s.ThawFloat("F", 0) != 0.1 || s.ThawString("S", "") != "x y" {
		t.Errorf("round trip: %v", m)
	}
	if m["telesapr.A"] != "4000000000" {
		t.Errorf("stored under %v", m)
	}
	if s.ThawInt("S", 9) != 9 || s.ThawFloat("S", 2) != 2 {
		t.Error("a string is not a number")
	}
}

func TestScrollArithmetic(t *testing.T) {
	t.Parallel()
	const track, lo, hi, page = 300.0, 0.0, 100.0, 25.0
	spacer := scrollSpacer(track, lo, hi, page)
	if spacer != 1200 {
		t.Fatalf("spacer %v", spacer)
	}
	// The greatest scrollTop is spacer-track, which is the value hi-page.
	if got := valueForScrollTop(spacer-track, lo, hi, spacer); got != hi-page {
		t.Errorf("end of the box is %v", got)
	}
	for _, v := range []float64{0, 13, 75} {
		if got := valueForScrollTop(scrollTopFor(v, lo, hi, spacer), lo, hi, spacer); got != v {
			t.Errorf("value %v came back %v", v, got)
		}
	}
	if scrollSpacer(track, 0, 10, 25) != track {
		t.Error("a range that fits has nothing to scroll")
	}
	if clampScroll(90, lo, hi, page) != 75 || clampScroll(-5, lo, hi, page) != 0 {
		t.Error("clampScroll")
	}
}

func TestFileHelpers(t *testing.T) {
	t.Parallel()
	filters := []platform.FileFilter{{Name: "TeleSAPR", Extensions: []string{"tlspr"}}, {Name: "STEP", Extensions: []string{"step", "stp"}}}
	if got := acceptAttribute(filters); got != ".tlspr,.step,.stp" {
		t.Errorf("accept %q", got)
	}
	if got := acceptAttribute(append(filters, platform.FileFilter{Name: "All", Extensions: []string{""}})); got != "" {
		t.Errorf("any file: %q", got)
	}
	if withExtension("part", "tlspr") != "part.tlspr" || withExtension("part.step", "tlspr") != "part.step" ||
		withExtension("part", "") != "part" {
		t.Error("withExtension")
	}
	if stem("/a/b/part.v2.tlspr") != "part.v2" || stem("noext") != "noext" || stem("/a/.hidden") != ".hidden" {
		t.Error("stem")
	}
	if firstExtension(filters, 1) != "step" || firstExtension(filters, 9) != "tlspr" || firstExtension(nil, 0) != "" {
		t.Error("firstExtension")
	}
	if folderOrHome("/work/a.tlspr") != "/work" || folderOrHome("a.tlspr") != homeDir {
		t.Error("folderOrHome")
	}
}

func TestTextOffsets(t *testing.T) {
	t.Parallel()
	s := "aé\U0001F600b" // a, e-acute (2 bytes), emoji (4 bytes, 2 units), b
	if got := utf16ToByteOffset(s, 4); got != 7 {
		t.Errorf("4 units is byte %d", got)
	}
	if got := byteToUTF16Offset(s, 7); got != 4 {
		t.Errorf("byte 7 is unit %d", got)
	}
	for _, u := range []int{0, 1, 2, 4, 5} {
		if got := byteToUTF16Offset(s, utf16ToByteOffset(s, u)); got != u {
			t.Errorf("unit %d came back %d", u, got)
		}
	}
	if commonPrefix("print", "printf") != "print" || commonPrefix("aé", "aè") != "a" || commonPrefix("x", "y") != "" {
		t.Error("commonPrefix")
	}
}

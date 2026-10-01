package tkgui

import (
	"slices"
	"testing"

	"github.com/msorc/takigo/event"
	tkplatform "github.com/msorc/takigo/platform"

	"github.com/spacesolve/spacesolve/pkg/platform"
)

// TestConsoleKeys types in a console: Return enters the box's text,
// Shift+Return starts another line in it, Up and Down bring back what
// was entered, Escape interrupts, and nothing is entered while busy.
func TestConsoleKeys(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	w.SetVisible(true)
	c := p.CreateConsole(w).(*Console)
	var submitted []string
	interrupts := 0
	c.Events().OnSubmit = func(line string) { submitted = append(submitted, line) }
	c.Events().OnInterrupt = func() { interrupts++ }
	c.SetPrompt(">>> ")
	c.Print(platform.ConsoleInfo, "hello\n")
	c.SetVisible(true)

	key := func(sym tkplatform.KeySym, state uint) {
		p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType,
			Window: c.input.Window().PlatformID, KeySym: sym, State: state})
	}
	var recalled []string
	run(p,
		func() { c.Focus() },
		func() {
			c.setTyped("a := 1")
			key(tkplatform.XK_Return, 0)
		},
		func() {
			// Shift+Return is a new line in the box, Return enters both.
			c.setTyped("if a > 0 {")
			key(tkplatform.XK_Return, tkplatform.ShiftMask)
			c.input.Insert(c.input.EndIndex(), "}")
			key(tkplatform.XK_Return, 0)
		},
		func() {
			c.setTyped("draft")
			key(tkplatform.XK_Up, 0)
			recalled = append(recalled, c.typed())
			key(tkplatform.XK_Up, 0)
			recalled = append(recalled, c.typed())
			key(tkplatform.XK_Up, 0)
			recalled = append(recalled, c.typed())
			key(tkplatform.XK_Down, 0)
			key(tkplatform.XK_Down, 0)
			recalled = append(recalled, c.typed())
		},
		func() {
			c.SetBusy(true)
			key(tkplatform.XK_Return, 0)
			key(tkplatform.XK_Escape, 0)
			c.SetBusy(false)
			c.Print(platform.ConsoleValue, "42\n")
		},
	)
	if want := []string{"a := 1", "if a > 0 {\n}"}; !slices.Equal(submitted, want) {
		t.Errorf("submitted %q, want %q", submitted, want)
	}
	// The second Up moves to the first of the two lines; the third
	// recalls from there.
	if want := []string{"if a > 0 {\n}", "if a > 0 {\n}", "a := 1", "draft"}; !slices.Equal(recalled, want) {
		t.Errorf("recalled %q, want %q", recalled, want)
	}
	if interrupts != 1 {
		t.Errorf("%d interrupts, want 1", interrupts)
	}
	if got, want := c.Text(), "hello\n>>> a := 1\n>>> if a > 0 {\n    }\n42\n"; got != want {
		t.Errorf("the console shows %q, want %q", got, want)
	}
	if got := c.typed(); got != "draft" {
		t.Errorf("the box holds %q after a Return while busy, want it kept", got)
	}
}

// TestPostToLoop runs what another goroutine posts on the loop's, and
// tells that goroutine when the loop has ended.
func TestPostToLoop(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	w.SetVisible(true)
	ran := make(chan bool, 1)
	onLoop := false
	go func() {
		done := make(chan struct{})
		p.PostToLoop(func() {
			onLoop = true
			close(done)
		})
		select {
		case <-done:
			ran <- true
		case <-p.LoopDone():
			ran <- false
		}
	}()
	run(p, func() {})
	if !<-ran || !onLoop {
		t.Error("what was posted didn't run on the loop")
	}
	select {
	case <-p.LoopDone():
	default:
		t.Error("LoopDone isn't closed after the loop ended")
	}
}

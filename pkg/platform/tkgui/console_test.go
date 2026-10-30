package tkgui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/takigo/takigo/event"
	tkplatform "github.com/takigo/takigo/platform"
	"github.com/takigo/takigo/widget/text"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/lineedit"
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
	busyStatus := ""
	run(p,
		func() { c.Focus() },
		func() {
			c.setTyped("a := 1")
			key(tkplatform.XK_Return, 0)
		},
		func() {
			// Shift+Return is a new line in the box, indented after the
			// brace; Return enters both.
			c.setTyped("if a > 0 {")
			key(tkplatform.XK_Return, tkplatform.ShiftMask)
			_ = c.input.Insert(c.input.EndIndex(), "}")
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
			busyStatus = c.status.GetText()
			key(tkplatform.XK_Return, 0)
			key(tkplatform.XK_Escape, 0)
			c.SetBusy(false)
			c.Print(platform.ConsoleValue, "42\n")
		},
	)
	if want := []string{"a := 1", "if a > 0 {\n\t}"}; !slices.Equal(submitted, want) {
		t.Errorf("submitted %q, want %q", submitted, want)
	}
	// The second Up moves to the first of the two lines; the third
	// recalls from there.
	if want := []string{"if a > 0 {\n\t}", "if a > 0 {\n\t}", "a := 1", "draft"}; !slices.Equal(recalled, want) {
		t.Errorf("recalled %q, want %q", recalled, want)
	}
	if busyStatus == "" || c.status.GetText() != "" {
		t.Errorf("the status says %q while busy, %q after", busyStatus, c.status.GetText())
	}
	if interrupts != 1 {
		t.Errorf("%d interrupts, want 1", interrupts)
	}
	if got, want := c.Text(), "hello\n>>> a := 1\n>>> if a > 0 {\n    \t}\n42\n"; got != want {
		t.Errorf("the console shows %q, want %q", got, want)
	}
	if got := c.typed(); got != "draft" {
		t.Errorf("the box holds %q after a Return while busy, want it kept", got)
	}
}

// TestConsoleReadline types readline's keys in a console: kills and
// yanks, words, Tab, Return on a statement that goes on, and searching
// the history with Control+R.
func TestConsoleReadline(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	w.SetVisible(true)
	c := p.CreateConsole(w).(*Console)
	var submitted []string
	ev := c.Events()
	ev.OnSubmit = func(line string) { submitted = append(submitted, line) }
	ev.IsUnfinished = func(text string) bool { return strings.Count(text, "{") > strings.Count(text, "}") }
	ev.OnComplete = func(text string, cursor int) (int, []string) {
		start := strings.LastIndexByte(text[:cursor], '.') + 1
		var out []string
		for _, w := range []string{"Groups(", "Group(", "Sketch"} {
			if strings.HasPrefix(w, text[start:cursor]) {
				out = append(out, w)
			}
		}
		return start, out
	}
	c.SetPrompt(">>> ")
	c.SetHistory([]string{"c := doc.Sketch.Circle(V(0, 0, 0), 2)", "doc.Groups()"})
	c.SetVisible(true)

	key := func(sym tkplatform.KeySym, state uint, str string) {
		p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType,
			Window: c.input.Window().PlatformID, KeySym: sym, State: state, Str: str})
	}
	ctrl := func(r rune) { key(tkplatform.KeySym(r), tkplatform.ControlMask, "") }
	alt := func(sym tkplatform.KeySym) { key(sym, tkplatform.Mod1Mask, "") }
	typeText := func(s string) {
		for _, r := range s {
			key(tkplatform.KeySym(r), 0, string(r))
		}
	}
	var seen []string
	look := func() { seen = append(seen, show(c.state())) }
	run(p,
		func() { c.Focus() },
		func() {
			c.setTyped("a := doc.Groups()")
			ctrl('a')
			alt('f')
			alt('f')
			look() // a := doc|.Groups()
			ctrl('k')
			look() // a := doc|
			alt(tkplatform.XK_BackSpace)
			look() // a := |
			ctrl('y')
			look() // a := doc.Groups()|
			ctrl('w')
			ctrl('u')
			look() // |
			ctrl('y')
			look() // a := doc.Groups()|
			alt('b')
			alt('d')
			ctrl('t')
			ctrl('h')
			look() // a := doc(|)
			c.setTyped("doc.Gr")
			key(tkplatform.XK_Tab, 0, "\t")
			look() // doc.Group|
			key(tkplatform.XK_Tab, 0, "\t")
			c.setTyped("")
			key(tkplatform.XK_Tab, 0, "\t")
			look() // a tab
		},
		func() {
			c.setTyped("if a {")
			key(tkplatform.XK_Return, 0, "")
			typeText("x")
			key(tkplatform.XK_Return, 0, "")
			look() // if a {\n\tx\n\t|
			c.setTyped("if a {\n}")
			key(tkplatform.XK_Return, 0, "")
		},
		func() {
			c.setTyped("draft")
			ctrl('r')
			typeText("Sk")
			look() // the circle, at Sk
			ctrl('r')
			key(tkplatform.XK_BackSpace, 0, "")
			key(tkplatform.XK_Escape, 0, "")
			look() // draft
			ctrl('r')
			key(tkplatform.KeySym(0xffe1), tkplatform.ShiftMask, "") // Shift_L
			typeText("Gro")
			key(tkplatform.XK_Return, 0, "")
			ctrl('p')
			look() // the circle
			ctrl('l')
		},
	)
	want := []string{"a := doc|.Groups()", "a := doc|", "a := |", "a := doc.Groups()|", "|",
		"a := doc.Groups()|", "a := doc(|)", "doc.Group|", "\t|",
		"if a {\n\tx\n\t|",
		"c := doc.|Sketch.Circle(V(0, 0, 0), 2)", "draft|", "doc.Groups()|"}
	if !slices.Equal(seen, want) {
		t.Errorf("the box showed\n%q\nwant\n%q", seen, want)
	}
	if want := []string{"if a {\n}", "doc.Groups()"}; !slices.Equal(submitted, want) {
		t.Errorf("submitted %q, want %q", submitted, want)
	}
	if got := c.Text(); got != "" {
		t.Errorf("after Control+L the console shows %q", got)
	}
}

// show writes what is in a console's box with a | at the cursor.
func show(s lineedit.State) string { return s.Text[:s.Cursor] + "|" + s.Text[s.Cursor:] }

// TestConsoleIndexes turns byte offsets in the box's text into the
// text's indexes and back.
func TestConsoleIndexes(t *testing.T) {
	t.Parallel()
	s := "añb\ncd"
	for off, idx := range map[int]text.Index{0: {Line: 1}, 3: {Line: 1, Char: 2}, 4: {Line: 1, Char: 3}, 5: {Line: 2}, 7: {Line: 2, Char: 2}} {
		if got := indexOf(s, off); got != idx {
			t.Errorf("indexOf(%d) = %v, want %v", off, got, idx)
		}
		if got := offsetOf(s, idx); got != off {
			t.Errorf("offsetOf(%v) = %d, want %d", idx, got, off)
		}
	}
	if offsetOf(s, text.Index{Line: 3}) != len(s) || offsetOf(s, text.Index{Line: 1, Char: 9}) != 4 {
		t.Error("an index past the text")
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

// TestConsoleClearScrolled clears a console scrolled down its text:
// the text's view must come back to its start, where the text is.
func TestConsoleClearScrolled(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	c := p.CreateConsole(w).(*Console)
	c.SetVisible(true)
	run(p,
		func() {
			for i := range 200 {
				c.Print(platform.ConsoleOutput, strings.Repeat("x", i%7)+"\n")
			}
		},
		func() { c.Clear() },
		func() { c.Print(platform.ConsoleInfo, "after\n") },
	)
	if got := c.Text(); got != "after\n" {
		t.Errorf("after clearing the console shows %q", got)
	}
}

// TestConsoleTabKeepsFocus has a Tab the console takes marked handled,
// so that the focus manager doesn't move the focus on from the box,
// and leaves a key it doesn't take unmarked.
func TestConsoleTabKeepsFocus(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	c := p.CreateConsole(w).(*Console)
	c.SetVisible(true)
	tab := &event.Event{Type: event.KeyPressType, Window: c.input.Window().PlatformID, KeySym: tkplatform.XK_Tab}
	other := &event.Event{Type: event.KeyPressType, Window: c.input.Window().PlatformID, KeySym: tkplatform.KeySym('q'), Str: "q"}
	run(p,
		func() { c.Focus() },
		func() {
			c.setTyped("doc.")
			p.app.Dispatcher().Dispatch(tab)
			p.app.Dispatcher().Dispatch(other)
		},
	)
	if !tab.Handled {
		t.Error("the console's Tab isn't marked handled")
	}
	if got := c.typed(); got != "doc.q" {
		t.Errorf("after Tab and q the box holds %q", got)
	}
}

// TestConsoleLayout gives what is typed a fifth of the panes' height,
// and shows the end of a text filled before the console was laid out
// at its bottom.
func TestConsoleLayout(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	c := p.CreateConsole(w).(*Console)
	c.Print(platform.ConsoleOutput, "before\n")
	c.SetVisible(true)
	// What help() prints comes once the window is shown, before it is
	// laid out.
	for i := range 100 {
		c.Print(platform.ConsoleOutput, fmt.Sprintf("line %d\n", i))
	}
	var share, first, last float64
	run(p, func() {}, func() {}, func() {
		share = float64(c.input.Win.Height) / float64(c.input.Win.Height+c.log.Win.Height)
		first, last = c.scroll.First, c.scroll.Last
	})
	if share < 0.17 || share > 0.23 {
		t.Errorf("what is typed has %.2f of the height, want 0.2", share)
	}
	// Scrolled past its end, the view would hold the last line alone.
	if last < 0.99 || last-first < 0.1 {
		t.Errorf("the view is %.2f to %.2f of the text, want a screenful ending at its end", first, last)
	}
}

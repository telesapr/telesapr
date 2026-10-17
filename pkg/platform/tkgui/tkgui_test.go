package tkgui

import (
	"image"
	"image/color"
	"maps"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/takigo/takigo/event"
	tkplatform "github.com/takigo/takigo/platform"
	"github.com/takigo/takigo/widget"

	"github.com/telesapr/telesapr/pkg/platform"
)

// requireGUI skips unless TELESAPR_GUI_TESTS is set (make test-gui
// sets it and runs these under Xvfb), so that go test doesn't open
// windows on the desktop.
func requireGUI(t *testing.T) *Platform {
	t.Helper()
	if os.Getenv("TELESAPR_GUI_TESTS") == "" {
		t.Skip("set TELESAPR_GUI_TESTS=1 (make test-gui) to run GUI tests")
	}
	p, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// run runs the event loop, with steps run in turn from timers, then
// quits.
func run(p *Platform, steps ...func()) {
	i := 0
	var next func()
	next = func() {
		if i == len(steps) {
			p.Exit()
			return
		}
		steps[i]()
		i++
		p.app.After(100*time.Millisecond, next)
	}
	p.app.After(200*time.Millisecond, next)
	p.Run()
}

// step is an action and the state it brings about, which runWaiting
// waits for before the next step, so that a busy machine slows a test
// down and does not fail it.
type step struct {
	do   func()
	done func() bool
}

// runWaiting runs the event loop, with each step run once the one
// before it is done, then quits. A step not done within stepTimeout
// fails the test.
func runWaiting(t *testing.T, p *Platform, steps ...step) {
	t.Helper()
	const stepTimeout = 20 * time.Second
	i := 0
	var deadline time.Time
	var poll func()
	poll = func() {
		if i > 0 && !steps[i-1].done() {
			if time.Now().After(deadline) {
				t.Errorf("step %d not done after %v", i-1, stepTimeout)
				p.Exit()
				return
			}
			p.app.After(10*time.Millisecond, poll)
			return
		}
		if i == len(steps) {
			p.Exit()
			return
		}
		steps[i].do()
		i++
		deadline = time.Now().Add(stepTimeout)
		p.app.After(10*time.Millisecond, poll)
	}
	p.app.After(10*time.Millisecond, poll)
	p.Run()
}

func TestWindowRendersAndReportsInput(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil).(*Window)
	w.SetMinContentSize(320, 240)
	renders := 0
	var events []platform.MouseEvent
	var keys []platform.KeyboardEvent
	ev := w.Events()
	ev.OnRender = func() {
		renders++
		width, height := w.ContentSize()
		img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
		for i := range img.Pix {
			img.Pix[i] = 0x80
		}
		img.Set(0, 0, color.RGBA{255, 0, 0, 255})
		w.canvas.present(img)
	}
	ev.OnMouseEvent = func(e platform.MouseEvent) bool { events = append(events, e); return true }
	ev.OnKeyboardEvent = func(e platform.KeyboardEvent) bool { keys = append(keys, e); return true }
	w.SetVisible(true)

	var width, height float64
	run(p,
		func() { width, height = w.ContentSize(); w.Focus() },
		func() {
			// Two quick clicks are a press, a release, then a double press.
			for range 2 {
				w.buttonPress(synthButton(1, 10, 20))
				w.mouse(platform.MouseRelease, synthButton(1, 10, 20), 1, 0)
			}
			w.wheel(&event.Event{Type: event.MouseWheelType, Delta: 120, X: 10, Y: 20})
			w.key(platform.KeyPress, synthKey('A', 1))
			w.key(platform.KeyPress, synthKey(0xffbf, 0))
		},
	)
	if renders == 0 {
		t.Error("never rendered")
	}
	if width < 320 || height < 240 {
		t.Errorf("content size %vx%v, want at least 320x240", width, height)
	}
	want := []platform.MouseEventType{platform.MousePress, platform.MouseRelease,
		platform.MouseDblPress, platform.MouseRelease, platform.MouseScrollVert}
	if len(events) != len(want) {
		t.Fatalf("events %+v", events)
	}
	for i, e := range events {
		if e.Type != want[i] {
			t.Errorf("event %d: %v, want %v", i, e.Type, want[i])
		}
	}
	if events[4].ScrollDelta != 1 {
		t.Errorf("scroll delta %v", events[4].ScrollDelta)
	}
	if len(keys) != 2 || keys[0].Chr != 'a' || !keys[0].ShiftDown ||
		keys[1].Key != platform.KeyFunction || keys[1].Num != 2 {
		t.Errorf("keys %+v", keys)
	}
}

func TestMenuStateAndClear(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	bar, _ := p.MainMenu()
	file := bar.AddSubMenu("&File")
	file.AddItem("&New", nil, true)
	recent := file.AddSubMenu("Open &Recent")
	recent.AddItem("a.slvs", nil, false)
	view := bar.AddSubMenu("&View")
	grid := view.AddItem("Show &Grid", nil, true)
	grid.SetIndicator(platform.IndicatorCheckMark)
	grid.SetActive(true)
	grid.SetEnabled(false)
	w.SetMenuBar(bar)

	e := grid.(*MenuItem).entry()
	if !e.Checked || e.State == 0 || e.Label != "Show Grid" || e.Underline != 5 {
		t.Errorf("grid entry %+v", *e)
	}
	r := recent.(*Menu)
	old := r.m
	recent.Clear()
	recent.AddItem("b.slvs", nil, false)
	fm := file.(*Menu)
	if fm.m.Entries()[1].SubMenu != r.m || r.m == old || len(r.m.Entries()) != 1 {
		t.Error("clearing a submenu didn't replace it in its cascade")
	}
	run(p)
}

// TestMenuBarKeyboard opens menus as GTK's menu bar does: Alt and a
// mnemonic, F10, then Left and Right to the neighbouring menus, with
// the first entry active so Return invokes it.
func TestMenuBarKeyboard(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil).(*Window)
	bar, _ := p.MainMenu()
	var invoked []string
	add := func(label, item string) *Menu {
		m := bar.AddSubMenu(label)
		m.AddItem(item, func() { invoked = append(invoked, item) }, true)
		return m.(*Menu)
	}
	file, edit, view := add("&File", "&New"), add("&Edit", "&Undo"), add("&View", "Zoom &In")
	w.SetMenuBar(bar)
	w.SetVisible(true)

	posted := func() string {
		var s string
		for _, m := range []*Menu{file, edit, view} {
			if m.m.IsPosted() {
				s += m.m.Entries()[0].Label
			}
		}
		return s
	}
	key := func(m *Menu, sym tkplatform.KeySym) {
		p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType,
			Window: m.m.Window().PlatformID, KeySym: sym})
	}
	// Each key waits for the menus it posts, and the step after it
	// records them.
	var got []string
	record := func() { got = append(got, posted()) }
	postedIs := func(want string) func() bool { return func() bool { return posted() == want } }
	runWaiting(t, p,
		step{func() { w.Focus() }, func() bool { return w.mapped }},
		step{func() { w.key(platform.KeyPress, synthKey('e', tkplatform.Mod1Mask)) }, postedIs("Undo")},
		step{func() { record(); key(edit, tkplatform.XK_Right) }, postedIs("Zoom In")},
		step{func() { record(); key(view, tkplatform.XK_Right) }, postedIs("New")},
		step{func() { record(); key(file, tkplatform.XK_Left) }, postedIs("Zoom In")},
		step{func() { record(); key(view, tkplatform.XK_Return) }, func() bool { return posted() == "" && len(invoked) > 0 }},
		step{func() { record(); w.key(platform.KeyPress, synthKey(xkF10, 0)) }, postedIs("New")},
		step{func() { record(); key(file, tkplatform.XK_Escape) }, postedIs("")},
		step{record, func() bool { return true }},
	)
	want := []string{"Undo", "Zoom In", "New", "Zoom In", "", "New", ""}
	if !slices.Equal(got, want) {
		t.Errorf("posted menus %q, want %q", got, want)
	}
	if !slices.Equal(invoked, []string{"Zoom &In"}) {
		t.Errorf("invoked %q", invoked)
	}
}

// TestMessageDialogKeys presses a message dialog's buttons from the
// keyboard: Alt and a mnemonic, and Return for the default button.
func TestMessageDialogKeys(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	w.SetVisible(true)
	ask := func(sym tkplatform.KeySym, state uint) platform.Response {
		d := p.CreateMessageDialog(w)
		d.SetMessage("Save?")
		d.AddButton("&Save", platform.ResponseYes, true)
		d.AddButton("Do&n't Save", platform.ResponseNo, false)
		d.AddButton("&Cancel", platform.ResponseCancel, false)
		dlg := d.(*MessageDialog)
		press := func(sym tkplatform.KeySym, state uint) {
			p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType,
				Window: dlg.win.PlatformID, KeySym: sym, State: state})
		}
		p.app.After(300*time.Millisecond, func() {
			press(sym, state)
			// Close a dialog the key left open, which then answers none.
			p.app.After(100*time.Millisecond, func() {
				if !dlg.win.IsDestroyed() {
					press(tkplatform.XK_Escape, 0)
				}
			})
		})
		return d.RunModal()
	}
	var got []platform.Response
	run(p, func() {
		got = append(got, ask('n', tkplatform.Mod1Mask), ask('C', tkplatform.Mod1Mask|tkplatform.ShiftMask),
			ask(tkplatform.XK_Return, 0), ask('n', 0))
	})
	want := []platform.Response{platform.ResponseNo, platform.ResponseCancel,
		platform.ResponseYes, platform.ResponseNone}
	if !slices.Equal(got, want) {
		t.Errorf("responses %v, want %v", got, want)
	}
}

// TestWheelScrolls checks that takigo's wheel events, which replace
// presses of buttons 4 and 5, reach the window as vertical scrolls.
func TestWheelScrolls(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil).(*Window)
	var deltas []float64
	w.Events().OnMouseEvent = func(e platform.MouseEvent) bool {
		if e.Type == platform.MouseScrollVert {
			deltas = append(deltas, e.ScrollDelta)
		}
		return true
	}
	w.SetVisible(true)
	run(p, func() {
		for _, d := range []int{120, -120, 30} {
			p.app.Dispatcher().DispatchWindow(&event.Event{Type: event.MouseWheelType,
				Window: w.canvas.Win.PlatformID, Delta: d, X: 10, Y: 20})
		}
	})
	if !slices.Equal(deltas, []float64{1, -1, 0.25}) {
		t.Errorf("scroll deltas %v, want [1 -1 0.25]", deltas)
	}
}

// TestTooltipStaysWhilePointerRests checks that a tooltip, a window of
// its own, shows after the pointer rests in its area, stays while the
// pointer moves within it and frames are drawn, and goes once it
// leaves.
func TestTooltipStaysWhilePointerRests(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil).(*Window)
	w.SetMinContentSize(320, 240)
	w.Events().OnRender = func() {
		width, height := w.ContentSize()
		w.canvas.present(image.NewRGBA(image.Rect(0, 0, int(width), int(height))))
	}
	w.SetVisible(true)
	motion := func(x, y int) {
		w.tooltip.motion(w.canvas, &event.Event{Type: event.MotionType, X: x, Y: y, RootX: x, RootY: y})
	}
	var early, rested, kept, left bool
	shown := func() bool { return w.tooltip.shown }
	runWaiting(t, p,
		step{func() {
			w.SetTooltip("Sketch line segment", 10, 10, 32, 32)
			motion(20, 20)
			early = w.tooltip.shown
		}, shown},
		step{func() { rested = w.tooltip.shown }, func() bool { return true }},
		step{func() {
			motion(25, 22)
			// Redrawing the frame, as hovering a button does, keeps it.
			w.SetTooltip("Sketch line segment", 10, 10, 32, 32)
			w.Invalidate()
		}, func() bool { return w.tooltip.win != nil }},
		step{func() { kept = w.tooltip.shown && w.tooltip.win != nil }, func() bool { return true }},
		step{func() { motion(100, 100); left = !w.tooltip.shown }, func() bool { return true }},
	)
	if early {
		t.Error("tooltip shown before the pointer rested")
	}
	if !rested {
		t.Error("tooltip not shown after the pointer rested")
	}
	if !kept {
		t.Error("tooltip gone while the pointer stays over the button")
	}
	if !left {
		t.Error("tooltip still shown after the pointer left its area")
	}
}

// TestFormDialog edits a form's entry as the user does, which the
// change callback answers by setting another field, and accepts it from
// the keyboard; Escape cancels it.
func TestFormDialog(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	w.SetVisible(true)
	ask := func(sym tkplatform.KeySym) (platform.FormValues, bool) {
		d := p.CreateFormDialog(w)
		d.SetTitle("Options")
		d.AddChoice("q", "Quality", []string{"Low", "High", "Custom"}, 0)
		d.AddEntry("n", "Number", "5", "things")
		d.AddCheck("c", "Strict", true)
		d.OnChange(func(key, value string) platform.FormValues {
			if key == "n" {
				return platform.FormValues{"q": "2"}
			}
			return nil
		})
		dlg := d.(*FormDialog)
		p.app.After(300*time.Millisecond, func() {
			win := dlg.fields[1].entry.Window()
			if sym == tkplatform.XK_Return {
				widget.Focus(p.app, win)
				p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType, Window: win.PlatformID,
					KeySym: '7', Str: "7"})
			}
			p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType, Window: win.PlatformID, KeySym: sym})
		})
		return d.RunModal()
	}
	var got platform.FormValues
	var ok, cancelled bool
	run(p, func() {
		got, ok = ask(tkplatform.XK_Return)
		_, cancelled = ask(tkplatform.XK_Escape)
	})
	if want := (platform.FormValues{"q": "2", "n": "75", "c": "1"}); !ok || !maps.Equal(got, want) {
		t.Errorf("accepted %v %v, want %v", got, ok, want)
	}
	if cancelled {
		t.Error("Escape accepted the form")
	}
}

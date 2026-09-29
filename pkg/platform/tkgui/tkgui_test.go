package tkgui

import (
	"image"
	"image/color"
	"os"
	"testing"
	"time"

	"github.com/spacesolve/spacesolve/pkg/platform"
)

// requireGUI skips unless SPACESOLVE_GUI_TESTS is set (make test-gui
// sets it and runs these under Xvfb), so that go test doesn't open
// windows on the desktop.
func requireGUI(t *testing.T) *Platform {
	t.Helper()
	if os.Getenv("SPACESOLVE_GUI_TESTS") == "" {
		t.Skip("set SPACESOLVE_GUI_TESTS=1 (make test-gui) to run GUI tests")
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
		width, height := w.GetContentSize()
		img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
		for i := range img.Pix {
			img.Pix[i] = 0x80
		}
		img.Set(0, 0, color.RGBA{255, 0, 0, 255})
		w.PresentFrame(img)
	}
	ev.OnMouseEvent = func(e platform.MouseEvent) bool { events = append(events, e); return true }
	ev.OnKeyboardEvent = func(e platform.KeyboardEvent) bool { keys = append(keys, e); return true }
	w.SetVisible(true)

	var width, height float64
	run(p,
		func() { width, height = w.GetContentSize(); w.Focus() },
		func() {
			// Two quick clicks are a press, a release, then a double press.
			for range 2 {
				w.buttonPress(synthButton(1, 10, 20))
				w.mouse(platform.MouseRelease, synthButton(1, 10, 20), 1, 0)
			}
			w.buttonPress(synthButton(4, 10, 20))
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
	bar, _ := p.GetOrCreateMainMenu()
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

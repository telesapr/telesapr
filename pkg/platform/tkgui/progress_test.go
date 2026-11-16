package tkgui

import (
	"image"
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/platform"
)

// TestProgressHoldsTheApplication shows a progress window over a
// window and updates it, as an open does on the loop: it is mapped,
// the windows repaint, and the application's timer, the work posted to
// the loop, the window's render and its close wait until it closes,
// then run once each.
func TestProgressHoldsTheApplication(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil).(*Window)
	w.SetMinContentSize(320, 240)
	renders := 0
	ev := w.Events()
	ev.OnRender = func() {
		renders++
		width, height := w.ContentSize()
		w.canvas.present(image.NewRGBA(image.Rect(0, 0, int(width), int(height))))
	}
	closes := 0
	ev.OnClose = func() { closes++ }
	w.SetVisible(true)
	// The property browser, which the watch covers too.
	browser := p.CreateWindow(platform.WindowTool, w)
	browser.SetVisible(true)

	timer := p.CreateTimer()
	fired := 0
	timer.SetOnTimeout(func() { fired++ })
	posted := 0

	var pr platform.Progress
	var pw *progressWindow
	var during struct{ fired, posted, renders, closes int }
	runWaiting(t, p,
		step{func() {}, func() bool { return renders > 0 && browser.(*Window).mapped }},
		step{func() {
			pr = p.ShowProgress(w, "Opening part.tlspr")
			pw = pr.(*progressWindow)
			if !pw.mapped {
				t.Error("the progress window wasn't mapped when ShowProgress returned")
			}
			if len(pw.covers) != 2 {
				t.Errorf("%d windows covered, want the main window and the browser", len(pw.covers))
			}
			if p.ShowProgress(w, "inner") == pr {
				t.Error("a nested progress made a second window")
			}
			rendersBefore := renders
			timer.RunAfter(0)
			p.PostToLoop(func() { posted++ })
			w.Invalidate()
			w.events.OnClose = nil
			p.runOrHold(func() { closes++ })
			// The work, which keeps the loop and updates the window.
			for i := range 6 {
				pr.Update("Regenerating", float64(i)/6)
				time.Sleep(progressEvery / 2)
			}
			long := "Regenerating g008-relief-fillet-round-the-window-of-the-tube (group 8 of 52)"
			time.Sleep(progressEvery)
			pr.Update(long, 0.5)
			if lw, top := pw.text.Win, pw.bar.Win.Y; lw.ReqHeight > top {
				t.Errorf("the text, %d high, doesn't fit over the bar at %d", lw.ReqHeight, top)
			}
			if lw := pw.text.Win; lw.ReqWidth > lw.Width {
				t.Errorf("the text, %d wide, doesn't fit in %d", lw.ReqWidth, lw.Width)
			}
			pr.Update("Writing part.stl", -1)
			time.Sleep(progressEvery)
			pr.Update("Writing part.stl", -1)
			during.fired, during.posted, during.closes = fired, posted, closes
			during.renders = renders - rendersBefore
			if got := pw.text.GetText(); got != "Writing part.stl" {
				t.Errorf("text %q", got)
			}
			pr.Close()
			pr.Close()
		}, func() bool { return fired > 0 && posted > 0 }},
		step{func() {}, func() bool { return true }},
	)
	if during.fired != 0 || during.posted != 0 || during.renders != 0 || during.closes != 0 {
		t.Errorf("ran while the progress window was up: %+v", during)
	}
	if fired != 1 || posted != 1 || closes != 1 {
		t.Errorf("after it closed: timer %d, posted %d, close %d; want 1 each", fired, posted, closes)
	}
	if p.progress != nil || len(p.held) != 0 {
		t.Error("the platform still holds the progress window")
	}
	if !pw.top.Destroyed() {
		t.Error("the progress window wasn't destroyed")
	}
}

// TestTimerRestartedWhileHeldFiresOnce restarts a timer whose timeout
// was held: only the restarted one fires.
func TestTimerRestartedWhileHeldFiresOnce(t *testing.T) {
	p := requireGUI(t)
	w := p.CreateWindow(platform.WindowToplevel, nil)
	w.SetVisible(true)
	timer := p.CreateTimer()
	fired := 0
	timer.SetOnTimeout(func() { fired++ })
	runWaiting(t, p,
		step{func() {
			pr := p.ShowProgress(w, "Exporting")
			timer.RunAfter(0)
			time.Sleep(progressEvery)
			pr.Update("Writing", -1) // the timeout comes and is held
			timer.RunAfter(50)
			pr.Close()
		}, func() bool { return fired > 0 }},
		step{func() {}, func() bool { return true }},
	)
	// Let a second timeout, which must not come, have its time.
	time.Sleep(100 * time.Millisecond)
	if fired != 1 {
		t.Errorf("fired %d times, want 1", fired)
	}
}

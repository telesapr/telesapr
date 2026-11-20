package ui

import (
	"context"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/agent"
)

// A program connected to the window is told, through the socket, when
// the user changed the drawing between its calls: an undo in the
// window, a new file; not its own scripts, nor a picture of a group
// other than the active one, which activates that group for a moment.
func TestAgentToldOfUserChanges(t *testing.T) {
	t.Parallel()
	a := newAgentApp(t)
	a.on(func() { a.ss.SetAgentFiles(false, nil) })
	seen := func(call func(ctx context.Context) error) agent.Seen {
		t.Helper()
		ctx, w := agent.WatchRevision(context.Background())
		if err := call(ctx); err != nil {
			t.Fatal(err)
		}
		s, ok := w.Seen()
		if !ok {
			t.Fatal("the window told nothing of the drawing's revision")
		}
		return s
	}
	describe := func(ctx context.Context) error {
		_, err := a.r.Describe(ctx, agent.Describing{})
		return err
	}
	since := func(what string, s, last agent.Seen, changed, another bool) {
		t.Helper()
		if c, an := s.Since(last); c != changed || an != another {
			t.Errorf("%s: %+v after %+v is changed %v, another %v; want %v, %v", what, s, last, c, an, changed, another)
		}
	}

	last := seen(describe)
	s := seen(func(ctx context.Context) error {
		_, err := a.r.Run(ctx, "doc.Sketch.Rect(V(0, 0, 0), V(40, 20, 0))\ndoc.Features.Extrude(5, \"plate\")\ndoc.Activate(doc.Group(\"g002\"))")
		return err
	})
	since("the program's script", s, last, false, false)
	last = s
	s = seen(func(ctx context.Context) error {
		_, err := a.r.Render(ctx, agent.Picture{View: "top", Group: "g003", Width: 40, Height: 30})
		return err
	})
	since("a picture of another group", s, last, false, false)
	last = s

	a.on(func() { a.p.MainMenuBar.Find("Edit", "Undo").OnTrigger() })
	s = seen(describe)
	since("an undo in the window", s, last, true, false)
	last = s
	s = seen(func(ctx context.Context) error {
		_, err := a.r.Measure(ctx, agent.Measuring{})
		return err
	})
	since("the call after", s, last, false, false)
	last = s

	// The user changes the drawing while the program's script runs:
	// told on that script's result.
	ran := make(chan agent.Seen, 1)
	go func() {
		ctx, w := agent.WatchRevision(context.Background())
		_, err := a.r.Run(ctx, "doc.Sketch.Circle(V(0, 0, 0), 5)\ntime.Sleep(400 * time.Millisecond)\ndoc.Sketch.Circle(V(0, 0, 0), 7)")
		s, ok := w.Seen()
		if err != nil || !ok {
			t.Errorf("the script during a change gave %v, a revision %v", err, ok)
		}
		ran <- s
	}()
	for busy := false; !busy; {
		a.on(func() { busy = a.ss.Console().Busy() })
	}
	a.on(func() { a.ss.UndoRemember() })
	s = <-ran
	since("a change while the program's script ran", s, last, true, false)
	last = s

	a.on(func() {
		a.ss.NewFile()
		a.ss.AfterNewFile()
	})
	s = seen(describe)
	since("a new file in the window", s, last, true, true)
}

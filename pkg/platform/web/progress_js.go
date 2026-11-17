//go:build js && wasm

package web

import (
	"time"

	"github.com/telesapr/telesapr/pkg/platform"
)

// A file is opened, saved and exported on the loop's goroutine, and the page
// has no other thread: the progress is the busy box over the page, its
// text and a bar, and each update that shows lets the page draw a
// frame. The dialogs that asked for the file are gone from the page by
// then, since each ends before it returns.

var _ platform.ProgressShower = (*Platform)(nil)

// progress is a Progress in the busy box.
type progress struct {
	p      *Platform
	last   time.Time
	closed bool
}

// ShowProgress shows title in the busy box and lets the page draw it.
func (p *Platform) ShowProgress(_ platform.Window, title string) platform.Progress {
	if p.progress != nil {
		return nestedProgress{p.progress}
	}
	pr := &progress{p: p, last: time.Now()}
	p.progress = pr
	p.w.Call("busy", title, -1)
	p.waitFrame()
	return pr
}

// Update shows text and fraction every yieldEvery, letting the page
// draw.
func (pr *progress) Update(text string, fraction float64) {
	if pr.closed || time.Since(pr.last) < yieldEvery {
		return
	}
	pr.last = time.Now()
	pr.p.w.Call("busy", text, fraction)
	pr.p.waitFrame()
}

// Close takes the busy box away.
func (pr *progress) Close() {
	if pr.closed {
		return
	}
	pr.closed = true
	pr.p.progress = nil
	pr.p.busy.end()
	pr.p.w.Call("idle")
}

// nestedProgress is the Progress of an operation inside one shown
// already: it shows there, and the outer one closes it.
type nestedProgress struct{ outer *progress }

func (n nestedProgress) Update(text string, fraction float64) { n.outer.Update(text, fraction) }
func (nestedProgress) Close()                                 {}

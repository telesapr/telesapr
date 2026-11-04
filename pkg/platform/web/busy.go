package web

import (
	"fmt"
	"time"
)

// A regeneration runs on the event loop's goroutine, and a page has no
// other thread: until it ends, the page draws nothing and the browser
// may call it unresponsive. Between its groups the application calls
// Platform.Yield, which, every yieldEvery, lets the page draw a frame;
// once the work has gone on for showBusyAfter it shows how long. What
// the loop runs next ends the busy spell.
const (
	// yieldEvery is how often the page is let to draw: often enough to
	// look alive, seldom enough that the frames cost little of the work.
	yieldEvery = 100 * time.Millisecond
	// showBusyAfter is how long work runs before the page says so: a
	// regeneration shorter than that shows nothing.
	showBusyAfter = 300 * time.Millisecond
)

// busyClock times a spell of work on the loop: when to let the page draw
// and when to show it.
type busyClock struct {
	start, last time.Time
	shown       bool
}

// step is a call between groups at now: whether to let the page draw,
// and, when it does, the text to show, "" while the spell is short.
func (b *busyClock) step(now time.Time) (yield bool, text string) {
	if b.start.IsZero() {
		b.start, b.last = now, now
		return false, ""
	}
	if now.Sub(b.last) < yieldEvery {
		return false, ""
	}
	b.last = now
	took := now.Sub(b.start)
	if took < showBusyAfter {
		return true, ""
	}
	b.shown = true
	return true, busyText(took)
}

// end closes the spell and reports whether it was shown, so that it is
// taken off the page.
func (b *busyClock) end() bool {
	shown := b.shown
	*b = busyClock{}
	return shown
}

// busyText is what the page shows during a spell that has taken took.
func busyText(took time.Duration) string {
	return fmt.Sprintf("Regenerating the model… %d s", int(took.Seconds()))
}

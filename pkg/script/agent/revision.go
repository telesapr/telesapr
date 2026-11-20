package agent

import (
	"context"
	"sync"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// This file tells a caller of a Backend whether the drawing changed
// between its calls by someone else: the user in the window, or
// another program connected to it. Each call says the drawing's
// revision (sketch.Revision) as it left it, and how many of the changes
// counted up to it were the call's own; the caller compares with what
// its last call said. The Backend's methods keep their signatures: what
// a call found rides on its context (WatchRevision), so that a caller
// that doesn't ask costs nothing more and the socket carries it on the
// answer it sends anyway.

// Seen is what a call found of the drawing's revision.
type Seen struct {
	// Drawing changes when another file is opened, or a new one
	// started.
	Drawing uint64 `json:"drawing"`
	// Change counts the changes of the drawing, as the call left it.
	Change uint64 `json:"change"`
	// Own is how many of those were the call's: what its script did,
	// or the file it opened. A change by anyone else while the call
	// ran, as the user's in the window during a long script, is not.
	Own uint64 `json:"own,omitempty"`
}

// Since reports whether the drawing changed by someone else's doing
// between last, what the caller's last call found, and the end of the
// call that found s; and, if it did, whether it is another drawing
// altogether (a script of the call's own that opened a file while the
// user changed the drawing would be taken for that).
func (s Seen) Since(last Seen) (changed, another bool) {
	changed = s.Change-s.Own != last.Change
	return changed, changed && s.Drawing != last.Drawing
}

// Watch is where a call made with WatchRevision's context puts what it
// found of the drawing's revision.
type Watch struct {
	mu    sync.Mutex
	seen  Seen
	known bool
}

// Seen is what the call found, and false if its Backend doesn't tell,
// or the call failed before it could.
func (w *Watch) Seen() (Seen, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.seen, w.known
}

func (w *Watch) set(s Seen) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seen, w.known = s, true
}

type watchKey struct{}

// WatchRevision is ctx with a Watch, which a Backend's call made with
// the context fills in.
func WatchRevision(ctx context.Context) (context.Context, *Watch) {
	w := &Watch{}
	return context.WithValue(ctx, watchKey{}, w), w
}

// KeepWatch is ctx with the Watch of from, if it has one: for a call
// made on a context that isn't derived from the caller's.
func KeepWatch(ctx, from context.Context) context.Context {
	if w := watchOf(from); w != nil {
		return context.WithValue(ctx, watchKey{}, w)
	}
	return ctx
}

func watchOf(ctx context.Context) *Watch {
	w, _ := ctx.Value(watchKey{}).(*Watch)
	return w
}

// runCounter is a host that counts the changes made inside its Run
// calls (api.SketchHost.RunChanges): a script's, not what else changed
// the sketch between them.
type runCounter interface {
	RunChanges() uint64
}

// revisionOf is the revision of h's sketch, and the changes made in
// its Run calls so far, read on the goroutine that owns the sketch; ok
// is false if it couldn't be read, as when the window is closing.
func revisionOf(h api.Host) (r sketch.Revision, runs uint64, ok bool) {
	err := try(func() {
		h.Run(func() {
			r = h.Sketch().Revision()
			if c, is := h.(runCounter); is {
				runs = c.RunChanges()
			}
		})
	})
	return r, runs, err == nil
}

// track reads the revision for a call made with ctx, if it has a
// Watch, and returns what puts it in the Watch at the call's end. With
// own, the changes the session's scripts made in between are the
// call's; without, the call changes nothing, and any change in between
// is someone else's.
func (s *Session) track(ctx context.Context, own bool) (end func()) {
	w := watchOf(ctx)
	if w == nil {
		return func() {}
	}
	h := s.engine.Host()
	before, runsBefore, ok := revisionOf(h)
	if !ok {
		return func() {}
	}
	return func() {
		after, runs, ok := revisionOf(h)
		if !ok {
			return
		}
		seen := Seen{Drawing: after.Drawing, Change: after.Change}
		if own {
			_, counts := h.(runCounter)
			seen.Own = after.Change - before.Change
			if counts {
				seen.Own = min(seen.Own, runs-runsBefore)
			}
		}
		w.set(seen)
	}
}

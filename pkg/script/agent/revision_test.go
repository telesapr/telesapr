//go:build !js

package agent

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// seenBy runs call with a watched context, and is what it found of the
// drawing's revision.
func seenBy(t *testing.T, call func(ctx context.Context) error) Seen {
	t.Helper()
	ctx, w := WatchRevision(context.Background())
	if err := call(ctx); err != nil {
		t.Fatal(err)
	}
	seen, ok := w.Seen()
	if !ok {
		t.Fatal("the call told nothing of the drawing's revision")
	}
	return seen
}

func describes(b Backend) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		_, err := b.Describe(ctx, Describing{})
		return err
	}
}

func runs(b Backend, code string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		_, err := b.Run(ctx, code)
		return err
	}
}

// wantSince checks what Since says of seen after last.
func wantSince(t *testing.T, what string, seen, last Seen, changed, another bool) {
	t.Helper()
	if c, a := seen.Since(last); c != changed || a != another {
		t.Errorf("%s: %+v after %+v is changed %v, another %v; want %v, %v", what, seen, last, c, a, changed, another)
	}
}

// A session tells what each call found of the drawing's revision: a
// script of the call's own, one rolled back and a file it opened are
// its own changes; a change between calls, and another file opened
// then, are not.
func TestSessionRevision(t *testing.T) {
	t.Parallel()
	s := newSession(t)
	doc := s.Engine().Doc()
	ctx := context.Background()

	last := seenBy(t, describes(s))
	seen := seenBy(t, runs(s, "doc.Sketch.Circle(V(0, 0, 0), 5)"))
	if seen.Own == 0 {
		t.Errorf("a script that drew counted no change of its own: %+v", seen)
	}
	wantSince(t, "the model's script", seen, last, false, false)
	last = seen
	seen = seenBy(t, runs(s, "doc.Sketch.Circle(V(0, 0, 0), 3)\nundefined()"))
	wantSince(t, "a script rolled back", seen, last, false, false)
	last = seen
	seen = seenBy(t, describes(s))
	wantSince(t, "a read", seen, last, false, false)
	if seen.Change != last.Change || seen.Own != 0 {
		t.Errorf("a read moved the revision from %+v to %+v", last, seen)
	}

	// Someone else draws between two calls.
	doc.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	seen = seenBy(t, describes(s))
	wantSince(t, "a line drawn between calls", seen, last, true, false)
	last = seen
	seen = seenBy(t, func(ctx context.Context) error {
		_, err := s.Measure(ctx, Measuring{})
		return err
	})
	wantSince(t, "the call after", seen, last, false, false)
	last = seen

	// Someone else opens a file; then the model does.
	file := filepath.Join(t.TempDir(), "drawing.tlspr")
	if err := doc.File.Save(file); err != nil {
		t.Fatal(err)
	}
	if err := doc.File.Load(file); err != nil {
		t.Fatal(err)
	}
	seen = seenBy(t, describes(s))
	wantSince(t, "a file opened between calls", seen, last, true, true)
	last = seen
	seen = seenBy(t, func(ctx context.Context) error {
		_, err := s.Open(ctx, file)
		return err
	})
	wantSince(t, "the model's open", seen, last, false, false)

	// A call made without a watch costs nothing and tells nothing.
	if _, err := s.Describe(ctx, Describing{}); err != nil {
		t.Fatal(err)
	}
}

// The revision goes through the socket: a Remote's caller sees the
// changes of another caller of the same session as someone else's.
func TestRemoteRevision(t *testing.T) {
	t.Parallel()
	dir := socketDir(t)
	s := newSession(t)
	serve(t, dir, s)
	r := remote(dir)
	defer r.Close()

	last := seenBy(t, describes(r))
	if direct := seenBy(t, describes(s)); direct != last {
		t.Errorf("through the socket the revision is %+v, at the session %+v", last, direct)
	}
	seen := seenBy(t, runs(r, "doc.Sketch.Circle(V(0, 0, 0), 5)"))
	wantSince(t, "the caller's script", seen, last, false, false)
	last = seen
	// The other caller, here the session itself, draws.
	if _, err := s.Run(context.Background(), "doc.Sketch.Circle(V(0, 0, 0), 2)"); err != nil {
		t.Fatal(err)
	}
	seen = seenBy(t, describes(r))
	wantSince(t, "another caller's script", seen, last, true, false)
	last = seen
	seen = seenBy(t, func(ctx context.Context) error {
		_, err := r.Measure(ctx, Measuring{})
		return err
	})
	wantSince(t, "the call after", seen, last, false, false)
}

package sketch_test

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestRevisionMovesOnChangesOnly checks that the revision moves at a
// change, an undo and a redo, at a new file and a sketch that follows
// another, and not at a regeneration, an activation, a read of a group
// past the active one or an edit that was refused.
func TestRevisionMovesOnChangesOnly(t *testing.T) {
	t.Parallel()
	sk := bolt(t)
	sk.GenerateAll()
	at := sk.Revision()
	same := func(what string) {
		t.Helper()
		if got := sk.Revision(); got != at {
			t.Fatalf("%s moved the revision from %+v to %+v", what, at, got)
		}
	}
	moved := func(what string, drawing bool) {
		t.Helper()
		got := sk.Revision()
		if got.Change <= at.Change || (got.Drawing != at.Drawing) != drawing {
			t.Fatalf("%s: revision %+v after %+v", what, got, at)
		}
		at = got
	}

	sk.GenerateAll()
	same("a regeneration")
	first := sk.GroupOrder[1]
	sk.Active = first
	sk.MarkGroupDirty(first, true)
	sk.GenerateAll()
	same("an activation")
	sk.MakeThrough(sk.GroupOrder[len(sk.GroupOrder)-1])
	same("a read past the active group")
	if err := sk.UndoRememberFor(func() error { return errors.New("refused") }); err == nil {
		t.Fatal("the refusal was lost")
	}
	same("a refused edit")
	if sk.UndoUndo() {
		t.Fatal("an undo with nothing remembered")
	}
	same("an undo of nothing")

	sk.UndoRemember()
	sk.Units = 1 - sk.Units
	moved("a change", false)
	sk.UndoUndo()
	moved("an undo", false)
	sk.UndoRedo()
	moved("a redo", false)
	sk.NoteChange()
	moved("a drag", false)

	next := sketch.NewSketch()
	next.Follow(sk)
	if r := next.Revision(); r.Change <= at.Change || r.Drawing == at.Drawing {
		t.Fatalf("a sketch that follows %+v has %+v", at, r)
	}
	sk.NewFile()
	moved("a new file", true)
}

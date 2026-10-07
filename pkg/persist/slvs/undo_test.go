package slvs

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/tlspr"

	"github.com/telesapr/telesapr/pkg/core"
)

func fixture(name string) string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "../../../testdata", name)
}

// TestUndoRedoRegenerates changes a dimension, then undoes and redoes
// the change: as in C++, each pop restores what defines the sketch and
// regenerates the rest, so the saves match the originals exactly.
func TestUndoRedoRegenerates(t *testing.T) {
	doc, err := LoadFile(fixture("constraint/pt_pt_distance/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	before := string(tlspr.Encode(doc.Sketch, tlspr.Options{}))

	sk.UndoRemember()
	var c *core.Constraint
	for i := range sk.Constraints {
		if sk.Constraints[i].Type == core.ConstraintPtPtDistance {
			c = &sk.Constraints[i]
		}
	}
	if c == nil {
		t.Fatal("no distance constraint")
	}
	c.ValA *= 2
	sk.GenerateAll()
	after := string(tlspr.Encode(doc.Sketch, tlspr.Options{}))
	if after == before {
		t.Fatal("changing the dimension changed nothing")
	}

	if !sk.UndoUndo() || string(tlspr.Encode(doc.Sketch, tlspr.Options{})) != before {
		t.Errorf("undo didn't restore the original")
	}
	if sk.Undo.Cnt != 0 || sk.Redo.Cnt != 1 {
		t.Errorf("stacks: undo %d redo %d, want 0 1", sk.Undo.Cnt, sk.Redo.Cnt)
	}
	if !sk.UndoRedo() || string(tlspr.Encode(doc.Sketch, tlspr.Options{})) != after {
		t.Errorf("redo didn't restore the change")
	}
	if sk.UndoRedo() {
		t.Errorf("redo with nothing to redo")
	}

	// Remembering a new state forgets what could be redone.
	sk.UndoUndo()
	sk.UndoRemember()
	if sk.Redo.Cnt != 0 {
		t.Errorf("redo count %d after remembering, want 0", sk.Redo.Cnt)
	}
}

// TestUndoReloadsLinked: undo drops the linked groups' imported data
// and reloads it from the linked file, as C++ does.
func TestUndoReloadsLinked(t *testing.T) {
	doc, err := LoadFile(fixture("group/link/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	before := string(tlspr.Encode(doc.Sketch, tlspr.Options{}))
	doc.Sketch.UndoRemember()
	if !doc.Sketch.UndoUndo() {
		t.Fatal("nothing to undo")
	}
	if got := string(tlspr.Encode(doc.Sketch, tlspr.Options{})); got != before {
		t.Errorf("undo changed the linked sketch")
	}
}

// TestUndoStackWraps: the stacks hold MaxUndo states, dropping the
// oldest.
func TestUndoStackWraps(t *testing.T) {
	doc, err := LoadFile(fixture("constraint/pt_pt_distance/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	sk := doc.Sketch
	for range 105 {
		sk.UndoRemember()
	}
	if sk.Undo.Cnt != 100 || sk.Undo.Write != 5 {
		t.Errorf("undo count %d write %d, want 100 5", sk.Undo.Cnt, sk.Undo.Write)
	}
}

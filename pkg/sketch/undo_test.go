package sketch

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
)

func TestUndoRememberStoresState(t *testing.T) {
	s := NewSketch()
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 1}, Name: "g1"})
	s.Params = append(s.Params, core.Param{H: core.HParam{V: 1}})
	s.Params[0].Val = 42
	s.UndoRemember()
	if s.Undo.Cnt != 1 {
		t.Errorf("undo count: got %d want 1", s.Undo.Cnt)
	}
	if len(s.Undo.D[0].Groups) != 1 || s.Undo.D[0].Groups[0].Name != "g1" {
		t.Errorf("stored groups: got %+v", s.Undo.D[0].Groups)
	}
	if s.Undo.D[0].Params[0].Val != 42 {
		t.Errorf("stored param value: got %v want 42", s.Undo.D[0].Params[0].Val)
	}
}

func TestUndoUndoRestoresPreviousState(t *testing.T) {
	s := NewSketch()
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 1}, Name: "first"})
	s.UndoRemember()
	// Modify the sketch.
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 2}, Name: "second"})
	if len(s.Groups) != 2 {
		t.Fatalf("setup: groups=%d want 2", len(s.Groups))
	}
	if !s.UndoUndo() {
		t.Fatalf("UndoUndo returned false (no state to restore)")
	}
	if len(s.Groups) != 1 || s.Groups[0].Name != "first" {
		t.Errorf("after undo: groups=%+v", s.Groups)
	}
	if s.Redo.Cnt != 1 {
		t.Errorf("redo count: got %d want 1", s.Redo.Cnt)
	}
}

func TestUndoRedoRestoresUndoneState(t *testing.T) {
	s := NewSketch()
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 1}, Name: "first"})
	s.UndoRemember()
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 2}, Name: "second"})
	s.UndoUndo()
	if !s.UndoRedo() {
		t.Fatalf("UndoRedo returned false")
	}
	if len(s.Groups) != 2 || s.Groups[1].Name != "second" {
		t.Errorf("after redo: groups=%+v", s.Groups)
	}
}

func TestUndoRememberClearsRedo(t *testing.T) {
	s := NewSketch()
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 1}})
	s.UndoRemember()
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 2}})
	s.UndoUndo()
	if s.Redo.Cnt == 0 {
		t.Fatalf("setup: redo stack empty")
	}
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 3}})
	s.UndoRemember()
	if s.Redo.Cnt != 0 {
		t.Errorf("redo after new action: got %d want 0", s.Redo.Cnt)
	}
}

func TestUndoRoundTripPreservesAllFields(t *testing.T) {
	s := NewSketch()
	s.Groups = append(s.Groups, Group{H: core.HGroup{V: 1}, Name: "g1"})
	s.Entities = append(s.Entities, core.Entity{
		EntityBase: core.EntityBase{H: core.HEntity{V: 5}, Type: core.EntityPointIn3D},
	})
	p := core.Param{H: core.HParam{V: 3}}
	p.Val = 7
	s.Params = append(s.Params, p)
	s.Constraints = append(s.Constraints, core.Constraint{
		ConstraintBase: core.ConstraintBase{H: core.HConstraint{V: 9}, Type: core.ConstraintPointsCoincident},
	})
	s.Requests = append(s.Requests, Request{
		H: core.HRequest{V: 11}, Type: RequestLineSegment,
	})
	s.Styles = append(s.Styles, core.Style{
		H: core.HStyle{V: 4}, Name: "default",
	})
	s.UndoRemember()
	// Replace everything with empty.
	s.Groups = nil
	s.Entities = nil
	s.Params = nil
	s.Constraints = nil
	s.Requests = nil
	s.Styles = nil
	s.UndoUndo()
	if len(s.Groups) != 1 || s.Groups[0].Name != "g1" {
		t.Errorf("groups: %+v", s.Groups)
	}
	if len(s.Entities) != 1 {
		t.Errorf("entities count: %d", len(s.Entities))
	}
	if len(s.Params) != 1 || s.Params[0].Val != 7 {
		t.Errorf("params: %+v", s.Params)
	}
	if len(s.Constraints) != 1 {
		t.Errorf("constraints count: %d", len(s.Constraints))
	}
	if len(s.Requests) != 1 {
		t.Errorf("requests count: %d", len(s.Requests))
	}
	if len(s.Styles) != 1 {
		t.Errorf("styles count: %d", len(s.Styles))
	}
}

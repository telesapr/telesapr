package sketch

import (
	"github.com/spacesolve/spacesolve/pkg/core"
)

// MaxUndoStates is the maximum number of undo states retained.
const MaxUndoStates = 32

// UndoState is one snapshot of the sketch state for undo/redo.
type UndoState struct {
	Groups      []Group
	Entities    []core.Entity
	Params      []core.Param
	Requests    []Request
	Constraints []core.Constraint
	Styles      []core.Style
}

// UndoStack holds up to MaxUndoStates entries in a circular buffer.
type UndoStack struct {
	D     [MaxUndoStates]UndoState
	Cnt   int
	Write int
}

// UndoEnableMenus is a no-op in the headless Go port; the GUI
// tracks its own enable/disable state.
func (s *Sketch) UndoEnableMenus() {}

// UndoRemember pushes the current sketch state onto the undo stack
// and clears the redo stack. Called whenever a user action modifies
// the sketch.
func (s *Sketch) UndoRemember() {
	s.PushFromCurrentOnto(&s.Undo)
	s.UndoClearStack(&s.Redo)
	s.UndoEnableMenus()
}

// UndoUndo pops one state off the undo stack, pushes the current
// state onto the redo stack, and restores the popped state as the
// current sketch. Returns true if a state was restored.
func (s *Sketch) UndoUndo() bool {
	if s.Undo.Cnt <= 0 {
		return false
	}
	s.PushFromCurrentOnto(&s.Redo)
	s.PopOntoCurrentFrom(&s.Undo)
	s.UndoEnableMenus()
	return true
}

// UndoRedo pops one state off the redo stack, pushes the current
// state onto the undo stack, and restores the popped state as the
// current sketch. Returns true if a state was restored.
func (s *Sketch) UndoRedo() bool {
	if s.Redo.Cnt <= 0 {
		return false
	}
	s.PushFromCurrentOnto(&s.Undo)
	s.PopOntoCurrentFrom(&s.Redo)
	s.UndoEnableMenus()
	return true
}

// PushFromCurrentOnto writes the current sketch state into uk's
// write position, advancing the write pointer and incrementing
// count. If the stack is full, the oldest entry is overwritten.
func (s *Sketch) PushFromCurrentOnto(uk *UndoStack) {
	idx := uk.Write
	if uk.Cnt == MaxUndoStates {
		s.UndoClearState(&uk.D[idx])
	} else {
		uk.Cnt++
	}
	dst := &uk.D[idx]
	dst.Groups = append([]Group(nil), s.Groups...)
	dst.Entities = append([]core.Entity(nil), s.Entities...)
	dst.Params = append([]core.Param(nil), s.Params...)
	dst.Requests = append([]Request(nil), s.Requests...)
	dst.Constraints = append([]core.Constraint(nil), s.Constraints...)
	dst.Styles = append([]core.Style(nil), s.Styles...)
	uk.Write = (uk.Write + 1) % MaxUndoStates
}

// PopOntoCurrentFrom reads from the most recent entry of uk,
// advancing in reverse, and replaces the current sketch state with
// it. Skips empty (zero) entries.
func (s *Sketch) PopOntoCurrentFrom(uk *UndoStack) {
	for uk.Cnt > 0 {
		uk.Cnt--
		idx := (uk.Write - 1 - (MaxUndoStates-uk.Cnt-1) + MaxUndoStates) % MaxUndoStates
		// The most recently written entry is at (Write - 1) mod M;
		// for a circular buffer of size M with Cnt entries written,
		// the last-written entry is at (Write - 1 + M) mod M, and
		// each prior entry is at decreasing indices.
		_ = idx
		break
	}
	if uk.Cnt == 0 && uk.Write == 0 {
		// No valid state.
		return
	}
	// Pop the last-written entry.
	idx := (uk.Write - 1 + MaxUndoStates) % MaxUndoStates
	src := &uk.D[idx]
	uk.Write = idx
	s.Groups = append([]Group(nil), src.Groups...)
	s.Entities = append([]core.Entity(nil), src.Entities...)
	s.Params = append([]core.Param(nil), src.Params...)
	s.Requests = append([]Request(nil), src.Requests...)
	s.Constraints = append([]core.Constraint(nil), src.Constraints...)
	s.Styles = append([]core.Style(nil), src.Styles...)
}

// UndoClearStack clears all entries in uk.
func (s *Sketch) UndoClearStack(uk *UndoStack) {
	for i := range uk.D {
		s.UndoClearState(&uk.D[i])
	}
	uk.Cnt = 0
	uk.Write = 0
}

// UndoClearState releases any state held by a single entry.
func (s *Sketch) UndoClearState(ut *UndoState) {
	ut.Groups = nil
	ut.Entities = nil
	ut.Params = nil
	ut.Requests = nil
	ut.Constraints = nil
	ut.Styles = nil
}

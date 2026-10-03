package sketch

import (
	"maps"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file ports solvespace/src/undoredo.cpp: whenever the user
// changes something, the state is recorded and pushed on a stack, and
// undo pops it.

// MaxUndo is the depth of the undo and redo stacks (C++
// SolveSpaceUI::MAX_UNDO).
const MaxUndo = 100

// UndoState is one snapshot of the sketch: what can't be regenerated
// (C++ UndoState).
type UndoState struct {
	Groups      []Group
	GroupOrder  []core.HGroup
	Requests    []Request
	Constraints []core.Constraint
	Params      []core.Param
	Styles      []core.Style
	Parameters  []Parameter
	Configs     []Configuration
	ActiveConf  string
	Drawing     *Drawing
	Units       core.Unit
	Active      core.HGroup
}

// UndoStack is a ring of up to MaxUndo states (C++ UndoStack).
type UndoStack struct {
	D     [MaxUndo]UndoState
	Cnt   int
	Write int
}

func wrapUndo(v int) int { return ((v % MaxUndo) + MaxUndo) % MaxUndo }

// UndoRemember records the current state for undo, and forgets what
// could be redone (C++ SolveSpaceUI::UndoRemember; the unsaved flag and
// the menus are the caller's).
func (s *Sketch) UndoRemember() {
	s.PushFromCurrentOnto(&s.Undo)
	s.UndoClearStack(&s.Redo)
}

// UndoRememberFor records the current state for undo, as UndoRemember
// does, and then runs fn to change it. When fn fails, the record is
// dropped, so a refused edit leaves no undo step.
func (s *Sketch) UndoRememberFor(fn func() error) error {
	s.PushFromCurrentOnto(&s.Undo)
	if err := fn(); err != nil {
		s.Undo.Cnt--
		s.Undo.Write = wrapUndo(s.Undo.Write - 1)
		s.UndoClearState(&s.Undo.D[s.Undo.Write])
		return err
	}
	s.UndoClearStack(&s.Redo)
	return nil
}

// UndoUndo restores the last remembered state, keeping the current one
// for redo; false if there is none (C++ SolveSpaceUI::UndoUndo).
func (s *Sketch) UndoUndo() bool {
	if s.Undo.Cnt <= 0 {
		return false
	}

	s.PushFromCurrentOnto(&s.Redo)
	s.PopOntoCurrentFrom(&s.Undo)
	s.ClearBrokenReferences()
	return true
}

// UndoRedo restores the last undone state (C++ SolveSpaceUI::UndoRedo).
func (s *Sketch) UndoRedo() bool {
	if s.Redo.Cnt <= 0 {
		return false
	}

	s.PushFromCurrentOnto(&s.Undo)
	s.PopOntoCurrentFrom(&s.Redo)
	s.ClearBrokenReferences()
	return true
}

// PushFromCurrentOnto records the current state on uk, over its oldest
// one when it's full (C++ SolveSpaceUI::PushFromCurrentOnto).
func (s *Sketch) PushFromCurrentOnto(uk *UndoStack) {
	if uk.Cnt == MaxUndo {
		s.UndoClearState(&uk.D[uk.Write])
		// And then write in to this one again
	} else {
		uk.Cnt++
	}

	uk.D[uk.Write] = *s.definingState()
	uk.Write = wrapUndo(uk.Write + 1)
}

// definingState is a copy of what defines the sketch, as undo keeps it:
// the groups without what is made from them, the requests, constraints,
// solver values, styles, parameters and configurations.
func (s *Sketch) definingState() *UndoState {
	ut := &UndoState{}
	ut.Groups = make([]Group, 0, len(s.Groups))
	for _, src := range s.Groups {
		// Copy what defines the group; zero out all the dynamic stuff
		// that will get regenerated.
		dest := Group{
			Tag:                    src.Tag,
			H:                      src.H,
			Type:                   src.Type,
			Order:                  src.Order,
			OpA:                    src.OpA,
			OpB:                    src.OpB,
			Visible:                src.Visible,
			Suppress:               src.Suppress,
			SuppressAll:            src.SuppressAll,
			RelaxConstraints:       src.RelaxConstraints,
			AllowRedundant:         src.AllowRedundant,
			SuppressDofCalculation: src.SuppressDofCalculation,
			AllDimsReference:       src.AllDimsReference,
			Scale:                  src.Scale,
			DofCheckOk:             src.DofCheckOk,
			ActiveWorkplane:        src.ActiveWorkplane,
			ValA:                   src.ValA,
			ValB:                   src.ValB,
			ValC:                   src.ValC,
			ExprA:                  src.ExprA,
			ExprB:                  src.ExprB,
			ExprC:                  src.ExprC,
			ExprScale:              src.ExprScale,
			Subtype:                src.Subtype,
			SkipFirst:              src.SkipFirst,
			MirrorSolid:            src.MirrorSolid,
			BooleanFailed:          src.BooleanFailed,
			MeshCombine:            src.MeshCombine,
			ForceToMesh:            src.ForceToMesh,
			Remap:                  maps.Clone(src.Remap),
			LinkFile:               src.LinkFile,
			Color:                  src.Color,
			Predef:                 src.Predef,
			Edges:                  slices.Clone(src.Edges),
			SolidEdges:             slices.Clone(src.SolidEdges),
			SolidFaces:             slices.Clone(src.SolidFaces),
			EdgeRadii:              slices.Clone(src.EdgeRadii),
			Name:                   src.Name,
		}
		ut.Groups = append(ut.Groups, dest)
	}
	ut.GroupOrder = append([]core.HGroup(nil), s.GroupOrder...)
	ut.Requests = append([]Request(nil), s.Requests...)
	ut.Constraints = append([]core.Constraint(nil), s.Constraints...)
	ut.Params = append([]core.Param(nil), s.Params...)
	ut.Styles = append([]core.Style(nil), s.Styles...)
	ut.Parameters = append([]Parameter(nil), s.Parameters...)
	ut.Configs = cloneConfigurations(s.Configurations)
	ut.ActiveConf = s.ActiveConfiguration
	ut.Drawing = s.Drawing.Clone()
	ut.Units = s.Units
	ut.Active = s.Active

	return ut
}

// PopOntoCurrentFrom makes the last state on uk the current one, then
// reloads the linked files and regenerates (C++
// SolveSpaceUI::PopOntoCurrentFrom).
func (s *Sketch) PopOntoCurrentFrom(uk *UndoStack) {
	if uk.Cnt <= 0 {
		panic("Cannot pop from empty undo stack")
	}
	uk.Cnt--
	uk.Write = wrapUndo(uk.Write - 1)

	ut := &uk.D[uk.Write]

	// Free everything in the main copy of the program before replacing it
	for _, hg := range s.GroupOrder {
		s.GetGroup(hg).Clear()
	}

	// And then take the state from the undo list
	s.Groups = ut.Groups
	s.GroupOrder = ut.GroupOrder
	s.Requests = ut.Requests
	s.Constraints = ut.Constraints
	s.Params = ut.Params
	s.Styles = ut.Styles
	s.Parameters = ut.Parameters
	s.Configurations = ut.Configs
	s.ActiveConfiguration = ut.ActiveConf
	s.Drawing = ut.Drawing
	s.Units = ut.Units
	s.Active = ut.Active

	// It has been moved, so no need to free it.
	*ut = UndoState{}

	// And reset the state everywhere else in the program, since the
	// sketch just changed a lot.
	if s.ReloadAllLinked != nil {
		s.ReloadAllLinked(s)
	}
	s.GenerateAll()
}

// UndoClearStack forgets every state on uk (C++
// SolveSpaceUI::UndoClearStack).
func (s *Sketch) UndoClearStack(uk *UndoStack) {
	for uk.Cnt > 0 {
		uk.Write = wrapUndo(uk.Write - 1)
		uk.Cnt--
		s.UndoClearState(&uk.D[uk.Write])
	}
	*uk = UndoStack{} // for good measure
}

// UndoClearState forgets one state (C++ SolveSpaceUI::UndoClearState).
func (s *Sketch) UndoClearState(ut *UndoState) {
	*ut = UndoState{}
}

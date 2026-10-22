package sketch

import (
	"maps"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file ports solvespace/src/undoredo.cpp: whenever the user
// changes something, the state is recorded and pushed on a stack, and
// undo pops it.

// MaxUndo is the depth of the undo and redo stacks.
const MaxUndo = 100

// UndoState is one snapshot of the sketch: what can't be regenerated.
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
	Datums      []Datum
	Drawing     *Drawing
	Units       core.Unit
	Active      core.HGroup
}

// UndoStack is a ring of up to MaxUndo states.
type UndoStack struct {
	D     [MaxUndo]UndoState
	Cnt   int
	Write int
}

func wrapUndo(v int) int { return ((v % MaxUndo) + MaxUndo) % MaxUndo }

// UndoRemember records the current state for undo, and forgets what
// could be redone.
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
		s.UndoDropNewest(1)
		return err
	}
	s.UndoClearStack(&s.Redo)
	return nil
}

// UndoDropNewest forgets the n states remembered last for undo,
// leaving the sketch as it is: what they would take back joins the
// step remembered before them.
func (s *Sketch) UndoDropNewest(n int) {
	for ; n > 0 && s.Undo.Cnt > 0; n-- {
		s.Undo.Cnt--
		s.Undo.Write = wrapUndo(s.Undo.Write - 1)
		s.UndoClearState(&s.Undo.D[s.Undo.Write])
	}
}

// UndoPushedSince is how many states were remembered for undo since
// the stack's write position was write, for a change that remembers
// its own steps; 0 when the position went back, as an undo moves it.
func (s *Sketch) UndoPushedSince(write int) int {
	n := wrapUndo(s.Undo.Write - write)
	if n > MaxUndo/2 {
		return 0
	}
	return n
}

// UndoUndo restores the last remembered state, keeping the current one
// for redo; false if there is none.
func (s *Sketch) UndoUndo() bool {
	if s.Undo.Cnt <= 0 {
		return false
	}

	s.PushFromCurrentOnto(&s.Redo)
	s.PopOntoCurrentFrom(&s.Undo)
	s.ClearBrokenReferences()
	return true
}

// UndoRedo restores the last undone state.
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
// one when it's full.
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
	for i := range s.Groups {
		ut.Groups = append(ut.Groups, s.Groups[i].defining())
	}
	ut.GroupOrder = append([]core.HGroup(nil), s.GroupOrder...)
	ut.Requests = append([]Request(nil), s.Requests...)
	ut.Constraints = append([]core.Constraint(nil), s.Constraints...)
	ut.Params = append([]core.Param(nil), s.Params...)
	ut.Styles = append([]core.Style(nil), s.Styles...)
	ut.Parameters = append([]Parameter(nil), s.Parameters...)
	ut.Configs = cloneConfigurations(s.Configurations)
	ut.ActiveConf = s.ActiveConfiguration
	ut.Datums = slices.Clone(s.Datums)
	ut.Drawing = s.Drawing.Clone()
	ut.Units = s.Units
	ut.Active = s.Active

	return ut
}

// PopOntoCurrentFrom makes the last state on uk the current one, then
// reloads the linked files and regenerates.
func (s *Sketch) PopOntoCurrentFrom(uk *UndoStack) {
	if uk.Cnt <= 0 {
		panic("Cannot pop from empty undo stack")
	}
	uk.Cnt--
	uk.Write = wrapUndo(uk.Write - 1)

	ut := &uk.D[uk.Write]

	// Free everything in the main copy of the program before replacing it
	for _, hg := range s.GroupOrder {
		s.Group(hg).Clear()
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
	s.Datums = ut.Datums
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

// UndoClearStack forgets every state on uk.
func (s *Sketch) UndoClearStack(uk *UndoStack) {
	for uk.Cnt > 0 {
		uk.Write = wrapUndo(uk.Write - 1)
		uk.Cnt--
		s.UndoClearState(&uk.D[uk.Write])
	}
	*uk = UndoStack{} // for good measure
}

// UndoClearState forgets one state.
func (s *Sketch) UndoClearState(ut *UndoState) {
	*ut = UndoState{}
}

// defining is a copy of what defines the group, as undo keeps it, with
// all that is made from it zero: what a regeneration makes the group
// from, beside its entities and params.
func (g *Group) defining() Group {
	return Group{
		Tag:                    g.Tag,
		H:                      g.H,
		Type:                   g.Type,
		Order:                  g.Order,
		OpA:                    g.OpA,
		OpB:                    g.OpB,
		Visible:                g.Visible,
		Suppress:               g.Suppress,
		SuppressAll:            g.SuppressAll,
		RelaxConstraints:       g.RelaxConstraints,
		AllowRedundant:         g.AllowRedundant,
		SuppressDofCalculation: g.SuppressDofCalculation,
		AllDimsReference:       g.AllDimsReference,
		Scale:                  g.Scale,
		DofCheckOk:             g.DofCheckOk,
		ActiveWorkplane:        g.ActiveWorkplane,
		ValA:                   g.ValA,
		ValB:                   g.ValB,
		ValC:                   g.ValC,
		ExprA:                  g.ExprA,
		ExprB:                  g.ExprB,
		ExprC:                  g.ExprC,
		ExprScale:              g.ExprScale,
		Extent:                 g.Extent,
		ExprExtent:             g.ExprExtent,
		Subtype:                g.Subtype,
		SkipFirst:              g.SkipFirst,
		MirrorSolid:            g.MirrorSolid,
		DraftFaces:             slices.Clone(g.DraftFaces),
		Appearance:             g.Appearance,
		FaceAppearances:        slices.Clone(g.FaceAppearances),
		Density:                g.Density,
		BooleanFailed:          g.BooleanFailed,
		MeshCombine:            g.MeshCombine,
		ForceToMesh:            g.ForceToMesh,
		Remap:                  maps.Clone(g.Remap),
		LinkFile:               g.LinkFile,
		LinkAsSolid:            g.LinkAsSolid,
		LinkParts:              slices.Clone(g.LinkParts),
		LinkConfiguration:      g.LinkConfiguration,
		LinkParams:             slices.Clone(g.LinkParams),
		Color:                  g.Color,
		Predef:                 g.Predef,
		Edges:                  slices.Clone(g.Edges),
		SolidEdges:             slices.Clone(g.SolidEdges),
		SolidFaces:             slices.Clone(g.SolidFaces),
		LoftThrough:            slices.Clone(g.LoftThrough),
		LoftAlongSides:         g.LoftAlongSides,
		SweepUp:                g.SweepUp,
		EdgeRadii:              slices.Clone(g.EdgeRadii),
		Name:                   g.Name,
	}
}

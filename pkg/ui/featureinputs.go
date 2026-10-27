package ui

import (
	"errors"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// What a feature is built from, the edges a fillet rounds, the faces a
// shell takes away, a sweep's path, a loft's profiles, a mirror's
// plane, is changed from its group's screen (ROADMAP F7): [change]
// takes the selection as the new inputs, and [remove] on a row takes
// that one away, each one step of undo through the setter in
// pkg/sketch the scripts reach too. The inputs are picked on the
// solid before the feature, whose edges and faces are hovered only
// when the group before the feature is active: [change] with nothing
// suitable selected makes it active and says what to select.

// ScreenChangeInputs makes group v's inputs the selection.
func (tw *textWindow) ScreenChangeInputs(link int, v uint32) {
	tw.ss.changeFeatureInputs(core.HGroup{V: v})
}

// awaitingInputs reports whether the group screen shown is waiting
// for its inputs to be picked, when it stays while they are.
func (tw *textWindow) awaitingInputs() bool {
	return tw.shown.screen == screenGroupInfo && tw.shown.inputsFor.V != 0 && tw.shown.inputsFor == tw.shown.group
}

// changeInputsLabel is the [change] link's text on g's screen: while
// the screen waits for the inputs to be picked, it says so.
func (tw *textWindow) changeInputsLabel(g *sketch.Group) string {
	if tw.shown.inputsFor == g.H {
		return "[take the selection]"
	}
	return "[change]"
}

// ScreenRemoveInput takes row v, an edge, a piece of the path or a
// face, from the shown group's inputs.
func (tw *textWindow) ScreenRemoveInput(link int, v uint32) {
	ss := tw.ss
	hg := tw.shown.group
	err := ss.SK.UndoRememberFor(func() error {
		g := ss.SK.Group(hg)
		if g == nil {
			return errors.New("the group is gone")
		}
		return ss.SK.RemoveFeatureInput(g.H, int(v))
	})
	if err != nil {
		ss.Report(err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.GW.ClearSuper()
}

// changeFeatureInputs makes group hg's inputs the selection, as one
// step of undo; with nothing suitable selected, it activates the group
// before hg, on whose solid the inputs are picked, and says what to
// select.
func (ss *Application) changeFeatureInputs(hg core.HGroup) {
	sk := ss.SK
	gw := ss.GW
	g := sk.Group(hg)
	if g == nil {
		return
	}
	gw.GroupSelection()
	gs := &gw.gs
	set, what := featureInputsFromSelection(sk, g, gs)
	if set == nil {
		// The screen stays while the inputs are picked.
		ss.TW.shown.inputsFor = hg
		ss.activateBefore(g)
		ss.Error("Select %s, then click [take the selection].", what)
		return
	}
	ss.TW.shown.inputsFor = core.HGroup{}
	if err := sk.UndoRememberFor(set); err != nil {
		ss.Error("Can't change what %s is built from: %s.", g.DescriptionString(), err)
		return
	}
	sk.Unsaved = true
	ss.UndoEnableMenus()
	gw.ClearSelection()
	gw.ClearSuper()
}

// activateBefore makes the group before g active, when it isn't and
// can be, so that the solid g works on is the one shown and picked.
func (ss *Application) activateBefore(g *sketch.Group) {
	sk := ss.SK
	prev := g.PreviousGroup(sk)
	if prev == nil || sk.Active == prev.H || sk.SuppressedReason(prev.H) != "" {
		return
	}
	sk.Active = prev.H
	ss.groupActivate(prev)
	ss.GW.ClearSuper()
}

// featureInputsFromSelection is the change that makes g's inputs the
// selection gs, or nil and what the selection lacks.
func featureInputsFromSelection(sk *sketch.Sketch, g *sketch.Group, gs *groupedSelection) (func() error, string) {
	switch g.Type {
	case sketch.GroupFillet, sketch.GroupChamfer, sketch.GroupSweep, sketch.GroupPathPattern:
		edges, names := gs.entity, gs.solidEdge
		if g.Type == sketch.GroupPathPattern {
			names = nil
		}
		if len(edges) == 0 && len(names) == 0 {
			return nil, featureInputsWanted(g.Type)
		}
		return func() error { return sk.SetFeatureEdges(g.H, edges, names) }, ""
	case sketch.GroupShell:
		faces := make([]srf.FaceID, 0, len(gs.face)+len(gs.solidFace))
		for _, he := range gs.face {
			faces = append(faces, srf.FaceID(he.V))
		}
		faces = append(faces, gs.solidFace...)
		if len(faces) == 0 {
			return nil, featureInputsWanted(g.Type)
		}
		return func() error { return sk.SetShellFaces(g.H, faces) }, ""
	case sketch.GroupLoft:
		profiles := selectedSketchGroups(sk, gs, g.H)
		if len(profiles) < 2 {
			return nil, featureInputsWanted(g.Type)
		}
		return func() error { return sk.SetLoftProfiles(g.H, profiles) }, ""
	case sketch.GroupMirror:
		if gs.n != 1 || gs.workplanes+gs.faces != 1 {
			return nil, featureInputsWanted(g.Type)
		}
		plane := gs.entity[0]
		return func() error { return sk.SetMirrorPlane(g.H, plane) }, ""
	}
	return nil, "a feature whose inputs can be changed"
}

// featureInputsWanted says what a feature of type t is built from, for
// the message asking for a selection.
func featureInputsWanted(t sketch.GroupType) string {
	switch t {
	case sketch.GroupFillet:
		return "the edges to round, line segments and circles of the sketch or edges of the solid before the fillet"
	case sketch.GroupChamfer:
		return "the edges to cut, line segments and circles of the sketch or edges of the solid before the chamfer"
	case sketch.GroupSweep:
		return "the path to sweep along, line segments, arcs and cubics or edges of the solid before the sweep"
	case sketch.GroupPathPattern:
		return "the path to repeat along, line segments and arcs"
	case sketch.GroupShell:
		return "the faces to take away, of the solid before the shell"
	case sketch.GroupLoft:
		return "entities of two or more sketches, the profiles in the order of the history"
	case sketch.GroupMirror:
		return "a workplane or a planar face to reflect in"
	}
	return "its inputs"
}

// selectedSketchGroups are the groups of the selected entities and
// points, other than except, each once, in the order of the history.
func selectedSketchGroups(sk *sketch.Sketch, gs *groupedSelection, except core.HGroup) []core.HGroup {
	chosen := map[core.HGroup]bool{}
	for _, he := range slices.Concat(gs.entity, gs.point) {
		if e := sk.Entity(he); e != nil && e.Group != except {
			chosen[e.Group] = true
		}
	}
	var out []core.HGroup
	for _, hg := range sk.GroupOrder {
		if chosen[hg] {
			out = append(out, hg)
		}
	}
	return out
}

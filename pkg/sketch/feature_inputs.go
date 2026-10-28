package sketch

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A feature's inputs, the edges a fillet rounds, the faces a shell
// takes away, the path a sweep follows, can be changed after it is
// made (ROADMAP F7): each setter checks the new inputs as the
// feature's constructor does, tries the feature on a copy of the solid
// before it, and stores them only when the trial succeeds, so a change
// the kernel can't make is refused and nothing moves. What later
// groups refer to in the feature and the change takes away is reported
// as a broken reference by the next regeneration, as when an earlier
// group is deleted. The feature's source, the group it works on, is not
// changed this way: that would make it another feature.

// inputGroup is group hg when it is of type t, or an error saying why
// its what can't be changed.
func (sk *Sketch) inputGroup(hg core.HGroup, t GroupType, what string) (*Group, error) {
	g := sk.Group(hg)
	if g == nil {
		return nil, fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != t {
		return nil, fmt.Errorf("%s is not a %s: only a %s's %s can be changed", g.DescriptionString(), groupTypeName(t), groupTypeName(t), what)
	}
	return g, nil
}

// groupTypeName names a feature's type in messages.
func groupTypeName(t GroupType) string {
	switch t {
	case GroupFillet:
		return "fillet"
	case GroupChamfer:
		return "chamfer"
	case GroupShell:
		return "shell"
	case GroupSweep:
		return "sweep"
	case GroupPathPattern:
		return "pattern along a path"
	case GroupLoft:
		return "loft"
	case GroupMirror:
		return "mirror"
	default:
		return "group"
	}
}

// inputBefore says why what, made by group of, can't be an input of
// group hg: it is made at or after it. A zero of (an input of the
// references) is before everything.
func (sk *Sketch) inputBefore(hg, of core.HGroup, what string) error {
	if of.V == 0 {
		return nil
	}
	if of == hg || !sk.GroupsInOrder(of, hg) {
		return fmt.Errorf("%s is made at or after %s: what a feature is built from must come before it", what, sk.anyGroupName(hg))
	}
	return nil
}

// solidBefore is the group whose running solid is the model before g,
// the solid g's inputs are picked on, or nil when there is none.
func (sk *Sketch) solidBefore(g *Group) *Group {
	prev := g.PreviousGroup(sk)
	if prev == nil {
		return nil
	}
	return sk.solidAt(prev.H)
}

// dedup is xs with each value kept once, at its first place.
func dedup[T comparable](xs []T) []T {
	var out []T
	seen := map[T]bool{}
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// edgesBefore says why the edges, sketch entities and edges of the
// solid by name, can't be inputs of group hg: one doesn't exist or is
// made at or after it.
func (sk *Sketch) edgesBefore(hg core.HGroup, edges []core.HEntity, names []topo.Name) error {
	for _, he := range edges {
		if sk.Entity(he) == nil {
			return fmt.Errorf("entity %s doesn't exist", sk.entityName(he))
		}
		if err := sk.inputBefore(hg, sk.entityGroup(he), sk.entityName(he)); err != nil {
			return err
		}
	}
	for _, n := range names {
		for _, id := range []srf.FaceID{n.A, n.B} {
			if err := sk.inputBefore(hg, faceGroup(id), sk.SolidEdgeName(n)); err != nil {
				return err
			}
		}
	}
	return nil
}

// carryEdgeRadii are g's edge radii as they stand when its edges become
// edges and then solidEdges: each radius stays with its own edge, by
// identity, and the radius of an edge that is gone goes with it.
func carryEdgeRadii(g *Group, edges []core.HEntity, solidEdges []topo.Name) []EdgeRadius {
	return carryEdgeValues(g, g.EdgeRadii, edges, solidEdges)
}

// carryEdgeValues is carryEdgeRadii for any of g's lists of edges' own
// values (edgeValueLists).
func carryEdgeValues(g *Group, list []EdgeRadius, edges []core.HEntity, solidEdges []topo.Name) []EdgeRadius {
	n := len(g.Edges) + len(g.SolidEdges)
	at := make([]int, n)
	for i := range at {
		at[i] = -1
	}
	for i, he := range g.Edges {
		for k, x := range edges {
			if x == he {
				at[i] = k
				break
			}
		}
	}
	for i, name := range g.SolidEdges {
		for k, x := range solidEdges {
			if x == name {
				at[len(g.Edges)+i] = len(edges) + k
				break
			}
		}
	}
	var out []EdgeRadius
	for _, er := range list {
		if er.Edge < 0 || er.Edge >= n || at[er.Edge] < 0 {
			continue
		}
		er.Edge = at[er.Edge]
		out = append(out, er)
	}
	slices.SortFunc(out, func(a, b EdgeRadius) int { return cmp.Compare(a.Edge, b.Edge) })
	return out
}

// SetFeatureEdges is SetFilletEdges, SetChamferEdges, SetSweepPath or
// SetPatternPath for group hg, whichever it is: what the group's screen
// calls with the selection. The caller remembers the undo state.
func (sk *Sketch) SetFeatureEdges(hg core.HGroup, edges []core.HEntity, solidEdges []topo.Name) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	switch g.Type {
	case GroupFillet:
		return sk.SetFilletEdges(hg, edges, solidEdges)
	case GroupChamfer:
		return sk.SetChamferEdges(hg, edges, solidEdges)
	case GroupSweep:
		return sk.SetSweepPath(hg, edges, solidEdges)
	case GroupPathPattern:
		if len(solidEdges) > 0 {
			return errors.New("a pattern's path is drawn in sketches, not along the solid's edges")
		}
		return sk.SetPatternPath(hg, edges)
	}
	return fmt.Errorf("%s has no edges to change", g.DescriptionString())
}

// RemoveFeatureInput takes input i away from group hg: of a fillet,
// chamfer, sweep or pattern its edge or path piece i, counting Edges
// and then SolidEdges, as the group's screen lists them; of a shell
// its face i. Taking the last one away is refused, as the feature would
// have nothing to work on. The caller remembers the undo state.
func (sk *Sketch) RemoveFeatureInput(hg core.HGroup, i int) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	switch g.Type {
	case GroupFillet, GroupChamfer, GroupSweep, GroupPathPattern:
		if i < 0 || i >= len(g.Edges)+len(g.SolidEdges) {
			return fmt.Errorf("%s has no edge %d", g.DescriptionString(), i+1)
		}
		edges, names := slices.Clone(g.Edges), slices.Clone(g.SolidEdges)
		if i < len(edges) {
			edges = slices.Delete(edges, i, i+1)
		} else {
			i -= len(edges)
			names = slices.Delete(names, i, i+1)
		}
		return sk.SetFeatureEdges(hg, edges, names)
	case GroupShell:
		if i < 0 || i >= len(g.SolidFaces) {
			return fmt.Errorf("%s has no face %d", g.DescriptionString(), i+1)
		}
		return sk.SetShellFaces(hg, slices.Delete(slices.Clone(g.SolidFaces), i, i+1))
	}
	return fmt.Errorf("%s has no inputs to take away one of", g.DescriptionString())
}

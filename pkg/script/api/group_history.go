package api

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// GroupHistory holds what a group depends on and where it sits in the
// list of groups, as Features > Move Group does: g.History().
type GroupHistory struct{ g Group }

// DependsOn lists the groups this group refers to, in history order:
// the group it extrudes or copies, the workplane it is drawn in, and the
// groups whose entities its sketch and constraints name. The references
// are left out.
func (v GroupHistory) DependsOn() (groups []Group) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for _, dep := range sk.GroupDependencies(g.H) {
			if dep.Group.V != core.HGroupReferences {
				groups = append(groups, Group{g.d, dep.Group})
			}
		}
	})
	return groups
}

// UsedBy lists the groups that refer to this group directly, in
// history order.
func (v GroupHistory) UsedBy() (groups []Group) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for _, h := range sk.GroupDependents(g.H) {
			groups = append(groups, Group{g.d, h})
		}
	})
	return groups
}

// MoveBefore moves the group in the history to just before other, as
// one undo step. A move that would put a group ahead of a group it
// uses is refused with an error naming both, and nothing changes:
// an extrusion can't move before its sketch, nor a sketch after its
// extrusion. The references stay first.
func (v GroupHistory) MoveBefore(other Group) error {
	g := v.g
	if other.H.V == 0 {
		fail("no group to move before")
	}
	other.d = g.d
	g.d.read(func(sk *sketch.Sketch) { other.get(sk) })
	return g.move(other.H)
}

// MoveToEnd moves the group to the end of the history, as MoveBefore
// does. Only this group moves: the sketches it uses stay where they
// are, as they come before it already. MoveToEndWithSources moves them
// along.
func (v GroupHistory) MoveToEnd() error {
	g := v.g
	return g.move(core.HGroup{})
}

// OwnSources lists the group with the groups only it needs, in history
// order: those it uses, directly or through each other, that no other
// group uses, as an extrusion's sketch. These are what
// MoveToEndWithSources moves.
func (v GroupHistory) OwnSources() (groups []Group) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		g.get(sk)
		for _, h := range sk.OwnSources(g.H) {
			groups = append(groups, Group{g.d, h})
		}
	})
	return groups
}

// MoveToEndWithSources moves the group to the end of the history
// together with its OwnSources, keeping their order, as one undo step:
// an extrusion moves with its sketch. It returns the groups moved, in
// their new order. A move that would put one of them ahead of a group
// it uses, or another group ahead of one of them that it uses, is
// refused with an error naming both, and nothing changes.
func (v GroupHistory) MoveToEndWithSources() ([]Group, error) {
	g := v.g
	return g.moveWithSources(core.HGroup{})
}

// MoveWithSourcesBefore moves the group with its OwnSources to just
// before other, as MoveToEndWithSources moves them to the end.
func (v GroupHistory) MoveWithSourcesBefore(other Group) ([]Group, error) {
	g := v.g
	if other.H.V == 0 {
		fail("no group to move before")
	}
	other.d = g.d
	g.d.read(func(sk *sketch.Sketch) { other.get(sk) })
	return g.moveWithSources(other.H)
}

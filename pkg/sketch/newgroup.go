package sketch

import "github.com/telesapr/telesapr/pkg/core"

// This file holds the part of making a group that doesn't depend on
// the selection, which C++ does at the end of Group::MenuGroup, so that
// a script can make groups without a user interface.

// NewGroup is a group as C++ Group::MenuGroup starts one: visible,
// grey, at scale 1.
func NewGroup() Group {
	return Group{
		Visible: true,
		Color:   core.NewRgbaColor(100, 100, 100, 255),
		Scale:   1,
	}
}

// AddGroupAfterActive adds g right after the active group, moving the
// later groups down, and gives a group that makes a solid the colour
// of the solid so far (C++ Group::MenuGroup). The caller activates and
// generates it.
func (sk *Sketch) AddGroupAfterActive(g Group) core.HGroup {
	// Copy color from the previous mesh-contributing group.
	if g.IsMeshGroup() && len(sk.GroupOrder) > 0 {
		if running := sk.GetRunningMeshGroupFor(sk.Active); running != nil {
			g.Color = running.Color
		}
	}
	sk.BumpOrderAfterActive(&g)
	return sk.AddGroup(g)
}

// BumpOrderAfterActive gives g the order after the active group's and
// moves the groups after that one down.
func (sk *Sketch) BumpOrderAfterActive(g *Group) {
	afterActive := false
	for _, hg := range sk.GroupOrder {
		gi := sk.GetGroup(hg)
		if afterActive {
			gi.Order++
		}
		if gi.H == sk.Active {
			g.Order = gi.Order + 1
			afterActive = true
		}
	}
}

// RegenerationPending reports whether a regeneration is scheduled and
// hasn't run, so that what is read from the sketch may be stale.
func (sk *Sketch) RegenerationPending() bool { return sk.scheduledGenerateAll }

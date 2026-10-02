package ui

import (
	"github.com/spacesolve/spacesolve/pkg/core"
)

// MoveGroup moves group h one place earlier or later in the history,
// as one undo step, or says why it can't (Group > Move Group Earlier
// and Later, and the arrows of the group list).
func (ss *SolveSpaceUI) MoveGroup(h core.HGroup, later bool) {
	sk := ss.SK
	order := sk.GroupOrder
	i := -1
	for k, hg := range order {
		if hg == h {
			i = k
		}
	}
	g := sk.GetGroup(h)
	if i < 0 || g == nil {
		return
	}
	var before core.HGroup
	switch {
	case later && i+1 >= len(order):
		ss.Error("%s is the last group already.", g.DescriptionString())
		return
	case later && i+2 < len(order):
		before = order[i+2]
	case !later && i == 0:
		ss.Error("The references group stays first.")
		return
	case !later:
		before = order[i-1]
	}
	if err := sk.CheckMoveGroup(h, before); err != nil {
		ss.Error("Can't move the group: %v.", err)
		return
	}
	ss.UndoRemember()
	if err := sk.MoveGroup(h, before); err != nil {
		ss.Error("Can't move the group: %v.", err)
	}
	ss.ScheduleShowTW()
}

// ScreenMoveGroup moves a group of the group list earlier ('u') or
// later ('d').
func (tw *TextWindow) ScreenMoveGroup(link int, v uint32) {
	tw.ss.MoveGroup(core.HGroup{V: v}, link == 'd')
}

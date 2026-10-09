package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
)

// MoveGroup moves group h one place earlier or later in the history,
// as one undo step, or says why it can't (Group > Move Group Earlier
// and Later, and the arrows of the group list).
func (ss *Application) MoveGroup(h core.HGroup, later bool) {
	sk := ss.SK
	order := sk.GroupOrder
	i := -1
	for k, hg := range order {
		if hg == h {
			i = k
		}
	}
	g := sk.Group(h)
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

// MoveGroupToEnd moves group h to the end of the history with the
// groups only it needs, as an extrusion's sketch, as one undo step, or
// says why it can't (Features > Move Group to End).
func (ss *Application) MoveGroupToEnd(h core.HGroup) {
	sk := ss.SK
	if sk.Group(h) == nil {
		return
	}
	if err := sk.CheckMoveGroupWithSources(h, core.HGroup{}); err != nil {
		ss.Error("Can't move the group: %v.", err)
		return
	}
	ss.UndoRemember()
	if _, err := sk.MoveGroupWithSources(h, core.HGroup{}); err != nil {
		ss.Error("Can't move the group: %v.", err)
	}
	ss.ScheduleShowTW()
}

// ScreenMoveGroup moves a group of the group list earlier ('u') or
// later ('d').
func (tw *textWindow) ScreenMoveGroup(link int, v uint32) {
	tw.ss.MoveGroup(core.HGroup{V: v}, link == 'd')
}

// MenuBrokenReferences shows what regenerations deleted, from the menu.
func (ss *Application) MenuBrokenReferences() {
	ss.GW.ClearSelection()
	ss.TW.GoToScreen(screenBrokenReferences)
	ss.GW.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// ScreenShowBrokenReferences shows what regenerations deleted.
func (tw *textWindow) ScreenShowBrokenReferences(link int, v uint32) {
	tw.GoToScreen(screenBrokenReferences)
}

// ScreenClearBrokenReferences empties the list.
func (tw *textWindow) ScreenClearBrokenReferences(link int, v uint32) {
	tw.ss.SK.ClearBrokenReferences()
	tw.ss.ScheduleShowTW()
}

// ShowBrokenReferences is the screen of what the last regeneration
// that deleted something deleted, because what it used was gone, and
// of the stored expressions that can't be evaluated; each with a link
// to the group it is in, where that group is still there.
func (tw *textWindow) ShowBrokenReferences() {
	sk := tw.ss.SK
	groupLink := func(h core.HGroup, name string) {
		if sk.Group(h) != nil {
			tw.Printf(false, "     %Fl%Ll%D%fin %s%E", h.V, lf(tw.ScreenSelectGroup), name)
		}
	}
	tw.Printf(true, "%FtBROKEN REFERENCES%E")
	if len(sk.BrokenReferences) == 0 {
		tw.Printf(false, "%Ba   (none: nothing was deleted for want of what it used)")
	}
	for _, b := range sk.BrokenReferences {
		tw.Printf(false, "   %Fx%s%E", b.String())
		if b.Item != b.GroupName {
			groupLink(b.Group, b.GroupName)
		}
	}
	if len(sk.BrokenReferences) > 0 {
		tw.Printf(false, "   Undo the deletion to bring them back. %Fl%Lc%f[clear the list]%E",
			lf(tw.ScreenClearBrokenReferences))
	}

	if len(sk.ExprErrors) > 0 {
		tw.Printf(true, "%FtEXPRESSIONS THAT COULD NOT BE EVALUATED%E")
		for _, e := range sk.ExprErrors {
			tw.Printf(false, "   %Fx%s%E", e.Error())
			if g := sk.Group(e.Group); g != nil {
				groupLink(e.Group, g.DescriptionString())
			}
		}
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}

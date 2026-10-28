package ui

import (
	"fmt"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A thread is several groups. Its helix group's screen says what the
// thread is in place of the helix's own rows, which a thread sets and
// which, changed one by one, would fight it; the group list folds the
// thread's groups into one row; both act on the thread as one.

// showThreadSummary shows, at the top of the screen of group g, one of
// the groups of the thread whose helix group is helix, what that thread
// is and what can be done with it as one.
func (tw *textWindow) showThreadSummary(g *sketch.Group, helix core.HGroup) {
	ss := tw.ss
	sk := ss.SK
	h := sk.Group(helix)
	if g.H != helix {
		tw.Printf(true, " %Ftone of the groups of the screw thread%E %Fl%Ll%D%f%s%E   %Fl%Le%D%f[edit thread]%E",
			helix.V, lf(tw.ScreenSelectGroup), groupDescription(h), helix.V, lf(tw.ScreenEditThread))
		if g.Type == sketch.GroupHelix {
			tw.Printf(false, "   its helix follows the thread: edit the thread to change it")
		}
		return
	}
	o := h.Thread.ThreadOptions
	unit := ss.UnitName()
	what := "screw thread"
	if o.Cosmetic {
		// Drawn by the convention and kept on the part, not modelled.
		what = "cosmetic thread"
	}
	tw.Printf(true, " %Ft%s%E   %Fl%Le%D%f[edit thread]%E", what, helix.V, lf(tw.ScreenEditThread))

	hand := "right-hand"
	if o.Spec.LeftHand {
		hand = "left-hand"
	}
	if n := o.Spec.NumStarts(); n > 1 {
		hand += fmt.Sprintf(", %d starts", n)
	}
	kind := "external"
	if o.Internal {
		kind = "internal"
	}
	tw.Printf(false, "%Ba   %s, %s %s pitch, %s, %s %s long%s, %s", o.Spec.String(), ss.MmToString(o.Spec.Pitch, false), unit, hand,
		ss.MmToString(o.Length, false), unit, exprNote(o.LengthExpr), kind)

	runOut := "none"
	if o.RunOut > 0 {
		runOut = ss.MmToString(o.RunOut, false) + " " + unit
	}
	more := ""
	if o.Through {
		more += ", to the end of the part"
	}
	if o.Reverse {
		more += ", flipped"
	}
	if o.Chamfer {
		more += ", chamfered end"
	}
	if o.LoadFlankBack && o.Spec.Buttress() {
		more += ", load flank back"
	}
	fit, err := o.FitClearance()
	if err != nil {
		fit = o.Clearance
	}
	clearance := ss.MmToString(fit, false) + " " + unit
	if o.Class != "" {
		clearance += " (class " + o.Class + ")"
	}
	if o.Phase != 0 {
		more += ", phase " + ss.DegreeToString(o.Phase) + "°"
	}
	tw.Printf(false, "%Bd   crest %s, clearance %s, run-out %s%s", o.Crest.String(), clearance, runOut, more)

	suppress := "[suppress the thread]"
	if sk.ThreadSuppressed(helix) {
		suppress = "[bring the thread back]"
	}
	hs, _ := sk.ThreadMembers(helix)
	fold := "[show them in the group list]"
	if tw.threadOpen[helix] {
		fold = "[fold them in the group list]"
	}
	groups := fmt.Sprintf("%d groups", len(hs))
	if len(hs) == 1 {
		groups = "1 group"
	}
	tw.Printf(false, "%Ba   %s %Fl%Lo%D%f%s%E   %Fl%Ls%D%f%s%E   %Fl%Lx%D%f[delete the thread]%E",
		groups, helix.V, lf(tw.ScreenThreadAction), fold, helix.V, lf(tw.ScreenThreadAction), suppress,
		helix.V, lf(tw.ScreenThreadAction))
	// What became of it last, below the actions, which stay put.
	if made := tw.shown.thread.made; made.Group == helix {
		if made.Sound {
			tw.Printf(false, "%Bd   %Fsmade: the solid came out sound%E")
		} else {
			tw.Printf(false, "%Bd   %Fxmade: the solid has naked edges or self-intersections%E")
		}
	} else if failed := tw.threadFailed(helix); failed != "" {
		tw.Printf(false, "%Bd   %Fx%s%E", failed)
	}
}

// threadFailed says which of the thread's groups failed and how, or ""
// when none did.
func (tw *textWindow) threadFailed(helix core.HGroup) string {
	sk := tw.ss.SK
	hs, err := sk.ThreadMembers(helix)
	if err != nil {
		return ""
	}
	for _, h := range hs {
		g := sk.Group(h)
		switch {
		case g.BooleanFailed:
			return fmt.Sprintf("the Boolean of %s failed", groupDescription(g))
		case g.Problem.Msg != "":
			return fmt.Sprintf("%s: %s", groupDescription(g), g.Problem.Msg)
		}
	}
	return ""
}

// ScreenThreadAction acts on the thread whose helix group is v, as one:
// shows its groups in the group list or folds them ('o'), shows or
// hides them ('v'), moves it a group earlier ('u') or later ('d'),
// leaves it out of the model or brings it back ('s'), or deletes it
// ('x').
func (tw *textWindow) ScreenThreadAction(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	hg := core.HGroup{V: v}
	hs, err := sk.ThreadMembers(hg)
	if err != nil {
		ss.Report(err)
		return
	}
	switch link {
	case 'o':
		if tw.threadOpen == nil {
			tw.threadOpen = map[core.HGroup]bool{}
		}
		tw.threadOpen[hg] = !tw.threadOpen[hg]
	case 'v':
		ss.UndoRemember()
		if err := sk.SetThreadVisible(hg, !sk.ThreadVisible(hg)); err != nil {
			ss.Report(err)
		}
		ss.GenerateAll(sketch.GenerateDirty)
	case 'u', 'd':
		ss.MoveThread(hg, link == 'd')
	case 's':
		ss.UndoRemember()
		if err := sk.SetThreadSuppressed(hg, !sk.ThreadSuppressed(hg)); err != nil {
			ss.Report(err)
		}
		ss.GW.ClearSuper()
	case 'x':
		ss.UndoRemember()
		if _, err := sk.ClearThreadItems(hg); err != nil {
			ss.Report(err)
			break
		}
		ss.deleteGroups(hs)
	}
	ss.ScheduleShowTW()
}

// MoveThread moves the thread whose helix group is hg, all its groups
// together, past the group before or after it in the history, as one
// undo step, or says why it can't (the arrows of the group list).
func (ss *Application) MoveThread(hg core.HGroup, later bool) {
	sk := ss.SK
	prev, next, err := sk.ThreadNeighbours(hg)
	if err != nil {
		ss.Report(err)
		return
	}
	var before core.HGroup
	switch {
	case later && next.V == 0:
		ss.Error("%s is at the end already.", sk.Group(hg).DescriptionString())
		return
	case later:
		if at := slices.Index(sk.GroupOrder, next); at+1 < len(sk.GroupOrder) {
			before = sk.GroupOrder[at+1]
		}
	case prev.V == 0 || prev.V == core.HGroupReferences:
		ss.Error("The references group stays first.")
		return
	default:
		before = prev
	}
	if err := sk.CheckMoveThread(hg, before); err != nil {
		ss.Error("Can't move the thread: %v.", err)
		return
	}
	ss.UndoRemember()
	if err := sk.MoveThread(hg, before); err != nil {
		ss.Error("Can't move the thread: %v.", err)
	}
	ss.ScheduleShowTW()
}

// threadListRow is the group list's row of the thread whose helix group
// is helix and whose groups are members: one row for them all, which
// activates the thread's last group, shows or hides them all, opens the
// thread's screen, and moves the thread as one.
func (tw *textWindow) threadListRow(helix core.HGroup, members []core.HGroup, bgc byte, afterActive bool) {
	ss := tw.ss
	sk := ss.SK
	last := members[len(members)-1]
	active := slices.Contains(members, ss.GW.activeGroupH())
	shown, ok, warn := true, true, false
	failed := last
	for _, h := range members {
		g := sk.Group(h)
		shown = shown && g.Visible
		if !g.IsSolvedOkay() && ok {
			ok, failed = false, h
		}
		warn = warn || g.Problem.Msg != "" || g.BooleanFailed || g.LinkProblem != ""
	}
	var fgSolve byte
	switch {
	case !ok:
		fgSolve = 'x'
	case warn:
		fgSolve = 'm'
	default:
		fgSolve = 's'
	}
	solveText, errText := "ok ", ""
	if !ok {
		solveText, errText = "", "ERR"
	}
	fgName := byte('l')
	suffix := ""
	switch {
	case sk.ThreadSuppressed(helix):
		fgName, suffix = 'g', " (suppressed)"
	case sk.Group(last).SuppressedBy.V != 0:
		fgName = 'g'
		suffix = fmt.Sprintf(" (suppressed with g%03x)", sk.Group(last).SuppressedBy.V)
	}
	fold := "[+]"
	if tw.threadOpen[helix] {
		fold = "[−]"
	}
	pick := func(c bool, a, b string) string {
		if c {
			return a
		}
		return b
	}
	what := "screw thread"
	if sk.Group(helix).Thread.Cosmetic {
		what = "cosmetic thread"
	}
	groups := fmt.Sprintf("%d groups", len(members))
	if len(members) == 1 {
		groups = "1 group"
	}
	name := fmt.Sprintf("%s (%s, %s)", groupDescription(sk.Group(helix)), what, groups)
	tw.Printf(false, "%Bp%Fd "+
		"%Ft%Fb%D%f%Ll%s%E "+
		"%Fb%s%D%f%Lv%s%E  "+
		"%Fp%D%f%s%Ll%s%E "+
		"%Fp%Ll%D%f%s%E%s %Fl%Lo%D%f%s%E"+
		"  %Fl%Lu%D%f↑%E %Fl%Ld%D%f↓%E",
		bgc,
		last.V, lf(tw.ScreenActivateGroup), pick(active, " "+radioTrue+" ", " "+radioFalse+" "),
		pick(afterActive, " - ", ""), helix.V, lf(tw.ScreenThreadAction),
		pick(afterActive, "", pick(shown, " "+checkTrue+" ", " "+checkFalse+" ")),
		fgSolve, failed.V, lf(tw.ScreenHowGroupSolved), solveText, errText,
		fgName, helix.V, lf(tw.ScreenSelectGroup), name, suffix,
		helix.V, lf(tw.ScreenThreadAction), fold,
		helix.V, lf(tw.ScreenThreadAction), helix.V, lf(tw.ScreenThreadAction))
}

// deleteGroups deletes the groups hs, and with them what used them, as
// deleteGroup deletes one. When the active group goes, the last group
// before them that is left is made active.
func (ss *Application) deleteGroups(hs []core.HGroup) {
	sk := ss.SK
	order := slices.Clone(sk.GroupOrder)
	was := ss.GW.activeGroupH()
	if slices.Contains(hs, was) {
		if prev := sk.Group(hs[0]).PreviousGroup(sk); prev != nil {
			sk.Active = prev.H
		}
	}
	// Reset the text window, since it shows what is about to be
	// deleted.
	ss.TW.ClearSuper()
	sk.Groups = slices.DeleteFunc(sk.Groups, func(g sketch.Group) bool { return slices.Contains(hs, g.H) })
	ss.GenerateAll(sketch.GenerateAll)
	if sk.ActivateSurvivor(order, was) {
		ss.groupActivate(sk.Group(sk.Active))
	}
	// Reset the graphics window, which also makes the default group
	// again if it was deleted.
	ss.GW.ClearSuper()
}

package ui

// The text window's screens for the groups, step dimension and tangent
// arcs, and the edit control's dispatch (C++ textscreens.cpp).

import (
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/version"
)

// lf makes a method value a LinkFunction for Printf.
func lf(f func(link int, v uint32)) LinkFunction { return f }

// groupNumConstraints counts the constraints in group hg (C++
// Group::GetNumConstraints).
func groupNumConstraints(sk *sketch.Sketch, hg core.HGroup) int {
	n := 0
	for i := range sk.Constraints {
		if sk.Constraints[i].Group == hg {
			n++
		}
	}
	return n
}

// ScreenHome goes back to the list of groups (C++
// TextWindow::ScreenHome).
func (tw *TextWindow) ScreenHome(link int, v uint32) {
	tw.GoToScreen(ScreenListOfGroups)
}

// ShowHeader starts a screen with the navigation bar that always
// appears at the top of the window (C++ TextWindow::ShowHeader).
func (tw *TextWindow) ShowHeader(withNav bool) {
	ss := tw.ss
	tw.ClearScreen()

	var header, desc string
	if ss.GW.LockedInWorkplane() {
		header = "in plane: "
		desc = ss.entityDescription(ss.SK.MustGetEntity(ss.GW.ActiveWorkplane()))
	} else {
		header = "drawing / constraining in 3d"
		desc = ""
	}

	// Navigation buttons
	if withNav {
		tw.Printf(false, " %Fl%Lh%fhome%E   %Ft%s%E%s", lf(tw.ScreenHome), header, desc)
	} else {
		tw.Printf(false, "        %Ft%s%E%s", header, desc)
	}

	// Leave space for the icons that are painted here.
	tw.Printf(false, "")
	tw.Printf(false, "")
}

// ScreenSelectGroup shows a group's screen (C++
// TextWindow::ScreenSelectGroup).
func (tw *TextWindow) ScreenSelectGroup(link int, v uint32) {
	tw.ss.GW.MenuEdit(CmdUnselectAll)
	tw.GoToScreen(ScreenGroupInfo)
	tw.shown.group = core.HGroup{V: v}
}

// ScreenToggleGroupShown hides or shows a group (C++
// TextWindow::ScreenToggleGroupShown).
func (tw *TextWindow) ScreenToggleGroupShown(link int, v uint32) {
	g := tw.ss.SK.GetGroup(core.HGroup{V: v})
	g.Visible = !g.Visible
	// If a group was just shown, then it might not have been generated
	// previously, so regenerate.
	tw.ss.GenerateAll(sketch.GenerateDirty)
}

// ScreenShowGroupsSpecial shows all, the unconstrained or no groups
// (C++ TextWindow::ScreenShowGroupsSpecial).
func (tw *TextWindow) ScreenShowGroupsSpecial(link int, v uint32) {
	sk := tw.ss.SK
	for _, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)
		switch link {
		case 's':
			g.Visible = true
		case 'c':
			g.Visible = g.Solved.Dof != 0
		case 'h':
			g.Visible = false
		}
	}
	tw.ss.GW.persistentDirty = true
}

// ScreenActivateGroup makes a group active (C++
// TextWindow::ScreenActivateGroup).
func (tw *TextWindow) ScreenActivateGroup(link int, v uint32) {
	ss := tw.ss
	if why := ss.SK.SuppressedReason(core.HGroup{V: v}); why != "" {
		ss.Error("%s can't be made active: it is left out of the model (%s).",
			ss.SK.GetGroup(core.HGroup{V: v}).DescriptionString(), why)
		return
	}
	ss.SK.Active = core.HGroup{V: v}
	ss.groupActivate(ss.SK.GetGroup(ss.SK.Active))
	ss.GW.ClearSuper()
}

// ReportHowGroupSolved shows why a group failed to solve (C++
// TextWindow::ReportHowGroupSolved).
func (tw *TextWindow) ReportHowGroupSolved(hg core.HGroup) {
	tw.ss.GW.ClearSuper()
	tw.GoToScreen(ScreenGroupSolveInfo)
	tw.shown.group = hg
	tw.ss.ScheduleShowTW()
}

// ScreenHowGroupSolved activates a group and shows how it solved (C++
// TextWindow::ScreenHowGroupSolved).
func (tw *TextWindow) ScreenHowGroupSolved(link int, v uint32) {
	if tw.ss.GW.activeGroupH().V != v {
		tw.ScreenActivateGroup(link, v)
	}
	tw.GoToScreen(ScreenGroupSolveInfo)
	tw.shown.group = core.HGroup{V: v}
}

// ScreenShowConfiguration shows the configuration (C++
// TextWindow::ScreenShowConfiguration).
func (tw *TextWindow) ScreenShowConfiguration(link int, v uint32) {
	tw.GoToScreen(ScreenConfiguration)
}

// ScreenShowEditView shows the view parameters (C++
// TextWindow::ScreenShowEditView).
func (tw *TextWindow) ScreenShowEditView(link int, v uint32) {
	tw.GoToScreen(ScreenEditView)
}

// ScreenGoToWebsite opens the project's home (C++
// TextWindow::ScreenGoToWebsite).
func (tw *TextWindow) ScreenGoToWebsite(link int, v uint32) {
	tw.ss.Platform.OpenInBrowser(version.Repository)
}

// ShowListOfGroups is the home screen: every group in the sketch, with
// options to hide or show them, and to view them in detail (C++
// TextWindow::ShowListOfGroups).
func (tw *TextWindow) ShowListOfGroups() {
	ss := tw.ss
	sk := ss.SK
	const (
		rTrue  = " " + radioTrue + " "
		rFalse = " " + radioFalse + " "
		cTrue  = " " + checkTrue + " "
		cFalse = " " + checkFalse + " "
	)

	tw.Printf(true, "%Ft active")
	tw.Printf(false, "%Ft    shown dof group-name%E")
	afterActive := false
	backgroundParity := false
	for _, hg := range sk.GroupOrder {
		g := sk.GetGroup(hg)

		s := groupDescription(g)
		active := g.H == ss.GW.activeGroupH()
		shown := g.Visible
		ok := g.IsSolvedOkay()
		warn := (g.Type == sketch.GroupDrawingWorkplane && g.PolyError.How != int(core.PolyGood)) ||
			((g.Type == sketch.GroupExtrude || g.Type == sketch.GroupLathe) &&
				sk.GetGroup(g.OpA).PolyError.How != int(core.PolyGood)) ||
			g.Problem.Msg != "" || g.LinkProblem != ""
		dof := g.Solved.Dof
		sdof := "ok "
		if ok && dof > 0 {
			if dof > 999 {
				sdof = "###"
			} else {
				sdof = sprintf("%-3d", dof)
			}
		}
		suffix := ""
		if g.ForceToMesh || g.IsTriangleMeshAssembly() {
			suffix = " (∆)"
		}

		ref := g.H.V == core.HGroupReferences
		pick := func(c bool, a, b string) string {
			if c {
				return a
			}
			return b
		}
		bgc := byte('a')
		if backgroundParity {
			bgc = 'd'
		}
		var fgSolve byte
		switch {
		case !ok:
			fgSolve = 'x'
		case warn:
			fgSolve = 'm'
		case dof > 0:
			fgSolve = 'i'
		default:
			fgSolve = 's'
		}
		solveText := ""
		if ok {
			solveText = pick(warn && ss.CheckClosedContour, "err", sdof)
		}
		fgName := byte('l')
		if g.Suppress || g.SuppressedBy.V != 0 {
			fgName = 'g'
		}
		// A group left out says so, and what left it out.
		switch {
		case g.SuppressedBy == g.H:
			suffix += " (suppressed)"
		case g.SuppressedBy.V != 0:
			suffix += sprintf(" (suppressed with g%03x)", g.SuppressedBy.V)
		}
		format := "%Bp%Fd " +
			"%Ft%s%Fb%D%f%Ll%s%E " +
			"%Fb%s%D%f%Ll%s%E  " +
			"%Fp%D%f%s%Ll%s%E " +
			"%Fp%Ll%D%f%s%E%s"
		args := []any{
			// Alternate between light and dark backgrounds, for
			// readability
			bgc,
			// Link that activates the group
			pick(ref, "   ", ""),
			g.H.V, lf(tw.ScreenActivateGroup),
			pick(ref, "", pick(active, rTrue, rFalse)),
			// Link that hides or shows the group
			pick(afterActive, " - ", ""),
			g.H.V, lf(tw.ScreenToggleGroupShown),
			pick(afterActive, "", pick(shown, cTrue, cFalse)),
			// Link to the errors, if a problem occurred while solving
			fgSolve,
			g.H.V, lf(tw.ScreenHowGroupSolved),
			solveText,
			pick(ok, "", "ERR"),
			// Link to a screen that gives more details on the group
			fgName,
			g.H.V, lf(tw.ScreenSelectGroup), s,
			suffix,
		}
		// Arrows that move the group earlier and later.
		if !ref {
			format += "  %Fl%Lu%D%f↑%E %Fl%Ld%D%f↓%E"
			args = append(args, g.H.V, lf(tw.ScreenMoveGroup), g.H.V, lf(tw.ScreenMoveGroup))
		}
		tw.Printf(false, format, args...)

		if active {
			afterActive = true
		}
		backgroundParity = !backgroundParity
	}

	tw.Printf(true, "  %Fl%Ls%fshow all%E / %Fl%Lc%fonly unconstrained%E / %Fl%Lh%fhide all%E",
		lf(tw.ScreenShowGroupsSpecial),
		lf(tw.ScreenShowGroupsSpecial),
		lf(tw.ScreenShowGroupsSpecial))
	tw.Printf(true, "  %Fl%Ls%fline styles%E /"+
		" %Fl%Ls%fview%E /"+
		" %Fl%Ls%fconfiguration%E",
		lf(tw.ScreenShowListOfStyles),
		lf(tw.ScreenShowEditView),
		lf(tw.ScreenShowConfiguration))
	tw.Printf(false, "  %Fl%Lr%fparameters%E", lf(tw.ScreenShowParameters))
	if n := len(sk.BrokenReferences) + len(sk.ExprErrors); n > 0 {
		tw.Printf(false, "  %Fx%Lb%fbroken references (%d)%E", lf(tw.ScreenShowBrokenReferences), n)
	}
}

// ScreenHoverGroupWorkplane hovers a group's workplane (C++
// TextWindow::ScreenHoverGroupWorkplane).
func (tw *TextWindow) ScreenHoverGroupWorkplane(link int, v uint32) {
	gw := tw.ss.GW
	gw.hover.Clear()
	gw.hover.Entity = core.HGroup{V: v}.Entity(0)
	gw.hover.Emphasized = true
}

// ScreenHoverEntity hovers an entity (C++
// TextWindow::ScreenHoverEntity).
func (tw *TextWindow) ScreenHoverEntity(link int, v uint32) {
	gw := tw.ss.GW
	gw.hover.Clear()
	gw.hover.Entity = core.HEntity{V: v}
	gw.hover.Emphasized = true
}

// ScreenHoverRequest hovers a request's first entity (C++
// TextWindow::ScreenHoverRequest).
func (tw *TextWindow) ScreenHoverRequest(link int, v uint32) {
	gw := tw.ss.GW
	gw.hover.Clear()
	gw.hover.Entity = core.HRequest{V: v}.Entity(0)
	gw.hover.Emphasized = true
}

// ScreenHoverConstraint hovers a constraint (C++
// TextWindow::ScreenHoverConstraint).
func (tw *TextWindow) ScreenHoverConstraint(link int, v uint32) {
	gw := tw.ss.GW
	if gw.showConstraints == SCMNoShow {
		return
	}
	gw.hover.Clear()
	gw.hover.Constraint = core.HConstraint{V: v}
	gw.hover.Emphasized = true
}

// ScreenSelectEntity selects an entity (C++
// TextWindow::ScreenSelectEntity).
func (tw *TextWindow) ScreenSelectEntity(link int, v uint32) {
	gw := tw.ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Entity: core.HEntity{V: v}})
}

// ScreenSelectRequest selects a request's first entity (C++
// TextWindow::ScreenSelectRequest).
func (tw *TextWindow) ScreenSelectRequest(link int, v uint32) {
	gw := tw.ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Entity: core.HRequest{V: v}.Entity(0)})
}

// ScreenSelectConstraint selects a constraint (C++
// TextWindow::ScreenSelectConstraint).
func (tw *TextWindow) ScreenSelectConstraint(link int, v uint32) {
	gw := tw.ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Constraint: core.HConstraint{V: v}})
}

// ScreenChangeGroupOption flips one of a group's options (C++
// TextWindow::ScreenChangeGroupOption).
func (tw *TextWindow) ScreenChangeGroupOption(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	ss.UndoRemember()
	g := sk.GetGroup(tw.shown.group)

	switch link {
	case 's':
		if g.Subtype == sketch.GroupTwoSided {
			g.Subtype = sketch.GroupOneSided
		}
		if g.Subtype == sketch.GroupTwoSkewed {
			g.Subtype = sketch.GroupOneSkewed
		}
	case 'S':
		if g.Subtype == sketch.GroupOneSided {
			g.Subtype = sketch.GroupTwoSided
		}
		if g.Subtype == sketch.GroupOneSkewed {
			g.Subtype = sketch.GroupTwoSkewed
		}
	case 'w':
		switch g.Subtype {
		case sketch.GroupOneSided:
			g.Subtype = sketch.GroupOneSkewed
		case sketch.GroupTwoSided:
			g.Subtype = sketch.GroupTwoSkewed
		case sketch.GroupOneSkewed:
			g.Subtype = sketch.GroupOneSided
		case sketch.GroupTwoSkewed:
			g.Subtype = sketch.GroupTwoSided
		}

	case 'k':
		g.SkipFirst = true
	case 'K':
		g.SkipFirst = false
	case 'm':
		// A mirror of the whole solid so far, or of its source alone.
		g.MirrorSolid = !g.MirrorSolid

	case 'c':
		as := sketch.CombineAs(v)
		if g.Type == sketch.GroupExtrude {
			// When an extrude group is first created, it's positioned
			// for a union extrusion. If no constraints were added, flip
			// it when we switch between union/assemble and
			// difference/intersection modes to avoid manual work doing
			// the same.
			cut := func(c sketch.CombineAs) bool {
				return c == sketch.CombineDifference || c == sketch.CombineIntersection
			}
			if g.MeshCombine != as && groupNumConstraints(sk, g.H) == 0 {
				if cut(g.MeshCombine) != cut(as) {
					g.ExtrusionForceVectorTo(sk, g.ExtrusionGetVector(sk).Negated())
				}
			}
		}
		g.MeshCombine = as

	case 'P':
		g.Suppress = !g.Suppress

	case 'A':
		if err := sk.SetSuppressAll(g.H, !g.SuppressAll); err != nil {
			ss.Error("%v.", err)
		}

	case 'r':
		g.RelaxConstraints = !g.RelaxConstraints

	case 'e':
		g.AllowRedundant = !g.AllowRedundant

	case 'D':
		g.SuppressDofCalculation = !g.SuppressDofCalculation

	case 'v':
		g.Visible = !g.Visible

	case 'd':
		g.AllDimsReference = !g.AllDimsReference

	case 'f':
		g.ForceToMesh = !g.ForceToMesh
	}

	sk.MarkGroupDirty(g.H, false)
	ss.GW.ClearSuper()
}

// ScreenChangeLoftTurn turns a loft's second profile by one curve,
// either way (v is +1 or -1, as an unsigned word).
func (tw *TextWindow) ScreenChangeLoftTurn(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	g := sk.GetGroup(tw.shown.group)
	if g == nil || g.Type != sketch.GroupLoft {
		return
	}
	ss.UndoRemember()
	g.ValA += float64(int32(v))
	g.ExprA = ""
	sk.MarkGroupDirty(g.H, false)
	ss.GW.Invalidate(false)
}

// ScreenColor edits a group's colour (C++ TextWindow::ScreenColor).
func (tw *TextWindow) ScreenColor(link int, v uint32) {
	tw.ss.UndoRemember()

	g := tw.ss.SK.GetGroup(tw.shown.group)
	tw.ShowEditControlWithColorPicker(3, g.Color)
	tw.edit.meaning = EditGroupColor
}

// ScreenOpacity edits a group's opacity (C++ TextWindow::ScreenOpacity).
func (tw *TextWindow) ScreenOpacity(link int, v uint32) {
	g := tw.ss.SK.GetGroup(tw.shown.group)

	tw.ShowEditControl(11, sprintf("%.2f", g.Color.AlphaF()))
	tw.edit.meaning = EditGroupOpacity
	tw.edit.group = g.H
}

// ScreenChangeExprA edits how many times a group repeats (C++
// TextWindow::ScreenChangeExprA).
func (tw *TextWindow) ScreenChangeExprA(link int, v uint32) {
	g := tw.ss.SK.GetGroup(tw.shown.group)

	text := g.ExprA
	if text == "" {
		text = sprintf("%d", int(g.ValA))
	}
	tw.ShowEditControl(10, text)
	tw.edit.meaning = EditTimesRepeated
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeGroupName renames a group (C++
// TextWindow::ScreenChangeGroupName).
func (tw *TextWindow) ScreenChangeGroupName(link int, v uint32) {
	g := tw.ss.SK.GetGroup(tw.shown.group)
	tw.ShowEditControl(12, groupDescription(g)[5:])
	tw.edit.meaning = EditGroupName
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeGroupScale edits a linked group's scale (C++
// TextWindow::ScreenChangeGroupScale).
func (tw *TextWindow) ScreenChangeGroupScale(link int, v uint32) {
	g := tw.ss.SK.GetGroup(tw.shown.group)

	text := g.ExprScale
	if text == "" {
		text = sprintf("%.3f", g.Scale)
	}
	tw.ShowEditControl(13, text)
	tw.edit.meaning = EditGroupScale
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeHelixPitch edits a helix's pitch, a chamfer's distance
// or a fillet's radius, all ValB (C++ TextWindow::ScreenChangeHelixPitch).
func (tw *TextWindow) ScreenChangeHelixPitch(link int, v uint32) {
	g := tw.ss.SK.GetGroup(tw.shown.group)
	text := g.ExprB
	if text == "" {
		text = sprintf("%.8f", g.ValB/tw.ss.MmPerUnit())
	}
	tw.ShowEditControl(3, text)
	tw.edit.meaning = EditHelixPitch
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeLinkAsSolid reads a linked STEP file as a solid ('s') or
// as a reference mesh, and reads it again.
func (tw *TextWindow) ScreenChangeLinkAsSolid(link int, v uint32) {
	ss := tw.ss
	g := ss.SK.GetGroup(tw.shown.group)
	if g == nil || g.Type != sketch.GroupLinked || g.LinkAsSolid == (link == 's') {
		return
	}
	ss.UndoRemember()
	g.LinkAsSolid = link == 's'
	if ss.SK.ReloadAllLinked != nil {
		ss.SK.ReloadAllLinked(ss.SK)
	}
	// A mesh forces its group to one; a solid that was one no longer.
	if g.LinkAsSolid && len(g.ImpShell.Surface) > 0 {
		g.ForceToMesh = false
	}
	ss.SK.MarkGroupDirty(g.H, false)
	ss.GW.ClearSuper()
}

// ScreenChangeSweepUp holds a sweep's frames to an axis, by the link's
// letter, or ('n') lets them turn least.
func (tw *TextWindow) ScreenChangeSweepUp(link int, v uint32) {
	ss := tw.ss
	g := ss.SK.GetGroup(tw.shown.group)
	if g == nil || g.Type != sketch.GroupSweep {
		return
	}
	ss.UndoRemember()
	switch link {
	case 'x':
		g.SweepUp = geom.Vector{X: 1}
	case 'y':
		g.SweepUp = geom.Vector{Y: 1}
	case 'z':
		g.SweepUp = geom.Vector{Z: 1}
	default:
		g.SweepUp = geom.Vector{}
	}
	ss.SK.MarkGroupDirty(g.H, false)
	ss.GW.ClearSuper()
}

// ScreenChangeGroupDraft edits an extrusion's draft or a sweep's twist,
// in degrees.
func (tw *TextWindow) ScreenChangeGroupDraft(link int, v uint32) {
	g := tw.ss.SK.GetGroup(tw.shown.group)
	text := g.ExprC
	if text == "" {
		text = sprintf("%g", g.ValC)
	}
	tw.ShowEditControl(10, text)
	tw.edit.meaning = EditGroupDraft
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangePitchOption fixes or frees a helix's pitch (C++
// TextWindow::ScreenChangePitchOption).
func (tw *TextWindow) ScreenChangePitchOption(link int, v uint32) {
	sk := tw.ss.SK
	g := sk.GetGroup(tw.shown.group)
	if g.ValB == 0.0 {
		g.ValB = sk.MustGetParam(g.H.Param(7)).Val * math.Pi / sk.MustGetParam(g.H.Param(3)).Val
	} else {
		g.ValB = 0.0
	}
	g.ExprB = ""
	tw.ss.GW.Invalidate(false)
}

// ScreenDeleteGroup deletes the shown group (C++
// TextWindow::ScreenDeleteGroup).
func (tw *TextWindow) ScreenDeleteGroup(link int, v uint32) {
	tw.ss.UndoRemember()
	tw.ss.deleteGroup(tw.shown.group)
}

// deleteGroup deletes group hg, and with it what used it. When the
// active group goes, the last group before it that is left is made
// active, rather than the first one.
func (ss *SolveSpaceUI) deleteGroup(hg core.HGroup) {
	sk := ss.SK
	order := slices.Clone(sk.GroupOrder)
	was := ss.GW.activeGroupH()
	if hg == was {
		if prev := sk.GetGroup(hg).PreviousGroup(sk); prev != nil {
			sk.Active = prev.H
		}
	}

	// Reset the text window, since we're displaying information about
	// the group that's about to get deleted.
	ss.TW.ClearSuper()

	// This is a major change, so let's re-solve everything.
	for i := range sk.Groups {
		if sk.Groups[i].H == hg {
			sk.Groups = append(sk.Groups[:i], sk.Groups[i+1:]...)
			break
		}
	}
	ss.GenerateAll(sketch.GenerateAll)
	if sk.ActivateSurvivor(order, was) {
		ss.groupActivate(sk.GetGroup(sk.Active))
	}

	// Reset the graphics window. This will also recreate the default
	// group if it was removed.
	ss.GW.ClearSuper()
}

// relativePath names path relative to the folder of the sketch's file,
// or "" when it can't (C++ Path::RelativeTo(saveFile.Parent())).
func (ss *SolveSpaceUI) relativePath(path string) string {
	if path == "" || ss.SaveFile == "" || !filepath.IsAbs(path) {
		return ""
	}
	rel, err := filepath.Rel(filepath.Dir(ss.SaveFile), path)
	if err != nil {
		return ""
	}
	return rel
}

// linkDisplayPath is how a group or image names its file: the engine
// keeps what the file said, relative to the sketch, and the UI the
// absolute paths it chose; C++ shows them relative when it can.
func (ss *SolveSpaceUI) linkDisplayPath(path string) string {
	if rel := ss.relativePath(path); rel != "" {
		return rel
	}
	return path
}

// wrapWords breaks text into lines of at most width characters, at
// spaces where it can.
func wrapWords(text string, width int) []string {
	var lines []string
	for len([]rune(text)) > width {
		r := []rune(text)
		cut := width
		for cut > width/2 && r[cut] != ' ' {
			cut--
		}
		if r[cut] != ' ' {
			cut = width
		}
		lines = append(lines, strings.TrimRight(string(r[:cut]), " "))
		text = strings.TrimLeft(string(r[cut:]), " ")
	}
	return append(lines, text)
}

// ShowGroupInfo is the screen about one group, where the user edits it
// (C++ TextWindow::ShowGroupInfo).
func (tw *TextWindow) ShowGroupInfo() {
	ss := tw.ss
	sk := ss.SK
	g := sk.GetGroup(tw.shown.group)
	s := "???"

	check := func(b bool) string {
		if b {
			return checkTrue
		}
		return checkFalse
	}
	radio := func(b bool) string {
		if b {
			return radioTrue
		}
		return radioFalse
	}

	if tw.shown.group.V == core.HGroupReferences {
		tw.Printf(true, "%FtGROUP  %E%s", groupDescription(g))
		tw.showGroupItems(g)
		return
	}
	tw.Printf(true, "%FtGROUP  %E%s [%Fl%Ll%D%frename%E/%Fl%Ll%D%fdel%E]",
		groupDescription(g),
		g.H.V, lf(tw.ScreenChangeGroupName),
		g.H.V, lf(tw.ScreenDeleteGroup))

	if why := sk.SuppressedReason(g.H); why != "" {
		tw.Printf(false, " %Fxleft out of the model: %s%E", why)
	}

	switch g.Type {
	case sketch.GroupLathe:
		tw.Printf(true, " %Ftlathe plane sketch")
	case sketch.GroupExtrude, sketch.GroupRotate, sketch.GroupTranslate, sketch.GroupRevolve,
		sketch.GroupHelix:
		switch g.Type {
		case sketch.GroupExtrude:
			s = "extrude plane sketch"
		case sketch.GroupTranslate:
			s = "translate original sketch"
		case sketch.GroupHelix:
			s = "create helical extrusion"
		case sketch.GroupRotate:
			s = "rotate original sketch"
		case sketch.GroupRevolve:
			s = "revolve original sketch"
		}
		tw.Printf(true, " %Ft%s%E", s)

		one := g.Subtype == sketch.GroupOneSided || g.Subtype == sketch.GroupOneSkewed
		two := g.Subtype == sketch.GroupTwoSided || g.Subtype == sketch.GroupTwoSkewed
		skew := g.Subtype == sketch.GroupOneSkewed || g.Subtype == sketch.GroupTwoSkewed

		if g.Type == sketch.GroupExtrude {
			tw.Printf(false,
				"%Ba   %f%Ls%Fd%s one-sided%E  "+
					"%f%LS%Fd%s two-sided%E  "+
					"%f%Lw%Fd%s skewed%E",
				lf(tw.ScreenChangeGroupOption), radio(one),
				lf(tw.ScreenChangeGroupOption), radio(two),
				lf(tw.ScreenChangeGroupOption), check(skew))
			tw.Printf(false, "%Bd   %Ftdraft%E %s deg%s %Fl%Ll%f%D[change]%E",
				sprintf("%g", g.ValC), exprNote(g.ExprC), lf(tw.ScreenChangeGroupDraft), g.H.V)
		} else {
			tw.Printf(false,
				"%Ba   %f%Ls%Fd%s one-sided%E  "+
					"%f%LS%Fd%s two-sided%E",
				lf(tw.ScreenChangeGroupOption), radio(one),
				lf(tw.ScreenChangeGroupOption), radio(two))
		}

		if g.Type == sketch.GroupRotate || g.Type == sketch.GroupTranslate {
			if g.Subtype == sketch.GroupOneSided {
				skip := g.SkipFirst
				tw.Printf(false,
					"%Bd   %Ftstart  %f%LK%Fd%s with original%E  "+
						"%f%Lk%Fd%s with copy #1%E",
					lf(tw.ScreenChangeGroupOption), radio(!skip),
					lf(tw.ScreenChangeGroupOption), radio(skip))
			}

			times := int(g.ValA)
			bg := byte('d')
			if g.Subtype == sketch.GroupOneSided {
				bg = 'a'
			}
			plural := "s"
			if times == 1 {
				plural = ""
			}
			tw.Printf(false, "%Bp   %Ftrepeat%E %d time%s%s %Fl%Ll%D%f[change]%E",
				bg, times, plural, exprNote(g.ExprA), g.H.V, lf(tw.ScreenChangeExprA))
		}
	case sketch.GroupMirror:
		tw.Printf(true, " %Ftmirror original sketch and solid%E")
		pl := g.Predef.EntityB
		tw.Printf(false, "%Ba   %Ftin plane%E %Fl%Ll%D%f%h%s%E",
			pl.V, lf(tw.ScreenSelectEntity), lf(tw.ScreenHoverEntity), sk.EntityName(pl))
		tw.Printf(false, "%Bd   %Ftsolid  %f%LK%Fd%s with original%E  "+
			"%f%Lk%Fd%s reflection only%E",
			lf(tw.ScreenChangeGroupOption), radio(!g.SkipFirst),
			lf(tw.ScreenChangeGroupOption), radio(g.SkipFirst))
		tw.Printf(false, "%Ba   %f%Lm%Fd%s the whole solid so far%E",
			lf(tw.ScreenChangeGroupOption), check(g.MirrorSolid))
	case sketch.GroupChamfer:
		tw.Printf(true, " %Ftchamfer edges of the solid%E")
		tw.Printf(false, "%Ba   %Ftdistance%E %s%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(g.ValB, false), exprNote(g.ExprB), lf(tw.ScreenChangeHelixPitch), g.H.V)
		tw.showGroupEdges(g, "edges")
	case sketch.GroupLoft:
		if len(g.LoftThrough) == 0 {
			tw.Printf(true, " %Ftloft between two profiles%E")
			tw.Printf(false, "%Ba   %Ftfrom%E %s", sk.GroupName(g.OpA))
			tw.Printf(false, "%Bd   %Ftto%E   %s", sk.GroupName(g.OpB))
			tw.Printf(false, "%Ba   %Ftturn the second by%E %d %Fl%Ll%f%D[-1]%E %Fl%Ll%f%D[+1]%E",
				int(g.ValA), lf(tw.ScreenChangeLoftTurn), -1, lf(tw.ScreenChangeLoftTurn), 1)
			break
		}
		tw.Printf(true, " %Ftsmooth loft through %d profiles%E", len(g.LoftThrough)+2)
		for i, h := range g.LoftProfiles() {
			bg := byte('a')
			if i%2 == 1 {
				bg = 'd'
			}
			tw.Printf(false, "%Bp   %Ft%d%E %s", bg, i+1, sk.GroupName(h))
		}
		bg := byte('a')
		if len(g.LoftThrough)%2 == 1 {
			bg = 'd'
		}
		tw.Printf(false, "%Bp   %Ftturn those after the first by%E %d %Fl%Ll%f%D[-1]%E %Fl%Ll%f%D[+1]%E",
			bg, int(g.ValA), lf(tw.ScreenChangeLoftTurn), -1, lf(tw.ScreenChangeLoftTurn), 1)
	case sketch.GroupFillet:
		tw.Printf(true, " %Ftround edges of the solid%E")
		tw.Printf(false, "%Ba   %Ftradius%E %s%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(g.ValB, false), exprNote(g.ExprB), lf(tw.ScreenChangeHelixPitch), g.H.V)
		tw.showGroupEdges(g, "edges")
	case sketch.GroupShell:
		tw.Printf(true, " %Fthollow the solid to walls%E")
		tw.Printf(false, "%Ba   %Ftthickness%E %s%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(g.ValB, false), exprNote(g.ExprB), lf(tw.ScreenChangeHelixPitch), g.H.V)
		tw.Printf(false, "%Bd   %Ftfaces taken away%E")
		for i, id := range g.SolidFaces {
			bg := byte('a')
			if i%2 == 1 {
				bg = 'd'
			}
			tw.Printf(false, "%Bp     %s", bg, sk.FaceName(id))
		}
	case sketch.GroupSweep:
		tw.Printf(true, " %Ftsweep plane sketch along path%E")
		tw.showGroupEdges(g, "path")
		tw.Printf(false, "%Bd   %Fttwist%E %s deg%s %Fl%Ll%f%D[change]%E",
			sprintf("%g", g.ValC), exprNote(g.ExprC), lf(tw.ScreenChangeGroupDraft), g.H.V)
		up := g.SweepUp
		tw.Printf(false, "%Ba   %Ftframes%E "+
			"%f%Ln%Fd%s turn least%E  "+
			"%f%Lx%Fd%s held to x%E  "+
			"%f%Ly%Fd%s y%E  "+
			"%f%Lz%Fd%s z%E",
			lf(tw.ScreenChangeSweepUp), radio(up.Magnitude() == 0),
			lf(tw.ScreenChangeSweepUp), radio(up == geom.Vector{X: 1}),
			lf(tw.ScreenChangeSweepUp), radio(up == geom.Vector{Y: 1}),
			lf(tw.ScreenChangeSweepUp), radio(up == geom.Vector{Z: 1}))
	case sketch.GroupPathPattern:
		tw.Printf(true, " %Ftrepeat solid along path%E")
		tw.Printf(false, "%Ba   %Ftcopies%E %d%s %Fl%Ll%D%f[change]%E",
			int(g.ValA), exprNote(g.ExprA), g.H.V, lf(tw.ScreenChangeExprA))
		tw.showGroupEdges(g, "path")
	case sketch.GroupLinked:
		tw.Printf(true, " %Ftlink geometry from file%E")
		tw.Printf(false, "%Ba   '%s'", ss.linkDisplayPath(g.LinkFile))
		tw.Printf(false, "%Bd   %Ftscaled by%E %#%s %Fl%Ll%f%D[change]%E",
			g.Scale, exprNote(g.ExprScale), lf(tw.ScreenChangeGroupScale), g.H.V)
		if link.IsStep(g.LinkFile) {
			tw.Printf(false, "%Ba   %Ftread as%E %f%Lm%Fd%s reference mesh%E  %f%Ls%Fd%s solid%E",
				lf(tw.ScreenChangeLinkAsSolid), radio(!g.LinkAsSolid),
				lf(tw.ScreenChangeLinkAsSolid), radio(g.LinkAsSolid))
		}
		for _, line := range g.LinkInfo {
			tw.Printf(false, "   %Fd%s%E", line)
		}
		if g.LinkProblem != "" {
			for i, line := range wrapWords("Not all of the file was read: "+g.LinkProblem+".", MaxCols-6) {
				indent := "   "
				if i > 0 {
					indent = "     "
				}
				tw.Printf(false, "%s%Fx%s%E", indent, line)
			}
		}
	case sketch.GroupDrawing3D:
		tw.Printf(true, " %Ftsketch in 3d%E")
	case sketch.GroupDrawingWorkplane:
		tw.Printf(true, " %Ftsketch in new workplane%E")
	default:
		tw.Printf(true, "???")
	}
	tw.Printf(false, "")

	if g.Type == sketch.GroupHelix {
		tw.Printf(false, "%Ft pitch - length per turn%E")

		if math.Abs(g.ValB) != 0.0 {
			tw.Printf(false, "  %Ba %#%s %Fl%Ll%f%D[change]%E",
				g.ValB/ss.MmPerUnit(), exprNote(g.ExprB), lf(tw.ScreenChangeHelixPitch), g.H.V)
		} else {
			tw.Printf(false, "  %Ba %# %E",
				sk.MustGetParam(g.H.Param(7)).Val*math.Pi/
					(sk.MustGetParam(g.H.Param(3)).Val*ss.MmPerUnit()),
				lf(tw.ScreenChangeHelixPitch), g.H.V)
		}
		tw.Printf(false, "   %Fd%f%LP%s  fixed", lf(tw.ScreenChangePitchOption), check(g.ValB != 0))

		tw.Printf(false, "") // blank line
	}

	if g.Type == sketch.GroupExtrude || g.Type == sketch.GroupLathe || g.Type == sketch.GroupRevolve ||
		g.Type == sketch.GroupLinked || g.Type == sketch.GroupHelix || g.Type == sketch.GroupSweep {
		un := g.MeshCombine == sketch.CombineUnion
		diff := g.MeshCombine == sketch.CombineDifference
		intr := g.MeshCombine == sketch.CombineIntersection
		asy := g.MeshCombine == sketch.CombineAssemble

		tw.Printf(false, " %Ftsolid model as")
		tw.Printf(false, "%Ba   %f%D%Lc%Fd%s union%E  "+
			"%f%D%Lc%Fd%s assemble%E  ",
			lf(tw.ScreenChangeGroupOption), sketch.CombineUnion, radio(un),
			lf(tw.ScreenChangeGroupOption), sketch.CombineAssemble, radio(asy))
		tw.Printf(false, "%Ba   %f%D%Lc%Fd%s difference%E  "+
			"%f%D%Lc%Fd%s intersection%E  ",
			lf(tw.ScreenChangeGroupOption), sketch.CombineDifference, radio(diff),
			lf(tw.ScreenChangeGroupOption), sketch.CombineIntersection, radio(intr))
		// The appearance when the window shades (roadmap 19.2).
		app := g.Appearance
		if app == "" {
			app = "plastic"
		}
		line := "%Bd   %Ftappearance%E"
		var args []any
		for _, name := range render.MaterialNames {
			line += "  %f%Lp%D%Fd%s " + name + "%E"
			args = append(args, lf(tw.ScreenChangeAppearance), 'a', appearanceIndex(name), radio(app == name))
		}
		tw.Printf(false, line, args...)

		if g.Type == sketch.GroupExtrude || g.Type == sketch.GroupLathe ||
			g.Type == sketch.GroupRevolve || g.Type == sketch.GroupHelix || g.Type == sketch.GroupSweep {
			tw.Printf(false,
				"%Bd   %Ftcolor   %E%Bz  %Bd (%@, %@, %@) %f%D%Lf%Fl[change]%E",
				&g.Color,
				float64(g.Color.RedF()), float64(g.Color.GreenF()), float64(g.Color.BlueF()),
				lf(tw.ScreenColor), tw.lineTop(tw.rows-1)+2)
			tw.Printf(false, "%Bd   %Ftopacity%E %@ %f%Lf%Fl[change]%E",
				float64(g.Color.AlphaF()), lf(tw.ScreenOpacity))
		}

		tw.Printf(false, "   %Fd%f%LP%s  suppress this group's solid model",
			lf(tw.ScreenChangeGroupOption), check(g.Suppress))

		tw.Printf(false, "")
	}

	tw.Printf(false, " %f%Lv%Fd%s  show entities from this group",
		lf(tw.ScreenChangeGroupOption), check(g.Visible))
	tw.Printf(false, " %f%LA%Fd%s  suppress this group and the groups that use it",
		lf(tw.ScreenChangeGroupOption), check(g.SuppressAll))

	if !g.IsForcedToMeshBySource(sk) && !g.IsTriangleMeshAssembly() {
		tw.Printf(false, " %f%Lf%Fd%s  force NURBS surfaces to triangle mesh",
			lf(tw.ScreenChangeGroupOption), check(g.ForceToMesh))
	} else {
		tw.Printf(false, " (model already forced to triangle mesh)")
	}

	tw.Printf(true, " %f%Lr%Fd%s  relax constraints and dimensions",
		lf(tw.ScreenChangeGroupOption), check(g.RelaxConstraints))

	tw.Printf(false, " %f%Le%Fd%s  allow redundant constraints",
		lf(tw.ScreenChangeGroupOption), check(g.AllowRedundant))

	tw.Printf(false, " %f%LD%Fd%s  suppress dof calculation (improves solver performance)",
		lf(tw.ScreenChangeGroupOption), check(g.SuppressDofCalculation))

	tw.Printf(false, " %f%Ld%Fd%s  treat all dimensions as reference",
		lf(tw.ScreenChangeGroupOption), check(g.AllDimsReference))

	if g.Problem.Msg != "" {
		tw.Printf(false, "")
		switch {
		case g.HasDraft():
			tw.Printf(false, "%FxThe draft is left out: %s.%E", g.Problem.Msg)
		case g.Type == sketch.GroupDrawing3D || g.Type == sketch.GroupDrawingWorkplane:
			tw.Printf(false, "%FxNo solid can be made from it: %s.%E", g.Problem.Msg)
		default:
			tw.Printf(false, "%FxThe group adds no solid: %s.%E", g.Problem.Msg)
		}
	}

	if g.BooleanFailed {
		tw.Printf(false, "")
		tw.Printf(false, "The Boolean operation failed. It may be ")
		tw.Printf(false, "possible to fix the problem by choosing ")
		tw.Printf(false, "'force NURBS surfaces to triangle mesh'.")
	}

	tw.showGroupItems(g)
}

// showGroupEdges lists the edges of a chamfer or the path of a sweep,
// each one hovered and selected from its row.
func (tw *TextWindow) showGroupEdges(g *sketch.Group, what string) {
	sk := tw.ss.SK
	tw.Printf(false, "%Ba   %Ft%s%E", what)
	for i, he := range g.Edges {
		bg := byte('d')
		if i%2 == 1 {
			bg = 'a'
		}
		tw.Printf(false, "%Bp     %Fl%Ll%D%f%h%s%E", bg, he.V,
			lf(tw.ScreenSelectEntity), lf(tw.ScreenHoverEntity), sk.EntityName(he))
		tw.showEdgeRadius(g, i, bg)
	}
	for i, n := range g.SolidEdges {
		bg := byte('d')
		if (len(g.Edges)+i)%2 == 1 {
			bg = 'a'
		}
		for k, line := range wrapWords(sk.SolidEdgeName(n), MaxCols-8) {
			indent := "     "
			if k > 0 {
				indent = "       "
			}
			tw.Printf(false, "%Bp%s%s", bg, indent, line)
		}
		tw.showEdgeRadius(g, len(g.Edges)+i, bg)
	}
	if len(g.Edges) == 0 && len(g.SolidEdges) == 0 {
		tw.Printf(false, "%Bd     (none)")
	}
}

// showEdgeRadius shows, under a fillet's edge i, the radius of its own
// that it has, or a link to give it one.
func (tw *TextWindow) showEdgeRadius(g *sketch.Group, i int, bg byte) {
	if g.Type != sketch.GroupFillet {
		return
	}
	val, expr, own := g.EdgeRadius(i)
	if !own {
		tw.Printf(false, "%Bp       %Fl%Ll%f%D[own radius]%E", bg, lf(tw.ScreenChangeEdgeRadius), uint32(i))
		return
	}
	tw.Printf(false, "%Bp       %Ftradius%E %s%s %Fl%Ll%f%D[change]%E %Fl%Ll%f%D[the fillet's]%E", bg,
		tw.ss.MmToString(val, false), exprNote(expr), lf(tw.ScreenChangeEdgeRadius), uint32(i),
		lf(tw.ScreenResetEdgeRadius), uint32(i))
}

// ScreenChangeEdgeRadius edits the own radius of the shown fillet's
// edge v.
func (tw *TextWindow) ScreenChangeEdgeRadius(link int, v uint32) {
	g := tw.ss.SK.GetGroup(tw.shown.group)
	val, text, _ := g.EdgeRadius(int(v))
	if text == "" {
		text = sprintf("%.8f", val/tw.ss.MmPerUnit())
	}
	tw.ShowEditControl(9, text)
	tw.edit.meaning = EditEdgeRadius
	tw.edit.group = g.H
	tw.edit.i = int(v)
}

// ScreenResetEdgeRadius gives the shown fillet's edge v the fillet's
// radius again.
func (tw *TextWindow) ScreenResetEdgeRadius(link int, v uint32) {
	tw.ss.setEdgeRadiusText(tw.shown.group, int(v), "")
}

// setEdgeRadiusText gives a fillet's edge its own radius from typed
// text, or "" the fillet's, as one step of undo.
func (ss *SolveSpaceUI) setEdgeRadiusText(hg core.HGroup, i int, s string) bool {
	err := ss.SK.UndoRememberFor(func() error {
		return ss.SK.SetEdgeRadiusText(hg, i, s)
	})
	if err != nil {
		ss.Error("%s.", capitalize(err.Error()))
		return false
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.GW.Invalidate(false)
	return true
}

// showGroupItems lists a group's requests and constraints (the
// list_items part of C++ TextWindow::ShowGroupInfo).
func (tw *TextWindow) showGroupItems(g *sketch.Group) {
	sk := tw.ss.SK
	parity := func(a int) byte {
		if a&1 != 0 {
			return 'd'
		}
		return 'a'
	}

	tw.Printf(false, "")
	tw.Printf(false, "%Ft requests in group")

	a := 0
	for i := range sk.Requests {
		r := &sk.Requests[i]
		if r.Group == tw.shown.group {
			tw.Printf(false, "%Bp   %Fl%Ll%D%f%h%s%E",
				parity(a), r.H.V,
				lf(tw.ScreenSelectRequest), lf(tw.ScreenHoverRequest),
				r.DescriptionString())
			a++
		}
	}
	if a == 0 {
		tw.Printf(false, "%Ba   (none)")
	}

	a = 0
	tw.Printf(false, "")
	tw.Printf(false, "%Ft constraints in group (%d DOF)", g.Solved.Dof)
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Group == tw.shown.group {
			ref := ""
			if c.Reference {
				ref = "(ref)"
			}
			tw.Printf(false, "%Bp   %Fl%Ll%D%f%h%s%E %s",
				parity(a), c.H.V,
				lf(tw.ScreenSelectConstraint), lf(tw.ScreenHoverConstraint),
				c.DescriptionString(), ref)
			a++
		}
	}
	if a == 0 {
		tw.Printf(false, "%Ba   (none)")
	}
}

// ScreenAllowRedundant allows the shown group redundant constraints
// (C++ TextWindow::ScreenAllowRedundant).
func (tw *TextWindow) ScreenAllowRedundant(link int, v uint32) {
	ss := tw.ss
	ss.UndoRemember()

	g := ss.SK.GetGroup(tw.shown.group)
	g.AllowRedundant = true
	ss.SK.MarkGroupDirty(tw.shown.group, false)

	tw.shown.screen = ScreenGroupInfo
	tw.Show()
}

// ShowGroupSolveInfo is the screen displayed when the sketch fails to
// solve: what failed, and (if the problem is a singular Jacobian) the
// constraints that could be removed to fix it (C++
// TextWindow::ShowGroupSolveInfo).
func (tw *TextWindow) ShowGroupSolveInfo() {
	sk := tw.ss.SK
	g := sk.GetGroup(tw.shown.group)
	if g.IsSolvedOkay() {
		// Go back to the default group info screen
		tw.shown.screen = ScreenGroupInfo
		tw.Show()
		return
	}

	tw.Printf(true, "%FtGROUP   %E%s", groupDescription(g))
	switch g.Solved.How {
	case core.SolveDidntConverge:
		tw.Printf(true, "%FxSOLVE FAILED!%Fd unsolvable constraints")
		tw.Printf(true, "the following constraints are incompatible")

	case core.SolveRedundantDidntConverge:
		tw.Printf(true, "%FxSOLVE FAILED!%Fd unsolvable constraints")
		tw.Printf(true, "the following constraints are unsatisfied")

	case core.SolveRedundantOkay:
		tw.Printf(true, "%FxSOLVE FAILED!%Fd redundant constraints")
		tw.Printf(true, "remove any one of these to fix it")

	case core.SolveTooManyUnknowns:
		tw.Printf(true, "Too many unknowns in a single group!")
		return

	default:
		panic("ui: unexpected solve result")
	}

	for i, hc := range g.Solved.Remove {
		c := sk.GetConstraint(hc)
		if c == nil {
			continue
		}

		bg := byte('a')
		if i&1 != 0 {
			bg = 'd'
		}
		tw.Printf(false, "%Bp   %Fl%Ll%D%f%h%s%E %Fl%Lj%D%f%h[show]%E",
			bg, c.H.V, lf(tw.ScreenSelectConstraint), lf(tw.ScreenHoverConstraint),
			c.DescriptionString(), c.H.V, lf(tw.ScreenShowConstraint), lf(tw.ScreenHoverConstraint))
	}

	if g.Solved.Timeout {
		tw.Printf(true, "%FxSome items in list have been omitted%Fd")
		tw.Printf(false, "%Fxbecause the operation timed out.%Fd")
	}

	tw.Printf(true, "It may be possible to fix the problem ")
	tw.Printf(false, "by selecting Edit -> Undo.")

	if g.Solved.How == core.SolveRedundantOkay {
		tw.Printf(true, "It is possible to suppress this error ")
		tw.Printf(false, "by %Fl%f%Llallowing redundant constraints%E in ",
			lf(tw.ScreenAllowRedundant))
		tw.Printf(false, "this group.")
	}
}

// ScreenStepDimFinish edits the step dimension's final value (C++
// TextWindow::ScreenStepDimFinish).
func (tw *TextWindow) ScreenStepDimFinish(link int, v uint32) {
	tw.edit.meaning = EditStepDimFinish
	var editValue string
	if tw.stepDim.isDistance {
		editValue = tw.ss.MmToString(tw.stepDim.finish, true)
	} else {
		editValue = sprintf("%.3f", tw.stepDim.finish)
	}
	tw.ShowEditControl(12, editValue)
}

// ScreenStepDimSteps edits the number of steps (C++
// TextWindow::ScreenStepDimSteps).
func (tw *TextWindow) ScreenStepDimSteps(link int, v uint32) {
	tw.edit.meaning = EditStepDimSteps
	tw.ShowEditControl(12, sprintf("%d", tw.stepDim.steps))
}

// ScreenStepDimGo steps the dimension, solving at each step (C++
// TextWindow::ScreenStepDimGo).
func (tw *TextWindow) ScreenStepDimGo(link int, v uint32) {
	ss := tw.ss
	hc := tw.shown.constraint
	c := ss.SK.GetConstraint(hc)
	if c == nil {
		return
	}
	ss.UndoRemember()

	start, finish := c.ValA, tw.stepDim.finish
	tw.stepDim.time = getMilliseconds()
	tw.stepDim.step = 1

	if tw.stepDim.timer == nil {
		tw.stepDim.timer = ss.Platform.CreateTimer()
	}
	tw.stepDim.timer.SetOnTimeout(func() {
		sd := &ss.TW.stepDim
		if sd.step <= sd.steps {
			c := ss.SK.GetConstraint(hc)
			if c == nil {
				return
			}
			c.ValA = start + ((finish-start)*float64(sd.step))/float64(sd.steps)
			ss.SK.MarkGroupDirty(c.Group, false)
			ss.GenerateAll(sketch.GenerateDirty)
			if !ss.ActiveGroupsOkay() {
				// Failed to solve, so quit
				return
			}
			sd.step++

			const stepMillis = 50
			time := getMilliseconds()
			if time-sd.time < stepMillis {
				sd.timer.RunAfterNextFrame()
			} else {
				sd.timer.RunAfter(int(time - sd.time - stepMillis))
			}
			sd.time = time
		} else {
			ss.TW.GoToScreen(ScreenListOfGroups)
			ss.ScheduleShowTW()
		}
		ss.GW.Invalidate(false)
	})
	tw.stepDim.timer.RunAfterNextFrame()
}

// ShowStepDimension is the screen to step a dimension from its value to
// a final one (C++ TextWindow::ShowStepDimension).
func (tw *TextWindow) ShowStepDimension() {
	ss := tw.ss
	c := ss.SK.GetConstraint(tw.shown.constraint)
	if c == nil {
		tw.shown.screen = ScreenListOfGroups
		tw.Show()
		return
	}

	tw.Printf(true, "%FtSTEP DIMENSION%E %s", c.DescriptionString())

	if tw.stepDim.isDistance {
		tw.Printf(true, "%Ba   %Ftstart%E    %s", ss.MmToString(c.ValA, false))
		tw.Printf(false, "%Bd   %Ftfinish%E   %s %Fl%Ll%f[change]%E",
			ss.MmToString(tw.stepDim.finish, false), lf(tw.ScreenStepDimFinish))
	} else {
		tw.Printf(true, "%Ba   %Ftstart%E    %@", c.ValA)
		tw.Printf(false, "%Bd   %Ftfinish%E   %@ %Fl%Ll%f[change]%E",
			tw.stepDim.finish, lf(tw.ScreenStepDimFinish))
	}
	tw.Printf(false, "%Ba   %Ftsteps%E    %d %Fl%Ll%f%D[change]%E",
		tw.stepDim.steps, lf(tw.ScreenStepDimSteps), 0)

	tw.Printf(true, " %Fl%Ll%fstep dimension now%E", lf(tw.ScreenStepDimGo))

	tw.Printf(true, "(or %Fl%Ll%fcancel operation%E)", lf(tw.ScreenHome))
}

// ScreenChangeTangentArc edits the tangent arc options (C++
// TextWindow::ScreenChangeTangentArc).
func (tw *TextWindow) ScreenChangeTangentArc(link int, v uint32) {
	ss := tw.ss
	switch link {
	case 'r':
		tw.edit.meaning = EditTangentArcRadius
		tw.ShowEditControl(3, ss.MmToString(ss.TangentArcRadius, true))

	case 'a':
		ss.TangentArcManual = !ss.TangentArcManual
	case 'm':
		ss.TangentArcModify = !ss.TangentArcModify
	}
}

// ShowTangentArc is the screen for the tangent arc options (C++
// TextWindow::ShowTangentArc).
func (tw *TextWindow) ShowTangentArc() {
	ss := tw.ss
	tw.Printf(true, "%FtTANGENT ARC PARAMETERS%E")

	tw.Printf(true, "%Ft radius of created arc%E")
	if ss.TangentArcManual {
		tw.Printf(false, "%Ba   %s %Fl%Lr%f[change]%E",
			ss.MmToString(ss.TangentArcRadius, false), lf(tw.ScreenChangeTangentArc))
	} else {
		tw.Printf(false, "%Ba   automatic")
	}

	check := func(b bool) string {
		if b {
			return checkTrue
		}
		return checkFalse
	}
	tw.Printf(false, "")
	tw.Printf(false, "  %Fd%f%La%s  choose radius automatically%E",
		lf(tw.ScreenChangeTangentArc), check(!ss.TangentArcManual))
	tw.Printf(false, "  %Fd%f%Lm%s  modify original entities%E",
		lf(tw.ScreenChangeTangentArc), check(ss.TangentArcModify))

	tw.Printf(false, "")
	tw.Printf(false, "To create a tangent arc at a point,")
	tw.Printf(false, "select that point and then choose")
	tw.Printf(false, "Sketch -> Tangent Arc at Point.")
	tw.Printf(true, "(or %Fl%Ll%fback to home screen%E)", lf(tw.ScreenHome))
}

// atoi is C atoi: leading space, a sign and digits; 0 without any.
func atoi(s string) int {
	s = strings.TrimLeft(s, " \t\n\r\f\v")
	end := 0
	if end < len(s) && (s[end] == '+' || s[end] == '-') {
		end++
	}
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	v, _ := strconv.ParseInt(s[:end], 10, 64)
	return int(int32(v))
}

// atof is C atof: the longest prefix that is a number; 0 without one.
func atof(s string) float64 {
	v, _ := strtod(s)
	return v
}

// strtod parses the longest number prefix of s after leading space, as
// C strtod does for decimal input, and reports how many bytes it used.
func strtod(s string) (float64, int) {
	i := 0
	for i < len(s) && strings.IndexByte(" \t\n\r\f\v", s[i]) >= 0 {
		i++
	}
	start := i
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	lower := strings.ToLower(s[i:])
	for _, w := range []string{"infinity", "inf", "nan"} {
		if strings.HasPrefix(lower, w) {
			v, _ := strconv.ParseFloat(s[start:i+len(w)], 64)
			return v, i + len(w)
		}
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0, 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	v, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil {
		// Out of range: C gives ±HUGE_VAL or 0, as ParseFloat does.
		return v, i
	}
	return v, i
}

// scanVector3 is sscanf(s, "%lf, %lf, %lf", ...) == 3.
func scanVector3(s string) (x, y, z float64, ok bool) {
	var vals [3]float64
	rest := s
	for k := range 3 {
		if k > 0 {
			// The ", " of the format: a literal comma, then any space.
			rest = strings.TrimLeft(rest, " \t\n\r\f\v")
			if !strings.HasPrefix(rest, ",") {
				return 0, 0, 0, false
			}
			rest = rest[1:]
		}
		v, n := strtod(rest)
		if n == 0 {
			return 0, 0, 0, false
		}
		vals[k] = v
		rest = rest[n:]
	}
	return vals[0], vals[1], vals[2], true
}

// exprNote is how a screen shows that a value comes from an
// expression: " (= n+1)", or nothing for a plain number.
func exprNote(expr string) string {
	if expr == "" {
		return ""
	}
	return " (= " + expr + ")"
}

// setGroupText gives a group value typed text, as one undo step, and
// reports whether it was taken; a refusal is shown as C++ shows its own.
func (ss *SolveSpaceUI) setGroupText(hg core.HGroup, f sketch.GroupValue, s string) bool {
	err := ss.SK.UndoRememberFor(func() error {
		return ss.SK.SetGroupText(hg, f, s)
	})
	if err != nil {
		ss.Error("%s.", capitalize(err.Error()))
		return false
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	return true
}

// EditControlDone handles the text entered in the edit control (C++
// TextWindow::EditControlDone).
func (tw *TextWindow) EditControlDone(s string) {
	ss := tw.ss
	sk := ss.SK
	tw.edit.showAgain = false

	switch tw.edit.meaning {
	case EditTimesRepeated:
		if e := ss.exprFromString(s, true); e != nil {
			g := sk.GetGroup(tw.edit.group)
			if !ss.setGroupText(g.H, sketch.GroupCopies, s) {
				break
			}
			if g.Type == sketch.GroupRotate {
				// If the group does not contain any constraints, then
				// set the numerical guess to space the copies uniformly
				// over one rotation. Don't touch the guess if we're
				// already constrained, because that would break
				// convergence.
				if groupNumConstraints(sk, g.H) == 0 {
					copies := g.ValA
					if g.SkipFirst {
						copies = g.ValA + 1
					}
					sk.MustGetParam(g.H.Param(3)).Val = math.Pi / (2 * copies)
				}
			}
		}

	case EditGroupName:
		if s == "" {
			ss.Error("Group name cannot be empty")
		} else {
			ss.UndoRemember()

			g := sk.GetGroup(tw.edit.group)
			g.Name = s
		}

	case EditGroupScale:
		if e := ss.exprFromString(s, true); e != nil {
			ss.setGroupText(tw.edit.group, sketch.GroupScale, s)
		}

	case EditHelixPitch: // stored in valB
		if e := ss.exprFromString(s, true); e != nil {
			ss.setGroupText(tw.edit.group, sketch.GroupPitch, s)
		}

	case EditGroupDraft:
		if e := ss.exprFromString(s, true); e != nil {
			ss.setGroupText(tw.edit.group, sketch.GroupValC, s)
		}

	case EditEdgeRadius:
		if e := ss.exprFromString(s, true); e != nil {
			ss.setEdgeRadiusText(tw.edit.group, tw.edit.i, s)
		}

	case EditGroupColor:
		if x, y, z, ok := scanVector3(s); ok {
			rgb := vec3(x, y, z).ClampWithin(0, 1)

			g := sk.GetGroup(tw.shown.group)
			if g == nil {
				break
			}
			g.Color = core.RgbaColorFromFloat(float32(rgb.X), float32(rgb.Y), float32(rgb.Z),
				g.Color.AlphaF())

			sk.MarkGroupDirty(g.H, false)
			ss.GW.ClearSuper()
		} else {
			ss.Error("Bad format: specify color as r, g, b")
		}

	case EditGroupOpacity:
		if e := ss.exprFromString(s, true); e != nil {
			alpha := ss.evalExpr(e)
			if alpha < 0 || alpha > 1 {
				ss.Error("Opacity must be between zero and one.")
			} else {
				g := sk.GetGroup(tw.edit.group)
				g.Color.Alpha = uint8(int(float64(float32(255.1)) * alpha))
				sk.MarkGroupDirty(g.H, false)
				ss.GW.ClearSuper()
			}
		}

	case EditTTFText:
		ss.UndoRemember()
		if r := sk.GetRequest(tw.edit.request); r != nil {
			r.Str = s
			sk.MarkGroupDirty(r.Group, false)
		}

	case EditStepDimFinish:
		if e := ss.exprFromString(s, true); e != nil {
			if tw.stepDim.isDistance {
				tw.stepDim.finish = ss.ExprToMm(e)
			} else {
				tw.stepDim.finish = ss.evalExpr(e)
			}
		}

	case EditStepDimSteps:
		tw.stepDim.steps = min(300, max(1, atoi(s)))

	case EditTangentArcRadius:
		if e := ss.exprFromString(s, true); e != nil {
			if ss.evalExpr(e) < core.LengthEps {
				ss.Error("Radius cannot be zero or negative.")
				break
			}
			ss.TangentArcRadius = ss.ExprToMm(e)
		}

	default:
		cnt := 0
		if tw.EditControlDoneForStyles(s) {
			cnt++
		}
		if tw.EditControlDoneForConfiguration(s) {
			cnt++
		}
		if tw.EditControlDoneForPaste(s) {
			cnt++
		}
		if tw.EditControlDoneForView(s) {
			cnt++
		}
		if tw.EditControlDoneForThread(s) {
			cnt++
		}
		if tw.EditControlDoneForParameters(s) {
			cnt++
		}
		if tw.EditControlDoneForCommands(s) {
			cnt++
		}
		if tw.EditControlDoneForSection(s) {
			cnt++
		}
		if tw.editMotion(s) {
			cnt++
		}
		if tw.editDrawing(s) {
			cnt++
		}
		if cnt != 1 {
			panic("ui: expected exactly one parameter to be edited")
		}
	}
	ss.GW.Invalidate(false)
	ss.ScheduleShowTW()

	if !tw.edit.showAgain {
		tw.HideEditControl()
		tw.edit.meaning = EditNothing
	}
}

package ui

// The text window's screens for the groups, step dimension and tangent
// arcs, and the edit control's dispatch.

import (
	"fmt"
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

// lf makes a method value a linkFunction for Printf.
func lf(f func(link int, v uint32)) linkFunction { return f }

// ScreenHome goes back to the list of groups.
func (tw *textWindow) ScreenHome(link int, v uint32) {
	tw.GoToScreen(screenListOfGroups)
}

// ShowHeader starts a screen with the navigation bar that always
// appears at the top of the window.
func (tw *textWindow) ShowHeader(withNav bool) {
	ss := tw.ss
	tw.ClearScreen()

	var header, desc string
	if ss.GW.LockedInWorkplane() {
		header = "in plane: "
		desc = ss.entityDescription(ss.SK.MustEntity(ss.GW.ActiveWorkplane()))
	} else {
		header = "drawing / constraining in 3d"
		desc = ""
	}

	// Navigation buttons, and where the screen is from home.
	if withNav {
		tw.Printf(false, " %Fl%Lh%fhome%E%s   %Ft%s%E%s", lf(tw.ScreenHome), tw.breadcrumb(), header, desc)
	} else {
		tw.Printf(false, "        %Ft%s%E%s", header, desc)
	}

	// Leave space for the icons that are painted here.
	tw.Printf(false, "")
	tw.Printf(false, "")
}

// screenTitles name the screens a breadcrumb names by their title.
var screenTitles = map[Screen]string{
	screenConfiguration:    "preferences",
	screenStepDimension:    "step dimension",
	screenListOfStyles:     "line styles",
	screenStyleInfo:        "style",
	screenPasteTransformed: "paste transformed",
	screenEditView:         "view",
	screenTangentArc:       "tangent arc",
	screenThread:           "screw thread",
	screenParameters:       "parameters",
	screenCommands:         "find command",
	screenBrokenReferences: "broken references",
	screenSection:          "section view",
	screenBillOfMaterials:  "bill of materials",
	screenMotionStudy:      "motion study",
	screenDrawing:          "drawing sheet",
	screenDatums:           "datums",
}

// breadcrumb says where the screen shown is from the list of groups,
// after "home": the group a group's screen is of, or the screen's
// title.
func (tw *textWindow) breadcrumb() string {
	const sep = " › "
	switch tw.shown.screen {
	case screenListOfGroups:
		return ""
	case screenGroupInfo, screenGroupSolveInfo:
		g := tw.ss.SK.Group(tw.shown.group)
		if g == nil {
			return ""
		}
		crumb := sep + groupDescription(g)
		if tw.shown.screen == screenGroupSolveInfo {
			crumb += sep + "solve"
		}
		return crumb
	}
	if title, ok := screenTitles[tw.shown.screen]; ok {
		return sep + title
	}
	return ""
}

// ScreenSelectGroup shows a group's screen.
func (tw *textWindow) ScreenSelectGroup(link int, v uint32) {
	tw.ss.GW.MenuEdit(cmdUnselectAll)
	tw.GoToScreen(screenGroupInfo)
	tw.shown.group = core.HGroup{V: v}
}

// ScreenToggleGroupShown hides or shows a group.
func (tw *textWindow) ScreenToggleGroupShown(link int, v uint32) {
	tw.ss.UndoRemember()
	g := tw.ss.SK.Group(core.HGroup{V: v})
	g.Visible = !g.Visible
	tw.ss.SK.MarkGroupDirty(g.H, false)
	// If a group was just shown, then it might not have been generated
	// previously, so regenerate.
	tw.ss.GenerateAll(sketch.GenerateDirty)
}

// ScreenShowGroupsSpecial shows all, the unconstrained or no groups.
func (tw *textWindow) ScreenShowGroupsSpecial(link int, v uint32) {
	sk := tw.ss.SK
	tw.ss.UndoRemember()
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
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

// ScreenActivateGroup makes a group active.
func (tw *textWindow) ScreenActivateGroup(link int, v uint32) {
	ss := tw.ss
	if why := ss.SK.SuppressedReason(core.HGroup{V: v}); why != "" {
		ss.Error("%s can't be made active: it is left out of the model (%s).",
			ss.SK.Group(core.HGroup{V: v}).DescriptionString(), why)
		return
	}
	ss.SK.Active = core.HGroup{V: v}
	ss.groupActivate(ss.SK.Group(ss.SK.Active))
	ss.GW.ClearSuper()
}

// ReportHowGroupSolved shows why a group failed to solve.
func (tw *textWindow) ReportHowGroupSolved(hg core.HGroup) {
	tw.ss.GW.ClearSuper()
	tw.GoToScreen(screenGroupSolveInfo)
	tw.shown.group = hg
	tw.ss.ScheduleShowTW()
}

// ScreenHowGroupSolved activates a group and shows how it solved.
func (tw *textWindow) ScreenHowGroupSolved(link int, v uint32) {
	if tw.ss.GW.activeGroupH().V != v {
		tw.ScreenActivateGroup(link, v)
	}
	tw.GoToScreen(screenGroupSolveInfo)
	tw.shown.group = core.HGroup{V: v}
}

// ScreenShowConfiguration shows the configuration.
func (tw *textWindow) ScreenShowConfiguration(link int, v uint32) {
	tw.GoToScreen(screenConfiguration)
}

// ScreenShowEditView shows the view parameters.
func (tw *textWindow) ScreenShowEditView(link int, v uint32) {
	tw.GoToScreen(screenEditView)
}

// ScreenGoToWebsite opens the project's home.
func (tw *textWindow) ScreenGoToWebsite(link int, v uint32) {
	tw.ss.Platform.OpenInBrowser(version.Repository)
}

// ShowListOfGroups is the home screen: every group in the sketch, with
// options to hide or show them, and to view them in detail.
func (tw *textWindow) ShowListOfGroups() {
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
		g := sk.Group(hg)

		s := groupDescription(g)
		active := g.H == ss.GW.activeGroupH()
		shown := g.Visible
		ok := g.IsSolvedOkay()
		warn := (g.Type == sketch.GroupDrawingWorkplane && g.PolyError.How != int(core.PolyGood)) ||
			((g.Type == sketch.GroupExtrude || g.Type == sketch.GroupLathe) &&
				sk.Group(g.OpA).PolyError.How != int(core.PolyGood)) ||
			g.Problem.Msg != "" || g.LinkProblem != ""
		dof := g.Solved.Dof
		sdof := "ok "
		if ok && dof > 0 {
			if dof > 999 {
				sdof = "###"
			} else {
				sdof = fmt.Sprintf("%-3d", dof)
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
			suffix += fmt.Sprintf(" (suppressed with g%03x)", g.SuppressedBy.V)
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

// ScreenHoverGroupWorkplane hovers a group's workplane.
func (tw *textWindow) ScreenHoverGroupWorkplane(link int, v uint32) {
	gw := tw.ss.GW
	gw.hover.Clear()
	gw.hover.Entity = core.HGroup{V: v}.Entity(0)
	gw.hover.Emphasized = true
}

// ScreenHoverEntity hovers an entity.
func (tw *textWindow) ScreenHoverEntity(link int, v uint32) {
	gw := tw.ss.GW
	gw.hover.Clear()
	gw.hover.Entity = core.HEntity{V: v}
	gw.hover.Emphasized = true
}

// ScreenHoverRequest hovers a request's first entity.
func (tw *textWindow) ScreenHoverRequest(link int, v uint32) {
	gw := tw.ss.GW
	gw.hover.Clear()
	gw.hover.Entity = core.HRequest{V: v}.Entity(0)
	gw.hover.Emphasized = true
}

// ScreenHoverConstraint hovers a constraint.
func (tw *textWindow) ScreenHoverConstraint(link int, v uint32) {
	gw := tw.ss.GW
	if gw.showConstraints == sCMNoShow {
		return
	}
	gw.hover.Clear()
	gw.hover.Constraint = core.HConstraint{V: v}
	gw.hover.Emphasized = true
}

// ScreenSelectEntity selects an entity.
func (tw *textWindow) ScreenSelectEntity(link int, v uint32) {
	gw := tw.ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Entity: core.HEntity{V: v}})
}

// ScreenSelectRequest selects a request's first entity.
func (tw *textWindow) ScreenSelectRequest(link int, v uint32) {
	gw := tw.ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Entity: core.HRequest{V: v}.Entity(0)})
}

// ScreenSelectConstraint selects a constraint.
func (tw *textWindow) ScreenSelectConstraint(link int, v uint32) {
	gw := tw.ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Constraint: core.HConstraint{V: v}})
}

// ScreenChangeGroupOption flips one of a group's options.
func (tw *textWindow) ScreenChangeGroupOption(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	ss.UndoRemember()
	g := sk.Group(tw.shown.group)

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
		skewed := g.Subtype == sketch.GroupOneSided || g.Subtype == sketch.GroupTwoSided
		if err := sk.SetGroupSkewed(g.H, skewed); err != nil {
			ss.Report(err)
		}

	case 'k', 'K':
		if err := sk.SetGroupSkipFirst(g.H, link == 'k'); err != nil {
			ss.Report(err)
		}
	case 'm':
		// A mirror of the whole solid so far, or of its source alone.
		g.MirrorSolid = !g.MirrorSolid

	case 'c':
		if err := sk.SetGroupCombine(g.H, sketch.CombineAs(v), true); err != nil {
			ss.Report(err)
		}

	case 'P':
		if err := sk.SetGroupSuppressSolid(g.H, !g.Suppress); err != nil {
			ss.Report(err)
		}

	case 'A':
		if err := sk.SetSuppressAll(g.H, !g.SuppressAll); err != nil {
			ss.Report(err)
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
func (tw *textWindow) ScreenChangeLoftTurn(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	g := sk.Group(tw.shown.group)
	if g == nil || g.Type != sketch.GroupLoft {
		return
	}
	ss.UndoRemember()
	g.ValA += float64(int32(v))
	g.ExprA = ""
	sk.MarkGroupDirty(g.H, false)
	ss.GW.Invalidate(false)
}

// ScreenColor edits a group's colour.
func (tw *textWindow) ScreenColor(link int, v uint32) {
	tw.ss.UndoRemember()

	g := tw.ss.SK.Group(tw.shown.group)
	tw.ShowEditControlWithColorPicker(3, g.Color)
	tw.edit.meaning = editGroupColor
}

// ScreenOpacity edits a group's opacity.
func (tw *textWindow) ScreenOpacity(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)

	tw.ShowEditControl(11, fmt.Sprintf("%.2f", g.Color.AlphaF()))
	tw.edit.meaning = editGroupOpacity
	tw.edit.group = g.H
}

// ScreenDensity edits the density of a group's solid.
func (tw *textWindow) ScreenDensity(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)

	text := ""
	if g.Density > 0 {
		text = strconv.FormatFloat(g.Density, 'g', -1, 64)
	}
	tw.ShowEditControl(11, text)
	tw.edit.meaning = editGroupDensity
	tw.edit.group = g.H
}

// ScreenChangeExprA edits how many times a group repeats.
func (tw *textWindow) ScreenChangeExprA(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)

	text := g.ExprA
	if text == "" {
		text = fmt.Sprintf("%d", int(g.ValA))
	}
	tw.ShowEditControl(10, text)
	tw.edit.meaning = editTimesRepeated
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeGroupName renames a group.
func (tw *textWindow) ScreenChangeGroupName(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)
	tw.ShowEditControl(12, groupDescription(g)[5:])
	tw.edit.meaning = editGroupName
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeGroupScale edits a linked group's scale.
func (tw *textWindow) ScreenChangeGroupScale(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)

	text := g.ExprScale
	if text == "" {
		text = fmt.Sprintf("%.3f", g.Scale)
	}
	tw.ShowEditControl(13, text)
	tw.edit.meaning = editGroupScale
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeHelixPitch edits a helix's pitch, a chamfer's distance, a
// fillet's radius or a sweep's corner radius, all ValB.
func (tw *textWindow) ScreenChangeHelixPitch(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)
	text := g.ExprB
	if text == "" {
		text = fmt.Sprintf("%.8f", g.ValB/tw.ss.MmPerUnit())
	}
	tw.ShowEditControl(3, text)
	tw.edit.meaning = editHelixPitch
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeLinkAsSolid reads a linked STEP file as a solid ('s') or
// as a reference mesh, and reads it again.
func (tw *textWindow) ScreenChangeLinkAsSolid(link int, v uint32) {
	ss := tw.ss
	g := ss.SK.Group(tw.shown.group)
	if g == nil || g.Type != sketch.GroupLinked || g.LinkAsSolid == (link == 's') {
		return
	}
	ss.UndoRemember()
	if err := ss.SK.SetGroupLinkAsSolid(g.H, link == 's'); err != nil {
		ss.Report(err)
	}
	ss.GW.ClearSuper()
}

// ScreenChangeSweepUp holds a sweep's frames to an axis, by the link's
// letter, or ('n') lets them turn least.
func (tw *textWindow) ScreenChangeSweepUp(link int, v uint32) {
	ss := tw.ss
	g := ss.SK.Group(tw.shown.group)
	if g == nil || g.Type != sketch.GroupSweep {
		return
	}
	ss.UndoRemember()
	up := geom.Vector{}
	switch link {
	case 'x':
		up = geom.Vector{X: 1}
	case 'y':
		up = geom.Vector{Y: 1}
	case 'z':
		up = geom.Vector{Z: 1}
	}
	if err := ss.SK.SetSweepUp(g.H, up); err != nil {
		ss.Report(err)
	}
	ss.GW.ClearSuper()
}

// ScreenChangeGroupDraft edits an extrusion's draft or a sweep's twist,
// in degrees.
func (tw *textWindow) ScreenChangeGroupDraft(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)
	text := g.ExprC
	if text == "" {
		text = fmt.Sprintf("%g", g.ValC)
	}
	tw.ShowEditControl(10, text)
	tw.edit.meaning = editGroupDraft
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangeChamferDistance2 edits a chamfer's second distance, on
// each edge's second face (roadmap 28.1).
func (tw *textWindow) ScreenChangeChamferDistance2(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)
	text := g.ExprC
	if text == "" {
		d := g.ValC
		if d == 0 {
			d = g.ValB
		}
		text = fmt.Sprintf("%.8f", d/tw.ss.MmPerUnit())
	}
	tw.ShowEditControl(3, text)
	tw.edit.meaning = editChamferDistance2
	tw.edit.group = core.HGroup{V: v}
}

// ScreenSwapChamferDistances swaps a chamfer's two distances, which
// chamfers each edge the other way round.
func (tw *textWindow) ScreenSwapChamferDistances(link int, v uint32) {
	tw.chamferDistances(core.HGroup{V: v}, tw.ss.SK.SwapChamferDistances)
}

// ScreenSameChamferDistances makes a chamfer's second distance its first
// again.
func (tw *textWindow) ScreenSameChamferDistances(link int, v uint32) {
	tw.chamferDistances(core.HGroup{V: v}, tw.ss.SK.SameChamferDistances)
}

// chamferDistances is one undo step of change to a chamfer's distances;
// a group that can't take it is left as it is.
func (tw *textWindow) chamferDistances(h core.HGroup, change func(core.HGroup) error) {
	ss := tw.ss
	if ss.SK.UndoRememberFor(func() error { return change(h) }) != nil {
		return
	}
	ss.GW.ClearSuper()
}

// draftSelectedSides makes the active extrusion's draft lean only the
// sides of the selected curves of its sketch, their copies at its ends,
// or the selected faces of the sides (Features > Draft Selected Sides,
// roadmap 28.2).
func (ss *Application) draftSelectedSides() {
	gw := ss.GW
	g := ss.SK.Group(gw.activeGroupH())
	if g == nil || g.Type != sketch.GroupExtrude || g.ValC == 0 && g.ExprC == "" {
		ss.Error("Activate a drafted extrusion first: Draft Selected Sides chooses which of its sides the draft leans.")
		return
	}
	gw.GroupSelection()
	if len(gw.gs.entity) == 0 {
		ss.Error("Select the sides to draft: the lines, arcs and circles of the extruded sketch, " +
			"their copies at the extrusion's ends, or the faces of the sides.")
		return
	}
	err := ss.SK.UndoRememberFor(func() error {
		return ss.SK.SetDraftFaces(g.H, gw.gs.entity)
	})
	if err != nil {
		ss.Error("Can't draft those sides: %s.", err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	gw.ClearSelection()
	gw.ClearSuper()
}

// loftGuidesFromSelection holds the active loft to the selected line
// segments, arcs and cubics as its guide curves (Features > Guide Loft
// by Selected Curves, roadmap 28.3).
func (ss *Application) loftGuidesFromSelection() {
	gw := ss.GW
	g := ss.SK.Group(gw.activeGroupH())
	if g == nil || g.Type != sketch.GroupLoft {
		ss.Error("Activate a loft first: Guide Loft by Selected Curves holds it to guide curves.")
		return
	}
	gw.GroupSelection()
	if len(gw.gs.entity) == 0 {
		ss.Error("Select the guide curves: line segments, arcs and cubics drawn before the loft, " +
			"each guide running from a point of the first profile through every profile to the last.")
		return
	}
	err := ss.SK.UndoRememberFor(func() error {
		return ss.SK.SetLoftGuides(g.H, gw.gs.entity)
	})
	if err != nil {
		ss.Error("Can't guide the loft by those: %s.", err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	gw.ClearSelection()
	gw.ClearSuper()
}

// ScreenLoftNoGuides takes a loft's guide curves away.
func (tw *textWindow) ScreenLoftNoGuides(link int, v uint32) {
	ss := tw.ss
	err := ss.SK.UndoRememberFor(func() error { return ss.SK.SetLoftGuides(core.HGroup{V: v}, nil) })
	if err != nil {
		ss.Report(err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.GW.ClearSuper()
}

// ScreenDraftAllSides makes an extrusion's draft lean every side again.
func (tw *textWindow) ScreenDraftAllSides(link int, v uint32) {
	ss := tw.ss
	err := ss.SK.UndoRememberFor(func() error { return ss.SK.SetDraftFaces(core.HGroup{V: v}, nil) })
	if err != nil {
		ss.Report(err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.GW.ClearSuper()
}

// showLoftEnds shows how a loft leaves its first profile and reaches
// its last (roadmap 28.3).
func (tw *textWindow) showLoftEnds(g *sketch.Group) {
	end := func(v float64) string {
		if v == 0 {
			return "free"
		}
		return fmt.Sprintf("square to it, %g", v)
	}
	tw.Printf(false, "%Bd   %Ftleaves the first%E %s%s %Fl%Ll%f%D[change]%E",
		end(g.ValB), exprNote(g.ExprB), lf(tw.ScreenChangeLoftStart), g.H.V)
	tw.Printf(false, "%Ba   %Ftreaches the last%E %s%s %Fl%Ll%f%D[change]%E",
		end(g.ValC), exprNote(g.ExprC), lf(tw.ScreenChangeGroupDraft), g.H.V)
	// Its guides: Features > Guide Loft by Selected Curves sets them.
	if len(g.Edges) == 0 {
		tw.Printf(false, "%Bd   %Ftguides%E none")
	} else {
		curves := "curves"
		if len(g.Edges) == 1 {
			curves = "curve"
		}
		tw.Printf(false, "%Bd   %Ftguides%E %d %s %Fl%Ll%f%D[none]%E",
			len(g.Edges), curves, lf(tw.ScreenLoftNoGuides), g.H.V)
	}
}

// ScreenChangeLoftStart edits how square a loft leaves its first
// profile, 0 for free.
func (tw *textWindow) ScreenChangeLoftStart(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)
	text := g.ExprB
	if text == "" {
		text = fmt.Sprintf("%g", g.ValB)
	}
	tw.ShowEditControl(3, text)
	tw.edit.meaning = editHelixPitch
	tw.edit.group = core.HGroup{V: v}
}

// ScreenChangePitchOption fixes or frees a helix's pitch.
func (tw *textWindow) ScreenChangePitchOption(link int, v uint32) {
	sk := tw.ss.SK
	tw.ss.UndoRemember()
	g := sk.Group(tw.shown.group)
	if g.ValB == 0.0 {
		g.ValB = sk.MustParam(g.H.Param(7)).Val * math.Pi / sk.MustParam(g.H.Param(3)).Val
	} else {
		g.ValB = 0.0
	}
	g.ExprB = ""
	sk.MarkGroupDirty(g.H, false)
	tw.ss.GW.Invalidate(false)
}

// ScreenDeleteGroup deletes the shown group.
func (tw *textWindow) ScreenDeleteGroup(link int, v uint32) {
	tw.ss.UndoRemember()
	tw.ss.deleteGroup(tw.shown.group)
}

// deleteGroup deletes group hg, and with it what used it. When the
// active group goes, the last group before it that is left is made
// active, rather than the first one.
func (ss *Application) deleteGroup(hg core.HGroup) {
	sk := ss.SK
	order := slices.Clone(sk.GroupOrder)
	was := ss.GW.activeGroupH()
	if hg == was {
		if prev := sk.Group(hg).PreviousGroup(sk); prev != nil {
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
		ss.groupActivate(sk.Group(sk.Active))
	}

	// Reset the graphics window. This will also recreate the default
	// group if it was removed.
	ss.GW.ClearSuper()
}

// relativePath names path relative to the folder of the sketch's file,
// or "" when it can't.
func (ss *Application) relativePath(path string) string {
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
// absolute paths it chose; they are shown relative when they can be.
func (ss *Application) linkDisplayPath(path string) string {
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

// ShowGroupInfo is the screen about one group, where the user edits it.
func (tw *textWindow) ShowGroupInfo() {
	ss := tw.ss
	sk := ss.SK
	g := sk.Group(tw.shown.group)

	if tw.shown.group.V == core.HGroupReferences {
		tw.Printf(true, "%FtGROUP  %E%s", groupDescription(g))
		tw.showGroupItems(g)
		return
	}
	actions, args := tw.groupActions(g)
	tw.Printf(true, "%FtGROUP  %E%s [%Fl%Ll%D%frename%E/%Fl%Ll%D%fdel%E] "+actions,
		append([]any{groupDescription(g),
			g.H.V, lf(tw.ScreenChangeGroupName),
			g.H.V, lf(tw.ScreenDeleteGroup)}, args...)...)

	if why := sk.SuppressedReason(g.H); why != "" {
		tw.Printf(false, " %Fxleft out of the model: %s%E", why)
	}

	switch g.Type {
	case sketch.GroupLathe:
		tw.Printf(true, " %Ftlathe plane sketch")
	case sketch.GroupExtrude, sketch.GroupRotate, sketch.GroupTranslate, sketch.GroupRevolve,
		sketch.GroupHelix:
		tw.showExtrusionInfo(g)
	case sketch.GroupMirror:
		tw.showMirrorInfo(g)
	case sketch.GroupChamfer:
		tw.showChamferInfo(g)
	case sketch.GroupLoft:
		tw.showLoftInfo(g)
	case sketch.GroupFillet:
		tw.showFilletInfo(g)
	case sketch.GroupShell:
		tw.showShellInfo(g)
	case sketch.GroupSweep:
		tw.showSweepInfo(g)
	case sketch.GroupPathPattern:
		tw.showPathPatternInfo(g)
	case sketch.GroupLinked:
		tw.showLinkedInfo(g)
	case sketch.GroupDrawing3D:
		tw.Printf(true, " %Ftsketch in 3d%E")
	case sketch.GroupDrawingWorkplane:
		if g.Subtype == sketch.GroupWorkplaneByFace {
			tw.showSketchOnFaceInfo(g)
		} else {
			tw.Printf(true, " %Ftsketch in new workplane%E")
		}
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
				sk.MustParam(g.H.Param(7)).Val*math.Pi/
					(sk.MustParam(g.H.Param(3)).Val*ss.MmPerUnit()),
				lf(tw.ScreenChangeHelixPitch), g.H.V)
		}
		tw.Printf(false, "   %Fd%f%LP%s  fixed", lf(tw.ScreenChangePitchOption), checkMark(g.ValB != 0))

		tw.Printf(false, "") // blank line
	}

	// Which of the solid's options a kind of group has is the sketch's
	// to say (HasCombine and the others), so scripts refuse the same.
	if g.HasAppearance() {
		if g.HasCombine() {
			un := g.MeshCombine == sketch.CombineUnion
			diff := g.MeshCombine == sketch.CombineDifference
			intr := g.MeshCombine == sketch.CombineIntersection
			asy := g.MeshCombine == sketch.CombineAssemble

			tw.Printf(false, " %Ftsolid model as")
			tw.Printf(false, "%Ba   %f%D%Lc%Fd%s union%E  "+
				"%f%D%Lc%Fd%s assemble%E  ",
				lf(tw.ScreenChangeGroupOption), sketch.CombineUnion, radioMark(un),
				lf(tw.ScreenChangeGroupOption), sketch.CombineAssemble, radioMark(asy))
			tw.Printf(false, "%Ba   %f%D%Lc%Fd%s difference%E  "+
				"%f%D%Lc%Fd%s intersection%E  ",
				lf(tw.ScreenChangeGroupOption), sketch.CombineDifference, radioMark(diff),
				lf(tw.ScreenChangeGroupOption), sketch.CombineIntersection, radioMark(intr))
		}
		// The appearance when the window shades (roadmap 19.2).
		app := g.Appearance
		if app == "" {
			app = "plastic"
		}
		var line strings.Builder
		line.WriteString("%Bd   %Ftappearance%E")
		var args []any
		for _, name := range render.MaterialNames {
			line.WriteString("  %f%Lp%D%Fd%s " + name + "%E")
			args = append(args, lf(tw.ScreenChangeAppearance), 'a', appearanceIndex(name), radioMark(app == name))
		}
		// The density, for the part's mass, goes on the same row: a row
		// of its own would move every row below it on the screen.
		if g.HasDensity() {
			density := "not set"
			if g.Density > 0 {
				density = strconv.FormatFloat(g.Density, 'g', -1, 64) + " g/cm³"
			}
			line.WriteString("   %Ftdensity%E %s %f%Lf%Fl[change]%E")
			args = append(args, density, lf(tw.ScreenDensity))
		}
		tw.Printf(false, line.String(), args...)

		if g.HasColor() {
			tw.Printf(false,
				"%Bd   %Ftcolor   %E%Bz  %Bd (%@, %@, %@) %f%D%Lf%Fl[change]%E",
				&g.Color,
				float64(g.Color.RedF()), float64(g.Color.GreenF()), float64(g.Color.BlueF()),
				lf(tw.ScreenColor), tw.lineTop(tw.rows-1)+2)
			tw.Printf(false, "%Bd   %Ftopacity%E %@ %f%Lf%Fl[change]%E",
				float64(g.Color.AlphaF()), lf(tw.ScreenOpacity))
		}

		if g.HasSolidSuppress() {
			tw.Printf(false, "   %Fd%f%LP%s  suppress this group's solid model",
				lf(tw.ScreenChangeGroupOption), checkMark(g.Suppress))
		}

		tw.Printf(false, "")
	}

	tw.Printf(false, " %f%Lv%Fd%s  show entities from this group",
		lf(tw.ScreenChangeGroupOption), checkMark(g.Visible))
	tw.Printf(false, " %f%LA%Fd%s  suppress this group and the groups that use it",
		lf(tw.ScreenChangeGroupOption), checkMark(g.SuppressAll))

	if !g.IsForcedToMeshBySource(sk) && !g.IsTriangleMeshAssembly() {
		tw.Printf(false, " %f%Lf%Fd%s  force NURBS surfaces to triangle mesh",
			lf(tw.ScreenChangeGroupOption), checkMark(g.ForceToMesh))
	} else {
		tw.Printf(false, " (model already forced to triangle mesh)")
	}

	tw.Printf(true, " %f%Lr%Fd%s  relax constraints and dimensions",
		lf(tw.ScreenChangeGroupOption), checkMark(g.RelaxConstraints))

	tw.Printf(false, " %f%Le%Fd%s  allow redundant constraints",
		lf(tw.ScreenChangeGroupOption), checkMark(g.AllowRedundant))

	tw.Printf(false, " %f%LD%Fd%s  suppress dof calculation (improves solver performance)",
		lf(tw.ScreenChangeGroupOption), checkMark(g.SuppressDofCalculation))

	tw.Printf(false, " %f%Ld%Fd%s  treat all dimensions as reference",
		lf(tw.ScreenChangeGroupOption), checkMark(g.AllDimsReference))

	if g.Problem.Msg != "" {
		tw.Printf(false, "")
		switch {
		case g.HasDraft():
			tw.Printf(false, "%FxThe draft is left out: %s.%E", g.Problem.Msg)
		case g.FaceProblem() != "" && g.Problem.Msg == g.FaceProblem():
			tw.Printf(false, "%FxThe sketch can't follow its face: %s.%E", g.Problem.Msg)
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
		if g.JoinsAssembly(ss.SK) {
			tw.Printf(false, "")
			tw.Printf(false, "The solid so far holds parts assembled side ")
			tw.Printf(false, "by side; where they stand in each other the ")
			tw.Printf(false, "Boolean is not defined. Analyze > Show ")
			tw.Printf(false, "Interfering Parts finds where. Parts that ")
			tw.Printf(false, "touch face to face often fail it too. Join ")
			tw.Printf(false, "or cut each part before it is assembled.")
		}
	}

	tw.showGroupItems(g)
}

// groupActions are the group list's actions for the group whose screen
// this is, to follow its name: make it active, show or hide it, move it
// earlier or later in the history. They share the name's line, so the
// screen's other lines stay where they were.
func (tw *textWindow) groupActions(g *sketch.Group) (format string, args []any) {
	format = "[%Fsactive%E/"
	if g.H != tw.ss.GW.activeGroupH() {
		format = "[%Fl%La%D%factivate%E/"
		args = []any{g.H.V, lf(tw.ScreenActivateGroup)}
	}
	shown := "hide"
	if !g.Visible {
		shown = "show"
	}
	format += "%Fl%Ls%D%f%s%E/%Fl%Lu%D%f↑%E/%Fl%Ld%D%f↓%E]"
	args = append(args, g.H.V, lf(tw.ScreenToggleGroupShown), shown,
		g.H.V, lf(tw.ScreenMoveGroup), g.H.V, lf(tw.ScreenMoveGroup))
	return format, args
}

// showExtrusionInfo shows the settings of a sketch.GroupExtrude, sketch.GroupRotate, sketch.GroupTranslate, sketch.GroupRevolve, sketch.GroupHelix group.
func (tw *textWindow) showExtrusionInfo(g *sketch.Group) {
	ss := tw.ss
	s := "???"

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
			lf(tw.ScreenChangeGroupOption), radioMark(one),
			lf(tw.ScreenChangeGroupOption), radioMark(two),
			lf(tw.ScreenChangeGroupOption), checkMark(skew))
		tw.Printf(false, "%Bd   %Ftdraft%E %s deg%s %Fl%Ll%f%D[change]%E",
			fmt.Sprintf("%g", g.ValC), exprNote(g.ExprC), lf(tw.ScreenChangeGroupDraft), g.H.V)
		if g.ValC != 0 || g.ExprC != "" {
			// Roadmap 28.2: the plane the sides lean from, and which.
			tw.Printf(false, "%Ba   %Ftfrom the plane%E %s along%s %Fl%Ll%f%D[change]%E",
				ss.MmToString(g.ValB, false), exprNote(g.ExprB), lf(tw.ScreenChangeHelixPitch), g.H.V)
			// Which: Features > Draft Selected Sides chooses them.
			if len(g.DraftFaces) == 0 {
				tw.Printf(false, "%Bd   %Ftsides%E all")
			} else {
				tw.Printf(false, "%Bd   %Ftsides%E %d of the sketch's %Fl%Ll%f%D[all]%E",
					len(g.DraftFaces), lf(tw.ScreenDraftAllSides), g.H.V)
			}
		}
	} else {
		tw.Printf(false,
			"%Ba   %f%Ls%Fd%s one-sided%E  "+
				"%f%LS%Fd%s two-sided%E",
			lf(tw.ScreenChangeGroupOption), radioMark(one),
			lf(tw.ScreenChangeGroupOption), radioMark(two))
	}

	if g.Type == sketch.GroupRotate || g.Type == sketch.GroupTranslate {
		if g.Subtype == sketch.GroupOneSided {
			skip := g.SkipFirst
			tw.Printf(false,
				"%Bd   %Ftstart  %f%LK%Fd%s with original%E  "+
					"%f%Lk%Fd%s with copy #1%E",
				lf(tw.ScreenChangeGroupOption), radioMark(!skip),
				lf(tw.ScreenChangeGroupOption), radioMark(skip))
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
}

// showMirrorInfo shows the settings of a sketch.GroupMirror group.
func (tw *textWindow) showMirrorInfo(g *sketch.Group) {
	ss := tw.ss
	sk := ss.SK

	tw.Printf(true, " %Ftmirror original sketch and solid%E")
	pl := g.Predef.EntityB
	tw.Printf(false, "%Ba   %Ftin plane%E %Fl%Ll%D%f%h%s%E",
		pl.V, lf(tw.ScreenSelectEntity), lf(tw.ScreenHoverEntity), sk.EntityName(pl))
	tw.Printf(false, "%Bd   %Ftsolid  %f%LK%Fd%s with original%E  "+
		"%f%Lk%Fd%s reflection only%E",
		lf(tw.ScreenChangeGroupOption), radioMark(!g.SkipFirst),
		lf(tw.ScreenChangeGroupOption), radioMark(g.SkipFirst))
	tw.Printf(false, "%Ba   %f%Lm%Fd%s the whole solid so far%E",
		lf(tw.ScreenChangeGroupOption), checkMark(g.MirrorSolid))
}

// showChamferInfo shows the settings of a sketch.GroupChamfer group.
func (tw *textWindow) showChamferInfo(g *sketch.Group) {
	ss := tw.ss

	tw.Printf(true, " %Ftchamfer edges of the solid%E")
	tw.Printf(false, "%Ba   %Ftdistance%E %s%s %Fl%Ll%f%D[change]%E",
		ss.MmToString(g.ValB, false), exprNote(g.ExprB), lf(tw.ScreenChangeHelixPitch), g.H.V)
	if g.ValC == 0 && g.ExprC == "" {
		tw.Printf(false, "%Bd   %Ftsecond distance%E the same %Fl%Ll%f%D[change]%E",
			lf(tw.ScreenChangeChamferDistance2), g.H.V)
	} else {
		tw.Printf(false, "%Bd   %Ftsecond distance%E %s%s %Fl%Ll%f%D[change]%E %Fl%Ll%f%D[swap]%E %Fl%Ll%f%D[same]%E",
			ss.MmToString(g.ValC, false), exprNote(g.ExprC), lf(tw.ScreenChangeChamferDistance2), g.H.V,
			lf(tw.ScreenSwapChamferDistances), g.H.V, lf(tw.ScreenSameChamferDistances), g.H.V)
	}
	tw.showGroupEdges(g, "edges")
}

// showLoftInfo shows the settings of a sketch.GroupLoft group.
func (tw *textWindow) showLoftInfo(g *sketch.Group) {
	ss := tw.ss
	sk := ss.SK

	if len(g.LoftThrough) == 0 {
		tw.Printf(true, " %Ftloft between two profiles%E")
		tw.Printf(false, "%Ba   %Ftfrom%E %s", sk.GroupName(g.OpA))
		tw.Printf(false, "%Bd   %Ftto%E   %s", sk.GroupName(g.OpB))
		tw.Printf(false, "%Ba   %Ftturn the second by%E %d %Fl%Ll%f%D[-1]%E %Fl%Ll%f%D[+1]%E",
			int(g.ValA), lf(tw.ScreenChangeLoftTurn), -1, lf(tw.ScreenChangeLoftTurn), 1)
		tw.showLoftEnds(g)
		return
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
	tw.showLoftEnds(g)
}

// showFilletInfo shows the settings of a sketch.GroupFillet group.
func (tw *textWindow) showFilletInfo(g *sketch.Group) {
	ss := tw.ss

	tw.Printf(true, " %Ftround edges of the solid%E")
	tw.Printf(false, "%Ba   %Ftradius%E %s%s %Fl%Ll%f%D[change]%E",
		ss.MmToString(g.ValB, false), exprNote(g.ExprB), lf(tw.ScreenChangeHelixPitch), g.H.V)
	tw.showGroupEdges(g, "edges")
}

// showShellInfo shows the settings of a sketch.GroupShell group.
func (tw *textWindow) showShellInfo(g *sketch.Group) {
	ss := tw.ss
	sk := ss.SK

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
}

// showSketchOnFaceInfo shows the face a sketch on a face lies in.
func (tw *textWindow) showSketchOnFaceInfo(g *sketch.Group) {
	sk := tw.ss.SK
	tw.Printf(true, " %Ftsketch on face%E")
	for _, id := range g.SolidFaces {
		tw.Printf(false, "%Ba   %Ftin%E %s", sk.FaceName(id))
	}
}

// showSweepInfo shows the settings of a sketch.GroupSweep group.
func (tw *textWindow) showSweepInfo(g *sketch.Group) {

	tw.Printf(true, " %Ftsweep plane sketch along path%E")
	tw.showGroupEdges(g, "path")
	tw.Printf(false, "%Bd   %Fttwist%E %s deg%s %Fl%Ll%f%D[change]%E",
		fmt.Sprintf("%g", g.ValC), exprNote(g.ExprC), lf(tw.ScreenChangeGroupDraft), g.H.V)
	if g.ValB == 0 && g.ExprB == "" {
		tw.Printf(false, "%Ba   %Ftcorners%E mitred %Fl%Ll%f%D[round]%E",
			lf(tw.ScreenChangeHelixPitch), g.H.V)
	} else {
		tw.Printf(false, "%Ba   %Ftcorner radius%E %s%s %Fl%Ll%f%D[change]%E",
			tw.ss.MmToString(g.ValB, true), exprNote(g.ExprB), lf(tw.ScreenChangeHelixPitch), g.H.V)
	}
	up := g.SweepUp
	tw.Printf(false, "%Ba   %Ftframes%E "+
		"%f%Ln%Fd%s turn least%E  "+
		"%f%Lx%Fd%s held to x%E  "+
		"%f%Ly%Fd%s y%E  "+
		"%f%Lz%Fd%s z%E",
		lf(tw.ScreenChangeSweepUp), radioMark(up.Magnitude() == 0),
		lf(tw.ScreenChangeSweepUp), radioMark(up == geom.Vector{X: 1}),
		lf(tw.ScreenChangeSweepUp), radioMark(up == geom.Vector{Y: 1}),
		lf(tw.ScreenChangeSweepUp), radioMark(up == geom.Vector{Z: 1}))
}

// showPathPatternInfo shows the settings of a sketch.GroupPathPattern group.
func (tw *textWindow) showPathPatternInfo(g *sketch.Group) {

	tw.Printf(true, " %Ftrepeat solid along path%E")
	tw.Printf(false, "%Ba   %Ftcopies%E %d%s %Fl%Ll%D%f[change]%E",
		int(g.ValA), exprNote(g.ExprA), g.H.V, lf(tw.ScreenChangeExprA))
	tw.showGroupEdges(g, "path")
}

// showLinkedInfo shows the settings of a sketch.GroupLinked group.
func (tw *textWindow) showLinkedInfo(g *sketch.Group) {
	ss := tw.ss

	tw.Printf(true, " %Ftlink geometry from file%E")
	tw.Printf(false, "%Ba   '%s' %Fl%Ll%f%D[relink]%E", ss.linkDisplayPath(g.LinkFile), lf(tw.ScreenRelink), g.H.V)
	tw.Printf(false, "%Bd   %Ftscaled by%E %#%s %Fl%Ll%f%D[change]%E",
		g.Scale, exprNote(g.ExprScale), lf(tw.ScreenChangeGroupScale), g.H.V)
	if link.IsStep(g.LinkFile) {
		tw.Printf(false, "%Ba   %Ftread as%E %f%Lm%Fd%s reference mesh%E  %f%Ls%Fd%s solid%E",
			lf(tw.ScreenChangeLinkAsSolid), radioMark(!g.LinkAsSolid),
			lf(tw.ScreenChangeLinkAsSolid), radioMark(g.LinkAsSolid))
	}
	tw.showLinkModel(g)
	for _, line := range g.LinkInfo {
		tw.Printf(false, "   %Fd%s%E", line)
	}
	if g.LinkProblem != "" {
		for i, line := range wrapWords("Not all of the file was read: "+g.LinkProblem+".", maxCols-6) {
			indent := "   "
			if i > 0 {
				indent = "     "
			}
			tw.Printf(false, "%s%Fx%s%E", indent, line)
		}
	}
}

// showGroupEdges lists the edges of a chamfer or the path of a sweep,
// each one hovered and selected from its row.
func (tw *textWindow) showGroupEdges(g *sketch.Group, what string) {
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
		for k, line := range wrapWords(sk.SolidEdgeName(n), maxCols-8) {
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
func (tw *textWindow) showEdgeRadius(g *sketch.Group, i int, bg byte) {
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
func (tw *textWindow) ScreenChangeEdgeRadius(link int, v uint32) {
	g := tw.ss.SK.Group(tw.shown.group)
	val, text, _ := g.EdgeRadius(int(v))
	if text == "" {
		text = fmt.Sprintf("%.8f", val/tw.ss.MmPerUnit())
	}
	tw.ShowEditControl(9, text)
	tw.edit.meaning = editEdgeRadius
	tw.edit.group = g.H
	tw.edit.i = int(v)
}

// ScreenResetEdgeRadius gives the shown fillet's edge v the fillet's
// radius again.
func (tw *textWindow) ScreenResetEdgeRadius(link int, v uint32) {
	tw.ss.setEdgeRadiusText(tw.shown.group, int(v), "")
}

// setEdgeRadiusText gives a fillet's edge its own radius from typed
// text, or "" the fillet's, as one step of undo.
func (ss *Application) setEdgeRadiusText(hg core.HGroup, i int, s string) {
	err := ss.SK.UndoRememberFor(func() error {
		return ss.SK.SetEdgeRadiusText(hg, i, s)
	})
	if err != nil {
		ss.Report(err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	ss.GW.Invalidate(false)
}

// showGroupItems lists a group's requests and constraints.
func (tw *textWindow) showGroupItems(g *sketch.Group) {
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

// ScreenAllowRedundant allows the shown group redundant constraints.
func (tw *textWindow) ScreenAllowRedundant(link int, v uint32) {
	ss := tw.ss
	ss.UndoRemember()

	g := ss.SK.Group(tw.shown.group)
	g.AllowRedundant = true
	ss.SK.MarkGroupDirty(tw.shown.group, false)

	tw.shown.screen = screenGroupInfo
	tw.Show()
}

// ShowGroupSolveInfo is the screen displayed when the sketch fails to
// solve: what failed, and (if the problem is a singular Jacobian) the
// constraints that could be removed to fix it.
func (tw *textWindow) ShowGroupSolveInfo() {
	sk := tw.ss.SK
	g := sk.Group(tw.shown.group)
	if g.IsSolvedOkay() {
		// Go back to the default group info screen
		tw.shown.screen = screenGroupInfo
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
		c := sk.Constraint(hc)
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

// ScreenStepDimFinish edits the step dimension's final value.
func (tw *textWindow) ScreenStepDimFinish(link int, v uint32) {
	tw.edit.meaning = editStepDimFinish
	var editValue string
	if tw.stepDim.isDistance {
		editValue = tw.ss.MmToString(tw.stepDim.finish, true)
	} else {
		editValue = fmt.Sprintf("%.3f", tw.stepDim.finish)
	}
	tw.ShowEditControl(12, editValue)
}

// ScreenStepDimSteps edits the number of steps.
func (tw *textWindow) ScreenStepDimSteps(link int, v uint32) {
	tw.edit.meaning = editStepDimSteps
	tw.ShowEditControl(12, fmt.Sprintf("%d", tw.stepDim.steps))
}

// ScreenStepDimGo steps the dimension, solving at each step.
func (tw *textWindow) ScreenStepDimGo(link int, v uint32) {
	ss := tw.ss
	hc := tw.shown.constraint
	c := ss.SK.Constraint(hc)
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
			c := ss.SK.Constraint(hc)
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
			ss.TW.GoToScreen(screenListOfGroups)
			ss.ScheduleShowTW()
		}
		ss.GW.Invalidate(false)
	})
	tw.stepDim.timer.RunAfterNextFrame()
}

// ShowStepDimension is the screen to step a dimension from its value to
// a final one.
func (tw *textWindow) ShowStepDimension() {
	ss := tw.ss
	c := ss.SK.Constraint(tw.shown.constraint)
	if c == nil {
		tw.shown.screen = screenListOfGroups
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

// ScreenChangeTangentArc edits the tangent arc options.
func (tw *textWindow) ScreenChangeTangentArc(link int, v uint32) {
	ss := tw.ss
	switch link {
	case 'r':
		tw.edit.meaning = editTangentArcRadius
		tw.ShowEditControl(3, ss.MmToString(ss.TangentArcRadius, true))

	case 'a':
		ss.TangentArcManual = !ss.TangentArcManual
	case 'm':
		ss.TangentArcModify = !ss.TangentArcModify
	}
}

// ShowTangentArc is the screen for the tangent arc options.
func (tw *textWindow) ShowTangentArc() {
	ss := tw.ss
	tw.Printf(true, "%FtTANGENT ARC PARAMETERS%E")

	tw.Printf(true, "%Ft radius of created arc%E")
	if ss.TangentArcManual {
		tw.Printf(false, "%Ba   %s %Fl%Lr%f[change]%E",
			ss.MmToString(ss.TangentArcRadius, false), lf(tw.ScreenChangeTangentArc))
	} else {
		tw.Printf(false, "%Ba   automatic")
	}

	tw.Printf(false, "")
	tw.Printf(false, "  %Fd%f%La%s  choose radius automatically%E",
		lf(tw.ScreenChangeTangentArc), checkMark(!ss.TangentArcManual))
	tw.Printf(false, "  %Fd%f%Lm%s  modify original entities%E",
		lf(tw.ScreenChangeTangentArc), checkMark(ss.TangentArcModify))

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

// setGroupText gives a group value typed text, as one undo step; a
// refusal is shown in a dialog.
func (ss *Application) setGroupText(hg core.HGroup, f sketch.GroupValue, s string) {
	err := ss.SK.UndoRememberFor(func() error {
		return ss.SK.SetGroupText(hg, f, s)
	})
	if err != nil {
		ss.Report(err)
		return
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
}

// setGroupCopies is setGroupText for a group's copies, which also
// re-spaces a rotate's copies (Sketch.SetGroupCopies).
func (ss *Application) setGroupCopies(hg core.HGroup, s string) bool {
	err := ss.SK.UndoRememberFor(func() error {
		return ss.SK.SetGroupCopies(hg, s, true)
	})
	if err != nil {
		ss.Report(err)
		return false
	}
	ss.SK.Unsaved = true
	ss.UndoEnableMenus()
	return true
}

// EditControlDone handles the text entered in the edit control.
func (tw *textWindow) EditControlDone(s string) {
	ss := tw.ss
	sk := ss.SK
	tw.edit.showAgain = false

	switch tw.edit.meaning {
	case editTimesRepeated:
		if e := ss.exprFromString(s); e != nil {
			g := sk.Group(tw.edit.group)
			ss.setGroupCopies(g.H, s)
		}

	case editGroupName:
		if s == "" {
			ss.Error("Group name cannot be empty")
		} else {
			ss.UndoRemember()

			g := sk.Group(tw.edit.group)
			g.Name = s
		}

	case editGroupScale:
		if e := ss.exprFromString(s); e != nil {
			ss.setGroupText(tw.edit.group, sketch.GroupScale, s)
		}

	case editLinkParam:
		tw.editLinkParamDone(s)

	case editHelixPitch: // stored in valB
		if e := ss.exprFromString(s); e != nil {
			ss.setGroupText(tw.edit.group, sketch.GroupPitch, s)
		}

	case editGroupDraft, editChamferDistance2:
		if e := ss.exprFromString(s); e != nil {
			ss.setGroupText(tw.edit.group, sketch.GroupValC, s)
		}

	case editEdgeRadius:
		if e := ss.exprFromString(s); e != nil {
			ss.setEdgeRadiusText(tw.edit.group, tw.edit.i, s)
		}

	case editGroupColor:
		if x, y, z, ok := scanVector3(s); ok {
			rgb := vec3(x, y, z).ClampWithin(0, 1)

			g := sk.Group(tw.shown.group)
			if g == nil {
				break
			}
			c := core.RGBAFromFloat(float32(rgb.X), float32(rgb.Y), float32(rgb.Z), 1)
			if err := sk.SetGroupColor(g.H, c); err != nil {
				ss.Report(err)
				break
			}
			ss.GW.ClearSuper()
		} else {
			ss.Error("Bad format: specify color as r, g, b")
		}

	case editGroupOpacity:
		if e := ss.exprFromString(s); e != nil {
			alpha := ss.evalExpr(e)
			if alpha < 0 || alpha > 1 {
				ss.Error("Opacity must be between zero and one.")
			} else {
				ss.UndoRemember()
				if err := sk.SetGroupOpacity(tw.edit.group, alpha); err != nil {
					ss.Report(err)
				}
				ss.GW.ClearSuper()
			}
		}

	case editGroupDensity:
		// An empty field clears the density.
		density := 0.0
		if strings.TrimSpace(s) != "" {
			e := ss.exprFromString(s)
			if e == nil {
				break
			}
			density = ss.evalExpr(e)
		}
		if density < 0 || math.IsNaN(density) {
			ss.Error("A density is a positive number of g/cm³, or nothing for none.")
			break
		}
		ss.UndoRemember()
		if err := sk.SetGroupDensity(tw.edit.group, density); err != nil {
			ss.Report(err)
			break
		}
		sk.Unsaved = true

	case editTTFText:
		ss.UndoRemember()
		_ = sk.SetTextString(tw.edit.request, s)

	case editStepDimFinish:
		if e := ss.exprFromString(s); e != nil {
			if tw.stepDim.isDistance {
				tw.stepDim.finish = ss.ExprToMm(e)
			} else {
				tw.stepDim.finish = ss.evalExpr(e)
			}
		}

	case editStepDimSteps:
		tw.stepDim.steps = min(300, max(1, atoi(s)))

	case editTangentArcRadius:
		if e := ss.exprFromString(s); e != nil {
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
		if tw.EditControlDoneForDatums(s) {
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
		tw.edit.meaning = editNothing
	}
}

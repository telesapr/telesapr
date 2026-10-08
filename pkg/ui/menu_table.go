package ui

import (
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
)

// menuHandler runs a menu command.
type menuHandler func(ss *Application, cmd Command)

var (
	mView menuHandler = func(ss *Application, c Command) { ss.GW.MenuView(c) }
	mEdit menuHandler = func(ss *Application, c Command) { ss.GW.MenuEdit(c) }
	mClip menuHandler = func(ss *Application, c Command) { ss.GW.MenuClipboard(c) }
	mReq  menuHandler = func(ss *Application, c Command) { ss.GW.MenuRequest(c) }
	mCon  menuHandler = func(ss *Application, c Command) { ss.MenuConstrain(c) }
	mFile menuHandler = func(ss *Application, c Command) { ss.MenuFile(c) }
	mGrp  menuHandler = func(ss *Application, c Command) { ss.MenuGroup(c, "") }
	mAna  menuHandler = func(ss *Application, c Command) { ss.MenuAnalyze(c) }
	mHelp menuHandler = func(ss *Application, c Command) { ss.MenuHelp(c) }
	mScr  menuHandler = func(ss *Application, c Command) { ss.MenuScript(c) }
)

// Accelerator modifier bits in menuEntry.accel.
const (
	shiftMask = 0x100
	ctrlMask  = 0x200
	fnMask    = 0x400
)

// menuEntry is a line of the menu table (C++ MenuEntry).
type menuEntry struct {
	level int    // 0 == on menu bar, 1 == one level down
	label string // or "" for a separator
	cmd   Command
	accel int // keyboard accelerator
	kind  platform.Indicator
	fn    menuHandler
}

const (
	kn    = platform.IndicatorNone
	kc    = platform.IndicatorCheckMark
	kr    = platform.IndicatorRadioMark
	shift = shiftMask
	ctrl  = ctrlMask
	fnKey = fnMask
)

// menuTable is the menu bar: the menus, their entries and what each
// runs. Tests add entries to it.
var menuTable = []menuEntry{
	{0, "&File", CmdNone, 0, kn, nil},
	{1, "&New", CmdNew, ctrl | 'n', kn, mFile},
	{1, "&Open...", CmdOpen, ctrl | 'o', kn, mFile},
	{1, "Open &Recent", CmdOpenRecent, 0, kn, mFile},
	{1, "&Save", CmdSave, ctrl | 's', kn, mFile},
	{1, "Save &As...", CmdSaveAs, ctrl | shift | 's', kn, mFile},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Export &Image...", CmdExportImage, 0, kn, mFile},
	{1, "Export 2d &View...", CmdExportView, 0, kn, mFile},
	{1, "Export 2d &Section...", CmdExportSection, 0, kn, mFile},
	{1, "Drawing S&heet...", CmdDrawingSheet, 0, kn, func(ss *Application, c Command) { ss.MenuDrawingSheet() }},
	{1, "Export &Drawing...", CmdExportDrawing, 0, kn, func(ss *Application, c Command) { ss.MenuExportDrawing() }},
	{1, "Export 3d &Wireframe...", CmdExportWireframe, 0, kn, mFile},
	{1, "Export Triangle &Mesh...", CmdExportMesh, 0, kn, mFile},
	{1, "Export &Surfaces...", CmdExportSurfaces, 0, kn, mFile},
	{1, "Im&port...", CmdImport, 0, kn, mFile},
	{1, "", CmdNone, 0, kn, nil},
	{1, "E&xit", CmdExit, ctrl | 'q', kn, mFile},

	{0, "&Edit", CmdNone, 0, kn, nil},
	{1, "&Undo", CmdUndo, ctrl | 'z', kn, mEdit},
	{1, "&Redo", CmdRedo, ctrl | 'y', kn, mEdit},
	{1, "Re&generate All", CmdRegenAll, ' ', kn, mEdit},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Snap Selection to &Grid", CmdSnapToGrid, '.', kn, mEdit},
	{1, "Rotate Imported &90°", CmdRotate90, '9', kn, mEdit},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Cu&t", CmdCut, ctrl | 'x', kn, mClip},
	{1, "&Copy", CmdCopy, ctrl | 'c', kn, mClip},
	{1, "&Paste", CmdPaste, ctrl | 'v', kn, mClip},
	{1, "Paste &Transformed...", CmdPasteTransform, ctrl | 't', kn, mClip},
	{1, "&Delete", CmdDelete, '\x7f', kn, mClip},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Select &Edge Chain", CmdSelectChain, ctrl | 'e', kn, mEdit},
	{1, "Select &All", CmdSelectAll, ctrl | 'a', kn, mEdit},
	{1, "&Unselect All", CmdUnselectAll, '\x1b', kn, mEdit},
	{1, "", CmdNone, 0, kn, nil},
	{1, "&Line Styles...", CmdEditLineStyles, 0, kn, mEdit},
	{1, "&View Projection...", CmdViewProjection, 0, kn, mEdit},
	{1, "Con&figuration...", CmdConfiguration, 0, kn, mEdit},
	{1, "Find Comman&d...", CmdFindCommand, ctrl | 'k', kn, func(ss *Application, c Command) { ss.MenuFindCommand() }},
	{1, "Para&meters...", CmdParameters, ctrl | shift | 'm', kn, func(ss *Application, c Command) { ss.MenuParameters() }},
	{1, "Move Group Earl&ier", CmdMoveGroupEarlier, 0, kn, func(ss *Application, c Command) { ss.MoveGroup(ss.GW.activeGroupH(), false) }},
	{1, "Move Gr&oup Later", CmdMoveGroupLater, 0, kn, func(ss *Application, c Command) { ss.MoveGroup(ss.GW.activeGroupH(), true) }},
	{1, "Move Group to E&nd", CmdMoveGroupToEnd, 0, kn, func(ss *Application, c Command) { ss.MoveGroupToEnd(ss.GW.activeGroupH()) }},
	{1, "Bro&ken References...", CmdBrokenReferences, 0, kn, func(ss *Application, c Command) { ss.MenuBrokenReferences() }},

	{0, "&View", CmdNone, 0, kn, mView},
	{1, "Zoom &In", CmdZoomIn, '+', kn, mView},
	{1, "Zoom &Out", CmdZoomOut, '-', kn, mView},
	{1, "Zoom To &Fit", CmdZoomToFit, 'f', kn, mView},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Align View to &Workplane", CmdOntoWorkplane, 'w', kn, mView},
	{1, "Nearest &Ortho View", CmdNearestOrtho, fnKey | 2, kn, mView},
	{1, "Nearest &Isometric View", CmdNearestIso, fnKey | 3, kn, mView},
	{1, "&Center View At Point", CmdCenterView, fnKey | 4, kn, mView},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Show Snap &Grid", CmdShowGrid, '>', kc, mView},
	{1, "Darken Inactive Solids", CmdDimSolidModel, 0, kc, mView},
	{1, "Use &Perspective Projection", CmdPerspectiveProj, '`', kc, mView},
	{1, "Show E&xploded View", CmdExplodeSketch, '\\', kc, mView},
	{1, "&Section View", CmdSectionView, ctrl | shift | 'x', kc, func(ss *Application, c Command) { ss.GW.MenuSectionView() }},
	{1, "Show Drawing S&heet", CmdShowDrawingSheet, 0, kc, func(ss *Application, c Command) { ss.GW.MenuShowDrawingSheet() }},
	{1, "Dimension &Units", CmdNone, 0, kn, nil},
	{2, "Dimensions in &Millimeters", CmdUnitsMm, 0, kr, mView},
	{2, "Dimensions in M&eters", CmdUnitsMeters, 0, kr, mView},
	{2, "Dimensions in &Inches", CmdUnitsInches, 0, kr, mView},
	{2, "Dimensions in &Feet and Inches", CmdUnitsFeetInches, 0, kr, mView},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Show &Toolbar", CmdShowToolbar, ctrl | '\t', kc, mView},
	{1, "Show Property Bro&wser", CmdShowTextWnd, '\t', kc, mView},
	{1, "", CmdNone, 0, kn, nil},
	{1, "&Full Screen", CmdFullScreen, ctrl | fnKey | 11, kc, mView},

	{0, "&New Group", CmdNone, 0, kn, mGrp},
	{1, "Sketch In &3d", CmdGroup3D, shift | '3', kn, mGrp},
	{1, "Sketch In New &Workplane", CmdGroupWrkpl, shift | 'w', kn, mGrp},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Step &Translating", CmdGroupTrans, shift | 't', kn, mGrp},
	{1, "Step &Rotating", CmdGroupRot, shift | 'r', kn, mGrp},
	{1, "", CmdNone, 0, kn, nil},
	{1, "E&xtrude", CmdGroupExtrude, shift | 'x', kn, mGrp},
	{1, "&Helix", CmdGroupHelix, shift | 'h', kn, mGrp},
	{1, "&Lathe", CmdGroupLathe, shift | 'l', kn, mGrp},
	{1, "Re&volve", CmdGroupRevolve, shift | 'v', kn, mGrp},
	{1, "Screw Thr&ead...", CmdGroupThread, shift | 'e', kn, func(ss *Application, c Command) { ss.MenuThread() }},
	{1, "Swee&p Along Path", CmdGroupSweep, shift | 'u', kn, mGrp},
	{1, "", CmdNone, 0, kn, nil},
	{1, "&Mirror", CmdGroupMirror, shift | 'y', kn, mGrp},
	{1, "&Chamfer Edges", CmdGroupChamfer, shift | 'q', kn, mGrp},
	{1, "&Fillet Edges", CmdGroupFillet, shift | 'f', kn, mGrp},
	{1, "Loft To Selected Pr&ofile", CmdGroupLoft, shift | 'o', kn, mGrp},
	{1, "&Shell Solid", CmdGroupShell, shift | 'g', kn, mGrp},
	{1, "Pattern Along Pat&h", CmdGroupPathPattern, 0, kn, mGrp},
	{1, "&Draft Selected Sides", CmdDraftSelectedSides, 0, kn, func(ss *Application, c Command) { ss.draftSelectedSides() }},
	{1, "Guide Loft by Selected &Curves", CmdLoftGuides, 0, kn, func(ss *Application, c Command) { ss.loftGuidesFromSelection() }},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Link / Assemble...", CmdGroupLink, shift | 'i', kn, mGrp},
	{1, "Link Recent", CmdGroupRecent, 0, kn, mGrp},

	{0, "&Sketch", CmdNone, 0, kn, mReq},
	{1, "In &Workplane", CmdSelWorkplane, '2', kr, mReq},
	{1, "Anywhere In &3d", CmdFreeIn3D, '3', kr, mReq},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Datum &Point", CmdDatumPoint, 'p', kn, mReq},
	{1, "Wor&kplane", CmdWorkplane, 0, kn, mReq},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Line &Segment", CmdLineSegment, 's', kn, mReq},
	{1, "C&onstruction Line Segment", CmdConstrSegment, shift | 's', kn, mReq},
	{1, "&Rectangle", CmdRectangle, 'r', kn, mReq},
	{1, "&Circle", CmdCircle, 'c', kn, mReq},
	{1, "&Arc of a Circle", CmdArc, 'a', kn, mReq},
	{1, "&Bezier Cubic Spline", CmdCubic, 'b', kn, mReq},
	{1, "", CmdNone, 0, kn, nil},
	{1, "&Text in TrueType Font", CmdTTFText, 't', kn, mReq},
	{1, "I&mage", CmdImage, 0, kn, mReq},
	{1, "", CmdNone, 0, kn, nil},
	{1, "To&ggle Construction", CmdConstruction, 'g', kn, mReq},
	{1, "Ta&ngent Arc at Point", CmdTangentArc, shift | 'a', kn, mReq},
	{1, "Split Curves at &Intersection", CmdSplitCurves, 'i', kn, mReq},

	{0, "&Constrain", CmdNone, 0, kn, mCon},
	{1, "&Distance / Diameter", CmdDistanceDia, 'd', kn, mCon},
	{1, "Re&ference Dimension", CmdRefDistance, shift | 'd', kn, mCon},
	{1, "A&ngle / Equal Angle", CmdAngle, 'n', kn, mCon},
	{1, "Reference An&gle", CmdRefAngle, shift | 'n', kn, mCon},
	{1, "Other S&upplementary Angle", CmdOtherAngle, 'u', kn, mCon},
	{1, "Toggle R&eference Dim", CmdReference, 'e', kn, mCon},
	{1, "", CmdNone, 0, kn, nil},
	{1, "&Horizontal", CmdHorizontal, 'h', kn, mCon},
	{1, "&Vertical", CmdVertical, 'v', kn, mCon},
	{1, "", CmdNone, 0, kn, nil},
	{1, "&On Point / Curve / Plane", CmdOnEntity, 'o', kn, mCon},
	{1, "E&qual Length / Radius", CmdEqual, 'q', kn, mCon},
	{1, "Length / Arc Ra&tio", CmdRatio, 'z', kn, mCon},
	{1, "Length / Arc Diff&erence", CmdDifference, 'j', kn, mCon},
	{1, "At &Midpoint", CmdAtMidpoint, 'm', kn, mCon},
	{1, "S&ymmetric", CmdSymmetric, 'y', kn, mCon},
	{1, "Para&llel / Tangent", CmdParallel, 'l', kn, mCon},
	{1, "&Perpendicular", CmdPerpendicular, '[', kn, mCon},
	{1, "Same Orient&ation", CmdOrientedSame, 'x', kn, mCon},
	{1, "Lock Point Where &Dragged", CmdWhereDragged, ']', kn, mCon},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Comment", CmdComment, ';', kn, mCon},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Mate &Concentric", CmdMateConcentric, 0, kn, func(ss *Application, c Command) { ss.MenuMate(c) }},
	{1, "Mate &Flush", CmdMateFlush, 0, kn, func(ss *Application, c Command) { ss.MenuMate(c) }},
	{1, "Mate &Hinge", CmdMateHinge, 0, kn, func(ss *Application, c Command) { ss.MenuMate(c) }},

	{0, "&Analyze", CmdNone, 0, kn, mAna},
	{1, "Measure &Volume", CmdVolume, ctrl | shift | 'v', kn, mAna},
	{1, "Measure A&rea", CmdArea, ctrl | shift | 'a', kn, mAna},
	{1, "Measure &Perimeter", CmdPerimeter, ctrl | shift | 'p', kn, mAna},
	{1, "Show &Interfering Parts", CmdInterference, ctrl | shift | 'i', kn, mAna},
	{1, "Show &Naked Edges", CmdNakedEdges, ctrl | shift | 'n', kn, mAna},
	{1, "Show &Center of Mass", CmdCenterOfMass, ctrl | shift | 'c', kn, mAna},
	{1, "Check S&olid", CmdCheckSolid, ctrl | shift | 'o', kn, mAna},
	{1, "&Bill of Materials", CmdBillOfMaterials, 0, kn, mAna},
	{1, "Measure Se&lection", CmdMeasure, ctrl | shift | 'e', kc, func(ss *Application, c Command) { ss.MenuMeasure() }},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Show &Underconstrained Points", CmdShowDOF, ctrl | shift | 'f', kn, mAna},
	{1, "", CmdNone, 0, kn, nil},
	{1, "&Trace Point", CmdTracePt, ctrl | shift | 't', kn, mAna},
	{1, "&Stop Tracing...", CmdStopTracing, ctrl | shift | 's', kn, mAna},
	{1, "Step &Dimension...", CmdStepDim, ctrl | shift | 'd', kn, mAna},
	{1, "Motion St&udy...", CmdMotionStudy, 0, kn, func(ss *Application, c Command) { ss.MenuMotionStudy() }},

	// C++ has no scripts; the menu is ours.
	{0, "S&cript", CmdNone, 0, kn, mScr},
	{1, "Show &Console", CmdScriptConsole, ctrl | shift | 'k', kc, mScr},
	{1, "", CmdNone, 0, kn, nil},
	{1, "&Run File...", CmdScriptRun, ctrl | shift | 'r', kn, mScr},
	{1, "&Stop", CmdScriptStop, 0, kn, mScr},
	{1, "", CmdNone, 0, kn, nil},
	{1, "Allow &AI Connections", CmdScriptAgent, 0, kc, mScr},

	{0, "&Help", CmdNone, 0, kn, mHelp},
	{1, "&User's Guide", CmdWebsite, 0, kn, mHelp},
	{1, "Go to &GitHub", CmdGithub, 0, kn, mHelp},
	{1, "&About", CmdAbout, 0, kn, mHelp},
}

// MenuItem is an entry of the main menu: the labels of the menus it
// is in and its own, without the & that marks a mnemonic, and its
// command.
type MenuItem struct {
	Path []string
	Cmd  Command
}

// MenuItems lists the entries of the main menu that run a command, in
// the menus' order.
func MenuItems() []MenuItem {
	var items []MenuItem
	var path []string
	for _, m := range menuTable {
		if m.label == "" {
			continue
		}
		path = append(path[:m.level], strings.ReplaceAll(m.label, "&", ""))
		if m.cmd != CmdNone {
			items = append(items, MenuItem{Path: append([]string(nil), path...), Cmd: m.cmd})
		}
	}
	return items
}

package ui

import (
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
)

// menuHandler runs a menu command.
type menuHandler func(ss *Application, cmd Command)

var (
	mView     menuHandler = func(ss *Application, c Command) { ss.GW.MenuView(c) }
	mEdit     menuHandler = func(ss *Application, c Command) { ss.GW.MenuEdit(c) }
	mClip     menuHandler = func(ss *Application, c Command) { ss.GW.MenuClipboard(c) }
	mSketch   menuHandler = func(ss *Application, c Command) { ss.GW.MenuSketch(c) }
	mCon      menuHandler = func(ss *Application, c Command) { ss.MenuConstrain(c) }
	mFile     menuHandler = func(ss *Application, c Command) { ss.MenuFile(c) }
	mFeatures menuHandler = func(ss *Application, c Command) { ss.MenuFeatures(c, "") }
	mAna      menuHandler = func(ss *Application, c Command) { ss.MenuAnalyze(c) }
	mHelp     menuHandler = func(ss *Application, c Command) { ss.MenuHelp(c) }
	mScr      menuHandler = func(ss *Application, c Command) { ss.MenuScript(c) }
	mShow     menuHandler = func(ss *Application, c Command) { ss.GW.MenuShow(c) }
)

// Accelerator modifier bits in menuEntry.accel.
const (
	shiftMask = 0x100
	ctrlMask  = 0x200
	fnMask    = 0x400
)

// menuEntry is a line of the menu table.
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
// runs. Tests add entries to it. The menus run in the order of the work:
// sketch, constrain, make features, assemble, analyse; an entry that was
// once elsewhere is still found by RunCommand under its former label
// (formerCommandLabels).
var menuTable = []menuEntry{
	{0, "&File", cmdNone, 0, kn, nil},
	{1, "&New", cmdNew, ctrl | 'n', kn, mFile},
	{1, "&Open...", cmdOpen, ctrl | 'o', kn, mFile},
	{1, "Open &Recent", cmdOpenRecent, 0, kn, mFile},
	{1, "&Save", cmdSave, ctrl | 's', kn, mFile},
	{1, "Save &As...", cmdSaveAs, ctrl | shift | 's', kn, mFile},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Im&port...", cmdImport, 0, kn, mFile},
	{1, "&Export", cmdNone, 0, kn, nil},
	{2, "&Image (.png)...", cmdExportImage, 0, kn, mFile},
	{2, "2d &View (.pdf, .eps, .svg, .dxf, .plt, .ngc)...", cmdExportView, 0, kn, mFile},
	{2, "2d &Section (.pdf, .eps, .svg, .dxf, .plt, .ngc)...", cmdExportSection, 0, kn, mFile},
	{2, "3d &Wireframe (.step, .dxf)...", cmdExportWireframe, 0, kn, mFile},
	{2, "Triangle &Mesh (.stl, .3mf, .obj, .html, .js, .wrl)...", cmdExportMesh, 0, kn, mFile},
	{2, "Sur&faces (.step)...", cmdExportSurfaces, 0, kn, mFile},
	{2, "&Drawing (.pdf, .eps, .svg, .dxf, .plt, .ngc)...", cmdExportDrawing, 0, kn, func(ss *Application, c Command) { ss.MenuExportDrawing() }},
	{1, "", cmdNone, 0, kn, nil},
	{1, "E&xit", cmdExit, ctrl | 'q', kn, mFile},

	{0, "&Edit", cmdNone, 0, kn, nil},
	{1, "&Undo", cmdUndo, ctrl | 'z', kn, mEdit},
	{1, "&Redo", cmdRedo, ctrl | 'y', kn, mEdit},
	{1, "Re&generate All", cmdRegenAll, ' ', kn, mEdit},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Cu&t", cmdCut, ctrl | 'x', kn, mClip},
	{1, "&Copy", cmdCopy, ctrl | 'c', kn, mClip},
	{1, "&Paste", cmdPaste, ctrl | 'v', kn, mClip},
	{1, "Paste &Transformed...", cmdPasteTransform, ctrl | 't', kn, mClip},
	{1, "&Delete", cmdDelete, '\x7f', kn, mClip},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Select &Edge Chain", cmdSelectChain, ctrl | 'e', kn, mEdit},
	{1, "Select &All", cmdSelectAll, ctrl | 'a', kn, mEdit},
	{1, "&Unselect All", cmdUnselectAll, '\x1b', kn, mEdit},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Snap Selection to &Grid", cmdSnapToGrid, '.', kn, mEdit},
	{1, "Rotate Imported &90°", cmdRotate90, '9', kn, mEdit},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Find Comman&d...", cmdFindCommand, ctrl | 'k', kn, func(ss *Application, c Command) { ss.MenuFindCommand() }},
	{1, "Para&meters...", cmdParameters, ctrl | shift | 'm', kn, func(ss *Application, c Command) { ss.MenuParameters() }},
	{1, "", cmdNone, 0, kn, nil},
	{1, "&Line Styles...", cmdEditLineStyles, 0, kn, mEdit},
	{1, "&View Projection...", cmdViewProjection, 0, kn, mEdit},
	{1, "Pre&ferences...", cmdConfiguration, 0, kn, mEdit},

	{0, "&View", cmdNone, 0, kn, mView},
	{1, "Zoom &In", cmdZoomIn, '+', kn, mView},
	{1, "Zoom &Out", cmdZoomOut, '-', kn, mView},
	{1, "Zoom To &Fit", cmdZoomToFit, 'f', kn, mView},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Align View to &Workplane", cmdOntoWorkplane, 'w', kn, mView},
	{1, "Nearest &Ortho View", cmdNearestOrtho, fnKey | 2, kn, mView},
	{1, "Nearest &Isometric View", cmdNearestIso, fnKey | 3, kn, mView},
	{1, "&Center View At Point", cmdCenterView, fnKey | 4, kn, mView},
	{1, "", cmdNone, 0, kn, nil},
	{1, "S&how", cmdNone, 0, kn, nil},
	{2, "&Workplanes of Inactive Groups", cmdShowWorkplanes, 0, kc, mShow},
	{2, "&Normals", cmdShowNormals, 0, kc, mShow},
	{2, "&Points", cmdShowPoints, 0, kc, mShow},
	{2, "&Construction Entities", cmdShowConstruction, 0, kc, mShow},
	{2, "", cmdNone, 0, kn, nil},
	{2, "Constraints &and Dimensions", cmdShowConstraintsAll, 0, kr, mShow},
	{2, "&Dimensions Only", cmdShowConstraintsDim, 0, kr, mShow},
	{2, "No Constrain&ts", cmdShowConstraintsNone, 0, kr, mShow},
	{2, "&Reference Dimensions", cmdShowReferences, 0, kc, mShow},
	{2, "Co&mments", cmdShowComments, 0, kc, mShow},
	{2, "", cmdNone, 0, kn, nil},
	{2, "&Shaded Solid", cmdShowShaded, 0, kc, mShow},
	{2, "Selectable &Faces", cmdShowFaces, 0, kc, mShow},
	{2, "Solid &Edges", cmdShowEdges, 0, kc, mShow},
	{2, "Solid &Outline", cmdShowOutlines, 0, kc, mShow},
	{2, "Triangle &Mesh", cmdShowMesh, 0, kc, mShow},
	{2, "", cmdNone, 0, kn, nil},
	{2, "Hidden Lines &Invisible", cmdShowOccludedInvisible, 0, kr, mShow},
	{2, "Hidden Lines Stipp&led", cmdShowOccludedStippled, 0, kr, mShow},
	{2, "Hidden Lines &Visible", cmdShowOccludedVisible, 0, kr, mShow},
	{1, "Show Snap &Grid", cmdShowGrid, '>', kc, mView},
	{1, "Darken Inactive Solids", cmdDimSolidModel, 0, kc, mView},
	{1, "Use &Perspective Projection", cmdPerspectiveProj, '`', kc, mView},
	{1, "Show E&xploded View", cmdExplodeSketch, '\\', kc, mView},
	{1, "&Section View", cmdSectionView, ctrl | shift | 'x', kc, func(ss *Application, c Command) { ss.GW.MenuSectionView() }},
	{1, "Show &Drawing Sheet", cmdShowDrawingSheet, 0, kc, func(ss *Application, c Command) { ss.GW.MenuShowDrawingSheet() }},
	{1, "Drawing Sh&eet...", cmdDrawingSheet, 0, kn, func(ss *Application, c Command) { ss.MenuDrawingSheet() }},
	{1, "Dimension &Units", cmdNone, 0, kn, nil},
	{2, "Dimensions in &Millimeters", cmdUnitsMm, 0, kr, mView},
	{2, "Dimensions in M&eters", cmdUnitsMeters, 0, kr, mView},
	{2, "Dimensions in &Inches", cmdUnitsInches, 0, kr, mView},
	{2, "Dimensions in &Feet and Inches", cmdUnitsFeetInches, 0, kr, mView},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Show &Toolbar", cmdShowToolbar, ctrl | '\t', kc, mView},
	{1, "Show Property Bro&wser", cmdShowTextWnd, '\t', kc, mView},
	{1, "", cmdNone, 0, kn, nil},
	{1, "&Full Screen", cmdFullScreen, ctrl | fnKey | 11, kc, mView},

	{0, "&Sketch", cmdNone, 0, kn, mSketch},
	{1, "In &Workplane", cmdSelWorkplane, '2', kr, mSketch},
	{1, "Anywhere In &3d", cmdFreeIn3D, '3', kr, mSketch},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Datum &Point", cmdDatumPoint, 'p', kn, mSketch},
	{1, "Wor&kplane", cmdWorkplane, 0, kn, mSketch},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Line &Segment", cmdLineSegment, 's', kn, mSketch},
	{1, "C&onstruction Line Segment", cmdConstrSegment, shift | 's', kn, mSketch},
	{1, "&Rectangle", cmdRectangle, 'r', kn, mSketch},
	{1, "&Circle", cmdCircle, 'c', kn, mSketch},
	{1, "&Arc of a Circle", cmdArc, 'a', kn, mSketch},
	{1, "&Bezier Cubic Spline", cmdCubic, 'b', kn, mSketch},
	{1, "", cmdNone, 0, kn, nil},
	{1, "&Text in TrueType Font", cmdTTFText, 't', kn, mSketch},
	{1, "I&mage", cmdImage, 0, kn, mSketch},
	{1, "", cmdNone, 0, kn, nil},
	{1, "To&ggle Construction", cmdConstruction, 'g', kn, mSketch},
	{1, "Ta&ngent Arc at Point", cmdTangentArc, shift | 'a', kn, mSketch},
	{1, "Split Curves at &Intersection", cmdSplitCurves, 'i', kn, mSketch},

	{0, "&Constrain", cmdNone, 0, kn, mCon},
	{1, "&Distance / Diameter", cmdDistanceDia, 'd', kn, mCon},
	{1, "Re&ference Dimension", cmdRefDistance, shift | 'd', kn, mCon},
	{1, "M&inimum Dimension", cmdMinDistance, 0, kn, mCon},
	{1, "Ma&ximum Dimension", cmdMaxDistance, 0, kn, mCon},
	{1, "A&ngle / Equal Angle", cmdAngle, 'n', kn, mCon},
	{1, "Reference An&gle", cmdRefAngle, shift | 'n', kn, mCon},
	{1, "Other S&upplementary Angle", cmdOtherAngle, 'u', kn, mCon},
	{1, "Toggle R&eference Dim", cmdReference, 'e', kn, mCon},
	{1, "", cmdNone, 0, kn, nil},
	{1, "&Horizontal", cmdHorizontal, 'h', kn, mCon},
	{1, "&Vertical", cmdVertical, 'v', kn, mCon},
	{1, "", cmdNone, 0, kn, nil},
	{1, "&On Point / Curve / Plane", cmdOnEntity, 'o', kn, mCon},
	{1, "On &Segment", cmdOnSegment, 0, kn, mCon},
	{1, "E&qual Length / Radius", cmdEqual, 'q', kn, mCon},
	{1, "Length / Arc Ra&tio", cmdRatio, 'z', kn, mCon},
	{1, "Length / Arc Diff&erence", cmdDifference, 'j', kn, mCon},
	{1, "At &Midpoint", cmdAtMidpoint, 'm', kn, mCon},
	{1, "S&ymmetric", cmdSymmetric, 'y', kn, mCon},
	{1, "Para&llel / Tangent", cmdParallel, 'l', kn, mCon},
	{1, "&Perpendicular", cmdPerpendicular, '[', kn, mCon},
	{1, "Same Orient&ation", cmdOrientedSame, 'x', kn, mCon},
	{1, "Lock Point Where &Dragged", cmdWhereDragged, ']', kn, mCon},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Comment", cmdComment, ';', kn, mCon},

	{0, "Fea&tures", cmdNone, 0, kn, mFeatures},
	{1, "Sketch In &3d", cmdGroup3D, shift | '3', kn, mFeatures},
	{1, "Sketch In New &Workplane", cmdGroupWrkpl, shift | 'w', kn, mFeatures},
	{1, "Sketch On Face", cmdGroupSketchOnFace, shift | 'b', kn, mFeatures},
	{1, "", cmdNone, 0, kn, nil},
	{1, "E&xtrude", cmdGroupExtrude, shift | 'x', kn, mFeatures},
	{1, "Re&volve", cmdGroupRevolve, shift | 'v', kn, mFeatures},
	{1, "&Lathe", cmdGroupLathe, shift | 'l', kn, mFeatures},
	{1, "&Helix", cmdGroupHelix, shift | 'h', kn, mFeatures},
	{1, "Swee&p Along Path", cmdGroupSweep, shift | 'u', kn, mFeatures},
	{1, "Loft To Selected Pr&ofile", cmdGroupLoft, shift | 'o', kn, mFeatures},
	{1, "Guide Loft by Selected &Curves", cmdLoftGuides, 0, kn, func(ss *Application, c Command) { ss.loftGuidesFromSelection() }},
	{1, "Screw Thr&ead...", cmdGroupThread, shift | 'e', kn, func(ss *Application, c Command) { ss.MenuThread() }},
	{1, "", cmdNone, 0, kn, nil},
	{1, "&Fillet Edges", cmdGroupFillet, shift | 'f', kn, mFeatures},
	{1, "Chamfer E&dges", cmdGroupChamfer, shift | 'q', kn, mFeatures},
	{1, "&Shell Solid", cmdGroupShell, shift | 'g', kn, mFeatures},
	{1, "Draft Selected S&ides", cmdDraftSelectedSides, 0, kn, func(ss *Application, c Command) { ss.draftSelectedSides() }},
	{1, "&Mirror", cmdGroupMirror, shift | 'y', kn, mFeatures},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Step &Translating", cmdGroupTrans, shift | 't', kn, mFeatures},
	{1, "Step &Rotating", cmdGroupRot, shift | 'r', kn, mFeatures},
	{1, "Pattern Alon&g Path", cmdGroupPathPattern, 0, kn, mFeatures},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Move Group E&arlier", cmdMoveGroupEarlier, 0, kn, func(ss *Application, c Command) { ss.MoveGroup(ss.GW.activeGroupH(), false) }},
	{1, "Move Gro&up Later", cmdMoveGroupLater, 0, kn, func(ss *Application, c Command) { ss.MoveGroup(ss.GW.activeGroupH(), true) }},
	{1, "Move Group to E&nd", cmdMoveGroupToEnd, 0, kn, func(ss *Application, c Command) { ss.MoveGroupToEnd(ss.GW.activeGroupH()) }},
	{1, "Bro&ken References...", cmdBrokenReferences, 0, kn, func(ss *Application, c Command) { ss.MenuBrokenReferences() }},

	{0, "Asse&mbly", cmdNone, 0, kn, nil},
	{1, "&Link / Assemble...", cmdGroupLink, shift | 'i', kn, mFeatures},
	{1, "Link &Recent", cmdGroupRecent, 0, kn, mFeatures},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Mate &Concentric", cmdMateConcentric, 0, kn, func(ss *Application, c Command) { ss.MenuMate(c) }},
	{1, "Mate &Flush", cmdMateFlush, 0, kn, func(ss *Application, c Command) { ss.MenuMate(c) }},
	{1, "Mate &Hinge", cmdMateHinge, 0, kn, func(ss *Application, c Command) { ss.MenuMate(c) }},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Show &Interfering Parts", cmdInterference, ctrl | shift | 'i', kn, mAna},
	{1, "&Bill of Materials", cmdBillOfMaterials, 0, kn, mAna},
	{1, "Motion St&udy...", cmdMotionStudy, 0, kn, func(ss *Application, c Command) { ss.MenuMotionStudy() }},

	{0, "&Analyze", cmdNone, 0, kn, mAna},
	{1, "Measure Se&lection", cmdMeasure, ctrl | shift | 'e', kc, func(ss *Application, c Command) { ss.MenuMeasure() }},
	{1, "Measure &Volume", cmdVolume, ctrl | shift | 'v', kn, mAna},
	{1, "Measure A&rea", cmdArea, ctrl | shift | 'a', kn, mAna},
	{1, "Measure &Perimeter", cmdPerimeter, ctrl | shift | 'p', kn, mAna},
	{1, "Show &Center of Mass", cmdCenterOfMass, ctrl | shift | 'c', kn, mAna},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Check S&olid", cmdCheckSolid, ctrl | shift | 'o', kn, mAna},
	{1, "Group Dependencies", cmdGroupDependencies, 0, kn, mAna},
	{1, "Ray-Cast Check", cmdRayCheck, 0, kn, mAna},
	{1, "Show &Naked Edges", cmdNakedEdges, ctrl | shift | 'n', kn, mAna},
	{1, "Show &Underconstrained Points", cmdShowDOF, ctrl | shift | 'f', kn, mAna},
	{1, "", cmdNone, 0, kn, nil},
	{1, "&Trace Point", cmdTracePt, ctrl | shift | 't', kn, mAna},
	{1, "&Stop Tracing...", cmdStopTracing, 0, kn, mAna}, // Ctrl+Shift+S is Save As
	{1, "Step &Dimension...", cmdStepDim, ctrl | shift | 'd', kn, mAna},

	{0, "Sc&ript", cmdNone, 0, kn, mScr},
	{1, "Show &Console", cmdScriptConsole, ctrl | shift | 'k', kc, mScr},
	{1, "", cmdNone, 0, kn, nil},
	{1, "&Run File...", cmdScriptRun, ctrl | shift | 'r', kn, mScr},
	{1, "&Stop", cmdScriptStop, 0, kn, mScr},
	{1, "", cmdNone, 0, kn, nil},
	{1, "Allow &AI Connections", cmdScriptAgent, 0, kc, mScr},

	{0, "&Help", cmdNone, 0, kn, mHelp},
	{1, "&User's Guide", cmdWebsite, 0, kn, mHelp},
	{1, "Go to &GitHub", cmdGithub, 0, kn, mHelp},
	{1, "&About", cmdAbout, 0, kn, mHelp},
}

// accelAliases are further keys that run a command, beside the one the
// menu shows: Ctrl+Shift+Z is Redo where Ctrl+Y is not the custom.
var accelAliases = []struct {
	cmd   Command
	accel int
}{
	{cmdRedo, ctrl | shift | 'z'},
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
		if m.cmd != cmdNone {
			items = append(items, MenuItem{Path: append([]string(nil), path...), Cmd: m.cmd})
		}
	}
	return items
}

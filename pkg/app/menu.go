package app

import (
	"strings"
)

// MenuKind describes the visual indicator requested by the menu entry.
type MenuKind int

const (
	MenuKindNone MenuKind = iota
	MenuKindCheck
	MenuKindRadio
)

// MenuHandler is implemented by UI handler methods that respond to a command.
type MenuHandler func(*UI, Command)

// MenuChildBuilder constructs child menu items dynamically when needed.
type MenuChildBuilder func(*UI, *MenuItem) []*MenuItem

// MenuEntry mirrors the historic static menu table from the C++ implementation.
type MenuEntry struct {
	Level        int
	Label        string
	Command      Command
	Kind         MenuKind
	Handler      MenuHandler
	Separator    bool
	ChildBuilder MenuChildBuilder
}

// MenuItem is the runtime representation of a menu entry.
type MenuItem struct {
	Label        string
	Command      Command
	Kind         MenuKind
	Handler      MenuHandler
	OnTriggered  func(*UI)
	Children     []*MenuItem
	Separator    bool
	ChildBuilder MenuChildBuilder
}

// menuEntries declares the hierarchical menu structure translated from the
// legacy C++ table. It is expanded into runtime tree nodes during UI startup.
var menuEntries = []MenuEntry{
	// File menu
	{Level: 0, Label: "&File", Command: CommandNone, Kind: MenuKindNone},
	{Level: 1, Label: "&New", Command: CommandNew, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "&Open...", Command: CommandOpen, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "Open &Recent", Command: CommandOpenRecent, Kind: MenuKindNone, ChildBuilder: (*UI).buildOpenRecentMenu},
	{Level: 1, Label: "&Save", Command: CommandSave, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "Save &As...", Command: CommandSaveAs, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Export &Image...", Command: CommandExportImage, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "Export 2d &View...", Command: CommandExportView, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "Export 2d &Section...", Command: CommandExportSection, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "Export 3d &Wireframe...", Command: CommandExportWireframe, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "Export Triangle &Mesh...", Command: CommandExportMesh, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "Export &Surfaces...", Command: CommandExportSurfaces, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Label: "Im&port...", Command: CommandImport, Kind: MenuKindNone, Handler: (*UI).MenuFile},
	{Level: 1, Separator: true},
	{Level: 1, Label: "E&xit", Command: CommandExit, Kind: MenuKindNone, Handler: (*UI).MenuFile},

	// Edit menu
	{Level: 0, Label: "&Edit", Command: CommandNone, Kind: MenuKindNone},
	{Level: 1, Label: "&Undo", Command: CommandUndo, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Label: "&Redo", Command: CommandRedo, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Label: "Re&generate All", Command: CommandRegenerateAll, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Snap Selection to &Grid", Command: CommandSnapToGrid, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Label: "Rotate Imported &90°", Command: CommandRotate90, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Cu&t", Command: CommandCut, Kind: MenuKindNone, Handler: (*UI).MenuClipboard},
	{Level: 1, Label: "&Copy", Command: CommandCopy, Kind: MenuKindNone, Handler: (*UI).MenuClipboard},
	{Level: 1, Label: "&Paste", Command: CommandPaste, Kind: MenuKindNone, Handler: (*UI).MenuClipboard},
	{Level: 1, Label: "Paste &Transformed...", Command: CommandPasteTransform, Kind: MenuKindNone, Handler: (*UI).MenuClipboard},
	{Level: 1, Label: "&Delete", Command: CommandDelete, Kind: MenuKindNone, Handler: (*UI).MenuClipboard},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Select &Edge Chain", Command: CommandSelectChain, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Label: "Select &All", Command: CommandSelectAll, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Label: "&Unselect All", Command: CommandUnselectAll, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Separator: true},
	{Level: 1, Label: "&Line Styles...", Command: CommandEditLineStyles, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Label: "&View Projection...", Command: CommandViewProjection, Kind: MenuKindNone, Handler: (*UI).MenuEdit},
	{Level: 1, Label: "Con&figuration...", Command: CommandConfiguration, Kind: MenuKindNone, Handler: (*UI).MenuEdit},

	// View menu
	{Level: 0, Label: "&View", Command: CommandNone, Kind: MenuKindNone},
	{Level: 1, Label: "Zoom &In", Command: CommandZoomIn, Kind: MenuKindNone, Handler: (*UI).MenuView},
	{Level: 1, Label: "Zoom &Out", Command: CommandZoomOut, Kind: MenuKindNone, Handler: (*UI).MenuView},
	{Level: 1, Label: "Zoom To &Fit", Command: CommandZoomToFit, Kind: MenuKindNone, Handler: (*UI).MenuView},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Align View to &Workplane", Command: CommandOntoWorkplane, Kind: MenuKindNone, Handler: (*UI).MenuView},
	{Level: 1, Label: "Nearest &Ortho View", Command: CommandNearestOrtho, Kind: MenuKindNone, Handler: (*UI).MenuView},
	{Level: 1, Label: "Nearest &Isometric View", Command: CommandNearestIso, Kind: MenuKindNone, Handler: (*UI).MenuView},
	{Level: 1, Label: "&Center View At Point", Command: CommandCenterView, Kind: MenuKindNone, Handler: (*UI).MenuView},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Show Snap &Grid", Command: CommandShowGrid, Kind: MenuKindCheck, Handler: (*UI).MenuView},
	{Level: 1, Label: "Darken Inactive Solids", Command: CommandDimSolidModel, Kind: MenuKindCheck, Handler: (*UI).MenuView},
	{Level: 1, Label: "Use &Perspective Projection", Command: CommandPerspectiveProj, Kind: MenuKindCheck, Handler: (*UI).MenuView},
	{Level: 1, Label: "Show E&xploded View", Command: CommandExplodeSketch, Kind: MenuKindCheck, Handler: (*UI).MenuView},
	{Level: 1, Label: "Dimension &Units", Command: CommandNone, Kind: MenuKindNone},
	{Level: 2, Label: "Dimensions in &Millimeters", Command: CommandUnitsMillimeters, Kind: MenuKindRadio, Handler: (*UI).MenuView},
	{Level: 2, Label: "Dimensions in M&eters", Command: CommandUnitsMeters, Kind: MenuKindRadio, Handler: (*UI).MenuView},
	{Level: 2, Label: "Dimensions in &Inches", Command: CommandUnitsInches, Kind: MenuKindRadio, Handler: (*UI).MenuView},
	{Level: 2, Label: "Dimensions in &Feet and Inches", Command: CommandUnitsFeetInches, Kind: MenuKindRadio, Handler: (*UI).MenuView},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Show &Toolbar", Command: CommandShowToolbar, Kind: MenuKindCheck, Handler: (*UI).MenuView},
	{Level: 1, Label: "Show Property Bro&wser", Command: CommandShowTextWindow, Kind: MenuKindCheck, Handler: (*UI).MenuView},
	{Level: 1, Separator: true},
	{Level: 1, Label: "&Full Screen", Command: CommandFullScreen, Kind: MenuKindCheck, Handler: (*UI).MenuView},

	// New Group menu
	{Level: 0, Label: "&New Group", Command: CommandNone, Kind: MenuKindNone},
	{Level: 1, Label: "Sketch In &3d", Command: CommandGroup3D, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Label: "Sketch In New &Workplane", Command: CommandGroupWorkplane, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Step &Translating", Command: CommandGroupTranslate, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Label: "Step &Rotating", Command: CommandGroupRotate, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Separator: true},
	{Level: 1, Label: "E&xtrude", Command: CommandGroupExtrude, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Label: "&Helix", Command: CommandGroupHelix, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Label: "&Lathe", Command: CommandGroupLathe, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Label: "Re&volve", Command: CommandGroupRevolve, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Link / Assemble...", Command: CommandGroupLink, Kind: MenuKindNone, Handler: (*UI).MenuGroup},
	{Level: 1, Label: "Link Recent", Command: CommandGroupRecent, Kind: MenuKindNone, ChildBuilder: (*UI).buildLinkRecentMenu},

	// Sketch menu
	{Level: 0, Label: "&Sketch", Command: CommandNone, Kind: MenuKindNone},
	{Level: 1, Label: "In &Workplane", Command: CommandSelectWorkplane, Kind: MenuKindRadio, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "Anywhere In &3d", Command: CommandFreeIn3D, Kind: MenuKindRadio, Handler: (*UI).MenuRequest},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Datum &Point", Command: CommandDatumPoint, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "Wor&kplane", Command: CommandWorkplane, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Line &Segment", Command: CommandLineSegment, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "C&onstruction Line Segment", Command: CommandConstructionSegment, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "&Rectangle", Command: CommandRectangle, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "&Circle", Command: CommandCircle, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "&Arc of a Circle", Command: CommandArc, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "&Bezier Cubic Spline", Command: CommandCubic, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Separator: true},
	{Level: 1, Label: "&Text in TrueType Font", Command: CommandTrueTypeText, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "I&mage", Command: CommandImage, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Separator: true},
	{Level: 1, Label: "To&ggle Construction", Command: CommandToggleConstruction, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "Ta&ngent Arc at Point", Command: CommandTangentArc, Kind: MenuKindNone, Handler: (*UI).MenuRequest},
	{Level: 1, Label: "Split Curves at &Intersection", Command: CommandSplitCurves, Kind: MenuKindNone, Handler: (*UI).MenuRequest},

	// Constrain menu
	{Level: 0, Label: "&Constrain", Command: CommandNone, Kind: MenuKindNone},
	{Level: 1, Label: "&Distance / Diameter", Command: CommandDistanceDiameter, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Re&ference Dimension", Command: CommandReferenceDistance, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "A&ngle / Equal Angle", Command: CommandAngle, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Reference An&gle", Command: CommandReferenceAngle, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Other S&upplementary Angle", Command: CommandOtherAngle, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Toggle R&eference Dim", Command: CommandToggleReference, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Separator: true},
	{Level: 1, Label: "&Horizontal", Command: CommandHorizontal, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "&Vertical", Command: CommandVertical, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Separator: true},
	{Level: 1, Label: "&On Point / Curve / Plane", Command: CommandOnEntity, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "E&qual Length / Radius", Command: CommandEqual, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Length / Arc Ra&tio", Command: CommandRatio, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Length / Arc Diff&erence", Command: CommandDifference, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "At &Midpoint", Command: CommandAtMidpoint, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "S&ymmetric", Command: CommandSymmetric, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Para&llel / Tangent", Command: CommandParallel, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "&Perpendicular", Command: CommandPerpendicular, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Same Orient&ation", Command: CommandOrientedSame, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Label: "Lock Point Where &Dragged", Command: CommandWhereDragged, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Comment", Command: CommandComment, Kind: MenuKindNone, Handler: (*UI).MenuConstrain},

	// Analyze menu
	{Level: 0, Label: "&Analyze", Command: CommandNone, Kind: MenuKindNone},
	{Level: 1, Label: "Measure &Volume", Command: CommandVolume, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Label: "Measure A&rea", Command: CommandArea, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Label: "Measure &Perimeter", Command: CommandPerimeter, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Label: "Show &Interfering Parts", Command: CommandInterference, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Label: "Show &Naked Edges", Command: CommandNakedEdges, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Label: "Show &Center of Mass", Command: CommandCenterOfMass, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Separator: true},
	{Level: 1, Label: "Show &Underconstrained Points", Command: CommandShowDegreesOfFreedom, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Separator: true},
	{Level: 1, Label: "&Trace Point", Command: CommandTracePoint, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Label: "&Stop Tracing...", Command: CommandStopTracing, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},
	{Level: 1, Label: "Step &Dimension...", Command: CommandStepDimension, Kind: MenuKindNone, Handler: (*UI).MenuAnalyze},

	// Help menu
	{Level: 0, Label: "&Help", Command: CommandNone, Kind: MenuKindNone},
	{Level: 1, Label: "&Language", Command: CommandLocale, Kind: MenuKindNone, ChildBuilder: (*UI).buildLocaleMenu},
	{Level: 1, Label: "&Website / Manual", Command: CommandWebsite, Kind: MenuKindNone, Handler: (*UI).MenuHelp},
	{Level: 1, Label: "&Go to GitHub commit", Command: CommandGitHub, Kind: MenuKindNone, Handler: (*UI).MenuHelp},
	{Level: 1, Label: "&About", Command: CommandAbout, Kind: MenuKindNone, Handler: (*UI).MenuHelp},
}

func buildMenuTree(entries []MenuEntry) []*MenuItem {
	var roots []*MenuItem
	var stack []*MenuItem
	var current *MenuItem

	for _, entry := range entries {
		// Handle separators
		if entry.Separator {
			if current != nil {
				current.Children = append(current.Children, &MenuItem{Separator: true})
			}
			continue
		}

		// Ensure the current submenu matches the desired depth.
		for entry.Level > 0 && entry.Level <= len(stack) {
			current = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		}

		item := &MenuItem{
			Label:        entry.Label,
			Command:      entry.Command,
			Kind:         entry.Kind,
			Handler:      entry.Handler,
			ChildBuilder: entry.ChildBuilder,
		}

		if entry.Level == 0 {
			roots = append(roots, item)
			current = item
			stack = stack[:0]
			continue
		}

		if current == nil {
			continue
		}

		current.Children = append(current.Children, item)

		if entry.ChildBuilder != nil || entry.Handler == nil {
			stack = append(stack, current)
			current = item
		}
	}

	return roots
}

func stripMnemonics(label string) string {
	return strings.ReplaceAll(label, "&", "")
}

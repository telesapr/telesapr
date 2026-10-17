package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Command is a menu or toolbar command.
type Command uint32

const (
	cmdNone Command = 0
	// File
	cmdNew Command = 100 + iota - 1
	cmdOpen
	cmdOpenRecent
	cmdSave
	cmdSaveAs
	cmdExportImage
	cmdExportMesh
	cmdExportSurfaces
	cmdExportView
	cmdExportSection
	cmdExportWireframe
	cmdImport
	cmdExit
	// View
	cmdZoomIn
	cmdZoomOut
	cmdZoomToFit
	cmdShowGrid
	cmdDimSolidModel
	cmdPerspectiveProj
	cmdExplodeSketch
	cmdOntoWorkplane
	cmdNearestOrtho
	cmdNearestIso
	cmdCenterView
	cmdShowToolbar
	cmdShowTextWnd
	cmdUnitsInches
	cmdUnitsFeetInches
	cmdUnitsMm
	cmdUnitsMeters
	cmdFullScreen
	// Edit
	cmdUndo
	cmdRedo
	cmdCut
	cmdCopy
	cmdPaste
	cmdPasteTransform
	cmdDelete
	cmdSelectChain
	cmdSelectAll
	cmdSnapToGrid
	cmdRotate90
	cmdUnselectAll
	cmdRegenAll
	cmdEditLineStyles
	cmdViewProjection
	cmdConfiguration
	// Request
	cmdSelWorkplane
	cmdFreeIn3D
	cmdDatumPoint
	cmdWorkplane
	cmdLineSegment
	cmdConstrSegment
	cmdCircle
	cmdArc
	cmdRectangle
	cmdCubic
	cmdTTFText
	cmdImage
	cmdSplitCurves
	cmdTangentArc
	cmdConstruction
	// Group
	cmdGroup3D
	cmdGroupWrkpl
	cmdGroupExtrude
	cmdGroupHelix
	cmdGroupLathe
	cmdGroupRevolve
	cmdGroupRot
	cmdGroupTrans
	cmdGroupLink
	cmdGroupRecent
	// Constrain
	cmdDistanceDia
	cmdRefDistance
	cmdMinDistance
	cmdMaxDistance
	cmdAngle
	cmdRefAngle
	cmdOtherAngle
	cmdReference
	cmdEqual
	cmdRatio
	cmdDifference
	cmdOnEntity
	cmdOnSegment
	cmdSymmetric
	cmdAtMidpoint
	cmdHorizontal
	cmdVertical
	cmdParallel
	cmdPerpendicular
	cmdOrientedSame
	cmdWhereDragged
	cmdComment
	// Analyze
	cmdVolume
	cmdArea
	cmdPerimeter
	cmdInterference
	cmdNakedEdges
	cmdShowDOF
	cmdCenterOfMass
	cmdTracePt
	cmdStopTracing
	cmdStepDim
	// Help
	cmdLocale
	cmdWebsite
	cmdGithub
	cmdAbout
	// Script
	cmdScriptConsole
	cmdScriptRun
	cmdScriptStop
	// Commands of this program's own, after the original ones so that
	// their numbers stay.
	cmdCheckSolid
	cmdRayCheck
	cmdGroupThread
	cmdParameters
	cmdFindCommand
	cmdMeasure
	cmdMoveGroupEarlier
	cmdMoveGroupLater
	cmdBrokenReferences
	cmdSectionView
	cmdGroupMirror
	cmdGroupChamfer
	cmdGroupSweep
	cmdGroupFillet
	cmdGroupLoft
	cmdGroupShell
	cmdScriptAgent
	cmdGroupPathPattern
	cmdBillOfMaterials
	cmdMotionStudy
	cmdMateConcentric
	cmdMateFlush
	cmdMateHinge
	cmdDrawingSheet
	cmdExportDrawing
	cmdMoveGroupToEnd
	cmdShowDrawingSheet
	cmdDraftSelectedSides
	cmdLoftGuides
	cmdShowWorkplanes
	cmdShowNormals
	cmdShowPoints
	cmdShowConstruction
	cmdShowReferences
	cmdShowComments
	cmdShowConstraintsAll
	cmdShowConstraintsDim
	cmdShowConstraintsNone
	cmdShowShaded
	cmdShowFaces
	cmdShowEdges
	cmdShowOutlines
	cmdShowMesh
	cmdShowOccludedInvisible
	cmdShowOccludedStippled
	cmdShowOccludedVisible
	cmdGroupSketchOnFace
	cmdGroupDependencies
)

// Pending is an operation to be completed by doing something with the
// mouse.
type Pending uint32

const (
	pendingNone Pending = iota
	pendingCommand
	pendingDraggingPoints
	pendingDraggingNewPoint
	pendingDraggingNewLinePoint
	pendingDraggingNewCubicPoint
	pendingDraggingNewArcPoint
	pendingDraggingConstraint
	pendingDraggingRadius
	pendingDraggingNormal
	pendingDraggingNewRadius
	pendingDraggingMarquee
)

// showConstraintMode says which constraints are drawn.
type showConstraintMode uint32

const (
	sCMNoShow showConstraintMode = iota
	sCMShowAll
	sCMShowDim
)

// DrawOccludedAs says how lines hidden by the solid are drawn.
type DrawOccludedAs uint32

const (
	occludedInvisible DrawOccludedAs = iota
	occludedStippled
	occludedVisible
)

// Selection is a selected or hovered entity or constraint.
type Selection struct {
	Tag        int
	Entity     core.HEntity
	Constraint core.HConstraint
	// Edge is an edge of the solid, by name, when its A is not zero.
	Edge topo.Name
	// Face is a face of the solid that no entity names, as a fillet's
	// or a Boolean tool's, by its identity (roadmap 14.1).
	Face       srf.FaceID
	Emphasized bool
}

// Equals compares what is selected.
func (s *Selection) Equals(b *Selection) bool {
	return s.Entity == b.Entity && s.Constraint == b.Constraint && s.Edge == b.Edge && s.Face == b.Face
}

// IsEmpty reports whether nothing is selected.
func (s *Selection) IsEmpty() bool {
	return s.Entity.V == 0 && s.Constraint.V == 0 && s.Edge.A == 0 && s.Face == 0
}

// Clear selects nothing.
func (s *Selection) Clear() {
	s.Entity, s.Constraint = core.HEntity{}, core.HConstraint{}
	s.Edge = topo.Name{}
	s.Emphasized = false
}

// Hover is a hovered item and how near the cursor it is.
type Hover struct {
	ZIndex    int
	Distance  float64
	Depth     float64
	Selection Selection
}

// groupedSelection is the selection sorted by kind.
type groupedSelection struct {
	point      []core.HEntity
	entity     []core.HEntity
	anyNormal  []core.HEntity
	vector     []core.HEntity
	face       []core.HEntity
	constraint []core.HConstraint
	// solidEdge lists the edges of the solid selected, and solidFace
	// its faces that no entity names, which n does not count.
	solidEdge []topo.Name
	solidFace []srf.FaceID

	points           int
	entities         int
	workplanes       int
	faces            int
	lineSegments     int
	circlesOrArcs    int
	arcs             int
	cubics           int
	periodicCubics   int
	anyNormals       int
	vectors          int
	constraints      int
	stylables        int
	constraintLabels int
	withEndpoints    int
	n                int
}

// pendingState is the operation in progress: the command chosen, or the
// drag.
type pendingState struct {
	operation Pending
	command   Command

	request    core.HRequest
	point      core.HEntity
	points     []core.HEntity
	requests   []core.HRequest
	circle     core.HEntity
	normal     core.HEntity
	constraint core.HConstraint

	description string
	filename    string

	hasSuggestion bool
	suggestion    sketch.Suggestion
}

// graphicsWindow is the window with the model and the toolbar.
type graphicsWindow struct {
	// What the window draws beside the sketch: the point being traced
	// and its path, the naked edges Analyze found, the line a drag
	// with Ctrl shows, where an export's origin was, and the centre of
	// mass.
	traced struct {
		Path  srf.SContour
		Point core.HEntity
	}
	nakedEdges srf.SEdgeList
	extraLine  struct {
		Draw     bool
		PtA, PtB geom.Vector
	}
	justExported struct {
		Draw, ShowOrigin bool
		Pt, U, V         geom.Vector
	}
	centerOfMass struct {
		Draw, Dirty bool
		Position    geom.Vector
	}

	// hoverTris is the grid hiddenBySolid looks triangles up in.
	hoverTris *hoverGrid
	ss        *Application

	window platform.Window

	openRecentMenu platform.Menu
	linkRecentMenu platform.Menu

	showGridMenuItem      platform.MenuItem
	dimSolidModelMenuItem platform.MenuItem
	measureMenuItem       platform.MenuItem
	sectionMenuItem       platform.MenuItem
	sectionMenuOn         bool
	// sheetShown shows the drawing sheet in place of the model (roadmap
	// 29.1); sheet is the sheet last made for it.
	sheetShown    bool
	sheet         sheetCache
	sheetMenuItem platform.MenuItem
	// sectionCap is the section view's cap, as last computed.
	sectionCap              sectionCap
	perspectiveProjMenuItem platform.MenuItem
	explodeMenuItem         platform.MenuItem
	showToolbarMenuItem     platform.MenuItem
	showTextWndMenuItem     platform.MenuItem
	scriptConsoleMenuItem   platform.MenuItem
	scriptAgentMenuItem     platform.MenuItem
	showMenuItems           map[Command]platform.MenuItem // View > Show
	enablementItems         map[Command]platform.MenuItem // entries greyed when they can't run
	enabledState            map[Command]bool              // as the platform last showed them
	fullScreenMenuItem      platform.MenuItem

	unitsMmMenuItem         platform.MenuItem
	unitsMetersMenuItem     platform.MenuItem
	unitsInchesMenuItem     platform.MenuItem
	unitsFeetInchesMenuItem platform.MenuItem

	inWorkplaneMenuItem platform.MenuItem
	in3dMenuItem        platform.MenuItem

	undoMenuItem platform.MenuItem
	redoMenuItem platform.MenuItem

	canvas          render.ViewportCanvas
	persistentDirty bool

	// The map from 2d screen coordinates to the coordinates of the 3d
	// sketch points: offset, projRight, projUp and scale. The sketch
	// reads it too, as View.
	view sketch.View

	orig struct {
		mouseDown         bool
		offset            geom.Vector
		projRight         geom.Vector
		projUp            geom.Vector
		mouse             geom.Point2d
		mouseOnButtonDown geom.Point2d
		marqueePoint      geom.Vector
		startedMoving     bool
	}
	// The projection the entities' screen bounding boxes were made for.
	cached struct {
		offset    geom.Vector
		projRight geom.Vector
		projUp    geom.Vector
		scale     float64
	}

	// Most recent mouse position, updated every time the mouse moves.
	currentMousePosition geom.Point2d

	// When the user is dragging a point, don't solve multiple times
	// without allowing a paint in between.
	havePainted bool

	context struct{ active bool }

	animateTimer platform.Timer
	// animating is set while AnimateOnto turns the view, which ends at
	// animateEnd.
	animating  bool
	animateEnd func()

	pending pendingState

	// The constraint being edited with the on-screen textbox.
	constraintBeingEdited core.HConstraint

	hoverList                   []Hover
	hover                       Selection
	hoverWasSelectedOnMousedown bool
	selection                   []Selection
	gs                          groupedSelection

	toolbarHovered Command

	// This sets what gets displayed.
	show sketch.Visibility

	showConstraints     showConstraintMode
	showTextWindow      bool
	showShaded          bool
	showFaces           bool
	showFacesDrawing    bool
	showFacesNonDrawing bool
	showMesh            bool

	drawOccludedAs DrawOccludedAs

	showSnapGrid  bool
	dimSolidModel bool
	// measuring is the measure mode (MenuMeasure), an extension.
	measuring bool

	// Per-entity drawing caches, for the sketch generation they were
	// made in.
	entityCaches map[core.HEntity]*entityDrawCache
	cacheGen     uint64
	cacheSketch  *sketch.Sketch

	// The lighting of the frame being painted, and the persistent part of
	// the last frame.
	lighting   render.Lighting
	persistent persistentLayer
}

// maxSelectableFaces limits the faces selected at once.
const maxSelectableFaces = 3

func newGraphicsWindow(ss *Application) *graphicsWindow {
	gw := &graphicsWindow{ss: ss}
	gw.view = sketch.DefaultView()
	// What graphicsWindow::Init sets; the sketch reads the show flags
	// even before the window is made.
	gw.show = sketch.DefaultVisibility()
	return gw
}

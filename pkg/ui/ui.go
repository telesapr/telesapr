package ui

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/render"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// Command is a menu or toolbar command (C++ Command, ui.h).
type Command uint32

const (
	CmdNone Command = 0
	// File
	CmdNew Command = 100 + iota - 1
	CmdOpen
	CmdOpenRecent
	CmdSave
	CmdSaveAs
	CmdExportImage
	CmdExportMesh
	CmdExportSurfaces
	CmdExportView
	CmdExportSection
	CmdExportWireframe
	CmdImport
	CmdExit
	// View
	CmdZoomIn
	CmdZoomOut
	CmdZoomToFit
	CmdShowGrid
	CmdDimSolidModel
	CmdPerspectiveProj
	CmdExplodeSketch
	CmdOntoWorkplane
	CmdNearestOrtho
	CmdNearestIso
	CmdCenterView
	CmdShowToolbar
	CmdShowTextWnd
	CmdUnitsInches
	CmdUnitsFeetInches
	CmdUnitsMm
	CmdUnitsMeters
	CmdFullScreen
	// Edit
	CmdUndo
	CmdRedo
	CmdCut
	CmdCopy
	CmdPaste
	CmdPasteTransform
	CmdDelete
	CmdSelectChain
	CmdSelectAll
	CmdSnapToGrid
	CmdRotate90
	CmdUnselectAll
	CmdRegenAll
	CmdEditLineStyles
	CmdViewProjection
	CmdConfiguration
	// Request
	CmdSelWorkplane
	CmdFreeIn3D
	CmdDatumPoint
	CmdWorkplane
	CmdLineSegment
	CmdConstrSegment
	CmdCircle
	CmdArc
	CmdRectangle
	CmdCubic
	CmdTTFText
	CmdImage
	CmdSplitCurves
	CmdTangentArc
	CmdConstruction
	// Group
	CmdGroup3D
	CmdGroupWrkpl
	CmdGroupExtrude
	CmdGroupHelix
	CmdGroupLathe
	CmdGroupRevolve
	CmdGroupRot
	CmdGroupTrans
	CmdGroupLink
	CmdGroupRecent
	// Constrain
	CmdDistanceDia
	CmdRefDistance
	CmdAngle
	CmdRefAngle
	CmdOtherAngle
	CmdReference
	CmdEqual
	CmdRatio
	CmdDifference
	CmdOnEntity
	CmdSymmetric
	CmdAtMidpoint
	CmdHorizontal
	CmdVertical
	CmdParallel
	CmdPerpendicular
	CmdOrientedSame
	CmdWhereDragged
	CmdComment
	// Analyze
	CmdVolume
	CmdArea
	CmdPerimeter
	CmdInterference
	CmdNakedEdges
	CmdShowDOF
	CmdCenterOfMass
	CmdTracePt
	CmdStopTracing
	CmdStepDim
	// Help
	CmdLocale
	CmdWebsite
	CmdGithub
	CmdAbout
	// Script, which C++ doesn't have
	CmdScriptConsole
	CmdScriptRun
	CmdScriptStop
)

// Pending is an operation to be completed by doing something with the
// mouse (C++ GraphicsWindow::Pending).
type Pending uint32

const (
	PendingNone Pending = iota
	PendingCommand
	PendingDraggingPoints
	PendingDraggingNewPoint
	PendingDraggingNewLinePoint
	PendingDraggingNewCubicPoint
	PendingDraggingNewArcPoint
	PendingDraggingConstraint
	PendingDraggingRadius
	PendingDraggingNormal
	PendingDraggingNewRadius
	PendingDraggingMarquee
)

// ShowConstraintMode says which constraints are drawn.
type ShowConstraintMode uint32

const (
	SCMNoShow ShowConstraintMode = iota
	SCMShowAll
	SCMShowDim
)

// DrawOccludedAs says how lines hidden by the solid are drawn.
type DrawOccludedAs uint32

const (
	OccludedInvisible DrawOccludedAs = iota
	OccludedStippled
	OccludedVisible
)

// Selection is a selected or hovered entity or constraint (C++
// GraphicsWindow::Selection).
type Selection struct {
	Tag        int
	Entity     core.HEntity
	Constraint core.HConstraint
	Emphasized bool
}

// Equals compares what is selected.
func (s *Selection) Equals(b *Selection) bool {
	return s.Entity == b.Entity && s.Constraint == b.Constraint
}

// IsEmpty reports whether nothing is selected.
func (s *Selection) IsEmpty() bool {
	return s.Entity.V == 0 && s.Constraint.V == 0
}

// Clear selects nothing.
func (s *Selection) Clear() {
	s.Entity, s.Constraint = core.HEntity{}, core.HConstraint{}
	s.Emphasized = false
}

// Hover is a hovered item and how near the cursor it is (C++
// GraphicsWindow::Hover).
type Hover struct {
	ZIndex    int
	Distance  float64
	Depth     float64
	Selection Selection
}

// groupedSelection is the selection sorted by kind (C++
// GraphicsWindow::gs, filled by GroupSelection).
type groupedSelection struct {
	point      []core.HEntity
	entity     []core.HEntity
	anyNormal  []core.HEntity
	vector     []core.HEntity
	face       []core.HEntity
	constraint []core.HConstraint

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

// pendingState is C++ GraphicsWindow::pending.
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
	suggestion    core.ConstraintType
}

// GraphicsWindow is the window with the model and the toolbar (C++
// GraphicsWindow).
type GraphicsWindow struct {
	ss *SolveSpaceUI

	window platform.Window

	openRecentMenu platform.Menu
	linkRecentMenu platform.Menu

	showGridMenuItem        platform.MenuItem
	dimSolidModelMenuItem   platform.MenuItem
	perspectiveProjMenuItem platform.MenuItem
	explodeMenuItem         platform.MenuItem
	showToolbarMenuItem     platform.MenuItem
	showTextWndMenuItem     platform.MenuItem
	scriptConsoleMenuItem   platform.MenuItem
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
	// reads it too (SS.GW in C++), as View.
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

	showConstraints     ShowConstraintMode
	showTextWindow      bool
	showShaded          bool
	showFaces           bool
	showFacesDrawing    bool
	showFacesNonDrawing bool
	showMesh            bool

	drawOccludedAs DrawOccludedAs

	showSnapGrid  bool
	dimSolidModel bool

	// Per-entity drawing caches, for the sketch generation they were
	// made in (C++ Entity::beziers, edges and screenBBox).
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

func newGraphicsWindow(ss *SolveSpaceUI) *GraphicsWindow {
	gw := &GraphicsWindow{ss: ss}
	gw.view = sketch.DefaultView()
	// What GraphicsWindow::Init sets; the sketch reads the show flags
	// even before the window is made.
	gw.show = sketch.DefaultVisibility()
	return gw
}

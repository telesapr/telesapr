package app

// Command identifies a UI action exposed through menus and shortcut
// registrations. The numeric values mirror the historic C++ enumerations
// so that translated resources and persisted preferences can be migrated
// progressively.
type Command int

const (
	CommandNone Command = 0
)

const (
	// File commands.
	CommandNew Command = 100 + iota
	CommandOpen
	CommandOpenRecent
	CommandSave
	CommandSaveAs
	CommandExportImage
	CommandExportMesh
	CommandExportSurfaces
	CommandExportView
	CommandExportSection
	CommandExportWireframe
	CommandImport
	CommandExit

	// View commands.
	CommandZoomIn
	CommandZoomOut
	CommandZoomToFit
	CommandShowGrid
	CommandDimSolidModel
	CommandPerspectiveProj
	CommandExplodeSketch
	CommandOntoWorkplane
	CommandNearestOrtho
	CommandNearestIso
	CommandCenterView
	CommandShowToolbar
	CommandShowTextWindow
	CommandUnitsInches
	CommandUnitsFeetInches
	CommandUnitsMillimeters
	CommandUnitsMeters
	CommandFullScreen

	// Edit commands.
	CommandUndo
	CommandRedo
	CommandCut
	CommandCopy
	CommandPaste
	CommandPasteTransform
	CommandDelete
	CommandSelectChain
	CommandSelectAll
	CommandSnapToGrid
	CommandRotate90
	CommandUnselectAll
	CommandRegenerateAll
	CommandEditLineStyles
	CommandViewProjection
	CommandConfiguration

	// Request (sketch) commands.
	CommandSelectWorkplane
	CommandFreeIn3D
	CommandDatumPoint
	CommandWorkplane
	CommandLineSegment
	CommandConstructionSegment
	CommandCircle
	CommandArc
	CommandRectangle
	CommandCubic
	CommandTrueTypeText
	CommandImage
	CommandSplitCurves
	CommandTangentArc
	CommandToggleConstruction

	// Group commands.
	CommandGroup3D
	CommandGroupWorkplane
	CommandGroupExtrude
	CommandGroupHelix
	CommandGroupLathe
	CommandGroupRevolve
	CommandGroupRotate
	CommandGroupTranslate
	CommandGroupLink
	CommandGroupRecent

	// Constraint commands.
	CommandDistanceDiameter
	CommandReferenceDistance
	CommandAngle
	CommandReferenceAngle
	CommandOtherAngle
	CommandToggleReference
	CommandEqual
	CommandRatio
	CommandDifference
	CommandOnEntity
	CommandSymmetric
	CommandAtMidpoint
	CommandHorizontal
	CommandVertical
	CommandParallel
	CommandPerpendicular
	CommandOrientedSame
	CommandWhereDragged
	CommandComment

	// Analysis commands.
	CommandVolume
	CommandArea
	CommandPerimeter
	CommandInterference
	CommandNakedEdges
	CommandShowDegreesOfFreedom
	CommandCenterOfMass
	CommandTracePoint
	CommandStopTracing
	CommandStepDimension

	// Help commands.
	CommandLocale
	CommandWebsite
	CommandGitHub
	CommandAbout
)

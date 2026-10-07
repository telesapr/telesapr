// Package ui is SolveSpace's user interface: the graphics window with
// its drawing, mouse tools and menus, the text window with its screens,
// and the application shell. It began as a port of SolveSpace's ui.h
// and GUI sources, over pkg/platform and pkg/render.
package ui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// ModelColors is how many model colours the settings keep.
const ModelColors = 8

// MaxRecent is how many recent files are remembered.
const MaxRecent = 8

// Application is the application: the sketch, its two windows and the
// user's configuration (C++ SolveSpaceUI, the SS global).
type Application struct {
	Platform platform.Platform
	Settings core.Settings

	SK *sketch.Sketch
	GW *GraphicsWindow
	TW *TextWindow

	// Little bits of extra configuration state.
	ModelColor               [ModelColors]core.RGBA
	LightDir                 [2]geom.Vector
	LightIntensity           [2]float64
	AmbientIntensity         float64
	ChordTol                 float64
	MaxSegments              int
	ExportChordTol           float64
	ExportMaxSegments        int
	TimeoutRedundantConstr   int // milliseconds
	AnimationSpeed           int // milliseconds
	CameraTangentSetting     float64
	GridSpacing              float64
	ExportScale              float64
	ExportOffset             float64
	ArcDimDefaultDiameter    bool
	ShowFullFilePath         bool
	FixExportColors          bool
	ExportBackgroundColor    bool
	DrawBackFaces            bool
	ShowContourAreas         bool
	CheckClosedContour       bool
	CameraNav                bool
	TurntableNav             bool
	ImmediatelyEditDimension bool
	AutomaticLineConstraints bool
	// BackgroundShells has the regenerations the interface schedules
	// make the solids off its thread (shelljob.go); the application
	// sets it, the tests don't.
	BackgroundShells bool
	shellJobs        shellJobs
	// lastDimensions are what the drawing's dimensions measured when
	// it was last made (drawingdim.go).
	lastDimensions []DimensionResult
	// ColorByFreedom colours the active group's entities by how much
	// of them their constraints fix (an extension).
	ColorByFreedom bool
	// Shaded lights the solid by the faces' appearances, through a
	// shader with the GL renderer (roadmap 19.1).
	Shaded bool
	// UseOpenGL draws the windows with OpenGL where the platform has
	// it (an extension; C++ chose at build time). RendererOverride,
	// "opengl" or "software", is the command line's choice for this
	// run, and wins over the setting until the setting is changed.
	UseOpenGL             bool
	RendererOverride      string
	ShowToolbar           bool
	ScreenshotFile        string
	BackgroundColor       core.RGBA
	ExportShadedTriangles bool
	ExportPwlCurves       bool
	ExportCanvasSizeAuto  bool
	ExportMode            bool
	// StepOneSurfacePerFace has STEP export write a face of several
	// patches as one surface; the tests turn it off to compare.
	StepOneSurfacePerFace bool
	// Windowless makes the graphics window behave as C++'s headless
	// build, which has none: the view is never fitted to it and the
	// camera is 297 by 210. The command-line tool sets it.
	Windowless   bool
	ExportMargin struct{ Left, Right, Bottom, Top float64 }
	ExportCanvas struct{ Width, Height, Dx, Dy float64 }
	GCode        struct {
		Depth      float64
		SafeHeight float64
		Passes     int
		Feed       float64
		PlungeFeed float64
	}

	// ViewUnits is the unit lengths are shown and typed in: the
	// document's (Sketch.Units), which it follows. DefaultUnits is the
	// unit of a new document, the last one the user chose.
	ViewUnits          core.Unit
	DefaultUnits       core.Unit
	AfterDecimalMm     int
	AfterDecimalInch   int
	AfterDecimalDegree int
	UseSIPrefixes      bool
	AutosaveInterval   int // minutes
	Explode            bool
	ExplodeDistance    float64
	UsePerspectiveProj bool

	// The tangent arcs made non-parametrically.
	TangentArcRadius float64
	TangentArcManual bool
	TangentArcModify bool

	SaveFile    string
	RecentFiles []string

	// Additional display stuff.
	Traced struct {
		Path  srf.SContour
		Point core.HEntity
	}
	NakedEdges srf.SEdgeList
	ExtraLine  struct {
		Draw     bool
		PtA, PtB geom.Vector
	}
	JustExportedInfo struct {
		Draw, ShowOrigin bool
		Pt, U, V         geom.Vector
	}
	CenterOfMass struct {
		Draw, Dirty bool
		Position    geom.Vector
	}

	// AllConsistent is false while a file is loading, so nothing draws a
	// sketch with dangling references.
	AllConsistent bool

	scheduledGenerateAll bool
	scheduledShowTW      bool
	refreshTimer         platform.Timer

	// scriptHost is what scripts work on the sketch through, and
	// messageSink, while a script runs, takes what would be shown in
	// a message dialog.
	scriptHost  *ScriptHost
	messageSink func(text string, isError bool)
	// console is the window scripts are typed in, see Console.
	console *ScriptConsole
	// AgentAllowed is the setting of Script > Allow AI Connections: a
	// program that draws by writing scripts, such as an AI model's, may
	// connect (see AllowAgent). AgentConfirm has the user asked before
	// each script of such a program runs. AgentSocketDir is where the
	// socket is made, the user's own directory for them if "".
	AgentAllowed   bool
	AgentConfirm   bool
	AgentSocketDir string
	// agent is what listens for such programs, nil when none may
	// connect.
	agent         *agentServer
	autosaveTimer platform.Timer

	// bitmapFont is the text font the canvases share.
	bitmapFont *res.BitmapFont
	// fonts are the platform's fonts, which text entities draw with.
	fonts *ttf.FontList

	// pixmaps are the image requests' pictures, converted for drawing.
	pixmaps map[*sketch.Image]*res.Pixmap

	// lastGeneration is the sketch generation the display last saw, to
	// notice a regeneration (C++ GenerateAll's persistentDirty and
	// centerOfMass.dirty).
	lastGeneration uint64
}

// New makes the application on a platform; Init loads the settings and
// opens the windows.
func New(p platform.Platform) *Application {
	ss := &Application{Platform: p, Settings: p.Settings(), StepOneSurfacePerFace: true}
	ss.GW = newGraphicsWindow(ss)
	ss.TW = newTextWindow(ss)
	return ss
}

// LoadSettings reads the configuration (the first part of C++
// SolveSpaceUI::Init).
func (ss *Application) LoadSettings() {
	s := ss.Settings
	ss.TangentArcRadius = 10.0
	ss.ExplodeDistance = 1.0

	defaults := [ModelColors]core.RGBA{
		rgbi(150, 150, 150), rgbi(100, 100, 100), rgbi(30, 30, 30), rgbi(150, 0, 0),
		rgbi(0, 100, 0), rgbi(0, 80, 80), rgbi(0, 0, 130), rgbi(80, 0, 80),
	}
	for i := range ss.ModelColor {
		ss.ModelColor[i] = core.ThawColor(s, "ModelColor_"+strconv.Itoa(i), defaults[i])
	}
	ss.LightIntensity[0] = s.ThawFloat("LightIntensity_0", 1.0)
	ss.LightIntensity[1] = s.ThawFloat("LightIntensity_1", 0.5)
	ss.AmbientIntensity = s.ThawFloat("Light_Ambient", 0.3)
	ss.LightDir[0].X = s.ThawFloat("LightDir_0_Right", -1.0)
	ss.LightDir[0].Y = s.ThawFloat("LightDir_0_Up", 1.0)
	ss.LightDir[0].Z = s.ThawFloat("LightDir_0_Forward", 0.0)
	ss.LightDir[1].X = s.ThawFloat("LightDir_1_Right", 1.0)
	ss.LightDir[1].Y = s.ThawFloat("LightDir_1_Up", 0.0)
	ss.LightDir[1].Z = s.ThawFloat("LightDir_1_Forward", 0.0)

	ss.ExportMode = false
	ss.ChordTol = s.ThawFloat("ChordTolerancePct", 0.1)
	ss.MaxSegments = int(s.ThawInt("MaxSegments", 20))
	ss.ExportChordTol = s.ThawFloat("ExportChordTolerance", 0.1)
	ss.ExportMaxSegments = int(s.ThawInt("ExportMaxSegments", 64))
	ss.TimeoutRedundantConstr = int(s.ThawInt("TimeoutRedundantConstraints", 1000))
	ss.AnimationSpeed = int(s.ThawInt("AnimationSpeed", 800))
	ss.DefaultUnits = core.Unit(s.ThawInt("ViewUnits", uint32(core.UnitMM)))
	ss.ViewUnits = ss.DefaultUnits
	ss.AfterDecimalMm = int(s.ThawInt("AfterDecimalMm", 2))
	ss.AfterDecimalInch = int(s.ThawInt("AfterDecimalInch", 3))
	ss.AfterDecimalDegree = int(s.ThawInt("AfterDecimalDegree", 2))
	ss.UseSIPrefixes = core.ThawBool(s, "UseSIPrefixes", false)
	ss.CameraTangentSetting = s.ThawFloat("CameraTangent", float64(float32(0.3))/1e3) // C++ 0.3f/1e3, in double
	ss.GridSpacing = s.ThawFloat("GridSpacing", 5.0)
	ss.ExportScale = s.ThawFloat("ExportScale", 1.0)
	ss.ExportOffset = s.ThawFloat("ExportOffset", 0.0)
	ss.ArcDimDefaultDiameter = core.ThawBool(s, "ArcDimDefaultDiameter", false)
	ss.ShowFullFilePath = core.ThawBool(s, "ShowFullFilePath", true)
	ss.FixExportColors = core.ThawBool(s, "FixExportColors", true)
	ss.ExportBackgroundColor = core.ThawBool(s, "ExportBackgroundColor", false)
	ss.DrawBackFaces = core.ThawBool(s, "DrawBackFaces", true)
	ss.CameraNav = core.ThawBool(s, "CameraNav", false)
	ss.TurntableNav = core.ThawBool(s, "TurntableNav", false)
	ss.ImmediatelyEditDimension = core.ThawBool(s, "ImmediatelyEditDimension", true)
	ss.CheckClosedContour = core.ThawBool(s, "CheckClosedContour", true)
	ss.AutomaticLineConstraints = core.ThawBool(s, "AutomaticLineConstraints", true)
	ss.ColorByFreedom = core.ThawBool(s, "ColorByFreedom", true)
	ss.Shaded = core.ThawBool(s, "Shaded", false)
	ss.UseOpenGL = core.ThawBool(s, "UseOpenGL", true)
	ss.ShowContourAreas = core.ThawBool(s, "ShowContourAreas", false)
	ss.ExportShadedTriangles = core.ThawBool(s, "ExportShadedTriangles", true)
	ss.ExportPwlCurves = core.ThawBool(s, "ExportPwlCurves", false)
	ss.BackgroundColor = core.ThawColor(s, "BackgroundColor", rgbi(0, 0, 0))
	ss.ExportCanvasSizeAuto = core.ThawBool(s, "ExportCanvasSizeAuto", true)
	ss.ExportMargin.Left = s.ThawFloat("ExportMargin_Left", 5.0)
	ss.ExportMargin.Right = s.ThawFloat("ExportMargin_Right", 5.0)
	ss.ExportMargin.Bottom = s.ThawFloat("ExportMargin_Bottom", 5.0)
	ss.ExportMargin.Top = s.ThawFloat("ExportMargin_Top", 5.0)
	ss.ExportCanvas.Width = s.ThawFloat("ExportCanvas_Width", 100.0)
	ss.ExportCanvas.Height = s.ThawFloat("ExportCanvas_Height", 100.0)
	ss.ExportCanvas.Dx = s.ThawFloat("ExportCanvas_Dx", 5.0)
	ss.ExportCanvas.Dy = s.ThawFloat("ExportCanvas_Dy", 5.0)
	ss.GCode.Depth = s.ThawFloat("GCode_Depth", 10.0)
	ss.GCode.SafeHeight = s.ThawFloat("GCode_SafeHeight", 5.0)
	ss.GCode.Passes = int(s.ThawInt("GCode_Passes", 1))
	ss.GCode.Feed = s.ThawFloat("GCode_Feed", 10.0)
	ss.GCode.PlungeFeed = s.ThawFloat("GCode_PlungeFeed", 10.0)
	ss.ShowToolbar = core.ThawBool(s, "ShowToolbar", true)
	ss.AgentAllowed = core.ThawBool(s, "AgentAllowed", false)
	ss.AgentConfirm = core.ThawBool(s, "AgentConfirm", false)
	ss.RecentFiles = nil
	for i := range MaxRecent {
		if raw := s.ThawString("RecentFile_"+strconv.Itoa(i), ""); raw != "" {
			ss.RecentFiles = append(ss.RecentFiles, raw)
		}
	}
	ss.AutosaveInterval = int(s.ThawInt("AutosaveInterval", 5))
	ss.applyEngineSettings()
}

// applyEngineSettings passes the settings the engine reads on.
func (ss *Application) applyEngineSettings() {
	if ss.SK != nil {
		ss.SK.Settings = ss.Settings
		// A linked solid keeps the tolerance it was read at: when the
		// chord tolerance in effect changes, the links are read again.
		was := ss.SK.ChordTolPercent()
		ss.SK.ChordTolPct = ss.ChordTol
		ss.SK.MaxSegments = ss.MaxSegments
		ss.SK.CheckClosedContour = ss.CheckClosedContour
		ss.SK.Exporting = ss.exportTolerance()
		if ss.SK.ChordTolPercent() != was && ss.SK.ReloadAllLinked != nil {
			ss.SK.ReloadAllLinked(ss.SK)
		}
	}
}

// exportTolerance is the tolerance an export follows curves at while
// ExportMode is on, and unset otherwise.
func (ss *Application) exportTolerance() srf.Tolerance {
	if !ss.ExportMode {
		return srf.Tolerance{}
	}
	return srf.Tol(ss.ExportChordTolMm(), ss.ExportMaxSegments)
}

// EnterExportMode has the regenerations that follow use the export
// tolerance, until leaveExportMode.
func (ss *Application) EnterExportMode() {
	ss.ExportMode = true
	ss.SK.Exporting = ss.exportTolerance()
}

// LeaveExportMode returns the regenerations to the display tolerance.
func (ss *Application) LeaveExportMode() {
	ss.ExportMode = false
	ss.SK.Exporting = srf.Tolerance{}
}

// SaveSettings writes the configuration (C++ SolveSpaceUI::Exit, less
// the exit).
func (ss *Application) SaveSettings() {
	s := ss.Settings
	if ss.GW.window != nil {
		ss.GW.window.FreezePosition(s, "GraphicsWindow")
	}
	if ss.TW.window != nil {
		ss.TW.window.FreezePosition(s, "TextWindow")
	}
	if ss.console != nil && ss.console.window != nil {
		ss.console.window.FreezePosition(s, "ScriptConsole")
	}
	for i := range MaxRecent {
		raw := ""
		if i < len(ss.RecentFiles) {
			raw = ss.RecentFiles[i]
		}
		s.FreezeString("RecentFile_"+strconv.Itoa(i), raw)
	}
	for i := range ss.ModelColor {
		core.FreezeColor(s, "ModelColor_"+strconv.Itoa(i), ss.ModelColor[i])
	}
	f32 := func(v float64) float64 { return float64(float32(v)) }
	s.FreezeFloat("LightIntensity_0", f32(ss.LightIntensity[0]))
	s.FreezeFloat("LightIntensity_1", f32(ss.LightIntensity[1]))
	s.FreezeFloat("Light_Ambient", f32(ss.AmbientIntensity))
	s.FreezeFloat("LightDir_0_Right", f32(ss.LightDir[0].X))
	s.FreezeFloat("LightDir_0_Up", f32(ss.LightDir[0].Y))
	s.FreezeFloat("LightDir_0_Forward", f32(ss.LightDir[0].Z))
	s.FreezeFloat("LightDir_1_Right", f32(ss.LightDir[1].X))
	s.FreezeFloat("LightDir_1_Up", f32(ss.LightDir[1].Y))
	s.FreezeFloat("LightDir_1_Forward", f32(ss.LightDir[1].Z))
	s.FreezeFloat("ChordTolerancePct", f32(ss.ChordTol))
	s.FreezeInt("MaxSegments", uint32(ss.MaxSegments))
	s.FreezeFloat("ExportChordTolerance", f32(ss.ExportChordTol))
	s.FreezeInt("ExportMaxSegments", uint32(ss.ExportMaxSegments))
	s.FreezeInt("TimeoutRedundantConstraints", uint32(ss.TimeoutRedundantConstr))
	s.FreezeInt("AnimationSpeed", uint32(ss.AnimationSpeed))
	s.FreezeInt("ViewUnits", uint32(ss.DefaultUnits))
	s.FreezeInt("AfterDecimalMm", uint32(ss.AfterDecimalMm))
	s.FreezeInt("AfterDecimalInch", uint32(ss.AfterDecimalInch))
	s.FreezeInt("AfterDecimalDegree", uint32(ss.AfterDecimalDegree))
	core.FreezeBool(s, "UseSIPrefixes", ss.UseSIPrefixes)
	s.FreezeFloat("CameraTangent", f32(ss.CameraTangentSetting))
	s.FreezeFloat("GridSpacing", ss.GridSpacing)
	s.FreezeFloat("ExportScale", ss.ExportScale)
	s.FreezeFloat("ExportOffset", ss.ExportOffset)
	core.FreezeBool(s, "ArcDimDefaultDiameter", ss.ArcDimDefaultDiameter)
	core.FreezeBool(s, "ShowFullFilePath", ss.ShowFullFilePath)
	core.FreezeBool(s, "FixExportColors", ss.FixExportColors)
	core.FreezeBool(s, "ExportBackgroundColor", ss.ExportBackgroundColor)
	core.FreezeBool(s, "DrawBackFaces", ss.DrawBackFaces)
	core.FreezeBool(s, "ShowContourAreas", ss.ShowContourAreas)
	core.FreezeBool(s, "CheckClosedContour", ss.CheckClosedContour)
	core.FreezeBool(s, "CameraNav", ss.CameraNav)
	core.FreezeBool(s, "TurntableNav", ss.TurntableNav)
	core.FreezeBool(s, "ImmediatelyEditDimension", ss.ImmediatelyEditDimension)
	core.FreezeBool(s, "AutomaticLineConstraints", ss.AutomaticLineConstraints)
	core.FreezeBool(s, "ColorByFreedom", ss.ColorByFreedom)
	core.FreezeBool(s, "Shaded", ss.Shaded)
	core.FreezeBool(s, "UseOpenGL", ss.UseOpenGL)
	core.FreezeBool(s, "ExportShadedTriangles", ss.ExportShadedTriangles)
	core.FreezeBool(s, "ExportPwlCurves", ss.ExportPwlCurves)
	core.FreezeColor(s, "BackgroundColor", ss.BackgroundColor)
	core.FreezeBool(s, "ExportCanvasSizeAuto", ss.ExportCanvasSizeAuto)
	s.FreezeFloat("ExportMargin_Left", ss.ExportMargin.Left)
	s.FreezeFloat("ExportMargin_Right", ss.ExportMargin.Right)
	s.FreezeFloat("ExportMargin_Bottom", ss.ExportMargin.Bottom)
	s.FreezeFloat("ExportMargin_Top", ss.ExportMargin.Top)
	s.FreezeFloat("ExportCanvas_Width", ss.ExportCanvas.Width)
	s.FreezeFloat("ExportCanvas_Height", ss.ExportCanvas.Height)
	s.FreezeFloat("ExportCanvas_Dx", ss.ExportCanvas.Dx)
	s.FreezeFloat("ExportCanvas_Dy", ss.ExportCanvas.Dy)
	s.FreezeFloat("GCode_Depth", ss.GCode.Depth)
	s.FreezeInt("GCode_Passes", uint32(ss.GCode.Passes))
	s.FreezeFloat("GCode_Feed", ss.GCode.Feed)
	s.FreezeFloat("GCode_PlungeFeed", ss.GCode.PlungeFeed)
	core.FreezeBool(s, "ShowToolbar", ss.ShowToolbar)
	core.FreezeBool(s, "AgentAllowed", ss.AgentAllowed)
	core.FreezeBool(s, "AgentConfirm", ss.AgentConfirm)
	s.FreezeInt("AutosaveInterval", uint32(ss.AutosaveInterval))

	// And the default styles, colours and line widths and such.
	if ss.SK != nil {
		ss.SK.FreezeDefaultStyles(s)
	}
}

func rgbi(r, g, b uint8) core.RGBA { return core.NewRGBA(r, g, b, 255) }

// Refresh regenerates what was scheduled, then shows the text window
// (C++ SolveSpaceUI::Refresh).
func (ss *Application) Refresh() {
	// GenerateAll must happen before updating displays.
	if ss.scheduledGenerateAll {
		ss.scheduledGenerateAll = false
		if ss.BackgroundShells {
			ss.SK.DeferShells = true
			ss.SK.Refresh()
			ss.SK.DeferShells = false
			if ss.SK.AnyShellsStale() {
				ss.startShellJob()
			}
		} else {
			ss.SK.Refresh()
		}
		ss.afterGenerate()
	}
	if ss.scheduledShowTW {
		ss.scheduledShowTW = false
		ss.TW.Show()
	}
}

// ScheduleGenerateAll regenerates the dirty groups once events are
// handled.
func (ss *Application) ScheduleGenerateAll() {
	ss.SK.ScheduleGenerateAll()
}

func (ss *Application) onScheduleGenerateAll() {
	ss.scheduledGenerateAll = true
	if ss.refreshTimer != nil {
		ss.refreshTimer.RunAfterProcessingEvents()
	}
}

// ScheduleShowTW shows the text window again once events are handled.
func (ss *Application) ScheduleShowTW() {
	ss.scheduledShowTW = true
	if ss.refreshTimer != nil {
		ss.refreshTimer.RunAfterProcessingEvents()
	}
}

// ScheduleAutosave arms the autosave timer.
func (ss *Application) ScheduleAutosave() {
	if ss.autosaveTimer != nil {
		ss.autosaveTimer.RunAfter(ss.AutosaveInterval * 60 * 1000)
	}
}

// GenerateAll regenerates the sketch, then updates the display state
// that depends on it (C++ SolveSpaceUI::GenerateAll).
func (ss *Application) GenerateAll(t sketch.GenerateType) {
	ss.applyEngineSettings()
	ss.SK.GenerateAllTyped(t)
	ss.afterGenerate()
}

// GenerateAllFindFree is GenerateAll with andFindFree, marking the free
// parameters.
func (ss *Application) GenerateAllFindFree(t sketch.GenerateType) {
	ss.applyEngineSettings()
	ss.SK.GenerateAllFindFree(t)
	ss.afterGenerate()
}

// afterGenerate is the end of C++ GenerateAll.
func (ss *Application) afterGenerate() {
	ss.AllConsistent = true
	ss.GW.persistentDirty = true
	ss.CenterOfMass.Dirty = true
	ss.lastGeneration = ss.SK.Generation
	ss.GW.Invalidate(false)
}

// ActiveGroupsOkay reports whether every group up to the active one
// solved (C++ SolveSpaceUI::ActiveGroupsOkay).
func (ss *Application) ActiveGroupsOkay() bool {
	for _, hg := range ss.SK.GroupOrder {
		g := ss.SK.Group(hg)
		if !g.IsSolvedOkay() {
			return false
		}
		if g.H == ss.GW.activeGroupH() {
			break
		}
	}
	return true
}

// MmPerUnit is the size of the view unit in mm.
func (ss *Application) MmPerUnit() float64 {
	switch ss.ViewUnits {
	case core.UnitInches, core.UnitFeetInches:
		// For feet and inches, the 'unit' is still inches.
		return 25.4
	case core.UnitMeters:
		return 1000.0
	}
	return 1.0
}

// UnitName is the view unit's abbreviation.
func (ss *Application) UnitName() string {
	switch ss.ViewUnits {
	case core.UnitInches, core.UnitFeetInches:
		return "in"
	case core.UnitMeters:
		return "m"
	case core.UnitMM:
		return "mm"
	}
	return ""
}

// MmToString formats a length in the view units; editable gives plain
// inches instead of feet and inches, which can't be parsed back (C++
// SolveSpaceUI::MmToString).
func (ss *Application) MmToString(v float64, editable bool) string {
	v /= ss.MmPerUnit()
	if ss.ViewUnits == core.UnitFeetInches && !editable {
		// Now convert v from inches to 64'ths of an inch, to make
		// rounding easier.
		v = math.Floor((v + (1.0 / 128.0)) * 64.0)
		feet := int(v / (12.0 * 64.0))
		v -= float64(feet) * 12.0 * 64.0
		// v is now the feet-less remainder in 1/64 inches
		inches := int(v / 64.0)
		numerator := int(v - (float64(inches) * 64.0))
		denominator := 64
		// Divide down to smallest denominator where the numerator is
		// still a whole number.
		for numerator != 0 && numerator&1 == 0 {
			numerator /= 2
			denominator /= 2
		}
		var b strings.Builder
		if feet != 0 {
			fmt.Fprintf(&b, "%d'-", feet)
		}
		// For something like 0.5, show 1/2" rather than 0 1/2"
		if feet != 0 || inches != 0 || numerator == 0 {
			fmt.Fprintf(&b, "%d", inches)
		}
		if numerator != 0 {
			fmt.Fprintf(&b, " %d/%d", numerator, denominator)
		}
		b.WriteString("\"")
		return b.String()
	}

	digits := ss.UnitDigitsAfterDecimal()
	minimum := 0.5 * math.Pow(10, float64(-digits))
	for v < minimum && v > geom.LengthEps {
		digits++
		minimum *= 0.1
	}
	return cFixed(digits, v)
}

func dimToString(dim int) string {
	switch dim {
	case 3:
		return "³"
	case 2:
		return "²"
	case 1:
		return ""
	}
	panic("ui: unexpected dimension")
}

// selectSIPrefixMm picks the metric unit for a measure whose order of
// magnitude in metres is ord, of dimension dim (1 length, 2 area, 3
// volume).
func selectSIPrefixMm(ord, dim int) (int, string) {
	switch dim {
	case 0, 1:
		switch {
		case ord >= 3:
			return 3, "km"
		case ord >= 0:
			return 0, "m"
		case ord >= -2:
			return -2, "cm"
		case ord >= -3:
			return -3, "mm"
		case ord >= -6:
			return -6, "µm"
		default:
			return -9, "nm"
		}
	case 2:
		switch {
		case ord >= 5:
			return 3, "km"
		case ord >= 0:
			return 0, "m"
		case ord >= -2:
			return -2, "cm"
		case ord >= -6:
			return -3, "mm"
		case ord >= -13:
			return -6, "µm"
		default:
			return -9, "nm"
		}
	case 3:
		switch {
		case ord >= 7:
			return 3, "km"
		case ord >= 0:
			return 0, "m"
		case ord >= -5:
			return -2, "cm"
		case ord >= -11:
			return -3, "mm"
		default:
			return -6, "µm"
		}
	}
	return 0, "m"
}

func selectSIPrefixInch(deg int) (int, string) {
	switch {
	case deg >= 0:
		return 0, "in"
	case deg >= -3:
		return -3, "mil"
	default:
		return -6, "µin"
	}
}

// MmToStringSI formats a measure of dimension dim with an SI prefix;
// dim 0 is a length, with SI prefixes only if the user wants them (C++
// SolveSpaceUI::MmToStringSI).
func (ss *Application) MmToStringSI(v float64, dim int) string {
	compact := false
	if dim == 0 {
		if !ss.UseSIPrefixes {
			return ss.MmToString(v, false)
		}
		compact = true
		dim = 1
	}
	inches := ss.ViewUnits == core.UnitInches || ss.ViewUnits == core.UnitFeetInches
	base := 1000.0
	if inches {
		base = 25.4
	}
	v /= math.Pow(base, float64(dim))
	vdeg := int(math.Log10(math.Abs(v)))
	unit := ""
	if math.Abs(v) > 0.0 {
		var sdeg int
		if inches {
			sdeg, unit = selectSIPrefixInch(vdeg / dim)
		} else {
			sdeg, unit = selectSIPrefixMm(vdeg, dim)
		}
		v /= math.Pow(10.0, float64(sdeg*dim))
	}
	if ss.ViewUnits == core.UnitFeetInches && math.Abs(v) > math.Pow(12.0, float64(dim)) {
		unit = "ft"
		v /= math.Pow(12.0, float64(dim))
	}
	pdeg := int(math.Ceil(math.Log10(math.Abs(v) + 1e-10)))
	sep := " "
	if compact {
		sep = ""
	}
	return cFormatG(pdeg+ss.UnitDigitsAfterDecimal(), v) + sep + unit + dimToString(dim)
}

// DegreeToString formats an angle in degrees.
func (ss *Application) DegreeToString(v float64) string {
	if math.Abs(v-math.Floor(v)) > 1e-10 {
		return cFixed(ss.AfterDecimalDegree, v)
	}
	return cFixed(0, v)
}

// ExprToMm evaluates an expression in view units to mm.
func (ss *Application) ExprToMm(e *core.Expr) float64 {
	return e.Eval(sketch.NewRuntime(ss.SK), nil) * ss.MmPerUnit()
}

// StringToMm parses a number in view units to mm (std::stod).
func (ss *Application) StringToMm(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v * ss.MmPerUnit()
}

// ChordTolMm is the chord tolerance in mm in use.
func (ss *Application) ChordTolMm() float64 { return ss.SK.Tol().ChordTolMm() }

// ExportChordTolMm is the chord tolerance for exported curves.
func (ss *Application) ExportChordTolMm() float64 { return ss.ExportChordTol / ss.ExportScale }

// UnitDigitsAfterDecimal is how many decimals the view unit shows.
func (ss *Application) UnitDigitsAfterDecimal() int {
	if ss.ViewUnits == core.UnitInches || ss.ViewUnits == core.UnitFeetInches {
		return ss.AfterDecimalInch
	}
	return ss.AfterDecimalMm
}

// SetUnitDigitsAfterDecimal sets how many decimals the view unit shows.
func (ss *Application) SetUnitDigitsAfterDecimal(v int) {
	if ss.ViewUnits == core.UnitInches || ss.ViewUnits == core.UnitFeetInches {
		ss.AfterDecimalInch = v
	} else {
		ss.AfterDecimalMm = v
	}
}

// CameraTangent is the perspective, or zero for orthographic.
func (ss *Application) CameraTangent() float64 {
	if !ss.UsePerspectiveProj {
		return 0
	}
	return ss.CameraTangentSetting
}

// Fonts is the TrueType fonts the sketch can use.
func (ss *Application) Fonts() *ttf.FontList {
	if ss.fonts == nil {
		return ttf.DefaultFonts
	}
	return ss.fonts
}

// cFormatG is C's "%.*g": a negative precision means the default, 6.
func cFormatG(prec int, v float64) string {
	if prec < 0 {
		prec = 6
	}
	if s, ok := cSpecial(v); ok {
		return s
	}
	return fmt.Sprintf("%.*g", prec, v)
}

// UseGL reports whether the windows are to be drawn with OpenGL: the
// command line's choice if it made one, else the setting.
func (ss *Application) UseGL() bool {
	switch ss.RendererOverride {
	case "opengl":
		return true
	case "software":
		return false
	}
	return ss.UseOpenGL
}

// SetUseOpenGL changes the renderer setting and makes both windows
// draw with the renderer it picks from the next frame on.
func (ss *Application) SetUseOpenGL(on bool) {
	ss.UseOpenGL = on
	ss.RendererOverride = ""
	if gw := ss.GW; gw.window != nil {
		gw.canvas = gw.window.CreateRenderer(on)
		gw.persistentDirty = true
		gw.Invalidate(false)
	}
	if tw := ss.TW; tw.window != nil {
		tw.canvas = tw.window.CreateRenderer(on)
		tw.window.Invalidate()
	}
	ss.ScheduleShowTW()
}

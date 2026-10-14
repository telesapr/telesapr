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

// modelColors is how many model colours the settings keep.
const modelColors = 8

// maxRecent is how many recent files are remembered.
const maxRecent = 8

// Application is the application: the sketch, its two windows and the
// user's configuration.
type Application struct {
	Platform platform.Platform
	Settings core.Settings

	SK *sketch.Sketch
	GW *graphicsWindow
	TW *textWindow

	// config is the user's configuration, as saved between runs.
	config

	// BackgroundShells has the regenerations the interface schedules
	// make the solids off its thread (shelljob.go); the application
	// sets it, the tests don't.
	BackgroundShells bool
	shellJobs        shellJobs
	// lastDimensions are what the drawing's dimensions measured when
	// it was last made (drawingdim.go).
	lastDimensions []DimensionResult
	// RendererOverride, "opengl" or "software", is the command line's
	// choice of renderer for this run, and wins over the UseOpenGL
	// setting until the setting is changed.
	RendererOverride string
	// ScreenshotFile, while set, is where the next redraw is saved.
	ScreenshotFile string
	// ExportMode is on while a regeneration follows the export
	// tolerance (EnterExportMode).
	ExportMode bool
	// StepOneSurfacePerFace has STEP export write a face of several
	// patches as one surface; the tests turn it off to compare.
	StepOneSurfacePerFace bool
	// Windowless makes the graphics window behave as if there were
	// none: the view is never fitted to it and the camera is 297 by
	// 210. The command-line tool sets it.
	Windowless bool

	// ViewUnits is the unit lengths are shown and typed in: the
	// document's (Sketch.Units), which it follows.
	ViewUnits          core.Unit
	Explode            bool
	ExplodeDistance    float64
	UsePerspectiveProj bool

	// The tangent arcs made non-parametrically.
	TangentArcRadius float64
	TangentArcManual bool
	TangentArcModify bool

	SaveFile string

	// AllConsistent is false while a file is loading, so nothing draws a
	// sketch with dangling references.
	AllConsistent bool

	scheduledGenerateAll bool
	scheduledShowTW      bool
	refreshTimer         platform.Timer

	// scriptHost is what scripts work on the sketch through, and
	// menu is the menu bar's table, menuTable: the handlers that look a
	// command up reach it through the application, since the table
	// names them.
	menu []menuEntry

	// messageSink, while a script runs, takes what would be shown in
	// a message dialog.
	scriptHost  *ScriptHost
	messageSink func(text string, isError bool)
	// console is the window scripts are typed in, see Console.
	console *ScriptConsole
	// AgentSocketDir is where the socket AI programs connect to is
	// made, the user's own directory for them if "".
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
	// notice a regeneration.
	lastGeneration uint64
}

// New makes the application on a platform; Init loads the settings and
// opens the windows.
func New(p platform.Platform) *Application {
	ss := &Application{Platform: p, Settings: p.Settings(), StepOneSurfacePerFace: true, menu: menuTable}
	ss.GW = newGraphicsWindow(ss)
	ss.TW = newTextWindow(ss)
	return ss
}

// LoadSettings reads the configuration.
func (ss *Application) LoadSettings() {
	ss.TangentArcRadius = 10.0
	ss.ExplodeDistance = 1.0
	ss.ExportMode = false
	ss.Thaw(ss.Settings)
	ss.ViewUnits = ss.DefaultUnits
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
		// The tolerance is a single-precision number in the settings and
		// in the dialogs that set it, so the default of 0.1% reads back
		// from the settings file as 0.10000000149011612%; that is not a
		// change of tolerance, and must not read every linked file again.
		if float32(ss.SK.ChordTolPercent()) != float32(was) && ss.SK.ReloadAllLinked != nil {
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

// SaveSettings writes the configuration.
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
	ss.Freeze(s)

	// And the default styles, colours and line widths and such.
	if ss.SK != nil {
		ss.SK.FreezeDefaultStyles(s)
	}
}

func rgbi(r, g, b uint8) core.RGBA { return core.NewRGBA(r, g, b, 255) }

// Refresh regenerates what was scheduled, then shows the text window.
func (ss *Application) Refresh() {
	// GenerateAll must happen before updating displays.
	if ss.scheduledGenerateAll {
		ss.scheduledGenerateAll = false
		if ss.BackgroundShells && !ss.GW.draggingGeometry() {
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
// that depends on it.
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

// afterGenerate is what the interface does after a regeneration.
func (ss *Application) afterGenerate() {
	ss.AllConsistent = true
	ss.GW.persistentDirty = true
	ss.GW.centerOfMass.Dirty = true
	ss.lastGeneration = ss.SK.Generation
	ss.GW.Invalidate(false)
}

// ActiveGroupsOkay reports whether every group up to the active one
// solved.
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
// inches instead of feet and inches, which can't be parsed back.
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
// dim 0 is a length, with SI prefixes only if the user wants them.
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

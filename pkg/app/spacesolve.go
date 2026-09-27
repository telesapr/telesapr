package app

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	ccore "cogentcore.org/core/core"
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

const (
	modelColorCount = 8
	maxUndoEntries  = 100
	maxRecentFiles  = 8

	slvsHeader = "\xB1\xB2\xB3SolveSpaceREVa"
)

// RectangleMargin stores the margin applied around exported content.
type RectangleMargin struct {
	Left   float64
	Right  float64
	Top    float64
	Bottom float64
}

// ExportCanvas stores the explicit dimensions for exports.
type ExportCanvas struct {
	Width  float64
	Height float64
	DX     float64
	DY     float64
}

// GCodeSettings mirrors the CNC export parameters from the C++ implementation.
type GCodeSettings struct {
	Depth      float64
	SafeHeight float64
	Passes     int
	Feed       float64
	PlungeFeed float64
}

// Preferences bundles the various configuration values previously stored on
// SolveSpaceUI. The fields intentionally mirror the original names to simplify
// porting code from the legacy implementation.
type Preferences struct {
	ModelColors                 [modelColorCount]core.RgbaColor
	LightDir                    [2]geom.Vector
	LightIntensity              [2]float64
	AmbientIntensity            float64
	ChordTolerance              float64
	ChordToleranceCalculated    float64
	MaxSegments                 int
	ExportChordTolerance        float64
	ExportMaxSegments           int
	TimeoutRedundantConstraints time.Duration
	AnimationSpeed              time.Duration
	CameraTangent               float64
	GridSpacing                 float64
	ExportScale                 float64
	ExportOffset                float64
	ArcDimDefaultDiameter       bool
	ShowFullFilePath            bool
	FixExportColors             bool
	ExportBackgroundColor       bool
	DrawBackFaces               bool
	ShowContourAreas            bool
	CheckClosedContour          bool
	CameraNav                   bool
	TurntableNav                bool
	ImmediatelyEditDimension    bool
	AutomaticLineConstraints    bool
	ShowToolbar                 bool
	ScreenshotFile              string
	BackgroundColor             core.RgbaColor
	ExportShadedTriangles       bool
	ExportPwlCurves             bool
	ExportCanvasSizeAuto        bool
	ExportMode                  bool
	ExportMargin                RectangleMargin
	ExportCanvas                ExportCanvas
	GCode                       GCodeSettings
	ViewUnits                   core.Unit
	AfterDecimalMm              int
	AfterDecimalInch            int
	AfterDecimalDegree          int
	UseSIPrefixes               bool
	AutosaveInterval            time.Duration
	Explode                     bool
	ExplodeDistance             float64
	UsePerspectiveProj          bool
	TangentArcRadius            float64
	ExportShadedBackground      bool
}

func defaultPreferences() Preferences {
	bg := core.NewRgbaColor(0, 0, 0, 255)
	return Preferences{
		ModelColors: [modelColorCount]core.RgbaColor{
			core.NewRgbaColor(150, 150, 150, 255),
			core.NewRgbaColor(100, 100, 100, 255),
			core.NewRgbaColor(30, 30, 30, 255),
			core.NewRgbaColor(150, 0, 0, 255),
			core.NewRgbaColor(0, 100, 0, 255),
			core.NewRgbaColor(0, 80, 80, 255),
			core.NewRgbaColor(0, 0, 130, 255),
			core.NewRgbaColor(80, 0, 80, 255),
		},
		LightDir: [2]geom.Vector{
			{X: -1, Y: 1, Z: 0},
			{X: 1, Y: 0, Z: 0},
		},
		LightIntensity:              [2]float64{1.0, 0.5},
		AmbientIntensity:            0.3,
		ChordTolerance:              0.1,
		MaxSegments:                 20,
		ExportChordTolerance:        0.1,
		ExportMaxSegments:           64,
		TimeoutRedundantConstraints: time.Second,
		AnimationSpeed:              800 * time.Millisecond,
		CameraTangent:               0.3 / 1e3,
		GridSpacing:                 5.0,
		ExportScale:                 1.0,
		ArcDimDefaultDiameter:       false,
		ShowFullFilePath:            true,
		FixExportColors:             true,
		DrawBackFaces:               true,
		CameraNav:                   false,
		TurntableNav:                false,
		ImmediatelyEditDimension:    true,
		CheckClosedContour:          true,
		AutomaticLineConstraints:    true,
		ShowContourAreas:            false,
		ExportShadedTriangles:       true,
		ExportPwlCurves:             false,
		ExportCanvasSizeAuto:        true,
		BackgroundColor:             bg,
		ExportMargin: RectangleMargin{
			Left: 5, Right: 5, Top: 5, Bottom: 5,
		},
		ExportCanvas: ExportCanvas{
			Width: 100, Height: 100, DX: 5, DY: 5,
		},
		GCode: GCodeSettings{
			Depth: 10, SafeHeight: 5, Passes: 1, Feed: 10, PlungeFeed: 10,
		},
		ViewUnits:          core.UnitMM,
		AfterDecimalMm:     2,
		AfterDecimalInch:   3,
		AfterDecimalDegree: 2,
		UseSIPrefixes:      false,
		AutosaveInterval:   5 * time.Minute,
		Explode:            false,
		ExplodeDistance:    1.0,
		UsePerspectiveProj: false,
		TangentArcRadius:   10.0,
	}
}

// FileState keeps track of persistence-related metadata.
type FileState struct {
	SavePath     string
	Unsaved      bool
	AutosavePath string
}

// UndoState captures a snapshot of the sketch plus active group information.
type UndoState struct {
	Sketch      *sketch.Sketch
	ActiveGroup core.HGroup
}

type undoStack struct {
	mu      sync.Mutex
	entries []UndoState
	limit   int
}

func newUndoStack(limit int) *undoStack {
	return &undoStack{limit: limit}
}

func (s *undoStack) Push(state UndoState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state.Sketch == nil {
		return
	}
	if len(s.entries) == s.limit {
		copy(s.entries, s.entries[1:])
		s.entries = s.entries[:s.limit-1]
	}
	s.entries = append(s.entries, state)
}

func (s *undoStack) Pop() (UndoState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.entries) == 0 {
		return UndoState{}, false
	}
	idx := len(s.entries) - 1
	state := s.entries[idx]
	s.entries = s.entries[:idx]
	return state, true
}

func (s *undoStack) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.entries[:0]
}

// GenerateKind mirrors SolveSpaceUI::Generate enumeration.
type GenerateKind uint32

const (
	GenerateDirty GenerateKind = iota
	GenerateAll
	GenerateRegenerate
	GenerateUntilActive
)

// SpaceSolve is the Go translation of the legacy SolveSpaceUI class. It holds
// the editable document, solver state, and user-interface integration points.
type SpaceSolve struct {
	mu sync.Mutex

	ui *UI

	sketch *sketch.Sketch

	prefs Preferences

	undo *undoStack
	redo *undoStack

	clipboard sketch.Clipboard

	activeGroup   core.HGroup
	allConsistent bool

	fileState   FileState
	recentFiles []string

	scheduledGenerateAll bool
	scheduledShowTW      bool

	refreshTimer  *time.Timer
	autosaveTimer *time.Timer

	images map[string]*LinkedImage
}

// LinkedImage tracks the last-loaded linked bitmap. The Cogent Core port keeps
// the metadata so reloading logic can be implemented incrementally.
type LinkedImage struct {
	Path     string
	Modified time.Time
}

// NewSpaceSolve constructs a SpaceSolve instance with default preferences,
// empty sketch, and Cogent Core UI.
func NewSpaceSolve() *SpaceSolve {
	sk := sketch.NewSketch()
	ss := &SpaceSolve{
		sketch: sk,
		prefs:  defaultPreferences(),
		undo:   newUndoStack(maxUndoEntries),
		redo:   newUndoStack(maxUndoEntries),
		images: make(map[string]*LinkedImage),
	}

	ss.ui = New(ss)
	ss.NewFile()
	return ss
}

// ClearExisting discards the current sketch and solver state without altering
// recent files or window metadata. It mirrors the legacy ClearExisting helper.
func (ss *SpaceSolve) ClearExisting() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.sketch = sketch.NewSketch()
	ss.undo.Clear()
	ss.redo.Clear()
	ss.activeGroup = core.HGroup{}
	ss.allConsistent = false
}

// AfterNewFile performs the post-load regeneration scheduling used throughout
// the classic SolveSpace UI.
func (ss *SpaceSolve) AfterNewFile() {
	ss.ScheduleGenerateAll()
	ss.ScheduleShowTW()
}

// HasUnsavedChanges reports whether the working document differs from the last
// saved version on disk.
func (ss *SpaceSolve) HasUnsavedChanges() bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.fileState.Unsaved
}

// MarkDirty records that the working sketch has diverged from the last saved
// snapshot.
func (ss *SpaceSolve) MarkDirty() {
	ss.mu.Lock()
	ss.markDirtyLocked()
	ss.mu.Unlock()
}

func (ss *SpaceSolve) markDirtyLocked() {
	ss.fileState.Unsaved = true
}

// OkayToStartNewFile automatically preserves the current sketch if it has been
// saved previously. The legacy UI prompted the user; in the Cogent port we
// silently auto-save to minimize data loss until dialogs arrive.
func (ss *SpaceSolve) OkayToStartNewFile() bool {
	ss.mu.Lock()
	unsaved := ss.fileState.Unsaved
	path := ss.fileState.SavePath
	ss.mu.Unlock()

	if !unsaved {
		return true
	}

	if path == "" {
		ss.setStatus("Current sketch has unsaved changes; continuing will discard them.")
		return true
	}

	if err := ss.Save(path); err != nil {
		ss.setStatus(fmt.Sprintf("Unable to auto-save %s: %v", filepath.Base(path), err))
		return true
	}

	ss.setStatus("Auto-saved current sketch before continuing.")
	return true
}

// GetFilenameAndSave mirrors the SolveSpaceUI helper by saving to the existing
// filename when available. Save As flows are deferred until file dialogs are
// wired up.
func (ss *SpaceSolve) GetFilenameAndSave(saveAs bool) error {
	ss.mu.Lock()
	path := ss.fileState.SavePath
	ss.mu.Unlock()

	if saveAs || path == "" {
		return errors.New("filename prompt not implemented; specify a path explicitly")
	}
	return ss.Save(path)
}

// SaveToFile persists the sketch to the supplied path and updates the status bar.
func (ss *SpaceSolve) SaveToFile(path string) error {
	if err := ss.Save(path); err != nil {
		return err
	}
	ss.setStatus(fmt.Sprintf("Saved %s", filepath.Base(path)))
	return nil
}

// LoadFromFile loads a document from disk and reports success for menu handlers.
func (ss *SpaceSolve) LoadFromFile(path string, canCancel bool) bool {
	if err := ss.Load(path); err != nil {
		if path == "" {
			path = "(unspecified)"
		}
		ss.setStatus(fmt.Sprintf("Failed to load %s: %v", filepath.Base(path), err))
		return false
	}
	return true
}

// UIInstance returns the Cogent Core UI associated with this app.
func (ss *SpaceSolve) UIInstance() *UI {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.ui
}

// Run launches the Cogent Core UI event loop.
func (ss *SpaceSolve) Run() {
	if ss.ui == nil {
		return
	}
	ss.ui.Run()
}

// RememberUndo captures the current sketch state onto the undo stack and clears
// the redo stack.
func (ss *SpaceSolve) RememberUndo() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	snap := UndoState{
		Sketch:      cloneSketch(ss.sketch),
		ActiveGroup: ss.activeGroup,
	}
	ss.undo.Push(snap)
	ss.redo.Clear()
}

// Undo restores the most recent undo state, pushing the current state to redo.
func (ss *SpaceSolve) Undo() bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	state, ok := ss.undo.Pop()
	if !ok {
		return false
	}
	ss.redo.Push(UndoState{
		Sketch:      ss.sketch,
		ActiveGroup: ss.activeGroup,
	})
	ss.applyUndoState(state)
	ss.markDirtyLocked()
	return true
}

// Redo reapplies a previously undone state.
func (ss *SpaceSolve) Redo() bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	state, ok := ss.redo.Pop()
	if !ok {
		return false
	}
	ss.undo.Push(UndoState{
		Sketch:      ss.sketch,
		ActiveGroup: ss.activeGroup,
	})
	ss.applyUndoState(state)
	ss.markDirtyLocked()
	return true
}

func (ss *SpaceSolve) applyUndoState(state UndoState) {
	if state.Sketch == nil {
		return
	}
	ss.sketch = state.Sketch
	ss.activeGroup = state.ActiveGroup
}

// MmToString converts a millimeter value to a formatted string respecting the
// current unit settings.
func (ss *SpaceSolve) MmToString(v float64, editable bool) string {
	unit := ss.MmPerUnit()
	value := v / unit
	format := "%.*f"
	precision := ss.UnitDigitsAfterDecimal()
	if editable {
		precision = max(precision, 4)
	}
	return fmt.Sprintf(format, precision, value)
}

// MmToStringSI converts to SI-prefixed string.
func (ss *SpaceSolve) MmToStringSI(v float64, dim int) string {
	if !ss.prefs.UseSIPrefixes {
		return ss.MmToString(v, false)
	}
	unit := ss.MmPerUnit()
	value := v / unit
	if dim == 2 {
		value = value / 1e2
	} else if dim == 3 {
		value = value / 1e3
	}
	prefix := ""
	switch {
	case math.Abs(value) >= 1e3:
		value /= 1e3
		prefix = "k"
	case math.Abs(value) >= 1:
		prefix = ""
	case math.Abs(value) >= 1e-3:
		value *= 1e3
		prefix = "m"
	default:
		value *= 1e6
		prefix = "µ"
	}
	return fmt.Sprintf("%.*f %s%s", ss.UnitDigitsAfterDecimal(), value, prefix, ss.UnitName())
}

// DegreeToString formats an angle value according to preferences.
func (ss *SpaceSolve) DegreeToString(v float64) string {
	return fmt.Sprintf("%.*f°", ss.prefs.AfterDecimalDegree, v)
}

// ExprToMm evaluates an expression in the context of the current sketch.
func (ss *SpaceSolve) ExprToMm(e *core.Expr) float64 {
	if e == nil {
		return 0
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return e.Eval(sketchParams{sk: ss.sketch}, nil)
}

// StringToMm parses a string in the current unit system.
func (ss *SpaceSolve) StringToMm(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v * ss.MmPerUnit()
}

// UnitName returns a localized unit label.
func (ss *SpaceSolve) UnitName() string {
	switch ss.prefs.ViewUnits {
	case core.UnitInches:
		return "in"
	case core.UnitMeters:
		return "m"
	case core.UnitFeetInches:
		return "ft"
	default:
		return "mm"
	}
}

// MmPerUnit returns the conversion factor between stored mm and displayed
// units.
func (ss *SpaceSolve) MmPerUnit() float64 {
	switch ss.prefs.ViewUnits {
	case core.UnitInches:
		return 25.4
	case core.UnitMeters:
		return 1000
	case core.UnitFeetInches:
		return 304.8
	default:
		return 1
	}
}

// UnitDigitsAfterDecimal returns the configured precision for the active unit.
func (ss *SpaceSolve) UnitDigitsAfterDecimal() int {
	switch ss.prefs.ViewUnits {
	case core.UnitInches, core.UnitFeetInches:
		return ss.prefs.AfterDecimalInch
	case core.UnitMeters:
		return ss.prefs.AfterDecimalMm
	default:
		return ss.prefs.AfterDecimalMm
	}
}

// SetUnitDigitsAfterDecimal updates the preferred precision for the active unit.
func (ss *SpaceSolve) SetUnitDigitsAfterDecimal(v int) {
	switch ss.prefs.ViewUnits {
	case core.UnitInches, core.UnitFeetInches:
		ss.prefs.AfterDecimalInch = v
	case core.UnitMeters:
		ss.prefs.AfterDecimalMm = v
	default:
		ss.prefs.AfterDecimalMm = v
	}
}

// ChordTolMm returns the display chord tolerance in millimeters.
func (ss *SpaceSolve) ChordTolMm() float64 {
	return ss.prefs.ChordTolerance
}

// ExportChordTolMm returns the export chord tolerance in millimeters.
func (ss *SpaceSolve) ExportChordTolMm() float64 {
	return ss.prefs.ExportChordTolerance
}

// GetMaxSegments returns the maximum number of polyline segments generated for
// splines.
func (ss *SpaceSolve) GetMaxSegments() int {
	return ss.prefs.MaxSegments
}

// CameraTangent returns the current camera tangent setting.
func (ss *SpaceSolve) CameraTangent() float64 {
	return ss.prefs.CameraTangent
}

// HandleFileCommand executes a menu command in the File menu.
func (ss *SpaceSolve) HandleFileCommand(cmd Command) {
	switch cmd {
	case CommandNew:
		if ss.OkayToStartNewFile() {
			ss.NewFile()
		}
	case CommandSave:
		if err := ss.GetFilenameAndSave(false); err != nil {
			ss.setStatus(fmt.Sprintf("Save failed: %v", err))
		} else {
			ss.setStatus("Saved sketch")
		}
	case CommandSaveAs:
		ss.setStatus("Save As requires a file dialog (not implemented yet)")
	case CommandOpen:
		ss.setStatus("Open dialog not yet wired up")
	case CommandExit:
		ss.Close()
		ccore.TheApp.QuitReq()
	default:
		ss.setStatus(fmt.Sprintf("%s not implemented yet", describeCommand(cmd)))
	}
}

// HandleEditCommand executes an Edit menu command.
func (ss *SpaceSolve) HandleEditCommand(cmd Command) {
	switch cmd {
	case CommandUndo:
		if !ss.Undo() {
			ss.setStatus("Nothing to undo")
		} else {
			ss.setStatus("Undo")
		}
	case CommandRedo:
		if !ss.Redo() {
			ss.setStatus("Nothing to redo")
		} else {
			ss.setStatus("Redo")
		}
	case CommandRegenerateAll:
		ss.GenerateAll(GenerateDirty, false, false)
	default:
		ss.setStatus(fmt.Sprintf("%s is not implemented yet", describeCommand(cmd)))
	}
}

// HandleClipboardCommand handles Cut/Copy/Paste commands.
func (ss *SpaceSolve) HandleClipboardCommand(cmd Command) {
	ss.setStatus(fmt.Sprintf("%s clipboard action pending", describeCommand(cmd)))
}

// HandleViewCommand implements the View menu actions.
func (ss *SpaceSolve) HandleViewCommand(cmd Command) {
	switch cmd {
	case CommandZoomIn, CommandZoomOut, CommandZoomToFit:
		ss.setStatus("Zoom controls not wired to canvas yet")
	case CommandShowToolbar:
		ss.prefs.ShowToolbar = !ss.prefs.ShowToolbar
		ss.setStatus(fmt.Sprintf("Toolbar visibility %t", ss.prefs.ShowToolbar))
	case CommandShowGrid:
		ss.setStatus("Grid display is controlled by the canvas subsystem")
	default:
		ss.setStatus(fmt.Sprintf("%s pending implementation", describeCommand(cmd)))
	}
}

// HandleHelpCommand handles Help menu entries.
func (ss *SpaceSolve) HandleHelpCommand(cmd Command) {
	switch cmd {
	case CommandAbout:
		ss.setStatus("SpaceSolve — Cogent Core port in progress")
	default:
		ss.setStatus(fmt.Sprintf("%s not implemented yet", describeCommand(cmd)))
	}
}

// HandleGroupCommand is a placeholder for Group menu commands.
func (ss *SpaceSolve) HandleGroupCommand(cmd Command) {
	ss.setStatus(fmt.Sprintf("%s is not available yet", describeCommand(cmd)))
}

// HandleRequestCommand handles sketch request commands.
func (ss *SpaceSolve) HandleRequestCommand(cmd Command) {
	ss.setStatus(fmt.Sprintf("%s tool not yet implemented", describeCommand(cmd)))
}

// HandleConstraintCommand handles constraint menu actions.
func (ss *SpaceSolve) HandleConstraintCommand(cmd Command) {
	ss.setStatus(fmt.Sprintf("%s constraint not yet implemented", describeCommand(cmd)))
}

// HandleAnalyzeCommand handles analysis actions.
func (ss *SpaceSolve) HandleAnalyzeCommand(cmd Command) {
	if cmd == CommandRegenerateAll {
		ss.GenerateAll(GenerateDirty, false, false)
		return
	}
	ss.setStatus(fmt.Sprintf("%s analysis pending", describeCommand(cmd)))
}

// NewFile clears the current sketch and resets state.
func (ss *SpaceSolve) NewFile() {
	ss.ClearExisting()
	ss.mu.Lock()
	ss.fileState = FileState{Unsaved: true}
	ss.mu.Unlock()
	ss.RememberUndo()
	ss.ScheduleAutosave()
	ss.AfterNewFile()
	ss.setStatus("Started a new sketch")
}

// Load loads a SolveSpace .slvs document.
func (ss *SpaceSolve) Load(path string) error {
	if path == "" {
		return errors.New("no filename specified")
	}
	path = filepath.Clean(path)

	autosavePath, useAutosave := ss.loadAutosaveFor(path)
	source := path
	if useAutosave {
		source = autosavePath
	}

	doc, err := slvs.LoadFile(source)
	if err != nil {
		ss.setStatus(fmt.Sprintf("Failed to read %s: %v", filepath.Base(source), err))
		ss.NewFile()
		return err
	}

	ss.mu.Lock()
	ss.sketch = cloneSketch(doc.Sketch)
	ss.fileState.SavePath = path
	ss.fileState.Unsaved = useAutosave
	if useAutosave {
		ss.fileState.AutosavePath = autosavePath
	} else {
		ss.fileState.AutosavePath = ""
	}
	ss.mu.Unlock()

	ss.RememberUndo()
	ss.AddRecentFile(path)
	ss.ScheduleAutosave()
	ss.AfterNewFile()

	if useAutosave {
		ss.setStatus(fmt.Sprintf("Loaded autosave for %s", filepath.Base(path)))
	} else {
		ss.setStatus(fmt.Sprintf("Loaded %s", filepath.Base(path)))
	}
	return nil
}

func (ss *SpaceSolve) loadAutosaveFor(path string) (string, bool) {
	backup := autosavePath(path)
	info, err := os.Stat(backup)
	if err != nil {
		return "", false
	}
	orig, err := os.Stat(path)
	if err != nil {
		return backup, true
	}
	if info.ModTime().After(orig.ModTime()) {
		return backup, true
	}
	return "", false
}

// Save writes the sketch to disk. When path is empty the existing save location
// is used.
func (ss *SpaceSolve) Save(path string) error {
	ss.mu.Lock()
	if path == "" {
		path = ss.fileState.SavePath
	}
	ss.mu.Unlock()
	if path == "" {
		return errors.New("no filename specified")
	}
	path = filepath.Clean(path)

	if err := ss.writeDocument(path); err != nil {
		return err
	}

	ss.mu.Lock()
	ss.fileState.SavePath = path
	ss.fileState.Unsaved = false
	ss.mu.Unlock()

	ss.AddRecentFile(path)
	ss.RemoveAutosave()
	return nil
}

func (ss *SpaceSolve) writeDocument(path string) error {
	ss.mu.Lock()
	doc := &slvs.Document{
		Header: slvsHeader,
		Sketch: cloneSketch(ss.sketch),
	}
	ss.mu.Unlock()

	data := doc.Save()
	return os.WriteFile(filepath.Clean(path), data, 0o644)
}

// AddRecentFile pushes a path onto the MRU list.
func (ss *SpaceSolve) AddRecentFile(path string) {
	path = filepath.Clean(path)
	ss.mu.Lock()
	defer ss.mu.Unlock()

	dupe := slices.IndexFunc(ss.recentFiles, func(s string) bool { return s == path })
	if dupe >= 0 {
		ss.recentFiles = append(ss.recentFiles[:dupe], ss.recentFiles[dupe+1:]...)
	}
	ss.recentFiles = append([]string{path}, ss.recentFiles...)
	if len(ss.recentFiles) > maxRecentFiles {
		ss.recentFiles = ss.recentFiles[:maxRecentFiles]
	}
	if ss.ui != nil {
		ss.ui.SetRecentFiles(ss.recentFiles)
	}
}

// OpenRecentFile loads a recent document if it exists.
func (ss *SpaceSolve) OpenRecentFile(path string) {
	if !ss.OkayToStartNewFile() {
		return
	}
	if err := ss.Load(path); err != nil {
		ss.setStatus(fmt.Sprintf("Failed to open %s: %v", path, err))
	}
}

// ScheduleGenerateAll queues a regenerate pass. The Cogent port coalesces
// requests to avoid redundant solver runs.
func (ss *SpaceSolve) ScheduleGenerateAll() {
	ss.mu.Lock()
	if ss.scheduledGenerateAll {
		ss.mu.Unlock()
		return
	}
	ss.scheduledGenerateAll = true
	ss.mu.Unlock()

	go func() {
		time.Sleep(10 * time.Millisecond)
		ss.GenerateAll(GenerateDirty, false, false)
	}()
}

// ScheduleShowTW is a placeholder maintained for API parity.
func (ss *SpaceSolve) ScheduleShowTW() {
	ss.mu.Lock()
	ss.scheduledShowTW = true
	ss.mu.Unlock()
}

// ScheduleAutosave arms the autosave timer based on preferences.
func (ss *SpaceSolve) ScheduleAutosave() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.autosaveTimer != nil {
		ss.autosaveTimer.Stop()
	}
	if ss.prefs.AutosaveInterval <= 0 {
		return
	}
	ss.autosaveTimer = time.AfterFunc(ss.prefs.AutosaveInterval, ss.Autosave)
}

// Autosave writes the backup file (filename.slvs~).
func (ss *SpaceSolve) Autosave() {
	ss.mu.Lock()
	path := ss.fileState.SavePath
	ss.mu.Unlock()

	if path == "" {
		return
	}
	backup := autosavePath(path)
	if err := ss.writeDocument(backup); err != nil {
		log.Printf("autosave failed: %v", err)
	} else {
		ss.mu.Lock()
		ss.fileState.AutosavePath = backup
		ss.mu.Unlock()
	}
	ss.ScheduleAutosave()
}

// RemoveAutosave deletes the autosave copy after a successful save.
func (ss *SpaceSolve) RemoveAutosave() {
	ss.mu.Lock()
	path := ss.fileState.AutosavePath
	ss.fileState.AutosavePath = ""
	ss.mu.Unlock()
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

// GenerateAll regenerates and solves the sketch with the engine, as
// C++ SolveSpaceUI::GenerateAll does, and reports whether every group
// solved.
func (ss *SpaceSolve) GenerateAll(kind GenerateKind, andFindFree, genForBBox bool) {
	ss.mu.Lock()
	ss.scheduledGenerateAll = false
	sk := ss.sketch
	ss.mu.Unlock()

	var failed error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				failed = fmt.Errorf("%v", rec)
			}
		}()
		sk.GenerateAll()
	}()
	if failed == nil {
		for _, g := range sk.Groups {
			if g.Solved.How != core.SolveOkay && g.Solved.How != core.SolveRedundantOkay {
				failed = fmt.Errorf("group %s didn't solve", g.Name)
				break
			}
		}
	}

	ss.mu.Lock()
	ss.allConsistent = failed == nil
	ss.mu.Unlock()
	if failed != nil {
		ss.setStatus(fmt.Sprintf("Solver error: %v", failed))
		return
	}
	ss.setStatus("Regenerated the sketch")
}

// SolveGroup is a placeholder that currently reruns the full solver.
func (ss *SpaceSolve) SolveGroup(h core.HGroup, andFindFree bool) {
	ss.GenerateAll(GenerateDirty, andFindFree, false)
}

// SolveGroupAndReport triggers SolveGroup and reports success in the status bar.
func (ss *SpaceSolve) SolveGroupAndReport(h core.HGroup, andFindFree bool) {
	ss.SolveGroup(h, andFindFree)
	if ss.allConsistent {
		ss.setStatus("Group solved successfully")
	}
}

// TestRankForGroup currently runs the global regenerate and returns the last
// solver result.
func (ss *SpaceSolve) TestRankForGroup(h core.HGroup) core.SolveResult {
	ss.GenerateAll(GenerateDirty, false, false)
	if ss.allConsistent {
		return core.SolveOkay
	}
	return core.SolveDidntConverge
}

// ActiveGroupsOkay returns true if the last solver pass succeeded.
func (ss *SpaceSolve) ActiveGroupsOkay() bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.allConsistent
}

// Clear releases timers and resets transient state.
func (ss *SpaceSolve) Clear() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if ss.refreshTimer != nil {
		ss.refreshTimer.Stop()
		ss.refreshTimer = nil
	}
	if ss.autosaveTimer != nil {
		ss.autosaveTimer.Stop()
		ss.autosaveTimer = nil
	}
	ss.images = make(map[string]*LinkedImage)
	ss.recentFiles = nil
}

// Close shuts down timers and clears autosave artifacts.
func (ss *SpaceSolve) Close() {
	ss.Clear()
	ss.RemoveAutosave()
}

func (ss *SpaceSolve) setStatus(msg string) {
	ss.mu.Lock()
	ui := ss.ui
	ss.mu.Unlock()
	if ui != nil {
		ui.setStatus(msg)
	}
}

func describeCommand(cmd Command) string {
	return fmt.Sprintf("Command %d", cmd)
}

// autosavePath replaces the extension with .slvs~.
func autosavePath(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, fmt.Sprintf("%s.slvs~", name))
}

// cloneSketch performs a deep copy of the provided sketch.
func cloneSketch(src *sketch.Sketch) *sketch.Sketch {
	if src == nil {
		return sketch.NewSketch()
	}
	dst := sketch.NewSketch()
	dst.Groups = cloneGroups(src.Groups)
	dst.GroupOrder = append([]core.HGroup(nil), src.GroupOrder...)
	dst.Requests = append([]sketch.Request(nil), src.Requests...)
	dst.Constraints = append([]core.Constraint(nil), src.Constraints...)
	dst.Params = append([]core.Param(nil), src.Params...)
	dst.Styles = append([]core.Style(nil), src.Styles...)
	dst.Entities = append([]core.Entity(nil), src.Entities...)
	return dst
}

func cloneGroups(groups []sketch.Group) []sketch.Group {
	if len(groups) == 0 {
		return nil
	}
	dst := make([]sketch.Group, len(groups))
	for i := range groups {
		dst[i] = groups[i]
		if groups[i].Mesh != nil {
			dst[i].Mesh = cloneMesh(groups[i].Mesh)
		}
		if groups[i].Remap != nil {
			dst[i].Remap = cloneEntityMap(groups[i].Remap)
		}
		if len(groups[i].Solved.Remove) > 0 {
			dst[i].Solved.Remove = append([]core.HConstraint(nil), groups[i].Solved.Remove...)
		}
	}
	return dst
}

func cloneMesh(m *sketch.Mesh) *sketch.Mesh {
	if m == nil {
		return nil
	}
	cp := &sketch.Mesh{
		Vertices:  append([]geom.Vector(nil), m.Vertices...),
		Triangles: append([]sketch.Triangle(nil), m.Triangles...),
	}
	return cp
}

func cloneEntityMap(src core.EntityMap) core.EntityMap {
	if src == nil {
		return nil
	}
	dst := make(core.EntityMap, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

type sketchParams struct {
	sk *sketch.Sketch
}

func (sp sketchParams) ParamValue(h core.HParam) (float64, bool) {
	if sp.sk == nil {
		return 0, false
	}
	for i := range sp.sk.Params {
		if sp.sk.Params[i].H == h {
			return sp.sk.Params[i].Val, true
		}
	}
	return 0, false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

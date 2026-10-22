package ui

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// ScriptHost is the application as scripts see it: it lets package
// api work on the sketch that is on screen, with the windows keeping
// up. A script runs on a goroutine of its own, so that the windows
// stay alive while it does; whatever it does to the sketch, it does
// through Run, on the event loop's goroutine.
type ScriptHost struct {
	ss *Application

	// Messages is where what the application would tell the user in a
	// dialog goes while a script runs; nil drops it.
	Messages io.Writer

	// poster is the event loop, nil when there is none.
	poster platform.LoopPoster
	// loop is the id of the loop's goroutine, once known.
	loop atomic.Int64

	// changed is whether the script's step has changed the sketch,
	// and so been remembered for undo.
	changed bool
	// failures collects the errors of a call that returns them.
	failures   []string
	collecting bool
}

var _ api.Host = (*ScriptHost)(nil)

// ErrLoopEnded is what a script's call panics with after the
// application's event loop has ended.
var ErrLoopEnded = errors.New("the application is closing")

// ScriptHost is the application's host for scripts.
func (ss *Application) ScriptHost() *ScriptHost {
	if ss.scriptHost == nil {
		ss.scriptHost = &ScriptHost{ss: ss}
		ss.scriptHost.poster, _ = ss.Platform.(platform.LoopPoster)
	}
	return ss.scriptHost
}

// goid is the id of the calling goroutine, which Go only tells in a
// stack trace: "goroutine 12 [running]:".
func goid() int64 {
	var buf [40]byte
	fields := strings.Fields(string(buf[:runtime.Stack(buf[:], false)]))
	if len(fields) < 2 {
		return -1
	}
	id, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return -1
	}
	return id
}

func (h *ScriptHost) Sketch() *sketch.Sketch { return h.ss.SK }

// Run runs fn on the event loop's goroutine and waits for it; a panic
// in fn is raised in the caller, where the interpreter reports it,
// rather than in the loop. Called on the loop's goroutine, as by what
// fn itself calls, it just runs fn.
func (h *ScriptHost) Run(fn func()) {
	if h.poster == nil || goid() == h.loop.Load() {
		fn()
		return
	}
	type result struct {
		panicked bool
		value    any
	}
	done := make(chan result, 1)
	h.poster.PostToLoop(func() {
		h.loop.Store(goid())
		r := result{panicked: true}
		defer func() {
			if r.panicked {
				r.value = recover()
			}
			done <- r
		}()
		fn()
		r.panicked = false
	})
	select {
	case r := <-done:
		if r.panicked {
			panic(r.value)
		}
	case <-h.poster.LoopDone():
		panic(ErrLoopEnded)
	}
}

// message takes what the application would show in a dialog.
func (h *ScriptHost) message(text string, isError bool) {
	text = strings.TrimSpace(text)
	if isError && h.collecting {
		h.failures = append(h.failures, text)
		return
	}
	if h.Messages != nil {
		kind := "Message"
		if isError {
			kind = "Error"
		}
		fmt.Fprintf(h.Messages, "%s: %s\n", kind, text)
	}
}

// collect runs fn, and returns as an error what it told the user was
// wrong.
func (h *ScriptHost) collect(fn func()) error {
	h.failures, h.collecting = nil, true
	defer func() { h.failures, h.collecting = nil, false }()
	sink := h.ss.messageSink
	h.ss.messageSink = h.message
	defer func() { h.ss.messageSink = sink }()
	fn()
	if len(h.failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(h.failures, "\n"))
}

// BeginEval starts a step of a script: what the application has to
// say goes to the script's output rather than into dialogs, which
// would stop the event loop under the script.
func (h *ScriptHost) BeginEval() {
	h.changed = false
	h.ss.messageSink = h.message
}

// EndEval ends a step of a script: the sketch is regenerated and the
// windows show it.
func (h *ScriptHost) EndEval() {
	ss := h.ss
	h.Regenerate()
	ss.messageSink = nil
	ss.GW.ClearNonexistentSelectionItems()
	ss.GW.EnsureValidActives()
	ss.ScheduleShowTW()
	ss.GW.Invalidate(false)
}

func (h *ScriptHost) Changing() {
	if !h.changed {
		h.changed = true
		h.ss.UndoRemember()
	}
}

// Remembered folds the n steps of undo a change remembered itself, as
// a menu command does, into the script's step.
func (h *ScriptHost) Remembered(n int) {
	if n <= 0 {
		return
	}
	if !h.changed {
		h.changed = true
		n--
	}
	h.ss.SK.UndoDropNewest(n)
	h.ss.UndoEnableMenus()
}

// Rollback takes back what the script's last step changed, for good:
// it can't be done again as what Undo takes back can. It is false if
// the step changed nothing.
func (h *ScriptHost) Rollback() bool {
	ss := h.ss
	if !h.changed || ss.SK.Undo.Cnt <= 0 {
		return false
	}
	h.changed = false
	ss.UndoUndo()
	ss.SK.UndoClearStack(&ss.SK.Redo)
	ss.UndoEnableMenus()
	return true
}

// Regenerate does now the regeneration that the event loop would get
// to after the script.
func (h *ScriptHost) Regenerate() {
	ss := h.ss
	if ss.SK.RegenerationPending() {
		ss.scheduledGenerateAll = false
		ss.SK.Refresh()
		ss.afterGenerate()
	}
}

func (h *ScriptHost) AddGroup(g sketch.Group, seed func(sk *sketch.Sketch, hg core.HGroup)) core.HGroup {
	h.ss.GW.ClearSelection()
	return h.ss.addNewGroupSeeded(g, false, seed)
}

// DeleteGroup is textWindow.ScreenDeleteGroup for any group.
func (h *ScriptHost) DeleteGroup(hg core.HGroup) {
	h.ss.deleteGroup(hg)
}

func (h *ScriptHost) Activate(hg core.HGroup) {
	ss := h.ss
	ss.SK.Active = hg
	ss.groupActivate(ss.SK.Group(hg))
	ss.GW.ClearSuper()
}

func (h *ScriptHost) Undo() bool {
	if h.ss.SK.Undo.Cnt <= 0 {
		return false
	}
	h.ss.UndoUndo()
	// What the script's step did so far is gone with it.
	h.changed = false
	return true
}

func (h *ScriptHost) Redo() bool {
	if h.ss.SK.Redo.Cnt <= 0 {
		return false
	}
	h.ss.UndoRedo()
	h.changed = false
	return true
}

func (h *ScriptHost) MmPerUnit() float64 { return h.ss.MmPerUnit() }

// Folder is the folder of the sketch's file, "" while it has none: what
// a script whose files are guarded may use beside its own folders.
func (h *ScriptHost) Folder() (dir string) {
	h.Run(func() {
		if h.ss.SaveFile != "" {
			dir = filepath.Dir(h.ss.SaveFile)
		}
	})
	return dir
}

// Load opens a file as File > Open does, without asking about the
// sketch that is open; that one stays if the file can't be read.
func (h *ScriptHost) Load(path string) error {
	ss := h.ss
	var lerr error
	// Before the read, for the links it reads; see Application.Load.
	was := ss.SaveFile
	ss.SaveFile = path
	err := h.collect(func() { lerr = ss.LoadFromFile(path) })
	if lerr != nil {
		ss.SaveFile = was
		return lerr
	}
	ss.SaveFile = path
	if !ss.Windowless {
		ss.AddToRecentList(path)
	}
	ss.AfterNewFile()
	ss.SK.Unsaved = false
	h.changed = false
	return err
}

// Save saves the sketch as File > Save As does.
func (h *ScriptHost) Save(path string) error {
	ss := h.ss
	var serr error
	err := h.collect(func() { serr = ss.SaveToFile(path) })
	if serr != nil {
		return serr
	}
	if !ss.Windowless {
		ss.AddToRecentList(path)
		ss.RemoveAutosave()
	}
	ss.SaveFile = path
	ss.SK.Unsaved = false
	ss.UpdateWindowTitles()
	return err
}

// leaveExportMode regenerates the sketch for display after an export
// regenerated it for the file, as the next click does.
func (h *ScriptHost) leaveExportMode() {
	ss := h.ss
	ss.GW.justExported.Draw = false
	if ss.ExportMode {
		ss.LeaveExportMode()
		ss.GenerateAll(sketch.GenerateAll)
	}
}

func (h *ScriptHost) Export(kind api.ExportKind, path string) error {
	ss := h.ss
	defer h.leaveExportMode()
	var xerr error
	err := h.collect(func() {
		switch kind {
		case api.ExportMesh:
			xerr = ss.ExportMeshTo(path)
		case api.ExportView:
			xerr = ss.ExportViewOrWireframeTo(path, false)
		case api.ExportWireframe:
			xerr = ss.ExportViewOrWireframeTo(path, true)
		case api.ExportSurfaces:
			xerr = ss.ExportSurfacesTo(path)
		case api.ExportSection:
			xerr = ss.ExportSectionHere(path)
		default:
			xerr = fmt.Errorf("nothing to export as %d", kind)
		}
	})
	if xerr != nil {
		return xerr
	}
	return err
}

// ShowDrawingSheet shows the drawing sheet in the window, or the model
// again.
func (h *ScriptHost) ShowDrawingSheet(show bool) error { return h.ss.GW.ShowSheet(show) }

// DrawingSheetShown reports whether the window shows the sheet.
func (h *ScriptHost) DrawingSheetShown() bool { return h.ss.GW.sheetShown }

// MeasureDrawing measures the drawing's dimensions.
func (h *ScriptHost) MeasureDrawing(d *sketch.Drawing) ([]float64, []error) {
	rs := h.ss.DrawingDimensions(d)
	values, errs := make([]float64, len(rs)), make([]error, len(rs))
	for i, r := range rs {
		values[i], errs[i] = r.Value, r.Err
	}
	return values, errs
}

// ExportDrawing writes the model's drawing to a vector file.
func (h *ScriptHost) ExportDrawing(path string, d *sketch.Drawing) error {
	defer h.leaveExportMode()
	var derr error
	err := h.collect(func() { derr = h.ss.ExportDrawingTo(path, d) })
	if derr != nil {
		return derr
	}
	return err
}

// Render draws the sketch into a file without disturbing the view on
// screen.
func (h *ScriptHost) Render(path string, width, height int, right, up geom.Vector) error {
	ss := h.ss
	view := ss.GW.view
	defer func() {
		ss.GW.view = view
		ss.GenerateAll(sketch.GenerateAll)
	}()
	var werr error
	err := h.collect(func() {
		if right == (geom.Vector{}) && up == (geom.Vector{}) {
			// A turn of the view that is still going on (Align View
			// to Workplane) is drawn where it ends.
			ss.GW.FinishAnimation()
			view = ss.GW.view
			werr = ss.WriteWindowPicture(path, uint(width), uint(height))
			return
		}
		werr = ss.WriteThumbnail(path, uint(width), uint(height), right, up)
	})
	if werr != nil {
		return werr
	}
	return err
}

func (h *ScriptHost) Selection() []sketch.Selection {
	var out []sketch.Selection
	for _, s := range h.ss.GW.selection {
		out = append(out, sketch.Selection{Entity: s.Entity, Constraint: s.Constraint, Edge: s.Edge, Face: s.Face})
	}
	return out
}

// Hovered is what the mouse is over in the window.
func (h *ScriptHost) Hovered() sketch.Selection {
	hv := h.ss.GW.hover
	return sketch.Selection{Entity: hv.Entity, Constraint: hv.Constraint, Edge: hv.Edge, Face: hv.Face}
}

func (h *ScriptHost) Select(sel []sketch.Selection) {
	gw := h.ss.GW
	gw.ClearSelection()
	for _, s := range sel {
		gw.MakeSelected(&Selection{Entity: s.Entity, Constraint: s.Constraint, Edge: s.Edge, Face: s.Face})
	}
	h.ss.ScheduleShowTW()
	gw.Invalidate(false)
}

// RunCommand runs a menu command, as choosing it does; what the
// command says went wrong is the error.
// Commands lists the main menu's commands for the command search.
func (h *ScriptHost) Commands() []string { return h.ss.CommandLabels() }

func (h *ScriptHost) RunCommand(label string) error {
	var err error
	if cerr := h.collect(func() { err = h.ss.RunCommand(label) }); err == nil {
		err = cerr
	}
	return err
}

// ForgetChanges lets the sketch's unsaved changes go, as "Don't Save"
// does.
func (h *ScriptHost) ForgetChanges() {
	h.ss.SK.Unsaved = false
	h.ss.RemoveAutosave()
	h.ss.UpdateWindowTitles()
}

// SetChordTolerance sets the application's chord tolerance, as
// Configuration does; the next regeneration gives it to the sketch.
func (h *ScriptHost) SetChordTolerance(pct float64) {
	h.ss.ChordTol = pct
	h.ss.applyEngineSettings()
}

// FileName is the file the drawing was opened from or saved to, "" for
// a new one.
func (h *ScriptHost) FileName() string { return h.ss.SaveFile }

// onLoop is the host for a call the window makes on its own thread
// through the scripting interface, as a menu entry does: Run then runs
// at once rather than posting to the loop it is on.
func (h *ScriptHost) onLoop() *ScriptHost {
	h.loop.Store(goid())
	return h
}

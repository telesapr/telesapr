package ui

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// ScriptHost is the application as scripts see it: it lets package
// api work on the sketch that is on screen, with the windows keeping
// up. A script runs on a goroutine of its own, so that the windows
// stay alive while it does; whatever it does to the sketch, it does
// through Run, on the event loop's goroutine.
type ScriptHost struct {
	ss *SolveSpaceUI

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
func (ss *SolveSpaceUI) ScriptHost() *ScriptHost {
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

func (h *ScriptHost) AddGroup(g sketch.Group) core.HGroup {
	h.ss.GW.ClearSelection()
	return h.ss.addNewGroup(g, false)
}

// DeleteGroup is TextWindow.ScreenDeleteGroup for any group.
func (h *ScriptHost) DeleteGroup(hg core.HGroup) {
	ss := h.ss
	sk := ss.SK
	if hg == ss.GW.activeGroupH() {
		if prev := sk.GetGroup(sk.Active).PreviousGroup(sk); prev != nil {
			sk.Active = prev.H
		}
	}
	ss.TW.ClearSuper()
	for i := range sk.Groups {
		if sk.Groups[i].H == hg {
			sk.Groups = append(sk.Groups[:i], sk.Groups[i+1:]...)
			break
		}
	}
	ss.GenerateAll(sketch.GenerateAll)
	ss.GW.ClearSuper()
}

func (h *ScriptHost) Activate(hg core.HGroup) {
	ss := h.ss
	ss.SK.Active = hg
	ss.groupActivate(ss.SK.GetGroup(hg))
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

// Load opens a file as File > Open does, without asking about the
// sketch that is open; that one stays if the file can't be read.
func (h *ScriptHost) Load(path string) error {
	ss := h.ss
	ok := false
	err := h.collect(func() { ok = ss.LoadFromFile(path) })
	if !ok {
		if err == nil {
			err = fmt.Errorf("couldn't load '%s'", path)
		}
		return err
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
	ok := false
	err := h.collect(func() { ok = ss.SaveToFile(path) })
	if !ok {
		if err == nil {
			err = fmt.Errorf("couldn't save '%s'", path)
		}
		return err
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
	ss.JustExportedInfo.Draw = false
	if ss.ExportMode {
		ss.ExportMode = false
		ss.GenerateAll(sketch.GenerateAll)
	}
}

func (h *ScriptHost) Export(kind api.ExportKind, path string) error {
	ss := h.ss
	defer h.leaveExportMode()
	return h.collect(func() {
		switch kind {
		case api.ExportMesh:
			ss.ExportMeshTo(path)
		case api.ExportView:
			ss.ExportViewOrWireframeTo(path, false)
		case api.ExportWireframe:
			ss.ExportViewOrWireframeTo(path, true)
		case api.ExportSurfaces:
			ss.ExportSurfacesTo(path)
		default:
			ss.Error("Nothing to export as %d.", kind)
		}
	})
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
	err := h.collect(func() { werr = ss.WriteThumbnail(path, uint(width), uint(height), right, up) })
	if werr != nil {
		return werr
	}
	return err
}

func (h *ScriptHost) Selection() []sketch.Selection {
	var out []sketch.Selection
	for _, s := range h.ss.GW.selection {
		out = append(out, sketch.Selection{Entity: s.Entity, Constraint: s.Constraint})
	}
	return out
}

func (h *ScriptHost) Select(sel []sketch.Selection) {
	gw := h.ss.GW
	gw.ClearSelection()
	for _, s := range sel {
		gw.MakeSelected(&Selection{Entity: s.Entity, Constraint: s.Constraint})
	}
	h.ss.ScheduleShowTW()
	gw.Invalidate(false)
}

// RunCommand runs a menu command, as choosing it does.
func (h *ScriptHost) RunCommand(label string) error { return h.ss.RunCommand(label) }

// SetChordTolerance sets the application's chord tolerance, as
// Configuration does; the next regeneration gives it to the sketch.
func (h *ScriptHost) SetChordTolerance(pct float64) {
	h.ss.ChordTol = pct
	h.ss.applyEngineSettings()
}

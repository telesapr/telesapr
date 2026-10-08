package ui

import (
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// Init loads the settings, makes a new file and opens the windows (C++
// SolveSpaceUI::Init).
func (ss *Application) Init() {
	ss.LoadSettings()
	// A missing linked file is looked for through the dialogs.
	link.LocateMissingFile = ss.locateMissingFile
	link.ImageCorrupted = func(file string) { ss.Error("The image '%s' is corrupted.", file) }
	// The platform's fonts, for the sketches the application edits and,
	// through the default, for the linked parts read outside them; a
	// platform with none, as the headless one in tests, keeps the
	// default as it is.
	if files := ss.Platform.FontFiles(); len(files) > 0 {
		ss.fonts = ttf.NewFontList(files)
		ttf.DefaultFonts = ss.fonts
	} else {
		ss.fonts = ttf.DefaultFonts
	}

	ss.refreshTimer = ss.Platform.CreateTimer()
	ss.refreshTimer.SetOnTimeout(ss.Refresh)

	ss.autosaveTimer = ss.Platform.CreateTimer()
	ss.autosaveTimer.SetOnTimeout(ss.Autosave)

	// The default styles (colors, line widths, etc.) are also stored in
	// the configuration file, but we will automatically load those as we
	// need them.

	ss.ScheduleAutosave()

	ss.NewFile()
	ss.AfterNewFile()

	if ss.TW.window != nil && ss.GW.window != nil {
		ss.TW.window.ThawPosition(ss.Settings, "TextWindow")
		ss.GW.window.ThawPosition(ss.Settings, "GraphicsWindow")
		ss.TW.window.SetVisible(true)
		ss.GW.window.SetVisible(true)
		ss.GW.window.Focus()
	}

	// The command-line tool has no drawing on screen to offer.
	if ss.AgentAllowed && !ss.Windowless {
		if err := ss.AllowAgent(true); err != nil {
			ss.Error("AI connections can't be allowed: %v", err)
		}
	}
}

// appHooks is what the sketch tells the application and asks of it.
type appHooks struct{ ss *Application }

func (h appHooks) Message(msg string)   { h.ss.Message("%s", msg) }
func (h appHooks) ScheduleGenerateAll() { h.ss.onScheduleGenerateAll() }
func (h appHooks) Generated(requests, constraints, groups int) {
	h.ss.onGenerated(requests, constraints, groups)
}

// RegenerateAfterDelete clears the selection and the editors that could
// refer to the deleted requests, then regenerates.
func (h appHooks) RegenerateAfterDelete() bool {
	h.ss.GW.regenerateAfterDelete()
	return true
}
func (h appHooks) DraggedParams() []core.HParam { return h.ss.MarkDraggedParams() }
func (h appHooks) Cancel() bool                 { return false }

// setSketch makes sk the sketch the application edits, wired to the
// view, the show flags and the event loop.
func (ss *Application) setSketch(sk *sketch.Sketch) {
	ss.SK = sk
	sk.Fonts = ss.fonts
	sk.Settings = ss.Settings
	ss.ViewUnits = sk.Units
	sk.View = &ss.GW.view
	sk.Show = &ss.GW.show
	sk.Hooks = appHooks{ss}
	sk.ReloadAllLinked = func(sk *sketch.Sketch) { link.ReloadAllLinked(sk, ss.SaveFile) }
	ss.pixmaps = nil
	ss.applyEngineSettings()
}

// NewFile starts a new sketch (C++ SolveSpaceUI::NewFile).
func (ss *Application) NewFile() {
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.Units = ss.DefaultUnits
	ss.setSketch(sk)
}

// AfterNewFile generates a new or loaded sketch and resets the windows
// to it (C++ SolveSpaceUI::AfterNewFile).
func (ss *Application) AfterNewFile() {
	// Clear out the traced point, which is no longer valid
	ss.GW.traced.Point = core.HEntity{}
	ss.GW.traced.Path.Points = nil
	// and the naked edges
	ss.GW.nakedEdges.Clear()

	// Quit export mode
	ss.GW.justExported.Draw = false
	ss.GW.centerOfMass.Draw = false
	ss.ExportMode = false

	// GenerateAll() expects the view to be valid, because it uses that
	// to fill in default values for extrusion depths etc.
	ss.GW.view.Offset = geom.Vector{}
	ss.GW.view.ProjRight = geom.Vector{X: 1}
	ss.GW.view.ProjUp = geom.Vector{Y: 1}

	ss.GenerateAll(sketch.GenerateAll)

	ss.GW.Init()
	ss.TW.Init()

	ss.SK.Unsaved = false

	ss.GW.ZoomToFit(false, false)

	// Create all the default styles; they'll get created on the fly
	// anyways, but can't hurt to do it now.
	ss.SK.CreateAllDefaultStyles()

	ss.UpdateWindowTitles()
}

// UpdateWindowTitles names the file in the window title.
func (ss *Application) UpdateWindowTitles() {
	if ss.GW.window == nil || ss.TW.window == nil {
		return
	}
	switch {
	case ss.SaveFile == "":
		ss.GW.window.SetTitle("(new sketch)")
	case !ss.GW.window.SetTitleForFilename(ss.SaveFile):
		if ss.ShowFullFilePath {
			ss.GW.window.SetTitle(ss.SaveFile)
		} else {
			ss.GW.window.SetTitle(filepath.Base(ss.SaveFile))
		}
	}
	if ss.SK != nil && ss.SK.AnyShellsStale() {
		ss.TW.window.SetTitle("Property Browser (making the solid; Esc stops)")
	} else {
		ss.TW.window.SetTitle("Property Browser")
	}
}

// LoadSketchForTest adopts an already loaded sketch as C++ does after
// LoadFromFile, then AfterNewFile.
func (ss *Application) LoadSketchForTest(sk *sketch.Sketch, saveFile string) {
	ss.SaveFile = saveFile
	ss.setSketch(sk)
	ss.AfterNewFile()
}

// MarkDraggedParams lists the params of what the graphics window is
// dragging: its pending points, circle and normal (C++
// SolveSpaceUI::MarkDraggedParams).
func (ss *Application) MarkDraggedParams() []core.HParam {
	sk := ss.SK
	pd := &ss.GW.pending
	var dragged []core.HParam
	for i := -1; i < len(pd.points); i++ {
		hp := pd.point
		if i >= 0 {
			hp = pd.points[i]
		}
		if hp.V == 0 {
			continue
		}
		// The pending point could be one in a group that has not yet
		// been processed, in which case the lookup will fail; but
		// that's not an error.
		if pt := sk.Entity(hp); pt != nil {
			switch pt.Type {
			case core.EntityPointNTrans, core.EntityPointIn3D, core.EntityPointNRotAxisTrans:
				dragged = append(dragged, pt.Param[0], pt.Param[1], pt.Param[2])
			case core.EntityPointIn2D:
				dragged = append(dragged, pt.Param[0], pt.Param[1])
			}
		}
	}
	if pd.circle.V != 0 {
		if circ := sk.Entity(pd.circle); circ != nil {
			if dist := sk.MustEntity(circ.Distance); dist.Type == core.EntityDistance {
				dragged = append(dragged, dist.Param[0])
			}
		}
	}
	if pd.normal.V != 0 {
		if norm := sk.Entity(pd.normal); norm != nil && norm.Type == core.EntityNormalIn3D {
			dragged = append(dragged, norm.Param[0], norm.Param[1], norm.Param[2], norm.Param[3])
		}
	}
	return dragged
}

// onGenerated is the user interface's part of the end of C++
// GenerateAll.
func (ss *Application) onGenerated(requests, constraints, groups int) {
	sk := ss.SK
	// Make sure the point that we're tracing exists.
	if ss.GW.traced.Point.V != 0 && sk.Entity(ss.GW.traced.Point) == nil {
		ss.GW.traced.Point = core.HEntity{}
	}
	// And if we're tracing a point, add its new value to the path
	if ss.GW.traced.Point.V != 0 {
		ss.GW.traced.Path.AddPoint(sk.MustEntity(ss.GW.traced.Point).PointGetNum(sk))
	}

	ss.GW.Invalidate(false)

	// Remove nonexistent selection items, for same reason we waited till
	// the end to put up a dialog box.
	ss.GW.ClearNonexistentSelectionItems()

	if requests > 0 || constraints > 0 || groups > 0 {
		// All sorts of interesting things could have happened; for
		// example, the active group or active workplane could have been
		// deleted. So clear all that out.
		if groups > 0 {
			ss.TW.ClearSuper()
		}
		ss.ScheduleShowTW()
		ss.GW.ClearSuper()
	}
}

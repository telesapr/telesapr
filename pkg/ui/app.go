package ui

import (
	"path/filepath"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/ttf"
)

// Init loads the settings, makes a new file and opens the windows (C++
// SolveSpaceUI::Init).
func (ss *SolveSpaceUI) Init() {
	ss.LoadSettings()
	ttf.SetFontFiles(ss.Platform.GetFontFiles())

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
}

// setSketch makes sk the sketch the application edits, wired to the
// view, the show flags and the event loop.
func (ss *SolveSpaceUI) setSketch(sk *sketch.Sketch) {
	ss.SK = sk
	sk.View = &ss.GW.view
	sk.Show = &ss.GW.show
	sk.OnScheduleGenerateAll = ss.onScheduleGenerateAll
	sk.Message = func(msg string) { ss.Message("%s", msg) }
	sk.ReloadAllLinked = func(sk *sketch.Sketch) { slvs.ReloadAllLinked(sk, ss.SaveFile) }
	ss.pixmaps = nil
	ss.applyEngineSettings()
}

// NewFile starts a new sketch (C++ SolveSpaceUI::NewFile).
func (ss *SolveSpaceUI) NewFile() {
	sk := sketch.NewSketch()
	sk.NewFile()
	ss.setSketch(sk)
}

// AfterNewFile generates a new or loaded sketch and resets the windows
// to it (C++ SolveSpaceUI::AfterNewFile).
func (ss *SolveSpaceUI) AfterNewFile() {
	// Clear out the traced point, which is no longer valid
	ss.Traced.Point = core.HEntity{}
	ss.Traced.Path.Points = nil
	// and the naked edges
	ss.NakedEdges.Clear()

	// Quit export mode
	ss.JustExportedInfo.Draw = false
	ss.CenterOfMass.Draw = false
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
func (ss *SolveSpaceUI) UpdateWindowTitles() {
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
	ss.TW.window.SetTitle("Property Browser")
}

// LoadSketchForTest adopts an already loaded sketch as C++ does after
// LoadFromFile, then AfterNewFile.
func (ss *SolveSpaceUI) LoadSketchForTest(sk *sketch.Sketch, saveFile string) {
	ss.SaveFile = saveFile
	ss.setSketch(sk)
	ss.AfterNewFile()
}

package ui

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// The renderer setting: the command line's choice wins until the box
// is clicked, which saves the setting and gives both windows a new
// renderer.
func TestRendererSetting(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	if !ss.UseOpenGL || !ss.UseGL() {
		t.Fatal("OpenGL is not the default")
	}
	ss.RendererOverride = "software"
	if ss.UseGL() {
		t.Error("the command line's software choice is ignored")
	}
	ss.RendererOverride = "opengl"
	ss.UseOpenGL = false
	if !ss.UseGL() {
		t.Error("the command line's opengl choice is ignored")
	}

	gwCanvas, twCanvas := ss.GW.canvas, ss.TW.canvas
	ss.TW.ScreenChangeUseOpenGL(0, 0)
	if ss.UseOpenGL || ss.UseGL() || ss.RendererOverride != "" {
		t.Errorf("after the click: setting %v, override %q", ss.UseOpenGL, ss.RendererOverride)
	}
	if ss.GW.canvas == gwCanvas || ss.TW.canvas == twCanvas {
		t.Error("the windows kept their renderers")
	}
	ss.SaveSettings()
	if core.ThawBool(p.Settings(), "UseOpenGL", true) {
		t.Error("the setting was not saved")
	}
	ss.LoadSettings()
	if ss.UseOpenGL {
		t.Error("the setting was not loaded")
	}
	ss.SetUseOpenGL(true)
	if !ss.UseGL() {
		t.Error("SetUseOpenGL(true) left software on")
	}
}

package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/export"
)

// exportOptions is what the export writers are told: the export
// settings, the view's scale, and the application's constraint labels
// and messages.
func (ss *Application) exportOptions() export.Options {
	o := export.Options{
		Scale:                 ss.ExportScale,
		Tol:                   ss.exportTolerance(),
		CanvasSizeAuto:        ss.ExportCanvasSizeAuto,
		Margin:                ss.ExportMargin,
		Canvas:                ss.ExportCanvas,
		ShadedTriangles:       ss.ExportShadedTriangles,
		FixColors:             ss.FixExportColors,
		ViewScale:             ss.GW.view.Scale,
		GCode:                 ss.GCode,
		StepOneSurfacePerFace: ss.StepOneSurfacePerFace,
		Ambient:               ss.AmbientIntensity,
		Format:                func(mm float64) string { return ss.MmToString(mm, false) },
		Constraints:           exportConstraints{ss},
		Warn:                  func(msg string) { ss.Message("%s", msg) },
	}
	for i := range o.Lights {
		o.Lights[i] = export.Light{Dir: ss.LightDir[i], Intensity: ss.LightIntensity[i]}
	}
	if ss.ExportBackgroundColor {
		bg := ss.BackgroundColor
		o.Background = &bg
	}
	return o
}

// exportConstraints gives a DXF file the constraints as the window
// shows them.
type exportConstraints struct{ ss *Application }

func (e exportConstraints) Visible(c *core.Constraint) bool { return e.ss.GW.constraintIsVisible(c) }
func (e exportConstraints) Label(c *core.Constraint) string { return e.ss.constraintLabel(c) }

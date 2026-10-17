package ui

import (
	"github.com/telesapr/telesapr/pkg/script/api"
)

// SetMaxSegments sets the application's limit on the pieces a curve is
// cut into for display and meshes, as Configuration does; the engine is
// told at once.
func (h *ScriptHost) SetMaxSegments(n int) {
	h.ss.MaxSegments = n
	h.ss.applyEngineSettings()
}

// SetCheckClosedContour sets the application's check of loops for
// crossing themselves, as Configuration does; the engine is told at
// once.
func (h *ScriptHost) SetCheckClosedContour(on bool) {
	h.ss.CheckClosedContour = on
	h.ss.applyEngineSettings()
}

// ExportOptions are the export settings, as the configuration screen
// holds them.
func (h *ScriptHost) ExportOptions() api.ExportOptions {
	ss := h.ss
	return api.ExportOptions{
		Scale:           ss.ExportScale,
		CutterOffset:    ss.ExportOffset,
		ShadedTriangles: ss.ExportShadedTriangles,
		PwlCurves:       ss.ExportPwlCurves,
		FixColors:       ss.FixExportColors,
		Background:      ss.ExportBackgroundColor,
		CanvasAuto:      ss.ExportCanvasSizeAuto,
		Margin: api.ExportMargin{
			Left: ss.ExportMargin.Left, Right: ss.ExportMargin.Right,
			Bottom: ss.ExportMargin.Bottom, Top: ss.ExportMargin.Top,
		},
		Canvas: api.ExportCanvas{
			Width: ss.ExportCanvas.Width, Height: ss.ExportCanvas.Height,
			Dx: ss.ExportCanvas.Dx, Dy: ss.ExportCanvas.Dy,
		},
		GCode: api.GCodeOptions{
			Depth: ss.GCode.Depth, SafeHeight: ss.GCode.SafeHeight, Passes: ss.GCode.Passes,
			Feed: ss.GCode.Feed, PlungeFeed: ss.GCode.PlungeFeed,
		},
		ChordTol:    ss.ExportChordTol,
		MaxSegments: ss.ExportMaxSegments,
	}
}

// SetExportOptions sets the export settings the way the configuration
// screen does: the lengths it keeps in single precision are rounded to
// it, so the two reach the same file.
func (h *ScriptHost) SetExportOptions(o api.ExportOptions) {
	ss := h.ss
	ss.ExportScale = toF32(o.Scale)
	ss.ExportOffset = toF32(o.CutterOffset)
	ss.ExportShadedTriangles = o.ShadedTriangles
	ss.ExportPwlCurves = o.PwlCurves
	ss.FixExportColors = o.FixColors
	ss.ExportBackgroundColor = o.Background
	ss.ExportCanvasSizeAuto = o.CanvasAuto
	ss.ExportMargin.Left = toF32(o.Margin.Left)
	ss.ExportMargin.Right = toF32(o.Margin.Right)
	ss.ExportMargin.Bottom = toF32(o.Margin.Bottom)
	ss.ExportMargin.Top = toF32(o.Margin.Top)
	ss.ExportCanvas.Width = toF32(o.Canvas.Width)
	ss.ExportCanvas.Height = toF32(o.Canvas.Height)
	ss.ExportCanvas.Dx = toF32(o.Canvas.Dx)
	ss.ExportCanvas.Dy = toF32(o.Canvas.Dy)
	ss.GCode.Depth = toF32(o.GCode.Depth)
	ss.GCode.SafeHeight = toF32(o.GCode.SafeHeight)
	ss.GCode.Passes = o.GCode.Passes
	ss.GCode.Feed = toF32(o.GCode.Feed)
	ss.GCode.PlungeFeed = toF32(o.GCode.PlungeFeed)
	ss.ExportChordTol = o.ChordTol
	ss.ExportMaxSegments = o.MaxSegments
	ss.applyEngineSettings()
}

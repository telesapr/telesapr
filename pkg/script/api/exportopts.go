package api

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
)

// ExportMargin is the space left around the exported geometry when the
// page size is automatic, in mm.
type ExportMargin struct{ Left, Right, Bottom, Top float64 }

// ExportCanvas is a 2d file's page when its size is fixed, in mm, and
// where the geometry starts on it.
type ExportCanvas struct{ Width, Height, Dx, Dy float64 }

// GCodeOptions is what an exported G-code file cuts with, in mm.
type GCodeOptions struct {
	Depth      float64
	SafeHeight float64
	// Passes is how many passes the depth is cut in, 1 to 1000.
	Passes     int
	Feed       float64
	PlungeFeed float64
}

// ExportOptions are the settings that change what an export writes:
// the ones the configuration screen shows. Lengths are in mm. Get them
// with File.ExportOptions, change the fields and give them back to
// File.SetExportOptions; they stay set for the rest of the session and
// apply to every export after, whether the script or the window asks.
type ExportOptions struct {
	// Scale is the export scale factor: the file is 1:Scale (1 is mm,
	// 25.4 is inch). It must be at least 0.001 in size.
	Scale float64
	// CutterOffset is the cutter radius offset, 0 for none; it must not
	// be negative. With an offset curves are exported as lines.
	CutterOffset float64
	// ShadedTriangles exports the shaded solid as 2d triangles.
	ShadedTriangles bool
	// PwlCurves exports curves as piecewise linear.
	PwlCurves bool
	// FixColors fixes white exported lines.
	FixColors bool
	// Background exports the background colour.
	Background bool
	// CanvasAuto sizes the page by Margin around the geometry; if it is
	// false, Canvas is the page.
	CanvasAuto bool
	Margin     ExportMargin
	Canvas     ExportCanvas
	GCode      GCodeOptions
	// ChordTol is the chord tolerance exported curves follow, in mm
	// before scaling by Scale; 0 or more.
	ChordTol float64
	// MaxSegments is the most pieces a curve is cut into when exported,
	// 7 to 1000.
	MaxSegments int
}

// validate says what is wrong with the options, or nil. The rules are
// the configuration screen's, which clamps or refuses the same values.
func (o ExportOptions) validate() error {
	finite := func(name string, v float64) error {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e11 {
			return fmt.Errorf("export option %s is %g, not a reasonable number", name, v)
		}
		return nil
	}
	for _, f := range []struct {
		name string
		v    float64
	}{
		{"Scale", o.Scale}, {"CutterOffset", o.CutterOffset}, {"ChordTol", o.ChordTol},
		{"Margin.Left", o.Margin.Left}, {"Margin.Right", o.Margin.Right},
		{"Margin.Bottom", o.Margin.Bottom}, {"Margin.Top", o.Margin.Top},
		{"Canvas.Width", o.Canvas.Width}, {"Canvas.Height", o.Canvas.Height},
		{"Canvas.Dx", o.Canvas.Dx}, {"Canvas.Dy", o.Canvas.Dy},
		{"GCode.Depth", o.GCode.Depth}, {"GCode.SafeHeight", o.GCode.SafeHeight},
		{"GCode.Feed", o.GCode.Feed}, {"GCode.PlungeFeed", o.GCode.PlungeFeed},
	} {
		if err := finite(f.name, f.v); err != nil {
			return err
		}
	}
	switch {
	case math.Abs(o.Scale) < 0.001:
		return fmt.Errorf("export scale must not be zero, not %g", o.Scale)
	case o.CutterOffset < 0:
		return fmt.Errorf("cutter radius offset must not be negative, not %g", o.CutterOffset)
	case o.ChordTol < 0:
		return fmt.Errorf("export chord tolerance must not be negative, not %g", o.ChordTol)
	case o.MaxSegments < MinSegments || o.MaxSegments > MaxSegmentsLimit:
		return fmt.Errorf("export max segments must be %d to %d, not %d", MinSegments, MaxSegmentsLimit, o.MaxSegments)
	case o.GCode.Passes < 1 || o.GCode.Passes > 1000:
		return fmt.Errorf("the G-code passes must be 1 to 1000, not %d", o.GCode.Passes)
	}
	return nil
}

// The range of a curve's pieces, display or export, as the
// configuration screen allows.
const (
	// MinSegments is the fewest pieces a curve can be limited to.
	MinSegments = 7
	// MaxSegmentsLimit is the most.
	MaxSegmentsLimit = 1000
)

// thawExportOptions reads the export options from a settings store with
// the defaults the application starts with, under the keys it keeps
// them by.
func thawExportOptions(s core.Settings) ExportOptions {
	return ExportOptions{
		Scale:           s.ThawFloat("ExportScale", 1.0),
		CutterOffset:    s.ThawFloat("ExportOffset", 0.0),
		ShadedTriangles: core.ThawBool(s, "ExportShadedTriangles", true),
		PwlCurves:       core.ThawBool(s, "ExportPwlCurves", false),
		FixColors:       core.ThawBool(s, "FixExportColors", true),
		Background:      core.ThawBool(s, "ExportBackgroundColor", false),
		CanvasAuto:      core.ThawBool(s, "ExportCanvasSizeAuto", true),
		Margin: ExportMargin{
			Left:   s.ThawFloat("ExportMargin_Left", 5.0),
			Right:  s.ThawFloat("ExportMargin_Right", 5.0),
			Bottom: s.ThawFloat("ExportMargin_Bottom", 5.0),
			Top:    s.ThawFloat("ExportMargin_Top", 5.0),
		},
		Canvas: ExportCanvas{
			Width:  s.ThawFloat("ExportCanvas_Width", 100.0),
			Height: s.ThawFloat("ExportCanvas_Height", 100.0),
			Dx:     s.ThawFloat("ExportCanvas_Dx", 5.0),
			Dy:     s.ThawFloat("ExportCanvas_Dy", 5.0),
		},
		GCode: GCodeOptions{
			Depth:      s.ThawFloat("GCode_Depth", 10.0),
			SafeHeight: s.ThawFloat("GCode_SafeHeight", 5.0),
			Passes:     int(s.ThawInt("GCode_Passes", 1)),
			Feed:       s.ThawFloat("GCode_Feed", 10.0),
			PlungeFeed: s.ThawFloat("GCode_PlungeFeed", 10.0),
		},
		ChordTol:    s.ThawFloat("ExportChordTolerance", 0.1),
		MaxSegments: int(s.ThawInt("ExportMaxSegments", 64)),
	}
}

// ExportOptions are the settings that change what an export writes, as
// the configuration screen shows them; see ExportOptions. In a script
// run without the application only the mesh export (STL, 3MF) reads
// them, and then only ChordTol, MaxSegments and Scale.
func (m FileMenu) ExportOptions() (o ExportOptions) {
	d := m.d
	d.h.Run(func() { o = d.h.ExportOptions() })
	return o
}

// SetExportOptions replaces the export options, as the configuration
// screen does one at a time, and fails, changing nothing, when one is
// out of range (see ExportOptions). Take the options with
// ExportOptions, change the fields wanted and give them back. In the
// application the values are the ones saved with its settings when it
// closes.
func (m FileMenu) SetExportOptions(o ExportOptions) error {
	if err := o.validate(); err != nil {
		return err
	}
	d := m.d
	d.h.Run(func() { d.h.SetExportOptions(o) })
	return nil
}

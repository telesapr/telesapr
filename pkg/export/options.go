// Package export writes a sketch's curves, mesh and solid in the
// vector, mesh and STEP file formats: what File > Export writes, given
// what to write and the Options of the export.
package export

import (
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Options is what an export is told beside the model.
type Options struct {
	// Scale is the export scale: a length in the file is the model's
	// divided by it.
	Scale float64
	// Tol is the tolerance curves and surfaces are followed at.
	Tol srf.Tolerance
	// CanvasSizeAuto sizes a 2d file's canvas to its contents plus the
	// Margin; otherwise the Canvas is used as given.
	CanvasSizeAuto bool
	Margin         Margin
	Canvas         Canvas
	// Background, if set, is the colour a 2d file is filled with.
	Background *core.RGBA
	// ShadedTriangles writes the mesh's triangles, shaded, into a 2d
	// file that can hold them.
	ShadedTriangles bool
	// FixColors rewrites near-white colours to black, for paper.
	FixColors bool
	// ViewScale is the view's pixels per mm, which style widths and
	// text heights given in pixels are converted with.
	ViewScale float64
	GCode     GCode
	// StepOneSurfacePerFace writes a STEP face of several patches as
	// one surface.
	StepOneSurfacePerFace bool
	// Lights and Ambient light the mesh formats that carry a scene.
	Lights  [2]Light
	Ambient float64
	// Format writes a length in mm as the G-code file's numbers; nil
	// is "%.3f".
	Format func(mm float64) string
	// Constraints is where a DXF file's dimensions come from; nil
	// writes none.
	Constraints Constraints
	// Warn, if set, is told what a file could not hold.
	Warn func(msg string)
	// WidthsMm are line widths in mm, by style, that take the place of
	// the styles' own, which are a view's on the screen: a drawing's
	// sheet is written with the weights drafting gives its lines.
	WidthsMm map[core.HStyle]float64
}

// Margin is the space left around a 2d file's contents, in mm.
type Margin struct{ Left, Right, Bottom, Top float64 }

// Canvas is a 2d file's page, in mm, and where its contents start.
type Canvas struct{ Width, Height, Dx, Dy float64 }

// GCode is what a G-code file cuts with.
type GCode struct {
	Depth      float64
	SafeHeight float64
	Passes     int
	Feed       float64
	PlungeFeed float64
}

// Light is a directional light of a scene.
type Light struct {
	Dir       geom.Vector
	Intensity float64
}

// Constraints is what a DXF file's dimensions ask of the user
// interface: which constraints are shown, and their labels.
type Constraints interface {
	Visible(c *core.Constraint) bool
	Label(c *core.Constraint) string
}

// hstyle is the handle of style v.
func hstyle(v uint32) core.HStyle { return core.HStyle{V: v} }

// warn tells the options' Warn what a file could not hold.
func (w *vectorWriter) warn(msg string) {
	if w.o.Warn != nil {
		w.o.Warn(msg)
	}
}

// format is a length in mm as a G-code number.
func (w *vectorWriter) format(mm float64) string {
	if w.o.Format != nil {
		return w.o.Format(mm)
	}
	return cfmt.Sprintf("%.3f", mm)
}

func (w *vectorWriter) styleExportColor(hs core.HStyle) core.RGBA {
	return w.sk.StyleColor(hs, true, w.o.FixColors)
}

func (w *vectorWriter) styleExportFillColor(hs core.HStyle) core.RGBA {
	return w.sk.StyleFillColor(hs, true, w.o.FixColors)
}

// styleWidthMm is a style's line width in mm.
func (w *vectorWriter) styleWidthMm(hs core.HStyle) float64 {
	if mm, ok := w.o.WidthsMm[hs]; ok {
		return mm
	}
	return w.sk.StyleWidth(hs, w.o.ViewScale) / w.o.ViewScale
}

// styleStippleScaleMm is a style's stipple scale in mm.
func (w *vectorWriter) styleStippleScaleMm(hs core.HStyle) float64 {
	s := w.sk.StyleGet(hs)
	switch s.WidthAs {
	case core.UnitsMM:
		return s.StippleScale
	case core.UnitsPixels:
		return s.StippleScale / w.o.ViewScale
	}
	return 1.0
}

// styleTextHeight is a style's text height in pixels.
func (w *vectorWriter) styleTextHeight(hs core.HStyle) float64 {
	return w.sk.StyleTextHeight(hs, w.o.ViewScale)
}

// hasExtension reports whether path's extension is ext, ignoring case.
func hasExtension(path, ext string) bool {
	return strings.EqualFold(strings.TrimPrefix(filepath.Ext(path), "."), ext)
}

func vec3(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }

package render

import (
	"image"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/res"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// HStroke and HFill address a canvas's interned strokes and fills, so
// geometry can be grouped into indexed draw calls (C++ Canvas::hStroke,
// Canvas::hFill). Zero is no stroke or fill.
type (
	HStroke uint32
	HFill   uint32
)

// Layer describes how geometry occludes other geometry. Within a layer,
// geometry with a higher z-index occludes geometry with a lower one, or
// geometry drawn earlier if z-indexes match (C++ Canvas::Layer).
type Layer int

const (
	LayerNormal    Layer = iota // Occluded by geometry with lower Z coordinate
	LayerOccluded               // Only drawn over geometry with lower Z coordinate
	LayerDepthOnly              // Like Normal, but only affects future occlusion, not color
	LayerBack                   // Always drawn below all other geometry
	LayerFront                  // Always drawn above all other geometry
	LayerLast      = LayerFront
)

// DrawOutlinesAs picks which outlines DrawOutlines draws: emphasized
// ones mark an abrupt change in surface curvature, contour ones the
// boundary of the mesh from this point of view (C++
// Canvas::DrawOutlinesAs).
type DrawOutlinesAs int

const (
	EmphasizedAndContour     DrawOutlinesAs = 0
	EmphasizedWithoutContour DrawOutlinesAs = 1
	ContourOnly              DrawOutlinesAs = 2
)

// Unit says whether a width is in millimetres (scales with the view)
// or pixels (C++ Canvas::Unit).
type Unit int

const (
	UnitMM Unit = iota
	UnitPX
)

// Stroke is a line style (C++ Canvas::Stroke).
type Stroke struct {
	H              HStroke
	Layer          Layer
	ZIndex         int
	Color          core.RgbaColor
	Width          float64
	Unit           Unit
	StipplePattern core.StipplePattern
	StippleScale   float64
}

// Equals compares everything but the handle.
func (s *Stroke) Equals(o *Stroke) bool {
	return s.Layer == o.Layer && s.ZIndex == o.ZIndex && s.Color.Equals(o.Color) &&
		s.Width == o.Width && s.Unit == o.Unit && s.StipplePattern == o.StipplePattern &&
		s.StippleScale == o.StippleScale
}

// WidthMm is the width in model millimetres.
func (s *Stroke) WidthMm(c *Camera) float64 {
	if s.Unit == UnitPX {
		return s.Width / c.Scale
	}
	return s.Width
}

// WidthPx is the width in pixels.
func (s *Stroke) WidthPx(c *Camera) float64 {
	if s.Unit == UnitMM {
		return s.Width * c.Scale
	}
	return s.Width
}

// StippleScaleMm is the stipple scale in model millimetres.
func (s *Stroke) StippleScaleMm(c *Camera) float64 {
	if s.Unit == UnitPX {
		return s.StippleScale / c.Scale
	}
	return s.StippleScale
}

// StippleScalePx is the stipple scale in pixels.
func (s *Stroke) StippleScalePx(c *Camera) float64 {
	if s.Unit == UnitMM {
		return s.StippleScale * c.Scale
	}
	return s.StippleScale
}

// FillPattern is solid, or one of two complementary checkerboards
// (C++ Canvas::FillPattern).
type FillPattern int

const (
	FillSolid FillPattern = iota
	FillCheckeredA
	FillCheckeredB
)

// Fill is a fill style (C++ Canvas::Fill).
type Fill struct {
	H       HFill
	Layer   Layer
	ZIndex  int
	Color   core.RgbaColor
	Pattern FillPattern
	Texture *res.Pixmap
}

// Equals compares everything but the handle.
func (f *Fill) Equals(o *Fill) bool {
	return f.Layer == o.Layer && f.ZIndex == o.ZIndex && f.Color.Equals(o.Color) &&
		f.Pattern == o.Pattern && f.Texture == o.Texture
}

// Canvas is something geometry can be drawn onto (C++ Canvas).
type Canvas interface {
	GetCamera() *Camera

	// GetStroke and GetFill intern a style; Stroke and Fill look one up.
	GetStroke(Stroke) HStroke
	GetFill(Fill) HFill
	Stroke(HStroke) *Stroke
	Fill(HFill) *Fill
	GetBitmapFont() *res.BitmapFont

	DrawLine(a, b geom.Vector, hcs HStroke)
	DrawEdges(el *srf.SEdgeList, hcs HStroke)
	// DrawBeziers returns false if the canvas can't draw curves, so
	// the caller must draw them as edges.
	DrawBeziers(bl *srf.SBezierList, hcs HStroke) bool
	DrawOutlines(ol *srf.SOutlineList, hcs HStroke, drawAs DrawOutlinesAs)
	DrawVectorText(text string, height float64, o, u, v geom.Vector, hcs HStroke)

	DrawQuad(a, b, c, d geom.Vector, hcf HFill)
	DrawPoint(o geom.Vector, hcs HStroke)
	DrawPolygon(p *srf.SPolygon, hcf HFill)
	DrawMesh(m *srf.SMesh, hcfFront, hcfBack HFill)
	DrawFaces(m *srf.SMesh, faces []uint32, hcf HFill)

	DrawPixmap(pm *res.Pixmap, o, u, v geom.Vector, ta, tb geom.Point2d, hcf HFill)
	InvalidatePixmap(pm *res.Pixmap)

	// CreateBatch returns a canvas for view-independent geometry that
	// can be drawn again cheaply, or nil if the backend has none.
	CreateBatch() BatchCanvas
	Clear()
}

// ViewportCanvas draws a view of the scene into a frame (C++
// ViewportCanvas).
type ViewportCanvas interface {
	Canvas
	SetCamera(Camera)
	SetLighting(Lighting)
	// SetClipPlane makes what is drawn from now on keep only the half
	// space h, until it is called with nil or the next StartFrame; for
	// the section view (an extension).
	SetClipPlane(h *srf.HalfSpace)

	StartFrame()
	FlushFrame()
	FinishFrame()
	// ReadFrame is the rendered frame, top row first.
	ReadFrame() *image.RGBA

	GetIdent() (vendor, renderer, version string)
}

// BatchCanvas records view-independent geometry to draw later (C++
// BatchCanvas).
type BatchCanvas interface {
	Canvas
	Finalize()
	Draw()
}

// CanvasBase holds the interned strokes and fills and the bitmap font
// every canvas has (the concrete part of C++ Canvas).
type CanvasBase struct {
	strokes    []Stroke
	fills      []Fill
	bitmapFont *res.BitmapFont
}

// GetStroke interns s, returning the handle of an equal stroke if there
// is one (C++ Canvas::GetStroke).
func (cb *CanvasBase) GetStroke(s Stroke) HStroke {
	for i := range cb.strokes {
		if cb.strokes[i].Equals(&s) {
			return cb.strokes[i].H
		}
	}
	s.H = HStroke(len(cb.strokes) + 1)
	cb.strokes = append(cb.strokes, s)
	return s.H
}

// GetFill interns f (C++ Canvas::GetFill).
func (cb *CanvasBase) GetFill(f Fill) HFill {
	for i := range cb.fills {
		if cb.fills[i].Equals(&f) {
			return cb.fills[i].H
		}
	}
	f.H = HFill(len(cb.fills) + 1)
	cb.fills = append(cb.fills, f)
	return f.H
}

// Stroke is the stroke with handle h.
func (cb *CanvasBase) Stroke(h HStroke) *Stroke {
	if h == 0 || int(h) > len(cb.strokes) {
		panic("render: no such stroke")
	}
	return &cb.strokes[h-1]
}

// Fill is the fill with handle h.
func (cb *CanvasBase) Fill(h HFill) *Fill {
	if h == 0 || int(h) > len(cb.fills) {
		panic("render: no such fill")
	}
	return &cb.fills[h-1]
}

// Strokes lists the interned strokes, in handle order.
func (cb *CanvasBase) Strokes() []Stroke { return cb.strokes }

// Fills lists the interned fills, in handle order.
func (cb *CanvasBase) Fills() []Fill { return cb.fills }

// GetBitmapFont returns the canvas's bitmap font, loading it the first
// time (C++ Canvas::GetBitmapFont).
func (cb *CanvasBase) GetBitmapFont() *res.BitmapFont {
	if cb.bitmapFont.IsEmpty() {
		cb.bitmapFont = res.NewBitmapFont()
	}
	return cb.bitmapFont
}

// SetBitmapFont shares an already loaded font, since loading Unifont
// is slow.
func (cb *CanvasBase) SetBitmapFont(f *res.BitmapFont) { cb.bitmapFont = f }

// Clear forgets the strokes and fills (C++ Canvas::Clear).
func (cb *CanvasBase) Clear() {
	cb.strokes = cb.strokes[:0]
	cb.fills = cb.fills[:0]
}

// CreateBatch returns nil: no batching (C++ Canvas::CreateBatch).
func (cb *CanvasBase) CreateBatch() BatchCanvas { return nil }

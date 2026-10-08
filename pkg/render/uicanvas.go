package render

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cxxutf8"
	"github.com/telesapr/telesapr/pkg/res"
)

// UICanvas draws user interface in integer screen coordinates onto a
// canvas whose camera is the identity in pixels; with Flip, y grows
// down (C++ UICanvas).
type UICanvas struct {
	Canvas Canvas
	Flip   bool
}

func (u *UICanvas) flip(y int) int {
	if u.Flip {
		return int(u.Canvas.Camera().Height) - y
	}
	return y
}

// DrawLine draws a line of the given width in pixels.
func (u *UICanvas) DrawLine(x1, y1, x2, y2 int, color core.RGBA, width, zIndex int) {
	va := geom.Vector{X: float64(x1) + 0.5, Y: float64(u.flip(y1)) + 0.5}
	vb := geom.Vector{X: float64(x2) + 0.5, Y: float64(u.flip(y2)) + 0.5}
	hcs := u.Canvas.InternStroke(Stroke{
		Layer: LayerNormal, ZIndex: zIndex, Width: float64(width), Color: color, Unit: UnitPX,
	})
	u.Canvas.DrawLine(va, vb, hcs)
}

// DrawRect fills and outlines a rectangle; an empty colour skips that
// part.
func (u *UICanvas) DrawRect(l, r, t, b int, fillColor, outlineColor core.RGBA, zIndex int) {
	va := geom.Vector{X: float64(l) + 0.5, Y: float64(u.flip(b)) + 0.5}
	vb := geom.Vector{X: float64(l) + 0.5, Y: float64(u.flip(t)) + 0.5}
	vc := geom.Vector{X: float64(r) + 0.5, Y: float64(u.flip(t)) + 0.5}
	vd := geom.Vector{X: float64(r) + 0.5, Y: float64(u.flip(b)) + 0.5}

	if !fillColor.IsEmpty() {
		hcf := u.Canvas.InternFill(Fill{Layer: LayerNormal, ZIndex: zIndex, Color: fillColor})
		u.Canvas.DrawQuad(va, vb, vc, vd, hcf)
	}
	if !outlineColor.IsEmpty() {
		hcs := u.Canvas.InternStroke(Stroke{
			Layer: LayerNormal, ZIndex: zIndex, Width: 1.0, Color: outlineColor, Unit: UnitPX,
		})
		u.Canvas.DrawLine(va, vb, hcs)
		u.Canvas.DrawLine(vb, vc, hcs)
		u.Canvas.DrawLine(vc, vd, hcs)
		u.Canvas.DrawLine(vd, va, hcs)
	}
}

// DrawPixmap draws pm with its top left corner at x, y.
func (u *UICanvas) DrawPixmap(pm *res.Pixmap, x, y, zIndex int) {
	hcf := u.Canvas.InternFill(Fill{
		Layer: LayerNormal, ZIndex: zIndex, Color: core.NewRGBA(255, 255, 255, 255),
	})
	oy := y
	if u.Flip {
		oy = u.flip(y) - pm.Height
	}
	u.Canvas.DrawPixmap(pm,
		geom.Vector{X: float64(x), Y: float64(oy)},
		geom.Vector{X: float64(pm.Width)},
		geom.Vector{Y: float64(pm.Height)},
		geom.Point2d{U: 0.0, V: 1.0},
		geom.Point2d{U: 1.0, V: 0.0},
		hcf)
}

// DrawBitmapChar draws one bitmap-font character with its bottom left
// corner at x, y (with Flip; its top left without).
func (u *UICanvas) DrawBitmapChar(codepoint rune, x, y int, color core.RGBA, zIndex int) {
	font := u.Canvas.BitmapFont()
	hcf := u.Canvas.InternFill(Fill{Layer: LayerNormal, ZIndex: zIndex, Color: color})

	if codepoint >= 0xe000 && codepoint <= 0xefff {
		// Special character, like a checkbox or a radio button
		x -= 3
	}

	s0, t0, s1, t1, w, h := font.LocateGlyph(codepoint)
	if font.TextureUpdated {
		// LocateGlyph modified the texture, reload it.
		u.Canvas.InvalidatePixmap(font.Texture)
		font.TextureUpdated = false
	}

	u.Canvas.DrawPixmap(font.Texture,
		geom.Vector{X: float64(x), Y: float64(u.flip(y))},
		geom.Vector{X: float64(w)},
		geom.Vector{Y: float64(h)},
		geom.Point2d{U: s0, V: t1},
		geom.Point2d{U: s1, V: t0},
		hcf)
}

// DrawBitmapText draws a string of bitmap-font characters.
func (u *UICanvas) DrawBitmapText(str string, x, y int, color core.RGBA, zIndex int) {
	font := u.Canvas.BitmapFont()
	for _, cp := range cxxutf8.Decode(str) {
		u.DrawBitmapChar(rune(cp), x, y, color, zIndex)
		x += font.Width(rune(cp)) * 8
	}
}

// CameraForPixels is c turned into the identity in pixels, with the
// origin at the bottom left, for drawing user interface over a frame
// (as GraphicsWindow::Paint and TextWindow::Paint set it up).
func CameraForPixels(c Camera) Camera {
	c.LoadIdentity()
	c.Offset.X = -c.Width / 2.0
	c.Offset.Y = -c.Height / 2.0
	return c
}

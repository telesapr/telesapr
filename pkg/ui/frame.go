package ui

import (
	"image"
	"image/png"
	"os"

	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
)

// bitmapFont is the text font the canvases share, since loading
// Unifont is slow.
func (gw *graphicsWindow) bitmapFont() *res.BitmapFont {
	if gw.ss.bitmapFont == nil {
		gw.ss.bitmapFont = res.NewBitmapFont()
	}
	return gw.ss.bitmapFont
}

// shareBitmapFont gives a canvas the shared bitmap font.
func (gw *graphicsWindow) shareBitmapFont(c render.Canvas) {
	if s, ok := c.(interface{ SetBitmapFont(*res.BitmapFont) }); ok {
		s.SetBitmapFont(gw.bitmapFont())
	}
}

// writeFramePng saves a rendered frame as a PNG.
func writeFramePng(frame *image.RGBA, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, frame); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

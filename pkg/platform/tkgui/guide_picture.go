package tkgui

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/png" // the guide's pictures are PNG
	"io"
)

// decodePicture decodes a picture into the RGBA a photo is made of.
func decodePicture(r io.Reader) (*image.RGBA, error) {
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decoding picture: %w", err)
	}
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Rect, src, b.Min, draw.Src)
	return out, nil
}

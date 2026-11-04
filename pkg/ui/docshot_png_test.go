package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// The manual's pictures are committed, so they are written as small as
// a lossless PNG of the same pixels can be made in Go, and the same
// bytes every run: zlib at its best compression, and a palette where a
// picture has no more than 256 colours, which most of the interface's
// flat-coloured pictures have. The palette's order is the order the
// colours are first met, row by row, so it depends on the pixels alone.

// docPaletteMost is the most colours a paletted PNG holds.
const docPaletteMost = 256

// writeDocPng saves a manual's picture as a PNG: paletted where its
// colours fit, otherwise true colour, at the best compression.
func writeDocPng(img *image.RGBA, path string) error {
	data, err := encodeDocPng(img)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// encodeDocPng is writeDocPng's encoding.
func encodeDocPng(img *image.RGBA) ([]byte, error) {
	var out image.Image = img
	if p := docPaletted(img); p != nil {
		out = p
	}
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&b, out); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// docPaletted is img with a palette of its own colours, or nil when it
// has more than docPaletteMost of them.
func docPaletted(img *image.RGBA) *image.Paletted {
	r := img.Bounds()
	index := map[color.RGBA]uint8{}
	var pal color.Palette
	p := image.NewPaletted(r, nil)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.RGBAAt(x, y)
			i, ok := index[c]
			if !ok {
				if len(pal) == docPaletteMost {
					return nil
				}
				i = uint8(len(pal))
				index[c] = i
				pal = append(pal, c)
			}
			p.SetColorIndex(x, y, i)
		}
	}
	p.Palette = pal
	return p
}

func TestDocPngLossless(t *testing.T) {
	t.Parallel()
	few := image.NewRGBA(image.Rect(0, 0, 40, 30))
	many := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for y := range 30 {
		for x := range 40 {
			few.SetRGBA(x, y, color.RGBA{R: uint8(x % 3 * 80), G: uint8(y % 2 * 200), B: 30, A: 255})
			many.SetRGBA(x, y, color.RGBA{R: uint8(x * 6), G: uint8(y * 8), B: uint8(x + y), A: 255})
		}
	}
	for name, img := range map[string]*image.RGBA{"few": few, "many": many} {
		data, err := encodeDocPng(img)
		if err != nil {
			t.Fatal(err)
		}
		again, err := encodeDocPng(img)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, again) {
			t.Errorf("%s: two encodings differ", name)
		}
		back, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if _, paletted := back.(*image.Paletted); paletted != (name == "few") {
			t.Errorf("%s: paletted %v", name, paletted)
		}
		for y := range 30 {
			for x := range 40 {
				r, g, b, a := back.At(x, y).RGBA()
				w := img.RGBAAt(x, y)
				if uint8(r>>8) != w.R || uint8(g>>8) != w.G || uint8(b>>8) != w.B || uint8(a>>8) != w.A {
					t.Fatalf("%s: pixel %d,%d is %v, want %v", name, x, y, back.At(x, y), w)
				}
			}
		}
	}
}

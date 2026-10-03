package res

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"

	"github.com/telesapr/telesapr/pkg/core"
)

// PixmapFormat is the layout of a pixmap's pixels (C++ Pixmap::Format).
type PixmapFormat int

const (
	FormatBGRA PixmapFormat = iota
	FormatRGBA
	FormatBGR
	FormatRGB
	FormatA
)

// Pixmap is a raster image; rows are Stride bytes apart, a multiple of
// four (C++ Pixmap).
type Pixmap struct {
	Format        PixmapFormat
	Width, Height int
	Stride        int
	Data          []byte
}

// NewPixmap makes a blank pixmap (C++ Pixmap::Create).
func NewPixmap(format PixmapFormat, width, height int) *Pixmap {
	pm := &Pixmap{Format: format, Width: width, Height: height}
	// Align to fulfill OpenGL texture requirements.
	stride := width * pm.BytesPerPixel()
	if stride%4 != 0 {
		stride += 4 - stride%4
	}
	pm.Stride = stride
	pm.Data = make([]byte, stride*height)
	return pm
}

// BytesPerPixel is the size of one pixel (C++ GetBytesPerPixel).
func (pm *Pixmap) BytesPerPixel() int {
	switch pm.Format {
	case FormatRGBA, FormatBGRA:
		return 4
	case FormatRGB, FormatBGR:
		return 3
	case FormatA:
		return 1
	}
	panic("res: unexpected pixmap format")
}

// GetPixel is the pixel at x, y (C++ Pixmap::GetPixel).
func (pm *Pixmap) GetPixel(x, y int) core.RgbaColor {
	p := pm.Data[y*pm.Stride+x*pm.BytesPerPixel():]
	switch pm.Format {
	case FormatRGBA:
		return core.NewRgbaColor(p[0], p[1], p[2], p[3])
	case FormatRGB:
		return core.NewRgbaColor(p[0], p[1], p[2], 255)
	case FormatBGRA:
		return core.NewRgbaColor(p[2], p[1], p[0], p[3])
	case FormatBGR:
		return core.NewRgbaColor(p[2], p[1], p[0], 255)
	case FormatA:
		return core.NewRgbaColor(255, 255, 255, p[0])
	}
	panic("res: unexpected pixmap format")
}

// SetPixel sets the pixel at x, y (C++ Pixmap::SetPixel).
func (pm *Pixmap) SetPixel(x, y int, c core.RgbaColor) {
	p := pm.Data[y*pm.Stride+x*pm.BytesPerPixel():]
	switch pm.Format {
	case FormatRGBA:
		p[0], p[1], p[2], p[3] = c.Red, c.Green, c.Blue, c.Alpha
	case FormatRGB:
		p[0], p[1], p[2] = c.Red, c.Green, c.Blue
	case FormatBGRA:
		p[0], p[1], p[2], p[3] = c.Blue, c.Green, c.Red, c.Alpha
	case FormatBGR:
		p[0], p[1], p[2] = c.Blue, c.Green, c.Red
	case FormatA:
		p[0] = c.Alpha
	}
}

// ConvertTo swaps between RGB(A) and BGR(A) (C++ Pixmap::ConvertTo).
func (pm *Pixmap) ConvertTo(format PixmapFormat) {
	ok := (pm.Format == FormatRGBA && format == FormatBGRA) ||
		(pm.Format == FormatBGRA && format == FormatRGBA) ||
		(pm.Format == FormatRGB && format == FormatBGR) ||
		(pm.Format == FormatBGR && format == FormatRGB)
	if !ok {
		panic("res: unexpected target format")
	}
	bpp := pm.BytesPerPixel()
	for j := 0; j < pm.Height; j++ {
		row := pm.Data[j*pm.Stride:]
		for i := 0; i != pm.Width*bpp; i += bpp {
			row[i], row[i+2] = row[i+2], row[i]
		}
	}
	pm.Format = format
}

// Equals compares format, size and pixels (C++ Pixmap::Equals).
func (pm *Pixmap) Equals(o *Pixmap) bool {
	if pm.Format != o.Format || pm.Width != o.Width || pm.Height != o.Height {
		return false
	}
	n := pm.Width * pm.BytesPerPixel()
	for y := 0; y < pm.Height; y++ {
		if !bytes.Equal(pm.Data[y*pm.Stride:y*pm.Stride+n], o.Data[y*o.Stride:y*o.Stride+n]) {
			return false
		}
	}
	return true
}

// Copy is a deep copy (C++ Pixmap::Copy).
func (pm *Pixmap) Copy() *Pixmap {
	c := *pm
	c.Data = append([]byte(nil), pm.Data...)
	return &c
}

// FromPng decodes a PNG as C++ does through libpng with EXPAND,
// GRAY_TO_RGB and SCALE_16: RGBA if the image has an alpha channel or
// transparency, else RGB; with flip, the rows bottom-up (C++
// Pixmap::FromPng).
func FromPng(data []byte, flip bool) (*Pixmap, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return FromImage(img, flip), nil
}

// ReadPng reads a PNG file (C++ Pixmap::ReadPng).
func ReadPng(path string, flip bool) (*Pixmap, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return FromPng(b, flip)
}

// scale16 is libpng's png_do_scale_16_to_8.
func scale16(v uint32) uint8 {
	return uint8((v*255 + 32895) >> 16)
}

// FromImage converts a decoded image the way FromPng lays it out.
func FromImage(img image.Image, flip bool) *Pixmap {
	format := FormatRGB
	switch m := img.(type) {
	case *image.NRGBA, *image.NRGBA64, *image.RGBA, *image.Alpha, *image.Alpha16:
		format = FormatRGBA
	case *image.RGBA64:
		// The PNG decoder returns this for opaque 16-bit RGB.
		format = FormatRGB
	case *image.Paletted:
		for _, c := range m.Palette {
			if _, _, _, a := c.RGBA(); a != 0xffff {
				format = FormatRGBA
				break
			}
		}
	}
	b := img.Bounds()
	pm := NewPixmap(format, b.Dx(), b.Dy())
	for y := 0; y < pm.Height; y++ {
		dy := y
		if flip {
			dy = pm.Height - y - 1
		}
		for x := 0; x < pm.Width; x++ {
			var c core.RgbaColor
			if n, ok := img.(*image.NRGBA); ok {
				i := n.PixOffset(b.Min.X+x, b.Min.Y+y)
				c = core.NewRgbaColor(n.Pix[i], n.Pix[i+1], n.Pix[i+2], n.Pix[i+3])
			} else {
				nc := color.NRGBA64Model.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA64)
				c = core.NewRgbaColor(scale16(uint32(nc.R)), scale16(uint32(nc.G)),
					scale16(uint32(nc.B)), scale16(uint32(nc.A)))
			}
			pm.SetPixel(x, dy, c)
		}
	}
	return pm
}

// ToNRGBA returns the pixmap as a non-premultiplied image.
func (pm *Pixmap) ToNRGBA() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, pm.Width, pm.Height))
	for y := 0; y < pm.Height; y++ {
		for x := 0; x < pm.Width; x++ {
			c := pm.GetPixel(x, y)
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.Red, c.Green, c.Blue, c.Alpha
		}
	}
	return img
}

// WritePng encodes the pixmap as a PNG; with flip, bottom-up (C++
// Pixmap::WritePng). The bytes are Go's encoder's, not libpng's.
func (pm *Pixmap) WritePng(w io.Writer, flip bool) error {
	var img image.Image
	switch pm.Format {
	case FormatA:
		g := image.NewGray(image.Rect(0, 0, pm.Width, pm.Height))
		for y := 0; y < pm.Height; y++ {
			copy(g.Pix[y*g.Stride:y*g.Stride+pm.Width], pm.Data[y*pm.Stride:])
		}
		img = g
	case FormatRGB, FormatBGR:
		r := image.NewRGBA(image.Rect(0, 0, pm.Width, pm.Height))
		for y := 0; y < pm.Height; y++ {
			for x := 0; x < pm.Width; x++ {
				c := pm.GetPixel(x, y)
				i := r.PixOffset(x, y)
				r.Pix[i], r.Pix[i+1], r.Pix[i+2], r.Pix[i+3] = c.Red, c.Green, c.Blue, 255
			}
		}
		img = r
	default:
		img = pm.ToNRGBA()
	}
	if flip {
		img = flipped{img}
	}
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("write png: %w", err)
	}
	return nil
}

// WritePngFile writes the pixmap to a PNG file.
func (pm *Pixmap) WritePngFile(path string, flip bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := pm.WritePng(f, flip); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

type flipped struct{ image.Image }

func (f flipped) At(x, y int) color.Color {
	b := f.Bounds()
	return f.Image.At(x, b.Max.Y-1-(y-b.Min.Y))
}

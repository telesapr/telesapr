package tkgui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"time"
)

// guideAnimation is an animated GIF of the guide, a video of the window
// made a picture the text can show. It keeps the file as it is and
// decodes one frame at a time onto screen: the guide's GIFs are full
// size with up to a few hundred frames, which decoded all at once take
// hundreds of megabytes, where the file is about one.
type guideAnimation struct {
	// head is the file up to its first block: the header, the screen's
	// size and the global colour table, which every frame is decoded
	// with.
	head   []byte
	frames []gifFrame
	// screen is the picture as the frames up to at have drawn it.
	screen *image.RGBA
	// saved is screen before a frame whose disposal restores it.
	saved *image.RGBA
	at    int
}

// gifFrame is one frame of a GIF: its blocks in the file, from its
// graphic control extension to the end of its image data, and what that
// extension says.
type gifFrame struct {
	blocks   []byte
	rect     image.Rectangle
	delay    time.Duration
	disposal byte
}

// errBadGIF is a GIF whose blocks cannot be followed.
var errBadGIF = errors.New("malformed gif")

// newGuideAnimation splits a GIF into its frames and draws the first.
func newGuideAnimation(data []byte) (*guideAnimation, error) {
	if len(data) < 13 || !bytes.HasPrefix(data, []byte("GIF8")) {
		return nil, fmt.Errorf("reading gif header: %w", errBadGIF)
	}
	w, h := le16(data[6:]), le16(data[8:])
	pos := 13
	if flags := data[10]; flags&0x80 != 0 {
		pos += 3 << (flags&7 + 1)
	}
	if pos > len(data) {
		return nil, fmt.Errorf("reading gif colour table: %w", errBadGIF)
	}
	a := &guideAnimation{head: data[:pos], screen: image.NewRGBA(image.Rect(0, 0, w, h))}
	start := -1 // where the current frame's control extension begins
	var delay time.Duration
	var disposal byte
	for {
		if pos >= len(data) {
			return nil, fmt.Errorf("gif ends without its trailer: %w", errBadGIF)
		}
		switch data[pos] {
		case 0x21: // an extension
			if pos+2 > len(data) {
				return nil, fmt.Errorf("reading gif extension: %w", errBadGIF)
			}
			if data[pos+1] == 0xF9 && pos+8 <= len(data) {
				start = pos
				disposal = data[pos+3] >> 2 & 7
				delay = time.Duration(le16(data[pos+4:])) * 10 * time.Millisecond
			}
			end, err := skipSubBlocks(data, pos+2)
			if err != nil {
				return nil, err
			}
			pos = end
		case 0x2C: // an image
			if pos+11 > len(data) {
				return nil, fmt.Errorf("reading gif image descriptor: %w", errBadGIF)
			}
			if start < 0 {
				start = pos
			}
			x, y := le16(data[pos+1:]), le16(data[pos+3:])
			rect := image.Rect(x, y, x+le16(data[pos+5:]), y+le16(data[pos+7:]))
			flags := data[pos+9]
			pos += 10
			if flags&0x80 != 0 {
				pos += 3 << (flags&7 + 1)
			}
			end, err := skipSubBlocks(data, pos+1) // past the LZW code size
			if err != nil {
				return nil, err
			}
			a.frames = append(a.frames, gifFrame{blocks: data[start:end], rect: rect, delay: delay, disposal: disposal})
			pos, start, delay, disposal = end, -1, 0, 0
		case 0x3B: // the trailer
			if len(a.frames) == 0 {
				return nil, fmt.Errorf("gif has no frame: %w", errBadGIF)
			}
			a.at = len(a.frames) - 1
			if err := a.step(); err != nil {
				return nil, err
			}
			return a, nil
		default:
			return nil, fmt.Errorf("gif block 0x%02x: %w", data[pos], errBadGIF)
		}
	}
}

func le16(b []byte) int { return int(b[0]) | int(b[1])<<8 }

// skipSubBlocks returns where the data sub-blocks from pos end, past
// their empty terminator.
func skipSubBlocks(data []byte, pos int) (int, error) {
	for {
		if pos >= len(data) {
			return 0, fmt.Errorf("gif data runs past the file: %w", errBadGIF)
		}
		n := int(data[pos])
		pos += 1 + n
		if n == 0 {
			return pos, nil
		}
	}
}

// delay is how long the frame on screen stays. A GIF that says none is
// shown at ten frames a second, as browsers do.
func (a *guideAnimation) delay() time.Duration {
	if d := a.frames[a.at].delay; d >= 20*time.Millisecond {
		return d
	}
	return 100 * time.Millisecond
}

// step draws the next frame on screen, the first again after the last:
// it disposes of the frame shown as that frame asks, then decodes the
// next on its own, as a GIF of the head and its blocks alone.
func (a *guideAnimation) step() error {
	next := (a.at + 1) % len(a.frames)
	if next == 0 {
		clear(a.screen.Pix)
	} else {
		switch shown := a.frames[a.at]; shown.disposal {
		case gif.DisposalBackground:
			draw.Draw(a.screen, shown.rect, image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			if a.saved != nil {
				copy(a.screen.Pix, a.saved.Pix)
			}
		}
	}
	f := a.frames[next]
	if f.disposal == gif.DisposalPrevious {
		if a.saved == nil {
			a.saved = image.NewRGBA(a.screen.Rect)
		}
		copy(a.saved.Pix, a.screen.Pix)
	}
	one := make([]byte, 0, len(a.head)+len(f.blocks)+1)
	one = append(append(append(one, a.head...), f.blocks...), 0x3B)
	m, err := gif.Decode(bytes.NewReader(one))
	if err != nil {
		return fmt.Errorf("decoding gif frame %d: %w", next, err)
	}
	draw.Draw(a.screen, m.Bounds(), m, m.Bounds().Min, draw.Over)
	a.at = next
	return nil
}

// shrinkInto draws src into dst, a smaller picture of the same shape,
// each pixel of dst the average of the area of src it covers, as
// richtext shrinks a picture to the page's width.
func shrinkInto(dst, src *image.RGBA) {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	dw, dh := dst.Rect.Dx(), dst.Rect.Dy()
	if dw == 0 || dh == 0 || sw == 0 || sh == 0 {
		return
	}
	fx, fy := float64(sw)/float64(dw), float64(sh)/float64(dh)
	for y := range dh {
		y0, y1 := float64(y)*fy, min(float64(y+1)*fy, float64(sh))
		for x := range dw {
			x0, x1 := float64(x)*fx, min(float64(x+1)*fx, float64(sw))
			var acc [4]float64
			var total float64
			for sy := int(y0); float64(sy) < y1; sy++ {
				wy := min(y1, float64(sy+1)) - max(y0, float64(sy))
				row := src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+sy)
				for sx := int(x0); float64(sx) < x1; sx++ {
					wgt := wy * (min(x1, float64(sx+1)) - max(x0, float64(sx)))
					p := src.Pix[row+4*sx : row+4*sx+4]
					for c := range 4 {
						acc[c] += float64(p[c]) * wgt
					}
					total += wgt
				}
			}
			o := dst.PixOffset(dst.Rect.Min.X+x, dst.Rect.Min.Y+y)
			for c := range 4 {
				dst.Pix[o+c] = uint8(min(max(acc[c]/total+0.5, 0), 255))
			}
		}
	}
}

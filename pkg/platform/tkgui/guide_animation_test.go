package tkgui

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"io/fs"
	"path"
	"strings"
	"testing"

	tkimage "github.com/takigo/takigo/image"

	"github.com/telesapr/telesapr/doc/guide"
	"github.com/telesapr/telesapr/pkg/internal/markdown"
)

// testGIF makes a GIF whose frames after the first cover part of the
// screen, each with another disposal, and a transparent hole in one.
func testGIF(t *testing.T) []byte {
	t.Helper()
	frame := func(r image.Rectangle, c uint8) *image.Paletted {
		m := image.NewPaletted(r, palette.Plan9)
		for i := range m.Pix {
			m.Pix[i] = c
		}
		return m
	}
	hole := frame(image.Rect(2, 2, 12, 10), 40)
	hole.Palette = append(color.Palette{}, palette.Plan9[:255]...)
	hole.Palette = append(hole.Palette, color.RGBA{})
	hole.Pix[hole.PixOffset(5, 5)] = 255
	g := &gif.GIF{
		Image: []*image.Paletted{
			frame(image.Rect(0, 0, 16, 12), 200),
			frame(image.Rect(1, 1, 6, 5), 10),
			hole,
			frame(image.Rect(8, 0, 16, 4), 90),
			frame(image.Rect(0, 6, 4, 12), 150),
		},
		Delay:    []int{5, 0, 12, 3, 7},
		Disposal: []byte{gif.DisposalNone, gif.DisposalBackground, gif.DisposalNone, gif.DisposalPrevious, gif.DisposalNone},
		Config:   image.Config{Width: 16, Height: 12},
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestGuideAnimationFramesMatchWholeDecode checks that the frames drawn
// one at a time from the file are those takigo draws decoding it whole,
// and that after the last the first comes again.
func TestGuideAnimationFramesMatchWholeDecode(t *testing.T) {
	t.Parallel()
	data := testGIF(t)
	want, err := tkimage.NewPhotoFramesFromGIF("want", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	a, err := newGuideAnimation(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.frames) != len(want) {
		t.Fatalf("%d frames, want %d", len(a.frames), len(want))
	}
	for i := range 2 * len(want) {
		w := want[i%len(want)]
		if !bytes.Equal(a.screen.Pix, w.Photo.RGBA().Pix) {
			t.Fatalf("step %d: frame %d differs", i, i%len(want))
		}
		if a.frames[a.at].delay != w.Delay {
			t.Errorf("frame %d delay %v, want %v", a.at, a.frames[a.at].delay, w.Delay)
		}
		if err := a.step(); err != nil {
			t.Fatal(err)
		}
	}
}

// TestGuideAnimationDelayDefault checks that a frame which gives no
// delay is shown for a tenth of a second.
func TestGuideAnimationDelayDefault(t *testing.T) {
	t.Parallel()
	a, err := newGuideAnimation(testGIF(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.step(); err != nil { // to frame 1, which says 0
		t.Fatal(err)
	}
	if d := a.delay(); d.Milliseconds() != 100 {
		t.Errorf("delay %v, want 100ms", d)
	}
}

// TestGuideAnimationRefusesBrokenFiles checks that a file cut short or
// not a GIF is an error, not a panic.
func TestGuideAnimationRefusesBrokenFiles(t *testing.T) {
	t.Parallel()
	data := testGIF(t)
	for _, n := range []int{0, 12, 13, 40, len(data) / 2, len(data) - 1} {
		if _, err := newGuideAnimation(data[:n]); err == nil {
			t.Errorf("%d bytes of %d read without an error", n, len(data))
		}
	}
	if _, err := newGuideAnimation([]byte("\x89PNG\r\n\x1a\n0000000000")); !errors.Is(err, errBadGIF) {
		t.Errorf("PNG read as a GIF: %v", err)
	}
}

// TestShrinkIntoAverages checks that shrinking keeps a flat colour and
// averages a pattern to its mean.
func TestShrinkIntoAverages(t *testing.T) {
	t.Parallel()
	src := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := range 6 {
		for x := range 8 {
			v := uint8(0)
			if (x+y)%2 == 0 {
				v = 200
			}
			src.SetRGBA(x, y, color.RGBA{v, 50, v, 255})
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, 4, 3))
	shrinkInto(dst, src)
	for y := range 3 {
		for x := range 4 {
			c := dst.RGBAAt(x, y)
			if c != (color.RGBA{100, 50, 100, 255}) {
				t.Errorf("pixel %d,%d is %v", x, y, c)
			}
		}
	}
}

// TestGuideVideosHaveGIFs checks that every video of the guide's pages
// has the GIF the window plays in its place, and that each plays
// through.
func TestGuideVideosHaveGIFs(t *testing.T) {
	t.Parallel()
	pages, err := fs.Glob(guide.Pages, "*.md")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, page := range pages {
		src, err := fs.ReadFile(guide.Pages, page)
		if err != nil {
			t.Fatal(err)
		}
		g := &guideWindow{pages: guide.Pages}
		for _, b := range g.animated(page, markdown.Parse(string(src))) {
			if b.Kind == markdown.Video {
				t.Errorf("%s: video %s has no GIF", page, b.Src)
			}
			if b.Kind != markdown.Picture || !strings.HasSuffix(b.Src, ".gif") {
				continue
			}
			seen++
			data, err := fs.ReadFile(guide.Pages, path.Join(path.Dir(page), b.Src))
			if err != nil {
				t.Fatal(err)
			}
			a, err := newGuideAnimation(data)
			if err != nil {
				t.Fatalf("%s: %v", b.Src, err)
			}
			for range a.frames {
				if err := a.step(); err != nil {
					t.Fatalf("%s: %v", b.Src, err)
				}
			}
		}
	}
	if seen == 0 {
		t.Error("no video found in the guide")
	}
}

// TestGuideWindowPlaysVideo checks that a page with a video shows its
// GIF in the text and that the picture shown changes as it plays, shrunk
// to the page's width or not.
func TestGuideWindowPlaysVideo(t *testing.T) {
	p := requireGUI(t)
	var name string
	var first []byte
	shown := func() []byte {
		img, _, _, _, ok := p.guide.view.ImageAt(name)
		if !ok {
			return nil
		}
		ph, ok := img.(*tkimage.Photo)
		if !ok {
			return nil
		}
		return ph.RGBA().Pix
	}
	runWaiting(t, p,
		step{
			do: func() { p.ShowGuide(guide.Pages, "getting-started.md", "https://example.org/guide/") },
			done: func() bool {
				for n := range p.guide.anims {
					name = n
				}
				return name != "" && shown() != nil && len(p.guide.playing) == 1
			},
		},
		step{
			do:   func() { first = bytes.Clone(shown()) },
			done: func() bool { return !bytes.Equal(first, shown()) },
		},
		step{
			do:   func() { p.guide.show("fillet.md", true) },
			done: func() bool { return len(p.guide.playing) == 1 && p.guide.playing[p.guide.anims[name]] == nil },
		},
	)
}

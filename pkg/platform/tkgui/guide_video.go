package tkgui

import (
	"io"
	"io/fs"
	"path"
	"strings"

	tkimage "github.com/takigo/takigo/image"

	"github.com/telesapr/telesapr/pkg/internal/markdown"
)

// The guide's videos play in the window as animated GIFs: a page's
// <video> is shown as the GIF beside its first source, videos/NAME.gif
// for videos/NAME.webm, which the text cannot tell from a picture; a
// timer then draws the GIF's frames into the photo the text shows. A
// video with no GIF stays a link to the page on the site.

// animated returns a page's blocks with each video that has a GIF made
// a picture of it.
func (g *guideWindow) animated(page string, blocks []markdown.Block) []markdown.Block {
	out := make([]markdown.Block, len(blocks))
	for i, b := range blocks {
		out[i] = b
		if b.Kind != markdown.Video || b.Src == "" {
			continue
		}
		src := strings.TrimSuffix(b.Src, path.Ext(b.Src)) + ".gif"
		if _, err := fs.Stat(g.pages, path.Clean(path.Join(path.Dir(page), src))); err == nil {
			out[i] = markdown.Block{Kind: markdown.Picture, Src: src}
		}
	}
	return out
}

// animation reads an animated picture, its first frame drawn; nil if
// it cannot be read.
func (g *guideWindow) animation(name string, r io.Reader) *tkimage.Photo {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil
	}
	a, err := newGuideAnimation(data)
	if err != nil {
		return nil
	}
	ph := tkimage.NewPhoto(g.p.newName("guidevideo"), a.screen)
	g.photos[name] = ph
	g.anims[ph.Name()] = a
	return ph
}

// play shows the next frame of the animation whose photo is named name
// when its frame's time is up, and so on until the page changes.
func (g *guideWindow) play(name string) {
	a := g.anims[name]
	g.playing[a] = g.p.app.After(a.delay(), func() {
		delete(g.playing, a)
		if g.frame.IsVisible() {
			if err := a.step(); err != nil {
				return
			}
			g.showFrame(name, a)
		}
		g.play(name)
	})
}

// stopPlaying stops the animations of the page shown.
func (g *guideWindow) stopPlaying() {
	for a, cancel := range g.playing {
		cancel()
		delete(g.playing, a)
	}
}

// showFrame puts the animation's frame in the text. richtext shows the
// photo itself or, where the page is narrower, a copy shrunk to its
// width under the same name, which takes the frame shrunk.
func (g *guideWindow) showFrame(name string, a *guideAnimation) {
	img, _, _, _, ok := g.view.ImageAt(name)
	if !ok {
		return
	}
	ph, ok := img.(*tkimage.Photo)
	if !ok {
		return
	}
	if ph.RGBA() != a.screen {
		shrinkInto(ph.RGBA(), a.screen)
	}
	ph.Invalidate()
	g.view.Display()
}

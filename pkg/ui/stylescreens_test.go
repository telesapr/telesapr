package ui

import (
	"image"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The z-index edit of a style takes a whole number and refuses anything
// else, with a message and no undo step.
func TestStyleZIndexEdit(t *testing.T) {
	ss, _ := newTestApp(t)
	var msgs []string
	ss.messageSink = func(text string, _ bool) { msgs = append(msgs, text) }
	h := ss.SK.NewCustomStyle()
	ss.SK.Unsaved = false
	start := ss.SK.StyleGet(h).ZIndex
	tw := ss.TW

	for _, bad := range []string{"x", "4a", "", "1.5"} {
		tw.edit.meaning, tw.edit.style = editStyleZIndex, h
		if !tw.EditControlDoneForStyles(bad) {
			t.Fatalf("%q: edit not taken as a style's", bad)
		}
		if z := ss.SK.StyleGet(h).ZIndex; z != start || ss.SK.Unsaved {
			t.Errorf("%q: z-index %d, unsaved %v; want it left alone", bad, z, ss.SK.Unsaved)
		}
	}
	if len(msgs) != 4 {
		t.Errorf("%d messages for 4 bad values: %q", len(msgs), msgs)
	}

	tw.edit.meaning, tw.edit.style = editStyleZIndex, h
	tw.EditControlDoneForStyles(" 0 ")
	if z := ss.SK.StyleGet(h).ZIndex; z != 0 || !ss.SK.Unsaved {
		t.Errorf("z-index %d, unsaved %v after 0", z, ss.SK.Unsaved)
	}
}

// A line drawn in a custom style goes back to the style made in its
// place when the style is deleted, on the styles screen or by a script,
// in the next frame: nothing regenerates, so the frame the persistent
// layer cache saved must not come back.
func TestStyleDeleteRedraws(t *testing.T) {
	for _, byScript := range []bool{false, true} {
		ss, _ := newTestApp(t)
		// The line is free; coloured by freedom it would hide its style.
		ss.ColorByFreedom = false
		w := ss.GW.window.(*headless.Window)
		sk := ss.SK
		hg := ss.GW.activeGroupH()
		// Construction, so that the open line is no contour to report.
		hr := sk.AddRequest(sketch.Request{Type: sketch.RequestLineSegment, Construction: true,
			Group: hg, Workplane: hg.Entity(0)})
		sk.GenerateAll()
		sk.MustEntity(hr.Entity(1)).PointForceTo(sk, geom.Vector{X: -40, Y: 30})
		sk.MustEntity(hr.Entity(2)).PointForceTo(sk, geom.Vector{X: 40, Y: 30})
		h := ss.SK.NewCustomStyle()
		sk.StyleGet(h).Color = core.NewRGBA(255, 0, 255, 255)
		sk.Request(hr).Style = h
		ss.GenerateAll(sketch.GenerateAll)

		hits := 0
		ss.GW.persistent.onHit = func() { hits++ }
		// The second frame of the same view is the one the cache saves.
		for range 2 {
			w.Render()
		}
		if n := magentaPixels(w.Frame); n < 100 {
			t.Fatalf("script %t: %d pixels of the style before it is deleted", byScript, n)
		}
		if byScript {
			doc := api.NewDoc(ss.ScriptHost())
			host := ss.ScriptHost()
			host.BeginEval()
			doc.Style("new-custom-style").Delete()
			host.EndEval()
		} else {
			ss.TW.ScreenDeleteStyle(0, h.V)
		}
		w.Render()
		if n := magentaPixels(w.Frame); n != 0 {
			t.Errorf("script %t: %d pixels of the deleted style (cache hits %d)", byScript, n, hits)
		}
	}
}

// magentaPixels counts the pixels of the custom style's colour about
// the line, right of the toolbar, whose constraint icons are magenta.
func magentaPixels(img *image.RGBA) int {
	n := 0
	for y := 135; y < 165; y++ {
		for x := 110; x < img.Bounds().Dx(); x++ {
			p := img.Pix[img.PixOffset(x, y):]
			if p[0] > 100 && p[1] < 60 && p[2] > 100 {
				n++
			}
		}
	}
	return n
}

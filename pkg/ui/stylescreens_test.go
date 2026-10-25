package ui

import (
	"image"
	"math"
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

// With the colouring by freedom on, a free construction line keeps the
// construction colour and a free line assigned a style, as the context
// menu does, its style's colour, width and dashes; a free line in the
// active group's own style is still coloured by its freedom.
func TestStyleColoursOverFreedom(t *testing.T) {
	ss, p := newTestApp(t)
	if !ss.ColorByFreedom {
		t.Fatal("the colouring by freedom is off by default")
	}
	w := ss.GW.window.(*headless.Window)
	sk := ss.SK
	hg := ss.GW.activeGroupH()
	line := func(y float64, construction bool) core.HRequest {
		hr := sk.AddRequest(sketch.Request{Type: sketch.RequestLineSegment, Construction: construction,
			Group: hg, Workplane: hg.Entity(0)})
		sk.GenerateAll()
		sk.MustEntity(hr.Entity(1)).PointForceTo(sk, geom.Vector{X: -40, Y: y})
		sk.MustEntity(hr.Entity(2)).PointForceTo(sk, geom.Vector{X: 40, Y: y})
		return hr
	}
	// Rows 100, 150 and 200 of the 600 pixel frame.
	line(40, true)
	styled := line(30, false)
	line(20, false)
	ss.GenerateAll(sketch.GenerateAll)

	h := ss.CreateCustomStyle(true)
	s := sk.StyleGet(core.HStyle{V: h})
	s.Color = core.NewRGBA(255, 0, 255, 255)
	s.Width = 5
	s.StippleType = core.StippleDash
	ss.GW.MakeSelectedEntity(styled.Entity(0))
	ss.AssignSelectionToStyle(h)
	// The regeneration the assignment schedules.
	p.RunTimers(10)
	w.Render()

	construction := core.NewRGBA(25, 178, 25, 255)
	magenta := core.NewRGBA(255, 0, 255, 255)
	for _, c := range []struct {
		what   string
		row    int
		color  core.RGBA
		min    int
		dashed bool
	}{
		{"the construction line", 100, construction, 300, false},
		{"the styled line", 150, magenta, 200, true},
		{"the line in the group's style", 200, colorFree, 300, false},
	} {
		// A continuous line is broken only where its end points and the
		// axis cross it.
		lit, runs := rowColor(w.Frame, c.row, c.color)
		if lit < c.min || c.dashed != (runs >= 8) {
			t.Errorf("%s: %d pixels of %v in %d runs along its row", c.what, lit, c.color, runs)
		}
	}
	if lit, _ := rowColor(w.Frame, 150, colorFree); lit > 20 {
		t.Errorf("the styled line has %d pixels coloured by freedom", lit)
	}
}

// rowColor counts the pixels near colour c about row y, right of the
// toolbar, and the runs of them along the row. A thin line is drawn at
// a part of its colour, so the colour is matched by its hue.
func rowColor(img *image.RGBA, y int, c core.RGBA) (lit, runs int) {
	on := false
	for x := 110; x < img.Bounds().Dx(); x++ {
		hit := false
		for dy := -3; dy <= 3; dy++ {
			p := img.Pix[img.PixOffset(x, y+dy):]
			if sameHue(p[0], p[1], p[2], c) {
				hit = true
			}
		}
		if hit {
			lit++
			if !on {
				runs++
			}
		}
		on = hit
	}
	return lit, runs
}

// sameHue reports whether a pixel is colour c drawn at some part of its
// strength: each channel the same fraction of c's, give or take.
func sameHue(r, g, b uint8, c core.RGBA) bool {
	if r < 40 && g < 40 && b < 40 {
		return false
	}
	f := float64(max(r, g, b)) / float64(max(c.Red, c.Green, c.Blue))
	near := func(v, want uint8) bool { return math.Abs(float64(v)-f*float64(want)) <= 30 }
	return near(r, c.Red) && near(g, c.Green) && near(b, c.Blue)
}

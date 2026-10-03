package ui

import (
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// benchmarkPaint times repainting a fixture in a 1920x1080 window, the
// view turning a little each frame as when orbiting.
func benchmarkPaint(b *testing.B, fixture string) {
	p := headless.New()
	ss := New(p)
	ss.Init()
	if !loadSlvs(b, ss, filepath.Join("..", "..", "testdata", fixture)) {
		b.Fatal("load failed")
	}
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 1920, 1080
	ss.GW.ZoomToFit(false, false)
	w.Render()
	v := &ss.GW.view
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.ProjRight = v.ProjRight.RotatedAroundAxis(geom.Vector{Y: 1}, 0.01)
		v.ProjUp = v.ProjUp.RotatedAroundAxis(geom.Vector{Y: 1}, 0.01)
		ss.GW.Invalidate(false)
		w.Render()
	}
}

func BenchmarkPaintTranslate(b *testing.B) { benchmarkPaint(b, "group/translate_nd/normal.slvs") }
func BenchmarkPaintText(b *testing.B)      { benchmarkPaint(b, "request/ttf_text/extrude.slvs") }
func BenchmarkPaintBoolean(b *testing.B) {
	benchmarkPaint(b, "group/boolean_tangent_fillet/normal.slvs")
}

// BenchmarkPaintHover times repainting as the mouse sweeps over a model
// with the view still: each frame changes only the hover highlight, so
// the persistent layer comes from the cache.
func BenchmarkPaintHover(b *testing.B) {
	p := headless.New()
	ss := New(p)
	ss.Init()
	if !loadSlvs(b, ss, filepath.Join("..", "..", "testdata", "group/translate_nd/normal.slvs")) {
		b.Fatal("load failed")
	}
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 1920, 1080
	ss.GW.ZoomToFit(false, false)
	w.Render()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := 660 + float64(i%60)*10
		w.Mouse(platform.MouseEvent{Type: platform.MouseMotion, X: x, Y: 540})
		ss.GW.Invalidate(false)
		w.Render()
	}
}

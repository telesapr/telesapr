package ui

import (
	"path/filepath"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/platform/headless"
)

// benchmarkPaint times repainting a fixture in a 1920x1080 window, the
// view turning a little each frame as when orbiting.
func benchmarkPaint(b *testing.B, fixture string) {
	p := headless.New()
	ss := New(p)
	ss.Init()
	if !ss.Load(filepath.Join("..", "..", "testdata", fixture)) {
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

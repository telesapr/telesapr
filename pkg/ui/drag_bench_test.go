package ui

import (
	"testing"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// dragApp is a window showing a zigzag polyline of n segments, each
// horizontal or vertical in turn, which leaves its first point free to
// drag; it returns the window and that point's position on the screen.
func dragApp(b *testing.B, n int) (*Application, *headless.Window, api.Entity, geom.Point2d) {
	b.Helper()
	p := headless.New()
	ss := New(p)
	ss.Init()
	doc := api.NewDoc(ss.ScriptHost())
	pts := make([]api.Vec, n+1)
	for i := range pts {
		pts[i] = api.V(float64(i/2*10+i%2*10), float64((i+1)/2*10), 0)
	}
	var first api.Entity
	doc.Batch(func() {
		segs := doc.Sketch.Polyline(false, pts...)
		for i, s := range segs {
			if i%2 == 0 {
				doc.Constrain.Horizontal(s)
			} else {
				doc.Constrain.Vertical(s)
			}
		}
		first = segs[0].Points()[0]
	})
	doc.Edit.Regen()
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 1920, 1080
	ss.GW.ZoomToFit(false, false)
	w.Render()
	pos := first.Pos()
	return ss, w, first, ss.GW.ProjectPoint(pos)
}

// benchmarkDrag times one step of dragging a point of a sketch of n
// segments with the mouse, as dragshells_test does: the drag itself,
// which solves the sketch and regenerates it, and the frame drawn
// after it, reported apart as drag-ms and paint-ms. The hit test that
// finds the point under the mouse is BenchmarkPaintHover's.
func benchmarkDrag(b *testing.B, n int) {
	ss, w, first, q := dragApp(b, n)
	was := first.Pos()
	gen := ss.SK.Generation
	ss.GW.MouseLeftDown(q.U, q.V, false, false)
	ss.GW.hover.Clear()
	ss.GW.pending.operation = pendingDraggingPoints
	ss.GW.pending.points = []core.HEntity{first.H}
	ss.GW.orig.mouse = q
	var drag, paint time.Duration
	b.ResetTimer()
	for i := range b.N {
		// Back and forth, so that the sketch never drifts off the screen.
		d := float64(i%20) - 10
		t0 := time.Now()
		ss.GW.dragPoints(q.U+d, q.V+d/2, false, false)
		ss.Refresh()
		t1 := time.Now()
		ss.GW.Invalidate(false)
		w.Render()
		t2 := time.Now()
		drag += t1.Sub(t0)
		paint += t2.Sub(t1)
	}
	b.StopTimer()
	if now := first.Pos(); now == was {
		b.Fatal("the point did not move")
	}
	if regens := ss.SK.Generation - gen; regens < uint64(b.N) {
		b.Fatalf("%d regenerations for %d steps", regens, b.N)
	}
	ss.GW.MouseLeftUp(q.U, q.V, false, false)
	b.ReportMetric(float64(drag.Microseconds())/float64(b.N)/1000, "drag-ms/op")
	b.ReportMetric(float64(paint.Microseconds())/float64(b.N)/1000, "paint-ms/op")
}

func BenchmarkDrag20(b *testing.B)  { benchmarkDrag(b, 20) }
func BenchmarkDrag150(b *testing.B) { benchmarkDrag(b, 150) }
func BenchmarkDrag400(b *testing.B) { benchmarkDrag(b, 400) }

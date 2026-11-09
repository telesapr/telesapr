package ui

import (
	"fmt"
	"math"
	"path/filepath"
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
// With lengths, each segment also has its length, so the drag carries
// the polyline whole and every corner is one whose orientation a drag
// keeps (pkg/sketch flip.go).
func dragApp(b *testing.B, n int, lengths bool) (*Application, *headless.Window, api.Entity, geom.Point2d) {
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
			if lengths {
				doc.Constrain.Length(s, 10)
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
func benchmarkDrag(b *testing.B, n int, lengths bool) {
	ss, w, first, q := dragApp(b, n, lengths)
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

func BenchmarkDrag20(b *testing.B)  { benchmarkDrag(b, 20, false) }
func BenchmarkDrag150(b *testing.B) { benchmarkDrag(b, 150, false) }
func BenchmarkDrag400(b *testing.B) { benchmarkDrag(b, 400, false) }

// BenchmarkDragDimensioned150 drags a polyline of 150 segments with
// their lengths, whose orientation the drag checks at every corner.
func BenchmarkDragDimensioned150(b *testing.B) { benchmarkDrag(b, 150, true) }

// mechanismChain is a window showing a chain of n bars 30 x 6 x 3 that
// move in a mechanism, each hinged to the one before and the first to
// a ground bar held, at 1920 x 1080; it returns the window and the
// chain's far end.
func mechanismChain(tb testing.TB, n int) (*Application, *headless.Window, api.Entity) {
	tb.Helper()
	p := headless.New()
	ss := New(p)
	ss.Init()
	doc := api.NewDoc(ss.ScriptHost())
	bar := api.New()
	bar.Sketch.Rect(api.V(0, 0, 0), api.V(30, 6, 0))
	bar.Sketch.Circle(api.V(3, 3, 0), 1.5)
	bar.Sketch.Circle(api.V(27, 3, 0), 1.5)
	bar.Features.Extrude(3, "")
	path := filepath.Join(tb.TempDir(), "bar.tlspr")
	if err := bar.File.Save(path); err != nil {
		tb.Fatal(err)
	}
	// Each bar's upright edges at its origin and at its far end, found
	// where its link put it, before anything turned it.
	uprights := func(g api.Group) (near, far api.Entity) {
		x0, y0 := math.Inf(1), math.Inf(1)
		for _, e := range g.Entities() {
			if e.Kind() == "line" {
				x0, y0 = min(x0, e.Start().Pos().X), min(y0, e.Start().Pos().Y)
			}
		}
		for _, e := range g.Entities() {
			if e.Kind() != "line" {
				continue
			}
			s, f := e.Start().Pos(), e.End().Pos()
			if math.Abs(s.X-f.X) > 1e-9 || math.Abs(s.Y-f.Y) > 1e-9 || math.Abs(s.Y-y0) > 1e-9 || s.Z < f.Z {
				continue
			}
			switch math.Round(s.X - x0) {
			case 0:
				near = e
			case 30:
				far = e
			}
		}
		return near, far
	}
	ground, err := doc.Assembly.Link(path, "ground")
	if err != nil {
		tb.Fatal(err)
	}
	_, prev := uprights(ground)
	var end api.Entity
	for i := range n {
		g, err := doc.Assembly.Link(path, fmt.Sprintf("bar%d", i))
		if err != nil {
			tb.Fatal(err)
		}
		g.AsLink().SetMoves(true)
		near, far := uprights(g)
		doc.Assembly.Hinge(prev, near)
		prev, end = far, far.Start()
	}
	doc.Edit.Regen()
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 1920, 1080
	ss.GW.ZoomToFit(false, false)
	w.Render()
	return ss, w, end
}

// startDrag starts a drag of point he of the window, under the mouse at
// q.
func startDrag(ss *Application, he core.HEntity, q geom.Point2d) {
	ss.GW.MouseLeftDown(q.U, q.V, false, false)
	ss.GW.hover.Clear()
	ss.GW.pending.operation = pendingDraggingPoints
	ss.GW.pending.points = []core.HEntity{he}
	ss.GW.orig.mouse = q
}

// BenchmarkDragMechanism10 times one step of dragging the end of a
// chain of ten bars that move in a mechanism (mechanismChain): the
// pull along the chain's ten freedoms, the solve of the mechanism, its
// ten solids and the frame drawn after, drawn from the parts' own
// meshes moved, as benchmarkDrag reports them.
func BenchmarkDragMechanism10(b *testing.B) {
	ss, w, end := mechanismChain(b, 10)
	was := end.Pos()
	q := ss.GW.ProjectPoint(was)
	startDrag(ss, end.H, q)
	var drag, paint time.Duration
	b.ResetTimer()
	for i := range b.N {
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
	if now := end.Pos(); now == was {
		b.Fatal("the chain's end did not move")
	}
	ss.GW.MouseLeftUp(q.U, q.V, false, false)
	b.ReportMetric(float64(drag.Microseconds())/float64(b.N)/1000, "drag-ms/op")
	b.ReportMetric(float64(paint.Microseconds())/float64(b.N)/1000, "paint-ms/op")
}

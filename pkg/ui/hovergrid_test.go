package ui

import (
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// threadWindow is the triple-start M12 thread, opened in a window of
// 1920 by 1080 and zoomed to fit.
func threadWindow(t testing.TB) *Application {
	ss := New(headless.New())
	ss.Init()
	if !ss.LoadFromFile(filepath.Join("..", "..", "testdata", "thread", "triple_m12.tlspr")) {
		t.Fatal("load failed")
	}
	ss.AfterNewFile()
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 1920, 1080
	ss.GW.ZoomToFit(false, false)
	w.Render()
	return ss
}

// hiddenByScan is hiddenBySolid trying every triangle.
func (gw *GraphicsWindow) hiddenByScan(p geom.Vector) bool {
	g := gw.sk().GetGroup(gw.activeGroupH())
	gw.view.Tangent = gw.ss.CameraTangent()
	towards, reach := gw.towardsViewer(p)
	scale := max(1e-3/gw.view.Scale, 1e-6)
	start := p.Plus(towards.ScaledBy(10 * scale))
	for i := range g.DisplayMesh.Triangles {
		if t, ok := rayHit(start, towards, &g.DisplayMesh.Triangles[i]); ok && t < reach {
			return true
		}
	}
	return false
}

// The grid finds the same hidden points as trying every triangle, at
// every point of every edge of the thread, in two views, orthographic
// and in perspective (roadmap 24.5).
func TestHoverGridSameAsScan(t *testing.T) {
	ss := threadWindow(t)
	gw := ss.GW
	tp := gw.activeTopology()
	if tp == nil {
		t.Fatal("no topology")
	}
	for turn := range 4 {
		if turn == 2 {
			ss.UsePerspectiveProj = true
			gw.view.ProjRight, gw.view.ProjUp = geom.Vector{X: 1}, geom.Vector{Y: 1}
		}
		hidden, n := 0, 0
		for _, e := range tp.Edges {
			for _, p := range e.Points {
				if got, want := gw.hiddenBySolid(p), gw.hiddenByScan(p); got != want {
					t.Fatalf("view %d, %v: grid %v, scan %v", turn, p, got, want)
				} else if got {
					hidden++
				}
				n++
			}
		}
		if hidden == 0 || hidden == n {
			t.Errorf("view %d: %d of %d points hidden", turn, hidden, n)
		}
		gw.view.ProjRight, gw.view.ProjUp = geom.Vector{X: 0.6, Y: 0.8}, geom.Vector{Z: 1}
	}
}

// BenchmarkHoverHidden times the hidden test of a hover on the thread,
// whose display mesh is large: one point, the view still.
func BenchmarkHoverHidden(b *testing.B) {
	ss := threadWindow(b)
	tp := ss.GW.activeTopology()
	p := tp.Edges[len(tp.Edges)/2].Points[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ss.GW.hiddenBySolid(p)
	}
}

func BenchmarkHoverHiddenScan(b *testing.B) {
	ss := threadWindow(b)
	tp := ss.GW.activeTopology()
	p := tp.Edges[len(tp.Edges)/2].Points[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ss.GW.hiddenByScan(p)
	}
}

// An edge of the solid on the side the viewer sees hovers, and one on
// the far side, behind the solid, doesn't: a block seen from above and
// in front, in an orthographic view and in perspective. (The
// orthographic ray once went away from the viewer, and an edge on a
// face facing the viewer counted as hidden behind the solid.)
func TestSolidEdgeHoverFacesViewer(t *testing.T) {
	ss, _ := newTestApp(t)
	scriptConsole(t, ss, `doc.Rect(V(0, 0, 0), V(20, 10, 0))`, `doc.Extrude(10, "block")`)
	gw := ss.GW
	w := gw.window.(*headless.Window)
	w.Width, w.Height = 1200, 900
	for _, perspective := range []bool{false, true} {
		ss.UsePerspectiveProj = perspective
		toward := geom.Vector{X: 0.3, Y: -0.5, Z: 0.8}.WithMagnitude(1)
		gw.view.ProjRight = geom.Vector{Z: 1}.Cross(toward).WithMagnitude(1)
		gw.view.ProjUp = toward.Cross(gw.view.ProjRight).WithMagnitude(1)
		gw.ZoomToFit(false, false)
		w.Render()
		for _, c := range []struct {
			p      geom.Vector
			hidden bool
		}{{geom.Vector{X: 10, Y: 0, Z: 10}, false}, {geom.Vector{X: 20, Y: 5, Z: 10}, false}, {geom.Vector{X: 10, Y: 10, Z: 0}, true}} {
			if got := gw.hiddenBySolid(c.p); got != c.hidden {
				t.Errorf("perspective %v: %v hidden %v, want %v", perspective, c.p, got, c.hidden)
			}
			// The block's lines lie along its edges and win the hover;
			// the edge of the solid is what hovers where there is none.
			if n, ok := gw.hoverSolidEdge(gw.ProjectPoint(c.p)); ok == c.hidden {
				t.Errorf("perspective %v: at %v the edge hovered is %v, %v", perspective, c.p, n, ok)
			}
		}
	}
}

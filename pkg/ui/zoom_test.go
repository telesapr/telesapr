package ui

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform/headless"
)

// zoomApp opens a fixture of testdata/solid in a window of 1200 by 900
// looking at it from a corner.
func zoomApp(t *testing.T, fixture string) *Application {
	t.Helper()
	ss := New(headless.New())
	ss.Init()
	if err := ss.LoadFromFile(filepath.Join("..", "..", "testdata", "solid", fixture)); err != nil {
		t.Fatal(err)
	}
	ss.AfterNewFile()
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 1200, 900
	v := &ss.GW.view
	v.ProjRight = geom.Vector{X: 1, Y: -1}.WithMagnitude(1)
	v.ProjUp = geom.Vector{X: 1, Y: 1, Z: 2}.WithMagnitude(1)
	return ss
}

// meshBounds is where on the screen the active group's solid falls, in
// pixels from the middle of the window.
func meshBounds(t *testing.T, ss *Application) (lo, hi geom.Point2d) {
	t.Helper()
	gw := ss.GW
	g := ss.SK.Group(gw.activeGroupH())
	g.GenerateDisplayItems(ss.SK)
	if len(g.DisplayMesh.Triangles) == 0 {
		t.Fatal("no solid")
	}
	lo, hi = geom.Point2d{U: math.Inf(1), V: math.Inf(1)}, geom.Point2d{U: math.Inf(-1), V: math.Inf(-1)}
	for _, tr := range g.DisplayMesh.Triangles {
		for _, p := range [...]geom.Vector{tr.A, tr.B, tr.C} {
			q := gw.ProjectPoint(p)
			lo.U, lo.V = min(lo.U, q.U), min(lo.V, q.V)
			hi.U, hi.V = max(hi.U, q.U), max(hi.V, q.V)
		}
	}
	return lo, hi
}

// TestZoomToFitSolidBesideToolbar fits a mirrored solid, which reaches
// well past the sketch it was drawn from, in parallel and in perspective
// projection: all of it lies in the part of the window the toolbar
// leaves free, in its middle, and fills it.
func TestZoomToFitSolidBesideToolbar(t *testing.T) {
	t.Parallel()
	for _, persp := range []bool{false, true} {
		ss := zoomApp(t, "mirror-apart.tlspr")
		ss.UsePerspectiveProj = persp
		gw := ss.GW
		gw.MenuView(cmdZoomToFit)

		camera := gw.Camera()
		area := gw.freeArea(&camera)
		tw := float64(toolbarWidth(toolbarColumns(900)))
		if area.Width != 1200-tw || area.Height != 900 || area.Mid.U != tw/2 || area.Mid.V != 0 {
			t.Fatalf("free area %+v, the toolbar %v wide", area, tw)
		}
		lo, hi := meshBounds(t, ss)
		left, right := area.Mid.U-area.Width/2, area.Mid.U+area.Width/2
		if lo.U < left || hi.U > right || lo.V < -450 || hi.V > 450 {
			t.Errorf("perspective %v: the solid spans %v to %v, out of the free area", persp, lo, hi)
		}
		mid := geom.Point2d{U: (lo.U + hi.U) / 2, V: (lo.V + hi.V) / 2}
		if math.Abs(mid.U-area.Mid.U) > 2 || math.Abs(mid.V) > 2 {
			t.Errorf("perspective %v: the solid's middle at %v, the free area's at %v", persp, mid, area.Mid)
		}
		// It fills nine tenths of the area across or up.
		fill := max((hi.U-lo.U)/area.Width, (hi.V-lo.V)/area.Height)
		if math.Abs(fill-0.9) > 0.01 {
			t.Errorf("perspective %v: the solid fills %.3f of the free area", persp, fill)
		}
	}
}

// TestZoomToFitNoToolbar fits to the whole window when the toolbar is
// hidden.
func TestZoomToFitNoToolbar(t *testing.T) {
	t.Parallel()
	ss := zoomApp(t, "mirror-apart.tlspr")
	ss.ShowToolbar = false
	ss.GW.MenuView(cmdZoomToFit)
	lo, hi := meshBounds(t, ss)
	if math.Abs(lo.U+hi.U) > 1e-6 || math.Abs(lo.V+hi.V) > 1e-6 {
		t.Errorf("the solid spans %v to %v, off the window's middle", lo, hi)
	}
}

// TestMenuZoomKeepsMiddle zooms from the menu, with the mouse off in a
// corner: what is in the middle of the free area stays there. The
// scroll wheel zooms about the mouse.
func TestMenuZoomKeepsMiddle(t *testing.T) {
	t.Parallel()
	ss := zoomApp(t, "mirror-apart.tlspr")
	gw := ss.GW
	gw.MenuView(cmdZoomToFit)
	camera := gw.Camera()
	mid := gw.freeArea(&camera).Mid
	at := gw.UnProjectPoint(mid)
	gw.currentMousePosition = geom.Point2d{U: -500, V: 400}
	scale := gw.view.Scale
	for _, cmd := range []Command{cmdZoomIn, cmdZoomIn, cmdZoomOut} {
		gw.MenuView(cmd)
		if q := gw.ProjectPoint(at); math.Abs(q.U-mid.U) > 1e-6 || math.Abs(q.V-mid.V) > 1e-6 {
			t.Errorf("after %v the middle went to %v from %v", cmd, q, mid)
		}
	}
	if r := gw.view.Scale / scale; math.Abs(r-1.2) > 1e-6 {
		t.Errorf("zoomed by %v, not 1.2", r)
	}

	mouse := gw.UnProjectPoint(gw.currentMousePosition)
	gw.MouseScroll(1)
	if q := gw.ProjectPoint(mouse); math.Abs(q.U+500) > 1e-6 || math.Abs(q.V-400) > 1e-6 {
		t.Errorf("the wheel moved what was under the mouse to %v", q)
	}
}

// TestSheetBesideToolbar shows the drawing sheet: its paper lies in the
// part of the window the toolbar leaves free, in its middle; a picture,
// with no toolbar, has it in its own middle.
func TestSheetBesideToolbar(t *testing.T) {
	t.Parallel()
	ss := zoomApp(t, "mirror-apart.tlspr")
	gw := ss.GW
	if err := gw.ShowSheet(true); err != nil {
		t.Fatal(err)
	}
	w, h := ss.drawing().Size()
	camera := gw.Camera()
	for _, inPicture := range []bool{false, true} {
		gw.inPicture = inPicture
		area := gw.freeArea(&camera)
		gw.inPicture = false
		if inPicture != (area.Mid.U == 0) {
			t.Fatalf("in a picture %v, the free area %+v", inPicture, area)
		}
		c := gw.sheetCamera(camera, area)
		lo, _ := c.ProjectPoint4(geom.Vector{})
		hi, _ := c.ProjectPoint4(geom.Vector{X: w, Y: h})
		lo, hi = lo.ScaledBy(c.Scale), hi.ScaledBy(c.Scale)
		if lo.X < area.Mid.U-area.Width/2 || hi.X > area.Mid.U+area.Width/2 || lo.Y < -450 || hi.Y > 450 {
			t.Errorf("in a picture %v, the paper spans %v to %v, out of %+v", inPicture, lo, hi, area)
		}
		if mid := (lo.X + hi.X) / 2; math.Abs(mid-area.Mid.U) > 1e-6 || math.Abs(lo.Y+hi.Y) > 1e-6 {
			t.Errorf("in a picture %v, the paper's middle at %v, %v", inPicture, mid, (lo.Y+hi.Y)/2)
		}
	}
}

// TestWindowPictureOfFreeArea pictures the window after a fit: the
// picture is of the part the toolbar leaves free, so the solid is in its
// middle and fills it as it fills that part.
func TestWindowPictureOfFreeArea(t *testing.T) {
	t.Parallel()
	ss := zoomApp(t, "mirror-apart.tlspr")
	gw := ss.GW
	gw.MenuView(cmdZoomToFit)
	c := gw.windowPictureCamera(800, 600)
	g := ss.SK.Group(gw.activeGroupH())
	lo, hi := geom.Point2d{U: math.Inf(1), V: math.Inf(1)}, geom.Point2d{U: math.Inf(-1), V: math.Inf(-1)}
	for _, tr := range g.DisplayMesh.Triangles {
		for _, p := range [...]geom.Vector{tr.A, tr.B, tr.C} {
			q, _ := c.ProjectPoint4(p)
			q = q.ScaledBy(c.Scale)
			lo.U, lo.V = min(lo.U, q.X), min(lo.V, q.Y)
			hi.U, hi.V = max(hi.U, q.X), max(hi.V, q.Y)
		}
	}
	if math.Abs(lo.U+hi.U) > 1e-6 || math.Abs(lo.V+hi.V) > 1e-6 {
		t.Errorf("the solid spans %v to %v, off the picture's middle", lo, hi)
	}
	if lo.U < -400 || hi.U > 400 || lo.V < -300 || hi.V > 300 {
		t.Errorf("the solid spans %v to %v, out of the picture", lo, hi)
	}
	if fill := (hi.U - lo.U) / 800; math.Abs(fill-0.9) > 0.01 {
		t.Errorf("the solid fills %.3f of the picture across", fill)
	}
}

// TestZoomToFitThread fits a rod threaded M10 from its top rim, made
// through the interface (hover_thread_side), from the side and turned to
// show three sides: the solid, not the circles the lathe of the thread's
// blank sweeps out past it, fills the free area, all of it in view. The
// fill is a little short of nine tenths, as the rod's sketch, drawn at
// its full diameter, stands past the part the thread turned down.
func TestZoomToFitThread(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("makes the thread through the interface, some seconds")
	}
	var r *uiScriptRunner
	runUIScriptWith(t, "hover_thread_side", func(rr *uiScriptRunner) { r = rr })
	gw := r.ss.GW
	for _, view := range []string{"side", "rotated"} {
		if view == "rotated" {
			r.setView(view)
		}
		gw.MenuView(cmdZoomToFit)
		camera := gw.Camera()
		area := gw.freeArea(&camera)
		lo, hi := meshBounds(t, r.ss)
		left, right := area.Mid.U-area.Width/2, area.Mid.U+area.Width/2
		top, bottom := area.Mid.V+area.Height/2, area.Mid.V-area.Height/2
		if lo.U < left || hi.U > right || lo.V < bottom || hi.V > top {
			t.Errorf("%s: the solid spans %v to %v, out of the free area", view, lo, hi)
		}
		fill := max((hi.U-lo.U)/area.Width, (hi.V-lo.V)/area.Height)
		t.Logf("%s: fill %.3f", view, fill)
		if fill < 0.85 || fill > 0.901 {
			t.Errorf("%s: the solid fills %.3f of the free area, want nearly 0.9", view, fill)
		}
	}
}

// TestZoomToFitArc fits a sketch of an arc: all of the arc as drawn,
// its bulge past its ends and centre too, lies in the free area.
func TestZoomToFitArc(t *testing.T) {
	t.Parallel()
	ss := zoomApp(t, filepath.Join("..", "tlspr", "golden", "request", "arc_of_circle", "normal.tlspr"))
	gw := ss.GW
	gw.MenuView(cmdZoomToFit)
	camera := gw.Camera()
	area := gw.freeArea(&camera)
	left, right := area.Mid.U-area.Width/2, area.Mid.U+area.Width/2
	top, bottom := area.Mid.V+area.Height/2, area.Mid.V-area.Height/2
	arcs := 0
	for i := range ss.SK.Entities {
		e := &ss.SK.Entities[i]
		if e.Type != core.EntityArcOfCircle {
			continue
		}
		arcs++
		for _, ed := range gw.getOrGenerateEdges(e).Edges {
			for _, p := range [...]geom.Vector{ed.A, ed.B} {
				q := gw.ProjectPoint(p)
				if q.U < left || q.U > right || q.V < bottom || q.V > top {
					t.Errorf("arc %x drawn through %v, out of the free area", e.H.V, q)
				}
			}
		}
	}
	if arcs == 0 {
		t.Fatal("no arc")
	}
}

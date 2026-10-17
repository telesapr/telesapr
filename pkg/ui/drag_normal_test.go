package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// normalDragApp is a window with a workplane drawn in 3d, sized and
// zoomed as a drag of its normal needs.
func normalDragApp(t *testing.T) (*Application, *api.Doc, api.Entity) {
	t.Helper()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Features.Sketch3D("free")
	wp := doc.Sketch.NewWorkplane(api.V(0, 0, 0), api.V(1, 0, 0), api.V(0, 1, 0))
	w := ss.GW.window.(*headless.Window)
	w.Width, w.Height = 800, 600
	ss.GW.ZoomToFit(false, false)
	w.Render()
	return ss, doc, wp
}

// Dragging the normal of a workplane and giving the orientation the
// drag came to to a script leave the same model: the window reaches
// the sketch by the entry point the script does. The saved files
// differ in the last digit of a number or two, as the script finds the
// quaternion from the vectors the drag's came to, so what is compared
// is the quaternion, to a trillionth.
func TestDragNormalMatchesSetOrientation(t *testing.T) {
	ss, _, wp := normalDragApp(t)
	ss.GW.pending.normal = wp.Normal().H
	ss.GW.pending.operation = pendingDraggingNormal
	ss.GW.orig.mouse.U, ss.GW.orig.mouse.V = 400, 300
	ss.GW.dragNormal(430, 280, false)
	u, v := wp.Orientation()
	if u.Minus(api.V(1, 0, 0)).Magnitude() < 1e-6 {
		t.Fatal("the drag did not turn the normal")
	}

	ss2, _, wp2 := normalDragApp(t)
	wp2.SetOrientation(u, v)
	qa := ss.SK.Entity(wp.Normal().H).NormalGetNum(ss.SK)
	qb := ss2.SK.Entity(wp2.Normal().H).NormalGetNum(ss2.SK)
	for i, d := range []float64{qa.W - qb.W, qa.X - qb.X, qa.Y - qb.Y, qa.Z - qb.Z} {
		if math.Abs(d) > 1e-12 {
			t.Errorf("component %d differs by %g: %v and %v", i, d, qa, qb)
		}
	}
}

// A normal that lies in a workplane stays where it is when it is
// dragged, and the window doesn't fail.
func TestDragFixedNormalStays(t *testing.T) {
	ss, doc, wp := normalDragApp(t)
	doc.Features.SketchIn(wp, "")
	c := doc.Sketch.Circle(api.V(1, 1, 0), 2)
	hn := ss.SK.Entity(c.H).Normal
	before := ss.SK.Entity(hn).NormalGetNum(ss.SK)
	ss.GW.pending.normal = hn
	ss.GW.pending.operation = pendingDraggingNormal
	ss.GW.orig.mouse.U, ss.GW.orig.mouse.V = 400, 300
	ss.GW.dragNormal(430, 280, false)
	if after := ss.SK.Entity(hn).NormalGetNum(ss.SK); after != before {
		t.Errorf("the circle's normal went from %v to %v", before, after)
	}
}

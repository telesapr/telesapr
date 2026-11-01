package render

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// With ClipToView, a line, an edge or a point just past the window's
// edge is not found from inside the window, as nothing of it is seen
// there; a line that crosses the window is found by its part inside.
func TestObjectPickerClipToView(t *testing.T) {
	t.Parallel()
	cam := Camera{Width: 600, Height: 600, Scale: 1, ProjRight: geom.Vector{X: 1}, ProjUp: geom.Vector{Y: 1}}
	// The mouse 10 px in from the right edge, at mid height.
	pick := func(clip bool, draw func(op *ObjectPicker, hcs HStroke)) (bool, float64) {
		op := &ObjectPicker{Cam: cam, SelRadius: 12, Point: geom.Point2d{U: 290}, ClipToView: clip}
		hcs := op.InternStroke(Stroke{Width: 1, Unit: UnitPX})
		hit := op.Pick(func() { draw(op, hcs) })
		return hit, op.MinDistance
	}
	pastEdge := func(op *ObjectPicker, hcs HStroke) {
		op.DrawLine(geom.Vector{X: 300.5, Y: -100}, geom.Vector{X: 300.5, Y: 100}, hcs)
	}
	if hit, _ := pick(false, pastEdge); !hit {
		t.Errorf("unclipped: the line past the edge is not found")
	}
	if hit, d := pick(true, pastEdge); hit {
		t.Errorf("clipped: the line past the edge is found at %g px", d)
	}
	edges := func(op *ObjectPicker, hcs HStroke) {
		var el srf.SEdgeList
		el.AddEdge(geom.Vector{X: 301, Y: -5}, geom.Vector{X: 301, Y: 5}, 0, 0, 0)
		op.DrawEdges(&el, hcs)
	}
	if hit, d := pick(true, edges); hit {
		t.Errorf("clipped: the edge past the edge is found at %g px", d)
	}
	point := func(op *ObjectPicker, hcs HStroke) { op.DrawPoint(geom.Vector{X: 301}, hcs) }
	if hit, d := pick(true, point); hit {
		t.Errorf("clipped: the point past the edge is found at %g px", d)
	}
	// From inside the window to far outside it: the nearest point of
	// the part inside is the same as of the whole.
	crossing := func(op *ObjectPicker, hcs HStroke) {
		op.DrawLine(geom.Vector{X: 280, Y: -1000}, geom.Vector{X: 280, Y: 1000}, hcs)
	}
	if hit, d := pick(true, crossing); !hit || d != 9.5 {
		t.Errorf("clipped: the crossing line is found %v at %g px, want at 9.5", hit, d)
	}
	// Wholly beside the window, above it.
	above := func(op *ObjectPicker, hcs HStroke) {
		op.DrawLine(geom.Vector{X: -1000, Y: 305}, geom.Vector{X: 1000, Y: 305}, hcs)
	}
	if hit, _ := pick(true, above); hit {
		t.Errorf("clipped: the line above the window is found")
	}
}

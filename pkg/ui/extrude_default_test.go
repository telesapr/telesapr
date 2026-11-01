package ui

import (
	"math"
	"testing"
)

// TestExtrudeDepthIgnoresZoom extrudes the same rectangle at three
// zooms and from the view rotated: the new extrusion starts at the
// depth its sketch gives, half its longer side to a round number, along
// the workplane's normal, whatever the view.
func TestExtrudeDepthIgnoresZoom(t *testing.T) {
	t.Parallel()
	var depths []float64
	for _, zoom := range [][]string{
		nil,
		{"menu View|Zoom In", "menu View|Zoom In"},
		{"menu View|Zoom Out", "menu View|Zoom Out", "menu View|Zoom Out"},
		{"view rotated"},
	} {
		r := newUIScriptRunner(t)
		for _, line := range []string{
			"key r", "move 200 200", "down L 200 200", "up L 200 200",
			"move 380 330", "down L 380 330", "up L 380 330", "key esc",
		} {
			r.step(line)
			r.settle()
		}
		for _, line := range append(zoom, "key x shift") {
			r.step(line)
			r.settle()
		}
		sk := r.ss.SK
		g := sk.Group(r.ss.GW.activeGroupH())
		v := g.ExtrusionGetVector(sk)
		if v.X != 0 || v.Y != 0 {
			t.Errorf("%v: extrusion vector %v is off the workplane's normal", zoom, v)
		}
		depths = append(depths, 2*v.Z)
	}
	// The rectangle is 180 by 130 pixels, at the default 5 pixels to
	// the mm 36 by 26 mm: half of 36 is 18, nearest 20.
	for i, d := range depths {
		if math.Abs(d-20) > 1e-9 {
			t.Errorf("extrusion %d is %g deep, want 20", i, d)
		}
	}
}

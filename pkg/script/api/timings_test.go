package api

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A sketch given Timings counts each group's solve, loops and solid
// over a regeneration; one without times nothing.
func TestSketchTimings(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	ex := d.Features.Extrude(5, "")
	sk := d.Host().Sketch()
	sk.GenerateAll()
	tm := &sketch.Timings{}
	sk.Timings = tm
	sk.GenerateAll()
	sk.Timings = nil
	found := false
	for _, g := range tm.Groups() {
		if g.Group == ex.H {
			found = g.Solves == 1 && g.LoopRuns == 1 && g.Solids == 1 && g.Solid > 0
		}
	}
	if !found {
		t.Errorf("the extrusion's figures: %+v", tm.Groups())
	}
	n := len(tm.Groups())
	sk.GenerateAll()
	if len(tm.Groups()) != n {
		t.Error("timed with no Timings")
	}
}

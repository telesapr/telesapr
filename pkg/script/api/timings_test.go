package api

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A sketch given Timings counts each group's solve, loops and solid
// over a regeneration (not asserting a duration: the Windows clock is
// too coarse to time a small solid); one without times nothing.
func TestSketchTimings(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	ex := d.Features.Extrude(5, "")
	sk := d.Host().Sketch()
	// Every solid made again: a regeneration of what is unchanged
	// would take the extrusion's from the cache and time no solid.
	sk.NoSolidCache = true
	sk.GenerateAll()
	tm := &sketch.Timings{}
	sk.Timings = tm
	sk.GenerateAll()
	sk.Timings = nil
	found := false
	for _, g := range tm.Groups() {
		if g.Group == ex.H {
			found = g.Solves == 1 && g.LoopRuns == 1 && g.Solids == 1
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

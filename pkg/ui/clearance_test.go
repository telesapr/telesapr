package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// The clearance and wall check's screen finds what a script does of
// testdata/solid/clearance.tlspr (examples/scripts/clearance): the same
// gaps and the same walls, and draws a line across each.
func TestClearanceScreenMatchesScript(t *testing.T) {
	// Not parallel: the application sets the link hooks and the fonts
	// for the whole process.
	ss, p := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	if err := doc.File.Load("../../testdata/solid/clearance.tlspr"); err != nil {
		t.Fatal(err)
	}
	ss.MenuClearance()
	c := &ss.TW.clearance
	c.wall = 1.5
	ss.checkClearances()
	want := doc.Assembly.Clearances(c.gap)
	if len(want) != 1 || len(c.clearances) != len(want) || len(ss.GW.nakedEdges.Edges) != len(want) {
		t.Fatalf("the screen found %d gaps and drew %d, a script %d", len(c.clearances), len(ss.GW.nakedEdges.Edges), len(want))
	}
	for i, w := range want {
		if got := c.clearances[i]; got.A != w.A.H || got.B != w.B.H || got.Gap != w.Gap || got.PA != w.PA {
			t.Errorf("gap %d: the screen %+v, a script %+v", i, got, w)
		}
	}
	// With the solids within parts, as a script's ClearancesWithin.
	ss.TW.ScreenChangeClearance('s', 0)
	ss.checkClearances()
	within := doc.Assembly.ClearancesWithin(c.gap)
	if !c.bySolid || len(c.clearances) != len(within) {
		t.Fatalf("by solids: the screen found %d gaps, a script %d", len(c.clearances), len(within))
	}
	ss.TW.ScreenChangeClearance('s', 0)
	ss.checkWalls()
	r := doc.Active().Analyze().WallThickness(1.5)
	if c.walls == nil || c.walls.Thinnest != r.Thinnest || len(c.walls.Thin) != len(r.Thin) || c.checked {
		t.Fatalf("walls: the screen %+v, a script %+v", c.walls, r)
	}
	if math.Abs(r.Thinnest.Thickness-1.2) > 1e-6 || len(ss.GW.nakedEdges.Edges) != min(len(r.Thin), maxWallLines)+1 {
		t.Errorf("thinnest %g, %d lines drawn for %d thin spots", r.Thinnest.Thickness, len(ss.GW.nakedEdges.Edges), len(r.Thin))
	}
	// A limit of nothing is refused; the screen keeps what it had.
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	ss.TW.edit.meaning = editClearanceGap
	if !ss.TW.editClearance("0") || c.gap != 1 || len(p.DialogResponses) != 0 {
		t.Errorf("a gap of 0 taken: %g", c.gap)
	}
	ss.TW.edit.meaning = editClearanceGap
	if !ss.TW.editClearance("2") || c.gap != 2 {
		t.Errorf("a gap of 2 not taken: %g", c.gap)
	}
	ss.TW.edit.meaning = editNothing
	if ss.TW.editClearance("2") {
		t.Error("an edit of something else taken")
	}
}

// The wall check of a sketch with no solid says so on the screen.
func TestClearanceScreenNoSolid(t *testing.T) {
	// Not parallel: the application sets the link hooks and the fonts
	// for the whole process.
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	ss.MenuClearance()
	ss.checkWalls()
	if c := &ss.TW.clearance; c.walls != nil || c.err == "" {
		t.Errorf("no solid: walls %+v, error %q", c.walls, c.err)
	}
}

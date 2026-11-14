package api

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// boxCut draws the rectangle x0..x1 by y0..y1 in XY and extrudes it 10
// up, combined as how.
func boxCut(d *Doc, x0, y0, x1, y1 float64, name string, how sketch.CombineAs) Group {
	d.Features.SketchAt(d.Origin(), V(1, 0, 0), V(0, 1, 0), name+"-outline")
	d.Sketch.Rect(V(x0, y0, 0), V(x1, y1, 0))
	g := d.Features.Extrude(10, name)
	g.SetCombine(how)
	return g
}

// TestGroupAfterActiveReadsCurrent changes a group put in before
// others: the regeneration goes only as far as the active group, and
// the groups after it, read, must be what the model now makes, not what
// it made before the change (a tube modelled over MCP on 2026-11-14: a
// cut drawn between two others, then turned from the union it was made
// as into a difference, left the next cut reading the union's solid,
// 971 mm³ too much, with no problem and a sound check).
func TestGroupAfterActiveReadsCurrent(t *testing.T) {
	t.Parallel()
	d := New()
	block := boxCut(d, 0, 0, 20, 20, "block", Union)
	hole := boxCut(d, 2, 2, 6, 6, "hole", Difference)
	d.Activate(block)
	// Half in the block: 80 mm³ of it in, 160 out.
	slot := boxCut(d, 18, 10, 24, 14, "slot", Difference)
	want := func(how string, mm3 float64) {
		t.Helper()
		if v := hole.Analyze().Volume(); math.Abs(v-mm3) > 1e-6 {
			t.Errorf("slot as %s: the hole after it reads %g mm³, want %g", how, v, mm3)
		}
		r := hole.Analyze().Check()
		if !r.Sound() {
			t.Errorf("slot as %s: the hole after it is unsound: %s", how, r.Summary())
		}
		if p := hole.Problem(); p != "" {
			t.Errorf("slot as %s: the hole after it has a problem: %s", how, p)
		}
	}
	want("difference", 4000-160-80)
	slot.SetCombine(Union)
	want("union", 4000-160+160)
	slot.SetCombine(Difference)
	want("difference again", 4000-160-80)
	if d.Active().H != slot.H {
		t.Errorf("reading the hole activated %s", d.Active().Name())
	}
	// The drawing's description says the same of the group after.
	info := d.Description().Groups
	if last := info[len(info)-1]; last.Volume == nil || math.Abs(*last.Volume-(4000-160-80)) > 1e-3 {
		t.Errorf("the description's last group, %s, has volume %v", last.Name, last.Volume)
	}
}

package api

import (
	"math"
	"strings"
	"testing"
)

// TestDescribeLeavesOldGroups changes a group before others: the
// regeneration goes only as far as the active group, so the hole after
// it is left as it was made. Describe and Description mark it "old"
// with nothing of how it came out, rather than make every group of the
// drawing, and a read of the hole itself makes it.
func TestDescribeLeavesOldGroups(t *testing.T) {
	t.Parallel()
	d := New()
	box := func(x0, y0, x1, y1 float64, name string) Group {
		d.Features.SketchAt(d.Origin(), V(1, 0, 0), V(0, 1, 0), name+"-outline")
		d.Sketch.Rect(V(x0, y0, 0), V(x1, y1, 0))
		return d.Features.Extrude(10, name)
	}
	block := box(0, 0, 20, 20, "block")
	hole := box(2, 2, 6, 6, "hole")
	hole.SetCombine(Difference)
	if v := hole.Analyze().Volume(); math.Abs(v-(4000-160)) > 1e-6 {
		t.Fatalf("the hole reads %g mm³, want %d", v, 4000-160)
	}
	d.Activate(block)
	slot := box(18, 10, 24, 14, "slot")
	slot.SetCombine(Difference)
	// A group added before others makes them all at the read after it; a
	// change after that goes only as far as the active group.
	if p := slot.Problem(); p != "" {
		t.Fatalf("the slot's problem is %q", p)
	}
	slot.SetCombine(Union)

	lineOf := func(text, name string) string {
		for line := range strings.SplitSeq(text, "\n") {
			if strings.Contains(line, name+", extrude:") {
				return line
			}
		}
		return ""
	}
	text := d.Describe()
	if line := lineOf(text, "-hole"); !strings.Contains(line, ": old (") || strings.Contains(line, "volume") {
		t.Errorf("the hole's line is %q", line)
	}
	if line := lineOf(text, "-slot"); !strings.Contains(line, "volume 4160 mm³") {
		t.Errorf("the slot's line is %q", line)
	}
	for _, info := range d.Description().Groups {
		if want := strings.Contains(info.Name, "-hole"); info.Old != want {
			t.Errorf("%s is old %v", info.Name, info.Old)
		} else if want && (info.Volume != nil || info.DOF != -1) {
			t.Errorf("the old hole has volume %v and %d DOF", info.Volume, info.DOF)
		}
	}
	if !hole.Old() {
		t.Fatal("Describe made the hole")
	}

	// 80 mm³ of the slot is in the block, 160 out, as much as the hole
	// takes away.
	if v := hole.Analyze().Volume(); math.Abs(v-4000) > 1e-6 {
		t.Errorf("the hole made now reads %g mm³, want 4000", v)
	}
	if hole.Old() {
		t.Error("a read of the hole left it old")
	}
	if line := lineOf(d.Describe(), "-hole"); !strings.Contains(line, "volume 4000 mm³") {
		t.Errorf("the hole's line once made is %q", line)
	}
	if a := d.Active(); a.H != slot.H {
		t.Errorf("the read activated %s", a.Name())
	}
}

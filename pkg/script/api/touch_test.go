package api

import (
	"strings"
	"testing"
)

// TestCheckPartsTouching: a rod lying on a block, touching it along a
// line, or a ball resting on it at a point, unites into a solid that is
// not a manifold there (roadmap K3). The check says so, where it said
// only "naked edges" or "self-intersecting edges"; sunk into the block,
// or standing off it, they are sound and it says nothing of it.
func TestCheckPartsTouching(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		ball    bool    // a ball in place of the rod
		z       float64 // the rod's axis or the ball's centre; radius 3, the block's top at 5
		touches bool
	}{
		{"touching", false, 8, true},
		{"sunk", false, 7, false},
		{"apart", false, 9, false},
		{"ball-touching", true, 8, true},
		{"ball-sunk", true, 7, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.1)
			d.Features.SketchIn(d.Workplane("XY"), "block")
			d.Sketch.Rect(V(0, -10, 0), V(20, 10, 0))
			d.Features.Extrude(5, "")
			var g Group
			if tc.ball {
				d.Features.SketchIn(d.Workplane("ZX"), "ball")
				axis := d.Sketch.Line(V(10, 0, tc.z-4), V(10, 0, tc.z+4))
				axis.SetConstruction(true)
				d.Sketch.Arc(V(10, 0, tc.z), V(10, 0, tc.z-3), V(10, 0, tc.z+3))
				d.Sketch.Line(V(10, 0, tc.z+3), V(10, 0, tc.z-3))
				g = d.Features.Lathe(axis, "")
			} else {
				d.Features.SketchIn(d.Workplane("YZ"), "rod")
				d.Sketch.Circle(V(0, 0, tc.z), 3)
				g = d.Features.Extrude(10, "")
			}
			r := g.Analyze().Check()
			if r.Touches != tc.touches {
				t.Fatalf("Touches %v, want %v: %s", r.Touches, tc.touches, r.Summary())
			}
			if !tc.touches {
				if !r.Sound() {
					t.Errorf("not sound: %s", r.Summary())
				}
				return
			}
			if r.Sound() {
				t.Fatalf("sound, want naked or crossing edges where the rod lies on the block")
			}
			if r.TouchAt.Z < 5-0.1 || r.TouchAt.Z > 5+0.1 {
				t.Errorf("the contact is at %v, want on the block's top, z 5", r.TouchAt)
			}
			if s := r.Summary(); !strings.Contains(s, "touch only at a point or along a line") {
				t.Errorf("the summary doesn't say the parts touch:\n%s", s)
			}
		})
	}
}

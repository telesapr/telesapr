package api

import (
	"strings"
	"testing"
)

// A sketch whose contours are no good as a profile says what is wrong
// and where, and so does a solid made from it, which is then not sound
// (roadmap 23.1).
func TestSketchProblems(t *testing.T) {
	t.Run("crossing", func(t *testing.T) {
		d := New()
		sk := d.Active()
		d.Sketch.Polyline(true, V(0, 0, 0), V(10, 10, 0), V(10, 0, 0), V(0, 10, 0))
		// Nothing is made from it yet.
		if p := sk.Problem(); p != "" {
			t.Errorf("the bow tie alone has problem %q", p)
		}
		ex := d.Features.Extrude(5, "")
		if p := sk.Problem(); !strings.HasPrefix(p, "the sketch crosses itself at (5, 5, 0)") {
			t.Errorf("the extruded bow tie's problem is %q", p)
		}
		p := ex.Problem()
		if !strings.Contains(p, "the sketch g002-sketch-in-plane crosses itself at (5, 5, 0)") {
			t.Errorf("the extrusion's problem is %q", p)
		}
		r := ex.Analyze().Check()
		if r.Sound() || r.Problem != p || !r.Empty() {
			t.Errorf("the extrusion's check: sound %t, problem %q, %s", r.Sound(), r.Problem, r.String())
		}
		if s := r.Summary(); !strings.HasPrefix(s, "The group has no solid: the sketch") {
			t.Errorf("the summary is %q", s)
		}
		if s := d.Describe(); !strings.Contains(s, "problem: the sketch g002-sketch-in-plane crosses itself") {
			t.Errorf("the description says nothing of it:\n%s", s)
		}
		// The sketch's own check is of the solid so far, none, and
		// sound: a sketch adds no solid by design.
		if r := sk.Analyze().Check(); !r.Sound() || r.Problem != "" {
			t.Errorf("the sketch's check: %s, problem %q", r.String(), r.Problem)
		}
	})

	t.Run("open", func(t *testing.T) {
		d := New()
		sk := d.Active()
		d.Sketch.Polyline(false, V(0, 0, 0), V(10, 0, 0), V(10, 10, 0))
		// Open is fine for a sketch no solid is made from: a sweep's
		// path is open.
		if p := sk.Problem(); p != "" {
			t.Errorf("an open sketch on its own has problem %q", p)
		}
		ex := d.Features.Extrude(5, "")
		if p := ex.Problem(); !strings.Contains(p, "is not closed: a contour has an open end at (") {
			t.Errorf("the extrusion's problem is %q", p)
		}
		if p := sk.Problem(); !strings.HasPrefix(p, "the sketch is not closed") {
			t.Errorf("the extruded open sketch's problem is %q", p)
		}
		// Closing it mends both.
		d.Activate(sk)
		d.Sketch.Line(V(10, 10, 0), V(0, 0, 0))
		// Groups after the active one regenerate when it is theirs.
		d.Activate(ex)
		r := ex.Analyze().Check()
		if sk.Problem() != "" || ex.Problem() != "" || !r.Sound() || r.Empty() {
			t.Errorf("closed, problems %q and %q", sk.Problem(), ex.Problem())
		}
	})

	t.Run("zero length", func(t *testing.T) {
		d := New()
		sk := d.Active()
		d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
		d.Sketch.Line(V(20, 20, 0), V(20, 20, 0))
		d.Features.Extrude(5, "")
		if p := sk.Problem(); !strings.HasPrefix(p, "the sketch has a curve of zero length at (20, 20, 0)") {
			t.Errorf("the problem is %q", p)
		}
	})

	t.Run("lathe", func(t *testing.T) {
		d := New()
		axis := d.Sketch.Line(V(0, -20, 0), V(0, 20, 0))
		axis.SetConstruction(true)
		d.Sketch.Polyline(false, V(5, 0, 0), V(10, 0, 0), V(10, 10, 0))
		la := d.Features.Lathe(axis, "")
		if p := la.Problem(); !strings.Contains(p, "is not closed") {
			t.Errorf("the lathe's problem is %q", p)
		}
	})

	t.Run("not flat", func(t *testing.T) {
		d := New()
		sk := d.Features.Sketch3D("path")
		d.Sketch.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(10, 10, 5), V(0, 10, 0))
		// A 3D sketch may leave the plane: nothing is made from it.
		if p := sk.Problem(); p != "" {
			t.Errorf("a 3D sketch's problem is %q", p)
		}
	})
}

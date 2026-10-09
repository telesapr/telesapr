package api

import "testing"

// A rectangle copied and pasted moved, and turned, lands where asked
// with its constraints; each paste is one undo step.
func TestCopyPaste(t *testing.T) {
	t.Parallel()
	d := New()
	step := d.Host().(*SketchHost).BeginEval
	d.SketchIn(d.Workplane("XY"), "")
	sides := d.Rect(V(0, 0, 0), V(40, 20, 0))
	before := len(d.Active().Constraints())
	d.Copy(sides...)

	step()
	moved := d.Paste(V(100, 0, 0), 0, 1)
	if len(moved) != 4 {
		t.Fatalf("pasted %d entities, want 4", len(moved))
	}
	if got := len(d.Active().Constraints()); got != 2*before {
		t.Errorf("%d constraints after the paste, want %d", got, 2*before)
	}
	near := func(p, want Vec) bool { return p.DistanceTo(want) < 1e-6 }
	found := false
	for _, e := range moved {
		found = found || near(e.Start().Pos(), V(100, 0, 0)) || near(e.End().Pos(), V(100, 0, 0))
	}
	if !found {
		t.Error("no pasted line starts or ends at (100, 0, 0)")
	}

	step()
	turned := d.Paste(V(0, 0, 0), 90, 1)
	found = false
	for _, e := range turned {
		found = found || near(e.Start().Pos(), V(-20, 40, 0)) || near(e.End().Pos(), V(-20, 40, 0))
	}
	if !found {
		t.Error("the rectangle turned a quarter about the origin has no corner at (-20, 40, 0)")
	}

	step()
	if !d.Undo() || len(d.Active().Constraints()) != 2*before {
		t.Errorf("undo of the turned paste left %d constraints", len(d.Active().Constraints()))
	}

	failsWith(t, "scale of 0", func() { d.Paste(V(0, 0, 0), 0, 0) })
	d.Free3D()
	failsWith(t, "in a workplane", func() { d.Copy(sides...) })
}

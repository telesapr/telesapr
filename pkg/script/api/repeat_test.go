package api

import (
	"math"
	"testing"
)

// Translate steps each copy by delta and Rotate turns each by the angle,
// as their doc comments say: a cube 2 a side repeated three times by 5
// along x reaches x = 12, and a block turned twice by 90 about z reaches
// up y as far as it reached along x. Both stepped twice as far, and
// Rotate four times, before the parameters were stored as the copies
// read them (half a step, a quarter turn).
func TestStepAndRepeatSpacing(t *testing.T) {
	t.Parallel()
	d := New()
	d.Rect(V(0, 0, 0), V(2, 2, 0))
	d.Extrude(2, "")
	tr := d.Translate(3, V(5, 0, 0), "")
	if _, hi := tr.Bounds(); !near(hi.X, 12) {
		t.Errorf("translate reaches x %g, want 12", hi.X)
	}
	if v := tr.Volume(); math.Abs(v-24) > 1e-6 {
		t.Errorf("translate volume %g, want 24", v)
	}

	d = New()
	d.Rect(V(4, -1, 0), V(6, 1, 0))
	d.Extrude(2, "")
	rot := d.Rotate(2, d.Origin(), d.Workplane("XY").Normal(), 90, "")
	if _, hi := rot.Bounds(); !near(hi.Y, 6) || !near(hi.X, 6) {
		t.Errorf("rotate reaches %v, want x and y 6", hi)
	}
	if v := rot.Volume(); math.Abs(v-16) > 1e-6 {
		t.Errorf("rotate volume %g, want 16", v)
	}
}

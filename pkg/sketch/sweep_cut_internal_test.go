package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestCutStepsNotAtCorner: two arcs alike about a corner put a cut at half
// their length exactly on the corner, which would leave a piece of no
// length and a stretch starting at the corner; there is no place there,
// and the cuts beside it are inside the arcs.
func TestCutStepsNotAtCorner(t *testing.T) {
	t.Parallel()
	y := geom.Vector{Y: 1}
	arc := srf.SweepStep{Arc: true, Center: geom.Vector{X: 15}, Axis: y, Angle: math.Pi / 3}
	corner := srf.SweepStep{Corner: true, Axis: y, Angle: math.Pi / 2}
	steps := []srf.SweepStep{arc, corner, arc}
	if first, second, ok := cutSteps(steps, 2, 0.5); ok {
		t.Fatalf("cut at the corner: %d and %d steps", len(first), len(second))
	}
	for _, f := range []float64{0.35, 0.65} {
		first, second, ok := cutSteps(steps, 2, f)
		if !ok {
			t.Fatalf("f %v: no cut", f)
		}
		for _, part := range [][]srf.SweepStep{first, second} {
			if part[0].Corner || part[len(part)-1].Corner {
				t.Errorf("f %v: a stretch ends at the corner", f)
			}
			for _, st := range part {
				if !st.Corner && stepLength(st, 2) <= 0 {
					t.Errorf("f %v: a piece of no length", f)
				}
			}
		}
	}
}

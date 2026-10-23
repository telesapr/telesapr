package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/thread"
)

// The ridges' end is moved off the run-out's circle: no flank of the
// last turn of any start meets the circle within a tenth of a pitch, by
// the least move, and a layout with nothing near it is left alone. The
// M10 with a run-out of 2 mm, whose flank met the circle 0.04 mm from
// its end, was unsound at any plane for the ridges.
func TestThreadRunOutClear(t *testing.T) {
	t.Parallel()
	type runOutCase struct {
		designation string
		length      float64
		runOut      float64
		level       bool
	}
	cases := []runOutCase{{"M10", 14, 2, false}, {"M10", 20, 2, false}, {"M8", 12, 1.64, false}}
	// The rest: whether they move is worked out below.
	for _, d := range []string{"M10", "M8", "M3", "M12xPh3P1", "Tr20x4", "1/4-20 UNC"} {
		for k := range 40 {
			cases = append(cases, runOutCase{d, 14, 0.5 + 0.1*float64(k), false}, runOutCase{d, 14, 0.5 + 0.1*float64(k), true})
		}
	}
	for i, tc := range cases {
		spec, err := thread.Parse(tc.designation)
		if err != nil {
			t.Fatal(err)
		}
		o := ThreadOptions{Spec: spec, Length: tc.length, RunOut: tc.runOut, LevelStarts: tc.level}
		l, err := o.layout(spec.Diameter / 2)
		if err != nil {
			t.Fatalf("%s run-out %g: %v", tc.designation, tc.runOut, err)
		}
		plain := o
		plain.RunOut = 0
		lp, err := plain.layout(spec.Diameter / 2)
		if err != nil {
			t.Fatal(err)
		}
		shift := l.ridgeSweep - lp.ridgeSweep
		// Moved where a flank of the end as it was met the circle.
		w := l.profile.WidthAt(l.profile.RootRadius) / 2
		was := l.ridgeBottom + l.baseW/2 + lp.ridgeSweep
		near := false
		for i := range spec.NumStarts() {
			z := was
			if !o.levelStarts() {
				z += float64(i) * spec.Pitch
			}
			near = near || math.Abs(z-w-l.runFrom) < runOutClear*spec.Pitch || math.Abs(z+w-l.runFrom) < runOutClear*spec.Pitch
		}
		if moved := shift != 0; moved != near || (i < 3 && !moved) {
			t.Errorf("%s run-out %g: the end moved by %g, a flank near the circle %v", tc.designation, tc.runOut, shift, near)
		}
		if math.Abs(shift) >= 2*runOutClear*spec.Pitch {
			t.Errorf("%s run-out %g: the end moved by %g, more than twice the clearance", tc.designation, tc.runOut, shift)
		}
		end := l.ridgeBottom + l.baseW/2 + l.ridgeSweep
		for i := range spec.NumStarts() {
			z := end
			if !o.levelStarts() {
				z += float64(i) * spec.Pitch
			}
			for _, flank := range []float64{z - w, z + w} {
				if gap := math.Abs(flank - l.runFrom); gap < runOutClear*spec.Pitch-1e-9 {
					t.Errorf("%s run-out %g: start %d's flank %g mm from the run-out's circle",
						tc.designation, tc.runOut, i, gap)
				}
			}
		}
	}
}

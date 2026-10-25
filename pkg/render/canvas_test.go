package render

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// A stroke is dashed only when its pattern has a repeat of a pixel or
// more, whichever unit the scale is in.
func TestStrokeDashScaleMm(t *testing.T) {
	t.Parallel()
	c := Camera{Scale: 5}
	for _, tc := range []struct {
		p     core.StipplePattern
		scale float64
		unit  Unit
		want  float64
	}{
		{core.StippleDash, 15, UnitPX, 3},
		{core.StippleDash, 3, UnitMM, 3},
		{core.StippleContinuous, 15, UnitPX, 0},
		{core.StippleZigzag, 15, UnitPX, 0},
		{core.StippleDash, 0, UnitPX, 0},
		{core.StippleDash, -15, UnitPX, 0},
		{core.StippleDash, math.NaN(), UnitPX, 0},
		{core.StippleDash, math.Inf(1), UnitMM, 0},
		// Dash repeats every 2 scales: 0.5 px is a pixel, 0.4 less.
		{core.StippleDash, 0.5, UnitPX, 0.1},
		{core.StippleDash, 0.4, UnitPX, 0},
	} {
		s := Stroke{StipplePattern: tc.p, StippleScale: tc.scale, Unit: tc.unit}
		if got := s.DashScaleMm(&c); math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("%s, scale %g unit %d: %g, want %g",
				core.StipplePatternName(tc.p), tc.scale, tc.unit, got, tc.want)
		}
	}
}

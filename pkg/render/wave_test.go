package render

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

func waveCamera() Camera {
	var c Camera
	c.LoadIdentity()
	c.Width, c.Height, c.PixelRatio, c.Scale = 200, 100, 1, 5
	return c
}

// A zigzag runs straight for two thirds of its period, then swings
// twice the line's width each way and back; the knots fall where the
// profile says.
func TestWaveZigzag(t *testing.T) {
	t.Parallel()
	c := waveCamera()
	// 15 px is 3 mm a scale, a period of 9 mm; 2 px wide swings 4 px,
	// 0.8 mm.
	s := Stroke{Width: 2, Unit: UnitPX, StipplePattern: core.StippleZigzag, StippleScale: 15}
	pts := s.Wave(&c, geom.Vector{}, geom.Vector{X: 9}, 0)
	want := []geom.Vector{{}, {X: 6}, {X: 6.75, Y: -0.8}, {X: 8.25, Y: 0.8}, {X: 9}}
	if len(pts) != len(want) {
		t.Fatalf("%d points %v, want %v", len(pts), pts, want)
	}
	for i := range want {
		if !pts[i].Equals(want[i]) {
			t.Errorf("point %d is %v, want %v", i, pts[i], want[i])
		}
	}
}

// The phase carries the wave from one edge of a curve to the next: the
// second edge starts where the first ended, across the line too.
func TestWavePhase(t *testing.T) {
	t.Parallel()
	c := waveCamera()
	s := Stroke{Width: 1.5, Unit: UnitPX, StipplePattern: core.StippleFreehand, StippleScale: 15}
	a, b, d := geom.Vector{}, geom.Vector{X: 4.3}, geom.Vector{X: 7.1}
	first := s.Wave(&c, a, b, 0)
	second := s.Wave(&c, b, d, 4.3)
	if !first[len(first)-1].Equals(second[0]) {
		t.Errorf("the first edge ends at %v, the second starts at %v", first[len(first)-1], second[0])
	}
	whole := s.Wave(&c, a, d, 0)
	end := whole[len(whole)-1]
	if got := second[len(second)-1]; !got.Equals(end) {
		t.Errorf("two edges end at %v, one at %v", got, end)
	}
	for _, p := range whole {
		if math.Abs(p.Y) > 2*1.5/c.Scale+1e-12 {
			t.Errorf("point %v swings more than twice the width", p)
		}
	}
}

// A segment is drawn straight when its pattern is no wave, when the
// period is under a pixel or not a number, and when the line is seen
// end on.
func TestWaveStraight(t *testing.T) {
	t.Parallel()
	c := waveCamera()
	a, b := geom.Vector{}, geom.Vector{X: 10}
	for _, s := range []Stroke{
		{StipplePattern: core.StippleContinuous, StippleScale: 15, Unit: UnitPX},
		{StipplePattern: core.StippleDash, StippleScale: 15, Unit: UnitPX},
		{StipplePattern: core.StippleFreehand, StippleScale: 0.5, Unit: UnitPX},
		{StipplePattern: core.StippleZigzag, StippleScale: -15, Unit: UnitPX},
		{StipplePattern: core.StippleZigzag, StippleScale: math.NaN(), Unit: UnitPX},
		{StipplePattern: core.StippleZigzag, StippleScale: math.Inf(1), Unit: UnitMM},
	} {
		if pts := s.Wave(&c, a, b, 0); pts != nil {
			t.Errorf("%s at %g: %d points", core.StipplePatternName(s.StipplePattern), s.StippleScale, len(pts))
		}
	}
	s := Stroke{StipplePattern: core.StippleZigzag, StippleScale: 15, Unit: UnitPX}
	if pts := s.Wave(&c, a, geom.Vector{Z: 10}, 0); pts != nil {
		t.Errorf("a line seen end on: %d points", len(pts))
	}
	if pts := s.Wave(&c, a, a, 0); pts != nil {
		t.Errorf("a line of no length: %d points", len(pts))
	}
}

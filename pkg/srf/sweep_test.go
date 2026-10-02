package srf

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// squareAt is a square of side a about (cx, cy) in the plane z = 0.
func squareAt(cx, cy, a float64) *SBezierLoopSet {
	h := a / 2
	return PolygonLoopSet([]geom.Vector{
		{X: cx - h, Y: cy - h}, {X: cx + h, Y: cy - h}, {X: cx + h, Y: cy + h}, {X: cx - h, Y: cy + h},
	}, geom.Vector{Z: 1})
}

// sweepCheck sweeps sbls along steps and checks the solid is
// watertight with the volume want.
func sweepCheck(t *testing.T, name string, sbls *SBezierLoopSet, steps []SweepStep, want float64) *SShell {
	t.Helper()
	var s SShell
	s.MakeFromSweepOf(sbls, steps, core.RgbaColor{}, 0, 0)
	if !meshIsSound(&s) {
		t.Errorf("%s: the sweep is not watertight", name)
	}
	if v := shellVolume(&s); math.Abs(v-want) > 1e-3*want {
		t.Errorf("%s: volume %g, want %g", name, v, want)
	}
	return &s
}

func TestSweepLine(t *testing.T) {
	for _, dz := range []float64{5, -5} {
		sweepCheck(t, "line", squareAt(0, 0, 2), []SweepStep{{Along: geom.Vector{Z: dz}}}, 20)
	}
}

// A square at distance R from an axis, turned about it, sweeps a
// volume of its area times R times the angle (Pappus).
func TestSweepArc(t *testing.T) {
	fineChordTol(t)
	for _, deg := range []float64{30, 60, 90, 135, 270, -60} {
		th := deg * math.Pi / 180
		st := SweepStep{Arc: true, Center: geom.Vector{X: 10}, Axis: geom.Vector{Y: 1}, Angle: th}
		sweepCheck(t, "arc", squareAt(0, 0, 2), []SweepStep{st}, 4*10*math.Abs(th))
	}
}

// A pipe bend: up, a quarter turn, and along.
func TestSweepBend(t *testing.T) {
	fineChordTol(t)
	a := 2.0
	steps := []SweepStep{
		{Along: geom.Vector{Z: 10}},
		{Arc: true, Center: geom.Vector{X: 8, Z: 10}, Axis: geom.Vector{Y: 1}, Angle: math.Pi / 2},
		{Along: geom.Vector{X: 12}},
	}
	want := a*a*10 + a*a*8*math.Pi/2 + a*a*12
	s := sweepCheck(t, "bend", squareAt(0, 0, a), steps, want)
	if end := SweepEnd(steps).Apply(geom.Vector{}); end.Minus(geom.Vector{X: 20, Z: 18}).Magnitude() > 1e-9 {
		t.Errorf("the path ends at %v", end)
	}
	// Holes are swept too: a square ring.
	ring := squareAt(0, 0, a)
	ring.Holes = append(ring.Holes, func() SBezierLoop {
		in := squareAt(0, 0, 1).Outer
		in.Reverse()
		return in
	}())
	sweepCheck(t, "ring", ring, steps, want*3/4)
	_ = s
}

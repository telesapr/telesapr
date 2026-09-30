package srf

import (
	"reflect"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/internal/par"
)

// TestIntersectionCurvesAtOnceSameAsInOrder intersects two surfaces
// that meet a third in the same line, up to LENGTH_EPS. In order, the
// second's curve follows the pwl of the first's; at once it can't see
// it, and has to be made over when the curves are added.
func TestIntersectionCurvesAtOnceSameAsInOrder(t *testing.T) {
	run := func(mode ParallelMode) []SCurve {
		SetParallelMode(mode)
		defer SetParallelMode(ParallelExact)
		if mode != ParallelOff {
			par.SetWorkers(8)
		}
		a, b := &SShell{}, &SShell{}
		for i, x := range []float64{0, 1e-9} {
			s := FromPlane(geom.Vector{X: x, Y: -1}, geom.Vector{X: 1}, geom.Vector{Y: 2})
			s.H = core.HSSurface{V: uint32(i + 1)}
			a.Surface = append(a.Surface, s)
		}
		s := FromPlane(geom.Vector{X: -1, Z: -1}, geom.Vector{X: 3}, geom.Vector{Z: 2})
		s.H = core.HSSurface{V: 1}
		b.Surface = append(b.Surface, s)

		var into SShell
		a.MakeIntersectionCurvesAgainst(b, &into)
		return into.Curve
	}

	want := run(ParallelOff)
	if len(want) != 2 || want[0].Points[0].P != want[1].Points[0].P ||
		want[0].Exact.Ctrl[0] == want[1].Exact.Ctrl[0] {
		t.Fatalf("in order: want two curves with one pwl, got %+v", want)
	}
	redone := sinkRedone.Load()
	got := run(ParallelExact)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("at once: %+v\nin order: %+v", got, want)
	}
	if sinkRedone.Load() != redone+1 {
		t.Errorf("%d curves made over, want 1", sinkRedone.Load()-redone)
	}
}

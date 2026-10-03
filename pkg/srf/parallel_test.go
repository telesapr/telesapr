package srf

import (
	"reflect"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/par"
)

// TestIntersectionCurvesAtOnceSameAsInOrder intersects two surfaces
// that meet a third in the same line, up to LENGTH_EPS. The second's
// curve follows the pwl of the first's: run at once it can't see it,
// and is made over when the curves are added, on one goroutine as on
// eight.
func TestIntersectionCurvesAtOnceSameAsInOrder(t *testing.T) {
	run := func(mode ParallelMode) []SCurve {
		SetParallelMode(mode)
		defer SetParallelMode(ParallelOn)
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

	redone := sinkRedone.Load()
	want := run(ParallelOff)
	if len(want) != 2 || want[0].Points[0].P != want[1].Points[0].P ||
		want[0].Exact.Ctrl[0] == want[1].Exact.Ctrl[0] {
		t.Fatalf("on one goroutine: want two curves with one pwl, got %+v", want)
	}
	if sinkRedone.Load() != redone+1 {
		t.Errorf("%d curves made over on one goroutine, want 1", sinkRedone.Load()-redone)
	}
	got := run(ParallelOn)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("on eight: %+v\non one: %+v", got, want)
	}
}

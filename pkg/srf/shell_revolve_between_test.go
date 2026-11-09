package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// A rectangle revolved through breaks of its own, a whole turn in three
// unequal pieces and arcs capped at their ends, with and without an
// edge on the axis, is sound and of the volume the angles give
// (MakeFromRevolutionBetween).
func TestRevolveBetweenVolumes(t *testing.T) {
	t.Parallel()
	tol := fineTol()
	for _, c := range []struct {
		name   string
		breaks []float64
		r0     float64
	}{
		{"whole", []float64{0.3, 0.3 + 2.5, 0.3 + 4, 0.3 + 2*math.Pi}, 1},
		{"whole solid", []float64{-1, 1.5, 3, -1 + 2*math.Pi}, 0},
		{"arc", []float64{-1, 0.2, -1 + 200*math.Pi/180}, 1},
		{"arc solid", []float64{0.5, 2, 3}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			sh := &SShell{Tol: tol}
			sh.MakeFromRevolutionBetween(profileLoopSet(c.r0, 2, 0, 1), geom.Vector{}, geom.Vector{Z: 1}, core.RGBA{}, noFaces{}, c.breaks)
			span := c.breaks[len(c.breaks)-1] - c.breaks[0]
			v, sound := soundVolume(sh)
			checkVolume(t, c.name, v, span/2*(4-c.r0*c.r0))
			if !sound {
				t.Errorf("not sound")
			}
		})
	}
}

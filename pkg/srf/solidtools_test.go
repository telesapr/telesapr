package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

func TestWindingNumber(t *testing.T) {
	t.Parallel()
	var m SMesh
	extrudedBox(geom.Vector{}, geom.Vector{X: 2, Y: 2, Z: 2}, Tolerance{}).TriangulateInto(&m)
	for _, c := range []struct {
		p    geom.Vector
		want float64
	}{
		{geom.Vector{X: 1, Y: 1, Z: 1}, 1},
		{geom.Vector{X: 0.01, Y: 1.99, Z: 0.5}, 1},
		{geom.Vector{X: 3, Y: 1, Z: 1}, 0},
		{geom.Vector{X: -0.01, Y: 1, Z: 1}, 0},
	} {
		if w := m.WindingNumber(c.p); math.Abs(w-c.want) > 1e-9 {
			t.Errorf("winding number at %v is %g, want %g", c.p, w, c.want)
		}
	}
}

// A triangle swept either way, with its points either way round,
// makes a sound prism of the right volume.
func TestMakePrism(t *testing.T) {
	t.Parallel()
	tri := []geom.Vector{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 0, Y: 3}}
	rev := []geom.Vector{tri[2], tri[1], tri[0]}
	for _, pts := range [][]geom.Vector{tri, rev} {
		for _, dz := range []float64{4, -4} {
			var s SShell
			s.MakePrism(pts, geom.Vector{}, geom.Vector{Z: dz}, core.RGBA{})
			var m SMesh
			s.TriangulateInto(&m)
			if v := m.CalculateVolume(); math.Abs(v-12) > 1e-9 {
				t.Errorf("prism of %v by %g has volume %g, want 12", pts, dz, v)
			}
		}
	}
}

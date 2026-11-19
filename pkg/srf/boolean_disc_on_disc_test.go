package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// TestUnionDiscSharingTopAndSide unites a disc 1 high with a disc of its
// radius lying in it, the two sharing the top or the bottom, or both,
// and the side's cylinder: the result is the first disc. The curve where
// the plane of the shared top meets the other's side runs back along
// the top's own circle, and was taken for a sliver beside it, the two
// cancelled, and the top lost its outline (found 2026-11-19 putting back
// a ledge's ring below a fillet past a disc's rim). Either order, at a
// coarse and a fine chord tolerance; sound and of the first disc's
// volume, short of it by no more than the chords' sag round the side.
func TestUnionDiscSharingTopAndSide(t *testing.T) {
	t.Parallel()
	for _, ct := range []float64{0.1, 0.005} {
		tol := Tol(ct, 2000)
		disc := func(z0, z1 float64) *SShell {
			return extrudedCurves(circleCurves(geom.Vector{Z: z0}, geom.Vector{X: 1}, geom.Vector{Y: 1}, 10),
				geom.Vector{}, geom.Vector{Z: z1 - z0}, tol)
		}
		want, sag := 100*math.Pi, ct*2*math.Pi*10
		for _, k := range []struct {
			name   string
			b0, b1 float64
		}{{"top", 0.25, 1}, {"bottom", 0, 0.75}, {"both", 0, 1}} {
			for _, rev := range []bool{false, true} {
				t.Run(fmt.Sprintf("%g %s %v", ct, k.name, rev), func(t *testing.T) {
					t.Parallel()
					a, b := disc(0, 1), disc(k.b0, k.b1)
					if rev {
						a, b = b, a
					}
					u := &SShell{Tol: tol}
					u.MakeFromBoolean(a, b, SurfaceUnion)
					v, ok := soundVolume(u)
					if !ok {
						t.Fatal("not sound")
					}
					if v > want || want-v > sag {
						t.Errorf("volume %.6f, want %.6f", v, want)
					}
				})
			}
		}
	}
}

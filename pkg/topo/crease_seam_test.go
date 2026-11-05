package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// TestCreaseSeamAfterBoolean cuts a pin of radius 1 out of the middle
// of a disc of radius 6 drafted both ways from z = 0 to ±5, as a
// sketch's two-sided draft makes it: two drafted prisms joined, their
// caps on z = 0 cancelling, carried through a Boolean of nothing as a
// sketch's first group is. From 8° on, the crease's curves, which named
// the dropped caps by handles the second Boolean gave to other patches,
// were thinned against those patches, each side's differently, and the
// cut left the crease open far from the pin
// (srf.TestDroppedSurfaceNamedByNone).
func TestCreaseSeamAfterBoolean(t *testing.T) {
	t.Parallel()
	// The tolerance a sketch at a chord tolerance of 0.005 gives a solid
	// of this size.
	tol := srf.Tol(0.00048828125, 20)
	col := core.NewRGBA(100, 100, 100, 255)
	for _, deg := range []float64{2, 5, 8, 10, 15, 20} {
		t.Run(fmt.Sprint(deg), func(t *testing.T) {
			t.Parallel()
			if testing.Short() && deg != 10 {
				t.Skip("only the draft of 10°, short")
			}
			a := deg * math.Pi / 180
			var halves []*srf.SShell
			for _, h := range []float64{-5, 5} {
				sh, err := topo.DraftedPrism([][]srf.SBezier{circle(6, 0, 0)}, v(0, 0, h), a, col, 0, tol)
				if err != nil {
					t.Fatal(err)
				}
				halves = append(halves, sh)
			}
			var joined, disc, out srf.SShell
			joined.MakeFromUnionOf(halves[0], halves[1])
			disc.MakeFromUnionOf(&srf.SShell{Tol: tol}, &joined)
			pin, err := topo.DraftedPrism([][]srf.SBezier{circle(1, 0, 0)}, v(0, 0, 2), 0, col, 0, tol)
			if err != nil {
				t.Fatal(err)
			}
			out.MakeFromDifferenceOf(&disc, pin)
			if out.BooleanFailed {
				t.Fatal("the Boolean failed")
			}
			// Two frustums from radius 6 to 6 - 5 tan α, less the pin.
			r := 6 - 5*math.Tan(a)
			want := 2*math.Pi*5/3*(36+6*r+r*r) - 2*math.Pi
			if got := sound(t, &out); math.Abs(got-want) > 2e-4*want {
				t.Errorf("volume %g, want %g", got, want)
			}
		})
	}
}

package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/topo"
)

// The dish's pocket with its whole rims x+ and y+ rounded, meeting at
// the corner (12, 6), written as STEP one surface per patch and one per
// face and read back: one closed solid, the volume ours. FreeCADCmd
// read all four files as one valid solid; our reader closed none before
// 2026-11-20. One surface per patch, the fillet's end on the crease
// starts its side on two control points at the corner, whose
// derivative along the side vanishes there: taken for a pole, the
// corner's parameters were copied from the point before it (collapses),
// and the kernel's search put the corner at two places in its
// parameters, one for each trim through it (makeTrimEdgesInto). One
// surface per face, the x+ fillet's sheet ends in the corner patch,
// which folds the sheet's side to the corner: the bounds meeting there
// leave from either end of the fold (foldEnd, withFolds); and at a
// radius of 1 the fillet's foot crossing the dish's seam, on the
// sheet's side a margin inside the grid, was on one cell's right side
// and the next one's bottom (perimeter).
func TestFilletDishRimStep(t *testing.T) {
	t.Parallel()
	for _, one := range []bool{false, true} {
		for _, r := range []float64{1, 2} {
			if testing.Short() && r != 1 {
				continue
			}
			suffix := ""
			if one {
				suffix = "_one"
			}
			name := fmt.Sprintf("dish_rims_x+y+_%g%s", r, suffix)
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				sh := dishPocket(fine(0.01))
				if err := topo.FilletPicks(sh, dishRim("x+", "y+"), r, 1<<30, false); err != nil {
					t.Fatal(err)
				}
				want := surfaceVolume(t, sh)
				_, read := stepRoundTrip(t, sh, one, name)
				sound(t, read)
				if got := surfaceVolume(t, read); math.Abs(got-want) > 2e-4*want {
					t.Errorf("volume read %.4f, ours %.4f", got, want)
				}
			})
		}
	}
}

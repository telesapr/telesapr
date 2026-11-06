package api

import (
	"fmt"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// Chamfers and fillets of the edge a plate 2 wide stands on a rod of
// radius 5 by, at sizes about 1.5 and at segment counts that put the
// rod's end rings' points on, or beside, the line the blend's foot runs
// along the rod. At a chamfer of 1.5 and 50 segments a curve or more the
// foot lies 0.00017 mm from the line joining two points of the rings,
// and the ear clipper's long edge between them ran along the foot's
// points without stopping at them: the rod's strip of triangles lay over
// the chamfer's, and 83 edges were left open, with nothing said
// (2026-11-04). The rod's face is meshed again where its edges run along
// another's open edge (srf's runsAlongOpen). Each blend is made and
// sound. The ordinary run takes both blends of 1.5 at every segment
// count and each other size once, at a segment count and kind of its
// own: 12 of the 40, side by side; make test-sweeps all of them.
func TestChamferFootOnRodGrid(t *testing.T) {
	t.Parallel()
	segs := []int{20, 50, 100, 200}
	sizes := []float64{1.45, 1.49, 1.5, 1.51, 1.55}
	if (testing.Short() || fixtures.RaceDetector) && !sweep.Full() {
		segs, sizes = []int{50}, []float64{1.5}
	}
	// The segment count and kind each size but 1.5 takes in the ordinary
	// run.
	type at struct {
		seg     int
		chamfer bool
	}
	others := map[float64]at{1.45: {50, true}, 1.49: {100, false}, 1.51: {200, true}, 1.55: {20, false}}
	for _, seg := range segs {
		for _, r := range sizes {
			for _, chamfer := range []bool{true, false} {
				if o, ok := others[r]; ok && o != (at{seg, chamfer}) && !sweep.Full() && len(sizes) > 1 {
					continue
				}
				name := fmt.Sprintf("%d segments, %s of %g", seg, map[bool]string{true: "chamfer", false: "fillet"}[chamfer], r)
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					d := New()
					d.Edit.SetChordTolerance(0.003)
					d.Edit.SetMaxSegments(seg)
					edge, before := gabledPlateOnRod(d, 10, 20, 2, 2)
					g, err := blendOrRefusal(d, edge, r, chamfer)
					if err != nil {
						t.Fatalf("refused: %v", err)
					}
					rep := g.Analyze().Check()
					if !rep.Sound() {
						t.Errorf("%s", rep.Summary())
					}
					if rep.Volume <= before {
						t.Errorf("volume %g, no more than the %g before", rep.Volume, before)
					}
				})
			}
		}
	}
}

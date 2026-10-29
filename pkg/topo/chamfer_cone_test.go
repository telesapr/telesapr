package topo_test

import (
	"fmt"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestChamferConeSweep chamfers some of the edges at a pyramid's apex,
// convex, and at a pyramid's pocket's, concave, the rest left sharp, and
// all three at a ledge's top and a rib's foot, where convex edges meet
// a concave one, or a ledge's top edge alone, or with the concave edge
// below it, and a rib's upright with a foot edge: each made, sound and
// closed. The rate allowed, 0, was
// fixed before the sweep first ran.
func TestChamferConeSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("corner", "apexsome", "pocketsome", "ledgeall", "riball", "ledgeone", "ledgetwo", "ribtwo"),
			sweep.Strings("picks", "1000", "1100", "1010", "1110"),
			sweep.Floats("d1", 0.5, 1.25, 2),
			sweep.Floats("d2", 0.5, 1.25, 2),
			sweep.Floats("d3", 0.5, 2),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		shape := p.String("corner")
		d := []float64{p.Float("d1"), p.Float("d2"), p.Float("d3")}
		if shape == "apexsome" || shape == "pocketsome" {
			d = append(d, 0)
			for i, c := range p.String("picks") {
				if c == '0' {
					d[i] = 0
				}
			}
		}
		sh, err := chamferOracleCase(shape, d, srf.Tol(0.002, 20))
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if rep := check.Solid(sh, &m); !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		return nil
	}).Require(t)
}

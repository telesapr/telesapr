package topo_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestChamferPairSweep chamfers two edges meeting above a third left
// sharp, each at its own two distances: a box's corner, a pocket's, a
// rib's foot, a ledge's top above a concave edge, and the foot of a
// prism's upright edge at angles from 45 to 135 degrees. Each must be
// made, sound and closed. The rate allowed, 0, was fixed before the
// sweep first ran.
func TestChamferPairSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("corner", "pair", "pocket", "rib", "ledge", "prismpair45", "prismpair75", "prismpair105", "prismpair135"),
			sweep.Floats("tx", 0.5, 2),
			sweep.Floats("sx", 0.5, 1.25, 2),
			sweep.Floats("ty", 0.5, 1.25),
			sweep.Floats("sy", 0.5, 2),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		shape := p.String("corner")
		d := []float64{p.Float("tx"), p.Float("sx"), p.Float("ty"), p.Float("sy")}
		if deg, ok := strings.CutPrefix(shape, "prismpair"); ok {
			a, _ := strconv.ParseFloat(deg, 64)
			shape, d = "prismpair", append([]float64{a}, d...)
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

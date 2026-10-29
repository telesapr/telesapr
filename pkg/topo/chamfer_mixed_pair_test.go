package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// ledgeAt is a block 20 high whose outline has an inside corner at (10,
// 10) of deg degrees, between a wall along x and one turned deg degrees
// from it, at the tolerance tol: the top edge from (10, 10, 20) along x
// is convex, the upright below it concave.
func ledgeAt(deg float64, tol srf.Tolerance) *srf.SShell {
	th := deg * math.Pi / 180
	sh := srf.SShell{Tol: tol}
	sh.MakePrism([]geom.Vector{v(0, 0, 0), v(20, 0, 0), v(20, 10, 0), v(10, 10, 0),
		v(10+10*math.Cos(th), 10+10*math.Sin(th), 0), v(0, 20, 0)}, v(0, 0, 0), v(0, 0, 20), core.NewRGBA(100, 100, 100, 255))
	for i := range sh.Surface {
		sh.Surface[i].Face = srf.FaceID(i + 1)
	}
	return &sh
}

// ribAt is the plate (0, 0, 0) to (20, 20, 5) with a block 10 high on it
// whose outline has a corner at (10, 10) of deg degrees, one side along
// x to (4, 10), the next square to it, at the tolerance tol: the
// block's upright edge at (10, 10) is convex, its foot edges concave.
func ribAt(deg float64, tol srf.Tolerance) *srf.SShell {
	th := deg * math.Pi / 180
	var a, b srf.SShell
	a.Tol, b.Tol = tol, tol
	a.MakePrism([]geom.Vector{v(0, 0, 0), v(20, 0, 0), v(20, 20, 0), v(0, 20, 0)}, v(0, 0, 0), v(0, 0, 5), core.NewRGBA(100, 100, 100, 255))
	b.MakePrism([]geom.Vector{v(10, 10, 5), v(4, 10, 5), v(4, 4, 5), v(10-6*math.Cos(th), 10-6*math.Sin(th), 5)}, v(0, 0, 0), v(0, 0, 10), core.NewRGBA(100, 100, 100, 255))
	out := &srf.SShell{Tol: tol}
	out.MakeFromUnionOf(&a, &b)
	for i := range out.Surface {
		out.Surface[i].Face = srf.FaceID(i + 1)
	}
	return out
}

// mixedPairCase chamfers, at the tolerance tol, the convex and the
// concave edge of ledgeAt's or ribAt's corner of deg degrees, the
// ledge's top edge along x at d1 and the upright below it at d2, the
// rib's upright at d1 and its foot edge along x at d2, each with k times
// its distance on its second face, or with k 0 the same. It is the shell
// and how many faces it had before.
func mixedPairCase(shape string, deg, d1, d2, k float64, tol srf.Tolerance) (*srf.SShell, int, error) {
	var sh *srf.SShell
	var picks []topo.FilletPick
	if shape == "ledge" {
		sh = ledgeAt(deg, tol)
		c := v(10, 10, 20)
		picks = []topo.FilletPick{{A: c, B: v(20, 10, 20), R: d1}, {A: c, B: v(10, 10, 0), R: d2}}
	} else {
		sh = ribAt(deg, tol)
		c := v(10, 10, 5)
		picks = []topo.FilletPick{{A: c, B: v(10, 10, 15), R: d1}, {A: c, B: v(4, 10, 5), R: d2}}
	}
	for i := range picks {
		picks[i].D2 = k * picks[i].R
	}
	tp, err := topo.Build(sh)
	if err != nil {
		return nil, 0, err
	}
	return sh, len(tp.Faces), topo.FilletPicks(sh, picks, 1, 1<<30, true)
}

// TestChamferMixedPairSweep chamfers a convex and a concave edge meeting
// above a third left sharp, at a ledge's top and a rib's foot whose
// walls meet at 60 to 135 degrees, over distances no more than 4 to 1
// apart, each of one distance or of two: each sound and closed, the corner made at once, two strips and
// the face that fills it more than the solid had, not in stages. The
// rate allowed, 0, was fixed before the sweep first ran.
func TestChamferMixedPairSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("corner", "ledge", "rib"),
			sweep.Floats("deg", 60, 75, 90, 105, 120, 135),
			sweep.Floats("d1", 0.5, 1.25, 2),
			sweep.Floats("d2", 0.5, 1.25, 2),
			sweep.Floats("second", 0, 0.6, 1.6),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		sh, before, err := mixedPairCase(p.String("corner"), p.Float("deg"), p.Float("d1"), p.Float("d2"), p.Float("second"), srf.Tol(0.002, 20))
		if err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if rep := check.Solid(sh, &m); !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		tp, err := topo.Build(sh)
		if err != nil {
			return err
		}
		if len(tp.Faces) != before+3 {
			return fmt.Errorf("%d faces from %d: not made at once", len(tp.Faces), before)
		}
		return nil
	}).Require(t)
}

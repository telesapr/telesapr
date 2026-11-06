package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// coaxialPair is a solid and a second one lying in it, their curved
// faces on each other but for eps (positive: the second larger, sunk in
// by eps): a cone in a countersink of its angle, or a ball in a seat of
// its radius. Roadmap K3, after the spike of 2026-11-04
// (doc/dev-notes/k3-fuse-spike.md), which did the same for a pin in a
// bore (TestPinInBoreSweep).
func coaxialPair(fixture string, eps float64, tol Tolerance) (a, b *SShell) {
	v := func(x, z float64) geom.Vector { return geom.Vector{X: x, Z: z} }
	switch fixture {
	case "cone":
		// A ring with a countersink at 45°, and a cone of that angle
		// standing in it, past it at both ends, its side off by eps
		// square to it.
		s := eps * math.Sqrt2
		a = lathedProfile([]geom.Vector{v(2, 0), v(8, 0), v(8, 3), v(5, 3)}, tol)
		b = lathedProfile([]geom.Vector{v(0, -0.5), v(1.5+s, -0.5), v(5.5+s, 3.5), v(0, 3.5)}, tol)
	default:
		// A block with a seat for a ball in its top, two thirds of the
		// ball deep, and a ball of the seat's radius but eps in it.
		c := geom.Vector{Z: 1}
		blk := extrudedBox(geom.Vector{X: -5, Y: -5, Z: -5}, geom.Vector{X: 5, Y: 5, Z: 0}, tol)
		a = &SShell{Tol: tol}
		a.MakeFromBoolean(blk, patchUpSphere(3, c, tol), SurfaceDiff)
		b = patchUpSphere(3+eps, c, tol)
	}
	return a, b
}

// TestCoaxialContactSweep is TestPinInBoreSweep for cones: a cone in a
// countersink of its angle, touching or off by a hair either way, turned,
// at two tolerances, by the three Booleans. Within the fuse (a tenth of
// the chord tolerance) the second is taken onto the first, so the union
// is both volumes, the difference the first's, and the intersection
// none. On devel of 2026-11-04 all 96 within the fuse were wrong (the
// Boolean failed, or at exact contact the union left the rim open).
func TestCoaxialContactSweep(t *testing.T) {
	t.Parallel()
	coaxialSweep(t, []string{"cone"}, 0)
}

// TestCoaxialSeatSweep is the same for a ball in a seat of its radius.
// On 2026-11-04 50 of 96 within the fuse were wrong (88 before
// snapToSpheres and classifyOnCoincident), all unions and differences:
// the seat's rim was the marched curve of the Boolean that made it, and
// the ball's crossing of the block's top was marched again through other
// points. A plane now meets a sphere in an exact circle
// (intersectPlaneAndSphere), the same in both Booleans, and an edge on a
// sphere lying on another is classified coincident where the edge's
// chord dips off it (roadmap K3).
func TestCoaxialSeatSweep(t *testing.T) {
	t.Parallel()
	coaxialSweep(t, []string{"seat"}, 0)
}

// coaxialSweep runs the coaxial fixtures named over offsets either side
// of touching, allowing failRate of them wrong.
func coaxialSweep(t *testing.T, fixtures []string, failRate float64) {
	t.Helper()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("fixture", fixtures...),
			sweep.Floats("eps", -1e-3, -1e-5, -1e-6, -1e-7, 0, 1e-7, 1e-6, 1e-5, 1e-3),
			sweep.Floats("chord", 0.1, 0.01),
			sweep.Floats("turn", 0, 35),
			sweep.Strings("op", "union", "difference", "intersection"),
		},
		MaxFailRate: failRate,
		Sample:      40,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		tol, eps := Tol(p.Float("chord"), 20), p.Float("eps")
		if math.Abs(eps) >= tol.ChordTolMm()/snapPlaneDiv {
			return nil // past the fuse: a sliver under the chord tolerance is K4's
		}
		a, b := coaxialPair(p.String("fixture"), eps, tol)
		a, b = nearTurn(a, p.Float("turn")), nearTurn(b, p.Float("turn"))
		aV, bV := shellVolume(a), shellVolume(b)
		var op SurfaceCombineAs
		var want float64
		switch p.String("op") {
		case "union":
			op, want = SurfaceUnion, aV+bV
		case "difference":
			op, want = SurfaceDiff, aV
		default:
			op, want = SurfaceInter, 0
		}
		var out SShell
		out.MakeFromBoolean(a, b, op)
		if out.BooleanFailed {
			return errUnsound(true)
		}
		var m SMesh
		out.TriangulateInto(&m)
		got := m.CalculateVolume()
		if want == 0 {
			if math.Abs(got) > 1e-2 {
				return fmt.Errorf("%w: %g, want none", errNearVolume, got)
			}
			return nil
		}
		if !meshSound(&m) {
			return errUnsound(false)
		}
		if math.Abs(got-want) > 2e-3*want {
			return fmt.Errorf("%w: %g, want %g", errNearVolume, got, want)
		}
		return nil
	}).Require(t)
}

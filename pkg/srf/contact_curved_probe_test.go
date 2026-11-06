package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// TestCurvedContactSweep is the K3 spike's probe of curved contact
// (doc/dev-notes/k3-fuse-spike.md), made a test: a pin in a bore of its
// radius, off by eps (a lathed tube and a lathed rod: coaxial cylinders),
// a ball resting on a plate, a cylinder lying on one, at offsets either
// side of touching, two chord tolerances, two turns and the three
// Booleans. Each is sound, but for the union of a round face resting on a
// plane within a tenth of the chord tolerance (snapTangent): the two then
// touch along a line, and their union is two solids joined along it, no
// manifold there, as the true union of solids that touch; its volume is
// theirs added. On 2026-11-04 the probe had pin 77 of 108 wrong, ball 12,
// cylinder 30; on 2026-11-05 before snapTangent, ball 3, cylinder 30.
// The pin is TestPinInBoreSweep's lathed fixture, but at offsets past the
// fuse too, which that sweep leaves out. The ordinary run takes 36 of
// the 324 Booleans, make test-sweeps all of them.
func TestCurvedContactSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("curved contact sweep: 324 Booleans, a sample of 36")
	}
	t.Parallel()
	v := func(x, z float64) geom.Vector { return geom.Vector{X: x, Z: z} }
	pair := func(fx string, eps float64, tol Tolerance) (a, b *SShell) {
		switch fx {
		case "pin":
			a = lathedProfile([]geom.Vector{v(3, 0), v(5, 0), v(5, 6), v(3, 6)}, tol)
			b = lathedProfile([]geom.Vector{v(0, -1), v(3+eps, -1), v(3+eps, 7), v(0, 7)}, tol)
		case "ball":
			a = extrudedBox(geom.Vector{X: -5, Y: -5, Z: -2}, geom.Vector{X: 5, Y: 5, Z: 0}, tol)
			b = patchUpSphere(3, geom.Vector{Z: 3 + eps}, tol)
		default:
			a = extrudedBox(geom.Vector{X: -5, Y: -5, Z: -2}, geom.Vector{X: 5, Y: 5, Z: 0}, tol)
			c := lathedProfile([]geom.Vector{v(0, -4), v(2, -4), v(2, 4), v(0, 4)}, tol)
			b = &SShell{Tol: tol}
			b.MakeFromTransformationOf(c, geom.Vector{Z: 2 + eps}, geom.QuaternionFromAxisAngle(geom.Vector{Y: 1}, math.Pi/2), 1)
		}
		return a, b
	}
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("fixture", "pin", "ball", "lying"),
			sweep.Floats("eps", -1e-2, -1e-3, -1e-5, -1e-6, 0, 1e-6, 1e-5, 1e-3, 1e-2),
			sweep.Floats("chord", 0.1, 0.01),
			sweep.Floats("turn", 0, 35),
			sweep.Strings("op", "union", "difference", "intersection"),
		},
		MaxFailRate: 0,
		Sample:      36,
		Parallel:    true,
	}
	ops := map[string]SurfaceCombineAs{"union": SurfaceUnion, "difference": SurfaceDiff, "intersection": SurfaceInter}
	grid.Run(func(p sweep.Point) error {
		fx, op := p.String("fixture"), ops[p.String("op")]
		a, b := pair(fx, p.Float("eps"), Tol(p.Float("chord"), 20))
		a, b = nearTurn(a, p.Float("turn")), nearTurn(b, p.Float("turn"))
		var out SShell
		out.MakeFromBoolean(a, b, op)
		if out.BooleanFailed {
			return errUnsound(true)
		}
		got, sound := soundVolume(&out)
		if got < 1e-3 || sound {
			return nil
		}
		// Two solids touching along a line: their volumes added, the
		// round's chords inside its true side.
		slack := 3 * a.Tol.ChordTolMm() * 100
		if op == SurfaceUnion && fx == "lying" && math.Abs(got-shellVolume(a)-shellVolume(b)) < slack {
			return nil
		}
		return fmt.Errorf("not sound, volume %.5f", got)
	}).Require(t)
}

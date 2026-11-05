package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
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
func TestCurvedContactSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("curved contact sweep: 324 Booleans, about 40 s")
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
	volume := func(sh *SShell) float64 {
		var m SMesh
		sh.TriangulateInto(&m)
		return m.CalculateVolume()
	}
	for _, fx := range []string{"pin", "ball", "lying"} {
		for _, eps := range []float64{-1e-2, -1e-3, -1e-5, -1e-6, 0, 1e-6, 1e-5, 1e-3, 1e-2} {
			for _, chord := range []float64{0.1, 0.01} {
				for _, turn := range []float64{0, 35} {
					a, b := pair(fx, eps, Tol(chord, 20))
					a, b = nearTurn(a, turn), nearTurn(b, turn)
					for _, op := range []SurfaceCombineAs{SurfaceUnion, SurfaceDiff, SurfaceInter} {
						var out SShell
						out.MakeFromBoolean(a, b, op)
						if out.BooleanFailed {
							t.Errorf("%s eps %g chord %g turn %g op %d: the Boolean failed", fx, eps, chord, turn, op)
							continue
						}
						got := volume(&out)
						if got < 1e-3 || meshIsSound(&out) {
							continue
						}
						// Two solids touching along a line: their volumes
						// added, the round's chords inside its true side.
						slack := 3 * a.Tol.ChordTolMm() * 100
						if op == SurfaceUnion && fx == "lying" && math.Abs(got-volume(a)-volume(b)) < slack {
							continue
						}
						t.Errorf("%s eps %g chord %g turn %g op %d: not sound, volume %.5f", fx, eps, chord, turn, op, got)
					}
				}
			}
		}
	}
}

package srf

import (
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// TestCurvedContactProbe is the K3 spike's probe of curved contact
// (doc/dev-notes/k3-fuse-spike.md), what snapToPlanes leaves: a pin in a
// bore of its radius, off by eps (a lathed tube and a lathed rod: coaxial
// cylinders), a ball resting on a plate, a cylinder lying on one, at
// offsets either side of touching, two chord tolerances, two turns and
// the three Booleans. It reports how many of each offset's 12 failed or
// came out with a mesh not closed, and fails nothing: run it with
// TELESAPR_K3_PROBE=1. On 2026-11-04: pin 77 of 108 (all 12 at eps 0),
// ball 12, cylinder 30.
func TestCurvedContactProbe(t *testing.T) {
	if os.Getenv("TELESAPR_K3_PROBE") == "" {
		t.Skip("the K3 spike's curved-contact probe: set TELESAPR_K3_PROBE=1")
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
	for _, fx := range []string{"pin", "ball", "lying"} {
		row, bad, total := "", 0, 0
		for _, eps := range []float64{-1e-2, -1e-3, -1e-5, -1e-6, 0, 1e-6, 1e-5, 1e-3, 1e-2} {
			n := 0
			for _, chord := range []float64{0.1, 0.01} {
				for _, turn := range []float64{0, 35} {
					a, b := pair(fx, eps, Tol(chord, 20))
					a, b = nearTurn(a, turn), nearTurn(b, turn)
					for _, op := range []SurfaceCombineAs{SurfaceUnion, SurfaceDiff, SurfaceInter} {
						var out SShell
						out.MakeFromBoolean(a, b, op)
						total++
						var m SMesh
						out.TriangulateInto(&m)
						empty := m.IsEmpty() || m.CalculateVolume() < 1e-3
						if out.BooleanFailed || (!empty && !meshIsSound(&out)) {
							n++
							bad++
						}
					}
				}
			}
			row += " " + strconv.FormatFloat(eps, 'g', -1, 64) + ":" + strconv.Itoa(n)
		}
		t.Logf("%-6s%s   (%d of %d)", fx, row, bad, total)
	}
}

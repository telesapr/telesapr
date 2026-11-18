package topo_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// surfaceVolume is the volume of sh's mesh carried out to its surfaces:
// each triangle's chords take the gap between the triangle and its
// surface, measured at the triangle's edges' midpoints along its normal,
// which a quadratic gap vanishing at the corners integrates exactly. A
// mesh's own volume is off by the chords' sag over the whole surface,
// some 0.0001 mm deep on average on the dish's cone at a chord tolerance
// of 0.002 mm and so about 0.6 mm³, and that error changes when a face is
// trimmed anew and meshed again: the difference of two meshes' volumes,
// as a fillet's volume taken, can be off by 1% to 10% of a fillet of 1
// where the solids themselves agree to 1e-6 (dishPocket). The carried
// volume is within 0.0001 mm³ of the dish's closed form from a chord
// tolerance of 0.005 mm down.
func surfaceVolume(t *testing.T, sh *srf.SShell) float64 {
	t.Helper()
	ms := sh.TriangulateBySurface()
	vol := 0.0
	for i := range ms {
		s := &sh.Surface[i]
		vol += ms[i].CalculateVolume()
		for _, tr := range ms[i].Triangles {
			n := tr.B.Minus(tr.A).Cross(tr.C.Minus(tr.A))
			area := n.Magnitude() / 2
			if area == 0 {
				continue
			}
			n = n.ScaledBy(1 / (2 * area))
			gap := 0.0
			for _, m := range []geom.Vector{
				tr.A.Plus(tr.B).ScaledBy(0.5), tr.B.Plus(tr.C).ScaledBy(0.5), tr.C.Plus(tr.A).ScaledBy(0.5),
			} {
				var uv geom.Point2d
				s.ClosestPointTo(m, &uv, false)
				gap += s.PointAtUv(uv).Minus(m).Dot(n)
			}
			vol += gap * area / 3
		}
	}
	return vol
}

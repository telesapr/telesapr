package topo_test

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// surfaceVolumeAbove is the volume of sh above the plane z = z0, as
// surfaceVolume measures it: each triangle wholly above the plane
// carried out to its surface, each that crosses it cut there and taken
// as it is (its share is a strip a triangle wide along the plane). The
// solid's cut by the plane closes it but adds nothing, its tetrahedra
// taken from a point of the plane.
func surfaceVolumeAbove(t *testing.T, sh *srf.SShell, z0 float64) float64 {
	t.Helper()
	o := geom.Vector{Z: z0}
	tetra := func(a, b, c geom.Vector) float64 {
		return a.Minus(o).Dot(b.Minus(o).Cross(c.Minus(o))) / 6
	}
	ms := sh.TriangulateBySurface()
	vol := 0.0
	for i := range ms {
		s := &sh.Surface[i]
		for _, tr := range ms[i].Triangles {
			pts := []geom.Vector{tr.A, tr.B, tr.C}
			above := 0
			for _, p := range pts {
				if p.Z >= z0 {
					above++
				}
			}
			switch above {
			case 0:
				continue
			case 3:
				vol += tetra(tr.A, tr.B, tr.C)
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
				continue
			}
			// The triangle cut by the plane: the polygon of it above.
			var poly []geom.Vector
			for k, p := range pts {
				q := pts[(k+1)%3]
				if p.Z >= z0 {
					poly = append(poly, p)
				}
				if (p.Z >= z0) != (q.Z >= z0) {
					f := (z0 - p.Z) / (q.Z - p.Z)
					poly = append(poly, p.Plus(q.Minus(p).ScaledBy(f)))
				}
			}
			for k := 1; k+1 < len(poly); k++ {
				vol += tetra(poly[0], poly[k], poly[k+1])
			}
		}
	}
	return vol
}

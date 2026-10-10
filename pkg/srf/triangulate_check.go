package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// earMeshStrayTol is how far, in chord tolerances, a chord of the ear
// clipper's mesh may part from the image of its uv segment on the
// surface before the face is a candidate for mendCrossings.
const earMeshStrayTol = 4

// earMeshStrays tells whether the ear clipper's triangles of s, still in
// uv, may cross the faces beyond: one is folded against the others (its
// normal on the surface turned the other way to theirs), or a triangle's
// edges or centre, straight in xyz, lie further than earMeshStrayTol
// chord tolerances from where their uv segment's middle is on the
// surface. The trims are polygons in uv; a chord whose uv image bends
// that far from it may pass a trim point on the wrong side (roadmap K4),
// as the ear clipper's fans of long chords across a patch ruled one way
// or a trimmed corner do. Sound meshes stray as far (a sphere's patches,
// a loft's), so it only says which faces to look at.
func (s *SSurface) earMeshStrays(tris []STriangle) bool {
	tol := earMeshStrayTol * s.Tol.ChordTolMm()
	side := 0.0
	for i := range tris {
		t := &tris[i]
		a, b, c := t.A, t.B, t.C
		pa, pb, pc := s.PointAt(a.X, a.Y), s.PointAt(b.X, b.Y), s.PointAt(c.X, c.Y)
		mid := a.Plus(b).Plus(c).ScaledBy(1.0 / 3)
		uvArea := (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
		n := pb.Minus(pa).Cross(pc.Minus(pa))
		ns := s.NormalAt(mid.X, mid.Y)
		if math.Abs(uvArea) > 1e-12 && n.Magnitude() > geom.LengthEps*geom.LengthEps && ns.Magnitude() > 0 {
			sd := math.Copysign(1, n.Dot(ns)*uvArea)
			if side != 0 && sd != side {
				return true
			}
			side = sd
		}
		for _, q := range [][2]geom.Vector{
			{pa.Plus(pb).Plus(pc).ScaledBy(1.0 / 3), mid},
			{pa.Plus(pb).ScaledBy(0.5), a.Plus(b).ScaledBy(0.5)},
			{pb.Plus(pc).ScaledBy(0.5), b.Plus(c).ScaledBy(0.5)},
			{pc.Plus(pa).ScaledBy(0.5), c.Plus(a).ScaledBy(0.5)},
		} {
			if q[0].Minus(s.PointAt(q[1].X, q[1].Y)).Magnitude() > tol {
				return true
			}
		}
	}
	return false
}

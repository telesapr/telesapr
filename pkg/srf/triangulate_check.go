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

// earSliverShare is how thin, as a share of its longest side, a
// triangle of an ear-clipped mesh is before the face is a candidate for
// mendCrossings.
const earSliverShare = 1e-3

// earMeshSlivers tells whether a face's ear-clipped triangles, in xyz,
// hold a sliver: one far thinner than it is long. On a plane any ear is
// taken, and three points along a trim that runs straight in xyz, a
// fillet's foot along a flat face, make one; the points are off the
// line in their last digits, so the sliver's own plane is turned
// anyhow, and the face's edges beside its end cross it (a rib's foot
// filleted round the front corner where its slope leans). On a curved
// face a trim lying along a row of its grid, within a fraction of the
// row's spacing, leaves a strip the clipper fans from a far grid point
// to the trim's points: its long chords cut under the surface by its
// sag and pass through the face beyond the trim (a fillet's foot on a
// rod, along its length). Most slivers cross nothing, so it only says
// which faces to look at.
func earMeshSlivers(tris []STriangle) bool {
	for i := range tris {
		t := &tris[i]
		ab, bc, ca := t.B.Minus(t.A), t.C.Minus(t.B), t.A.Minus(t.C)
		long := max(ab.MagSquared(), bc.MagSquared(), ca.MagSquared())
		if long < geom.LengthEps*geom.LengthEps {
			continue
		}
		// Twice the area over the longest side is the height.
		if ab.Cross(ca).MagSquared() < earSliverShare*earSliverShare*long*long {
			return true
		}
	}
	return false
}

package srf

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// STriMeta is the colour/face tag attached to each STriangle. It is
// also used as the per-curve/per-surface handle carrier during shell
// boolean operations.
type STriMeta struct {
	Face FaceID
	core.RgbaColor
}

// NewSTriMeta constructs an STriMeta with the given face id and color.
func NewSTriMeta(face FaceID, color core.RgbaColor) STriMeta {
	return STriMeta{Face: face, RgbaColor: color}
}

// STriangle is one face of an SMesh. The (a, b, c) vertices are
// stored along with per-vertex normals (an, bn, cn) computed at
// triangulation time.
type STriangle struct {
	Tag int
	Meta STriMeta

	A, B, C    geom.Vector
	NA, NB, NC geom.Vector
}

func STriangleFrom(meta STriMeta, a, b, c geom.Vector) STriangle {
	t := STriangle{
		Meta: meta,
		A:    a,
		B:    b,
		C:    c,
	}
	t.computeNormals()
	return t
}

func (t *STriangle) computeNormals() {
	n := t.Normal()
	t.NA = n
	t.NB = n
	t.NC = n
}

// Normal returns the unweighted face normal (cross product, not
// normalized). Use NormalUnit for the normalized version.
func (t *STriangle) Normal() geom.Vector {
	return t.B.Minus(t.A).Cross(t.C.Minus(t.A))
}

// NormalUnit returns the normalized face normal.
func (t *STriangle) NormalUnit() geom.Vector {
	return t.Normal().Normal()
}

// FlipNormal reverses the vertex winding so the face normal inverts.
func (t *STriangle) FlipNormal() {
	t.A, t.B = t.B, t.A
	t.NA, t.NB = t.NB, t.NA
	t.AuxVSwap()
}

// AuxVSwap is a placeholder for any auxiliary vertex-tag inversion.
// In the C++ code it operates on auxA/auxB; in the Go port these are
// held outside the STriangle, so this is a no-op.
func (t *STriangle) AuxVSwap() {}

// MinAltitude returns the shortest altitude (distance from a vertex
// to the opposite edge). Used by SMesh::Simplify and by the polyline
// builder to skip near-degenerate triangles.
func (t *STriangle) MinAltitude() float64 {
	e1 := t.B.Minus(t.A).Magnitude()
	e2 := t.C.Minus(t.B).Magnitude()
	e3 := t.A.Minus(t.C).Magnitude()
	if e1 < 1e-20 || e2 < 1e-20 || e3 < 1e-20 {
		return 0
	}
	area := t.Area()
	return 2 * area / max3(e1, e2, e3)
}

func max3(a, b, c float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

// Area returns the unsigned area of the triangle (|cross|/2).
func (t *STriangle) Area() float64 {
	return 0.5 * t.Normal().Magnitude()
}

// SignedVolume returns the signed volume of the tetrahedron formed
// by the triangle and the origin.
func (t *STriangle) SignedVolume() float64 {
	return t.A.Dot(t.B.Cross(t.C)) / 6.0
}

// ContainsPoint returns true if p lies in the triangle's plane and
// inside or on the boundary of the triangle. Does not check plane
// containment first.
func (t *STriangle) ContainsPoint(p geom.Vector) bool {
	v0 := t.C.Minus(t.A)
	v1 := t.B.Minus(t.A)
	v2 := p.Minus(t.A)

	dot00 := v0.Dot(v0)
	dot01 := v0.Dot(v1)
	dot02 := v0.Dot(v2)
	dot11 := v1.Dot(v1)
	dot12 := v1.Dot(v2)

	den := dot00*dot11 - dot01*dot01
	if mathAbs(den) < 1e-20 {
		return false
	}
	u := (dot11*dot02 - dot01*dot12) / den
	v := (dot00*dot12 - dot01*dot02) / den

	return u >= 0 && v >= 0 && u+v <= 1
}

// IsDegenerate returns true if any two vertices coincide.
func (t *STriangle) IsDegenerate() bool {
	return t.A.DistanceTo(t.B) < 1e-12 ||
		t.B.DistanceTo(t.C) < 1e-12 ||
		t.A.DistanceTo(t.C) < 1e-12
}

// Transform applies a (translate, scale) transform and produces a
// new triangle. Rotation is intentionally not supported here; for
// rotated copies use SSurface::Transform.
func (t STriangle) Transform(o, u, v geom.Vector) STriangle {
	return STriangle{
		Tag:  t.Tag,
		Meta: t.Meta,
		A:    t.A.Plus(u.ScaledBy(o.X)).Plus(v.ScaledBy(o.Y)).Plus(o),
		B:    t.B.Plus(u.ScaledBy(o.X)).Plus(v.ScaledBy(o.Y)).Plus(o),
		C:    t.C.Plus(u.ScaledBy(o.X)).Plus(v.ScaledBy(o.Y)).Plus(o),
	}
}

// Raytrace computes the intersection of ray (point, dir) with the
// triangle's plane. The intersection is returned via t and inters;
// the ray hits the triangle if t > 0 and the hit point lies within
// the triangle in plane coords.
func (t *STriangle) Raytrace(point, dir geom.Vector) (tt float64, inters geom.Vector, ok bool) {
	n := t.Normal()
	den := dir.Dot(n)
	if mathAbs(den) < 1e-20 {
		return 0, geom.Vector{}, false
	}
	tt = t.A.Minus(point).Dot(n) / den
	if tt < 0 {
		return 0, geom.Vector{}, false
	}
	inters = point.Plus(dir.ScaledBy(tt))
	return tt, inters, t.ContainsPoint(inters)
}

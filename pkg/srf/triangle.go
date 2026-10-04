package srf

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
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
	Tag  int
	Meta STriMeta

	A, B, C    geom.Vector
	NA, NB, NC geom.Vector
}

// STriangleFrom is a triangle with the given vertices, and zero
// vertex normals (C++ STriangle::From).
func STriangleFrom(meta STriMeta, a, b, c geom.Vector) STriangle {
	return STriangle{Meta: meta, A: a, B: b, C: c}
}

// Normal is the triangle's unnormalized normal (C++ STriangle::Normal).
func (t *STriangle) Normal() geom.Vector {
	ab, bc := t.B.Minus(t.A), t.C.Minus(t.B)
	return ab.Cross(bc)
}

// FlipNormal swaps a and b (C++ STriangle::FlipNormal).
func (t *STriangle) FlipNormal() {
	t.A, t.B = t.B, t.A
	t.NA, t.NB = t.NB, t.NA
}

// MinAltitude is the triangle's smallest altitude (C++
// STriangle::MinAltitude).
func (t *STriangle) MinAltitude() float64 {
	altA := t.A.DistanceToLine(t.B, t.C.Minus(t.B))
	altB := t.B.DistanceToLine(t.C, t.A.Minus(t.C))
	altC := t.C.DistanceToLine(t.A, t.B.Minus(t.A))

	return geom.Min(altA, geom.Min(altB, altC))
}

// ContainsPoint reports whether p, projected along the normal, lies in
// the triangle; never for a zero-area triangle (C++
// STriangle::ContainsPoint).
func (t *STriangle) ContainsPoint(p geom.Vector) bool {
	n := t.Normal()
	if t.MinAltitude() < geom.LengthEps {
		// shouldn't happen; zero-area triangle
		return false
	}
	return t.ContainsPointProjd(n.WithMagnitude(1), p)
}

// ContainsPointProjd reports whether p, projected along n, lies in the
// triangle (edges included; C++ STriangle::ContainsPointProjd).
func (t *STriangle) ContainsPointProjd(n, p geom.Vector) bool {
	ab, bc, ca := t.B.Minus(t.A), t.C.Minus(t.B), t.A.Minus(t.C)

	noAB := n.Cross(ab)
	if noAB.Dot(p) < noAB.Dot(t.A)-geom.LengthEps {
		return false
	}

	noBC := n.Cross(bc)
	if noBC.Dot(p) < noBC.Dot(t.B)-geom.LengthEps {
		return false
	}

	noCA := n.Cross(ca)
	// Not >=, which a NaN would fail.
	return !(noCA.Dot(p) < noCA.Dot(t.C)-geom.LengthEps)
}

// containsPointProjdWithin is ContainsPointProjd with its tolerance a
// distance, LengthEps, from the triangle's sides: ContainsPointProjd's
// is LengthEps over the product of n's length and a side's, which for
// a triangle a hundredth of a millimetre across lets in points a
// hundredth of its size outside it.
func (t *STriangle) containsPointProjdWithin(n, p geom.Vector) bool {
	for _, side := range [3][2]geom.Vector{{t.A, t.B}, {t.B, t.C}, {t.C, t.A}} {
		out := n.Cross(side[1].Minus(side[0]))
		if m := out.Magnitude(); m > 0 && out.Dot(p.Minus(side[0])) < -geom.LengthEps*m {
			return false
		}
	}
	return true
}

// edgeCrossesWithin reports whether the segment from a to b, whose ends
// lie on either side of the triangle's plane (unit normal nn, offset d),
// crosses the plane within LengthEps of the triangle, measured in the
// plane at the crossing point. containsPointProjdWithin along the
// segment measures that distance across the segment instead, which for
// a segment nearly in the triangle's plane is the in-plane distance
// times the sine of their angle: a long edge of a flat face passing a
// sliver of the next face, all but in its plane, was taken to cross it
// millimetres away (NIST_MBE_PMI_5 and _6). The mesh checker asks both,
// so that it counts no crossing it didn't before (the boxes of
// translate_asy, which do cross, keep C++'s count).
func (t *STriangle) edgeCrossesWithin(nn geom.Vector, d float64, a, b geom.Vector) bool {
	p := geom.AtIntersectionOfPlaneAndLine(nn, d, a, b)
	return t.containsPointProjdWithin(nn, p)
}

// Raytrace intersects the ray rayPoint + t*rayDir with the triangle,
// which must face against the ray (C++ STriangle::Raytrace; the
// Moeller-Trumbore algorithm).
func (t *STriangle) Raytrace(rayPoint, rayDir geom.Vector) (tt float64, inters geom.Vector, ok bool) {
	// Find vectors for two edges sharing vertex A.
	edge1 := t.B.Minus(t.A)
	edge2 := t.C.Minus(t.A)

	// Begin calculating determinant - also used to calculate U parameter.
	pvec := rayDir.Cross(edge2)

	// If determinant is near zero, ray lies in plane of triangle.
	// Also, cull back facing triangles here.
	det := edge1.Dot(pvec)
	if -det < geom.LengthEps {
		return 0, geom.Vector{}, false
	}
	invDet := 1.0 / det

	// Calculate distance from vertex A to ray origin.
	tvec := rayPoint.Minus(t.A)

	// Calculate U parameter and test bounds.
	u := tvec.Dot(pvec) * invDet
	if u < 0 || u > 1 {
		return 0, geom.Vector{}, false
	}

	// Prepare to test V parameter.
	qvec := tvec.Cross(edge1)

	// Calculate V parameter and test bounds.
	v := rayDir.Dot(qvec) * invDet
	if v < 0 || u+v > 1 {
		return 0, geom.Vector{}, false
	}

	// Calculate t, ray intersects triangle.
	tt = edge2.Dot(qvec) * invDet

	// Calculate intersection point.
	return tt, rayPoint.Plus(rayDir.ScaledBy(tt)), true
}

// SignedVolume is the signed volume of the tetrahedron of the triangle
// and the origin (C++ STriangle::SignedVolume).
func (t *STriangle) SignedVolume() float64 {
	return t.A.Dot(t.B.Cross(t.C)) / 6.0
}

// Area is the triangle's area (C++ STriangle::Area).
func (t *STriangle) Area() float64 {
	ab := t.A.Minus(t.B)
	cb := t.C.Minus(t.B)
	return ab.Cross(cb).Magnitude() / 2.0
}

// IsDegenerate reports whether a vertex lies on the opposite edge (C++
// STriangle::IsDegenerate).
func (t *STriangle) IsDegenerate() bool {
	return t.A.OnLineSegment(t.B, t.C, geom.LengthEps) ||
		t.B.OnLineSegment(t.A, t.C, geom.LengthEps) ||
		t.C.OnLineSegment(t.A, t.B, geom.LengthEps)
}

// Transform takes every vertex and normal as coordinates in the
// (u, v, n) basis (C++ STriangle::Transform).
func (t STriangle) Transform(u, v, n geom.Vector) STriangle {
	tr := t
	tr.A = tr.A.ScaleOutOfCsys(u, v, n)
	tr.NA = tr.NA.ScaleOutOfCsys(u, v, n)
	tr.B = tr.B.ScaleOutOfCsys(u, v, n)
	tr.NB = tr.NB.ScaleOutOfCsys(u, v, n)
	tr.C = tr.C.ScaleOutOfCsys(u, v, n)
	tr.NC = tr.NC.ScaleOutOfCsys(u, v, n)
	return tr
}

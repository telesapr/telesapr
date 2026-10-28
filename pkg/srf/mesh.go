package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// This file ports the SMesh part of solvespace/src/mesh.cpp: operations
// on triangle meshes, like the mesh Booleans using the BSP.

// SMesh is a list of triangles, with the flags the Booleans use to pick
// which triangles to keep.
type SMesh struct {
	Triangles []STriangle

	FlipNormal           bool
	KeepInsideOtherShell bool
	KeepCoplanar         bool
	AtLeastOneDiscarded  bool
	IsTransparent        bool
}

// Clear drops the triangles; it keeps the flags.
func (m *SMesh) Clear() {
	m.Triangles = m.Triangles[:0]
}

func (m *SMesh) IsEmpty() bool { return len(m.Triangles) == 0 }

// AddTriangleN adds the triangle a, b, c, wound so its normal points
// along n; a (nearly) collinear one is dropped.
func (m *SMesh) AddTriangleN(meta STriMeta, n, a, b, c geom.Vector) {
	ab, bc := b.Minus(a), c.Minus(b)
	np := ab.Cross(bc)
	if np.Magnitude() < 1e-10 {
		// ugh; gl sometimes tessellates to collinear triangles
		return
	}
	if np.Dot(n) > 0 {
		m.AddTriangle3(meta, a, b, c)
	} else {
		m.AddTriangle3(meta, c, b, a)
	}
}

// AddTriangle3 adds the triangle a, b, c.
func (m *SMesh) AddTriangle3(meta STriMeta, a, b, c geom.Vector) {
	m.AddTriangle(STriangle{Meta: meta, A: a, B: b, C: c})
}

// AddTriangle adds a copy of t.
func (m *SMesh) AddTriangle(t STriangle) {
	m.Triangles = append(m.Triangles, t)
}

// DoBounding extends (vmax, vmin) to include v.
func DoBounding(v geom.Vector, vmax, vmin *geom.Vector) {
	vmax.X = geom.Max(vmax.X, v.X)
	vmax.Y = geom.Max(vmax.Y, v.Y)
	vmax.Z = geom.Max(vmax.Z, v.Z)

	vmin.X = geom.Min(vmin.X, v.X)
	vmin.Y = geom.Min(vmin.Y, v.Y)
	vmin.Z = geom.Min(vmin.Z, v.Z)
}

// Bounding is the mesh's bounding box.
func (m *SMesh) Bounding() (vmax, vmin geom.Vector) {
	vmin = geom.Vector{X: veryPositive, Y: veryPositive, Z: veryPositive}
	vmax = geom.Vector{X: veryNegative, Y: veryNegative, Z: veryNegative}
	for i := range m.Triangles {
		st := &m.Triangles[i]
		DoBounding(st.A, &vmax, &vmin)
		DoBounding(st.B, &vmax, &vmin)
		DoBounding(st.C, &vmax, &vmin)
	}
	return vmax, vmin
}

// MakeEdgesInPlaneInto reports the edges of the boundary of the
// region(s) of the mesh that lie within the plane n. p = d.
func (m *SMesh) MakeEdgesInPlaneInto(sel *SEdgeList, n geom.Vector, d float64) {
	var mc SMesh
	mc.MakeFromCopyOf(m)

	// Delete all triangles in the mesh that do not lie in our export plane.
	kept := mc.Triangles[:0]
	for _, tr := range mc.Triangles {
		if math.Abs(n.Dot(tr.A)-d) >= geom.LengthEps ||
			math.Abs(n.Dot(tr.B)-d) >= geom.LengthEps ||
			math.Abs(n.Dot(tr.C)-d) >= geom.LengthEps {
			continue
		}
		tr.Tag = 0
		kept = append(kept, tr)
	}
	mc.Triangles = kept

	// Select the naked edges in our resulting open mesh.
	root := SKdNodeFrom(&mc)
	root.SnapToMesh(&mc)
	root.MakeCertainEdgesInto(sel, EdgeNakedOrSelfInter, false, nil, nil, 0)
}

// Simplify replaces the triangles from start on, which must be
// coplanar, with fewer ones covering the same area: it pastes them
// together into convex polygons, merging collinear edges where it can,
// and triangulates those. It is used after a triangle has been split
// against the BSP.
func (m *SMesh) Simplify(start int) {
	meta := m.Triangles[start].Meta

	var tout []STriangle

	var n geom.Vector
	var conv []geom.Vector

	start0 := start

	l := m.Triangles
	for i := start; i < len(l); i++ {
		tr := &l[i]
		if tr.MinAltitude() < geom.LengthEps {
			tr.Tag = 1
		} else {
			tr.Tag = 0
		}
	}

	wrap := func(v, n int) int { return ((v % n) + n) % n }

	for {
		conv = conv[:0]
		i := start
		for ; i < len(l); i++ {
			tr := &l[i]
			if tr.Tag != 0 {
				continue
			}

			tr.Tag = 1
			n = tr.Normal().WithMagnitude(1)
			conv = append(conv, tr.A, tr.B, tr.C)

			start = i + 1
			break
		}
		if i >= len(l) {
			break
		}

		for {
			didAdd := false

			for j := 0; j < len(conv); j++ {
				convc := len(conv)
				a := conv[wrap(j-1, convc)]
				b := conv[j]
				d := conv[wrap(j+1, convc)]
				e := conv[wrap(j+2, convc)]

				var c geom.Vector
				for i = start; i < len(l); i++ {
					tr := &l[i]
					if tr.Tag != 0 {
						continue
					}

					switch {
					case tr.A.Equals(d) && tr.B.Equals(b):
						c = tr.C

					case tr.B.Equals(d) && tr.C.Equals(b):
						c = tr.A

					case tr.C.Equals(d) && tr.A.Equals(b):
						c = tr.B

					default:
						continue
					}
					// The vertex at C must be convex; but the others must
					// be tested
					ab := b.Minus(a)
					bc := c.Minus(b)
					cd := d.Minus(c)
					de := e.Minus(d)

					bDot := ab.Cross(bc).Dot(n)
					dDot := cd.Cross(de).Dot(n)

					bDot /= geom.Min(ab.Magnitude(), bc.Magnitude())
					dDot /= geom.Min(cd.Magnitude(), de.Magnitude())

					switch {
					case math.Abs(bDot) < geom.LengthEps && math.Abs(dDot) < geom.LengthEps:
						conv[wrap(j+1, convc)] = c
						// and remove the vertex at j, which is a dup
						conv = append(conv[:j], conv[j+1:]...)

					case math.Abs(bDot) < geom.LengthEps && dDot > 0:
						conv[j] = c

					case math.Abs(dDot) < geom.LengthEps && bDot > 0:
						conv[wrap(j+1, convc)] = c

					case bDot > 0 && dDot > 0:
						// conv[j] is unchanged, conv[j+1] goes to [j+2]
						conv = append(conv[:j+1], append([]geom.Vector{c}, conv[j+1:]...)...)

					default:
						continue
					}

					didAdd = true
					tr.Tag = 1
					break
				}
			}
			if !didAdd {
				break
			}
		}

		// I need to debug why this is required; sometimes the above code
		// still generates a convex polygon
		convc := len(conv)
		for i := range convc {
			a := conv[wrap(i-1, convc)]
			b := conv[i]
			c := conv[wrap(i+1, convc)]
			ab := b.Minus(a)
			bc := c.Minus(b)
			bDot := ab.Cross(bc).Dot(n)
			bDot /= geom.Min(ab.Magnitude(), bc.Magnitude())

			if bDot < 0 {
				return // XXX, shouldn't happen
			}
		}

		for i := range convc - 2 {
			tr := STriangleFrom(meta, conv[0], conv[i+1], conv[i+2])
			if tr.MinAltitude() > geom.LengthEps {
				tout = append(tout, tr)
			}
		}
	}

	m.Triangles = m.Triangles[:start0]
	for i := range tout {
		m.AddTriangle(tout[i])
	}
}

// AddAgainstBsp adds the triangles of srcm that the Boolean keeps
// against bsp3, as the mesh's flags say, split where they cross the
// other mesh.
func (m *SMesh) AddAgainstBsp(srcm *SMesh, bsp3 *SBsp3) {
	var sc bspScratch
	for i := range srcm.Triangles {
		st := &srcm.Triangles[i]
		pn := len(m.Triangles)
		m.AtLeastOneDiscarded = false
		insertOrCreate(bsp3, st, m, &sc, 0)
		if !m.AtLeastOneDiscarded && len(m.Triangles) != pn+1 {
			m.Triangles = m.Triangles[:pn]
			if m.FlipNormal {
				m.AddTriangle3(st.Meta, st.C, st.B, st.A)
			} else {
				m.AddTriangle3(st.Meta, st.A, st.B, st.C)
			}
		}
		if len(m.Triangles)-pn > 1 {
			m.Simplify(pn)
		}
	}
}

// MakeFromUnionOf adds the union of a and b.
func (m *SMesh) MakeFromUnionOf(a, b *SMesh) {
	bspa := SBsp3FromMesh(a)
	bspb := SBsp3FromMesh(b)

	m.FlipNormal = false
	m.KeepInsideOtherShell = false

	m.KeepCoplanar = true
	m.AddAgainstBsp(b, bspa)

	m.KeepCoplanar = false
	m.AddAgainstBsp(a, bspb)
}

// MakeFromDifferenceOf adds a minus b.
func (m *SMesh) MakeFromDifferenceOf(a, b *SMesh) {
	bspa := SBsp3FromMesh(a)
	bspb := SBsp3FromMesh(b)

	m.FlipNormal = true
	m.KeepCoplanar = true
	m.KeepInsideOtherShell = true
	m.AddAgainstBsp(b, bspa)

	m.FlipNormal = false
	m.KeepCoplanar = false
	m.KeepInsideOtherShell = false
	m.AddAgainstBsp(a, bspb)
}

// MakeFromIntersectionOf adds the intersection of a and b.
func (m *SMesh) MakeFromIntersectionOf(a, b *SMesh) {
	bspa := SBsp3FromMesh(a)
	bspb := SBsp3FromMesh(b)

	m.KeepInsideOtherShell = true
	m.FlipNormal = false

	m.KeepCoplanar = false
	m.AddAgainstBsp(a, bspb)

	m.KeepCoplanar = true
	m.AddAgainstBsp(b, bspa)
}

// MakeFromCopyOf adds a's triangles.
func (m *SMesh) MakeFromCopyOf(a *SMesh) {
	if m == a {
		panic("Can't make from copy of self")
	}
	m.Triangles = append(m.Triangles, a.Triangles...)
}

// MakeFromAssemblyOf adds the triangles of a and of b.
func (m *SMesh) MakeFromAssemblyOf(a, b *SMesh) {
	m.MakeFromCopyOf(a)
	m.MakeFromCopyOf(b)
}

// MakeFromTransformationOf adds a's triangles, scaled, then rotated,
// then translated; a mirroring scale keeps the mesh outside out.
func (m *SMesh) MakeFromTransformationOf(a *SMesh, trans geom.Vector, q geom.Quaternion, scale float64) {
	for _, tt := range a.Triangles {
		tt.A = tt.A.ScaledBy(scale)
		tt.B = tt.B.ScaledBy(scale)
		tt.C = tt.C.ScaledBy(scale)
		if scale < 0 {
			// The mirroring would otherwise turn a closed mesh inside out.
			tt.A, tt.B = tt.B, tt.A
		}
		tt.A = q.Rotate(tt.A).Plus(trans)
		tt.B = q.Rotate(tt.B).Plus(trans)
		tt.C = q.Rotate(tt.C).Plus(trans)
		m.AddTriangle(tt)
	}
}

// FirstIntersectionWith is the face of the farthest-along triangle,
// with a face, that the ray rayPoint + t*rayDir hits; 0 if none. The
// caller makes the ray from the view.
func (m *SMesh) FirstIntersectionWith(rayPoint, rayDir geom.Vector) uint32 {
	return m.FirstIntersectionIn(rayPoint, rayDir, nil)
}

// FirstIntersectionIn is FirstIntersectionWith counting only hits in
// the half space h, if it isn't nil: a face cut away by a section
// isn't there to hit.
func (m *SMesh) FirstIntersectionIn(rayPoint, rayDir geom.Vector, h *HalfSpace) uint32 {
	face, _ := m.FirstHitIn(rayPoint, rayDir, h)
	return face
}

// FirstHitIn is FirstIntersectionIn and the point where the ray meets
// that face, as a point picked on the face.
func (m *SMesh) FirstHitIn(rayPoint, rayDir geom.Vector, h *HalfSpace) (uint32, geom.Vector) {
	var face uint32
	var hit geom.Vector
	faceT := veryNegative
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		if tr.Meta.Face == 0 {
			continue
		}

		t, at, ok := tr.Raytrace(rayPoint, rayDir)
		if !ok {
			continue
		}
		if h != nil && !h.Contains(at) {
			continue
		}
		if t > faceT {
			face, hit = uint32(tr.Meta.Face), at
			faceT = t
		}
	}

	return face, hit
}

// CenterOfMass is the centroid of the volume the mesh encloses.
func (m *SMesh) CenterOfMass() geom.Vector {
	center := geom.Vector{}
	vol := 0.0
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		tvol := tr.SignedVolume()
		center = center.Plus(tr.A.Plus(tr.B.Plus(tr.C)).ScaledBy(tvol / 4.0))
		vol += tvol
	}
	return center.ScaledBy(1.0 / vol)
}

// PrecomputeTransparency sorts the opaque triangles before the
// transparent ones, for drawing, in std::sort's order; and notes
// whether the comparisons met a transparent triangle.
func (m *SMesh) PrecomputeTransparency() {
	CxxSort(m.Triangles, func(sta, stb *STriangle) bool {
		colora, colorb := sta.Meta.RGBA, stb.Meta.RGBA
		opaquea := colora.IsEmpty() || colora.Alpha == 255
		opaqueb := colorb.IsEmpty() || colorb.Alpha == 255

		if !opaquea || !opaqueb {
			m.IsTransparent = true
		}
		return opaquea != opaqueb && opaquea
	})
}

// RemoveDegenerateTriangles drops the degenerate triangles.
func (m *SMesh) RemoveDegenerateTriangles() {
	out := m.Triangles[:0]
	for _, tr := range m.Triangles {
		if tr.IsDegenerate() {
			continue
		}
		tr.Tag = 0
		out = append(out, tr)
	}
	m.Triangles = out
}

// CalculateVolume is the volume the mesh encloses, integrated under
// each triangle.
func (m *SMesh) CalculateVolume() float64 {
	vol := 0.0
	for _, tr := range m.Triangles {
		// Translate to place vertex A at (x, y, 0)
		trans := geom.Vector{X: tr.A.X, Y: tr.A.Y}
		tr.A = tr.A.Minus(trans)
		tr.B = tr.B.Minus(trans)
		tr.C = tr.C.Minus(trans)

		// Rotate to place vertex B on the y-axis. Depending on
		// whether the triangle is CW or CCW, C is either to the
		// right or to the left of the y-axis. This handles the
		// sign of our normal.
		u := geom.Vector{X: -tr.B.Y, Y: tr.B.X}
		u = u.WithMagnitude(1)
		v := geom.Vector{X: tr.B.X, Y: tr.B.Y}
		v = v.WithMagnitude(1)
		n := geom.Vector{Z: 1}

		tr.A = tr.A.DotInToCsys(u, v, n)
		tr.B = tr.B.DotInToCsys(u, v, n)
		tr.C = tr.C.DotInToCsys(u, v, n)

		n = tr.Normal().WithMagnitude(1)

		// Triangles on edge don't contribute
		if math.Abs(n.Z) < geom.LengthEps {
			continue
		}

		// The plane has equation p dot n = a dot n
		d := tr.A.Dot(n)
		// nx*x + ny*y + nz*z = d
		// nz*z = d - nx*x - ny*y
		A, B, C := -n.X/n.Z, -n.Y/n.Z, d/n.Z

		mac, mbc := tr.C.Y/tr.C.X, (tr.C.Y-tr.B.Y)/tr.C.X
		xc, yb := tr.C.X, tr.B.Y

		// I asked Maple for
		//    int(int(A*x + B*y +C, y=mac*x..(mbc*x + yb)), x=0..xc);
		integral :=
			(1.0/3)*(A*(mbc-mac)+
				(1.0/2)*B*(mbc*mbc-mac*mac))*(xc*xc*xc) +
				(1.0/2)*(A*yb+B*yb*mbc+C*(mbc-mac))*xc*xc +
				C*yb*xc +
				(1.0/2)*B*yb*yb*xc

		vol += integral
	}
	return vol
}

// CalculateSurfaceArea is the area of the triangles on the given faces.
func (m *SMesh) CalculateSurfaceArea(faces []uint32) float64 {
	area := 0.0
	for _, f := range faces {
		for i := range m.Triangles {
			t := &m.Triangles[i]
			if f != uint32(t.Meta.Face) {
				continue
			}
			area += t.Area()
		}
	}
	return area
}

// HalfSpace is the points p with N.p <= D: what a section plane keeps,
// its normal N pointing to the side cut away.
type HalfSpace struct {
	N geom.Vector
	D float64
}

// Contains reports whether p is in the half space, or within
// LengthEps of its plane.
func (h *HalfSpace) Contains(p geom.Vector) bool { return h.N.Dot(p) <= h.D+geom.LengthEps }

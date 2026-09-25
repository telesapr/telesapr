package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SMesh is a list of triangles with mesh-level flags controlling
// rendering and CSG behavior. Boolean operations
// (MakeFromUnionOf/DifferenceOf/IntersectionOf) and BSP-driven
// insertion (AddAgainstBsp) are implemented in Phase 4.
type SMesh struct {
	Triangles []STriangle

	FlipNormal          bool
	KeepInsideOtherShell bool
	KeepCoplanar        bool
	AtLeastOneDiscarded  bool
	IsTransparent       bool
}

func (m *SMesh) Clear() {
	m.Triangles = m.Triangles[:0]
	m.FlipNormal = false
	m.KeepInsideOtherShell = false
	m.KeepCoplanar = false
	m.AtLeastOneDiscarded = false
	m.IsTransparent = false
}

func (m *SMesh) IsEmpty() bool { return len(m.Triangles) == 0 }

// AddTriangle appends one triangle to the mesh.
func (m *SMesh) AddTriangle(t STriangle) {
	m.Triangles = append(m.Triangles, t)
}

// AddTriangle3 is a convenience constructor for (meta, a, b, c).
func (m *SMesh) AddTriangle3(meta STriMeta, a, b, c geom.Vector) {
	m.AddTriangle(STriangleFrom(meta, a, b, c))
}

// GetBounding returns the axis-aligned bounding box of all triangle
// vertices. If the mesh is empty, the box is empty (max < min).
func (m *SMesh) GetBounding() (vmax, vmin geom.Vector) {
	vmin = geom.Vector{X: 1e18, Y: 1e18, Z: 1e18}
	vmax = geom.Vector{X: -1e18, Y: -1e18, Z: -1e18}
	for _, t := range m.Triangles {
		for _, p := range [3]geom.Vector{t.A, t.B, t.C} {
			if p.X < vmin.X {
				vmin.X = p.X
			}
			if p.Y < vmin.Y {
				vmin.Y = p.Y
			}
			if p.Z < vmin.Z {
				vmin.Z = p.Z
			}
			if p.X > vmax.X {
				vmax.X = p.X
			}
			if p.Y > vmax.Y {
				vmax.Y = p.Y
			}
			if p.Z > vmax.Z {
				vmax.Z = p.Z
			}
		}
	}
	return vmax, vmin
}

// DoBounding accumulates one vertex into a running bounding box.
func DoBounding(b geom.Vector, vmax, vmin *geom.Vector) {
	if b.X < vmin.X {
		vmin.X = b.X
	}
	if b.Y < vmin.Y {
		vmin.Y = b.Y
	}
	if b.Z < vmin.Z {
		vmin.Z = b.Z
	}
	if b.X > vmax.X {
		vmax.X = b.X
	}
	if b.Y > vmax.Y {
		vmax.Y = b.Y
	}
	if b.Z > vmax.Z {
		vmax.Z = b.Z
	}
}

// RemoveDegenerateTriangles drops triangles whose minimum altitude
// is below eps. Used after shelling/trimming to clean up near-zero
// areas that the boolean produced.
func (m *SMesh) RemoveDegenerateTriangles(eps float64) {
	out := m.Triangles[:0]
	for _, t := range m.Triangles {
		if t.IsDegenerate() {
			continue
		}
		if t.MinAltitude() < eps {
			continue
		}
		out = append(out, t)
	}
	m.Triangles = out
}

// PrecomputeTransparency sets IsTransparent if any triangle's color
// has alpha < 255. Used by the renderer to switch blending modes.
func (m *SMesh) PrecomputeTransparency() {
	m.IsTransparent = false
	for _, t := range m.Triangles {
		if t.Meta.Alpha < 255 {
			m.IsTransparent = true
			return
		}
	}
}

// Simplify drops triangles with minimum altitude < chordTol.
// Equivalent to RemoveDegenerateTriangles(chordTol) but called
// after chord-tolerance-based mesh simplification passes.
func (m *SMesh) Simplify(chordTol float64) {
	m.RemoveDegenerateTriangles(chordTol)
}

// MakeFromCopyOf clones another mesh (with flags).
func (m *SMesh) MakeFromCopyOf(src *SMesh) {
	m.Clear()
	m.FlipNormal = src.FlipNormal
	m.KeepInsideOtherShell = src.KeepInsideOtherShell
	m.KeepCoplanar = src.KeepCoplanar
	m.AtLeastOneDiscarded = src.AtLeastOneDiscarded
	m.IsTransparent = src.IsTransparent
	m.Triangles = make([]STriangle, len(src.Triangles))
	copy(m.Triangles, src.Triangles)
}

// MakeFromTransformationOf applies (translate, rotate, scale) to
// every vertex of src. Quaternion::Rotate handles the rotation;
// the scale is applied uniformly.
func (m *SMesh) MakeFromTransformationOf(src *SMesh, trans geom.Vector, q geom.Quaternion, scale float64) {
	m.Clear()
	m.FlipNormal = src.FlipNormal
	m.KeepInsideOtherShell = src.KeepInsideOtherShell
	m.KeepCoplanar = src.KeepCoplanar
	m.AtLeastOneDiscarded = src.AtLeastOneDiscarded
	m.IsTransparent = src.IsTransparent
	m.Triangles = make([]STriangle, len(src.Triangles))
	for i, t := range src.Triangles {
		newT := STriangle{
			Tag:  t.Tag,
			Meta: t.Meta,
		}
		newT.A = q.Rotate(t.A.ScaledBy(scale)).Plus(trans)
		newT.B = q.Rotate(t.B.ScaledBy(scale)).Plus(trans)
		newT.C = q.Rotate(t.C.ScaledBy(scale)).Plus(trans)
		newT.NA = q.Rotate(t.NA)
		newT.NB = q.Rotate(t.NB)
		newT.NC = q.Rotate(t.NC)
		m.Triangles[i] = newT
	}
}

// MakeEdgesInPlaneInto emits an SEdge for every triangle whose
// normal is parallel to the given plane normal n. Used by
// SSurface::MakeEdgesInto to recover trim edges from a triangulated
// mesh.
func (m *SMesh) MakeEdgesInPlaneInto(sel *SEdgeList, n geom.Vector, d float64) {
	for _, t := range m.Triangles {
		// Check the plane equation n.x = d.
		if mathAbs(t.A.Dot(n)-d) > 1e-9 {
			continue
		}
		if mathAbs(t.B.Dot(n)-d) > 1e-9 {
			continue
		}
		if mathAbs(t.C.Dot(n)-d) > 1e-9 {
			continue
		}
		sel.AddEdge(t.A, t.B, int(t.Meta.Face), 0, t.Tag)
		sel.AddEdge(t.B, t.C, int(t.Meta.Face), 0, t.Tag)
		sel.AddEdge(t.C, t.A, int(t.Meta.Face), 0, t.Tag)
	}
}

// CalculateVolume returns the unsigned volume enclosed by the mesh,
// treating it as a closed triangle soup. The signed-volume sum
// divided by 6 gives the net enclosed volume; we return the absolute
// value.
func (m *SMesh) CalculateVolume() float64 {
	v := 0.0
	for _, t := range m.Triangles {
		v += t.SignedVolume()
	}
	if v < 0 {
		v = -v
	}
	return v
}

// CalculateSurfaceArea sums triangle areas, optionally restricted to
// a subset of faces.
func (m *SMesh) CalculateSurfaceArea(faces []uint32) float64 {
	if faces == nil {
		sum := 0.0
		for _, t := range m.Triangles {
			sum += t.Area()
		}
		return sum
	}
	allowed := make(map[uint32]bool, len(faces))
	for _, f := range faces {
		allowed[f] = true
	}
	sum := 0.0
	for _, t := range m.Triangles {
		if allowed[uint32(t.Meta.Face)] {
			sum += t.Area()
		}
	}
	return sum
}

// GetCenterOfMass returns the centroid of all triangle vertices.
// For a closed mesh, the volume-weighted centroid is more accurate;
// this simple average is the surface centroid.
func (m *SMesh) GetCenterOfMass() geom.Vector {
	if len(m.Triangles) == 0 {
		return geom.Vector{}
	}
	sum := geom.Vector{}
	n := 0
	for _, t := range m.Triangles {
		sum = sum.Plus(t.A).Plus(t.B).Plus(t.C)
		n += 3
	}
	return sum.ScaledBy(1.0 / float64(n))
}

// MakeFromUnionOf produces A ∪ B by ray-cast classifying each
// triangle of a and b against the other's BSP. Goal Item 5:
// uses SBsp3 for proper inside/outside classification. Triangles
// outside the other mesh are kept; inside are dropped.
func (m *SMesh) MakeFromUnionOf(a, b *SMesh) {
	m.Clear()
	if a != nil && b != nil && len(a.Triangles) > 0 && len(b.Triangles) > 0 {
		bspA := SBsp3FromMesh(b)
		bspB := SBsp3FromMesh(a)
		// Keep a-triangles outside b.
		for _, t := range a.Triangles {
			if !triangleClassifiedInside(t, bspA) {
				m.Triangles = append(m.Triangles, t)
			}
		}
		for _, t := range b.Triangles {
			if !triangleClassifiedInside(t, bspB) {
				m.Triangles = append(m.Triangles, t)
			}
		}
	} else {
		m.MakeFromAssemblyOf(a, b)
	}
}

// MakeFromAssemblyOf is a non-CSG assembly: triangles from both a
// and b are concatenated. Used for step-and-repeat patterns.
func (m *SMesh) MakeFromAssemblyOf(a, b *SMesh) {
	m.Clear()
	if a != nil {
		m.Triangles = append(m.Triangles, a.Triangles...)
		m.FlipNormal = a.FlipNormal
		m.KeepInsideOtherShell = a.KeepInsideOtherShell
		m.KeepCoplanar = a.KeepCoplanar
		m.AtLeastOneDiscarded = a.AtLeastOneDiscarded
		m.IsTransparent = a.IsTransparent
	}
	if b != nil {
		m.Triangles = append(m.Triangles, b.Triangles...)
	}
}

// MakeFromDifferenceOf produces A - B by ray-cast: triangles in a
// whose centroid is OUTSIDE b are kept; triangles INSIDE b are
// dropped. Goal Item 5: uses SBsp3 for proper classification.
func (m *SMesh) MakeFromDifferenceOf(a, b *SMesh) {
	m.Clear()
	if a == nil {
		return
	}
	if b == nil || len(b.Triangles) == 0 {
		m.MakeFromCopyOf(a)
		return
	}
	bsp := SBsp3FromMesh(b)
	for _, t := range a.Triangles {
		if !triangleClassifiedInside(t, bsp) {
			m.Triangles = append(m.Triangles, t)
		}
	}
}

// MakeFromIntersectionOf produces A ∩ B by ray-cast: triangles in a
// whose centroid is INSIDE b are kept. Goal Item 5: uses SBsp3.
func (m *SMesh) MakeFromIntersectionOf(a, b *SMesh) {
	m.Clear()
	if a == nil || b == nil {
		return
	}
	bsp := SBsp3FromMesh(b)
	for _, t := range a.Triangles {
		if triangleClassifiedInside(t, bsp) {
			m.Triangles = append(m.Triangles, t)
		}
	}
}

// triangleClassifiedInside returns true if the centroid of t is
// inside the BSP mesh (ray-cast odd count).
func triangleClassifiedInside(t STriangle, bsp *SBsp3) bool {
	c := t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3.0)
	// Ray-cast in the +X direction. Use a non-axis direction so we
	// don't graze the plane of any triangle.
	dir := geom.Vector{X: 1, Y: 0.3141593, Z: 0.2718282}
	count := 0
	for _, u := range bsp.collectTriangles() {
		if _, _, ok := u.Raytrace(c, dir); ok {
			count++
		}
	}
	return count%2 == 1
}

// collectTriangles flattens the BSP into a triangle slice. Used
// for point-in-mesh ray-cast.
func (b *SBsp3) collectTriangles() []STriangle {
	if b == nil {
		return nil
	}
	out := []STriangle{b.Tri}
	for n := b.More; n != nil; n = n.More {
		out = append(out, n.Tri)
	}
	if b.Pos != nil {
		out = append(out, b.Pos.collectTriangles()...)
	}
	if b.Neg != nil {
		out = append(out, b.Neg.collectTriangles()...)
	}
	return out
}

// pointInsideMesh does a ray-cast from p in the +X direction and
// counts triangle intersections. Odd = inside.
func pointInsideMesh(p geom.Vector, m *SMesh) bool {
	count := 0
	for _, t := range m.Triangles {
		if _, _, ok := t.Raytrace(p, geom.Vector{X: 1}); ok {
			count++
		}
	}
	return count%2 == 1
}

// AddAgainstBsp adds triangles from srcm to m, classifying each by
// the BSP bsp3. Triangles inside the BSP get dropped; outside are
// kept. Used by SShell boolean operations to subtract one mesh from
// another after BSP classification.
func (m *SMesh) AddAgainstBsp(srcm *SMesh, bsp3 *SBsp3) {
	if srcm == nil {
		return
	}
	for _, t := range srcm.Triangles {
		c := t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3.0)
		cls := classifyPointAgainstBsp(c, bsp3)
		if cls != BspNeg {
			m.Triangles = append(m.Triangles, t)
		}
	}
}

func classifyPointAgainstBsp(p geom.Vector, b *SBsp3) BspClass {
	if b == nil {
		return BspCoplanar
	}
	c := ClassifyPointAgainstPlane(p, b.N, b.D)
	switch c {
	case BspPos:
		if b.Pos != nil {
			return classifyPointAgainstBsp(p, b.Pos)
		}
		return BspPos
	case BspNeg:
		if b.Neg != nil {
			return classifyPointAgainstBsp(p, b.Neg)
		}
		return BspNeg
	default:
		// Coplanar with the partitioner plane — check the More
		// chain (each successive coplanar splitter) before deciding.
		if b.More != nil {
			return classifyPointAgainstBsp(p, b.More)
		}
		return BspCoplanar
	}
}

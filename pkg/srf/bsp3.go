package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SBsp3 is a 3-D BSP tree over STriangle. Used by SMesh boolean
// operations to do CSG (union, difference, intersection) by
// partitioning space and testing each triangle's classification.
type SBsp3 struct {
	N    geom.Vector
	D    float64
	Tri  STriangle
	Pos  *SBsp3
	Neg  *SBsp3
	More *SBsp3

	Edges *SBsp2
}

// SBsp3FromMesh builds an SBsp3 from a closed triangle mesh by
// recursively selecting splitting planes (median centroid along
// longest axis) and inserting each triangle.
func SBsp3FromMesh(m *SMesh) *SBsp3 {
	if m == nil || len(m.Triangles) == 0 {
		return nil
	}
	root := &SBsp3{}
	insertListIntoBsp3(root, m.Triangles, 0)
	return root
}

func insertListIntoBsp3(b *SBsp3, tris []STriangle, depth int) {
	if len(tris) == 0 {
		return
	}
	if depth > 30 || len(tris) <= 4 {
		// Leaf fallback: pick first triangle as plane.
		b.Tri = tris[0]
		b.N = tris[0].NormalUnit()
		b.D = b.N.Dot(tris[0].A)
		for _, t := range tris[1:] {
			b.insert(t)
		}
		return
	}
	// Find longest axis of the bounding box.
	bmin := tris[0].A
	bmax := tris[0].A
	for _, t := range tris {
		for _, p := range [3]geom.Vector{t.A, t.B, t.C} {
			if p.X < bmin.X {
				bmin.X = p.X
			}
			if p.Y < bmin.Y {
				bmin.Y = p.Y
			}
			if p.Z < bmin.Z {
				bmin.Z = p.Z
			}
			if p.X > bmax.X {
				bmax.X = p.X
			}
			if p.Y > bmax.Y {
				bmax.Y = p.Y
			}
			if p.Z > bmax.Z {
				bmax.Z = p.Z
			}
		}
	}
	dx := bmax.X - bmin.X
	dy := bmax.Y - bmin.Y
	dz := bmax.Z - bmin.Z
	axis := 0
	maxd := dx
	if dy > maxd {
		axis = 1
		maxd = dy
	}
	if dz > maxd {
		axis = 2
		maxd = dz
	}
	planeNormal := geom.Vector{}
	switch axis {
	case 0:
		planeNormal.X = 1
	case 1:
		planeNormal.Y = 1
	default:
		planeNormal.Z = 1
	}
	// Use median centroid of the triangles as the splitting plane
	// position (more robust to asymmetric distributions than bbox
	// midpoint).
	centroids := make([]float64, len(tris))
	for i, t := range tris {
		c := t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3.0)
		centroids[i] = axisCoord(c, axis)
	}
	sortFloat64s(centroids)
	planeD := centroids[len(centroids)/2]
	b.N = planeNormal
	b.D = planeD
	var pos, neg, more []STriangle
	for _, t := range tris {
		c := ClassifyPointAgainstPlane(t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0/3.0), b.N, b.D)
		switch c {
		case BspPos:
			pos = append(pos, t)
		case BspNeg:
			neg = append(neg, t)
		default:
			more = append(more, t)
		}
	}
	// If all triangles are coplanar (flat geometry case), use the
	// first triangle's own plane as the partitioner.
	if len(more) == len(tris) {
		// Use the first triangle's plane; split based on vertex
		// distances from the triangle's centroid.
		first := tris[0]
		b.N = first.NormalUnit()
		b.D = b.N.Dot(first.A.Plus(first.B).Plus(first.C).ScaledBy(1.0 / 3.0))
		// All triangles in this group are roughly coplanar; just
		// store them all in More.
		more = tris[:]
		pos = nil
		neg = nil
	}
	if len(more) > 0 {
		b.Tri = more[0]
		for _, t := range more[1:] {
			b.More = SBsp3InsertOrCreate(b.More, t, nil)
		}
	}
	if len(pos) > 0 {
		b.Pos = &SBsp3{}
		insertListIntoBsp3(b.Pos, pos, depth+1)
	}
	if len(neg) > 0 {
		b.Neg = &SBsp3{}
		insertListIntoBsp3(b.Neg, neg, depth+1)
	}
}

func sortFloat64s(s []float64) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// Insert adds one triangle to the BSP, splitting it across the
// partition plane if necessary.
func (b *SBsp3) insert(t STriangle) *SBsp3 {
	return SBsp3InsertOrCreate(b, t, nil)
}

// SBsp3InsertOrCreate is the static helper that recursively inserts
// a triangle into the BSP.
func SBsp3InsertOrCreate(where *SBsp3, t STriangle, instead *SMesh) *SBsp3 {
	if where == nil {
		return &SBsp3{
			N:   t.NormalUnit(),
			D:   t.NormalUnit().Dot(t.A),
			Tri: t,
		}
	}
	// Classify each vertex of t against where's plane.
	cA := ClassifyPointAgainstPlane(t.A, where.N, where.D)
	cB := ClassifyPointAgainstPlane(t.B, where.N, where.D)
	cC := ClassifyPointAgainstPlane(t.C, where.N, where.D)
	if cA == BspCoplanar && cB == BspCoplanar && cC == BspCoplanar {
		// Coplanar with the partition plane.
		where.More = SBsp3InsertOrCreate(where.More, t, instead)
		return where
	}
	// If all on positive side, recurse positive.
	if cA != BspNeg && cB != BspNeg && cC != BspNeg {
		where.Pos = SBsp3InsertOrCreate(where.Pos, t, instead)
		return where
	}
	// If all on negative side, recurse negative.
	if cA != BspPos && cB != BspPos && cC != BspPos {
		where.Neg = SBsp3InsertOrCreate(where.Neg, t, instead)
		return where
	}
	// Triangle straddles the plane; for simplicity, push it down
	// into both sides. (Full implementation would split the
	// triangle at the intersection line; Phase 4 extends this.)
	where.Pos = SBsp3InsertOrCreate(where.Pos, t, instead)
	where.Neg = SBsp3InsertOrCreate(where.Neg, t, instead)
	return where
}

// IntersectionWith returns the intersection of segment (a, b) with
// the BSP's partition plane. Returns false if the segment is
// parallel to the plane.
func (b *SBsp3) IntersectionWith(a, dir geom.Vector) (geom.Vector, bool) {
	if b == nil {
		return geom.Vector{}, false
	}
	den := dir.Dot(b.N)
	if mathAbs(den) < 1e-20 {
		return geom.Vector{}, false
	}
	t := (b.D - a.Dot(b.N)) / den
	return a.Plus(dir.ScaledBy(t)), true
}

// GenerateInPaintOrder walks the BSP from the inside out so that
// triangles are emitted in back-to-front order for the painter's
// algorithm. Used by the renderer when transparency is enabled.
// Phase 2 implementation: in-order traversal of the POS subtree
// first, then the More list, then NEG.
func (b *SBsp3) GenerateInPaintOrder(out *SMesh) {
	if b == nil {
		return
	}
	b.Neg.GenerateInPaintOrder(out)
	out.AddTriangle(b.Tri)
	if b.More != nil {
		b.More.GenerateInPaintOrder(out)
	}
	b.Pos.GenerateInPaintOrder(out)
}

// RayTriangleIntersection walks the BSP and tests each leaf's
// triangle against the ray. Returns the closest intersection
// (smallest t > 0). Used for accurate point-in-mesh tests and
// ray-cast queries.
//
// Phase 7 + Goal Item 13 implementation: when the ray start is
// strictly on one side of the partitioner plane, recurse into
// that child (if any) and test the More chain there. When the
// partitioner itself is the leaf (no children), test its triangle.
// Coplanar start: test the More chain and recurse into both
// subtrees.
func (b *SBsp3) RayTriangleIntersection(a, dir geom.Vector) (geom.Vector, float64, bool) {
	if b == nil {
		return geom.Vector{}, 0, false
	}
	if dir.Magnitude() < 1e-20 {
		return geom.Vector{}, 0, false
	}
	// Test this node's partitioner triangle first.
	bestT := mathInfPos
	var bestPt geom.Vector
	found := false
	if b.Tri.A != b.Tri.B || b.Tri.B != b.Tri.C {
		if tt, hit, ok := b.Tri.Raytrace(a, dir); ok && tt >= 0 {
			if tt < bestT {
				bestT = tt
				bestPt = hit
				found = true
			}
		}
	}
	// Test the More chain.
	for m := b.More; m != nil; m = m.More {
		if tt, hit, ok := m.Tri.Raytrace(a, dir); ok && tt >= 0 {
			if tt < bestT {
				bestT = tt
				bestPt = hit
				found = true
			}
		}
	}
	// Recurse based on classification.
	c := ClassifyPointAgainstPlane(a, b.N, b.D)
	switch c {
	case BspPos:
		if pt, t, ok := b.Pos.RayTriangleIntersection(a, dir); ok {
			if !found || t < bestT {
				return pt, t, ok
			}
		}
	case BspNeg:
		if pt, t, ok := b.Neg.RayTriangleIntersection(a, dir); ok {
			if !found || t < bestT {
				return pt, t, ok
			}
		}
	default:
		if pt, t, ok := b.Pos.RayTriangleIntersection(a, dir); ok {
			if !found || t < bestT {
				return pt, t, ok
			}
		}
		if pt, t, ok := b.Neg.RayTriangleIntersection(a, dir); ok {
			if !found || t < bestT {
				return pt, t, ok
			}
		}
	}
	if found {
		return bestPt, bestT, true
	}
	return geom.Vector{}, 0, false
}

// InsertConvex inserts a convex polygon (n vertices, vertex slice)
// as a single BSP subtree, recursively partitioning it by the
// existing BSP's planes.
func (b *SBsp3) InsertConvex(meta STriMeta, vertex []geom.Vector, instead *SMesh) *SBsp3 {
	if len(vertex) < 3 {
		return b
	}
	if b == nil {
		// First triangle.
		t := STriangleFrom(meta, vertex[0], vertex[1], vertex[2])
		return &SBsp3{
			N:   t.NormalUnit(),
			D:   t.NormalUnit().Dot(t.A),
			Tri: t,
		}
	}
	// Classify each vertex against b's plane.
	pos, neg := []geom.Vector{}, []geom.Vector{}
	for _, v := range vertex {
		c := ClassifyPointAgainstPlane(v, b.N, b.D)
		if c == BspPos {
			pos = append(pos, v)
		} else if c == BspNeg {
			neg = append(neg, v)
		} else {
			pos = append(pos, v)
			neg = append(neg, v)
		}
	}
	if len(pos) == 0 {
		b.Neg = b.Neg.InsertConvex(meta, neg, instead)
		return b
	}
	if len(neg) == 0 {
		b.Pos = b.Pos.InsertConvex(meta, pos, instead)
		return b
	}
	// Splits the polygon; emit two children.
	if len(pos) >= 3 {
		b.Pos = b.Pos.InsertConvex(meta, pos, instead)
	}
	if len(neg) >= 3 {
		b.Neg = b.Neg.InsertConvex(meta, neg, instead)
	}
	return b
}

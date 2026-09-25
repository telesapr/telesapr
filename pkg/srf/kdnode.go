package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SKdNode is a kd-tree over STriangles, used to accelerate
// queries like "find the closest triangle to point p" and "snap
// a point to the nearest vertex on the mesh". Construction uses
// axis-aligned bounding boxes with median splits; the leaf node
// holds up to 4 triangles for brute-force nearest lookup.
type SKdNode struct {
	BoxMin, BoxMax geom.Vector
	Axis           int // 0=X, 1=Y, 2=Z, -1=leaf
	Split          float64
	Pos            *SKdNode
	Neg            *SKdNode
	Triangles      []STriangle
}

// BuildMeshKdTree builds an SKdNode over the triangles of mesh m.
func BuildMeshKdTree(m *SMesh) *SKdNode {
	if m == nil || len(m.Triangles) == 0 {
		return nil
	}
	root := &SKdNode{}
	buildMeshKdRecursive(root, m.Triangles, 0)
	return root
}

func buildMeshKdRecursive(node *SKdNode, tris []STriangle, depth int) {
	bn, bx := trianglesBoundingBox(tris)
	node.BoxMin = bn
	node.BoxMax = bx
	if len(tris) <= 4 || depth > 30 {
		node.Triangles = append([]STriangle{}, tris...)
		node.Axis = -1
		return
	}
	dx := bx.X - bn.X
	dy := bx.Y - bn.Y
	dz := bx.Z - bn.Z
	if dx >= dy && dx >= dz {
		node.Axis = 0
	} else if dy >= dz {
		node.Axis = 1
	} else {
		node.Axis = 2
	}
	// Sort and split at median centroid.
	ts := append([]STriangle{}, tris...)
	sortTrisByCentroidAxis(ts, node.Axis)
	mid := len(ts) / 2
	node.Split = triCentroidAxis(ts[mid], node.Axis)
	node.Pos = &SKdNode{}
	node.Neg = &SKdNode{}
	buildMeshKdRecursive(node.Pos, ts[mid:], depth+1)
	buildMeshKdRecursive(node.Neg, ts[:mid], depth+1)
}

func trianglesBoundingBox(tris []STriangle) (min, max geom.Vector) {
	if len(tris) == 0 {
		return geom.Vector{}, geom.Vector{}
	}
	min = tris[0].A
	max = tris[0].A
	for _, t := range tris {
		for _, p := range [3]geom.Vector{t.A, t.B, t.C} {
			if p.X < min.X {
				min.X = p.X
			}
			if p.Y < min.Y {
				min.Y = p.Y
			}
			if p.Z < min.Z {
				min.Z = p.Z
			}
			if p.X > max.X {
				max.X = p.X
			}
			if p.Y > max.Y {
				max.Y = p.Y
			}
			if p.Z > max.Z {
				max.Z = p.Z
			}
		}
	}
	return min, max
}

func triCentroidAxis(t STriangle, axis int) float64 {
	c := t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3.0)
	switch axis {
	case 0:
		return c.X
	case 1:
		return c.Y
	default:
		return c.Z
	}
}

func sortTrisByCentroidAxis(tris []STriangle, axis int) {
	if len(tris) < 16 {
		for i := 1; i < len(tris); i++ {
			for j := i; j > 0 && triCentroidAxis(tris[j-1], axis) > triCentroidAxis(tris[j], axis); j-- {
				tris[j-1], tris[j] = tris[j], tris[j-1]
			}
		}
		return
	}
	var qs func(lo, hi int)
	qs = func(lo, hi int) {
		if lo >= hi {
			return
		}
		pivot := triCentroidAxis(tris[(lo+hi)/2], axis)
		i, j := lo, hi
		for i <= j {
			for triCentroidAxis(tris[i], axis) < pivot {
				i++
			}
			for triCentroidAxis(tris[j], axis) > pivot {
				j--
			}
			if i <= j {
				tris[i], tris[j] = tris[j], tris[i]
				i++
				j--
			}
		}
		qs(lo, j)
		qs(i, hi)
	}
	qs(0, len(tris)-1)
}

// FindClosestTriangle returns the triangle in the kd-tree closest
// to point p, with the squared distance via distSq (if non-nil).
// Returns nil if the kd-tree is empty.
func FindClosestTriangle(node *SKdNode, p geom.Vector, distSq *float64) *STriangle {
	if node == nil {
		return nil
	}
	if !pointInBox(p, node.BoxMin, node.BoxMax) {
		// Quick rejection: compute squared distance from p to box
		// surface; if it's already worse than the current best, abort.
		return nil
	}
	if node.Axis == -1 {
		// Leaf: test every triangle.
		var best *STriangle
		var bestDistSq float64 = mathInfPos
		for i := range node.Triangles {
			t := &node.Triangles[i]
			d := pointTriangleSquaredDistance(p, t)
			if d < bestDistSq {
				bestDistSq = d
				best = t
			}
		}
		if distSq != nil {
			*distSq = bestDistSq
		}
		return best
	}
	// Recurse into both children (this is correct because the closest
	// triangle could be on either side of the split).
	left := FindClosestTriangle(node.Neg, p, distSq)
	right := FindClosestTriangle(node.Pos, p, distSq)
	if left == nil {
		if distSq != nil {
			*distSq = mathInfPos
		}
		return right
	}
	if right == nil {
		return left
	}
	leftDist := pointTriangleSquaredDistance(p, left)
	rightDist := pointTriangleSquaredDistance(p, right)
	if leftDist < rightDist {
		if distSq != nil {
			*distSq = leftDist
		}
		return left
	}
	if distSq != nil {
		*distSq = rightDist
	}
	return right
}

// MakeMeshInto is the Phase 2 version of SKdNode::MakeMeshInto:
// walks the kd-tree and appends all triangles to m.
func (n *SKdNode) MakeMeshInto(m *SMesh) {
	if n == nil {
		return
	}
	if n.Axis == -1 {
		for i := range n.Triangles {
			m.AddTriangle(n.Triangles[i])
		}
		return
	}
	n.Pos.MakeMeshInto(m)
	n.Neg.MakeMeshInto(m)
}

func pointInBox(p, bmin, bmax geom.Vector) bool {
	return p.X >= bmin.X && p.X <= bmax.X &&
		p.Y >= bmin.Y && p.Y <= bmax.Y &&
		p.Z >= bmin.Z && p.Z <= bmax.Z
}

func pointTriangleSquaredDistance(p geom.Vector, t *STriangle) float64 {
	// Closest point on triangle to p (Real-Time Collision Detection
	// by Christer Ericson, section 5.1.5).
	a := t.A
	b := t.B
	c := t.C
	ab := b.Minus(a)
	ac := c.Minus(a)
	ap := p.Minus(a)
	d1 := ab.Dot(ap)
	d2 := ac.Dot(ap)
	if d1 <= 0 && d2 <= 0 {
		return ap.MagSquared()
	}
	bp := p.Minus(b)
	d3 := ab.Dot(bp)
	d4 := ac.Dot(bp)
	if d3 >= 0 && d4 <= d3 {
		return bp.MagSquared()
	}
	vc := d1*d4 - d3*d2
	if vc <= 0 && d1 >= 0 && d3 <= 0 {
		v := d1 / (d1 - d3)
		closest := a.Plus(ab.ScaledBy(v))
		return p.Minus(closest).MagSquared()
	}
	cp := p.Minus(c)
	d5 := ab.Dot(cp)
	d6 := ac.Dot(cp)
	if d6 >= 0 && d5 <= d6 {
		return cp.MagSquared()
	}
	vb := d5*d2 - d1*d6
	if vb <= 0 && d2 >= 0 && d6 <= 0 {
		w := d2 / (d2 - d6)
		closest := a.Plus(ac.ScaledBy(w))
		return p.Minus(closest).MagSquared()
	}
	va := d3*d6 - d5*d4
	if va <= 0 && (d4-d3) >= 0 && (d5-d6) >= 0 {
		w := (d4 - d3) / ((d4 - d3) + (d5 - d6))
		closest := b.Plus(c.Minus(b).ScaledBy(w))
		return p.Minus(closest).MagSquared()
	}
	denom := 1.0 / (va + vb + vc)
	v := vb * denom
	w := vc * denom
	closest := a.Plus(ab.ScaledBy(v)).Plus(ac.ScaledBy(w))
	return p.Minus(closest).MagSquared()
}

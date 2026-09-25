package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SEdgeLl is a singly-linked list node of SEdge pointers, used as
// the leaf payload of SKdNodeEdges.
type SEdgeLl struct {
	Edge *SEdge
	Next *SEdgeLl
}

// SKdNodeEdges is a kd-tree over a set of edges, used to accelerate
// line-vs-many-edges intersection queries (any line passing through
// the kd-tree box hits only the edges in the cell it crosses, not
// every edge in the mesh). Phase 2: minimal version with axis-aligned
// bounding boxes; full SAH-based subdivision comes later if needed.
type SKdNodeEdges struct {
	BoxMin, BoxMax geom.Vector
	Axis           int // 0=X, 1=Y, 2=Z, -1=leaf
	Split          float64
	Pos            *SKdNodeEdges
	Neg            *SKdNodeEdges
	Edges          *SEdgeLl
}

// BuildEdgeKdTree builds a kd-tree over the edges in el. The tree
// subdivides recursively: it picks the longest axis, sorts the
// edges' centroids along that axis, and splits at the median.
// Recursion stops when fewer than 4 edges remain in a cell.
func BuildEdgeKdTree(el *SEdgeList) *SKdNodeEdges {
	if len(el.Edges) == 0 {
		return nil
	}
	if len(el.Edges) <= 4 {
		var ll *SEdgeLl
		for i := len(el.Edges) - 1; i >= 0; i-- {
			ll = &SEdgeLl{Edge: &el.Edges[i], Next: ll}
		}
		bn, bx := edgesBoundingBox(el.Edges)
		return &SKdNodeEdges{Edges: ll, BoxMin: bn, BoxMax: bx, Axis: -1}
	}
	// Bounding box of all edges.
	bn, bx := edgesBoundingBox(el.Edges)
	root := &SKdNodeEdges{BoxMin: bn, BoxMax: bx}
	// Pick longest axis.
	dx := bx.X - bn.X
	dy := bx.Y - bn.Y
	dz := bx.Z - bn.Z
	if dx >= dy && dx >= dz {
		root.Axis = 0
	} else if dy >= dz {
		root.Axis = 1
	} else {
		root.Axis = 2
	}
	// Sort edges by centroid along the chosen axis and split at median.
	edges := append([]SEdge{}, el.Edges...)
	sortEdgesByCentroidAxis(edges, root.Axis)
	mid := len(edges) / 2
	root.Split = centroidAxis(edges[mid], root.Axis)
	posList := &SEdgeList{Edges: edges[mid:]}
	negList := &SEdgeList{Edges: edges[:mid]}
	root.Pos = BuildEdgeKdTree(posList)
	root.Neg = BuildEdgeKdTree(negList)
	return root
}

func edgesBoundingBox(edges []SEdge) (min, max geom.Vector) {
	if len(edges) == 0 {
		return geom.Vector{}, geom.Vector{}
	}
	min = edges[0].A
	max = edges[0].A
	for _, e := range edges {
		for _, p := range [2]geom.Vector{e.A, e.B} {
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

func centroidAxis(e SEdge, axis int) float64 {
	c := e.A.Plus(e.B).ScaledBy(0.5)
	switch axis {
	case 0:
		return c.X
	case 1:
		return c.Y
	default:
		return c.Z
	}
}

func sortEdgesByCentroidAxis(edges []SEdge, axis int) {
	// Insertion sort for small slices, qsort for larger.
	if len(edges) < 16 {
		for i := 1; i < len(edges); i++ {
			for j := i; j > 0 && centroidAxis(edges[j-1], axis) > centroidAxis(edges[j], axis); j-- {
				edges[j-1], edges[j] = edges[j], edges[j-1]
			}
		}
		return
	}
	// Simple quicksort.
	var qs func(lo, hi int)
	qs = func(lo, hi int) {
		if lo >= hi {
			return
		}
		pivot := centroidAxis(edges[(lo+hi)/2], axis)
		i, j := lo, hi
		for i <= j {
			for centroidAxis(edges[i], axis) < pivot {
				i++
			}
			for centroidAxis(edges[j], axis) > pivot {
				j--
			}
			if i <= j {
				edges[i], edges[j] = edges[j], edges[i]
				i++
				j--
			}
		}
		qs(lo, j)
		qs(i, hi)
	}
	qs(0, len(edges)-1)
}

// EdgeKdIntersect reports the closest intersection of segment (a, b)
// with any edge in the kd-tree, returning the intersection point and
// the edge that was hit. Returns nil edge if no intersection.
func EdgeKdIntersect(node *SKdNodeEdges, a, b geom.Vector) (geom.Vector, *SEdge) {
	if node == nil {
		return geom.Vector{}, nil
	}
	dir := b.Minus(a)
	if !lineIntersectsBox(a, dir, node.BoxMin, node.BoxMax) {
		return geom.Vector{}, nil
	}
	if node.Axis == -1 {
		// Leaf: test all edges.
		bestT := mathInfPos
		var bestPt geom.Vector
		var bestEdge *SEdge
		for ll := node.Edges; ll != nil; ll = ll.Next {
			t, ok := ll.Edge.Crossing(a, dir)
			if !ok || t < 0 || t > 1 {
				continue
			}
			if t < bestT {
				bestT = t
				bestPt = a.Plus(dir.ScaledBy(t))
				bestEdge = ll.Edge
			}
		}
		return bestPt, bestEdge
	}
	// Decide which side of the split the segment start is on.
	startVal := axisCoord(a, node.Axis)
	if startVal < node.Split {
		pt, e := EdgeKdIntersect(node.Neg, a, b)
		if e != nil {
			return pt, e
		}
		return EdgeKdIntersect(node.Pos, a, b)
	}
	pt, e := EdgeKdIntersect(node.Pos, a, b)
	if e != nil {
		return pt, e
	}
	return EdgeKdIntersect(node.Neg, a, b)
}

func axisCoord(v geom.Vector, axis int) float64 {
	switch axis {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}

func lineIntersectsBox(a, dir, bmin, bmax geom.Vector) bool {
	if dir.Magnitude() < 1e-20 {
		return false
	}
	tmin, tmax := 0.0, 1.0
	axes := [3]float64{dir.X, dir.Y, dir.Z}
	orig := [3]float64{a.X, a.Y, a.Z}
	mins := [3]float64{bmin.X, bmin.Y, bmin.Z}
	maxs := [3]float64{bmax.X, bmax.Y, bmax.Z}
	for i := 0; i < 3; i++ {
		if axes[i] == 0 {
			if orig[i] < mins[i] || orig[i] > maxs[i] {
				return false
			}
			continue
		}
		t1 := (mins[i] - orig[i]) / axes[i]
		t2 := (maxs[i] - orig[i]) / axes[i]
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		if t1 > tmin {
			tmin = t1
		}
		if t2 < tmax {
			tmax = t2
		}
		if tmin > tmax {
			return false
		}
	}
	return true
}

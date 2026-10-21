package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// MeshTree is a tree of boxes over a mesh's triangles, to find the
// nearest points of two meshes, the point of one nearest a point, and
// where a ray leaves one, without looking at every triangle: the
// clearance between the parts of an assembly and the thickness of
// their walls (pkg/sketch).
type MeshTree struct {
	tri   []STriangle
	idx   []int32
	nodes []meshNode
}

// meshNode is a box of the tree: a leaf holds triangles idx[first :
// first+n], another node its two children.
type meshNode struct {
	lo, hi      geom.Vector
	left, right int32
	first, n    int32
}

// meshLeaf is how many triangles a leaf holds at most.
const meshLeaf = 4

// NewMeshTree is the tree of m's triangles. It holds them as they are;
// m must not change while the tree is used.
func NewMeshTree(m *SMesh) *MeshTree {
	t := &MeshTree{tri: m.Triangles, idx: make([]int32, len(m.Triangles))}
	for i := range t.idx {
		t.idx[i] = int32(i)
	}
	if len(t.idx) > 0 {
		t.build(0, int32(len(t.idx)))
	}
	return t
}

// Len is the number of triangles in the tree.
func (t *MeshTree) Len() int { return len(t.tri) }

// Triangle is the tree's triangle i, by its place in the mesh.
func (t *MeshTree) Triangle(i int) *STriangle { return &t.tri[i] }

// build makes the node of triangles idx[first:end], splitting them at
// the middle along the longest side of their centres' box, and is its
// index.
func (t *MeshTree) build(first, end int32) int32 {
	at := int32(len(t.nodes))
	t.nodes = append(t.nodes, meshNode{left: -1, right: -1, first: first, n: end - first})
	lo, hi := boxOf(t.tri[t.idx[first]])
	clo, chi := centre(t.tri[t.idx[first]]), centre(t.tri[t.idx[first]])
	for _, i := range t.idx[first+1 : end] {
		l, h := boxOf(t.tri[i])
		lo, hi = minV(lo, l), maxV(hi, h)
		c := centre(t.tri[i])
		clo, chi = minV(clo, c), maxV(chi, c)
	}
	t.nodes[at].lo, t.nodes[at].hi = lo, hi
	if end-first <= meshLeaf {
		return at
	}
	axis, ext := 0, chi.X-clo.X
	if e := chi.Y - clo.Y; e > ext {
		axis, ext = 1, e
	}
	if e := chi.Z - clo.Z; e > ext {
		axis, ext = 2, e
	}
	if ext == 0 {
		return at
	}
	part := t.idx[first:end]
	slices.SortStableFunc(part, func(a, b int32) int {
		ca, cb := centre(t.tri[a]).Element(axis), centre(t.tri[b]).Element(axis)
		switch {
		case ca < cb:
			return -1
		case ca > cb:
			return 1
		}
		return 0
	})
	mid := first + (end-first)/2
	left := t.build(first, mid)
	right := t.build(mid, end)
	t.nodes[at].left, t.nodes[at].right = left, right
	return at
}

// MeshNearest is where two meshes come nearest: the distance, a
// triangle of each by its place in its mesh, and the point on each.
type MeshNearest struct {
	Dist   float64
	A, B   int
	PA, PB geom.Vector
}

// Nearest is where the meshes of t and o come nearest, if it is less
// than limit apart; a distance of 0 is where they meet. A limit of
// +Inf finds it however far.
func (t *MeshTree) Nearest(o *MeshTree, limit float64) (MeshNearest, bool) {
	best := MeshNearest{Dist: limit, A: -1}
	if len(t.nodes) == 0 || len(o.nodes) == 0 {
		return best, false
	}
	t.nearest(o, 0, 0, &best)
	return best, best.A >= 0
}

// nearest looks for a pair nearer than best in nodes i of t and j of
// o, the nearer child first so that best shrinks early.
func (t *MeshTree) nearest(o *MeshTree, i, j int32, best *MeshNearest) {
	a, b := &t.nodes[i], &o.nodes[j]
	if best.Dist == 0 || boxDistance(a.lo, a.hi, b.lo, b.hi) >= best.Dist {
		return
	}
	switch {
	case a.left < 0 && b.left < 0:
		for _, ti := range t.idx[a.first : a.first+a.n] {
			for _, oi := range o.idx[b.first : b.first+b.n] {
				d, p, q := triangleDistance(&t.tri[ti], &o.tri[oi])
				if d < best.Dist {
					*best = MeshNearest{Dist: d, A: int(ti), B: int(oi), PA: p, PB: q}
				}
			}
		}
	case b.left < 0 || a.left >= 0 && diagonal(a) >= diagonal(b):
		c0, c1 := a.left, a.right
		if boxDistance(t.nodes[c1].lo, t.nodes[c1].hi, b.lo, b.hi) < boxDistance(t.nodes[c0].lo, t.nodes[c0].hi, b.lo, b.hi) {
			c0, c1 = c1, c0
		}
		t.nearest(o, c0, j, best)
		t.nearest(o, c1, j, best)
	default:
		c0, c1 := b.left, b.right
		if boxDistance(a.lo, a.hi, o.nodes[c1].lo, o.nodes[c1].hi) < boxDistance(a.lo, a.hi, o.nodes[c0].lo, o.nodes[c0].hi) {
			c0, c1 = c1, c0
		}
		t.nearest(o, i, c0, best)
		t.nearest(o, i, c1, best)
	}
}

// NearestPoint is the point of the mesh nearest p, how far it is, and
// its triangle by its place in the mesh; -1 for an empty mesh.
func (t *MeshTree) NearestPoint(p geom.Vector) (d float64, tri int, q geom.Vector) {
	d, tri = math.Inf(1), -1
	if len(t.nodes) == 0 {
		return d, tri, q
	}
	var walk func(i int32)
	walk = func(i int32) {
		nd := &t.nodes[i]
		if pointBoxDistance(p, nd.lo, nd.hi) >= d {
			return
		}
		if nd.left < 0 {
			for _, ti := range t.idx[nd.first : nd.first+nd.n] {
				tr := &t.tri[ti]
				r := closestOnTriangle(p, tr.A, tr.B, tr.C)
				if dd := r.Minus(p).Magnitude(); dd < d {
					d, tri, q = dd, int(ti), r
				}
			}
			return
		}
		c0, c1 := nd.left, nd.right
		if pointBoxDistance(p, t.nodes[c1].lo, t.nodes[c1].hi) < pointBoxDistance(p, t.nodes[c0].lo, t.nodes[c0].hi) {
			c0, c1 = c1, c0
		}
		walk(c0)
		walk(c1)
	}
	walk(0)
	return d, tri, q
}

// Within are the triangles, by their places in the mesh, that come
// within r of p, in no order.
func (t *MeshTree) Within(p geom.Vector, r float64) []int {
	var found []int
	if len(t.nodes) == 0 {
		return nil
	}
	var walk func(i int32)
	walk = func(i int32) {
		nd := &t.nodes[i]
		if pointBoxDistance(p, nd.lo, nd.hi) > r {
			return
		}
		if nd.left < 0 {
			for _, ti := range t.idx[nd.first : nd.first+nd.n] {
				tr := &t.tri[ti]
				if closestOnTriangle(p, tr.A, tr.B, tr.C).Minus(p).Magnitude() <= r {
					found = append(found, int(ti))
				}
			}
			return
		}
		walk(nd.left)
		walk(nd.right)
	}
	walk(0)
	return found
}

// MeshHit is where a ray first meets a mesh: how far along it, by the
// length of its direction, the triangle by its place in the mesh, and
// the cosine of the angle between the ray and the triangle's normal,
// positive where the ray leaves the solid there.
type MeshHit struct {
	T   float64
	Tri int
	Cos float64
}

// Ray is where the ray p + t·dir first meets the mesh for t past tmin,
// leaving out triangle skip (the one it starts from, or -1).
func (t *MeshTree) Ray(p, dir geom.Vector, tmin float64, skip int) (MeshHit, bool) {
	best := MeshHit{T: math.Inf(1), Tri: -1}
	if len(t.nodes) == 0 {
		return best, false
	}
	inv := geom.Vector{X: 1 / dir.X, Y: 1 / dir.Y, Z: 1 / dir.Z}
	var walk func(i int32)
	walk = func(i int32) {
		nd := &t.nodes[i]
		if enter, ok := slab(p, inv, nd.lo, nd.hi, best.T); !ok || enter > best.T {
			return
		}
		if nd.left < 0 {
			for _, ti := range t.idx[nd.first : nd.first+nd.n] {
				if int(ti) == skip {
					continue
				}
				tr := &t.tri[ti]
				if tt, cos, ok := rayTriangle(p, dir, tr.A, tr.B, tr.C); ok && tt > tmin && tt < best.T {
					best = MeshHit{T: tt, Tri: int(ti), Cos: cos}
				}
			}
			return
		}
		walk(nd.left)
		walk(nd.right)
	}
	walk(0)
	return best, best.Tri >= 0
}

// slab is where the ray p + t·dir (inv its direction's reciprocals)
// enters the box, and false if it misses it before tmax.
func slab(p, inv, lo, hi geom.Vector, tmax float64) (float64, bool) {
	enter, exit := math.Inf(-1), tmax
	for a := range 3 {
		o, r := p.Element(a), inv.Element(a)
		t0, t1 := (lo.Element(a)-o)*r, (hi.Element(a)-o)*r
		if math.IsNaN(t0) || math.IsNaN(t1) {
			// Along the slab's plane: in it or not.
			if o < lo.Element(a) || o > hi.Element(a) {
				return 0, false
			}
			continue
		}
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		enter, exit = max(enter, t0), min(exit, t1)
	}
	return enter, enter <= exit && exit >= 0
}

// boxOf is a triangle's box.
func boxOf(t STriangle) (lo, hi geom.Vector) {
	return minV(minV(t.A, t.B), t.C), maxV(maxV(t.A, t.B), t.C)
}

// centre is a triangle's centroid.
func centre(t STriangle) geom.Vector {
	return t.A.Plus(t.B).Plus(t.C).ScaledBy(1.0 / 3)
}

// diagonal is the squared length of a node's box's diagonal.
func diagonal(n *meshNode) float64 { return n.hi.Minus(n.lo).MagSquared() }

// minV and maxV are the least and greatest of two vectors, axis by axis.
func minV(a, b geom.Vector) geom.Vector {
	return geom.Vector{X: min(a.X, b.X), Y: min(a.Y, b.Y), Z: min(a.Z, b.Z)}
}

func maxV(a, b geom.Vector) geom.Vector {
	return geom.Vector{X: max(a.X, b.X), Y: max(a.Y, b.Y), Z: max(a.Z, b.Z)}
}

// boxDistance is the least distance between two boxes, 0 where they
// meet.
func boxDistance(alo, ahi, blo, bhi geom.Vector) float64 {
	var s float64
	for a := range 3 {
		g := max(alo.Element(a)-bhi.Element(a), blo.Element(a)-ahi.Element(a), 0)
		s += g * g
	}
	return math.Sqrt(s)
}

// pointBoxDistance is the distance from p to the box, 0 inside it.
func pointBoxDistance(p, lo, hi geom.Vector) float64 {
	return boxDistance(p, p, lo, hi)
}

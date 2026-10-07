// Package cdt triangulates a planar region bounded by segments: a
// constrained Delaunay triangulation of a set of points, of which the
// boundary segments are edges, keeping the triangles that lie inside
// by the even-odd rule. Points that no segment uses are inserted too,
// so a region can be filled with a grid of interior points.
//
// The orientation and in-circle predicates are exact: a determinant
// near zero is computed again in rational arithmetic, so the result
// does not depend on rounding.
package cdt

import (
	"errors"
	"math"
	"slices"
	"sort"
)

// Point is a point of the plane.
type Point struct{ X, Y float64 }

// ErrCrossing is returned, with the triangles, when two segments cross
// or a segment runs over another; the region is then only as good as
// the boundary was.
var ErrCrossing = errors.New("cdt: segments cross")

type edge [2]int32

type tri struct {
	v    [3]int32
	dead bool
}

type triangulation struct {
	pts   []Point
	tris  []tri
	edges map[edge]int32 // directed edge -> triangle that has it
	near  []int32        // a live triangle at each vertex
	cons  map[edge]int   // undirected (low, high) -> how many segments run along it
	last  int32
	rnd   uint32
	cross bool
}

// Triangulate returns the triangles of the region the segments bound,
// counter-clockwise, as indices into pts. A segment that occurs twice
// cancels out, as the even-odd rule implies. Points equal to an
// earlier point are merged into it.
func Triangulate(pts []Point, segs [][2]int) ([][3]int, error) {
	if len(pts) < 3 || len(segs) < 3 {
		return nil, nil
	}
	// Merge points that are equal.
	index := make([]int32, len(pts))
	uniq := make([]Point, 0, len(pts))
	orig := make([]int, 0, len(pts))
	seen := make(map[Point]int32, len(pts))
	for i, p := range pts {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			return nil, errors.New("cdt: a point is not finite")
		}
		if j, ok := seen[p]; ok {
			index[i] = j
			continue
		}
		seen[p] = int32(len(uniq))
		index[i] = int32(len(uniq))
		uniq = append(uniq, p)
		orig = append(orig, i)
	}
	n := len(uniq)
	if n < 3 {
		return nil, nil
	}

	t := &triangulation{
		edges: make(map[edge]int32, 6*n),
		cons:  make(map[edge]int),
		rnd:   2463534242,
	}
	// Normalise into the unit square, then add a triangle around it.
	lo, hi := uniq[0], uniq[0]
	for _, p := range uniq {
		lo.X, lo.Y = math.Min(lo.X, p.X), math.Min(lo.Y, p.Y)
		hi.X, hi.Y = math.Max(hi.X, p.X), math.Max(hi.Y, p.Y)
	}
	size := math.Max(hi.X-lo.X, hi.Y-lo.Y)
	if size == 0 {
		return nil, nil
	}
	t.pts = make([]Point, n+3)
	for i, p := range uniq {
		t.pts[i] = Point{(p.X - lo.X) / size, (p.Y - lo.Y) / size}
	}
	t.pts[n] = Point{-100, -100}
	t.pts[n+1] = Point{100, -100}
	t.pts[n+2] = Point{0.5, 100}
	t.near = make([]int32, n+3)
	t.addTri(int32(n), int32(n+1), int32(n+2))

	for _, i := range insertionOrder(t.pts[:n]) {
		t.insert(int32(i))
	}
	for _, s := range segs {
		if s[0] < 0 || s[0] >= len(pts) || s[1] < 0 || s[1] >= len(pts) {
			return nil, errors.New("cdt: a segment names a point that isn't there")
		}
		a, b := index[s[0]], index[s[1]]
		if a == b {
			continue
		}
		k := edge{a, b}
		if a > b {
			k = edge{b, a}
		}
		t.cons[k]++
	}
	// Insert each distinct segment once; its count gives the parity.
	keys := make([]edge, 0, len(t.cons))
	for k := range t.cons {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	for _, k := range keys {
		t.insertSegment(k[0], k[1], 0)
	}

	out := t.inside(int32(n))
	res := make([][3]int, len(out))
	for i, tr := range out {
		res[i] = [3]int{orig[tr[0]], orig[tr[1]], orig[tr[2]]}
	}
	if t.cross {
		return res, ErrCrossing
	}
	return res, nil
}

// insertionOrder sorts points along a space-filling path, so that each
// insertion starts its walk near where it ends.
func insertionOrder(pts []Point) []int {
	order := make([]int, len(pts))
	keys := make([]uint64, len(pts))
	for i, p := range pts {
		order[i] = i
		keys[i] = hilbert(uint32(math.Max(0, math.Min(1, p.X))*65535), uint32(math.Max(0, math.Min(1, p.Y))*65535))
	}
	sort.SliceStable(order, func(a, b int) bool { return keys[order[a]] < keys[order[b]] })
	return order
}

// hilbert is the index of (x, y) along a Hilbert curve over a grid of
// 2^16 by 2^16.
func hilbert(x, y uint32) uint64 {
	const n = 1 << 16
	var d uint64
	for s := uint32(n / 2); s > 0; s /= 2 {
		var rx, ry uint32
		if x&s != 0 {
			rx = 1
		}
		if y&s != 0 {
			ry = 1
		}
		d += uint64(s) * uint64(s) * uint64((3*rx)^ry)
		if ry == 0 {
			if rx == 1 {
				x = n - 1 - x
				y = n - 1 - y
			}
			x, y = y, x
		}
	}
	return d
}

func (t *triangulation) addTri(a, b, c int32) {
	i := int32(len(t.tris))
	t.tris = append(t.tris, tri{v: [3]int32{a, b, c}})
	t.edges[edge{a, b}] = i
	t.edges[edge{b, c}] = i
	t.edges[edge{c, a}] = i
	t.near[a], t.near[b], t.near[c] = i, i, i
	t.last = i
}

func (t *triangulation) killTri(i int32) {
	tr := &t.tris[i]
	tr.dead = true
	for k := range 3 {
		e := edge{tr.v[k], tr.v[(k+1)%3]}
		if j, ok := t.edges[e]; ok && j == i {
			delete(t.edges, e)
		}
	}
}

// across is the triangle on the other side of the edge a->b of a
// triangle, or -1.
func (t *triangulation) across(a, b int32) int32 {
	if i, ok := t.edges[edge{b, a}]; ok {
		return i
	}
	return -1
}

func (t *triangulation) random() uint32 {
	t.rnd ^= t.rnd << 13
	t.rnd ^= t.rnd >> 17
	t.rnd ^= t.rnd << 5
	return t.rnd
}

// locate finds a triangle that contains p, on its boundary or inside.
func (t *triangulation) locate(p Point) int32 {
	i := t.last
	if t.tris[i].dead {
		for j, v := range slices.Backward(t.tris) {
			if !v.dead {
				i = int32(j)
				break
			}
		}
	}
	for steps := 0; steps < 4*len(t.tris)+16; steps++ {
		tr := t.tris[i].v
		start := int(t.random() % 3)
		moved := false
		for k := range 3 {
			e := (start + k) % 3
			a, b := tr[e], tr[(e+1)%3]
			if orient(t.pts[a], t.pts[b], p) < 0 {
				if j := t.across(a, b); j >= 0 {
					i = j
					moved = true
					break
				}
			}
		}
		if !moved {
			return i
		}
	}
	// The walk should always end; fall back to a search.
	for j := range t.tris {
		if t.tris[j].dead {
			continue
		}
		v := t.tris[j].v
		if orient(t.pts[v[0]], t.pts[v[1]], p) >= 0 && orient(t.pts[v[1]], t.pts[v[2]], p) >= 0 &&
			orient(t.pts[v[2]], t.pts[v[0]], p) >= 0 {
			return int32(j)
		}
	}
	return i
}

// insert adds point i by Bowyer and Watson's algorithm: the triangles
// whose circumcircle holds it are removed, and the hole is filled with
// a fan around it.
func (t *triangulation) insert(i int32) {
	p := t.pts[i]
	start := t.locate(p)
	cavity := []int32{start}
	in := map[int32]bool{start: true}
	for k := 0; k < len(cavity); k++ {
		tr := t.tris[cavity[k]].v
		for e := range 3 {
			j := t.across(tr[e], tr[(e+1)%3])
			if j < 0 || in[j] {
				continue
			}
			v := t.tris[j].v
			if incircle(t.pts[v[0]], t.pts[v[1]], t.pts[v[2]], p) > 0 {
				in[j] = true
				cavity = append(cavity, j)
			}
		}
	}
	// The cavity's boundary edges, kept before the triangles go.
	var rim []edge
	for _, c := range cavity {
		tr := t.tris[c].v
		for e := range 3 {
			a, b := tr[e], tr[(e+1)%3]
			if j := t.across(a, b); j < 0 || !in[j] {
				rim = append(rim, edge{a, b})
			}
		}
	}
	for _, c := range cavity {
		t.killTri(c)
	}
	for _, e := range rim {
		t.addTri(e[0], e[1], i)
	}
}

// findEdge is whether the triangulation has an edge between a and b.
func (t *triangulation) findEdge(a, b int32) bool {
	_, ok := t.edges[edge{a, b}]
	if !ok {
		_, ok = t.edges[edge{b, a}]
	}
	return ok
}

// fan lists the triangles around vertex a, starting at a live one.
func (t *triangulation) fan(a int32) []int32 {
	start := t.near[a]
	if t.tris[start].dead || !hasVertex(t.tris[start].v, a) {
		start = -1
		for j, v := range slices.Backward(t.tris) {
			if !v.dead && hasVertex(v.v, a) {
				start = int32(j)
				break
			}
		}
		if start < 0 {
			return nil
		}
		t.near[a] = start
	}
	out := []int32{start}
	// Turn one way, then if the fan is open the other.
	cur := start
	for {
		v := t.tris[cur].v
		k := vertexIndex(v, a)
		next := t.across(v[k], v[(k+1)%3])
		if next < 0 {
			break
		}
		if next == start {
			return out
		}
		out = append(out, next)
		cur = next
	}
	cur = start
	for {
		v := t.tris[cur].v
		k := vertexIndex(v, a)
		prev := t.across(v[(k+2)%3], v[k])
		if prev < 0 || prev == start {
			break
		}
		out = append(out, prev)
		cur = prev
	}
	return out
}

func hasVertex(v [3]int32, a int32) bool { return v[0] == a || v[1] == a || v[2] == a }

func vertexIndex(v [3]int32, a int32) int {
	for k := range 3 {
		if v[k] == a {
			return k
		}
	}
	return -1
}

// insertSegment makes a-b an edge, removing the triangles it crosses
// and filling each side again. A vertex on the segment splits it.
func (t *triangulation) insertSegment(a, b int32, depth int) {
	if a == b || depth > 1000 {
		return
	}
	if t.findEdge(a, b) {
		return
	}
	pa, pb := t.pts[a], t.pts[b]
	// The triangle at a whose wedge holds b.
	var u, w int32 = -1, -1
	var first int32 = -1
	for _, c := range t.fan(a) {
		v := t.tris[c].v
		k := vertexIndex(v, a)
		cu, cw := v[(k+1)%3], v[(k+2)%3]
		ou := orient(pa, pb, t.pts[cu])
		ow := orient(pa, pb, t.pts[cw])
		if ou == 0 && dot(pa, pb, t.pts[cu]) > 0 {
			// A vertex on the segment: split there.
			t.insertSegment(a, cu, depth+1)
			t.insertSegment(cu, b, depth+1)
			return
		}
		if ow == 0 && dot(pa, pb, t.pts[cw]) > 0 {
			t.insertSegment(a, cw, depth+1)
			t.insertSegment(cw, b, depth+1)
			return
		}
		if ou < 0 && ow > 0 {
			u, w, first = cu, cw, c
			break
		}
	}
	if first < 0 {
		t.cross = true
		return
	}
	crossed := []int32{first}
	left := []int32{w}
	right := []int32{u}
	stop := b
	for {
		next := t.across(u, w)
		if next < 0 {
			t.cross = true
			return
		}
		if t.isConstrained(u, w) {
			t.cross = true
		}
		crossed = append(crossed, next)
		v := t.tris[next].v
		x := v[(vertexIndex(v, w)+2)%3] // the triangle is (w, u, x)
		if x == b {
			break
		}
		o := orient(pa, pb, t.pts[x])
		if o == 0 {
			// The segment runs through x: end this part there.
			stop = x
			break
		}
		if o > 0 {
			left = append(left, x)
			w = x
		} else {
			right = append(right, x)
			u = x
		}
	}
	for _, c := range crossed {
		t.killTri(c)
	}
	t.fillLeft(left, a, stop)
	// The right side, seen from stop to a, is on its left.
	rev := make([]int32, len(right))
	for i := range right {
		rev[i] = right[len(right)-1-i]
	}
	t.fillLeft(rev, stop, a)
	if stop != b {
		t.insertSegment(stop, b, depth+1)
	}
}

func (t *triangulation) isConstrained(a, b int32) bool {
	k := edge{a, b}
	if a > b {
		k = edge{b, a}
	}
	return t.cons[k] > 0
}

// fillLeft triangulates the polygon p, chain..., q, whose chain lies to
// the left of p->q, choosing at each step the vertex whose triangle
// with p and q has an empty circumcircle.
func (t *triangulation) fillLeft(chain []int32, p, q int32) {
	if len(chain) == 0 {
		return
	}
	ci := 0
	pp, pq := t.pts[p], t.pts[q]
	for i := 1; i < len(chain); i++ {
		if incircle(pp, pq, t.pts[chain[ci]], t.pts[chain[i]]) > 0 {
			ci = i
		}
	}
	c := chain[ci]
	t.fillLeft(chain[:ci], p, c)
	t.fillLeft(chain[ci+1:], c, q)
	t.addTri(p, q, c)
}

// inside lists the triangles inside the segments by the even-odd
// rule, leaving out those at the outer triangle's corners (vertices n
// and up).
func (t *triangulation) inside(n int32) [][3]int32 {
	parity := make([]int8, len(t.tris))
	for i := range parity {
		parity[i] = -1
	}
	var queue []int32
	for i := range t.tris {
		tr := t.tris[i]
		if !tr.dead && (tr.v[0] >= n || tr.v[1] >= n || tr.v[2] >= n) {
			parity[i] = 0
			queue = append(queue, int32(i))
		}
	}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		v := t.tris[c].v
		for e := range 3 {
			a, b := v[e], v[(e+1)%3]
			j := t.across(a, b)
			if j < 0 || parity[j] >= 0 {
				continue
			}
			k := edge{a, b}
			if a > b {
				k = edge{b, a}
			}
			parity[j] = parity[c] ^ int8(t.cons[k]&1)
			queue = append(queue, j)
		}
	}
	var out [][3]int32
	for i, tr := range t.tris {
		if tr.dead || parity[i] != 1 || tr.v[0] >= n || tr.v[1] >= n || tr.v[2] >= n {
			continue
		}
		out = append(out, tr.v)
	}
	return out
}

func dot(a, b, c Point) float64 {
	return (b.X-a.X)*(c.X-a.X) + (b.Y-a.Y)*(c.Y-a.Y)
}

package srf

import (
	"math"
	"slices"
	"sync"

	"github.com/telesapr/telesapr/pkg/geom"
)

// surfaceIndex is a bounding-volume tree over the control-point boxes
// of a shell's surfaces. A Boolean asks, for every edge it classifies
// and every ray it casts, which surfaces a line meets; testing each
// surface of the other shell, and working out its box each time, made
// that quadratic in the surfaces (half of a 12 by 12 step and repeat).
//
// The tree only narrows the search. Every surface it gives is then put
// to the test the loops it replaces made (lineOutsideBox, on the same
// box), in the order of the shell's surfaces, so a search through the
// index finds what the loop over all of them found, in the same order.
type surfaceIndex struct {
	n     int // len(Surface) when built; a shell that has since changed is searched whole
	boxes []surfaceBox
	nodes []surfaceNode
	order []int32 // surface indices, each leaf a run of them

	// ends are the ends of the surfaces' trims by x, for
	// findVertsOnCurve: each with its place in the loop over the
	// surfaces, their trims and the two ends of each. An end with a
	// coordinate that is not a number has no place by x, and is in
	// odd, given to every search. They are sorted by the first search,
	// which a Boolean of surfaces with no exact curves never makes.
	ends, odd []trimEnd
	endsOnce  sync.Once
}

// trimEnd is an end of a trim, and where the loop over them reaches it.
type trimEnd struct {
	p   geom.Vector
	seq int
}

// surfaceBox is a surface's box, as AxisAlignedBounding gives it.
type surfaceBox struct{ max, min geom.Vector }

// surfaceNode is a node of the tree: a box around everything below it,
// and either two children (left, left+1) or, for a leaf, a run
// order[first:first+count].
type surfaceNode struct {
	box          surfaceBox
	left         int32 // first child; 0 for a leaf
	first, count int32
}

// surfaceIndexMin is the fewest surfaces a shell has an index for:
// below it, testing each one is as quick as building the tree.
const surfaceIndexMin = 32

// surfaceLeafSize is how many surfaces a leaf holds at most.
const surfaceLeafSize = 4

// indexSlack widens each box of the tree beyond the LengthEps by which
// lineOutsideBox widens it, against rounding, so that the tree never
// leaves out a surface that test would keep.
const indexSlack = 16 * geom.LengthEps

// newSurfaceIndex builds the index of the shell's surfaces as they are.
func newSurfaceIndex(sh *SShell) *surfaceIndex {
	n := len(sh.Surface)
	x := &surfaceIndex{n: n, boxes: make([]surfaceBox, n), order: make([]int32, n)}
	for i := range sh.Surface {
		b := &x.boxes[i]
		sh.Surface[i].AxisAlignedBounding(&b.max, &b.min)
		x.order[i] = int32(i)
	}
	if n > 0 {
		x.nodes = make([]surfaceNode, 1, 2*(n/surfaceLeafSize+1))
		x.build(0, 0, int32(n))
	}
	return x
}

// sortEnds makes ends and odd.
func (x *surfaceIndex) sortEnds(sh *SShell) {
	for i := range sh.Surface {
		for j := range sh.Surface[i].Trim {
			stb := &sh.Surface[i].Trim[j]
			for _, p := range [2]geom.Vector{stb.StartP, stb.FinishP} {
				e := trimEnd{p, len(x.ends) + len(x.odd)}
				if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsNaN(p.Z) {
					x.odd = append(x.odd, e)
				} else {
					x.ends = append(x.ends, e)
				}
			}
		}
	}
	slices.SortFunc(x.ends, func(a, b trimEnd) int {
		if c := cmpFloat(a.p.X, b.p.X); c != 0 {
			return c
		}
		return a.seq - b.seq
	})
}

// trimEndsNear appends to buf, and returns, the ends of the shell's
// trims that may lie in the box (amax, amin), as OutsideAndNotOn
// decides it, in the order of the loop over the surfaces, their trims
// and the two ends of each. ok is false when the index no longer fits
// the shell.
func (x *surfaceIndex) trimEndsNear(sh *SShell, amax, amin geom.Vector, buf []trimEnd) (near []trimEnd, ok bool) {
	if x == nil || x.n != len(sh.Surface) {
		return nil, false
	}
	x.endsOnce.Do(func() { x.sortEnds(sh) })
	near = buf[:0]
	near = append(near, x.odd...)
	lo, hi := amin.X-indexSlack, amax.X+indexSlack
	first, _ := slices.BinarySearchFunc(x.ends, lo, func(e trimEnd, t float64) int { return cmpFloat(e.p.X, t) })
	for _, e := range x.ends[first:] {
		if e.p.X > hi {
			break
		}
		if e.p.Y < amin.Y-indexSlack || e.p.Y > amax.Y+indexSlack ||
			e.p.Z < amin.Z-indexSlack || e.p.Z > amax.Z+indexSlack {
			continue
		}
		near = append(near, e)
	}
	slices.SortFunc(near, func(a, b trimEnd) int { return a.seq - b.seq })
	return near, true
}

// build makes node the root of the tree over order[first:last], split
// at the median of the boxes' centres along the longest side.
func (x *surfaceIndex) build(node, first, last int32) {
	box := surfaceBox{
		max: geom.Vector{X: veryNegative, Y: veryNegative, Z: veryNegative},
		min: geom.Vector{X: veryPositive, Y: veryPositive, Z: veryPositive},
	}
	for _, i := range x.order[first:last] {
		b := &x.boxes[i]
		b.max.MakeMaxMin(&box.max, &box.min)
		b.min.MakeMaxMin(&box.max, &box.min)
	}
	pad := geom.Vector{X: indexSlack, Y: indexSlack, Z: indexSlack}
	box.max, box.min = box.max.Plus(pad), box.min.Minus(pad)
	x.nodes[node] = surfaceNode{box: box, first: first, count: last - first}
	if last-first <= surfaceLeafSize {
		return
	}

	size := box.max.Minus(box.min)
	axis := 0
	if size.Y > size.X {
		axis = 1
	}
	if size.Z > size.Element(axis) {
		axis = 2
	}
	centre := func(i int32) float64 {
		b := &x.boxes[i]
		return b.max.Element(axis) + b.min.Element(axis)
	}
	slices.SortFunc(x.order[first:last], func(a, b int32) int {
		if c := cmpFloat(centre(a), centre(b)); c != 0 {
			return c
		}
		return int(a - b)
	})
	mid := first + (last-first)/2

	left := int32(len(x.nodes))
	x.nodes = append(x.nodes, surfaceNode{}, surfaceNode{})
	x.nodes[node].left = left
	x.build(left, first, mid)
	x.build(left+1, mid, last)
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// near appends to buf, and returns, the indices of the shell's surfaces
// whose box the line (a, b) meets, as lineOutsideBox decides it, in
// increasing order. It is nil, with ok false, when the index no longer
// fits the shell, which is then to be searched whole.
func (x *surfaceIndex) near(sh *SShell, a, b geom.Vector, asSegment bool, buf []int) (near []int, ok bool) {
	if x == nil || x.n != len(sh.Surface) {
		return nil, false
	}
	near = buf[:0]
	if x.n == 0 {
		return near, true
	}
	seg := surfaceBox{max: a, min: a}
	b.MakeMaxMin(&seg.max, &seg.min)
	d := b.Minus(a)

	var stack [64]int32
	stack[0] = 0
	top := 1
	for top > 0 {
		top--
		nd := &x.nodes[stack[top]]
		if !boxMayMeetLine(&nd.box, a, d, &seg, asSegment) {
			continue
		}
		if nd.left == 0 {
			for _, i := range x.order[nd.first : nd.first+nd.count] {
				bx := &x.boxes[i]
				if !lineOutsideBox(bx.max, bx.min, a, b, asSegment) {
					near = append(near, int(i))
				}
			}
			continue
		}
		stack[top], stack[top+1] = nd.left, nd.left+1
		top += 2
	}
	slices.Sort(near)
	return near, true
}

// boxMayMeetLine is a quick test that is false only when the line from
// a along d (the segment to a+d when asSegment, whose box is seg) can't
// meet box, which is widened already.
func boxMayMeetLine(box *surfaceBox, a, d geom.Vector, seg *surfaceBox, asSegment bool) bool {
	if asSegment {
		return !(seg.min.X > box.max.X || seg.max.X < box.min.X ||
			seg.min.Y > box.max.Y || seg.max.Y < box.min.Y ||
			seg.min.Z > box.max.Z || seg.max.Z < box.min.Z)
	}
	// The slabs: the parameters at which the line is within the box
	// along each axis must overlap.
	tmin, tmax := math.Inf(-1), math.Inf(1)
	for k := range 3 {
		ak, dk := a.Element(k), d.Element(k)
		lo, hi := box.min.Element(k), box.max.Element(k)
		if dk == 0 {
			if ak < lo || ak > hi {
				return false
			}
			continue
		}
		t0, t1 := (lo-ak)/dk, (hi-ak)/dk
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		tmin, tmax = math.Max(tmin, t0), math.Min(tmax, t1)
		if tmin > tmax {
			return false
		}
	}
	return true
}

// lineOutsideBox is LineEntirelyOutsideBbox for the box (amax, amin).
func lineOutsideBox(amax, amin, a, b geom.Vector, asSegment bool) bool {
	if !geom.BoundingBoxIntersectsLine(amax, amin, a, b, asSegment) {
		// The line segment could fail to intersect the bbox, but lie entirely
		// within it and intersect the surface.
		if a.OutsideAndNotOn(amax, amin) && b.OutsideAndNotOn(amax, amin) {
			return true
		}
	}
	return false
}

// surfacesNear is the indices of the shell's surfaces whose box the
// line (a, b) meets (LineEntirelyOutsideBbox is false), in order: from
// the index while the shell has one, or else by testing every surface.
// buf is reused for them.
func (s *SShell) surfacesNear(a, b geom.Vector, asSegment bool, buf []int) []int {
	if near, ok := s.index.near(s, a, b, asSegment, buf); ok {
		return near
	}
	all := buf[:0]
	for i := range s.Surface {
		if !s.Surface[i].LineEntirelyOutsideBbox(a, b, asSegment) {
			all = append(all, i)
		}
	}
	return all
}

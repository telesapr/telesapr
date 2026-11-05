package topo

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// splitPinchedLoops splits each of a surface's loops that passes a
// vertex twice, as the loop round two lobes of a face touching at one
// point does (a band's end face whose top touches a cone at a point,
// roadmap K3), into the two loops either side of the vertex, where both
// run the way an outer loop does: two faces, as the mesh makes them,
// where one loop counted one and the Euler check failed. A hole that
// touches its face's outer loop at a vertex is left one loop: its piece
// runs the other way, and the face it bounds is one disc.
func (t *Topology) splitPinchedLoops(s *srf.SSurface, loops [][]int) [][]int {
	var out [][]int
	for len(loops) > 0 {
		l := loops[len(loops)-1]
		loops = loops[:len(loops)-1]
		a, b, ok := t.pinch(s, l)
		if !ok {
			out = append(out, l)
			continue
		}
		loops = append(loops, a, b)
	}
	// The loops in the order chain gave them, the pieces of each in
	// place of it, so that faces are made in the same order as before.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// splitTouchingHoles takes apart each outer loop of a surface and a hole
// of it that touch at two or more vertices. Chained by the sharpest left
// turn, a plate's top whose fillet's foot just reaches both its sides
// comes out as the plate's outline with the foot's circle as a hole,
// touching at the two points where the foot meets the sides; the top is
// two faces there, one either side of the circle, touching at those
// points. The two loops' co-edges are chained again turning most
// sharply right, which goes round each piece alone, and that is kept
// where every loop it makes runs as an outer one does. A hole touching
// its outer loop at one vertex is one face with a hole, and is left so.
func (t *Topology) splitTouchingHoles(s *srf.SSurface, loops [][]int) [][]int {
	if len(loops) < 2 {
		return loops
	}
	for i, li := range loops {
		if !outerUV(t.loopUV(s, li)) {
			continue
		}
		at := map[int]bool{}
		for _, ce := range li {
			at[t.Start(ce)] = true
		}
		for j, lj := range loops {
			if j == i || t.sharedVertices(lj, at) < 2 || outerUV(t.loopUV(s, lj)) {
				continue
			}
			pieces, err := t.chainTurning(slices.Concat(li, lj), true)
			if err != nil || len(pieces) < 2 || !t.allOuter(s, pieces) {
				continue
			}
			var out [][]int
			for k, l := range loops {
				switch k {
				case i:
					out = append(out, pieces...)
				case j:
				default:
					out = append(out, l)
				}
			}
			return t.splitTouchingHoles(s, out)
		}
	}
	return loops
}

// sharedVertices counts the co-edges of loop l that start at a vertex
// of at.
func (t *Topology) sharedVertices(l []int, at map[int]bool) int {
	n := 0
	for _, ce := range l {
		if at[t.Start(ce)] {
			n++
		}
	}
	return n
}

// allOuter reports whether every loop runs as an outer loop does.
func (t *Topology) allOuter(s *srf.SSurface, loops [][]int) bool {
	for _, l := range loops {
		if !outerUV(t.loopUV(s, l)) {
			return false
		}
	}
	return true
}

// pinch finds a vertex the loop passes twice and, if the loop is an
// outer one and its two pieces either side of the vertex are outer
// loops too, gives the pieces.
func (t *Topology) pinch(s *srf.SSurface, l []int) (a, b []int, ok bool) {
	if len(l) < 4 {
		return nil, nil, false
	}
	seen := map[int]int{}
	outer := 0 // unknown until a vertex comes twice; projecting is dear
	for j, ce := range l {
		v := t.Start(ce)
		i, again := seen[v]
		if !again {
			seen[v] = j
			continue
		}
		if outer == 0 {
			outer = -1
			if outerUV(t.loopUV(s, l)) {
				outer = 1
			}
		}
		if outer < 0 {
			return nil, nil, false
		}
		a = append([]int(nil), l[i:j]...)
		b = append(append([]int(nil), l[j:]...), l[:i]...)
		if outerUV(t.loopUV(s, a)) && outerUV(t.loopUV(s, b)) {
			return a, b, true
		}
	}
	return nil, nil, false
}

// outerUV reports whether a loop's points in a surface's parameters run
// the way an outer loop does (makeFaces).
func outerUV(uv []geom.Vector) bool {
	return uvArea(uv) < 0
}

// uvArea is the signed area of a polygon in the surface's parameters,
// positive anticlockwise.
func uvArea(uv []geom.Vector) float64 {
	a := 0.0
	for k := range uv {
		p, q := uv[k], uv[(k+1)%len(uv)]
		a += (p.X*q.Y - q.X*p.Y) / 2
	}
	return a
}

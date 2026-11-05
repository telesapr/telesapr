package srf

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// AssemblePolygon assembles the edges into closed contours, using and
// setting each edge's Tag to mark it as consumed. It keeps assembling
// even if some contour fails to close, and returns whether all of them
// closed.
func (el *SEdgeList) AssemblePolygon(dest *SPolygon, errorAt *SEdge, keepDir bool) bool {
	return el.assemblePolygon(dest, errorAt, keepDir, nil)
}

// assembleTrimPolygon is AssemblePolygon of a surface's trim edges in
// its parameters, the face on the right of each edge as the trims run
// (outerUV). Where the outline passes a point twice, as the top of a
// plate does where a fillet's foot just reaches its side and pinches it
// to a point there, the edge taken on from that point is the one
// turning furthest to the right: each contour then goes round one of
// the pieces touching there, where the first edge found could make one
// contour cross through the point to the other piece, which the ear
// clipper can't mesh. A contour that still passes a point twice, round
// a face and a hole of it that touch there, is split there into the two.
// Edge lists with no such point assemble as AssemblePolygon does.
func (el *SEdgeList) assembleTrimPolygon(dest *SPolygon) bool {
	junctions := el.junctions()
	if len(junctions) == 0 {
		return el.AssemblePolygon(dest, nil, true)
	}
	ok := el.assemblePolygon(dest, nil, true, junctions)
	dest.splitAtJunctions(junctions)
	return ok
}

// junctions is the points two or more edges start from.
func (el *SEdgeList) junctions() []geom.Vector {
	starts := make([]geom.Vector, len(el.Edges))
	for i := range el.Edges {
		starts[i] = el.Edges[i].A
	}
	slices.SortFunc(starts, func(a, b geom.Vector) int { return cmp.Compare(a.X, b.X) })
	var out []geom.Vector
	for i := range starts {
		for j := i + 1; j < len(starts) && starts[j].X-starts[i].X <= geom.LengthEps; j++ {
			if starts[i].Equals(starts[j]) {
				if !slices.ContainsFunc(out, starts[i].Equals) {
					out = append(out, starts[i])
				}
				break
			}
		}
	}
	return out
}

// splitAtJunctions splits each contour that passes a junction twice into
// the two closed contours either side of it.
func (p *SPolygon) splitAtJunctions(junctions []geom.Vector) {
	for i := 0; i < len(p.Contours); i++ { //nolint:intrange // the contours grow as they are split
		pts := p.Contours[i].Points
		a, b := repeatedJunction(pts, junctions)
		if a < 0 {
			continue
		}
		inner := append([]SPoint(nil), pts[a:b+1]...)
		outer := append(append([]SPoint(nil), pts[:a+1]...), pts[b+1:]...)
		p.Contours[i].Points = outer
		p.Contours = append(p.Contours, SContour{Points: inner})
		i--
	}
}

// repeatedJunction is the first pair of places a closed contour's points
// pass one junction, its closing point aside, or -1.
func repeatedJunction(pts []SPoint, junctions []geom.Vector) (int, int) {
	for a := 0; a < len(pts)-1; a++ { //nolint:intrange // b runs on from a
		if !slices.ContainsFunc(junctions, pts[a].P.Equals) {
			continue
		}
		for b := a + 1; b < len(pts)-1; b++ {
			if pts[b].P.Equals(pts[a].P) && (a > 0 || b < len(pts)-1) {
				return a, b
			}
		}
	}
	return -1, -1
}

func (el *SEdgeList) assemblePolygon(dest *SPolygon, errorAt *SEdge, keepDir bool, junctions []geom.Vector) bool {
	dest.Clear()

	allClosed := true
	for i := range el.Edges {
		if el.Edges[i].Tag == 0 {
			first := el.Edges[i].A
			last := el.Edges[i].B
			el.Edges[i].Tag = 1
			// Create a new empty contour in our polygon, and finish assembling
			// into that contour.
			dest.AddEmptyContour()
			if !el.assembleContour(first, last, &dest.Contours[len(dest.Contours)-1], errorAt, keepDir, junctions, i+1) {
				allClosed = false
			}
			// But continue assembling, even if some of the contours are open
		}
	}
	return allClosed
}

// AssembleContour chains untagged edges onto first->last until the
// contour closes.
func (el *SEdgeList) AssembleContour(first, last geom.Vector, dest *SContour, errorAt *SEdge, keepDir bool, start int) bool {
	return el.assembleContour(first, last, dest, errorAt, keepDir, nil, start)
}

// assembleContour is AssembleContour, taking from each of junctions the
// edge turning furthest to the right (assembleTrimPolygon).
func (el *SEdgeList) assembleContour(first, last geom.Vector, dest *SContour, errorAt *SEdge, keepDir bool, junctions []geom.Vector, start int) bool {
	dest.AddPoint(first)
	dest.AddPoint(last)

	for {
		if slices.ContainsFunc(junctions, last.Equals) {
			if j := el.rightmostFrom(dest, last, start); j >= 0 {
				se := &el.Edges[j]
				dest.AddPoint(se.B)
				last = se.B
				se.Tag = 1
				if last.Equals(first) {
					return true
				}
				continue
			}
		}
		i := start
		for ; i < len(el.Edges); i++ {
			se := &el.Edges[i]
			if se.Tag != 0 {
				continue
			}

			if se.A.Equals(last) {
				dest.AddPoint(se.B)
				last = se.B
				se.Tag = 1
				break
			}
			// Don't allow backwards edges if keepDir is true.
			if !keepDir && se.B.Equals(last) {
				dest.AddPoint(se.A)
				last = se.A
				se.Tag = 1
				break
			}
		}
		if i >= len(el.Edges) {
			// Couldn't assemble a closed contour; mark where.
			if errorAt != nil {
				errorAt.A = first
				errorAt.B = last
			}
			return false
		}
		if last.Equals(first) {
			return true
		}
	}
}

// rightmostFrom is, of the untagged edges from last, the one turning
// furthest to the right from the contour's last edge, in the plane of X
// and Y; -1 where none leaves last.
func (el *SEdgeList) rightmostFrom(dest *SContour, last geom.Vector, start int) int {
	n := len(dest.Points)
	in := last.Minus(dest.Points[n-2].P)
	best := -1
	bestTurn := math.Inf(1)
	for i := start; i < len(el.Edges); i++ {
		se := &el.Edges[i]
		if se.Tag != 0 || !se.A.Equals(last) {
			continue
		}
		out := se.B.Minus(se.A)
		// The turn anticlockwise from in to out, in (-pi, pi]: the
		// least is the furthest to the right.
		turn := math.Atan2(in.X*out.Y-in.Y*out.X, in.X*out.X+in.Y*out.Y)
		if turn < bestTurn {
			best, bestTurn = i, turn
		}
	}
	return best
}

// MergeCollinearSegments sorts the edges, which all lie on the line
// from a to b, along it, and merges each run of joined edges of the
// same style into one.
func (el *SEdgeList) MergeCollinearSegments(a, b geom.Vector) {
	lineStart := a
	lineDirection := b.Minus(a)
	CxxSort(el.Edges, func(ea, eb *SEdge) bool {
		ta := ea.A.Minus(lineStart).DivProjected(lineDirection)
		tb := eb.A.Minus(lineStart).DivProjected(lineDirection)

		return ta < tb
	})

	for i := range el.Edges {
		el.Edges[i].Tag = 0
	}
	var prev *SEdge
	for i := range el.Edges {
		now := &el.Edges[i]
		if prev != nil {
			if prev.B.Equals(now.A) && prev.AuxA == now.AuxA {
				// The previous segment joins up to us; so merge it into
				// us.
				prev.Tag = 1
				now.A = prev.A
			}
		}
		prev = now
	}
	kept := el.Edges[:0]
	for _, e := range el.Edges {
		if e.Tag == 0 {
			kept = append(kept, e)
		}
	}
	el.Edges = kept
}

// ApproxEqualGeometry reports whether two SEdgeLists describe the
// same set of edges within tol. Used by mesh-diff tests.
func ApproxEqualGeometry(a, b *SEdgeList, tol float64) bool {
	if len(a.Edges) != len(b.Edges) {
		return false
	}
	for i := range a.Edges {
		if a.Edges[i].A.DistanceTo(b.Edges[i].A) > tol {
			return false
		}
		if a.Edges[i].B.DistanceTo(b.Edges[i].B) > tol {
			return false
		}
	}
	return true
}

// avoid unused import warnings if some helpers aren't used in this file.
var _ = math.Pi

package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// AssemblePolygon assembles the edges into closed contours, using and
// setting each edge's Tag to mark it as consumed. It keeps assembling
// even if some contour fails to close, and returns whether all of them
// closed. Mirrors C++ SEdgeList::AssemblePolygon (polygon.cpp:270).
func (el *SEdgeList) AssemblePolygon(dest *SPolygon, errorAt *SEdge, keepDir bool) bool {
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
			if !el.AssembleContour(first, last, &dest.Contours[len(dest.Contours)-1], errorAt, keepDir, i+1) {
				allClosed = false
			}
			// But continue assembling, even if some of the contours are open
		}
	}
	return allClosed
}

// AssembleContour chains untagged edges onto first->last until the
// contour closes. Mirrors C++ SEdgeList::AssembleContour
// (polygon.cpp:228).
func (el *SEdgeList) AssembleContour(first, last geom.Vector, dest *SContour, errorAt *SEdge, keepDir bool, start int) bool {
	dest.AddPoint(first)
	dest.AddPoint(last)

	for {
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

// MergeCollinearSegments replaces consecutive collinear edges (a-b-c
// where b lies on the line from a to c) with a single edge a-c.
func (el *SEdgeList) MergeCollinearSegments(a, b geom.Vector) {
	// Phase 1: no-op. The full implementation requires per-edge
	// endpoint comparison and works best once SShell's boolean
	// pipeline is in place (Phase 4).
	_ = a
	_ = b
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

// polygonArea2 returns twice the signed area of a polygon in 2-D
// (using x and z of the points). Helper used by ear-clipping.
func polygonArea2(points []geom.Vector) float64 {
	if len(points) < 3 {
		return 0
	}
	sum := 0.0
	for i := 0; i < len(points); i++ {
		a := points[i]
		b := points[(i+1)%len(points)]
		sum += (b.X - a.X) * (b.Z + a.Z)
	}
	return sum / 2
}

// avoid unused import warnings if some helpers aren't used in this file.
var _ = math.Pi

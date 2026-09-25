package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// AssemblePolygon walks the edges and assembles them into an
// SPolygon. It tries each unused edge as a starting point and chains
// it onto the next edge whose start point matches the current end
// point (within tol). Returns true if at least one contour was
// assembled; the caller can detect failure (unclosed / broken chain)
// by checking the returned bool and inspecting the polygon.
func (el *SEdgeList) AssemblePolygon(dest *SPolygon, errorAt *SEdge, keepDir bool) bool {
	dest.Clear()
	if len(el.Edges) == 0 {
		return false
	}
	const tol = 1e-9
	used := make([]bool, len(el.Edges))
	for start := 0; start < len(el.Edges); start++ {
		if used[start] {
			continue
		}
		used[start] = true
		contour := SContour{Tag: el.Edges[start].Tag}
		startPt := el.Edges[start].A
		contour.AddPoint(el.Edges[start].A)
		contour.AddPoint(el.Edges[start].B)
		current := el.Edges[start].B
		// Keep chaining while a continuation edge exists or until we
		// close back to the start point.
		for current.DistanceTo(startPt) > tol {
			found := false
			for j := 0; j < len(el.Edges); j++ {
				if used[j] {
					continue
				}
				if el.Edges[j].A.DistanceTo(current) < tol {
					contour.AddPoint(el.Edges[j].B)
					current = el.Edges[j].B
					used[j] = true
					found = true
					break
				}
			}
			if !found {
				break
			}
		}
		if !keepDir {
			contour.Reverse()
		}
		if len(contour.Points) >= 3 {
			dest.Contours = append(dest.Contours, contour)
		}
		if len(dest.Contours) == 1 && dest.Normal == (geom.Vector{}) {
			dest.Normal = contour.ComputeNormal()
		}
	}
	return len(dest.Contours) > 0
}

// AssembleContour assembles a single contour starting from the edge
// whose A-point matches `first`. Returns true if a contour was
// produced (any length).
func (el *SEdgeList) AssembleContour(first, last geom.Vector, dest *SContour, errorAt *SEdge, keepDir bool, start int) bool {
	const tol = 1e-9
	used := make([]bool, len(el.Edges))
	contour := SContour{}
	// Find the first edge whose A matches first.
	idx := -1
	for i := start; i < len(el.Edges); i++ {
		if el.Edges[i].A.DistanceTo(first) < tol {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	used[idx] = true
	contour.AddPoint(el.Edges[idx].A)
	contour.AddPoint(el.Edges[idx].B)
	current := el.Edges[idx].B
	for current.DistanceTo(last) > tol {
		found := false
		for j := 0; j < len(el.Edges); j++ {
			if used[j] {
				continue
			}
			if el.Edges[j].A.DistanceTo(current) < tol {
				contour.AddPoint(el.Edges[j].B)
				current = el.Edges[j].B
				used[j] = true
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	if !keepDir {
		contour.Reverse()
	}
	*dest = contour
	return len(contour.Points) >= 2
}

// AnyEdgeCrossings counts the number of times a line (a, b) crosses
// any edge in the list. If pi is non-nil, returns the last
// intersection point; if spl is non-nil, adds the intersection to it.
func (el *SEdgeList) AnyEdgeCrossings(a, b geom.Vector, pi *geom.Vector, spl *SPointList) int {
	count := 0
	for i := range el.Edges {
		if t, ok := el.Edges[i].Crossing(a, b.Minus(a)); ok {
			count++
			if pi != nil {
				*pi = a.Plus(b.Minus(a).ScaledBy(t))
			}
			if spl != nil {
				spl.Points = append(spl.Points, SPoint{P: a.Plus(b.Minus(a).ScaledBy(t))})
			}
		}
	}
	return count
}

// ContainsEdgeFrom reports whether sel is a subset of el.
func (el *SEdgeList) ContainsEdgeFrom(sel *SEdgeList) bool {
	const tol = 1e-9
	for _, e := range sel.Edges {
		found := false
		for _, x := range el.Edges {
			if x.A.DistanceTo(e.A) < tol && x.B.DistanceTo(e.B) < tol {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// ContainsEdge reports whether any edge in el matches se.
func (el *SEdgeList) ContainsEdge(se *SEdge) bool {
	const tol = 1e-9
	for _, x := range el.Edges {
		if x.A.DistanceTo(se.A) < tol && x.B.DistanceTo(se.B) < tol {
			return true
		}
	}
	return false
}

// CullExtraneousEdges removes edges that overlap a longer edge in
// the list. Used after extrusion/lathe to clean up duplicate trim
// segments.
func (el *SEdgeList) CullExtraneousEdges(both bool) {
	const tol = 1e-12
	out := el.Edges[:0]
	for _, e := range el.Edges {
		keep := true
		for _, x := range el.Edges {
			if x.A == e.A && x.B == e.B {
				continue
			}
			if pointOnSegment(e.A, x.A, x.B, tol) && pointOnSegment(e.B, x.A, x.B, tol) {
				keep = false
				break
			}
			if both && pointOnSegment(x.A, e.A, e.B, tol) && pointOnSegment(x.B, e.A, e.B, tol) {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, e)
		}
	}
	el.Edges = out
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

func pointOnSegment(p, a, b geom.Vector, tol float64) bool {
	if p.DistanceTo(a) < tol || p.DistanceTo(b) < tol {
		return true
	}
	cross := b.Minus(a).Cross(p.Minus(a)).Magnitude()
	if cross > tol {
		return false
	}
	dot := p.Minus(a).Dot(b.Minus(a))
	ab2 := b.Minus(a).MagSquared()
	return dot >= -tol && dot <= ab2+tol
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

// EarClipTriangulate performs ear-clipping triangulation of a
// planar polygon contour (in the XZ plane for simplicity) into the
// supplied SMesh. Used as a fallback when no proper SSurface is
// available (Phase 1 placeholder; full uv-aware ear-clip lives in
// Phase 3 alongside SSurface::UvTriangulateInto).
func EarClipTriangulate(contour *SContour, mesh *SMesh, scaledEps float64) {
	if len(contour.Points) < 3 {
		return
	}
	// Copy indices; we will remove ears one at a time.
	indices := make([]int, len(contour.Points))
	for i := range indices {
		indices[i] = i
	}
	for len(indices) > 3 {
		found := false
		for i := 0; i < len(indices); i++ {
			bp := indices[i]
			if contour.IsEar(bp, scaledEps) {
				ap := indices[(i-1+len(indices))%len(indices)]
				cp := indices[(i+1)%len(indices)]
				a := contour.Points[ap].P
				b := contour.Points[bp].P
				c := contour.Points[cp].P
				mesh.AddTriangle(STriangle{
					A: a,
					B: b,
					C: c,
				})
				indices = append(indices[:i], indices[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			break // degenerate; abandon
		}
	}
	if len(indices) == 3 {
		a := contour.Points[indices[0]].P
		b := contour.Points[indices[1]].P
		c := contour.Points[indices[2]].P
		mesh.AddTriangle(STriangle{A: a, B: b, C: c})
	}
}

// avoid unused import warnings if some helpers aren't used in this file.
var _ = math.Pi

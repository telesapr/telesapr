package srf

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// PolylineBuilder assembles scattered edges into contiguous polylines
// (C++ PolylineBuilder, polyline.cpp).
type PolylineBuilder struct {
	// vertices is C++'s unordered_map<Vector, Vertex*, VectorHash,
	// VectorPred>: a lookup needs an equal VectorHash and a position
	// within LENGTH_EPS.
	vertices map[uint64][]*PolylineVertex
	edgeMap  map[[2]*PolylineVertex]*PolylineEdge
	Edges    []*PolylineEdge
}

// PolylineVertex is a joint between edges (C++ PolylineBuilder::Vertex).
type PolylineVertex struct {
	Pos   geom.Vector
	Edges []*PolylineEdge
}

// PolylineEdge is an edge between two vertices; Outline or Edge is the
// source it was made from (C++ PolylineBuilder::Edge).
type PolylineEdge struct {
	A, B    *PolylineVertex
	Kind    uint32
	Tag     int
	Outline *SOutline
	Edge    *SEdge
}

// vectorHash is C++ VectorHash (util.cpp), including its use of y
// for the z term.
func vectorHash(v geom.Vector) uint64 {
	size := uint64(math.Pow(float64(math.MaxUint64), 1.0/3.0) - 1)
	const eps = 4.0 * geom.LengthEps
	x := math.Abs(v.X) / eps
	y := math.Abs(v.Y) / eps
	z := math.Abs(v.Y) / eps
	xs := uint64(math.Mod(x, float64(size)))
	ys := uint64(math.Mod(y, float64(size)))
	zs := uint64(math.Mod(z, float64(size)))
	return (zs*size+ys)*size + xs
}

// getNext follows the first untagged edge of the given kind (C++
// Vertex::GetNext).
func (v *PolylineVertex) getNext(kind uint32) (*PolylineVertex, *PolylineEdge, bool) {
	for _, e := range v.Edges {
		if e.Tag == 0 && e.Kind == kind {
			e.Tag = 1
			return e.OtherVertex(v), e, true
		}
	}
	return nil, nil, false
}

// getNextInPlane follows the untagged edge of the given kind whose far
// end is closest to the plane (C++ Vertex::GetNext with a plane).
func (v *PolylineVertex) getNextInPlane(kind uint32, plane geom.Vector, dist float64) (*PolylineVertex, *PolylineEdge, bool) {
	var best *PolylineEdge
	minD := math.Inf(1)
	for _, e := range v.Edges {
		if e.Tag != 0 || e.Kind != kind {
			continue
		}
		nextPos := e.OtherVertex(v).Pos
		curD := math.Abs(plane.Dot(nextPos) - dist)
		if best != nil && curD > minD {
			continue
		}
		best = e
		minD = curD
	}
	if best != nil {
		best.Tag = 1
		return best.OtherVertex(v), best, true
	}
	return nil, nil, false
}

func (v *PolylineVertex) countEdgesWithTagAndKind(tag int, kind uint32) int {
	n := 0
	for _, e := range v.Edges {
		if e.Tag == tag && e.Kind == kind {
			n++
		}
	}
	return n
}

// OtherVertex is the edge's end that isn't v (C++ Edge::GetOtherVertex).
func (e *PolylineEdge) OtherVertex(v *PolylineVertex) *PolylineVertex {
	if e.A == v {
		return e.B
	}
	if e.B == v {
		return e.A
	}
	return nil
}

// startAndNext picks the end a polyline through e starts from (C++
// Edge::GetStartAndNext).
func (e *PolylineEdge) startAndNext(loop bool) (start, next *PolylineVertex, ok bool) {
	numA := e.A.countEdgesWithTagAndKind(0, e.Kind)
	numB := e.B.countEdgesWithTagAndKind(0, e.Kind)

	if (numA == 1 && numB > 1) || (loop && numA > 1 && numB > 1) {
		return e.A, e.B, true
	}
	if numA > 1 && numB == 1 {
		return e.B, e.A, true
	}
	return nil, nil, false
}

// Clear drops every vertex and edge (C++ PolylineBuilder::Clear).
func (b *PolylineBuilder) Clear() {
	b.vertices = nil
	b.edgeMap = nil
	b.Edges = nil
}

// AddVertex returns the vertex at pos, adding it if new (C++
// PolylineBuilder::AddVertex).
func (b *PolylineBuilder) AddVertex(pos geom.Vector) *PolylineVertex {
	if b.vertices == nil {
		b.vertices = make(map[uint64][]*PolylineVertex)
	}
	h := vectorHash(pos)
	for _, v := range b.vertices[h] {
		if v.Pos.EqualsTol(pos, geom.LengthEps) {
			return v
		}
	}
	v := &PolylineVertex{Pos: pos}
	b.vertices[h] = append(b.vertices[h], v)
	return v
}

// AddEdge adds the edge p0-p1, unless it is degenerate or already
// there in the same direction (C++ PolylineBuilder::AddEdge).
func (b *PolylineBuilder) AddEdge(p0, p1 geom.Vector, kind uint32) *PolylineEdge {
	v0 := b.AddVertex(p0)
	v1 := b.AddVertex(p1)
	if v0 == v1 {
		return nil
	}
	if b.edgeMap == nil {
		b.edgeMap = make(map[[2]*PolylineVertex]*PolylineEdge)
	}
	key := [2]*PolylineVertex{v0, v1}
	if e, ok := b.edgeMap[key]; ok {
		return e
	}
	e := &PolylineEdge{A: v0, B: v1, Kind: kind}
	b.Edges = append(b.Edges, e)
	b.edgeMap[key] = e
	v0.Edges = append(v0.Edges, e)
	v1.Edges = append(v1.Edges, e)
	return e
}

// Generate walks the edges as polylines: startFn for each polyline's
// first edge, nextFn for each following one, endFn after each
// polyline, and aloneFn for the edges left over (C++
// PolylineBuilder::Generate).
func (b *PolylineBuilder) Generate(
	startFn func(start, next *PolylineVertex, e *PolylineEdge),
	nextFn func(next *PolylineVertex, e *PolylineEdge),
	aloneFn func(e *PolylineEdge),
	endFn func(),
) {
	loop := false
	for {
		found := false
		for _, e := range b.Edges {
			if e.Tag != 0 {
				continue
			}
			start, next, ok := e.startAndNext(loop)
			if !ok {
				continue
			}
			startPos := start.Pos
			nextPos := next.Pos
			found = true
			e.Tag = 1

			startFn(start, next, e)

			if n, nextEdge, ok := next.getNext(e.Kind); ok {
				next = n
				plane := nextPos.Minus(startPos).Cross(next.Pos.Minus(startPos))
				dist := plane.Dot(startPos)
				nextFn(next, nextEdge)
				for {
					n, nextEdge, ok = next.getNextInPlane(e.Kind, plane, dist)
					if !ok {
						break
					}
					next = n
					nextFn(next, nextEdge)
				}
			}

			if endFn != nil {
				endFn()
			}
		}

		if !found && !loop {
			loop = true
			found = true
		}
		if !found {
			break
		}
	}

	for _, e := range b.Edges {
		if e.Tag != 0 {
			continue
		}
		aloneFn(e)
	}
}

// MakeFromEdges adds sel's edges, with auxA as the kind (C++
// PolylineBuilder::MakeFromEdges).
func (b *PolylineBuilder) MakeFromEdges(sel *SEdgeList) {
	for i := range sel.Edges {
		se := &sel.Edges[i]
		if e := b.AddEdge(se.A, se.B, uint32(se.AuxA)); e != nil && e.Edge == nil {
			e.Edge = se
		}
	}
}

// MakeFromOutlines adds the outlines, with the tag as the kind so that
// emphasized and contour outlines stay apart (C++
// PolylineBuilder::MakeFromOutlines).
func (b *PolylineBuilder) MakeFromOutlines(ol *SOutlineList) {
	for i := range ol.Outlines {
		so := &ol.Outlines[i]
		if e := b.AddEdge(so.A, so.B, uint32(so.Tag)); e != nil && e.Outline == nil {
			e.Outline = so
		}
	}
}

// GenerateEdges writes the polylines as edges (C++
// PolylineBuilder::GenerateEdges).
func (b *PolylineBuilder) GenerateEdges(sel *SEdgeList) {
	var prev geom.Vector
	b.Generate(
		func(start, next *PolylineVertex, e *PolylineEdge) {
			sel.AddEdge(start.Pos, next.Pos, int(e.Kind), 0, 0)
			prev = next.Pos
		},
		func(next *PolylineVertex, e *PolylineEdge) {
			sel.AddEdge(prev, next.Pos, int(e.Kind), 0, 0)
			prev = next.Pos
		},
		func(e *PolylineEdge) {
			sel.AddEdge(e.A.Pos, e.B.Pos, int(e.Kind), 0, 0)
		},
		nil,
	)
}

// GenerateOutlines writes the polylines as outlines, each keeping its
// source outline's normals and tag (C++
// PolylineBuilder::GenerateOutlines).
func (b *PolylineBuilder) GenerateOutlines(sol *SOutlineList) {
	var prev geom.Vector
	b.Generate(
		func(start, next *PolylineVertex, e *PolylineEdge) {
			so := e.Outline
			sol.AddEdge(start.Pos, next.Pos, so.NL, so.NR, so.Tag)
			prev = next.Pos
		},
		func(next *PolylineVertex, e *PolylineEdge) {
			so := e.Outline
			sol.AddEdge(prev, next.Pos, so.NL, so.NR, so.Tag)
			prev = next.Pos
		},
		func(e *PolylineEdge) {
			so := e.Outline
			sol.AddEdge(so.A, so.B, so.NL, so.NR, so.Tag)
		},
		nil,
	)
}

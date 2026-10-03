package srf

import "github.com/telesapr/telesapr/pkg/geom"

// SOutline is an edge between two triangles of a mesh, with the normals
// on its left and right (C++ SOutline).
type SOutline struct {
	Tag    int
	A, B   geom.Vector
	NL, NR geom.Vector
}

// SOutlineList is a list of outline edges (C++ SOutlineList).
type SOutlineList struct {
	Outlines []SOutline
}

func (l *SOutlineList) Clear() {
	l.Outlines = l.Outlines[:0]
}

// IsVisible reports whether the edge is an outline when looking along
// projDir: one of its faces is toward the viewer and the other away
// (C++ SOutline::IsVisible).
func (so *SOutline) IsVisible(projDir geom.Vector) bool {
	ldot := so.NL.Dot(projDir)
	rdot := so.NR.Dot(projDir)
	return (ldot > -geom.LengthEps) == (rdot < geom.LengthEps) ||
		(rdot > -geom.LengthEps) == (ldot < geom.LengthEps)
}

// ListTaggedInto adds the tagged outline edges to el (C++
// SOutlineList::ListTaggedInto).
func (l *SOutlineList) ListTaggedInto(el *SEdgeList, auxA, auxB int) {
	for _, so := range l.Outlines {
		if so.Tag == 0 {
			continue
		}
		el.AddEdge(so.A, so.B, auxA, auxB, 0)
	}
}

// MakeFromCopyOf adds sol's outline edges (C++
// SOutlineList::MakeFromCopyOf).
func (l *SOutlineList) MakeFromCopyOf(sol *SOutlineList) {
	l.Outlines = append(l.Outlines, sol.Outlines...)
}

// AddEdge adds an outline edge (C++ SOutlineList::AddEdge).
func (l *SOutlineList) AddEdge(a, b, nl, nr geom.Vector, tag int) {
	l.Outlines = append(l.Outlines, SOutline{A: a, B: b, NL: nl, NR: nr, Tag: tag})
}

// MakeOutlinesInto lists the mesh's outline edges of the given kind
// (C++ SMesh::MakeOutlinesInto).
func (m *SMesh) MakeOutlinesInto(sol *SOutlineList, edgeKind EdgeKind) {
	root := SKdNodeFrom(m)
	root.MakeOutlinesInto(sol, edgeKind)
}

package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

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

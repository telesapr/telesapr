package srf

import "github.com/telesapr/telesapr/pkg/geom"

// SOutline is an edge between two triangles of a mesh, with the normals
// on its left and right.
type SOutline struct {
	Tag    int
	A, B   geom.Vector
	NL, NR geom.Vector
}

// SOutlineList is a list of outline edges.
type SOutlineList struct {
	Outlines []SOutline
}

func (l *SOutlineList) Clear() {
	l.Outlines = l.Outlines[:0]
}

// IsVisible reports whether the edge is an outline when looking along
// projDir: one of its faces is toward the viewer and the other away.
func (so *SOutline) IsVisible(projDir geom.Vector) bool {
	ldot := so.NL.Dot(projDir)
	rdot := so.NR.Dot(projDir)
	return (ldot > -geom.LengthEps) == (rdot < geom.LengthEps) ||
		(rdot > -geom.LengthEps) == (ldot < geom.LengthEps)
}

// ListTaggedInto adds the tagged outline edges to el.
func (l *SOutlineList) ListTaggedInto(el *SEdgeList, auxA, auxB int) {
	for _, so := range l.Outlines {
		if so.Tag == 0 {
			continue
		}
		el.AddEdge(so.A, so.B, auxA, auxB, 0)
	}
}

// MakeFromCopyOf adds sol's outline edges.
func (l *SOutlineList) MakeFromCopyOf(sol *SOutlineList) {
	l.Outlines = append(l.Outlines, sol.Outlines...)
}

// AddEdge adds an outline edge.
func (l *SOutlineList) AddEdge(a, b, nl, nr geom.Vector, tag int) {
	l.Outlines = append(l.Outlines, SOutline{A: a, B: b, NL: nl, NR: nr, Tag: tag})
}

// MakeOutlinesInto lists the mesh's outline edges of the given kind: the
// edges two triangles share end for end, tagged 1 where they are
// emphasized or sharp; untagged edges between triangles with the same
// normal are left out, since they cannot produce an outline.
func (m *SMesh) MakeOutlinesInto(sol *SOutlineList, edgeKind EdgeKind) {
	// The triangles are the tree's copies of the mesh's, so that they are
	// read in the order SolveSpace reads them in, which is the order the
	// outlines come out in: an edge is listed as the first of the two
	// triangles that have it, and so gets that one's direction and
	// normals. The exports are ordered by it too, and are compared with
	// SolveSpace's bytes.
	root := SKdNodeFrom(m)
	var tris []*STriangle
	root.ClearTags()
	root.ListTrianglesInto(&tris)
	ix := newMateIndex(tris)

	edgeTris := map[triPair]bool{}
	for _, tr := range tris {
		for j := range 3 {
			a := tr.vertex(j)
			b := tr.vertex((j + 1) % 3)

			var info EdgeOnInfo
			ix.find(a, b, &info)
			if info.Count != 1 {
				continue
			}
			if checkAndAddTrianglePair(edgeTris, tr, info.Tr) {
				continue
			}

			tag := 0
			switch edgeKind {
			case EdgeEmphasized:
				if tr.Meta.Face != info.Tr.Meta.Face {
					tag = 1
				}
			case EdgeSharp:
				if isSharp(tr, j, &info) {
					tag = 1
				}
			case EdgeSharpOrFace:
				if tr.Meta.Face != info.Tr.Meta.Face || isSharp(tr, j, &info) {
					tag = 1
				}
			default:
				panic("srf: unexpected edge kind")
			}

			nl := tr.Normal().WithMagnitude(1.0)
			nr := info.Tr.Normal().WithMagnitude(1.0)

			// We don't add edges with the same left and right
			// normals because they can't produce outlines.
			if tag == 0 && nl.Equals(nr) {
				continue
			}
			sol.AddEdge(a, b, nl, nr, tag)
		}
	}
}

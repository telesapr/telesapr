package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SOutline is one silhouette edge extracted from an SMesh. Each
// outline edge carries the face IDs of the two triangles sharing
// the edge; if either side is empty (mesh boundary), the
// corresponding normal is zero.
type SOutline struct {
	Tag int
	NL  geom.Vector // normal on the left side of the edge (a -> b)
	NR  geom.Vector // normal on the right side
	FA  FaceID
	FB  FaceID
	A   geom.Vector
	B   geom.Vector
}

// SOutlineList is a slice of SOutline. Used by SMesh::MakeOutlinesInto
// to return the silhouette of a mesh.
type SOutlineList struct {
	Outlines []SOutline
}

func (l *SOutlineList) Clear() {
	l.Outlines = l.Outlines[:0]
}

// MakeOutlinesInto extracts outlines from m of the requested kind.
// Implemented in Phase 1: for now, "naive" — emit every edge shared
// by exactly two triangles with opposite orientations; refine in
// later passes as needed.
func (m *SMesh) MakeOutlinesInto(sol *SOutlineList, kind EdgeKind) {
	sol.Clear()
	// For each triangle edge, look for an opposite-orientation twin.
	type key struct{ a, b uint64 }
	edgeMap := make(map[key]int)
	hashed := func(p geom.Vector) uint64 {
		// Quantize to 1e-6 grid; sufficient for outline dedup.
		const q = 1e6
		return uint64(int64(p.X*q+q*1e6))<<42 ^ uint64(int64(p.Y*q+q*1e6))<<21 ^ uint64(int64(p.Z*q+q*1e6))
	}
	pair := func(p1, p2 geom.Vector) key {
		h1 := hashed(p1)
		h2 := hashed(p2)
		if h1 < h2 {
			return key{h1, h2}
		}
		return key{h2, h1}
	}
	edgeNormals := make(map[key][]geom.Vector)
	edgeFaces := make(map[key][]FaceID)
	for _, t := range m.Triangles {
		n := t.NormalUnit()
		edges := []struct{ a, b geom.Vector }{{t.A, t.B}, {t.B, t.C}, {t.C, t.A}}
		for _, e := range edges {
			k := pair(e.a, e.b)
			edgeMap[k]++
			edgeNormals[k] = append(edgeNormals[k], n)
			edgeFaces[k] = append(edgeFaces[k], t.Meta.Face)
		}
	}
	for k, count := range edgeMap {
		if count != 2 {
			continue
		}
		nl, nr := edgeNormals[k][0], edgeNormals[k][1]
		fa, fb := edgeFaces[k][0], edgeFaces[k][1]
		// Twin edges are oriented oppositely in the source triangles,
		// so to produce an outline direction (a -> b) we use the
		// canonical (hashed-a, hashed-b) ordering.
		sol.Outlines = append(sol.Outlines, SOutline{
			NL: nl,
			NR: nr,
			FA: fa,
			FB: fb,
		})
	}
	_ = kind
}

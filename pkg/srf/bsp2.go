package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// BspClass is the BSP partition classification. Positive (POS)
// means the point/triangle is on the same side as the partitioning
// plane's normal; NEG is the opposite side; COPLANAR lies on the
// plane itself.
type BspClass uint32

const (
	BspPos      BspClass = 100
	BspNeg      BspClass = 101
	BspCoplanar BspClass = 200
)

// ClassifyPointAgainstPlane returns BspPos, BspNeg, or BspCoplanar
// for point p relative to plane (n, d). Coplanar threshold is
// core.LengthEps * max(|n|, 1).
func ClassifyPointAgainstPlane(p, n geom.Vector, d float64) BspClass {
	v := p.Dot(n) - d
	if v > 1e-9 {
		return BspPos
	}
	if v < -1e-9 {
		return BspNeg
	}
	return BspCoplanar
}

// SBsp2 is a 2-D BSP tree over edges, used for point-in-polygon
// tests on arbitrary planes (e.g. trim-region classification).
type SBsp2 struct {
	NP   geom.Vector // normal to the BSP plane
	NO   geom.Vector // outer normal of the edge (for coplanar edges)
	D    float64
	Edge SEdge

	Pos  *SBsp2
	Neg  *SBsp2
	More *SBsp2
}

// InsertEdge adds an edge (nedge, nnp, out) into the BSP starting
// at `where`. Returns the new root.
func (b *SBsp2) InsertEdge(nedge *SEdge, nnp, out geom.Vector) *SBsp2 {
	return SBsp2InsertOrCreateEdge(b, nedge, nnp, out)
}

// IntersectionWith returns the intersection of the segment (a, b)
// with the plane stored in b. Returns a zero vector and false if no
// valid intersection exists.
func (b *SBsp2) IntersectionWith(a, dir geom.Vector) (geom.Vector, bool) {
	if b == nil {
		return geom.Vector{}, false
	}
	den := dir.Dot(b.NP)
	if mathAbs(den) < 1e-20 {
		return geom.Vector{}, false
	}
	t := (b.D - a.Dot(b.NP)) / den
	return a.Plus(dir.ScaledBy(t)), true
}

// SBsp2InsertOrCreateEdge is the C++ static helper that recursively
// inserts an edge into the BSP.
func SBsp2InsertOrCreateEdge(where *SBsp2, nedge *SEdge, nnp, out geom.Vector) *SBsp2 {
	if where == nil {
		return &SBsp2{
			NP:   nnp,
			NO:   out,
			D:    nedge.A.Dot(nnp),
			Edge: *nedge,
		}
	}
	// Classify both endpoints of the new edge.
	aCls := ClassifyPointAgainstPlane(nedge.A, where.NP, where.D)
	bCls := ClassifyPointAgainstPlane(nedge.B, where.NP, where.D)
	// If both on the same side, recurse on that side.
	if aCls == BspPos && bCls == BspPos {
		where.Pos = SBsp2InsertOrCreateEdge(where.Pos, nedge, nnp, out)
		return where
	}
	if aCls == BspNeg && bCls == BspNeg {
		where.Neg = SBsp2InsertOrCreateEdge(where.Neg, nedge, nnp, out)
		return where
	}
	// Edge straddles the plane (or is coplanar); add to `more`.
	where.More = SBsp2InsertOrCreateEdge(where.More, nedge, nnp, out)
	return where
}

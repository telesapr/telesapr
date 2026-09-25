package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// SBspUv is a 2-D BSP tree over uv-space edges, used to test
// whether a point (u, v) on a surface lies inside or outside the
// trim region. Built by recursive insertion of uv edges with a
// "more" chain for coplanar edges, then queried via ClassifyPoint
// or ClassifyEdge.
type SBspUv struct {
	A, B geom.Point2d
	Pos  *SBspUv
	Neg  *SBspUv
	More *SBspUv
}

// BspClass is the SBspUv-specific classification. Used by SShell
// boolean operations to determine which side of a trim edge a
// surface region lies on.
type BspUvClass int

const (
	BspUvInside      BspUvClass = 100
	BspUvOutside     BspUvClass = 200
	BspUvEdgeParallel BspUvClass = 300
	BspUvEdgeAntipar  BspUvClass = 400
	BspUvEdgeOther   BspUvClass = 500
)

// InsertEdge inserts an edge into the BSP, starting from `where`.
// Returns the new root (which may differ from `where` if the edge
// creates a new partition).
func InsertBspUvEdge(where *SBspUv, a, b geom.Point2d) *SBspUv {
	return sbspUvInsertOrCreateEdge(where, a, b)
}

func sbspUvInsertOrCreateEdge(where *SBspUv, a, b geom.Point2d) *SBspUv {
	if where == nil {
		return &SBspUv{A: a, B: b}
	}
	// Classify endpoints against where's line.
	cA := classifyPointUv(a, where.A, where.B)
	cB := classifyPointUv(b, where.A, where.B)
	if cA == BspUvInside && cB == BspUvInside {
		where.Pos = sbspUvInsertOrCreateEdge(where.Pos, a, b)
		return where
	}
	if cA == BspUvOutside && cB == BspUvOutside {
		where.Neg = sbspUvInsertOrCreateEdge(where.Neg, a, b)
		return where
	}
	where.More = sbspUvInsertOrCreateEdge(where.More, a, b)
	return where
}

// ClassifyPoint returns the classification of point pt with respect
// to the trim region stored in b.
func (b *SBspUv) ClassifyPoint(pt geom.Point2d) BspUvClass {
	if b == nil {
		return BspUvOutside
	}
	c := classifyPointUv(pt, b.A, b.B)
	switch c {
	case BspUvInside:
		if b.Pos != nil {
			return b.Pos.ClassifyPoint(pt)
		}
		return BspUvInside
	case BspUvOutside:
		if b.Neg != nil {
			return b.Neg.ClassifyPoint(pt)
		}
		return BspUvOutside
	default:
		// Straddles the edge; try one side, then the other.
		r := BspUvInside
		if b.Pos != nil {
			r = b.Pos.ClassifyPoint(pt)
		}
		if r != BspUvInside {
			return r
		}
		if b.Neg != nil {
			return b.Neg.ClassifyPoint(pt)
		}
		return BspUvInside
	}
}

// ClassifyEdge returns whether the segment from a to b lies inside,
// outside, or straddles the trim region. Returns BspUvInside if
// every point of the segment is inside; BspUvOutside if every
// point is outside; otherwise BspUvEdgeParallel / BspUvEdgeAntipar /
// BspUvEdgeOther (which describe the type of endpoint
// classification).
//
// Goal Item 8: used during curve-surface intersection to split
// trim curves at surface crossings. Implementation: classify the
// endpoints (a and b), then check the midpoint. If all three are
// on the same side the edge is inside or outside; otherwise the
// edge straddles the trim.
func (b *SBspUv) ClassifyEdge(a, p geom.Point2d) BspUvClass {
	if b == nil {
		return BspUvOutside
	}
	ca := b.ClassifyPoint(a)
	cb := b.ClassifyPoint(p)
	if ca == cb && ca != BspUvEdgeOther && ca != BspUvEdgeParallel && ca != BspUvEdgeAntipar {
		return ca
	}
	// Sample the midpoint for a robust classification.
	mid := geom.Point2d{U: (a.U + p.U) / 2, V: (a.V + p.V) / 2}
	cm := b.ClassifyPoint(mid)
	if cm == ca && cm == cb {
		return cm
	}
	return BspUvEdgeOther
}

func classifyPointUv(pt, a, b geom.Point2d) BspUvClass {
	v := (b.U-a.U)*(pt.V-a.V) - (b.V-a.V)*(pt.U-a.U)
	if v > 1e-12 {
		return BspUvInside
	}
	if v < -1e-12 {
		return BspUvOutside
	}
	// Collinear; check orientation along the line.
	if (pt.U-a.U)*(pt.U-b.U) > 1e-12 || (pt.V-a.V)*(pt.V-b.V) > 1e-12 {
		return BspUvOutside
	}
	return BspUvEdgeOther
}

// BuildBspUvFromEdges constructs an SBspUv from an edge list (in
// uv-space). The edge list is read from el; srf is the surface
// hosting the trim and is currently unused.
func BuildBspUvFromEdges(el *SEdgeList, srf interface{}) *SBspUv {
	var root *SBspUv
	for i := range el.Edges {
		a := geom.Point2d{U: el.Edges[i].A.X, V: el.Edges[i].A.Y}
		b := geom.Point2d{U: el.Edges[i].B.X, V: el.Edges[i].B.Y}
		root = InsertBspUvEdge(root, a, b)
	}
	return root
}

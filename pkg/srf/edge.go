package srf

import "github.com/spacesolve/spacesolve/pkg/geom"

// EdgeKind classifies edges emitted by SMesh::MakeOutlinesInto. The
// classification drives which edges become visible outlines (drawn
// silhouettes, error edges, etc.).
type EdgeKind uint32

const (
	EdgeNakedOrSelfInter EdgeKind = 100
	EdgeSelfInter        EdgeKind = 200
	EdgeTurning          EdgeKind = 300
	EdgeEmphasized       EdgeKind = 400
	EdgeSharp            EdgeKind = 500
)

// SEdge is a 3-D line segment used by SEdgeList for polygon assembly
// and kd-tree queries. AuxA/AuxB are user-defined slots used by
// callers to carry mesh/curve handles alongside the geometric data.
type SEdge struct {
	Tag  int
	AuxA int
	AuxB int
	A, B geom.Vector
}

func SEdgeFrom(a, b geom.Vector) SEdge {
	return SEdge{A: a, B: b}
}

// SEdgeList is a slice of SEdge. The kd-tree fields are populated by
// BuildKdEdges (Phase 2) when an accelerated intersection query is
// needed.
type SEdgeList struct {
	Edges   []SEdge
	KdEdges interface{}
}

func (el *SEdgeList) Clear() {
	el.Edges = el.Edges[:0]
	el.KdEdges = nil
}

func (el *SEdgeList) AddEdge(a, b geom.Vector, auxA, auxB, tag int) {
	el.Edges = append(el.Edges, SEdge{
		Tag:  tag,
		AuxA: auxA,
		AuxB: auxB,
		A:    a,
		B:    b,
	})
}

func (el *SEdgeList) Len() int { return len(el.Edges) }

// Crossing reports the parametric t along edge e at which line
// (p0, dp) crosses e. If the line is parallel to e, ok=false.
// 2-D variant used for planar polygon-edge intersection.
func (e *SEdge) Crossing(p0, dp geom.Vector) (t float64, ok bool) {
	v1 := p0.Minus(e.A)
	v2 := e.B.Minus(e.A)
	v3 := geom.Vector{X: -dp.Y, Y: dp.X, Z: 0}
	if dp.Magnitude() < 1e-20 {
		return 0, false
	}
	den := v2.Dot(v3)
	if mathAbs(den) < 1e-20 {
		return 0, false
	}
	t = v2.Cross(v1).Z / den
	u := dp.Cross(v1).Z / den
	return t, u >= 0 && u <= 1
}

func mathAbs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

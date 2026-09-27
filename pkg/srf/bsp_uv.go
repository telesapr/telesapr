package srf

import (
	"math"
	"sort"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

// SBspUv is a 2-D BSP tree over uv-space edges, used to test
// whether a point (u, v) on a surface lies inside or outside the
// trim region. Built by recursive insertion of uv edges with a
// "more" chain for coincident edges, then queried via ClassifyPoint
// or ClassifyEdge. Mirrors C++ SBspUv (solvespace/src/srf/boolean.cpp).
type SBspUv struct {
	A, B geom.Point2d
	Pos  *SBspUv
	Neg  *SBspUv
	More *SBspUv
}

// BspUvClass is the SBspUv-specific classification. Used by SShell
// boolean operations to determine which side of a trim edge a
// surface region lies on.
type BspUvClass int

const (
	BspUvInside       BspUvClass = 100
	BspUvOutside      BspUvClass = 200
	BspUvEdgeParallel BspUvClass = 300
	BspUvEdgeAntipar  BspUvClass = 400
	BspUvEdgeOther    BspUvClass = 500
)

// SBspUvFrom builds a BSP from the uv-space edge list el (points
// stored in the X/Y components). Edges are inserted longest-first for
// numerical stability. Mirrors C++ SBspUv::From.
func SBspUvFrom(el *SEdgeList, srf *SSurface) *SBspUv {
	work := make([]SEdge, len(el.Edges))
	copy(work, el.Edges)
	sort.SliceStable(work, func(i, j int) bool {
		li := work[i].A.Minus(work[i].B).Magnitude()
		lj := work[j].A.Minus(work[j].B).Magnitude()
		return li > lj
	})
	var bsp *SBspUv
	for i := range work {
		bsp = bspUvInsertOrCreateEdge(bsp, work[i].A.ProjectXy(), work[i].B.ProjectXy(), srf)
	}
	return bsp
}

// scalePoints linearizes srf about pt so that uv distances become xyz
// distances, letting tolerances apply consistently in xyz. A nil srf
// leaves the points unscaled. Mirrors C++ SBspUv::ScalePoints.
func scalePoints(pt, a, b *geom.Point2d, srf *SSurface) {
	if srf == nil {
		return
	}
	var tu, tv geom.Vector
	srf.TangentsAt(pt.U, pt.V, &tu, &tv, true)
	mu, mv := tu.Magnitude(), tv.Magnitude()
	pt.U *= mu
	pt.V *= mv
	a.U *= mu
	a.V *= mv
	b.U *= mu
	b.V *= mv
}

// scaledSignedDistanceToLine is positive on the right-hand side of
// a->b (the "inside" of a trim edge). Mirrors C++
// SBspUv::ScaledSignedDistanceToLine.
func scaledSignedDistanceToLine(pt, a, b geom.Point2d, srf *SSurface) float64 {
	scalePoints(&pt, &a, &b, srf)
	n := b.Minus(a).Normal().WithMagnitude(1)
	d := a.Dot(n)
	return pt.Dot(n) - d
}

// scaledDistanceToLine mirrors C++ SBspUv::ScaledDistanceToLine; b is
// the line direction (not its end point).
func scaledDistanceToLine(pt, a, b geom.Point2d, asSegment bool, srf *SSurface) float64 {
	scalePoints(&pt, &a, &b, srf)
	return pt.DistanceToLine(a, b, asSegment)
}

func bspUvInsertOrCreateEdge(where *SBspUv, ea, eb geom.Point2d, srf *SSurface) *SBspUv {
	if where == nil {
		return &SBspUv{A: ea, B: eb}
	}
	where.insertEdge(ea, eb, srf)
	return where
}

// insertEdge mirrors C++ SBspUv::InsertEdge: coincident edges go on
// the More chain, edges straddling this node's line are split.
func (b *SBspUv) insertEdge(ea, eb geom.Point2d, srf *SSurface) {
	dea := scaledSignedDistanceToLine(ea, b.A, b.B, srf)
	deb := scaledSignedDistanceToLine(eb, b.A, b.B, srf)

	switch {
	case math.Abs(dea) < geom.LengthEps && math.Abs(deb) < geom.LengthEps:
		// Line segment is coincident with this one, store in same node
		b.More = &SBspUv{A: ea, B: eb, More: b.More}
	case math.Abs(dea) < geom.LengthEps:
		// Point A lies on this line, but point B does not
		if deb > 0 {
			b.Pos = bspUvInsertOrCreateEdge(b.Pos, ea, eb, srf)
		} else {
			b.Neg = bspUvInsertOrCreateEdge(b.Neg, ea, eb, srf)
		}
	case math.Abs(deb) < geom.LengthEps:
		// Point B lies on this line, but point A does not
		if dea > 0 {
			b.Pos = bspUvInsertOrCreateEdge(b.Pos, ea, eb, srf)
		} else {
			b.Neg = bspUvInsertOrCreateEdge(b.Neg, ea, eb, srf)
		}
	case dea > 0 && deb > 0:
		b.Pos = bspUvInsertOrCreateEdge(b.Pos, ea, eb, srf)
	case dea < 0 && deb < 0:
		b.Neg = bspUvInsertOrCreateEdge(b.Neg, ea, eb, srf)
	default:
		// New edge crosses this one; we need to split.
		n := b.B.Minus(b.A).Normal().WithMagnitude(1)
		d := b.A.Dot(n)
		t := (d - n.Dot(ea)) / n.Dot(eb.Minus(ea))
		pi := ea.Plus(eb.Minus(ea).ScaledBy(t))
		if dea > 0 {
			b.Pos = bspUvInsertOrCreateEdge(b.Pos, ea, pi, srf)
			b.Neg = bspUvInsertOrCreateEdge(b.Neg, pi, eb, srf)
		} else {
			b.Neg = bspUvInsertOrCreateEdge(b.Neg, ea, pi, srf)
			b.Pos = bspUvInsertOrCreateEdge(b.Pos, pi, eb, srf)
		}
	}
}

// ClassifyPoint classifies p against the trim region. When p lies on
// a trim edge, eb (a second point along the direction being tested)
// decides between EDGE_PARALLEL, EDGE_ANTIPARALLEL and EDGE_OTHER.
// Mirrors C++ SBspUv::ClassifyPoint.
func (b *SBspUv) ClassifyPoint(p, eb geom.Point2d, srf *SSurface) BspUvClass {
	if b == nil {
		return BspUvOutside
	}
	dp := scaledSignedDistanceToLine(p, b.A, b.B, srf)

	if math.Abs(dp) < geom.LengthEps {
		for f := b; f != nil; f = f.More {
			ba := f.B.Minus(f.A)
			if scaledDistanceToLine(p, f.A, ba, true, srf) < geom.LengthEps {
				if scaledDistanceToLine(eb, f.A, ba, false, srf) < geom.LengthEps {
					if ba.Dot(eb.Minus(p)) > 0 {
						return BspUvEdgeParallel
					}
					return BspUvEdgeAntipar
				}
				return BspUvEdgeOther
			}
		}
		// Pick arbitrarily which side to send it down, doesn't matter
		if b.Neg != nil {
			return b.Neg.ClassifyPoint(p, eb, srf)
		}
		return BspUvOutside
	} else if dp > 0 {
		if b.Pos != nil {
			return b.Pos.ClassifyPoint(p, eb, srf)
		}
		return BspUvInside
	}
	if b.Neg != nil {
		return b.Neg.ClassifyPoint(p, eb, srf)
	}
	return BspUvOutside
}

// ClassifyEdge classifies the edge ea->eb by its midpoint, retrying at
// another point if the midpoint lands on a non-parallel trim edge.
// Mirrors C++ SBspUv::ClassifyEdge.
func (b *SBspUv) ClassifyEdge(ea, eb geom.Point2d, srf *SSurface) BspUvClass {
	ret := b.ClassifyPoint(ea.Plus(eb).ScaledBy(0.5), eb, srf)
	if ret == BspUvEdgeOther {
		// Perhaps the edge is tangent at its midpoint (and we screwed up
		// somewhere earlier and failed to split it); try a different
		// point on the edge.
		ret = b.ClassifyPoint(ea.Plus(eb.Minus(ea).ScaledBy(0.294)), eb, srf)
	}
	return ret
}

// MinimumDistanceToEdge returns the xyz-scaled distance from p to the
// nearest trim edge. Mirrors C++ SBspUv::MinimumDistanceToEdge.
func (b *SBspUv) MinimumDistanceToEdge(p geom.Point2d, srf *SSurface) float64 {
	if b == nil {
		return geom.VeryPositive
	}
	dn := b.Neg.MinimumDistanceToEdge(p, srf)
	dpos := b.Pos.MinimumDistanceToEdge(p, srf)

	as, bs := b.A, b.B
	scalePoints(&p, &as, &bs, srf)
	d := p.DistanceToLine(as, bs.Minus(as), true)

	return math.Min(d, math.Min(dn, dpos))
}

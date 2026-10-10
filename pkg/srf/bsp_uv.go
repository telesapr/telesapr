package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// SBspUv is a 2-D BSP tree over uv-space edges, used to test whether a
// point (u, v) on a surface lies inside or outside the trim region.
// Built by recursive insertion of uv edges with a "more" chain for
// coincident edges, then queried via ClassifyPoint or ClassifyEdge.
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

// SBspUvFrom builds a BSP from the uv-space edge list el (points stored
// in the X/Y components). Edges are inserted longest-first for
// numerical stability.
func SBspUvFrom(el *SEdgeList, srf *SSurface) *SBspUv {
	work := make([]SEdge, len(el.Edges))
	copy(work, el.Edges)
	CxxSort(work, func(a, b *SEdge) bool {
		la := a.A.Minus(a.B).Magnitude()
		lb := b.A.Minus(b.B).Magnitude()
		return la > lb
	})
	var bsp *SBspUv
	for i := range work {
		bsp = bspUvInsertOrCreateEdge(bsp, work[i].A.ProjectXy(), work[i].B.ProjectXy(), srf)
	}
	return bsp
}

// scalePoints linearizes srf about pt so that uv distances become xyz
// distances, letting tolerances apply consistently in xyz. A nil srf
// leaves the points unscaled.
func scalePoints(pt, a, b *geom.Point2d, srf *SSurface) {
	if srf == nil {
		return
	}
	scalePointsBy(pt, a, b, uvScaleAt(*pt, srf))
}

// uvScale is how far a step in u and in v goes on a surface at a point:
// the lengths of its tangents there.
type uvScale struct{ mu, mv float64 }

// uvScaleAt is the scale scalePoints applies about pt.
func uvScaleAt(pt geom.Point2d, srf *SSurface) uvScale {
	var tu, tv geom.Vector
	srf.TangentsAt(pt.U, pt.V, &tu, &tv, true)
	return uvScale{tu.Magnitude(), tv.Magnitude()}
}

// scalePointsBy is scalePoints with the scale about pt given.
func scalePointsBy(pt, a, b *geom.Point2d, sc uvScale) {
	mu, mv := sc.mu, sc.mv
	pt.U *= mu
	pt.V *= mv
	a.U *= mu
	a.V *= mv
	b.U *= mu
	b.V *= mv
}

// scaledSignedDistanceToLine is positive on the right-hand side of a->b
// (the "inside" of a trim edge).
func scaledSignedDistanceToLine(pt, a, b geom.Point2d, srf *SSurface) float64 {
	scalePoints(&pt, &a, &b, srf)
	return signedDistanceToLine(pt, a, b)
}

// signedDistanceToLine is positive on the right-hand side of a->b.
func signedDistanceToLine(pt, a, b geom.Point2d) float64 {
	n := b.Minus(a).Normal().WithMagnitude(1)
	d := a.Dot(n)
	return pt.Dot(n) - d
}

// scaledDistanceToLine is the distance of p from the line through a
// along b, scaled by b's length; b is the line direction, not its end
// point.
func scaledDistanceToLine(pt, a, b geom.Point2d, asSegment bool, srf *SSurface) float64 {
	scalePoints(&pt, &a, &b, srf)
	return pt.DistanceToLine(a, b, asSegment)
}

func bspUvInsertOrCreateEdge(where *SBspUv, ea, eb geom.Point2d, srf *SSurface) *SBspUv {
	if where == nil {
		return &SBspUv{A: ea, B: eb}
	}
	where.insertEdge(uvEnd{p: ea}, uvEnd{p: eb}, srf)
	return where
}

// uvEnd is an end of an edge being inserted, with the scale about it
// once it has been worked out: the edge is measured against every node
// it passes, always about its own ends, and the surface's tangents
// there are the slow part of that.
type uvEnd struct {
	p     geom.Point2d
	sc    uvScale
	known bool
}

// scaledSignedDistance is scaledSignedDistanceToLine from e to the line
// a->b.
func (e *uvEnd) scaledSignedDistance(a, b geom.Point2d, srf *SSurface) float64 {
	if srf == nil {
		return scaledSignedDistanceToLine(e.p, a, b, nil)
	}
	if !e.known {
		e.sc, e.known = uvScaleAt(e.p, srf), true
	}
	pt := e.p
	scalePointsBy(&pt, &a, &b, e.sc)
	return signedDistanceToLine(pt, a, b)
}

func bspUvInsertEnds(where *SBspUv, ea, eb uvEnd, srf *SSurface) *SBspUv {
	if where == nil {
		return &SBspUv{A: ea.p, B: eb.p}
	}
	where.insertEdge(ea, eb, srf)
	return where
}

// insertEdge inserts an edge: coincident edges go on the More chain,
// edges straddling this node's line are split.
func (b *SBspUv) insertEdge(ea, eb uvEnd, srf *SSurface) {
	dea := ea.scaledSignedDistance(b.A, b.B, srf)
	deb := eb.scaledSignedDistance(b.A, b.B, srf)

	switch {
	case math.Abs(dea) < geom.LengthEps && math.Abs(deb) < geom.LengthEps:
		// Line segment is coincident with this one, store in same node
		b.More = &SBspUv{A: ea.p, B: eb.p, More: b.More}
	case math.Abs(dea) < geom.LengthEps:
		// Point A lies on this line, but point B does not
		if deb > 0 {
			b.Pos = bspUvInsertEnds(b.Pos, ea, eb, srf)
		} else {
			b.Neg = bspUvInsertEnds(b.Neg, ea, eb, srf)
		}
	case math.Abs(deb) < geom.LengthEps:
		// Point B lies on this line, but point A does not
		if dea > 0 {
			b.Pos = bspUvInsertEnds(b.Pos, ea, eb, srf)
		} else {
			b.Neg = bspUvInsertEnds(b.Neg, ea, eb, srf)
		}
	case dea > 0 && deb > 0:
		b.Pos = bspUvInsertEnds(b.Pos, ea, eb, srf)
	case dea < 0 && deb < 0:
		b.Neg = bspUvInsertEnds(b.Neg, ea, eb, srf)
	default:
		// New edge crosses this one; we need to split.
		n := b.B.Minus(b.A).Normal().WithMagnitude(1)
		d := b.A.Dot(n)
		t := (d - n.Dot(ea.p)) / n.Dot(eb.p.Minus(ea.p))
		pi := uvEnd{p: ea.p.Plus(eb.p.Minus(ea.p).ScaledBy(t))}
		if dea > 0 {
			b.Pos = bspUvInsertEnds(b.Pos, ea, pi, srf)
			b.Neg = bspUvInsertEnds(b.Neg, pi, eb, srf)
		} else {
			b.Neg = bspUvInsertEnds(b.Neg, ea, pi, srf)
			b.Pos = bspUvInsertEnds(b.Pos, pi, eb, srf)
		}
	}
}

// ClassifyPoint classifies p against the trim region. When p lies on a
// trim edge, eb (a second point along the direction being tested)
// decides between EDGE_PARALLEL, EDGE_ANTIPARALLEL and EDGE_OTHER.
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
// nearest trim edge.
func (b *SBspUv) MinimumDistanceToEdge(p geom.Point2d, srf *SSurface) float64 {
	if b == nil {
		return geom.VeryPositive
	}
	dn := b.Neg.MinimumDistanceToEdge(p, srf)
	dpos := b.Pos.MinimumDistanceToEdge(p, srf)

	as, bs := b.A, b.B
	scalePoints(&p, &as, &bs, srf)
	d := p.DistanceToLine(as, bs.Minus(as), true)

	return geom.Min(d, geom.Min(dn, dpos))
}

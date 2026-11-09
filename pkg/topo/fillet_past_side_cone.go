package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A curved side may be a cone about an axis square to the face: a disc
// extruded with a draft, a bore drafted wider toward its mouth. It is
// taken as a cylinder is (fillet_past_side_curved.go), its radius
// changing along the axis by k: the slab, a prism, clears the cone at
// every height of the side, and the cutter is the cone's ring, sector
// or bore revolved through the side's angles, its seams where
// arcBreaks puts them, so that none lies under the fillet. The cone
// goes on above the face as far as the cutter reaches, so the side
// grows up by the fillet's sections along the cone carried on.

// coneSideOf is curvedSideOf for a cone: the cone face s lies on, as a
// side the foot on a flat face of outward normal n at height top runs
// into, or false where it is none: its axis not square to the face, the
// face's plane not on the side of the apex the cone opens to, or its
// faces not making one side (curvedExtent).
func (t *Topology) coneSideOf(s int, n geom.Vector, top float64) (*curvedSide, bool) {
	sf := t.Surface(s)
	if sf == nil {
		return nil, false
	}
	f, ok := srf.RoundFaceOf(sf)
	if !ok || !f.IsCone() {
		return nil, false
	}
	along := f.Axis.Dot(n)
	if math.Abs(along) < 1-1e-9 {
		return nil, false
	}
	o := f.Point.Plus(n.ScaledBy(top - f.Point.Dot(n)))
	t0 := o.Minus(f.Point).Dot(f.Axis)
	if t0 <= geom.LengthEps {
		return nil, false
	}
	tan := math.Tan(f.HalfAngle)
	k := tan
	if along < 0 {
		k = -tan
	}
	e1 := n.NormalDir(0).Normal()
	sgn := 1.0
	if f.Outward < 0 {
		sgn = -1
	}
	c := &curvedSide{o: o, axis: n, r: t0 * tan, sgn: sgn, k: k, e1: e1, e2: n.Cross(e1).Normal()}
	if !t.curvedExtent(c) {
		return nil, false
	}
	return c, true
}

// onCone reports whether sf lies on c's cone, its outward normal the
// same way.
func onCone(sf *srf.SSurface, c *curvedSide) bool {
	f, ok := srf.RoundFaceOf(sf)
	if !ok || !f.IsCone() || math.Abs(f.Axis.Dot(c.axis)) < 1-1e-9 {
		return false
	}
	if f.Point.DistanceToLine(c.o, c.axis) > geom.LengthEps {
		return false
	}
	// The apex lies where the radius falls to nothing.
	if math.Abs(f.Point.Minus(c.o).Dot(c.axis)+c.r/c.k) > geom.LengthEps {
		return false
	}
	if math.Abs(math.Tan(f.HalfAngle)-math.Abs(c.k)) > 1e-9 {
		return false
	}
	return (f.Outward < 0) == (c.sgn < 0)
}

// coneCutter is curvedCutter's solid on a cone from h0 up to h1: the
// profile between the cone and the layout's far side (or the axis, for a
// bore filled whole) revolved through the side's angles at arcBreaks,
// the profile laid in the half plane of the first break. The cone's
// line is split at the side's foot: there the cone meets the plane of
// the solid's bottom or a ledge's top, which, without an exact circle
// of the cutter's own lying in it, the Boolean marched and left open
// (2026-11-09). It is not split in the face's plane, where the fillet's
// foot crosses the rim: a seam of the cutter there, met by the fillet
// tangent to that plane, left the trims open (2026-11-09).
func (c *curvedSide) coneCutter(l curvedLayout, h0, h1 float64, side *srf.SSurface, tol srf.Tolerance) (srf.SShell, bool) {
	r0, r1 := c.radiusAt(h0), c.radiusAt(h1)
	breaks := c.arcBreaks()
	b0 := breaks[0]
	for i := range breaks {
		breaks[i] -= b0
	}
	p := func(d, h float64) geom.Vector { return c.at(d, b0, h) }
	var pts []geom.Vector
	switch {
	case c.sgn > 0:
		pts = []geom.Vector{p(r0, h0), p(l.far, h0), p(l.far, h1), p(r1, h1), p(c.radiusAt(c.lo), c.lo)}
	case l.far == 0:
		pts = []geom.Vector{p(0, h0), p(r0, h0), p(c.radiusAt(c.lo), c.lo), p(r1, h1), p(0, h1)}
	default:
		pts = []geom.Vector{p(l.far, h0), p(r0, h0), p(c.radiusAt(c.lo), c.lo), p(r1, h1), p(l.far, h1)}
	}
	// A cutter starting at the side's foot (curvedCutter) has its first
	// corner there already.
	pts = slices.CompactFunc(pts, func(a, b geom.Vector) bool { return a.Equals(b) })
	if pts[len(pts)-1].Equals(pts[0]) {
		pts = pts[:len(pts)-1]
	}
	sbl := srf.SBezierList{Curves: lines(pts)}
	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	allClosed, allCoplanar := false, false
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&sbl, &poly, nil, tol, &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	if !allClosed || len(sets.Sets) != 1 {
		return srf.SShell{}, false
	}
	cut := srf.SShell{Tol: tol}
	cut.MakeFromRevolutionBetween(&sets.Sets[0], c.o, c.axis, side.Color, noFaces{}, breaks)
	// The pieces of the caps, one a profile's line swept through each
	// pair of breaks, are merged into one plane each: left in pieces,
	// their seams made the cut back leave faces of the slab standing
	// (2026-11-09).
	cut.MergeCoincidentSurfaces()
	asSide(&cut, side)
	return cut, true
}

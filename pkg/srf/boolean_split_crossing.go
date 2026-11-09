package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// An exact curve split where it meets a surface tangent to it there
// is refined onto that surface poorly: the curve touches the surface
// rather than crossing it, and the root is a double one. Where a
// cylinder cuts a fillet's foot, its arc in the face's plane meets the
// fillet, tangent to that plane along the foot, exactly at the foot:
// refined, the split stood 2.8e-4 off the foot's own split there (more
// than the chord tolerance), and the cut cylinder's trim did not close
// (fillet_past_side_curved.go in pkg/topo). The split lies where the
// curve crosses an exact trim curve of that surface, which two curves
// crossing find well.

// grazeReach is how far, in chord tolerances, the split of a curve
// grazing a surface may stand from where it crosses the surface's trim.
const grazeReach = 10

// minCrossing is the least sine of the angle at which the curve and the
// trim must cross for their crossing to stand for the split. A foot
// that only just reaches a flat side crosses the side's line at a
// hundredth of a radian or less, at two points close together, where
// the split refined as ever is the better (TestFilletRodJustPastSides).
const minCrossing = 0.1

// splitWhereGrazing is p, where MakeCopySplitAgainst refined the split
// of curve c against sf (at uv), or where c crosses an exact trim of sf
// within grazeReach chord tolerances of p, where c is exact and runs
// along sf at p. Elsewhere, and where that crossing lies within
// LengthEps of p, p is as it was.
func splitWhereGrazing(c *SCurve, sf *SSurface, uv geom.Point2d, p geom.Vector, tol Tolerance, shells ...*SShell) geom.Vector {
	if !c.IsExact || !grazes(&c.Exact, sf, uv, p) {
		return p
	}
	sh := shellOf(sf, shells...)
	if sh == nil {
		return p
	}
	if q, ok := trimCrossing(&c.Exact, sf, sh, p, grazeReach*tol.ChordTolMm()); ok && !q.Equals(p) {
		return q
	}
	return p
}

// grazes reports whether the exact curve c runs along the surface sf at
// p, sf's point at uv: tangent to it there, within a hundredth.
func grazes(c *SBezier, sf *SSurface, uv geom.Point2d, p geom.Vector) bool {
	t, _ := c.ClosestPointTo(p, false)
	d := c.TangentAt(t)
	n := sf.NormalAt(uv.U, uv.V)
	if d.Magnitude() == 0 || n.Magnitude() == 0 {
		return false
	}
	return math.Abs(d.WithMagnitude(1).Dot(n.WithMagnitude(1))) < 0.01
}

// trimCrossing is where the exact curve c crosses an exact trim curve
// of sf, sh's surface, nearest p and within reach of it, and whether it
// crosses one there.
func trimCrossing(c *SBezier, sf *SSurface, sh *SShell, p geom.Vector, reach float64) (geom.Vector, bool) {
	best, found := p, false
	for i := range sh.Curve {
		sc := &sh.Curve[i]
		if !sc.IsExact || (sc.SurfA != sf.H && sc.SurfB != sf.H) {
			continue
		}
		q, ok := curvesCross(c, &sc.Exact, p)
		if d := q.Minus(p).Magnitude(); ok && d < reach {
			best, reach, found = q, d, true
		}
	}
	return best, found
}

// curvesCross is where the curves a and b cross, found from near p by
// Newton's method on their tangent lines, and whether they do: not
// where they run along each other or pass apart.
func curvesCross(a, b *SBezier, p geom.Vector) (geom.Vector, bool) {
	ta, _ := a.ClosestPointTo(p, false)
	tb, _ := b.ClosestPointTo(p, false)
	for range 30 {
		pa, pb := a.PointAt(ta), b.PointAt(tb)
		if pa.Minus(pb).Magnitude() < lengthEps/1000 {
			break
		}
		da, db := a.TangentAt(ta), b.TangentAt(tb)
		if da.Cross(db).Magnitude() < minCrossing*da.Magnitude()*db.Magnitude() {
			return p, false
		}
		sa, sb := geom.ClosestPointBetweenLines(pa, da, pb, db)
		ta = math.Min(math.Max(ta+sa, 0), 1)
		tb = math.Min(math.Max(tb+sb, 0), 1)
	}
	pa, pb := a.PointAt(ta), b.PointAt(tb)
	if pa.Minus(pb).Magnitude() > lengthEps/10 {
		return p, false
	}
	return pa, true
}

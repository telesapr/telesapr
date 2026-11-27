package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Beside a point where two curves in one face touch tangent, as a
// conical hole's circle in the plane of a ring's bottom touches the
// ring's inner circle, the one lies between the other's chords and its
// arc for a long way, whatever the chords' length: the chord from the
// point sags below the arc by y(L-y)/2R at y along it, the other circle
// by y²/2R', and over the first part of each chord the chord is the
// deeper. The trim BSP, made of the chords, puts the hole's circle
// there inside the ring's face, and its pieces beside the point were
// kept, dangling from it, though the whole circle lies outside the face
// but at the point (roadmap K3). Within a chord's sag of a trim whose
// curve is exact, the side is taken from the exact curve.

// touchesNear tells whether any of touches lies within reach of the
// surface's bounding box.
func (s *SSurface) touchesNear(touches []geom.Vector, reach float64) bool {
	if len(touches) == 0 {
		return false
	}
	var hi, lo geom.Vector
	s.AxisAlignedBounding(&hi, &lo)
	r := geom.Vector{X: reach, Y: reach, Z: reach}
	hi, lo = hi.Plus(r), lo.Minus(r)
	for _, p := range touches {
		if p.X >= lo.X && p.X <= hi.X && p.Y >= lo.Y && p.Y <= hi.Y && p.Z >= lo.Z && p.Z <= hi.Z {
			return true
		}
	}
	return false
}

// outsideExactTrim tells whether the segment a-b of a curve where own
// meets s, which s's trim BSP puts inside, lies outside the exact curve
// of the trim whose chords its middle lies within the sag of
// (curvedClearance), by more than LengthEps. sh holds the curves the
// trims name (the shell the Boolean is made into).
func (s *SSurface) outsideExactTrim(sh *SShell, own *SSurface, a, b geom.Vector) bool {
	// The surface is the other operand's, which Booleans side by side
	// share: its closest-point cache is left as it was.
	cached := s.Cached
	defer func() {
		if s.Cached != cached {
			s.Cached = cached
		}
	}()
	p := own.ClosestPointOnThisAndSurface(s, a.Plus(b).ScaledBy(0.5))
	el := &s.curvedEdges
	tol := s.Tol.ChordTolMm()
	best, bestD := -1, veryPositive
	for i := range el.Edges {
		e := &el.Edges[i]
		band := math.Max(tol, chordSag(el, i))
		if d := distanceToSegment(p, e.A, e.B); d < band && d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 {
		return false
	}
	e := &el.Edges[best]
	sc := sh.FindCurve(core.HSCurve{V: uint32(e.AuxA)})
	if sc == nil || !sc.IsExact || sc.Exact.Degree < 2 {
		return false
	}
	dir := e.B.Minus(e.A)
	if dir.Magnitude() < geom.LengthEps || s.Bsp == nil {
		return false
	}
	mid := e.A.Plus(e.B).ScaledBy(0.5)
	var muv geom.Point2d
	s.ClosestPointTo(mid, &muv, true)
	n := s.NormalAtUv(muv)
	if n.Magnitude() < geom.LengthEps {
		return false
	}
	// The side of the chord the face lies on, from the BSP a little off
	// the chord's middle, where the chords are what it is made of.
	w := n.Cross(dir).WithMagnitude(1)
	probe := mid.Plus(w.ScaledBy(math.Min(dir.Magnitude()/8, 100*geom.LengthEps)))
	var puv geom.Point2d
	s.ClosestPointTo(probe, &puv, true)
	switch s.Bsp.ClassifyPoint(puv, geom.Point2d{}, s) {
	case BspUvInside:
	case BspUvOutside:
		w = w.ScaledBy(-1)
	default:
		return false
	}
	t, _ := sc.Exact.ClosestPointTo(p, true)
	t = math.Max(0, math.Min(1, t))
	return p.Minus(sc.Exact.PointAt(t)).Dot(w) < -geom.LengthEps
}

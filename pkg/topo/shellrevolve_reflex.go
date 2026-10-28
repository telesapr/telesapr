package topo

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A revolve of more than half a turn has ends that meet at the axis in
// a reflex corner, so its inside isn't what lies past both ends' planes
// moved in: those half-spaces would cut the material across the axis.
// What a kept end walls off is what lies within the thickness of the
// end itself, a half-plane from the axis: a slab along it, rounded
// about the axis. With both ends kept the two slabs and the gap between
// the ends make one region, the gap grown by the thickness. Either is
// cut as one prism along the axis, so its planes and its round meet
// tangentially inside one solid, not in a Boolean.

// endFrame is the radial direction u of end k of rev (0 at Start, 1 at
// End) and the direction square to it into the solid's material.
func endFrame(rev Revolution, k int, radial geom.Vector) (u, into geom.Vector) {
	axis := rev.Axis.WithMagnitude(1)
	a := []float64{rev.Start, rev.End}[k]
	u = radial.RotatedAroundAxis(axis, a)
	// Into the solid from the end: the way the turn goes from Start,
	// back from End.
	step := math.Copysign(1e-3, rev.End-rev.Start)
	if k == 1 {
		step = -step
	}
	into = axis.Cross(u).WithMagnitude(1)
	if rev.Pt.Plus(radial).RotatedAbout(rev.Pt, axis, a+step).Minus(rev.Pt.Plus(u)).Dot(into) < 0 {
		into = into.ScaledBy(-1)
	}
	return u, into
}

// reflexEndsCut is the prism along the axis of rev, a revolve of more
// than half a turn, holding what lies within thick of its kept ends
// (keep[0] the end at Start, keep[1] at End), over the whole of the
// profile's size.
func reflexEndsCut(rev Revolution, keep [2]bool, radial geom.Vector, thick float64, tol srf.Tolerance, color core.RGBA) (*srf.SShell, error) {
	axis := rev.Axis.WithMagnitude(1)
	lo, hi := profileBox(rev.Profile)
	size := hi.Minus(lo).Magnitude() + rev.Pt.Minus(lo).Magnitude() + thick + 1
	far := 2 * size
	uS, nS := endFrame(rev, 0, radial)
	uE, nE := endFrame(rev, 1, radial)
	sigma := math.Copysign(1, rev.End-rev.Start)
	turn := math.Abs(rev.End - rev.Start)

	// The outline about the axis: lead points, the round from arcFrom to
	// arcTo (angles from the start, the way the turn goes, at radius
	// thick), tail points, closed back to the first lead point.
	var lead, tail []geom.Vector
	var arcFrom, arcTo float64
	switch {
	case keep[0] && keep[1]:
		// Out along the start, round to the end, out along it, and back
		// across the gap.
		lead = []geom.Vector{uS.ScaledBy(far).Plus(nS.ScaledBy(thick))}
		arcFrom, arcTo = math.Pi/2, turn-math.Pi/2
		gap := nS.Plus(nE).ScaledBy(-1).WithMagnitude(1)
		tail = []geom.Vector{uE.ScaledBy(far).Plus(nE.ScaledBy(thick)), gap.ScaledBy(2 * far)}
	case keep[0] || keep[1]:
		// Out along the end on its material's side, half a turn round the
		// axis, out along it behind.
		u, n := uS, nS
		arcFrom, arcTo = math.Pi/2, 3*math.Pi/2
		if keep[1] {
			u, n = uE, nE
			arcFrom, arcTo = turn-math.Pi/2, turn-3*math.Pi/2
		}
		lead = []geom.Vector{u.ScaledBy(far).Plus(n.ScaledBy(thick))}
		tail = []geom.Vector{u.ScaledBy(far).Minus(n.ScaledBy(thick))}
	default:
		return nil, errors.New("no end of the revolve is kept")
	}

	base := rev.Pt.Minus(axis.ScaledBy(size))
	at := func(p geom.Vector) geom.Vector { return base.Plus(p) }
	dir := func(phi float64) geom.Vector { return uS.RotatedAroundAxis(axis, sigma*phi).ScaledBy(thick) }
	var sbl srf.SBezierList
	line := func(a, b geom.Vector) { sbl.Curves = append(sbl.Curves, srf.SBezierFrom2(at(a), at(b))) }
	line(lead[0], dir(arcFrom))
	n := max(1, int(math.Ceil(math.Abs(arcTo-arcFrom)/(math.Pi/2)-1e-9)))
	step := (arcTo - arcFrom) / float64(n)
	w := math.Cos(step / 2)
	for i := range n {
		a0 := arcFrom + float64(i)*step
		sb := srf.SBezierFrom3(at(dir(a0)), at(dir(a0+step/2).ScaledBy(1/w)), at(dir(a0+step)))
		sb.Weight[1] = w
		sbl.Curves = append(sbl.Curves, sb)
	}
	line(dir(arcTo), tail[0])
	for i := 1; i < len(tail); i++ {
		line(tail[i-1], tail[i])
	}
	line(tail[len(tail)-1], lead[0])

	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	allClosed, allCoplanar := false, false
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&sbl, &poly, nil, tol, &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	if !allClosed || len(sets.Sets) != 1 {
		return nil, errors.New("the prism that cuts the inside back from the revolve's ends does not close")
	}
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromExtrusionOf(&sets.Sets[0], geom.Vector{}, axis.ScaledBy(2*size), color)
	return sh, nil
}

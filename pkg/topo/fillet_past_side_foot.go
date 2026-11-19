package topo

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A fillet that takes material away (a hole's rim) beside a curved side
// may cut down past the side's foot, where the solid goes on below it (a
// ledge round a disc) and the cutter starts (curvedCutter). Where the
// cut's outline in the plane of the foot touches the foot's circle, or
// comes just inside it, the cut back does not close: a chamfer of 3.5 on
// the rim of a hole of 4 near an upright disc's rim, its cone's circle on
// the ledge's top of radius 4.5 against the foot's of 8, failed from 0.500
// to 0.504 from the rim at a chord tolerance of 0.0005 (the circles 0 to
// 0.004 apart), and at 0.504 at a tolerance of 0.002; it was made at
// 0.499 and 0.4995, the circles crossing, and at 0.505 (2026-11-19). The
// band does not follow the tolerance. Where the circles cross by a
// little, the cut back finds where they cross, and the cut past the side
// below the foot is a sliver within keepBeyond's allowance: such a
// fillet is made at the nearest size at which they cross, where that
// moves its surface by no more than the chord tolerance
// (footClearSize); others are refused, saying so.

// footInside is how far inside the foot's circle, as a multiple of the
// chord tolerance, the cut's outline in the foot's plane is taken to
// touch it; footPast how far past it the outline must reach for the cut
// back to find the crossing, and footPastMost the most it may.
const (
	footInside   = 10
	footPast     = 0.25
	footPastMost = 2
)

// footTouches reports whether reach, how far past the side at its foot
// the cut reaches, touches the foot's circle at tolerance tol.
func footTouches(reach, tol float64) bool {
	return reach > -footInside*tol && reach < footPast*tol
}

// footReach is how far past c's side, at its foot, the cut of the
// rounded edge fe at size r reaches at most (negative where it stays
// inside), or -Inf where it reaches no lower than the foot: its cross
// sections at points along the edge, where they cross the plane of the
// foot, fe's distances on its faces taken from its size fe.r to r.
func (c *curvedSide) footReach(fe *filletEdge, r float64, flat bool) float64 {
	g := *fe
	for k := range 2 {
		g.ss[k] *= r / fe.r
	}
	h := func(p geom.Vector) float64 { return p.Minus(c.o).Dot(c.axis) - c.lo }
	const along, across = 4 * contactSamples, 64
	best := math.Inf(-1)
	for i := range along {
		sec := g.section(g.curve.PointAt((float64(i)+0.5)/along), flat)
		prev := sec.PointAt(0)
		for j := 1; j <= across; j++ {
			p := sec.PointAt(float64(j) / across)
			if ha, hb := h(prev), h(p); (ha >= 0) != (hb >= 0) {
				q := prev.Plus(p.Minus(prev).ScaledBy(ha / (ha - hb)))
				best = math.Max(best, c.past(q))
			}
			prev = p
		}
	}
	return best
}

// footClearSize makes the picks' fillet past the sides of off, which at
// r came out unsound, at the size nearest r at which its cut crosses the
// foot's circle of each curved side it touches there at r (footTouches)
// by footPast to footPastMost chord tolerances, the size moved by no
// more than moves the fillet's surface by the chord tolerance:
// (sqrt 2 - 1)|r - c| for a fillet, |r - c|/sqrt 2 for a chamfer, both
// between faces square to each other. touches is set where the cut
// touches a side's foot at r, whether or not a size was found.
func footClearSize(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool,
	off *runOff) (out srf.SShell, made float64, touches, ok bool) {
	if off.edge == nil || off.varies {
		return srf.SShell{}, r, false, false
	}
	for _, p := range picks {
		if p.R != 0 || p.D2 != 0 {
			return srf.SShell{}, r, false, false
		}
	}
	tol := sh.Tol.ChordTolMm()
	var sides []*curvedSide
	for _, s := range off.distinct() {
		if c := s.curved; c != nil && c.below && footTouches(c.footReach(off.edge, r, flat), tol) {
			sides = append(sides, c)
		}
	}
	if len(sides) == 0 {
		return srf.SShell{}, r, false, false
	}
	move := math.Sqrt2 - 1
	if flat {
		move = 1 / math.Sqrt2
	}
	most := tol / move
	var sizes []float64
	for _, k := range []float64{0.25, 0.5, 0.75, 1} {
		sizes = append(sizes, r-k*most, r+k*most)
	}
	slices.SortStableFunc(sizes, func(a, b float64) int { return cmp.Compare(math.Abs(a-r), math.Abs(b-r)) })
	for _, size := range sizes {
		crosses := true
		for _, c := range sides {
			if d := c.footReach(off.edge, size, flat); d < footPast*tol || d > footPastMost*tol {
				crosses = false
			}
		}
		if !crosses {
			continue
		}
		if out, ok := widenedFillet(sh, picks, size, origin, flat, off); ok {
			return out, size, true, true
		}
	}
	return srf.SShell{}, r, true, false
}

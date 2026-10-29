package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A twist turns the frames along a straight piece too, so beside a sharp
// corner the piece's sides would be no prism and the mitre no plane both
// sides share. The twist is held still there instead: each line beside a
// corner is split where its mitre's reach ends, the part at the corner
// swept as a prism with the frames as they are where it starts, and the
// twist is spread over the rest of the path. Its rate climbs from nothing
// at a held stretch's end over the profile's reach, so the twisted
// surfaces leave the prisms tangent.

// holdMargin is how much further than its mitre reaches a stretch is
// held, as a share of the profile's reach.
const holdMargin = 0.1

// holdReach is how far along each line beside corner k its mitre can
// reach: a profile point within reach of the path, carried along the
// line onto the plane halving the corner, travels at most reach over the
// cosine of half the turn.
func (sp *SweepPath) holdReach(k int) float64 {
	c := sp.tangentInto(k - 1).Dot(sp.tangent(float64(k)))
	half := math.Cos(math.Acos(math.Max(-1, math.Min(1, c))) / 2)
	return sp.opts.Reach * (1/math.Max(half, 1e-6) + holdMargin)
}

// heldPieces are the path's pieces with each line beside a corner split
// where the stretch held for the corner ends, where enough of the line is
// left past it to twist on; the pieces as they are when none is split.
func (sp *SweepPath) heldPieces() []srf.SBezier {
	if sp.opts.Reach <= 0 {
		return sp.pieces
	}
	var out []srf.SBezier
	for i, pc := range sp.pieces {
		if !sp.prism[i] {
			out = append(out, pc)
			continue
		}
		a, b := pc.Start(), pc.Finish()
		l := b.Minus(a).Magnitude()
		lo, hi := 0.0, l
		if sp.corner[i] {
			lo = sp.holdReach(i)
		}
		if sp.corner[i+1] {
			hi = l - sp.holdReach(i+1)
		}
		// Too little left to twist on: the whole line is held.
		if hi-lo < 0.1*l {
			out = append(out, pc)
			continue
		}
		at := func(d float64) geom.Vector { return a.Plus(b.Minus(a).ScaledBy(d / l)) }
		cuts := []geom.Vector{a}
		if lo > 0 {
			cuts = append(cuts, at(lo))
		}
		if hi < l {
			cuts = append(cuts, at(hi))
		}
		cuts = append(cuts, b)
		for j := 0; j+1 < len(cuts); j++ {
			out = append(out, srf.SBezierFrom2(cuts[j], cuts[j+1]))
		}
	}
	return out
}

// holdTwist spreads the twist over the path with it held still along the
// prisms beside corners, refusing a twisted path whose corners it can't
// be held still beside.
func (sp *SweepPath) holdTwist() error {
	first := slices.Index(sp.corner, true)
	refuse := &srf.CornerError{At: sp.pieces[first].Start(), Err: ErrPathCornerTwisted}
	if sp.opts.Reach <= 0 {
		return refuse
	}
	var holds [][2]float64
	for i := range sp.pieces {
		if !sp.prism[i] && !sp.rigid[i] {
			continue
		}
		lo, hi := sp.samples[i*framesPerPiece].arc, sp.samples[(i+1)*framesPerPiece].arc
		if n := len(holds); n > 0 && holds[n-1][1] >= lo {
			holds[n-1][1] = hi
			continue
		}
		holds = append(holds, [2]float64{lo, hi})
	}
	law := newTwistLaw(holds, sp.opts.Reach, sp.length)
	if law == nil {
		return refuse
	}
	sp.law = law
	return nil
}

// twistLaw spreads a twist along a path's arc length at a rate that is
// nothing on the held stretches and climbs linearly to its full over ramp
// away from them: the rate is piecewise linear between breaks b, w its
// value at each, cum the integral up to each.
type twistLaw struct {
	b, w, cum []float64
}

// newTwistLaw is the law holding the twist still on holds, intervals of
// arc length in order, along a path of the given length; nil where the
// holds leave nowhere to twist.
func newTwistLaw(holds [][2]float64, ramp, length float64) *twistLaw {
	rate := func(a float64) float64 {
		d := math.Inf(1)
		for _, h := range holds {
			switch {
			case a < h[0]:
				d = math.Min(d, h[0]-a)
			case a > h[1]:
				d = math.Min(d, a-h[1])
			default:
				return 0
			}
		}
		return math.Min(1, d/ramp)
	}
	bs := []float64{0, length}
	for j, h := range holds {
		bs = append(bs, h[0], h[1], h[0]-ramp, h[1]+ramp)
		if j > 0 {
			bs = append(bs, (holds[j-1][1]+h[0])/2)
		}
	}
	bs = slices.DeleteFunc(bs, func(a float64) bool { return a < 0 || a > length })
	slices.Sort(bs)
	bs = slices.Compact(bs)
	law := &twistLaw{b: bs, w: make([]float64, len(bs)), cum: make([]float64, len(bs))}
	for k, a := range bs {
		law.w[k] = rate(a)
		if k > 0 {
			law.cum[k] = law.cum[k-1] + (a-bs[k-1])*(law.w[k]+law.w[k-1])/2
		}
	}
	if law.cum[len(bs)-1] <= 1e-9*length {
		return nil
	}
	return law
}

// share is the share of the twist turned by at arc length a.
func (l *twistLaw) share(a float64) float64 {
	n := len(l.b)
	total := l.cum[n-1]
	if a <= 0 {
		return 0
	}
	if a >= l.b[n-1] {
		return 1
	}
	k, _ := slices.BinarySearch(l.b, a)
	k = max(k, 1)
	// The rate is linear across the break's interval.
	a0, w0 := l.b[k-1], l.w[k-1]
	slope := (l.w[k] - w0) / (l.b[k] - a0)
	x := a - a0
	return (l.cum[k-1] + w0*x + slope*x*x/2) / total
}

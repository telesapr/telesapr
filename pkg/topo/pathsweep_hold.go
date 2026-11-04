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
// surfaces leave the prisms tangent. A corner beside an arc is a rigid
// run's (pathsweep_joint.go), held whole; a line at either end of the run
// is split where its joint's reach ends too, and the rest of it leaves
// the run and twists, so a line and an arc about such a corner have
// somewhere to twist. Where that leaves nowhere, the arcs beside such
// corners are split where their joints' reach ends as well, and the rest
// of each takes the twist along its own circle.

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

// lineHold is how far along the line piece i, beside the corner at
// boundary k, the twist is held still for the corner: its mitre's reach
// where the corner is between two lines, its joint's where an arc is
// beside it (srf.JointReach, for the profile's points in reach of the
// path in the corner's plane), and a share of the profile's reach more.
// Where the joint can't be measured, the whole line.
func (sp *SweepPath) lineHold(k, i int) float64 {
	if !sp.rigid[i] || straightPiece(&sp.pieces[k-1]) && straightPiece(&sp.pieces[k]) {
		return sp.holdReach(k)
	}
	back, on, err := sp.jointReach(k)
	if err != nil {
		return math.Inf(1)
	}
	d := on
	if i == k-1 {
		d = back
	}
	return d + holdMargin*sp.opts.Reach
}

// arcHold is the angle along the arc piece i, beside the corner at
// boundary k, the twist is held still for the corner: its joint's reach
// along the arc, and a share of the profile's reach more, as an angle
// on the arc's radius. Where the joint can't be measured, the whole arc.
func (sp *SweepPath) arcHold(k, i int) float64 {
	center, _, _, ok := arcOf(&sp.pieces[i])
	back, on, err := sp.jointReach(k)
	if !ok || err != nil {
		return math.Inf(1)
	}
	d := on
	if i == k-1 {
		d = back
	}
	r := sp.pieces[i].Start().Minus(center).Magnitude()
	return d + holdMargin*sp.opts.Reach/r
}

// jointReach is how far the joint of the corner at boundary k reaches
// back along the piece before it and on along the piece after, for the
// profile's points in reach of the path in the corner's plane: lengths
// along a line, angles along an arc (srf.JointReach).
func (sp *SweepPath) jointReach(k int) (back, on float64, err error) {
	cs := sp.cornerStep(k)
	at := sp.pieces[k].Start()
	across := cs.Axis.Cross(sp.tangentInto(k - 1)).WithMagnitude(1)
	const n = 32
	pts := make([]geom.Vector, 0, n+1)
	for j := range n + 1 {
		pts = append(pts, at.Plus(across.ScaledBy(sp.opts.Reach*(2*float64(j)/n-1))))
	}
	return srf.JointReach(sp.pieceStep(k-1), cs, sp.pieceStep(k), pts)
}

// subArc is the part of the arc pc from angle a to angle b along it,
// an arc as arcOf reads one, with pc's tags.
func subArc(pc srf.SBezier, a, b float64) srf.SBezier {
	center, axis, _, _ := arcOf(&pc)
	r0 := pc.Start().Minus(center)
	th := b - a
	p0 := center.Plus(r0.RotatedAroundAxis(axis, a))
	p2 := center.Plus(r0.RotatedAroundAxis(axis, b))
	mid := r0.RotatedAroundAxis(axis, (a+b)/2)
	out := pc
	out.Degree = 2
	out.Ctrl = [srf.MaxDegree + 1]geom.Vector{p0, center.Plus(mid.ScaledBy(1 / math.Cos(th/2))), p2}
	out.Weight = [srf.MaxDegree + 1]float64{1, math.Cos(th / 2), 1}
	return out
}

// heldPieces are the path's pieces with each line beside a corner, a
// mitred one's or one in a rigid run's, split where the stretch held for
// the corner ends, where enough of the line is left past it to twist on;
// with arcs, each arc of a rigid run beside a corner is split so too,
// where its joint's reach ends. free marks the parts of arcs split off
// past a joint, which the rigid run is not to grow back along
// (growRigid). The pieces as they are, and free nil, when none is split.
func (sp *SweepPath) heldPieces(arcs bool) (out []srf.SBezier, free []bool) {
	if sp.opts.Reach <= 0 {
		return sp.pieces, nil
	}
	split := false
	for i, pc := range sp.pieces {
		if arcs && sp.rigid[i] {
			if _, _, angle, ok := arcOf(&pc); ok {
				parts, past := sp.splitArc(i, angle)
				out = append(out, parts...)
				free = append(free, past...)
				split = split || len(parts) > 1
				continue
			}
		}
		// A line of a rigid run is held only as far as its joints reach
		// too: the rest leaves the run and twists, so a run of a line and
		// an arc about a corner has somewhere to twist.
		if !sp.prism[i] && (!sp.rigid[i] || !straightPiece(&sp.pieces[i])) {
			out = append(out, pc)
			free = append(free, false)
			continue
		}
		a, b := pc.Start(), pc.Finish()
		l := b.Minus(a).Magnitude()
		lo, hi := 0.0, l
		if sp.corner[i] {
			lo = sp.lineHold(i, i)
		}
		if sp.corner[i+1] {
			hi = l - sp.lineHold(i+1, i)
		}
		// Too little left to twist on: the whole line is held.
		if hi-lo < 0.1*l {
			out = append(out, pc)
			free = append(free, false)
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
			free = append(free, false)
		}
		split = split || len(cuts) > 2
	}
	if !split {
		return sp.pieces, nil
	}
	return out, free
}

// splitArc is the arc piece i, turning by angle, of a rigid run, cut
// where the joints of the corners at its ends reach, with the parts past
// them marked; the arc whole where it has no corner at an end, or too
// little is left past the joints to twist on.
func (sp *SweepPath) splitArc(i int, angle float64) ([]srf.SBezier, []bool) {
	pc := sp.pieces[i]
	lo, hi := 0.0, angle
	if sp.corner[i] {
		lo = sp.arcHold(i, i)
	}
	if sp.corner[i+1] {
		hi = angle - sp.arcHold(i+1, i)
	}
	if lo == 0 && hi == angle || hi-lo < 0.1*angle {
		return []srf.SBezier{pc}, []bool{false}
	}
	var parts []srf.SBezier
	var past []bool
	if lo > 0 {
		parts, past = append(parts, subArc(pc, 0, lo)), append(past, false)
	}
	parts, past = append(parts, subArc(pc, lo, hi)), append(past, true)
	if hi < angle {
		parts, past = append(parts, subArc(pc, hi, angle)), append(past, false)
	}
	// The ends are the piece's own, so the chain stays joined exactly.
	parts[0].Ctrl[0] = pc.Start()
	parts[len(parts)-1].Ctrl[2] = pc.Finish()
	return parts, past
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

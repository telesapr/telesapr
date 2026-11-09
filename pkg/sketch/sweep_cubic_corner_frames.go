package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The runs of a sweep cut at its corners beside a cubic are swept each
// along its own path, the profile taken at each corner from where the
// run before left it, turned about the corner's axis as a mitre turns
// it (sweepRuns.starts). Both runs' sections at the corner are then one
// section turned, which is what the corner's fill and the touching
// points (sweep_cubic_corner_touch.go) are made from. Each run's frames
// move the profile relative to the run's own first frame, so that holds
// for frames that turn otherwise than by rotation-minimising too, as
// long as the two runs agree at the corner:
//
//   - A twist turns the frames about the path. Twisted up to the corner,
//     the two runs' fitted skins overlap inside the turn twisted, and
//     the Booleans fail to close the curves they meet in, beside a
//     straight line's exact prism as beside a fitted one (tried
//     2026-11-09, and with the twist only slowed there by topo's twist
//     law, which leaves the skins in many patches). So the twist is held
//     still beside each corner over as far as a mitre there would reach,
//     as the kernel holds it beside a corner between two lines: the path
//     is cut there too, smoothly (cutSmooth), the stretches at the
//     corner swept untwisted, as with no twist, and the twist spread by
//     length over the rest, each run turning on from where the one
//     before stopped (at full rate from the cut, with no ramp: the runs
//     meet flush on the section between them, an edge where the twist
//     starts). Where the cubic bends beside the corner its tube overlaps
//     the other further than a line's would, and the runs are tried
//     again held twice as far (holdScales).
//   - Frames held to a direction are no relative motion: each run's
//     frame at the corner is the held one, so the corner is made only
//     where the held frame before it, turned by the corner, is the held
//     frame after it, as the direction lying along the corner's axis
//     makes it. Otherwise the sections either side differ and no fill
//     joins them; the kernel refuses such a corner between two lines
//     the same way (topo.ErrPathCornerHeld). Along lines, and arcs about
//     the held direction, held frames move the profile rigidly, and the
//     run is swept exactly.
//   - Round a closed path, the frames carried round come back turned
//     about the tangent where the path closes, by an angle its shape
//     decides; that turn is taken back as a twist, on top of the user's
//     whole turns, so that the last run ends on the section the first
//     starts from. A plane path's frames come back unturned, and its
//     runs stay exact.

// heldAgree reports whether frames held to up, turned by a corner from
// the tangent d1 to d2, are the frames held to up after it; true for no
// up, and where the path runs along up, which the fit refuses.
func heldAgree(up, d1, d2 geom.Vector) bool {
	if up.Magnitude() == 0 {
		return true
	}
	held := func(t geom.Vector) geom.Vector { return up.Minus(t.ScaledBy(up.Dot(t))) }
	r1, r2 := held(d1), held(d2)
	if r1.Magnitude() < 1e-9 || r2.Magnitude() < 1e-9 {
		return true
	}
	axis := d1.Cross(d2).WithMagnitude(1)
	turned := r1.WithMagnitude(1).RotatedAroundAxis(axis, math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2)))))
	return turned.Minus(r2.WithMagnitude(1)).Magnitude() <= 1e-7
}

// heldAlong reports whether frames held to up turn along an arc about
// axis as the arc's rigid turn does: up along the axis, or no up.
func heldAlong(up, axis geom.Vector) bool {
	if up.Magnitude() == 0 {
		return true
	}
	return up.WithMagnitude(1).Cross(axis.WithMagnitude(1)).Magnitude() < 1e-9
}

// heldTurns is a pathError at the first corner where frames held to a
// direction don't turn as the corner turns the path.
func (sr *sweepRuns) heldTurns() error {
	for k, turn := range sr.turns {
		if sr.corner(k) && !heldAgree(sr.up, sr.into[k], sr.out[k]) {
			x := turn.Center
			return pathErr(&x, "the path has a sharp corner at (%g, %g, %g) next to a cubic, where the frames held to "+
				"a direction turn otherwise than the path does; hold them to the corner's axis", x.X, x.Y, x.Z)
		}
	}
	return nil
}

// closingTwist is the twist, in radians, the runs of a closed path are
// twisted by so that their frames meet where it closes: the user's
// twist, which must be whole turns, less the turn the frames, fitted
// untwisted, come back by.
func (sr *sweepRuns) closingTwist(twist float64) (float64, error) {
	x := sr.chains[0][0].start
	if k := twist / (2 * math.Pi); math.Abs(k-math.Round(k)) > 1e-9 {
		return 0, pathErr(&x, "a closed path's twist must be whole turns, or the profile doesn't meet itself")
	}
	t0 := sr.chains[0][0].tangentAt(false)
	n := t0.NormalDir(0).WithMagnitude(1)
	m := sr.end().Q.Rotate(n)
	back := math.Atan2(n.Cross(m).Dot(t0), n.Dot(m))
	if math.Abs(back) < 1e-9 {
		back = 0
	}
	return twist - back, nil
}

// closes is a pathError unless the runs of a closed path take the
// profile back onto itself where the path closes, within sweepTol a
// profile's reach out.
func (sr *sweepRuns) closes() error {
	x := sr.chains[0][0].start
	end := sr.end()
	t0 := sr.chains[0][0].tangentAt(false)
	n := t0.NormalDir(0).WithMagnitude(sr.reach)
	for _, p := range []geom.Vector{x, x.Plus(n), x.Plus(t0.Cross(n))} {
		if end.Apply(p).Minus(p).Magnitude() > sweepTol {
			return pathErr(&x, "the path closes, but the profile swept round its sharp corners next to a cubic "+
				"doesn't meet itself at (%g, %g, %g)", x.X, x.Y, x.Z)
		}
	}
	return nil
}

// holdMargin is how much further than a corner's mitre would reach a
// twist is held still beside it, as a share of the profile's reach.
const holdMargin = 0.1

// holdLength is how far along the path either side of a corner turning
// by angle the twist is held still: as far as a mitre of the corner
// would reach along a line, a profile point within reach carried onto
// the plane halving the corner travelling at most reach over the cosine
// of half the turn, and a share of the reach more.
func (sr *sweepRuns) holdLength(angle float64) float64 {
	return sr.hold * sr.reach * (1/math.Max(math.Cos(angle/2), 1e-6) + holdMargin)
}

// holdScales are the multiples of holdLength a twist is held still
// over beside a corner, tried in order: as far as a mitre would reach
// leaves the most of the path to twist on, but where the cubic bends
// beside the corner the runs' tubes overlap further than a line's would,
// and their twisted skins fail to join; held twice as far, more of them
// join (TestSweepSharpCornerBesideCubicFramesSweep).
var holdScales = []float64{1, 2}

// heldBy is the runs with the twist held still beside each corner over
// hold times holdLength and spread over the rest, fitted; a pathError
// where that leaves nowhere to twist.
func (sr *sweepRuns) heldBy(hold float64) (*sweepRuns, error) {
	c := &sweepRuns{up: sr.up, reach: sr.reach, closed: sr.closed, whole: sr.whole, kinds: sr.kinds,
		twist: sr.twist, hold: hold}
	c.cut(sr.whole, sr.kinds)
	c.cut(c.heldCuts(sr.whole, sr.kinds))
	if err := c.spreadTwist(sr.twist); err != nil {
		return nil, err
	}
	if err := c.fit(); err != nil {
		return nil, err
	}
	return c, nil
}

// holdsToTry are the runs to join: sr, and, twisted, sr with the twist
// held still further beside the corners, as far as holdScales go.
func (sr *sweepRuns) holdsToTry() []*sweepRuns {
	out := []*sweepRuns{sr}
	if sr.twist == 0 {
		return out
	}
	for _, hold := range holdScales {
		if hold <= sr.hold {
			continue
		}
		if c, err := sr.heldBy(hold); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// heldCuts are the chain, cut at the corners kinds marks (kinds[i] the
// cut before piece i), with each piece beside a corner cut again where
// the twist's hold ends (holdLength), and the kinds of its cuts. A
// piece too short for its holds, with a tenth of it left to twist on, is
// held whole.
func (sr *sweepRuns) heldCuts(chain []pathPiece, kinds []int) ([]pathPiece, []int) {
	var out []pathPiece
	outKinds := []int{cutNone}
	k := 0
	for i, p := range chain {
		outKinds[len(outKinds)-1] = kinds[i]
		lo, hi := 0.0, 0.0
		if kinds[i] == cutCorner {
			lo = sr.holdLength(sr.turns[k].Angle)
			k++
		}
		if kinds[i+1] == cutCorner {
			hi = sr.holdLength(sr.turns[k].Angle)
		}
		l := pathLength(p)
		if lo+hi == 0 || lo+hi > 0.9*l {
			out = append(out, p)
			outKinds = append(outKinds, cutNone)
			continue
		}
		var parts []pathPiece
		rest := p
		if lo > 0 {
			a, b := splitAlong(rest, lo)
			parts, rest = append(parts, a), b
		}
		if hi > 0 {
			a, b := splitAlong(rest, pathLength(rest)-hi)
			parts, rest = append(parts, a), b
		}
		parts = append(parts, rest)
		for j, q := range parts {
			out = append(out, q)
			if j+1 < len(parts) {
				outKinds = append(outKinds, cutSmooth)
			} else {
				outKinds = append(outKinds, cutNone)
			}
		}
	}
	outKinds[len(outKinds)-1] = cutNone
	return out, outKinds
}

// spreadTwist spreads the twist over the runs that touch no corner, by
// length, each run's share its own: those beside a corner are held
// still. A pathError where every run is beside a corner, leaving
// nowhere to twist.
func (sr *sweepRuns) spreadTwist(twist float64) error {
	held := func(k int) bool {
		return sr.pre(k) && sr.corner(k-1) || sr.post(k) && sr.corner(k)
	}
	total := 0.0
	lengths := make([]float64, len(sr.chains))
	for k, run := range sr.chains {
		for _, p := range run {
			lengths[k] += pathLength(p)
		}
		if !held(k) {
			total += lengths[k]
		}
	}
	if total == 0 {
		x := sr.turns[sr.firstCorner()].Center
		return pathErr(&x, "the path has a sharp corner at (%g, %g, %g) next to a cubic, and the profile is "+
			"twisted along it: the twist, held still beside each such corner as far as its mitre would reach, "+
			"has no stretch of the path left to turn on", x.X, x.Y, x.Z)
	}
	sr.twists = make([]float64, len(sr.chains))
	for k := range sr.chains {
		if !held(k) {
			sr.twists[k] = twist * lengths[k] / total
		}
	}
	return nil
}

// pieceLength is the piece's length, summed over chords of its curves.
func pathLength(p pathPiece) float64 {
	l := 0.0
	for i := range p.curves {
		l += curveLength(&p.curves[i], 1)
	}
	return l
}

// curveLength is the length of sb from its start to t, summed over
// chords.
func curveLength(sb *srf.SBezier, t float64) float64 {
	const n = 256
	l, prev := 0.0, sb.PointAt(0)
	for j := 1; j <= n; j++ {
		q := sb.PointAt(t * float64(j) / n)
		l += q.Minus(prev).Magnitude()
		prev = q
	}
	return l
}

// splitAlong cuts p where it is d along it from its start, into the
// piece up to there and the piece on from it.
func splitAlong(p pathPiece, d float64) (pathPiece, pathPiece) {
	ci := 0
	for ; ci+1 < len(p.curves); ci++ {
		l := curveLength(&p.curves[ci], 1)
		if d <= l {
			break
		}
		d -= l
	}
	sb := &p.curves[ci]
	// Bisect for the parameter at d along the curve.
	lo, hi := 0.0, 1.0
	for range 50 {
		if m := (lo + hi) / 2; curveLength(sb, m) < d {
			lo = m
		} else {
			hi = m
		}
	}
	t := (lo + hi) / 2
	return splitPiece(p, ci, t, sb.PointAt(t))
}

// clearOfArcs is a pathError where the profile, placed where a run's
// arc starts, reaches the arc's axis, as a rounded corner too small for
// it does: the sweep along it would fold. A rigid sweep's path is
// checked the same way (sweepPath).
func (sr *sweepRuns) clearOfArcs(sbls *srf.SBezierLoopSetSet, starts []srf.Rigid) error {
	for k, chain := range sr.chains {
		s := 0
		for _, p := range chain {
			if p.arc {
				if err := clearOfArc(sbls, starts[k].Then(sr.runs[k].Rigid(float64(s))), p); err != nil {
					return err
				}
			}
			s += len(p.curves)
		}
	}
	return nil
}

// clearOfArc is clearOfArcs' error for the arc p, the profile placed
// at its start by at.
func clearOfArc(sbls *srf.SBezierLoopSetSet, at srf.Rigid, p pathPiece) error {
	radial := p.start.Minus(p.center)
	for si := range sbls.Sets {
		for _, l := range sbls.Sets[si].Loops() {
			for ci := range l.Curves {
				for _, q := range l.Curves[ci].Ctrl[:l.Curves[ci].Degree+1] {
					r := at.Apply(q).Minus(p.center)
					if r.Minus(p.axis.ScaledBy(r.Dot(p.axis))).Dot(radial) > sweepTol {
						continue
					}
					x := p.center
					if p.rounded {
						return pathErr(&x, "the profile reaches the axis of the rounded corner about "+
							"(%g, %g, %g): the corner radius is too small for it", x.X, x.Y, x.Z)
					}
					return pathErr(&x, "the profile reaches the axis of the arc about (%g, %g, %g): "+
						"the bend is too tight for it", x.X, x.Y, x.Z)
				}
			}
		}
	}
	return nil
}

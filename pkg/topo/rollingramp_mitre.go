package topo

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/srf"
)

// A seam crossed within a ramp a hair short of an inside mitre: the
// section rampCrossings puts on the seam stands so near the mitre's,
// whose weights aren't 1, that the skin's weights interpolate below zero
// between them. The blend is tried again without it (rampSkipNear): the
// foot crosses the seam within the end patch, and the piece of it past
// the seam is carried onto the next patch after the faces are cut back,
// as a mitre's foot that crosses onto the mate's patch is
// (mitreSeam, rollingmitre_seam.go).

// rampNearShare is how near an end, as a share of the sections' span, a
// ramp's section on a seam stands for the skin through it to fail as
// crossingNearEnd says.
const rampNearShare = 0.01

// rbRampSkip is a ramp crossing left without a section of its own: the
// end it is near, and the patch of the shared face the chain's foot is
// on before it crosses, away from the end.
type rbRampSkip struct {
	spec  rbSpec
	end   int
	inner int
}

// rampSkipNear marks the crossings of ramped whose sections, at arc
// lengths secS, stand within rampNearShare of an inside mitre end
// (from s0 to s1), to be left without a section; false where there are
// none, or they were left out already.
func (rb *rollingBall) rampSkipNear(ramped []rbRampCrossing, secS []float64, s0, s1 float64) bool {
	if len(rb.rampSkips) > 0 {
		return false
	}
	var skips []rbRampSkip
	for x, rc := range ramped {
		k := rc.spec.end
		end := rb.ends[k]
		if end == nil || !end.mitre || !end.concave || rc.side != 1-rb.creaseSide(k) {
			continue
		}
		d := secS[x] - s0
		if k == 1 {
			d = s1 - secS[x]
		}
		if d >= rampNearShare*(s1-s0) {
			continue
		}
		// The ramp's scan runs up the arc length: at the start the face
		// away from the end is the later one.
		inner := rc.faces[1]
		if k == 1 {
			inner = rc.faces[0]
		}
		skips = append(skips, rbRampSkip{spec: rc.spec, end: k, inner: inner})
	}
	rb.rampSkips = skips
	return len(skips) > 0
}

// withoutSkipped is ramped less the crossings rampSkipNear left out.
func (rb *rollingBall) withoutSkipped(ramped []rbRampCrossing) []rbRampCrossing {
	if len(rb.rampSkips) == 0 {
		return ramped
	}
	return slices.DeleteFunc(ramped, func(rc rbRampCrossing) bool {
		return slices.ContainsFunc(rb.rampSkips, func(sk rbRampSkip) bool { return sk.spec == rc.spec })
	})
}

// skippedInner is the patch the chain's foot is on short of the seam
// left without a section near end k (rampSkipNear).
func (rb *rollingBall) skippedInner(k int) (int, bool) {
	for _, sk := range rb.rampSkips {
		if sk.end == k {
			return sk.inner, true
		}
	}
	return -1, false
}

// mitreFootFace is the patch of the shared face end k's foot of bl runs
// along up to the mitre's point m: the end section's, but where the seam
// near the end was left without a section, the patch short of it.
func (bl *rbBlend) mitreFootFace(k, i int) (int, *srf.SSurface) {
	f := bl.endSec(k).foot[i].f
	if g, ok := bl.rb.skippedInner(k); ok {
		f = g
	}
	return f, bl.rb.t.Surface(f)
}

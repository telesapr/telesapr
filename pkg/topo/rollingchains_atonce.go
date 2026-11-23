package topo

import (
	"errors"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/par"
)

// The chains of one group joined by mitres, blended at once. Blended in
// order, two chains meeting in a mitre share it: the first to reach it
// fits it (curvedEndSection) and the other copies it (fromMate), so each
// chain waited for the one before. Here every mitre is fitted first, by
// the chain that comes first in the group, as blending in order would
// have, and then the chains are blended side by side: each reads of the
// others only what is fixed by then, their mitres, radii and the
// centres their balls run through near their ends (centresNear, worked
// out beforehand for that reason). The result is the one blending in
// order gives, to the bit, whatever the goroutines' timing, and so is
// the error, that of the first chain in order that fails.

// errMitreLater says that an end's mitre is left for its chain's blend
// to fit, which refuses it (blendOpen's check of the end's plane).
var errMitreLater = errors.New("topo: the mitre is fitted with its chain")

// blendChainsAtOnce blends the chains rbs, which meet in mitres, their
// spans and kinked mitres set (blendMitred): the blends in the order of
// rbs, or the error of the first chain that fails.
func blendChainsAtOnce(rbs []*rollingBall) ([]*rbBlend, error) {
	// What a chain's blend changes of it and is read again by the next
	// try at other spans (blendGroup): a chain after the first that
	// fails is put back as it was, as if it hadn't been blended.
	saved := make([]rbBlendState, len(rbs))
	for i, rb := range rbs {
		saved[i] = rb.blendState()
	}
	// The centres are worked out once, on first use, by whichever chain
	// asks: each chain's at each mitred end, all at once, before any
	// reads another's.
	type at struct {
		rb *rollingBall
		k  int
	}
	var ends []at
	for _, rb := range rbs {
		for k, end := range rb.ends {
			if !rb.closed && end != nil && end.mitre {
				ends = append(ends, at{rb, k})
			}
		}
	}
	par.For(len(ends), func(i int) { ends[i].rb.centresNear(ends[i].k) })
	pos := make(map[*rollingBall]int, len(rbs))
	for i, rb := range rbs {
		pos[rb] = i
	}
	fitErrs := make([]error, len(rbs))
	// Beside the fits, the last sections of the mitres each chain copies
	// from its mate, which its blend finds first and endSection keeps:
	// found here, they are what the blend would find (keepsEndSection),
	// so its blend starts with both. An error is left for the blend to
	// find again, in its place.
	var copied []at
	for _, rb := range rbs {
		for k, end := range rb.ends {
			if !rb.closed && end != nil && end.mitre && end.mate != rb && pos[end.mate] < pos[rb] &&
				rb.keepsEndSection(k) && end.n.Dot(rb.beyond(k)) >= endAlong {
				copied = append(copied, at{rb, k})
			}
		}
	}
	par.For(len(rbs)+len(copied), func(i int) {
		if i >= len(rbs) {
			c := copied[i-len(rbs)]
			_, _, _ = c.rb.endSection(c.k, 0)
			return
		}
		fitErrs[i] = rbs[i].fitMitres(pos)
	})
	// The chains up to the first whose mitre couldn't be fitted are
	// blended: it fails again in its blend, where it fails in order, and
	// the chains after it would not have been reached.
	n := len(rbs)
	if i := slices.IndexFunc(fitErrs, func(err error) bool { return err != nil }); i >= 0 {
		n = i + 1
	}
	bls := make([]*rbBlend, n)
	errs := make([]error, n)
	par.For(n, func(i int) {
		rb := rbs[i]
		sf, secs, cross, err := rb.blendOpen()
		if err == nil {
			bls[i], err = rb.patches(sf, secs, cross)
		}
		errs[i] = err
	})
	failed := slices.IndexFunc(errs, func(err error) bool { return err != nil })
	if failed < 0 && n < len(rbs) {
		failed = n - 1
		errs[failed] = fitErrs[failed]
	}
	if failed < 0 {
		return bls, nil
	}
	for i := failed + 1; i < len(rbs); i++ {
		rbs[i].setBlendState(saved[i])
	}
	return nil, errs[failed]
}

// fitMitres fits the mitres of the chain's ends that meet another
// chain's coming later in the group, each chain's place in it pos, and
// keeps each as the end's shared section, which the other copies; the
// error of the first that can't be fitted. Kinked mitres are fitted
// already, and an end that meets its own chain's is fitted in its blend.
func (rb *rollingBall) fitMitres(pos map[*rollingBall]int) error {
	if rb.closed {
		return nil
	}
	// The ends to fit, in order, up to one whose plane is refused.
	var fit []int
	var refused error
	for k, end := range rb.ends {
		if end == nil || !end.mitre || end.mate == rb || pos[end.mate] < pos[rb] || rb.kinkedEnd(k) || rb.mateKinked(k) {
			continue
		}
		// The mitre's plane halves the turn at the crease, so it faces
		// on past the end; blendOpen refuses one that faces along it.
		if end.n.Dot(rb.beyond(k)) < endAlong {
			refused = errMitreLater
			break
		}
		fit = append(fit, k)
	}
	// A chain's two mitres are fitted at once: each reads of the chain
	// only what its own end sets (endS, mitreS, mitreC by end) and what
	// is fixed by now. They are kept in order, up to the first that
	// fails, as fitting them in turn would.
	secs := make([]rbSection, len(fit))
	errs := make([]error, len(fit))
	rb.eachWhile(len(fit), func(i int) bool {
		k := fit[i]
		// Beside the end's section, the searches its fit to the swept
		// surfaces' cut makes next (curvedEndSection's points), which
		// don't depend on it: the sections they find are kept
		// (keptSection), and the fit finds them again at once.
		var sec rbSection
		var err error
		rb.each(2, func(j int) {
			if j == 1 {
				rb.warmMitreCut(k)
				return
			}
			sec, _, err = rb.endSection(k, 0)
		})
		if err == nil {
			sec, err = rb.curvedEndSection(k, sec)
		}
		secs[i], errs[i] = sec, err
		return err == nil
	})
	for i, k := range fit {
		if errs[i] != nil {
			return errs[i]
		}
		rb.ends[k].shared = &secs[i]
	}
	return refused
}

// warmMitreCut makes the searches curvedEndSection makes for mitre end
// k's points (fitCut), for the sections they find to be kept, where they
// are (keptSection); what they find is dropped.
func (rb *rollingBall) warmMitreCut(k int) {
	end := rb.ends[k]
	if rb.kept == nil || !end.mitre {
		return
	}
	n := rb.mitreCutPoints()
	rb.each(n-1, func(i int) {
		tj := float64(i+1) / float64(n)
		_, _, _ = rb.mitreRoot(k, func(x rbSection) float64 { return rb.endHeight(end, x.arc.PointAt(tj)) })
	})
}

// rbBlendState is what a chain's blend changes of it that a later blend
// of it reads: its ends' planes, turned to face on past the ends, the
// ball's first and last own sections and whether its ramps are eased.
// An eased ramp is kept for the chain's later blends on purpose
// (rollingBall.easeRamp); only a chain with no mitre is ever eased.
type rbBlendState struct {
	n        [2]geom.Vector
	d, sign  [2]float64
	target   [2]float64
	easeRamp bool
}

// blendState is the chain's rbBlendState.
func (rb *rollingBall) blendState() rbBlendState {
	st := rbBlendState{target: rb.target, easeRamp: rb.easeRamp}
	for k, end := range rb.ends {
		if end != nil {
			st.n[k], st.d[k], st.sign[k] = end.n, end.d, end.sign
		}
	}
	return st
}

// setBlendState puts the chain back to st.
func (rb *rollingBall) setBlendState(st rbBlendState) {
	rb.target, rb.easeRamp = st.target, st.easeRamp
	for k, end := range rb.ends {
		if end != nil {
			end.n, end.d, end.sign = st.n[k], st.d[k], st.sign[k]
		}
	}
}

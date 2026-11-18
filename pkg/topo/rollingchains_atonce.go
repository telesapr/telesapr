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
	// asks: each chain its own, before any reads another's.
	par.For(len(rbs), func(i int) {
		rb := rbs[i]
		for k, end := range rb.ends {
			if !rb.closed && end != nil && end.mitre {
				rb.centresNear(k)
			}
		}
	})
	pos := make(map[*rollingBall]int, len(rbs))
	for i, rb := range rbs {
		pos[rb] = i
	}
	fitErrs := make([]error, len(rbs))
	par.For(len(rbs), func(i int) {
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
	for k, end := range rb.ends {
		if end == nil || !end.mitre || end.mate == rb || pos[end.mate] < pos[rb] || rb.kinkedEnd(k) || rb.mateKinked(k) {
			continue
		}
		// The mitre's plane halves the turn at the crease, so it faces
		// on past the end; blendOpen refuses one that faces along it.
		if end.n.Dot(rb.beyond(k)) < endAlong {
			return errMitreLater
		}
		sec, _, err := rb.endSection(k, 0)
		if err != nil {
			return err
		}
		if sec, err = rb.curvedEndSection(k, sec); err != nil {
			return err
		}
		end.shared = &sec
	}
	return nil
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

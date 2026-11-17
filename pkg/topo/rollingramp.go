package topo

import (
	"math"
	"slices"
)

// rbRampCrossing is a seam a foot crosses within a ramp, between an
// end's section and the ball's own: the section of the ramp whose foot
// is on it, the faces either side and the side the foot is on.
type rbRampCrossing struct {
	spec  rbSpec
	faces [2]int
	side  int
}

// rampFootS is the arc length of the ball's own section side i's foot
// of end k's ramp section at share lam is taken from: from the end's
// (lam 0) to where the ball's own sections begin (lam 1).
func (rb *rollingBall) rampFootS(k, i int, lam float64) float64 {
	if rb.ends[k].mitre {
		// As mitreSectionWithin takes its first and last.
		return (1-lam)*rb.endS[k][i] + lam*rb.target[k]
	}
	return rb.rampS(k, lam)[i]
}

// rampCrossings are the seams the feet cross within the ramps of the
// ends that have one. The ball's sections between the ramps are
// scanned for them (crossings), but a seam that passes an end's corner
// a few tenths off, as one of a tube a band stands on passing the
// band's apex, is crossed within the ramp: without a section there the
// face's patch either side of the seam is cut back along the wrong
// foot, and its loop doesn't close.
func (rb *rollingBall) rampCrossings() ([]rbRampCrossing, error) {
	var out []rbRampCrossing
	for k, end := range rb.ends {
		if end.fade || end.pivot {
			continue
		}
		for i := range 2 {
			a, b := rb.rampFootS(k, i, 0), rb.target[k]
			lo, hi := math.Min(a, b), math.Max(a, b)
			if !(hi-lo > 1e-6*rb.length) {
				continue
			}
			n := int(math.Min(400, math.Max(8, 16*(hi-lo)/rb.r)))
			scan := make([]rbSection, n+1)
			for j := range scan {
				sec, err := rb.section(lo + (hi-lo)*float64(j)/float64(n))
				if err != nil {
					return nil, err
				}
				scan[j] = sec
			}
			at, pairs, sides, err := rb.crossings(scan)
			if err != nil {
				return nil, err
			}
			for x, s := range at {
				if sides[x] != i || math.Abs(s-a) < 1e-6*rb.length || math.Abs(s-b) < 1e-6*rb.length {
					// The other side's, or at the end's own foot (a seam
					// through the end's vertex, splitMitreSeams) or where
					// the ball's own sections begin, which are scanned.
					continue
				}
				lam := rb.rampLamAt(k, i, s)
				out = append(out, rbRampCrossing{spec: rbSpec{end: k, lam: lam}, faces: pairs[x], side: i})
			}
		}
	}
	slices.SortFunc(out, func(x, y rbRampCrossing) int {
		if x.spec.end != y.spec.end {
			return x.spec.end - y.spec.end
		}
		return int(math.Copysign(1, x.spec.lam-y.spec.lam))
	})
	return out, nil
}

// rampLamAt is the share of the way along end k's ramp at which side
// i's foot is the ball's own at arc length s: rampFootS runs one way
// from the end's foot to the target, so it is found by halving.
func (rb *rollingBall) rampLamAt(k, i int, s float64) float64 {
	a := rb.rampFootS(k, i, 0)
	lo, hi := 0.0, 1.0
	for range 60 {
		m := (lo + hi) / 2
		if (rb.rampFootS(k, i, m)-s)*(rb.target[k]-a) < 0 {
			lo = m
		} else {
			hi = m
		}
	}
	return (lo + hi) / 2
}

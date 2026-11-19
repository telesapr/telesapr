package topo

import (
	"errors"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/internal/par"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Rolling-ball fillets of several chains that meet in mitres: a rim that
// turns sharply at two or more creases of its wall is one chain between
// each two creases, and the chains are filleted together, each end's
// mitre fitted against the swept surface of the chain it meets there
// (rollingmitre.go). The faces are cut back for every chain before the
// solid is checked, since none is closed without the others.

// errNotMitred says that the chains through the picked edges don't all
// meet each other in mitres.
var errNotMitred = errors.New("topo: the chains don't meet in mitres")

// rollingBallFillets rounds the chains through edges, each with its
// radius of radii, or chamfers them where flat, where every end of every
// chain meets another's in a mitre; errNotMitred where they don't. The
// chains' patches get origins origin, origin+1, and so on.
func rollingBallFillets(sh *srf.SShell, t *Topology, edges []int, radii []float64, origin uint32, flat bool) error {
	return rollingBallFilletsAt(sh, t, edges, radii, origin, flat, false)
}

// rollingBallFilletsAt is rollingBallFillets; with partial, a chain's end
// that meets no other chain ends at its face as a lone chain's does
// (linkMitresAt).
func rollingBallFilletsAt(sh *srf.SShell, t *Topology, edges []int, radii []float64, origin uint32, flat, partial bool) error {
	var rbs []*rollingBall
	taken := map[int]bool{}
	for i, e := range edges {
		if taken[e] {
			continue
		}
		rb, err := t.newChain(e, radii[i], flat)
		if err != nil {
			return err
		}
		for _, pc := range rb.pieces {
			taken[pc.Edge] = true
		}
		rbs = append(rbs, rb)
	}
	if !linkMitresAt(rbs, partial) {
		return errNotMitred
	}
	// A chain is traced from its own edges and faces alone, and the
	// groups of chains joined by mitres share nothing but the faces they
	// project points on, which they find independently: every chain is
	// traced on its own goroutine, and each group blended on its own,
	// its chains side by side once their mitres are fitted
	// (blendChainsAtOnce).
	groups := mitreGroups(rbs)
	var bls []*rbBlend
	var err error
	t.shell.ProjectIndependently(func() {
		// A mitre's plane halves the turn from one chain to the other:
		// every chain is traced before any plane is set.
		if err = eachChain(rbs, func(rb *rollingBall) error { return rb.trace() }); err != nil {
			return
		}
		if err = mitreChains(rbs); err != nil {
			return
		}
		bls, err = blendGroups(rbs, groups)
	})
	if err != nil {
		return err
	}
	return putBlends(sh, bls, origin)
}

// mitreChains sets the mitres of the traced chains rbs: their turns,
// the inside mitres, the mitres' planes and kinks.
func mitreChains(rbs []*rollingBall) error {
	for _, rb := range rbs {
		rb.turn()
	}
	// Pivots whose feet on the crease are one point meet in a mitre.
	if err := insideMitres(rbs); err != nil {
		return err
	}
	for _, rb := range rbs {
		if err := rb.mitrePlane(); err != nil {
			return err
		}
	}
	if err := setKinks(rbs); err != nil {
		return err
	}
	for _, rb := range rbs[1:] {
		if rb.sigma != rbs[0].sigma {
			return filletErr(midpoint(rb.pts), "the edges meeting in a mitre are one convex and one concave: round them one at a time")
		}
	}
	return nil
}

// blendGroups blends the chains rbs, split into the groups of chains
// joined by mitres, each group on its own goroutine and its chains on
// goroutines of their own (blendChainsAtOnce); the blends are in
// the order of rbs. The error is that of the first group that failed,
// the groups in the order of their first chains.
func blendGroups(rbs []*rollingBall, groups [][]int) ([]*rbBlend, error) {
	bls := make([]*rbBlend, len(rbs))
	errs := make([]error, len(groups))
	par.For(len(groups), func(g int) {
		grp := make([]*rollingBall, len(groups[g]))
		for j, i := range groups[g] {
			grp[j] = rbs[i]
		}
		gbls, err := blendGroup(grp)
		if err != nil {
			errs[g] = err
			return
		}
		for j, i := range groups[g] {
			bls[i] = gbls[j]
		}
	})
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return bls, nil
}

// blendGroup blends the chains rbs, which meet each other in mitres.
func blendGroup(rbs []*rollingBall) ([]*rbBlend, error) {
	// As one chain's mitre (blend): sections in two spans first, which
	// follow it down to fine tolerances, then whole, then in four, finer
	// still; every chain of the group alike, as they share their mitres.
	// A chain mitred or pivoted at one end only ends at its face at the
	// other, and along a curved edge, or on a curved face, that end's
	// conics stray from the swept surface as a lone chain's do (blend):
	// its sections fitted to their cuts come last, which leave the
	// mitres' sections as they are.
	var bls []*rbBlend
	var err error
	for _, fitCuts := range []bool{false, true} {
		if fitCuts && !endsAtFace(rbs) {
			break
		}
		for _, spans := range mitreSpans {
			fbls, ferr := blendMitred(rbs, spans, fitCuts)
			if ferr == nil {
				bls, err = fbls, nil
				break
			}
			if err == nil {
				err = ferr
			}
		}
		if bls != nil {
			break
		}
	}
	if bls == nil {
		return nil, err
	}
	return bls, nil
}

// mitreGroups splits the chains rbs into the groups joined by mitres:
// the indices into rbs of each, in order, the groups in the order of
// their first chains.
func mitreGroups(rbs []*rollingBall) [][]int {
	group := make(map[*rollingBall]int, len(rbs))
	var groups [][]int
	for _, rb := range rbs {
		if _, ok := group[rb]; ok {
			continue
		}
		g := len(groups)
		groups = append(groups, nil)
		stack := []*rollingBall{rb}
		group[rb] = g
		for len(stack) > 0 {
			cur := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, end := range cur.ends {
				if m := end.mate; m != nil {
					if _, ok := group[m]; !ok {
						group[m] = g
						stack = append(stack, m)
					}
				}
			}
		}
	}
	for i, rb := range rbs {
		g := group[rb]
		groups[g] = append(groups[g], i)
	}
	return groups
}

// eachChain calls fn on every chain of rbs at once. The error is that
// of the first chain in rbs that failed, as in one loop: the chains
// before it did as they would have, and what fn did to those after is
// dropped with them, the caller going on with none.
func eachChain(rbs []*rollingBall, fn func(rb *rollingBall) error) error {
	errs := make([]error, len(rbs))
	par.For(len(rbs), func(i int) {
		errs[i] = fn(rbs[i])
	})
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// endsAtFace reports whether a chain of rbs ends at a face at either
// end, meeting no other chain there: an end that is no mitre and no
// inside corner's pivot (a chain picked with the next rim edge of a
// pocket, ending at the wall past its other end, is pivoted at one end
// only).
func endsAtFace(rbs []*rollingBall) bool {
	return slices.ContainsFunc(rbs, func(rb *rollingBall) bool {
		return !rb.closed && slices.ContainsFunc(rb.ends[:], func(e *rbEnd) bool { return e != nil && e.mate == nil && !e.fade })
	})
}

// blendMitred blends each of the chains rbs, which meet in mitres, its
// sections split into spans, and an end that meets no
// mitre fitted to its cuts where fitCuts: an end fits the mitre where
// the chain it meets there comes later in rbs, and copies the other's.
func blendMitred(rbs []*rollingBall, spans int, fitCuts bool) ([]*rbBlend, error) {
	for _, rb := range rbs {
		rb.spans, rb.fitCuts = spans, fitCuts
		for _, end := range rb.ends {
			end.shared = nil
		}
	}
	// A kinked mitre is the larger end's to fit, and round a rim each
	// chain's may copy the one before's: every one is fitted first.
	for _, rb := range rbs {
		for k := range rb.ends {
			if !rb.kinkedEnd(k) {
				continue
			}
			sec, _, err := rb.endSection(k, 0)
			if err != nil {
				return nil, err
			}
			rb.ends[k].shared = &sec
		}
	}
	return blendChainsAtOnce(rbs)
}

// filletMitredPicks rounds the picked edges as chains that meet in
// mitres (rollingBallFillets), each of its pick's radius, r where it has
// none, and each one a rolling ball's; errNotMitred where they aren't.
// The shell is changed only when it succeeds.
func filletMitredPicks(sh *srf.SShell, t *Topology, picks []FilletPick, r float64, origin uint32, flat bool) error {
	edges := make([]int, 0, len(picks))
	radii := pickRadii(picks, r)
	for _, p := range picks {
		var e int
		var err error
		if p.Arc {
			if e = t.curvedEdgeAt(p.At); e < 0 {
				return errNotMitred
			}
		} else if e, err = t.wholeEdge(p.A, p.B); err != nil {
			return errNotMitred
		}
		if !t.generalBlendEdge(e) {
			return errNotMitred
		}
		edges = append(edges, e)
	}
	var cp srf.SShell
	cp.MakeFromCopyOf(sh)
	if err := rollingBallFillets(&cp, t, edges, radii, origin, flat); err != nil {
		return err
	}
	*sh = cp
	return nil
}

// filletCornerPicks rounds picks, each of its own radius or r, that meet
// at corners where the third edge stays sharp, some of them the rolling
// ball's (an edge ending on a curved face): every pick a rolling-ball chain, the
// chains meeting in mitres there (rollingmitre.go) and their other ends
// at their faces, flat or curved, as a lone chain's. It is what
// filletApart refuses, its picks' fillets meeting; errNotMitred where
// the picks don't meet so. The shell is changed only when it succeeds.
func filletCornerPicks(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool) error {
	t, err := Build(sh)
	if err != nil {
		return errNotMitred
	}
	edges := make([]int, 0, len(picks))
	for _, p := range picks {
		e, err := t.pickEdge(p)
		if err != nil {
			return errNotMitred
		}
		edges = append(edges, e)
	}
	var cp srf.SShell
	cp.MakeFromCopyOf(sh)
	if err := rollingBallFilletsAt(&cp, t, edges, pickRadii(picks, r), origin, flat, true); err != nil {
		return err
	}
	*sh = cp
	return nil
}

// pickRadii are the picks' radii, r for a pick with none of its own.
func pickRadii(picks []FilletPick, r float64) []float64 {
	radii := make([]float64, len(picks))
	for i, p := range picks {
		radii[i] = r
		if p.R != 0 {
			radii[i] = p.R
		}
	}
	return radii
}

// setKinks finds the kink of every chain's larger end at a mitre of two
// radii (rollingkink.go) and gives each chain its kinks. A chain's two
// kinks within kinkMerge of each other are one, both ends taking it.
// Chains of one radius meeting in a mitre share its curve as it is, one
// way or the other, so each takes the other's kinks: they go on along
// such chains until none is new.
func setKinks(rbs []*rollingBall) error {
	for _, rb := range rbs {
		rb.kinks = nil
		for k := range rb.ends {
			if !rb.kinkedEnd(k) {
				continue
			}
			if err := rb.findKink(k); err != nil {
				return err
			}
			kt := rb.ends[k].kink
			if i := slices.IndexFunc(rb.kinks, func(x float64) bool { return math.Abs(x-kt) < kinkMerge }); i >= 0 {
				rb.ends[k].kink = rb.kinks[i]
				continue
			}
			rb.kinks = append(rb.kinks, kt)
		}
	}
	for range rbs {
		changed := false
		for _, rb := range rbs {
			for _, end := range rb.ends {
				if rb.closed || end == nil || !end.mitre || end.mate == rb || end.mate.r != rb.r || end.deeper || end.mate.ends[end.mateK].deeper {
					continue
				}
				back := (end.ban[0] < 0) != (end.mate.ends[end.mateK].ban[0] < 0)
				for _, t := range end.mate.kinks {
					if back {
						t = 1 - t
					}
					if !slices.ContainsFunc(rb.kinks, func(x float64) bool { return math.Abs(x-t) < kinkMerge }) {
						rb.kinks = append(rb.kinks, t)
						changed = true
					}
				}
			}
		}
		if !changed {
			break
		}
	}
	for _, rb := range rbs {
		slices.Sort(rb.kinks)
	}
	return nil
}

// kinkMerge is how near two kinks' arc parameters are taken as one.
const kinkMerge = 1e-6

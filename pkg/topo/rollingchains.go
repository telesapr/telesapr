package topo

import (
	"errors"
	"slices"

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
	// A mitre's plane halves the turn from one chain to the other: every
	// chain is traced before any plane is set.
	for _, rb := range rbs {
		if err := rb.trace(); err != nil {
			return err
		}
	}
	for _, rb := range rbs {
		if err := rb.mitrePlane(); err != nil {
			return err
		}
		rb.turn()
	}
	for _, rb := range rbs[1:] {
		if rb.sigma != rbs[0].sigma {
			return filletErr(midpoint(rb.pts), "the edges meeting in a mitre are one convex and one concave: round them one at a time")
		}
	}
	// As one chain's mitre (blend): split sections first, which follow it
	// down to fine tolerances; every chain alike, as the chains share
	// their mitres.
	// A chain mitred at one end only ends at its face at the other, and
	// along a curved edge that end's conics stray from the swept surface
	// as a lone chain's do (blend): its sections fitted to their cuts
	// come last, which leave the mitres' sections as they are.
	var bls []*rbBlend
	var err error
	for _, try := range [][2]bool{{true, false}, {false, false}, {true, true}, {false, true}} {
		if try[1] && !partlyMitred(rbs) {
			break
		}
		fbls, ferr := blendMitred(rbs, try[0], try[1])
		if ferr == nil {
			bls, err = fbls, nil
			break
		}
		if err == nil {
			err = ferr
		}
	}
	if err != nil {
		return err
	}
	return putBlends(sh, bls, origin)
}

// partlyMitred reports whether a chain of rbs is mitred at one end only.
func partlyMitred(rbs []*rollingBall) bool {
	return slices.ContainsFunc(rbs, func(rb *rollingBall) bool { return rb.anyMitre() && !rb.mitred() })
}

// blendMitred blends each of the chains rbs, which meet in mitres, its
// sections split at their middles where split, and an end that meets no
// mitre fitted to its cuts where fitCuts: an end fits the mitre where
// the chain it meets there came later, and copies the other's.
func blendMitred(rbs []*rollingBall, split, fitCuts bool) ([]*rbBlend, error) {
	for _, rb := range rbs {
		rb.splitEnds, rb.fitCuts = split, fitCuts
		for _, end := range rb.ends {
			end.shared = nil
		}
	}
	bls := make([]*rbBlend, 0, len(rbs))
	for _, rb := range rbs {
		sf, secs, cross, err := rb.blendOpen()
		if err != nil {
			return nil, err
		}
		bl, err := rb.patches(sf, secs, cross)
		if err != nil {
			return nil, err
		}
		bls = append(bls, bl)
	}
	return bls, nil
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

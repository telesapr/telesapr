package topo

import (
	"errors"

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

// rollingBallFillets rounds the chains through edges with radius r, or
// chamfers them where flat, where every end of every chain meets
// another's in a mitre; errNotMitred where they don't. The chains'
// patches get origins origin, origin+1, and so on.
func rollingBallFillets(sh *srf.SShell, t *Topology, edges []int, r float64, origin uint32, flat bool) error {
	var rbs []*rollingBall
	taken := map[int]bool{}
	for _, e := range edges {
		if taken[e] {
			continue
		}
		rb, err := t.newChain(e, r, flat)
		if err != nil {
			return err
		}
		for _, pc := range rb.pieces {
			taken[pc.Edge] = true
		}
		rbs = append(rbs, rb)
	}
	if !linkMitres(rbs) {
		return errNotMitred
	}
	if flat {
		// A mitre is where the swept surfaces meet, found from the ball's
		// centres; a chamfer has no ball.
		return filletErr(rbs[0].t.Vertices[rbs[0].ends[0].v].P, "chamfers that meet at a crease of the wall "+
			"are not mitred: fillet the edges, or chamfer them one at a time")
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
	bls, err := blendMitred(rbs, true)
	if err != nil {
		if fbls, ferr := blendMitred(rbs, false); ferr == nil {
			bls, err = fbls, nil
		}
	}
	if err != nil {
		return err
	}
	return putBlends(sh, bls, origin)
}

// blendMitred blends each of the chains rbs, which meet in mitres, its
// sections split at their middles where split: an end fits the mitre
// where the chain it meets there came later, and copies the other's.
func blendMitred(rbs []*rollingBall, split bool) ([]*rbBlend, error) {
	for _, rb := range rbs {
		rb.splitEnds = split
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
// mitres (rollingBallFillets), all of one radius and each one a rolling
// ball's; errNotMitred where they aren't. The shell is changed only when
// it succeeds.
func filletMitredPicks(sh *srf.SShell, t *Topology, picks []FilletPick, r float64, origin uint32, flat bool) error {
	edges := make([]int, 0, len(picks))
	for _, p := range picks {
		if p.R != 0 && p.R != r {
			return errNotMitred
		}
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
	if err := rollingBallFillets(&cp, t, edges, r, origin, flat); err != nil {
		return err
	}
	*sh = cp
	return nil
}

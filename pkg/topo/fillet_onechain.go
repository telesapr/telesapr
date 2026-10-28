package topo

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// One closed chain: picks that are the pieces of one closed edge of the
// solid, going on round from one to the next where the faces at its
// sides meet smoothly, as the foot of a rib whose top was rounded first
// goes round from its flat sides onto the fillets of its uprights and
// of its slope's edges. Step 10 takes the straight pieces between flat
// faces but not their ends on the fillets, and the rolling ball takes
// the whole chain as one; it was refused as a rolling pick with others
// before.

// errNotOneChain says that filletOneChain doesn't take the picks.
var errNotOneChain = errors.New("topo: the picks aren't one closed chain")

// filletOneChain rounds picks of sh with radius r, or chamfers them
// where flat, by the rolling ball along the closed chain they make. Every
// piece of the chain must be picked or, where fresh is given, be fresh:
// an edge a stage before made (filletInStages), as the arcs a rib's
// rounded uprights leave on its foot. errNotOneChain where the picks
// aren't such a chain; the shell is changed only when it is rounded.
func filletOneChain(sh *srf.SShell, picks []FilletPick, r float64, origin uint32, flat bool, fresh func(t *Topology, e int) bool) error {
	if len(picks) < 2 {
		return errNotOneChain
	}
	var cp srf.SShell
	cp.MakeFromCopyOf(sh)
	t, err := Build(&cp)
	if err != nil {
		return errNotOneChain
	}
	picked := map[int]bool{}
	first := -1
	for _, p := range picks {
		e, err := t.pickEdge(p)
		if err != nil {
			return errNotOneChain
		}
		if p.R != 0 && math.Abs(p.R-r) > geom.LengthEps {
			return errNotOneChain
		}
		picked[e] = true
		if first < 0 || t.generalBlendEdge(e) && !t.generalBlendEdge(first) {
			first = e
		}
	}
	pieces, _, closed := t.blendChain(first)
	if !closed {
		return errNotOneChain
	}
	rolling, inChain := false, map[int]bool{}
	for _, pc := range pieces {
		inChain[pc.Edge] = true
		if !picked[pc.Edge] && (fresh == nil || !fresh(t, pc.Edge)) {
			return errNotOneChain
		}
		rolling = rolling || t.generalBlendEdge(pc.Edge)
	}
	if !rolling {
		return errNotOneChain
	}
	for e := range picked {
		if !inChain[e] {
			return errNotOneChain
		}
	}
	if err := rollingBallFillet(&cp, t, first, r, origin, flat); err != nil {
		if errors.Is(err, errNotRolling) {
			return errNotOneChain
		}
		return byRollingBall(err)
	}
	*sh = cp
	return nil
}

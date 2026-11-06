package sketch

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// movesInMechanism reports whether group g is a part of a mechanism: a
// link flagged to move (Group.LinkMoves) that is not left out whole.
func movesInMechanism(g *Group) bool {
	return g != nil && g.Type == GroupLinked && g.LinkMoves && g.SuppressedBy.V == 0
}

// Mechanism is the mechanism group hg is a part of, its groups in the
// group order: the longest run of consecutive groups that move in a
// mechanism (movesInMechanism) containing hg. Any other group stands
// alone, and is a mechanism of one, [hg], so that a caller treats every
// group as a block of groups solved together. A group of any other kind
// between two flagged links, a 3D sketch for a datum too, ends the run:
// such a group goes before the mechanism or after it.
func (sk *Sketch) Mechanism(hg core.HGroup) []core.HGroup {
	// Asked of every group at every regeneration: the flag first, the
	// search of the group order only for a part that moves.
	if !movesInMechanism(sk.Group(hg)) {
		return []core.HGroup{hg}
	}
	i := slices.Index(sk.GroupOrder, hg)
	if i < 0 {
		return []core.HGroup{hg}
	}
	lo, hi := i, i+1
	for lo > 0 && movesInMechanism(sk.Group(sk.GroupOrder[lo-1])) {
		lo--
	}
	for hi < len(sk.GroupOrder) && movesInMechanism(sk.Group(sk.GroupOrder[hi])) {
		hi++
	}
	return slices.Clone(sk.GroupOrder[lo:hi])
}

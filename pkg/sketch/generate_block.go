package sketch

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// generateGroup makes group hg's entities and params: those of its
// requests, its constraints and the group itself, the lists kept sorted
// by handle. It returns the params the group's solve may change, the
// ones it made.
func (sk *Sketch) generateGroup(hg core.HGroup) []core.HParam {
	g := sk.Group(hg)
	before := make(map[core.HParam]bool, len(sk.Params))
	for _, p := range sk.Params {
		before[p.H] = true
	}
	groupRequestIndex := 0
	for ri := range sk.Requests {
		r := &sk.Requests[ri]
		if r.Group != hg {
			continue
		}
		r.GroupRequestIndex = groupRequestIndex
		groupRequestIndex++

		entities := core.EntityList(sk.Entities)
		params := core.ParamList(sk.Params)
		r.Generate(sk, &entities, &params)
		sk.Entities = []core.Entity(entities)
		sk.Params = []core.Param(params)
	}
	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		if c.Group != hg {
			continue
		}
		GenerateConstraint(c, &sk.Params)
	}
	// The entity and param lists are kept sorted by handle; the group
	// iterates over them in that order.
	sk.sortByHandle()
	g.Generate(sk, &sk.Entities, &sk.Params)
	sk.sortByHandle()
	made := make([]core.HParam, 0, len(sk.Params)-len(before))
	for _, p := range sk.Params {
		if !before[p.H] {
			made = append(made, p.H)
		}
	}
	return made
}

// blockFrom is the block of groups regenerated together that starts at
// the group order's index i: the groups of a mechanism (Sketch.Mechanism)
// from i on, or the group there alone.
func (sk *Sketch) blockFrom(i int) []core.HGroup {
	hg := sk.GroupOrder[i]
	if !movesInMechanism(sk.Group(hg)) {
		return []core.HGroup{hg}
	}
	m := sk.Mechanism(hg)
	return m[slices.Index(m, hg):]
}

// widenToMechanism widens the range of group order indices first to
// last, those a regeneration solves, to the ends of the mechanisms
// either end falls in, so that a change in any group of a mechanism
// solves all of them, and none is solved without the others. An index
// out of the group order (none to solve, or every group after) is
// left as it is.
func (sk *Sketch) widenToMechanism(first, last int) (int, int) {
	n := len(sk.GroupOrder)
	if first >= 0 && first < n {
		first = slices.Index(sk.GroupOrder, sk.Mechanism(sk.GroupOrder[first])[0])
	}
	if last >= 0 && last < n && last != math.MaxInt {
		m := sk.Mechanism(sk.GroupOrder[last])
		last = slices.Index(sk.GroupOrder, m[len(m)-1])
	}
	return first, last
}

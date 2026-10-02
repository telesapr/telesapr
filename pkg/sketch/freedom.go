package sketch

import (
	"github.com/spacesolve/spacesolve/pkg/core"
)

// This file is not a port: C++ SolveSpace shows which points are free
// only on demand, with Analyze > Show Degrees of Freedom. Here each
// entity of a group can tell how much of it its constraints fix, after
// every regeneration, for colouring and for scripts.

// Freedom is how much of an entity its group's constraints fix.
type Freedom int

const (
	// FreedomUnknown is for an entity whose group did not solve, or
	// whose state was not worked out.
	FreedomUnknown Freedom = iota
	// FreedomFixed is an entity none of whose own parameters can move:
	// it is fully constrained.
	FreedomFixed
	// FreedomPartly is an entity some of whose parameters can move.
	FreedomPartly
	// FreedomFree is an entity all of whose parameters can move: no
	// constraint holds it.
	FreedomFree
)

func (f Freedom) String() string {
	switch f {
	case FreedomFixed:
		return "fixed"
	case FreedomPartly:
		return "partly free"
	case FreedomFree:
		return "free"
	}
	return "unknown"
}

// freedomCache is the free params of one group at one generation.
type freedomCache struct {
	generation uint64
	group      core.HGroup
	free       map[core.HParam]bool
}

// GroupFreedom reports which of the params group hg solves for are
// free to move, at the values of the last regeneration; nil when the
// group did not solve or is too large. The answer is kept until the
// next regeneration.
func (sk *Sketch) GroupFreedom(hg core.HGroup) map[core.HParam]bool {
	for _, c := range sk.freedom {
		if c.group == hg && c.generation == sk.Generation {
			return c.free
		}
	}
	g := sk.GetGroup(hg)
	var free map[core.HParam]bool
	if g != nil && g.IsSolvedOkay() && hg.V != HGroupReferences {
		_ = catchPanic(func() {
			sys, sg := sk.writeEqSystemForGroup(hg, sk.groupParams(hg))
			free = sys.FreeParams(sg)
		})
	}
	// Keep the last few groups asked about.
	c := freedomCache{generation: sk.Generation, group: hg, free: free}
	if len(sk.freedom) >= 4 {
		sk.freedom = sk.freedom[1:]
	}
	sk.freedom = append(sk.freedom, c)
	return free
}

// EntityFreedom is how much of entity he its group's constraints fix:
// whether all, some or none of the params it is made of, and that its
// group solves for, can move. Params from earlier groups, such as a
// workplane's, are fixed in this group and don't count.
func (sk *Sketch) EntityFreedom(he core.HEntity) Freedom {
	e := sk.GetEntity(he)
	if e == nil {
		return FreedomUnknown
	}
	free := sk.GroupFreedom(e.Group)
	if free == nil {
		return FreedomUnknown
	}
	var params []core.HParam
	sk.entityParams(e, &params, map[core.HEntity]bool{})
	nFree, n := 0, 0
	for _, h := range params {
		f, ok := free[h]
		if !ok {
			continue
		}
		n++
		if f {
			nFree++
		}
	}
	switch {
	case n == 0 || nFree == 0:
		return FreedomFixed
	case nFree == n:
		return FreedomFree
	}
	return FreedomPartly
}

// entityParams appends the params entity e is made of: its own, and
// those of its points, normal and distance.
func (sk *Sketch) entityParams(e *core.Entity, out *[]core.HParam, seen map[core.HEntity]bool) {
	if seen[e.H] {
		return
	}
	seen[e.H] = true
	for _, h := range e.Param {
		if h.V != 0 {
			*out = append(*out, h)
		}
	}
	sub := func(h core.HEntity) {
		if h.V == 0 {
			return
		}
		if se := sk.GetEntity(h); se != nil {
			sk.entityParams(se, out, seen)
		}
	}
	for _, h := range e.Point {
		sub(h)
	}
	sub(e.Normal)
	sub(e.Distance)
}

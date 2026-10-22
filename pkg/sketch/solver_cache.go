package sketch

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/solver"
)

// keptSystem is the system that solves group hg for groupParams: the
// one that solved it last, if it solved the same params, with every
// unknown's value taken from the sketch again, else a new one, kept.
// The solver works on copies of the group's params, with the sketch's
// values as initial guesses; a system kept keeps the Jacobians it
// wrote, which point at its copies, so that a solve of the group with
// the same equations, as each step of a drag is, evaluates them rather
// than writing them again. What they were written from is checked to
// the bit (solver.System.WriteJacobian), so a kept system solves as a
// new one would.
func (sk *Sketch) keptSystem(hg core.HGroup, groupParams []core.HParam) *solver.System {
	if !sk.NoSolverCache {
		if sys := sk.systems[hg]; sys != nil && sameParams(sys, groupParams) {
			sys.Reseed(sk.Param)
			return sys
		}
	}
	sys := &solver.System{SK: sk.Param}
	for _, h := range groupParams {
		p := *sk.Param(h)
		p.Known = false
		sys.AddParam(p)
	}
	if !sk.NoSolverCache {
		if sk.systems == nil || len(sk.systems) > 2*len(sk.Groups) {
			// Groups deleted leave their systems behind; a sketch
			// with twice as many kept as it has groups starts again.
			sk.systems = make(map[core.HGroup]*solver.System)
		}
		sk.systems[hg] = sys
	}
	return sys
}

// keepSystem lets go of the system that solved group hg if it has
// nothing worth keeping: where substitution and the equations solved
// alone leave no system to write, as for a sketch held by horizontals,
// verticals and coincidences, a kept system saves nothing and only
// adds to what the collector scans at every regeneration.
func (sk *Sketch) keepSystem(hg core.HGroup, sys *solver.System) {
	if sys.Unsolved() == 0 && sk.systems[hg] == sys {
		delete(sk.systems, hg)
	}
}

// sameParams is whether sys's unknowns are hs, which are in handle
// order as the system keeps them.
func sameParams(sys *solver.System, hs []core.HParam) bool {
	return slices.EqualFunc(sys.Param, hs, func(p *solver.Param, h core.HParam) bool { return p.H == h })
}

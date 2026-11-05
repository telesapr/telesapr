package sketch

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// spanSweep is what a fitted sweep's spans are made from: the path, one
// loop set of the profile, and the identities its faces take.
type spanSweep struct {
	sp         *topo.SweepPath
	loops      [][]srf.SBezier
	color      core.RGBA
	start, end uint32
	tol        srf.Tolerance
	// next is the identity the next span's patches start from, and its
	// caps' where they are cut: each span's are its own, so that two
	// spans' faces never share one.
	next uint32
}

// span is the shell of the sweep from lo to hi, its caps the sweep's own
// at the path's ends and new identities where it is cut.
func (ss *spanSweep) span(lo, hi float64) (*srf.SShell, error) {
	start, end := ss.start, ss.end
	if lo > 0 {
		start = ss.next
		ss.next++
	}
	if hi < ss.sp.End() {
		end = ss.next
		ss.next++
	}
	sh, err := topo.MakePathSweepSpan(ss.sp, ss.loops, ss.color, start, end, ss.next, ss.tol, lo, hi)
	if err != nil {
		return nil, err
	}
	ss.next += uint32(len(sh.Surface))
	return sh, nil
}

// spans are the shells of the sweep from lo to hi in spans each clear of
// itself; false where a span still crosses itself after depth cuts, no
// cut can be made in it, or the budget of sweeps runs out.
func (ss *spanSweep) spans(lo, hi float64, depth int, budget *int) ([]*srf.SShell, bool) {
	if *budget <= 0 {
		return nil, false
	}
	*budget--
	// A span that fails to close, where its own passes meet along an
	// edge, is cut as one that crosses itself is.
	if sh, err := ss.span(lo, hi); err == nil {
		if _, crosses := shellCrossing(sh, true); !crosses {
			return []*srf.SShell{sh}, true
		}
	}
	if depth == 0 {
		return nil, false
	}
	for _, f := range sweepCuts {
		m, ok := ss.sp.CutAt(lo, hi, f)
		if !ok {
			continue
		}
		a, ok := ss.spans(lo, m, depth-1, budget)
		if !ok {
			continue
		}
		b, ok := ss.spans(m, hi, depth-1, budget)
		if !ok {
			continue
		}
		return append(a, b...), true
	}
	return nil, false
}

// fittedInStretches is the solid a profile passes through along a fitted
// path whose sweep crosses itself, or nil where it can't be made so: as
// sweepInStretches, the union of spans each clear of itself. The whole
// path crosses itself, so it is cut at least once; where the union of
// the spans fails, the first cut is tried elsewhere, the Boolean of
// fitted skins meeting where the path comes back being what fails.
func (g *Group) fittedInStretches(sk *Sketch, src *Group, start, end uint32) *srf.SShell {
	prof := g.sweepProfile(src)
	sp := g.fittedPath
	if !sp.Spannable() {
		return nil
	}
	defer sk.lendPathPace(sp)()
	for _, f := range sweepCuts {
		m, ok := sp.CutAt(0, sp.End(), f)
		if !ok {
			continue
		}
		ss := spanSweep{sp: sp, color: g.Color, start: start, end: end, tol: sk.tol,
			next: srf.OriginFor(g.H.V, 0)}
		var parts []*srf.SShell
		for i := range prof.Sets {
			set := &prof.Sets[i]
			ss.loops = [][]srf.SBezier{set.Outer.Curves}
			for _, h := range set.Holes {
				ss.loops = append(ss.loops, h.Curves)
			}
			budget := sweepStretchSweeps
			a, okA := ss.spans(0, m, sweepStretchDepth-1, &budget)
			b, okB := ss.spans(m, sp.End(), sweepStretchDepth-1, &budget)
			if !okA || !okB {
				parts = nil
				break
			}
			parts = append(append(parts, a...), b...)
		}
		if parts == nil {
			continue
		}
		if out := joinStretches(parts, sk.tol); out != nil {
			return out
		}
	}
	return nil
}

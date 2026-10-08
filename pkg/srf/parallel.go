package srf

import (
	"log"
	"os"
	"sync/atomic"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/par"
)

// This file has what lets the kernel's long loops run on several
// goroutines and still give one result, whatever the goroutines'
// timing: the result of the loops run in order.
//
// A loop whose iterations write only what they own just runs at once
// (MakeClassifyingBsps, TriangulateInto). The loops of a Boolean over
// one operand's curves and surfaces also write to the result shell,
// which they do afterwards, in order, and would read the closest-point
// caches of the surfaces they project points on (SSurface.Cached),
// which makes a projection depend on the one before it; so for operands
// with curved surfaces the caches are off while they run. Planes have
// no use for them. The loops run the same way on one goroutine as on
// several, so the result is the same to the bit whatever the number
// (roadmap 17.1: there was an exact mode, which ran curved operands'
// loops in order with the caches on).

// ParallelMode says whether the loops run on several goroutines.
type ParallelMode int

const (
	// ParallelOn runs them on all the processors.
	ParallelOn ParallelMode = iota
	// ParallelOff runs everything on one goroutine.
	ParallelOff
)

var parallelMode = ParallelOn

// SetParallelMode sets the mode; like the chord tolerance, not while
// a sketch is generated.
func SetParallelMode(m ParallelMode) {
	parallelMode = m
	if m == ParallelOff {
		par.SetWorkers(1)
	} else {
		par.SetWorkers(0)
	}
}

// SetParallelModeFromEnv takes TELESAPR_PARALLEL: "on" (the default)
// has the solids generated on all the processors, "off" on one; the
// result is the same.
func SetParallelModeFromEnv() {
	switch v := os.Getenv("TELESAPR_PARALLEL"); v {
	case "", "on":
	case "off":
		SetParallelMode(ParallelOff)
	default:
		log.Printf("TELESAPR_PARALLEL is '%s'; it can be on or off.", v)
	}
}

// GetParallelMode returns the mode.
func GetParallelMode() ParallelMode { return parallelMode }

// planeBasis returns the origin and the u and v edges of a surface that
// is a parallelogram in a plane, for which ClosestPointTo is a formula.
func (s *SSurface) planeBasis() (orig, bu, bv geom.Vector, ok bool) {
	if s.DegM != 1 || s.DegN != 1 {
		return orig, bu, bv, false
	}
	orig = s.Ctrl[0][0]
	bu = s.Ctrl[1][0].Minus(orig)
	bv = s.Ctrl[0][1].Minus(orig)
	return orig, bu, bv, s.Ctrl[1][1].Equals(orig.Plus(bu).Plus(bv))
}

// allPlanes reports whether no surface of the shell uses its
// closest-point cache.
func (s *SShell) allPlanes() bool {
	for i := range s.Surface {
		if _, _, _, ok := s.Surface[i].planeBasis(); !ok {
			return false
		}
	}
	return true
}

// booleanLoops says how to run a loop of the Boolean of a and b over
// the curves or surfaces of one of them: always at once (on as many
// goroutines as there are, one when the mode is off), and whether that
// takes switching the closest-point caches of their surfaces off.
func booleanLoops(a, b *SShell) (atOnce, noCache bool) {
	return true, !a.allPlanes() || !b.allPlanes()
}

// setNoCache switches the closest-point caches of the shells' surfaces
// off or back on.
func setNoCache(off bool, shells ...*SShell) {
	for _, sh := range shells {
		for i := range sh.Surface {
			sh.Surface[i].noCache = off
		}
	}
}

// loop calls fn for every i in [0, n), at once or in order.
func loop(atOnce bool, n int, fn func(i int)) {
	if atOnce {
		par.For(n, fn)
		return
	}
	for i := range n {
		fn(i)
	}
}

// curveSink is where IntersectAgainst puts the curves it finds: into
// the shell directly, or, for a surface intersected while others are,
// on a list that commitTo adds to the shell afterwards.
type curveSink struct {
	shell    *SShell
	deferred bool
	entries  []sinkEntry
}

// sinkEntry is a curve on a deferred sink's list, or an exact curve
// that was considered and dropped.
type sinkEntry struct {
	curve SCurve
	added bool
	exact *exactCurve // nil for a marched curve
}

// exactCurve is a call of AddExactIntersectionCurve and the curve it
// found to follow.
type exactCurve struct {
	s, srfB        *SSurface
	sb             SBezier
	agnstA, agnstB *SShell
	match          exactMatch
}

// exactMatch is the curve identical to a new exact curve, whose pwl
// the new one follows.
type exactMatch struct {
	curve     *SCurve // nil if there is none
	backwards bool
	own       int // its entry on a deferred sink's list, or -1
}

// findExactCurve returns the first exact curve of curves that is sb or
// its reverse, or -1.
func findExactCurve(curves []SCurve, sb *SBezier) (i int, backwards bool) {
	sbrev := *sb
	sbrev.Reverse()
	for i := range curves {
		se := &curves[i]
		if se.IsExact {
			if sb.Equals(&se.Exact) {
				return i, false
			}
			if sbrev.Equals(&se.Exact) {
				return i, true
			}
		}
	}
	return -1, false
}

// findExact looks for a curve identical to sb, as far as the sink
// knows: in the shell, then on its own list.
func (k *curveSink) findExact(sb *SBezier) exactMatch {
	if i, backwards := findExactCurve(k.shell.Curve, sb); i >= 0 {
		return exactMatch{curve: &k.shell.Curve[i], backwards: backwards, own: -1}
	}
	sbrev := *sb
	sbrev.Reverse()
	for j := range k.entries {
		e := &k.entries[j]
		if !e.added || !e.curve.IsExact {
			continue
		}
		if sb.Equals(&e.curve.Exact) {
			return exactMatch{curve: &e.curve, own: j}
		}
		if sbrev.Equals(&e.curve.Exact) {
			return exactMatch{curve: &e.curve, backwards: true, own: j}
		}
	}
	return exactMatch{own: -1}
}

// add takes a marched curve.
func (k *curveSink) add(sc SCurve) {
	if !k.deferred {
		k.shell.addAndAssignCurveID(sc)
		return
	}
	k.entries = append(k.entries, sinkEntry{curve: sc, added: true})
}

// putExact takes the outcome of AddExactIntersectionCurve: the curve,
// or nil if it was dropped.
func (k *curveSink) putExact(sc *SCurve, ex exactCurve) {
	if !k.deferred {
		if sc != nil {
			k.shell.addAndAssignCurveID(*sc)
		}
		return
	}
	e := sinkEntry{exact: &ex}
	if sc != nil {
		e.curve, e.added = *sc, true
	}
	k.entries = append(k.entries, e)
}

// sinkRedone counts the exact curves commitTo made over, for the tests.
var sinkRedone atomic.Int64

// commitTo adds the list of a deferred sink to the shell, which by now
// has the curves of the surfaces before this one: those from
// into.Curve[first] on. An exact curve that found none to follow in
// the shell as it was is looked up again among them, as the loop in
// order would have; if that finds another curve than the sink knew of,
// the exact curve is added over again.
func (k *curveSink) commitTo(into *SShell, first int) {
	// Where each entry went in into.Curve; -1 if nowhere, or if it was
	// made over.
	at := make([]int, len(k.entries))
	for j := range k.entries {
		e := &k.entries[j]
		at[j] = -1
		if ex := e.exact; ex != nil && (ex.match.curve == nil || ex.match.own >= 0) {
			i, backwards := findExactCurve(into.Curve[first:], &ex.sb)
			var same bool
			if i < 0 {
				same = ex.match.curve == nil
			} else {
				same = ex.match.own >= 0 && at[ex.match.own] == first+i &&
					backwards == ex.match.backwards
			}
			if !same {
				sinkRedone.Add(1)
				ex.s.AddExactIntersectionCurve(&ex.sb, ex.srfB, ex.agnstA, ex.agnstB, into)
				continue
			}
		}
		if e.added {
			at[j] = len(into.Curve)
			into.addAndAssignCurveID(e.curve)
		}
	}
}

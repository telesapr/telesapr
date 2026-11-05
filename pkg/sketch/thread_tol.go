package sketch

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ridgeRetol tells whether the helix group g, an internal thread's
// ridges swept at the sketch's tolerance, must be swept again at a
// finer one, and returns it. The ridges' foot lies past the bore by as
// little as the gap between the turns at the root lets it
// (threadLayout.baseR): a tenth of a millimetre on a 2-8 BUTT nut, and
// 0.054 of the pitch on any ISO nut. The foot's chords sag towards the
// axis by up to the chord tolerance; one that dips into the bore cuts
// the bored surface twice and leaves the Boolean's trims open (the 2-8
// BUTT nut at 0.2%). Swept at half the foot's overlap, none can. Only
// the threads whose chords do dip are swept again, so the others keep
// their meshes. A screw's foot sags further into the part.
func (g *Group) ridgeRetol(sk *Sketch) (srf.Tolerance, bool) {
	// A further level start's helix keeps no thread of its own: left
	// at the sketch's, as no thread whose foot can dip has level starts
	// (DIN 513's and ISO's nuts keep clear even at 4 mm, and B1.9 is
	// read with one start).
	o := g.Thread.ThreadOptions
	if g.Type != GroupHelix || !g.Thread.IsThread() || !o.Internal || o.Cosmetic {
		return srf.Tolerance{}, false
	}
	l, err := o.layout(0)
	if err != nil {
		return srf.Tolerance{}, false
	}
	into := l.baseR - l.profile.RootRadius
	t := g.ThisShell.Tol
	if !(into > 0) || t.ChordTolMm() <= into/2 {
		return srf.Tolerance{}, false
	}
	pt, axis := g.revolveAxis(sk)
	if !chordsDipInto(&g.ThisShell, pt, axis, l.profile.RootRadius) {
		return srf.Tolerance{}, false
	}
	return srf.Tol(into/2, t.MaxSegments()).WithMesher(t.Mesher()).WithAngle(t.AngleDeg()), true
}

// chordsDipInto reports whether a chord of the shell's curves whose
// ends both lie outside the cylinder of radius r about the axis
// through pt passes inside it.
func chordsDipInto(sh *srf.SShell, pt, axis geom.Vector, r float64) bool {
	n := axis.WithMagnitude(1)
	// off is p's offset from the axis, square to it.
	off := func(p geom.Vector) geom.Vector {
		d := p.Minus(pt)
		return d.Minus(n.ScaledBy(d.Dot(n)))
	}
	for ci := range sh.Curve {
		pts := sh.Curve[ci].Points
		for i := 1; i < len(pts); i++ {
			a, b := off(pts[i-1].P), off(pts[i].P)
			if a.Magnitude() < r || b.Magnitude() < r {
				continue
			}
			ab := b.Minus(a)
			l2 := ab.Dot(ab)
			if l2 == 0 {
				continue
			}
			t := math.Max(0, math.Min(1, -a.Dot(ab)/l2))
			if a.Plus(ab.ScaledBy(t)).Magnitude() < r {
				return true
			}
		}
	}
	return false
}

// threadOwnPitchTol and threadOwnCrestTol bound the chord tolerance a
// thread of its own tolerance (hasOwnTol) is made at, as parts of its
// pitch and of a rounded crest's radius (threadSolidTol). Coarser, the
// Booleans of the chamfer's and the turning's cones with the ridges
// leave trims open where the cones cross the ridges' flanks and crests
// near the end's plane, at every way of making the chamfer
// (chamferTries): ISO, UNC and trapezoidal nuts 16 mm long in parts of
// twice their diameter from about 0.1 of the pitch, buttress nuts from
// 0.07, and a 1-12 BUTT nut's rounded crest (0.13 of the pitch) from
// 0.15 of its radius. A buttress nut's ridges, whose steep flank stands
// almost square to the axis, meet a bore chorded that coarsely in
// trims that don't close even without a chamfer: a 1-12 and a 2-8 BUTT
// nut 20 mm long, flat crest, at 1% (0.25 and 0.31 of the pitch). At
// these, every such nut measured comes out sound at chord tolerances
// up to 1%. A rounded root (RoundRoot) is made at threadOwnRootTol of
// its arcs' radius: coarser, the Booleans fail where the arcs, 0.07 of
// a pitch, cross the turned or bored surface (every B1.9 thread from
// 1/2 to 4 inches and 4 to 20 threads per inch was sound at 0.17 of
// the radius, most at 0.24).
const (
	threadOwnPitchTol = 0.05
	threadOwnCrestTol = 0.1
	threadOwnRootTol  = 0.17
)

// hasOwnTol tells whether a thread made with the options o is made at
// a tolerance of its own where the sketch's is coarser
// (threadSolidTol): a modelled thread with a chamfer or a rounded
// root, or a buttress nut's.
func (o ThreadOptions) hasOwnTol() bool {
	return !o.Cosmetic && (o.Chamfer || o.RoundRoot || o.Internal && o.Spec.Buttress())
}

// threadSolidTol is the tolerance the group g's solid is made at when
// g is one of the groups of a thread of its own tolerance (hasOwnTol)
// and the sketch's tolerance is coarser than the thread can be made at
// (ownTolLimit), and whether it is. Its sketches are made at it too: a
// Boolean takes each surface it copies to its result's tolerance, so
// the part's surfaces reach the thread's turning at the thread's
// tolerance through the sketch before it, as the cones and the ridges
// need them (at the sketch's, the end's plane meets the cones, and the
// bore the ridges, in chords too coarse for the ridges' curves to close
// on). The groups after the thread take the part back to the sketch's.
//
// A rounded root's thread needs the part it is turned or bored in made
// at its tolerance too, every group before it (partTolLimit).
func (sk *Sketch) threadSolidTol(g *Group) (srf.Tolerance, bool) {
	// Most models have no such thread: they are told apart without
	// finding every thread's groups, for every group.
	if !sk.hasThreadOfOwnTol() {
		return srf.Tolerance{}, false
	}
	of := sk.ThreadOfGroups()
	limit := sk.partTolLimit(g, of)
	if helix, ok := of[g.H]; ok {
		if o := sk.Group(helix).Thread.LoadStated(); o.hasOwnTol() {
			limit = finer(limit, o.ownTolLimit())
		}
	}
	t := sk.tol
	if !(limit > 0) || t.ChordTolMm() <= limit {
		return srf.Tolerance{}, false
	}
	return srf.Tol(limit, t.MaxSegments()).WithMesher(t.Mesher()).WithAngle(t.AngleDeg()), true
}

// ownTolLimit is the chord tolerance a thread of its own tolerance
// (hasOwnTol) is made at where the sketch's is coarser: a part of its
// pitch, or of a rounded crest's or root's radius where that is finer
// (threadOwnPitchTol, threadOwnCrestTol, threadOwnRootTol).
func (o ThreadOptions) ownTolLimit() float64 {
	limit := threadOwnPitchTol * o.Spec.Pitch
	if l, err := o.layout(0); err == nil {
		if r := l.profile.CrestRoundRadius; r > 0 {
			limit = min(limit, threadOwnCrestTol*r)
		}
		if r := l.profile.RootRoundRadius; r > 0 {
			limit = min(limit, threadOwnRootTol*r)
		}
	}
	return limit
}

// partTolLimit is the finest tolerance of the threads with rounded
// roots (RoundRoot) whose groups come after g, of the threads of
// (ThreadOfGroups), or 0 for none: g's solid is part of what such a
// thread is turned or bored in, and is made at the thread's tolerance
// (threadSolidTol). Made at a coarser one, the part's edges keep its
// chords through the Booleans after it, which take its surfaces to
// their own tolerance but not its curves, and the ridges' union with
// the turned part left edges open along the root between the turns,
// though every group of the thread was made at the finer tolerance: a
// 3/4-10 BUTT screw at 0.5% had 34, on a rod alone or with a head
// added after it.
func (sk *Sketch) partTolLimit(g *Group, of map[core.HGroup]core.HGroup) float64 {
	i := slices.Index(sk.GroupOrder, g.H)
	if i < 0 {
		return 0
	}
	limit := 0.0
	seen := map[core.HGroup]bool{}
	for _, h := range sk.GroupOrder[i+1:] {
		helix, ok := of[h]
		if !ok || seen[helix] || helix == of[g.H] {
			continue
		}
		seen[helix] = true
		if o := sk.Group(helix).Thread.LoadStated(); o.RoundRoot && o.hasOwnTol() {
			limit = finer(limit, o.ownTolLimit())
		}
	}
	return limit
}

// finer is the finer of two chord tolerances, where 0 is none.
func finer(a, b float64) float64 {
	switch {
	case a <= 0:
		return b
	case b <= 0:
		return a
	}
	return min(a, b)
}

// hasThreadOfOwnTol tells whether a group keeps a modelled thread made
// at a tolerance of its own (hasOwnTol).
func (sk *Sketch) hasThreadOfOwnTol() bool {
	for i := range sk.Groups {
		if t := &sk.Groups[i].Thread; t.IsThread() && t.hasOwnTol() {
			return true
		}
	}
	return false
}

// groupSolidTol is the tolerance g's solid is made at: the sketch's,
// or a finer one for the groups of a thread of its own tolerance
// (threadSolidTol).
func (sk *Sketch) groupSolidTol(g *Group) srf.Tolerance {
	if t, ok := sk.threadSolidTol(g); ok {
		return t
	}
	return sk.tol
}

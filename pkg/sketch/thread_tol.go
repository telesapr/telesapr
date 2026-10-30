package sketch

import (
	"math"

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

package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// An open fillet's end that runs off both its faces, each past the far
// end w of its edge with the flat end face onto the face past it, as a
// plate's foot on a cone whose top is low at the end and whose cone
// past the plate's side is cut flat short of where the fillet reaches:
// the end section, in the end face's plane, crosses both next faces.
// Each side is cut as a run-off alone is (runOffCuts): its contact
// curve back to p2 under its next face, that face's edge with the
// side's face split there. The patches at the end are cut by both next
// faces in turn (clipConsumed), and the end face loses both its edges
// with the sides' faces and takes the end section between the two
// crossings, p1 of each, and from each p1 the edge it shares with that
// next face to its w (closeNext). Made where both next faces fill a
// corner (no fillet taking material away). Where the two cuts cross on
// the fillet, the two next faces meet across it instead
// (rollingrunoff_crossed.go). Both ends of a fillet may run off both
// their faces, each cut by its own next faces, where their cuts lie
// apart along the edge (runOffApart refuses them where they overlap).

// bothRunOffs refuses an end that runs off both its faces where this
// can't follow it, once each side's cuts are found.
func (rb *rollingBall) bothRunOffs(row []srf.SSurface, k int, cut rbEndCut) error {
	for _, ro := range cut.runOffs() {
		if ro.away {
			return filletErr(ro.p1, "the fillet's end runs off both its faces here and takes material away at "+
				"one of them: make it smaller")
		}
	}
	sec := endSection(row, k)
	r0, r1 := cut.offOn(0), cut.offOn(1)
	if r0 == nil || r1 == nil {
		return filletErr(sec.PointAt(0.5), "the fillet's end runs off one of its faces twice here: make it smaller")
	}
	if bezierRoot(&sec, r0.h, 0, 1) >= bezierRoot(&sec, r1.h, 0, 1) {
		// The two cuts cross on the fillet (rollingrunoff_crossed.go).
		return rb.crossedRunOffs(cut, sec.PointAt(0.5))
	}
	return nil
}

// endSection is the fillet's section at end k, from side 0 to side 1.
func endSection(row []srf.SSurface, k int) srf.SBezier {
	if k == 1 {
		return row[len(row)-1].EdgeCurve(2)
	}
	return row[0].EdgeCurve(0)
}

// endSectionPart is the part of end k's section the end face takes:
// from side 0 to side 1, up to where each side's run-off cuts it, at
// its p1.
func endSectionPart(row []srf.SSurface, k int, cut rbEndCut) srf.SBezier {
	sec := endSection(row, k)
	t0, t1 := 0.0, 1.0
	if ro := cut.offOn(0); ro != nil {
		t0 = bezierRoot(&sec, ro.h, 0, 1)
	}
	if ro := cut.offOn(1); ro != nil {
		t1 = bezierRoot(&sec, ro.h, 0, 1)
	}
	part := sec.Sub(t0, t1)
	part.Ctrl[0], part.Ctrl[part.Degree] = sec.Start(), sec.Finish()
	if ro := cut.offOn(0); ro != nil {
		part.Ctrl[0] = ro.p1
	}
	if ro := cut.offOn(1); ro != nil {
		part.Ctrl[part.Degree] = ro.p1
	}
	return part
}

// clipRunOffs cuts the patches of an end that runs off by its next
// faces: the loops of those cut, by patch, and the curves across them
// in each next face, by run-off.
func (rb *rollingBall) clipRunOffs(sh *srf.SShell, row []srf.SSurface, cut rbEndCut) (map[int][]srf.SBezier,
	map[*rbRunOff][]srf.SBezier, error) {
	loops := map[int][]srf.SBezier{}
	across := map[*rbRunOff][]srf.SBezier{}
	tol := math.Max(sh.Tol.ChordTolMm()/10, 1e-6)
	ros := cut.runOffs()
	first, after := len(row), 0
	for _, ro := range ros {
		first, after = min(first, ro.first), max(after, ro.after)
	}
	for b := first; b < after; b++ {
		var on []*rbRunOff
		for _, ro := range ros {
			if b >= ro.first && b < ro.after {
				on = append(on, ro)
			}
		}
		switch len(on) {
		case 0:
			continue
		case 1:
			l, a, err := clipOne(&row[b], on[0], tol)
			if err != nil {
				return nil, nil, err
			}
			loops[b] = l
			across[on[0]] = append(across[on[0]], a...)
		default:
			l, a, err := clipConsumed(&row[b], on[0], on[1], tol)
			if err != nil {
				return nil, nil, err
			}
			if l == nil && on[0].crossed {
				// Past where the cuts cross: cut away whole.
				loops[b] = nil
				continue
			}
			if l == nil {
				return nil, nil, filletErr(row[b].PointAt(0.5, 0.5), "the fillet's end runs off both its faces "+
					"here, and the faces past them cut it away: make it smaller")
			}
			if !on[0].crossed && len(sharedEnds(a[0], a[1])) > 0 {
				return nil, nil, filletErr(row[b].PointAt(0.5, 0.5), "the fillet's end runs off both its faces "+
					"here, and the faces past them meet across it: make it smaller")
			}
			loops[b] = l
			across[on[0]] = append(across[on[0]], a[0]...)
			across[on[1]] = append(across[on[1]], a[1]...)
		}
	}
	if len(ros) == 2 && ros[0].crossed {
		x, err := crossedX(across[ros[0]], across[ros[1]], ros[0].meet)
		if err != nil {
			return nil, nil, err
		}
		ros[0].x, ros[1].x = x, x
	}
	for _, ro := range ros {
		for b := range loops {
			snapEnds(loops[b], ro.p1At, ro.p1)
		}
		for _, r := range ros {
			snapEnds(across[r], ro.p1At, ro.p1)
		}
	}
	return loops, across, nil
}

// clipOne is patch p cut by ro's next face, as clipPatch cuts it.
func clipOne(p *srf.SSurface, ro *rbRunOff, tol float64) ([]srf.SBezier, []srf.SBezier, error) {
	if ro.ns != nil {
		// Across a curved next face, both faces mesh the same cubics but
		// each through its own surface: within a tenth of the chord
		// tolerance of both, their points still parted by 1e-6 there,
		// past the mesh's join (naked edges on a band's end at 20 on the
		// cone, its fillets of 1.725 to 1.8 and 2.3 at 21). Nearer first,
		// where 64 cubics follow it.
		if l, a, err := clipPatch(p, ro, runOffAcrossTol); err == nil {
			return l, a, nil
		}
	}
	return clipPatch(p, ro, tol)
}

// closeRunOffBoth puts an end that runs off both its faces into the end
// face and the two next faces: the end face loses its edges with both
// sides' faces and takes the end section from one p1 to the other, and
// each next face is closed as a run-off alone closes it (closeNext).
func (e *runOffEdit) closeRunOffBoth(cut rbEndCut, endSec srf.SBezier, across map[*rbRunOff][]srf.SBezier) error {
	if cut.off.crossed {
		return e.closeRunOffCrossed(cut, across)
	}
	t := e.rb.t
	v := t.Vertices[cut.end.v].P
	a, b := cut.off, cut.also
	wa, wb := t.Vertices[a.w].P, t.Vertices[b.w].P
	pairs := [][2]geom.Vector{{v, wa}, {v, wb}}
	// The end face's loop runs from w on into p1 where its kept trim
	// there finishes at w.
	got, err := e.put(cut.end.f, pairs, []srf.SBezier{endSec}, a.p1, b.p1, wa)
	if err != nil {
		return err
	}
	for _, ro := range []*rbRunOff{a, b} {
		if err := e.closeNext(cut, ro, across[ro], finishesAt(got, ro.p1)); err != nil {
			return err
		}
	}
	return nil
}

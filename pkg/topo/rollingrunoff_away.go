package topo

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
)

// An end that runs off side j's face where the fillet takes material
// away (a convex edge, as an arched bar's top edge whose side is lower
// at the end than the fillet reaches): the ball rolls inside the solid,
// and its end section, in the end face's plane, crosses the next face
// at p1 on the end face's own edge with that face, between w and the
// edge's far end, not on past w as where the fillet fills a corner.
// The fillet is cut by the next face as there, and keeps what lies
// under it; but the faces round shrink rather than grow. The end face
// loses its corner at w: its edges with side j's face and with the
// next face, up to p1, give way to the end section from side 1-j's cut
// to p1. The next face loses its corner at w too: its edges with side
// j's face, up to p2, and with the end face, up to p1, give way to the
// fillet's cut from p2 to p1. Side j's face is cut back along the
// contact curve to p2 as for any run-off. Nothing grows, so a curved
// next face needs neither carrying on nor its tangent plane: the cut
// lies on the face itself.

// runOffAway marks ro as an end that takes material away, once
// runOffCross has found p1 on the end face's edge with the next face.
func (rb *rollingBall) runOffAway(ro *rbRunOff) error {
	if ro.tangent {
		// The tangent plane was taken for an end section the curved face
		// carried on did not cross: no end that takes material away.
		return filletErr(ro.p1, "the fillet's end runs off the face on side %d here onto a curved face it "+
			"crosses otherwise than a fillet that takes material away does: make it smaller", ro.j+1)
	}
	ro.away = true
	return nil
}

// splitAway splits the end face's edge with the next face at p1, where
// the end of a fillet that takes material away cuts it, before the
// faces' corners at w give way to the fillet's end. The split's curves
// take handles past h, the next one the fillet's end gives a curve, and
// h moves on past them.
func (rb *rollingBall) splitAway(sh *srf.SShell, ro *rbRunOff, h *core.HSCurve) error {
	w := rb.t.Vertices[ro.w].P
	if ro.consumed {
		return filletErr(ro.p1, "the fillet takes the face on side %d whole and takes material away at its end here, "+
			"which it can't yet follow: make it smaller", ro.j+1)
	}
	if ro.p1.Equals(w) {
		return nil
	}
	if err := splitNearest(sh, ro.p1, w); err != nil {
		return filletErr(ro.p1, "the fillet can't cut the end face's edge with the next face here: make it smaller")
	}
	if nh := newCurveHandle(sh); nh.V > h.V {
		*h = nh
	}
	return nil
}

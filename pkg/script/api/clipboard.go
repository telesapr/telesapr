package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The clipboard is the sketch's own, which Edit > Copy and Edit > Paste
// use too: what a script copies the window pastes, and the other way
// round. Copying and pasting are in the active workplane, as in the
// window.

// activeWorkplane fails unless a workplane is active, and returns it.
func activeWorkplane(sk *sketch.Sketch) *core.Entity {
	wp := sk.ActiveWorkplane()
	if core.IsFreeIn3D(wp) {
		fail("copy and paste work in a workplane; sketch in one, or call SetWorkplane")
	}
	return sk.MustEntity(wp)
}

// Copy puts the entities on the clipboard, with the constraints between
// them, as Edit > Copy does with them selected. Workplanes are left out.
func (d *Doc) Copy(entities ...Entity) {
	d.read(func(sk *sketch.Sketch) {
		activeWorkplane(sk)
		sel := make([]sketch.Selection, 0, len(entities))
		for _, e := range entities {
			e.get(sk)
			sel = append(sel, sketch.Selection{Entity: e.H})
		}
		sk.CopySelection(sel)
	})
}

// Paste adds what is on the clipboard to the active group, in the
// active workplane, turned by angle degrees about the workplane's
// origin, scaled by scale and then moved by move, as Edit > Paste
// Transformed does once; a scale of -1 mirrors it. It returns what
// was drawn by pasting, a line for a line, without their points.
func (d *Doc) Paste(move Vec, angle, scale float64) (pasted []Entity) {
	d.change(func(sk *sketch.Sketch) {
		wp := activeWorkplane(sk)
		if len(sk.Clipboard.Requests) == 0 {
			fail("the clipboard is empty; Copy first")
		}
		if scale == 0 {
			fail("a scale of 0 would paste everything onto one point")
		}
		origin := sk.MustEntity(wp.Point[0]).PointGetNum(sk)
		wn := sk.MustEntity(wp.Normal).NormalN(sk)
		theta := angle * math.Pi / 180
		// Turned about the origin: Q*(p - o) + o + t = Q*p + (t + o - Q*o).
		t := move.Plus(origin).Minus(origin.RotatedAroundAxis(wn, theta))
		for _, s := range sk.PasteClipboard(t, theta, scale) {
			// What was drawn, not its points: a line, not its ends.
			if s.Entity.V != 0 && s.Entity.IsFromRequest() && s.Entity == s.Entity.Request().Entity(0) {
				pasted = append(pasted, Entity{d, s.Entity})
			}
		}
	})
	return pasted
}

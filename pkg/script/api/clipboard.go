package api

import (
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

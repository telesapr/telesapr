package api

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Orientation is where the entity's own x and y directions point: for a
// normal, and for the workplane, circle or arc that has one, the unit
// vectors u and v of its turn (its normal points along u cross v).
func (e Entity) Orientation() (u, v Vec) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		hn := sk.NormalOf(ent.H)
		if hn.V == 0 {
			fail("%s has no normal", e.describe(sk))
		}
		q := sk.Entity(hn).NormalGetNum(sk)
		u, v = q.RotationU(), q.RotationV()
	})
	return u, v
}

// SetOrientation turns a normal, or the workplane, circle or arc that
// has one, so that its x and y directions point along u and w, which
// must not be parallel (w is made square to u), as dragging the normal
// in the window does; the solver then moves it as the constraints
// require. Only a normal free in 3D can turn: one in a workplane, or
// copied from another group, is fixed and the call fails.
func (e Entity) SetOrientation(u, w Vec) {
	checkFinite("an orientation", u)
	checkFinite("an orientation", w)
	if u.Magnitude() == 0 || u.Cross(w).Magnitude() == 0 {
		fail("the directions of an orientation must differ")
	}
	u = u.WithMagnitude(1)
	w = w.Minus(u.ScaledBy(w.Dot(u))).WithMagnitude(1)
	e.d.change(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		err := sk.OrientNormal(ent.H, geom.QuaternionFromBasis(u, w))
		switch {
		case errors.Is(err, sketch.ErrNormalLocked):
			fail("%s has a fixed normal: it lies in a workplane or is copied from another group", e.describe(sk))
		case err != nil:
			panic(fmt.Errorf("%s: %w", e.describe(sk), err))
		}
	})
}

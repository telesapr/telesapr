package api

import (
	"errors"
	"fmt"
	"math"

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
	e.d.moving(func(sk *sketch.Sketch) {
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

// Turn turns the part that point e is a point of by degrees about axis,
// through e, as Shift- or Ctrl-dragging the point in the window does.
// The turn is counter-clockwise looking against the axis. A part that
// moves in a mechanism (LinkGroup.SetMoves) turns as its mates let it,
// the parts joined to it with it: a lid on its hinge turns about the
// hinge, by as much of the turn as is about the hinge; one on a driven
// hinge turns its drive with it, within the drive's bounds, as dragging
// it does; and a part with no freedom stays. Any other linked part is
// put where it is turned, then solved. Only a point of a linked part
// can be turned so.
func (e Entity) Turn(axis Vec, degrees float64) {
	checkFinite("an axis", axis)
	if axis.Magnitude() == 0 {
		fail("a turn needs an axis, not the zero vector")
	}
	if math.IsNaN(degrees) || math.IsInf(degrees, 0) {
		fail("a turn of %g degrees", degrees)
	}
	e.d.moving(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		q := geom.QuaternionFromAxisAngle(axis.WithMagnitude(1), degrees*math.Pi/180)
		if !sk.TurnPointBy(ent.H, q) {
			fail("%s isn't a point of a linked part, which a turn turns", e.describe(sk))
		}
	})
}

package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// ErrNormalLocked is what OrientNormal returns for a normal that nothing
// turns: one lying in a workplane, or copied from another group.
var ErrNormalLocked = errors.New("the normal is fixed")

// NormalOf is the normal entity that he stands for: he itself when it is
// a normal, the normal of a workplane, a circle or an arc otherwise. It
// is zero for any other entity, and for one that doesn't exist.
func (sk *Sketch) NormalOf(he core.HEntity) core.HEntity {
	e := sk.Entity(he)
	if e == nil {
		return core.HEntity{}
	}
	switch {
	case e.IsNormal():
		return he
	case e.Type == core.EntityWorkplane, e.Type == core.EntityCircle, e.Type == core.EntityArcOfCircle:
		return e.Normal
	}
	return core.HEntity{}
}

// OrientNormal turns the normal of he (see NormalOf) to q and marks its
// group to be solved again, as dragging the normal in the window does;
// the solver then moves it as the constraints require. The normal of a
// part that moves in a mechanism turns the part along the mechanism's
// freedom instead, as TurnPointBy does, the parts joined to it with it:
// a lid on its hinge turns about the hinge by as much of the turn as
// is about the hinge, one on a driven hinge turns its drive with it,
// and a part with no freedom stays. Only a normal
// with parameters of its own can turn: one free in 3D, or one turned
// from another by a group's own rotation. A normal in a workplane or a
// copy of another group's is fixed, and an unknown quaternion is refused;
// both give an error and change nothing.
func (sk *Sketch) OrientNormal(he core.HEntity, q geom.Quaternion) error {
	hn := sk.NormalOf(he)
	if hn.V == 0 {
		return fmt.Errorf("orienting entity e%08x: it has no normal", he.V)
	}
	n := sk.Entity(hn)
	switch n.Type {
	case core.EntityNormalIn3D, core.EntityNormalNRot:
	default:
		return fmt.Errorf("orienting entity e%08x: %w", he.V, ErrNormalLocked)
	}
	if !(q.Magnitude() > 0) {
		return fmt.Errorf("orienting entity e%08x: the orientation has no size", he.V)
	}
	if sk.turnable(hn) {
		sk.pullNormalTo(hn, q.WithMagnitude(1))
		return nil
	}
	n.NormalForceTo(sk, q.WithMagnitude(1))
	sk.MarkGroupDirtyByEntity(hn)
	return nil
}

package sketch

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// seedAxisAngles turns the part that group hg places to where each of
// its axis-angle constraints (a screw mate's turn) asks, before the
// solve. The constraint's equation holds its point in a half-plane
// through the axis; a solve that starts a quarter turn or more from it
// meets the slope of zero there, or the half-plane opposite, so a turn
// set in one step, as a parameter typed from 0 to 450, would not be
// reached. Turned first about the axis, the part starts at the angle,
// and the solve has only the rest to do. A point placed by a part's
// translation and rotation (a link's) is turned; any other is left to
// the solve.
//
// The groups of a mechanism are seeded together (hgs): a constraint in
// any of them turns the part of any of them its point is on, as a
// driven hinge's, made in the active group, turns a crank placed by a
// group before it; an axis on that same part is left out, as it would
// turn with it.
func (sk *Sketch) seedAxisAngles(hgs []core.HGroup) {
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		// A bound is no turn to start from: it holds only where the
		// turn would cross it, and seeded there it would put the part
		// a bound's width from the turn its dimension holds.
		if !slices.Contains(hgs, c.Group) || c.Type != core.ConstraintAxisAngle || c.Reference || c.Limit != core.LimitNone {
			continue
		}
		p, o, n := sk.Entity(c.PtB), sk.Entity(c.PtA), sk.Entity(c.EntityA)
		if p == nil || o == nil || n == nil || !slices.Contains(hgs, p.Group) || p.Type != core.EntityPointNRotTrans ||
			o.Group == p.Group || n.Group == p.Group {
			continue
		}
		delta := (c.ValA - c.AxisAngleNum(sk)) * math.Pi / 180
		if math.Abs(delta) < 1e-9 {
			continue
		}
		at := o.PointGetNum(sk)
		axis, _, _ := c.AxisFrameNum(sk)
		r := geom.QuaternionFromAxisAngle(axis, delta)
		var t geom.Vector
		var q geom.Quaternion
		vals := []*float64{&t.X, &t.Y, &t.Z, &q.W, &q.X, &q.Y, &q.Z}
		for k, v := range vals {
			*v = sk.MustParam(p.Param[k]).Val
		}
		t = at.Plus(r.Rotate(t.Minus(at)))
		q = r.Times(q)
		for k, v := range vals {
			sk.MustParam(p.Param[k]).Val = *v
		}
	}
}

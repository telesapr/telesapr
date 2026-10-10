package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// AlignSameOrientation turns the normal of a same-orientation
// constraint that the active group can move onto the one it can't, as
// near as the constraint allows, so that the solver starts at the
// answer instead of finding a quarter turn of its own. A normal of an
// earlier group is fixed; when both or neither are free nothing moves.
func (s *Sketch) AlignSameOrientation(c *core.Constraint, active core.HGroup) {
	nfree := s.MustEntity(c.EntityA)
	nref := s.MustEntity(c.EntityB)
	if nref.Group == active {
		nref, nfree = nfree, nref
	}
	if nfree.Group != active || nref.Group == active {
		return
	}
	ru, rv := nref.NormalU(s), nref.NormalV(s)
	fu, fv := nfree.NormalU(s), nfree.NormalV(s)
	if math.Abs(fu.Dot(ru)) < math.Abs(fu.Dot(rv)) {
		// An odd number of quarter turns about the normal satisfies the
		// constraint as well; keep the one nearest.
		ru, rv = rv, ru
	}
	if fu.Dot(ru) > 0 {
		fu = ru
	} else {
		fu = ru.ScaledBy(-1)
	}
	if fv.Dot(rv) > 0 {
		fv = rv
	} else {
		fv = rv.ScaledBy(-1)
	}
	nfree.NormalForceTo(s, geom.QuaternionFromBasis(fu, fv))
}

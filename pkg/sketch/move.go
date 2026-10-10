package sketch

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// MovablePoints are the points that moving entity he moves: the point
// itself, or a curve's, a text's or an image's points; none for what
// has no points of its own to move, as a normal or a workplane.
func (sk *Sketch) MovablePoints(he core.HEntity) []core.HEntity {
	e := sk.Entity(he)
	if e == nil {
		return nil
	}
	if e.IsPoint() {
		return []core.HEntity{he}
	}
	switch e.Type {
	case core.EntityLineSegment, core.EntityArcOfCircle, core.EntityCubic, core.EntityCubicPeriodic,
		core.EntityCircle, core.EntityTTFText, core.EntityImage:
		n, ok := EntityPointCount(e.Type, e.ExtraPoints)
		if !ok {
			return nil
		}
		return slices.Clone(e.Point[:n])
	}
	return nil
}

// MovePointsBy moves each point by d, as a drag would put it, and
// marks their groups to be solved again; a point named twice moves
// once. The solve then keeps the moved points as near where they were
// put as the constraints let it, given them as the dragged ones.
func (sk *Sketch) MovePointsBy(points []core.HEntity, d geom.Vector) {
	var done []core.HEntity
	for _, hp := range points {
		if slices.Contains(done, hp) {
			continue
		}
		done = append(done, hp)
		p := sk.Entity(hp)
		p.PointForceTo(sk, p.PointGetNum(sk).Plus(d))
		sk.MarkGroupDirtyByEntity(hp)
	}
}

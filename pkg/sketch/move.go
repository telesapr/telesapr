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
		sk.MovePointTo(hp, sk.Entity(hp).PointGetNum(sk).Plus(d))
	}
}

// MovePointTo puts point he at p, as dragging it there does, and marks
// what it moves to be solved again. A point of a part that moves in a
// mechanism is pulled along the mechanism's freedom (pullPoint): the
// part turns and slides as its joints let it, and the parts joined to
// it move with it. Any other point is put at p (PointForceTo), and the
// solve then moves it as its constraints require, following it there in
// steps where the move is large (continuation.go).
func (sk *Sketch) MovePointTo(he core.HEntity, p geom.Vector) {
	if sk.pullable(he) {
		sk.pullPoint(he, p)
		return
	}
	e := sk.Entity(he)
	sk.noteMove(he, e.PointGetNum(sk))
	e.PointForceTo(sk, p)
	sk.MarkGroupDirtyByEntity(he)
}

// PlacePointTo puts point he at p as the tool drawing it does, while the
// point is placed: as MovePointTo, but its solve is not followed in
// steps nor checked for corners turned over (continuation.go), since what
// is being drawn has no shape yet to keep, and its points are put down
// far from where they were made in one move.
func (sk *Sketch) PlacePointTo(he core.HEntity, p geom.Vector) {
	sk.MovePointTo(he, p)
	sk.moves = slices.DeleteFunc(sk.moves, func(m pointMove) bool { return m.p == he })
}

package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// nudgeSelection moves the selected points, and the points of the
// selected curves, texts and images, by a pixel of the view in the
// arrow's direction on the screen, ten with Shift: one step of undo
// each, solved as a drag is, with the points held as near where they
// were put as the constraints let them. What can't be dragged stays.
// It reports whether there was anything to move.
func (gw *graphicsWindow) nudgeSelection(arrow int, shift bool) bool {
	ss := gw.ss
	sk := gw.sk()
	var points []core.HEntity
	for _, s := range gw.selection {
		if s.Entity.V == 0 {
			continue
		}
		for _, hp := range sk.MovablePoints(s.Entity) {
			if p := sk.Entity(hp); p != nil && gw.entityCanBeDragged(p) {
				points = append(points, hp)
			}
		}
	}
	if len(points) == 0 {
		return false
	}
	v := &gw.view
	step := 1 / v.Scale
	if shift {
		step *= 10
	}
	var d geom.Vector
	switch arrow {
	case platform.ArrowLeft:
		d = v.ProjRight.ScaledBy(-step)
	case platform.ArrowRight:
		d = v.ProjRight.ScaledBy(step)
	case platform.ArrowUp:
		d = v.ProjUp.ScaledBy(step)
	case platform.ArrowDown:
		d = v.ProjUp.ScaledBy(-step)
	default:
		return false
	}
	ss.UndoRemember()
	sk.MovePointsBy(points, d)
	gw.pending.points = points
	ss.GenerateAll(sketch.GenerateDirty)
	gw.pending.points = nil
	ss.ScheduleShowTW()
	gw.Invalidate(false)
	return true
}

package ui

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// UpdateDraggedPoint moves a point with the mouse.
func (gw *graphicsWindow) UpdateDraggedPoint(hp core.HEntity, mx, my float64) {
	sk := gw.sk()
	p := sk.Entity(hp)
	pos := p.PointGetNum(sk)
	gw.UpdateDraggedNum(&pos, mx, my)
	p.PointForceTo(sk, pos)
	gw.ss.ScheduleShowTW()
}

// UpdateDraggedNum moves pos by the mouse's movement.
func (gw *graphicsWindow) UpdateDraggedNum(pos *geom.Vector, mx, my float64) {
	v := &gw.view
	*pos = pos.Plus(v.ProjRight.ScaledBy((mx - gw.orig.mouse.U) / v.Scale))
	*pos = pos.Plus(v.ProjUp.ScaledBy((my - gw.orig.mouse.V) / v.Scale))
}

// AddPointToDraggedList drags a point too, unless it's there already,
// or is a transformed point of the same group, which moves with it.
func (gw *graphicsWindow) AddPointToDraggedList(hp core.HEntity) {
	sk := gw.sk()
	p := sk.Entity(hp)
	for _, hee := range gw.pending.points {
		if hee == hp {
			// Exact same point.
			return
		}
		pe := sk.Entity(hee)
		if pe.Type == p.Type && pe.Type != core.EntityPointIn2D && pe.Type != core.EntityPointIn3D &&
			pe.Group == p.Group {
			// Transform-type point, from the same group. So it handles
			// the same unknowns.
			return
		}
	}
	gw.pending.points = append(gw.pending.points, hp)
}

// StartDraggingByEntity drags an entity's points.
func (gw *graphicsWindow) StartDraggingByEntity(he core.HEntity) {
	sk := gw.sk()
	e := sk.Entity(he)
	switch {
	case e.IsPoint():
		gw.AddPointToDraggedList(e.H)
	case e.Type == core.EntityLineSegment || e.Type == core.EntityArcOfCircle ||
		e.Type == core.EntityCubic || e.Type == core.EntityCubicPeriodic ||
		e.Type == core.EntityCircle || e.Type == core.EntityTTFText || e.Type == core.EntityImage:
		pts, ok := sketch.EntityPointCount(e.Type, e.ExtraPoints)
		if !ok {
			panic("ui: no entity info")
		}
		for i := range pts {
			gw.AddPointToDraggedList(e.Point[i])
		}
	case e.Type == core.EntityFaceNormalPt || e.Type == core.EntityFaceNTrans ||
		e.Type == core.EntityFaceNRotTrans:
		// Rotate groups are unstable dragging; revolve and helix faces
		// too, since their point is on the axis. Files saved before v3.2
		// didn't specify point[0], so check it exists.
		if pt := sk.Entity(e.Point[0]); pt != nil {
			if gw.entityCanBeDragged(pt) {
				gw.AddPointToDraggedList(e.Point[0])
			}
		}
	}
}

// StartDraggingBySelection drags the selection and the hovered item.
func (gw *graphicsWindow) StartDraggingBySelection() {
	for _, s := range gw.selection {
		if s.Entity.V == 0 {
			continue
		}
		gw.StartDraggingByEntity(s.Entity)
	}
	// The user might select a point, and then click it again to start
	// dragging; but the point just got unselected by that click. So drag
	// the hovered item too, and they'll always have it.
	if gw.hover.Entity.V != 0 {
		if dragEntity := gw.ChooseFromHoverToDrag().Entity; dragEntity.V != 0 {
			gw.StartDraggingByEntity(dragEntity)
		}
	}
}

// MouseMoved handles the mouse moving to x, y (screen pixels from the
// centre, y up).
func (gw *graphicsWindow) MouseMoved(x, y float64, leftDown, middleDown, rightDown, shiftDown, ctrlDown bool) {
	ss := gw.ss
	sk := gw.sk()
	v := &gw.view
	if gw.window.IsEditorVisible() || gw.context.active {
		return
	}

	ss.GW.extraLine.Draw = false

	if !gw.orig.mouseDown {
		// If someone drags the mouse into our window with the left button
		// already depressed, then we don't have our starting point; so
		// don't try.
		leftDown = false
	}

	if rightDown {
		middleDown = true
		shiftDown = !shiftDown
	}

	// Not passing right-button and middle-button drags to the toolbar
	// avoids spurious hover events from trackpad pans.
	if ss.ShowToolbar && !middleDown {
		if gw.ToolbarMouseMoved(int(x), int(y)) {
			gw.hover.Clear()
			return
		}
	}

	if !leftDown && (gw.pending.operation == pendingDraggingPoints ||
		gw.pending.operation == pendingDraggingMarquee) {
		gw.ClearPending(true)
		gw.Invalidate(false)
	}

	mp := geom.Point2d{U: x, V: y}
	gw.currentMousePosition = mp

	if rightDown && gw.orig.mouse.DistanceTo(mp) < 5 && !gw.orig.startedMoving {
		// Avoid accidentally panning (or rotating if shift is down) if
		// the user wants a context menu.
		return
	}
	gw.orig.startedMoving = true

	// If the middle button is down, then mouse movement is used to pan
	// and rotate our view. This wins over everything else.
	if middleDown {
		gw.hover.Clear()

		dx := (x - gw.orig.mouse.U) / v.Scale
		dy := (y - gw.orig.mouse.V) / v.Scale

		switch {
		case !shiftDown && !ctrlDown:
			sign := 1.0
			if ss.CameraNav {
				sign = -1.0
			}
			s := 0.3 * (geom.Pi / 180) * v.Scale * sign // degrees per pixel
			if ss.TurntableNav {                        // lock the Z to vertical
				v.ProjRight = gw.orig.projRight.RotatedAroundAxis(geom.Vector{Z: 1}, -s*dx)
				v.ProjUp = gw.orig.projUp.RotatedAroundAxis(
					geom.Vector{X: gw.orig.projRight.X, Y: gw.orig.projRight.Y, Z: gw.orig.projRight.Y}, s*dy)
			} else {
				v.ProjRight = gw.orig.projRight.RotatedAroundAxis(gw.orig.projUp, -s*dx)
				v.ProjUp = gw.orig.projUp.RotatedAroundAxis(gw.orig.projRight, s*dy)
			}
			gw.NormalizeProjectionVectors()
		case ctrlDown:
			theta := libm.Atan2(gw.orig.mouse.V, gw.orig.mouse.U)
			theta -= libm.Atan2(y, x)
			ss.GW.extraLine.Draw = true
			ss.GW.extraLine.PtA = gw.UnProjectPoint(geom.Point2d{})
			ss.GW.extraLine.PtB = gw.UnProjectPoint(mp)

			normal := gw.orig.projRight.Cross(gw.orig.projUp)
			v.ProjRight = gw.orig.projRight.RotatedAroundAxis(normal, theta)
			v.ProjUp = gw.orig.projUp.RotatedAroundAxis(normal, theta)

			gw.NormalizeProjectionVectors()
		default:
			v.Offset.X = gw.orig.offset.X + dx*v.ProjRight.X + dy*v.ProjUp.X
			v.Offset.Y = gw.orig.offset.Y + dx*v.ProjRight.Y + dy*v.ProjUp.Y
			v.Offset.Z = gw.orig.offset.Z + dx*v.ProjRight.Z + dy*v.ProjUp.Z
		}

		gw.orig.projRight = v.ProjRight
		gw.orig.projUp = v.ProjUp
		gw.orig.offset = v.Offset
		gw.orig.mouse = geom.Point2d{U: x, V: y}

		if ss.TW.shown.screen == screenEditView {
			if gw.havePainted {
				ss.ScheduleShowTW()
			}
		}
		gw.Invalidate(false)
		gw.havePainted = false
		return
	}

	if gw.pending.operation == pendingNone {
		dm := gw.orig.mouse.DistanceTo(mp)
		// If we're currently not doing anything, then see if we should
		// start dragging something.
		if leftDown && dm > 3 {
			var e *core.Entity
			dragEntity := gw.ChooseFromHoverToDrag().Entity
			if dragEntity.V != 0 {
				e = sk.Entity(dragEntity)
			}
			if e != nil && e.Type != core.EntityWorkplane {
				if !gw.hoverWasSelectedOnMousedown {
					// The user clicked an unselected entity, which means
					// they're dragging just the hovered thing, not the full
					// selection. So clear all the selection except that
					// entity.
					gw.ClearSelection()
					gw.MakeSelectedEntity(dragEntity)
				}
				switch {
				case e.Type == core.EntityCircle && len(gw.selection) <= 1:
					// Drag the radius.
					gw.pending.circle = dragEntity
					gw.pending.operation = pendingDraggingRadius
				case e.IsNormal():
					gw.pending.normal = dragEntity
					gw.pending.operation = pendingDraggingNormal
				default:
					gw.StartDraggingBySelection()
					gw.hover.Clear()
					gw.pending.operation = pendingDraggingPoints
				}
			} else if gw.hover.Constraint.V != 0 && sk.Constraint(gw.hover.Constraint).HasLabel() {
				gw.ClearSelection()
				gw.pending.constraint = gw.hover.Constraint
				gw.pending.operation = pendingDraggingConstraint
			}
			if gw.pending.operation != pendingNone {
				// We just started a drag, so remember for the undo before
				// the drag changes anything.
				ss.UndoRemember()
				if gw.draggingGeometry() {
					// A job from an earlier edit would be made for a
					// state that is past.
					ss.stopShellJob()
				}
			} else if gw.hover.Constraint.V == 0 {
				// That's just marquee selection, which should not cause an
				// undo remember.
				if dm > 10 {
					if gw.hover.Entity.V != 0 {
						// Avoid accidentally selecting workplanes when
						// starting drags.
						gw.MakeUnselectedEntity(gw.hover.Entity, false)
						gw.hover.Clear()
					}
					gw.pending.operation = pendingDraggingMarquee
					gw.orig.marqueePoint = gw.UnProjectPoint(gw.orig.mouseOnButtonDown)
				}
			}
		} else if !leftDown {
			// Otherwise, just hit test and give up; but don't hit test if
			// the mouse is down, because then the user could hover a point,
			// mouse down (thus selecting it), and drag, in an effort to drag
			// the point, but instead hover a different entity before we move
			// far enough to start the drag.
			//
			// Hit testing can potentially take a lot of time. If we haven't
			// painted since last time we highlighted something, don't hit
			// test again, since this just causes a lag.
			if !gw.havePainted {
				return
			}
			gw.HitTestMakeSelection(mp)
		}
		return
	}

	// If the user has started an operation from the menu, but not
	// completed it, then just do the selection.
	if gw.pending.operation == pendingCommand {
		gw.HitTestMakeSelection(mp)
		return
	}

	if gw.pending.operation == pendingDraggingPoints && ctrlDown {
		ss.GW.extraLine.PtA = gw.UnProjectPoint(gw.orig.mouseOnButtonDown)
		ss.GW.extraLine.PtB = gw.UnProjectPoint(mp)
		ss.GW.extraLine.Draw = true
	}

	// We're currently dragging something; so do that. But if we haven't
	// painted since the last time we solved, do nothing, because there's
	// no sense solving a frame and not displaying it.
	if !gw.havePainted {
		return
	}

	gw.havePainted = false
	switch gw.pending.operation {
	case pendingDraggingConstraint:
		gw.dragConstraint(x, y)
		return

	case pendingDraggingNewLinePoint, pendingDraggingNewPoint:
		gw.dragNewPoint(x, y, ctrlDown)

	case pendingDraggingPoints:
		gw.dragPoints(x, y, shiftDown, ctrlDown)

	case pendingDraggingNewCubicPoint:
		gw.dragNewCubicPoint(x, y)

	case pendingDraggingNewArcPoint:
		gw.dragNewArcPoint(x, y, ctrlDown)

	case pendingDraggingNewRadius, pendingDraggingRadius:
		gw.dragRadius(x, y, ctrlDown)

	case pendingDraggingNormal:
		gw.dragNormal(x, y, ctrlDown)

	case pendingDraggingMarquee:
		gw.dragMarquee(x, y)
		return

	default:
		panic("ui: unexpected pending operation")
	}
}

// dragPoints moves what a pendingDraggingPoints drag holds to the mouse.
func (gw *graphicsWindow) dragPoints(x, y float64, shiftDown, ctrlDown bool) {
	ss := gw.ss
	sk := gw.sk()
	v := &gw.view
	mp := geom.Point2d{U: x, V: y}

	if shiftDown || ctrlDown {
		// Edit the rotation associated with a POINT_N_ROT_TRANS,
		// either within (ctrlDown) or out of (shiftDown) the plane of
		// the screen. So first get the rotation to apply, in qt.
		var qt geom.Quaternion
		if ctrlDown {
			d := mp.DistanceTo(gw.orig.mouseOnButtonDown)
			if d < 25 {
				// Don't start dragging the position about the normal
				// until we're a little ways out, to get a reasonable
				// reference pos
				qt = geom.Quaternion{W: 1}
			} else {
				ob := gw.orig.mouseOnButtonDown
				theta := libm.Atan2(gw.orig.mouse.V-ob.V, gw.orig.mouse.U-ob.U)
				theta -= libm.Atan2(y-ob.V, x-ob.U)
				gn := v.ProjRight.Cross(v.ProjUp)
				qt = geom.QuaternionFromAxisAngle(gn, -theta)
			}
		} else {
			dx := -(x - gw.orig.mouse.U)
			dy := -(y - gw.orig.mouse.V)
			s := 0.3 * (geom.Pi / 180) // degrees per pixel
			qt = geom.QuaternionFromAxisAngle(v.ProjUp, -s*dx).Times(
				geom.QuaternionFromAxisAngle(v.ProjRight, s*dy))
		}

		// Now apply this rotation to the points being dragged.
		for _, he := range gw.pending.points {
			e := sk.Entity(he)
			if e.Type != core.EntityPointNRotTrans {
				if ctrlDown {
					p := e.PointGetNum(sk)
					p = p.Minus(ss.GW.extraLine.PtA)
					p = qt.Rotate(p)
					p = p.Plus(ss.GW.extraLine.PtA)
					e.PointForceTo(sk, p)
				} else {
					gw.UpdateDraggedPoint(he, x, y)
				}
			} else {
				q := e.PointGetQuaternion(sk)
				p := e.PointGetNum(sk)
				q = qt.Times(q)
				e.PointForceQuaternionTo(sk, q)
				// Let's rotate about the selected point; so fix up the
				// translation so that that point didn't move.
				e.PointForceTo(sk, p)
			}
			sk.MarkGroupDirtyByEntity(e.H)
		}
	} else {
		for _, he := range gw.pending.points {
			gw.UpdateDraggedPoint(he, x, y)
			sk.MarkGroupDirtyByEntity(he)
		}
	}
	gw.orig.mouse = mp
}

// dragMarquee moves what a pendingDraggingMarquee drag holds to the mouse.
func (gw *graphicsWindow) dragMarquee(x, y float64) {
	mp := geom.Point2d{U: x, V: y}

	gw.orig.mouse = mp
	gw.Invalidate(false)
}

// dragConstraint moves what a pendingDraggingConstraint drag holds to the mouse.
func (gw *graphicsWindow) dragConstraint(x, y float64) {
	sk := gw.sk()
	mp := geom.Point2d{U: x, V: y}

	c := sk.Constraint(gw.pending.constraint)
	gw.UpdateDraggedNum(&c.Disp.Offset, x, y)
	gw.orig.mouse = mp
	gw.Invalidate(false)
}

// dragNewPoint moves what a pendingDraggingNewLinePoint, pendingDraggingNewPoint drag holds to the mouse.
func (gw *graphicsWindow) dragNewPoint(x, y float64, ctrlDown bool) {
	sk := gw.sk()
	mp := geom.Point2d{U: x, V: y}

	if gw.pending.operation == pendingDraggingNewLinePoint {
		if !ctrlDown {
			gw.pending.suggestion, gw.pending.hasSuggestion = gw.SuggestConstraint(gw.pending.request)
		} else {
			gw.pending.hasSuggestion = false
		}
	}
	gw.UpdateDraggedPoint(gw.pending.point, x, y)
	gw.HitTestMakeSelection(mp)
	sk.MarkGroupDirtyByEntity(gw.pending.point)
	gw.orig.mouse = mp
}

// dragNewCubicPoint moves what a pendingDraggingNewCubicPoint drag holds to the mouse.
func (gw *graphicsWindow) dragNewCubicPoint(x, y float64) {
	sk := gw.sk()
	mp := geom.Point2d{U: x, V: y}

	gw.UpdateDraggedPoint(gw.pending.point, x, y)
	gw.HitTestMakeSelection(mp)

	hr := gw.pending.point.Request()
	if gw.pending.point == hr.Entity(4) {
		// The very first segment; dragging final point drags both
		// tangent points.
		p0 := sk.Entity(hr.Entity(1)).PointGetNum(sk)
		p3 := sk.Entity(hr.Entity(4)).PointGetNum(sk)
		p1 := p0.ScaledBy(2.0 / 3).Plus(p3.ScaledBy(1.0 / 3))
		p2 := p0.ScaledBy(1.0 / 3).Plus(p3.ScaledBy(2.0 / 3))
		sk.Entity(hr.Entity(1+1)).PointForceTo(sk, p1)
		sk.Entity(hr.Entity(1+2)).PointForceTo(sk, p2)
	} else {
		// A subsequent segment; dragging point drags only final
		// tangent point.
		i := sk.Entity(hr.Entity(0)).ExtraPoints
		pn := sk.Entity(hr.Entity(4 + i)).PointGetNum(sk)
		pnm2 := sk.Entity(hr.Entity(2 + i)).PointGetNum(sk)
		pnm1 := pn.Plus(pnm2).ScaledBy(0.5)
		sk.Entity(hr.Entity(3+i)).PointForceTo(sk, pnm1)
	}

	gw.orig.mouse = mp
	sk.MarkGroupDirtyByEntity(gw.pending.point)
}

// dragNewArcPoint moves what a pendingDraggingNewArcPoint drag holds to the mouse.
func (gw *graphicsWindow) dragNewArcPoint(x, y float64, ctrlDown bool) {
	sk := gw.sk()
	mp := geom.Point2d{U: x, V: y}

	gw.UpdateDraggedPoint(gw.pending.point, x, y)
	gw.HitTestMakeSelection(mp)

	hr := gw.pending.point.Request()
	ona := sk.Entity(hr.Entity(2)).PointGetNum(sk)
	onb := sk.Entity(hr.Entity(3)).PointGetNum(sk)
	center := ona.Plus(onb).ScaledBy(0.5)

	sk.Entity(hr.Entity(1)).PointForceTo(sk, center)
	gw.suggestFor(hr, ctrlDown)

	gw.orig.mouse = mp
	sk.MarkGroupDirtyByEntity(gw.pending.point)
}

// dragRadius moves what a pendingDraggingNewRadius, pendingDraggingRadius drag holds to the mouse.
func (gw *graphicsWindow) dragRadius(x, y float64, ctrlDown bool) {
	ss := gw.ss
	sk := gw.sk()
	v := &gw.view
	mp := geom.Point2d{U: x, V: y}

	circle := sk.Entity(gw.pending.circle)
	center := sk.Entity(circle.Point[0]).PointGetNum(sk)
	c2 := gw.ProjectPoint(center)
	r := c2.DistanceTo(mp) / v.Scale
	sk.Entity(circle.Distance).DistanceForceTo(sk, r)
	if gw.pending.operation == pendingDraggingNewRadius {
		gw.suggestFor(gw.pending.circle.Request(), ctrlDown)
	}

	sk.MarkGroupDirtyByEntity(gw.pending.circle)
	ss.ScheduleShowTW()
}

// dragNormal moves what a pendingDraggingNormal drag holds to the mouse.
func (gw *graphicsWindow) dragNormal(x, y float64, ctrlDown bool) {
	sk := gw.sk()
	v := &gw.view
	mp := geom.Point2d{U: x, V: y}

	normal := sk.Entity(gw.pending.normal)
	p := sk.Entity(normal.Point[0]).PointGetNum(sk)
	p2 := gw.ProjectPoint(p)

	q := normal.NormalGetNum(sk)
	u, vv := q.RotationU(), q.RotationV()

	if ctrlDown {
		theta := libm.Atan2(gw.orig.mouse.V-p2.V, gw.orig.mouse.U-p2.U)
		theta -= libm.Atan2(y-p2.V, x-p2.U)

		n := v.ProjRight.Cross(v.ProjUp)
		u = u.RotatedAroundAxis(n, -theta)
		vv = vv.RotatedAroundAxis(n, -theta)
	} else {
		dx := -(x - gw.orig.mouse.U)
		dy := -(y - gw.orig.mouse.V)
		s := 0.3 * (geom.Pi / 180) // degrees per pixel
		u = u.RotatedAroundAxis(v.ProjUp, -s*dx)
		u = u.RotatedAroundAxis(v.ProjRight, s*dy)
		vv = vv.RotatedAroundAxis(v.ProjUp, -s*dx)
		vv = vv.RotatedAroundAxis(v.ProjRight, s*dy)
	}
	gw.orig.mouse = mp
	normal.NormalForceTo(sk, geom.QuaternionFromBasis(u, vv))

	sk.MarkGroupDirtyByEntity(gw.pending.normal)
}

// ClearPending cancels the operation in progress.
func (gw *graphicsWindow) ClearPending(scheduleShowTW bool) {
	if gw.draggingGeometry() {
		gw.endGeometryDrag()
	}
	gw.pending = pendingState{}
	if scheduleShowTW {
		gw.ss.ScheduleShowTW()
	}
}

// draggingGeometry reports whether the operation in progress moves the
// sketch's geometry, and so has to be regenerated on this thread: a
// shell job takes a copy of the sketch and gives its solids back only if
// the sketch is defined the same way as when it was made, and a drag
// changes the sketch on every frame, so the job's work would be taken
// back and the solid would never follow the pointer.
func (gw *graphicsWindow) draggingGeometry() bool {
	switch gw.pending.operation {
	case pendingDraggingPoints, pendingDraggingRadius, pendingDraggingNormal:
		return true
	}
	return false
}

// endGeometryDrag is called when such a drag ends. The regeneration its
// last frame scheduled, which has not been made yet, is made here, so
// that the solids are the ones the drag ended with: while the drag lasts
// they are made on this thread, and the frame after it would be made in
// a job, a step behind the sketch.
func (gw *graphicsWindow) endGeometryDrag() {
	if gw.ss.scheduledGenerateAll {
		gw.ss.Refresh()
	}
}

// IsFromPending reports whether request r is being drawn.
func (gw *graphicsWindow) IsFromPending(r core.HRequest) bool {
	return slices.Contains(gw.pending.requests, r)
}

// AddToPending marks r as being drawn.
func (gw *graphicsWindow) AddToPending(r core.HRequest) {
	gw.pending.requests = append(gw.pending.requests, r)
}

// ReplacePending swaps a request being drawn for another.
func (gw *graphicsWindow) ReplacePending(before, after core.HRequest) {
	for i, req := range gw.pending.requests {
		if req == before {
			gw.pending.requests[i] = after
		}
	}
}

// MouseMiddleOrRightDown starts a pan or rotation.
func (gw *graphicsWindow) MouseMiddleOrRightDown(x, y float64) {
	if gw.window.IsEditorVisible() {
		return
	}
	gw.orig.offset = gw.view.Offset
	gw.orig.projUp = gw.view.ProjUp
	gw.orig.projRight = gw.view.ProjRight
	gw.orig.mouse = geom.Point2d{U: x, V: y}
	gw.orig.startedMoving = false
}

// MouseRightUp stops drawing, or shows the context menu.
func (gw *graphicsWindow) MouseRightUp(x, y float64) {
	ss := gw.ss
	ss.GW.extraLine.Draw = false
	gw.Invalidate(false)

	// Don't show a context menu if the user is right-clicking the
	// toolbar, or if they are finishing a pan.
	if gw.ToolbarMouseMoved(int(x), int(y)) {
		return
	}
	if gw.orig.startedMoving || gw.context.active {
		return
	}

	switch gw.pending.operation {
	case pendingDraggingNewLinePoint, pendingDraggingNewArcPoint, pendingDraggingNewRadius:
		if gw.pending.hasSuggestion {
			gw.applySuggestion(gw.pending.suggestion)
		}
	}

	switch gw.pending.operation {
	case pendingDraggingNewLinePoint, pendingDraggingNewCubicPoint, pendingDraggingNewArcPoint,
		pendingDraggingNewRadius, pendingDraggingNewPoint:
		// Special case; use a right click to stop drawing lines, since a
		// left click would draw another one. This is quicker and more
		// intuitive than hitting escape. Likewise for other entities for
		// consistency.
		gw.ClearPending(true)
		return
	}

	gw.showContextMenu(x, y)
}

// AddRequest adds a request in the active group and workplane.
func (gw *graphicsWindow) AddRequest(typ sketch.RequestType, rememberForUndo bool) core.HRequest {
	if rememberForUndo {
		gw.ss.UndoRemember()
	}
	return gw.sk().AddRequestOfType(typ, false)
}

// SnapToEntityByScreenPoint is the point of entity he nearest the
// screen point pp.
func (gw *graphicsWindow) SnapToEntityByScreenPoint(pp geom.Point2d, he core.HEntity) geom.Vector {
	sk := gw.sk()
	e := sk.Entity(he)
	if e.IsPoint() {
		return e.PointGetNum(sk)
	}
	edges := gw.getOrGenerateEdges(e)

	minD := -1.0
	k := 0.0
	found := -1
	for i, ed := range edges.Edges {
		p0 := gw.ProjectPoint(ed.A)
		p1 := gw.ProjectPoint(ed.B)
		dir := p1.Minus(p0)
		d := pp.DistanceToLine(p0, dir, true)
		if minD > 0.0 && d > minD {
			continue
		}
		minD = d
		k = pp.Minus(p0).Dot(dir) / dir.Dot(dir)
		found = i
	}
	if found < 0 {
		return gw.UnProjectPoint(pp)
	}
	edge := edges.Edges[found]
	return edge.A.Plus(edge.B.Minus(edge.A).ScaledBy(k))
}

// ConstrainPointByHovered constrains pt onto the hovered point, circle
// or line, if any.
func (gw *graphicsWindow) ConstrainPointByHovered(pt core.HEntity, projected *geom.Point2d) bool {
	if gw.hover.Entity.V == 0 {
		return false
	}
	sk := gw.sk()
	point := sk.Entity(pt)
	e := sk.Entity(gw.hover.Entity)
	switch {
	case e.IsPoint():
		point.PointForceTo(sk, e.PointGetNum(sk))
		gw.ss.constrainCoincident(e.H, pt)
		return true
	case e.IsCircle():
		if projected != nil {
			point.PointForceTo(sk, gw.SnapToEntityByScreenPoint(*projected, e.H))
		}
		gw.ss.constrain(core.ConstraintPtOnCircle, pt, core.HEntity{}, e.H)
		return true
	case e.Type == core.EntityLineSegment:
		if projected != nil {
			point.PointForceTo(sk, gw.SnapToEntityByScreenPoint(*projected, e.H))
		}
		gw.ss.constrain(core.ConstraintPtOnLine, pt, core.HEntity{}, e.H)
		return true
	}
	return false
}

// MouseEvent dispatches a mouse event from the window, in window
// coordinates, as screen pixels from the centre with y up.
func (gw *graphicsWindow) MouseEvent(event platform.MouseEvent) bool {
	width, height := gw.window.ContentSize()
	x := event.X - width/2
	y := height/2 - event.Y

	switch event.Type {
	case platform.MouseMotion:
		gw.MouseMoved(x, y, event.Button == platform.ButtonLeft, event.Button == platform.ButtonMiddle,
			event.Button == platform.ButtonRight, event.ShiftDown, event.ControlDown)
	case platform.MousePress:
		switch event.Button {
		case platform.ButtonLeft:
			gw.MouseLeftDown(x, y, event.ShiftDown, event.ControlDown)
		case platform.ButtonMiddle, platform.ButtonRight:
			gw.MouseMiddleOrRightDown(x, y)
		}
	case platform.MouseDblPress:
		if event.Button == platform.ButtonLeft {
			gw.MouseLeftDoubleClick(x, y)
		}
	case platform.MouseRelease:
		switch event.Button {
		case platform.ButtonLeft:
			gw.MouseLeftUp(x, y, event.ShiftDown, event.ControlDown)
		case platform.ButtonRight:
			gw.MouseRightUp(x, y)
		}
	case platform.MouseScrollVert:
		delta := event.ScrollDelta
		if event.ShiftDown {
			delta /= 10
		}
		gw.MouseScroll(delta)
	case platform.MouseLeave:
		gw.MouseLeave()
	}
	return true
}

// MouseLeftDown handles a left click: the pending command, or a
// selection.
func (gw *graphicsWindow) MouseLeftDown(mx, my float64, shiftDown, ctrlDown bool) {
	ss := gw.ss
	sk := gw.sk()
	gw.orig.mouseDown = true

	if gw.window.IsEditorVisible() {
		gw.orig.mouse = geom.Point2d{U: mx, V: my}
		gw.orig.mouseOnButtonDown = gw.orig.mouse
		gw.window.HideEditor()
		return
	}
	ss.TW.HideEditControl()

	if ss.ShowToolbar {
		if gw.ToolbarMouseDown(int(mx), int(my)) {
			return
		}
	}
	// Nothing in the model is picked or drawn on while the drawing
	// sheet is shown in its place.
	if gw.sheetShown {
		return
	}

	// The suggestion made while dragging is read before MouseMoved
	// below replaces it, and handed to the click that applies it.
	hasConstraintSuggestion := gw.pending.hasSuggestion
	constraintSuggestion := gw.pending.suggestion

	// Make sure the hover is up to date.
	gw.MouseMoved(mx, my, false, false, false, false, false)
	gw.orig.mouse = geom.Point2d{U: mx, V: my}
	gw.orig.mouseOnButtonDown = gw.orig.mouse

	var hr core.HRequest
	var hc core.HConstraint
	switch gw.pending.operation {
	case pendingCommand:
		hr, hc = gw.clickForCommand(mx, my)

	case pendingDraggingRadius:
		gw.clickEndsRadius()

	case pendingDraggingNewPoint, pendingDraggingNewArcPoint:
		gw.clickPlacesPoint(mx, my, constraintSuggestion, hasConstraintSuggestion)

	case pendingDraggingNewCubicPoint:
		gw.clickPlacesCubicPoint(mx, my)

	case pendingDraggingNewLinePoint:
		gw.clickPlacesLinePoint(mx, my, constraintSuggestion, hasConstraintSuggestion)

	default:
		if gw.pending.operation == pendingDraggingNewRadius && hasConstraintSuggestion {
			gw.applySuggestion(constraintSuggestion)
		}
		gw.ClearPending(true)
		if !gw.hover.IsEmpty() {
			if !ctrlDown {
				gw.hoverWasSelectedOnMousedown = gw.IsSelected(&gw.hover)
				gw.MakeSelected(&gw.hover)
			} else {
				gw.MakeUnselected(&gw.hover, true)
			}
		}
	}

	// Activate group with newly created request/constraint
	var g *sketch.Group
	if hr.V != 0 {
		g = sk.Group(sk.Request(hr).Group)
	}
	if hc.V != 0 {
		g = sk.Group(sk.Constraint(hc).Group)
	}
	if g != nil {
		g.Visible = true
	}

	ss.ScheduleShowTW()
	gw.Invalidate(false)
}

// clickForCommand is a left click while pendingCommand.
func (gw *graphicsWindow) clickForCommand(mx, my float64) (core.HRequest, core.HConstraint) {
	var hr core.HRequest
	var hc core.HConstraint

	switch gw.pending.command {
	case cmdDatumPoint:
		hr = gw.placeDatumPoint(mx, my)

	case cmdLineSegment, cmdConstrSegment:
		hr = gw.placeLineSegment(mx, my)

	case cmdRectangle:
		hr = gw.placeRectangle(mx, my)

	case cmdCircle:
		hr = gw.placeCircle(mx, my)

	case cmdArc:
		hr = gw.placeArc(mx, my)

	case cmdCubic:
		hr = gw.placeCubic(mx, my)

	case cmdWorkplane:
		hr = gw.placeWorkplane(mx, my)

	case cmdTTFText:
		hr = gw.placeTTFText(mx, my)

	case cmdImage:
		hr = gw.placeImage(mx, my)

	case cmdComment:
		hc = gw.placeComment(mx, my)

	default:
		panic("ui: unexpected pending menu id")
	}
	return hr, hc
}

// placeDatumPoint starts a cmdDatumPoint at the click.
func (gw *graphicsWindow) placeDatumPoint(mx, my float64) core.HRequest {
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	hr := gw.AddRequest(sketch.RequestDatumPoint, true)
	pointForceTo(hr.Entity(0), v)
	gw.ConstrainPointByHovered(hr.Entity(0), &mouse)
	gw.ClearSuper()
	return hr
}

// placeLineSegment starts a cmdLineSegment, cmdConstrSegment at the click.
func (gw *graphicsWindow) placeLineSegment(mx, my float64) core.HRequest {
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	hr := gw.AddRequest(sketch.RequestLineSegment, true)
	sk.Request(hr).Construction = gw.pending.command == cmdConstrSegment
	pointForceTo(hr.Entity(1), v)
	gw.ConstrainPointByHovered(hr.Entity(1), &mouse)

	gw.ClearSuper()
	gw.AddToPending(hr)

	gw.pending.operation = pendingDraggingNewLinePoint
	gw.pending.request = hr
	gw.pending.point = hr.Entity(2)
	gw.pending.description = "click next point of line, or press Esc"
	pointForceTo(gw.pending.point, v)
	return hr
}

// placeRectangle starts a cmdRectangle at the click.
func (gw *graphicsWindow) placeRectangle(mx, my float64) core.HRequest {
	ss := gw.ss
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	var hr core.HRequest

	if !gw.LockedInWorkplane() {
		ss.Error("Can't draw rectangle in 3d; first, activate a workplane " +
			"with Sketch -> In Workplane.")
		gw.ClearSuper()
		return hr
	}
	var lns [4]core.HRequest
	ss.UndoRemember()
	for i := range 4 {
		lns[i] = gw.AddRequest(sketch.RequestLineSegment, false)
		gw.AddToPending(lns[i])
	}
	for i := range 4 {
		ss.constrainCoincident(lns[i].Entity(1), lns[(i+1)%4].Entity(2))
		pointForceTo(lns[i].Entity(1), v)
		pointForceTo(lns[i].Entity(2), v)
	}
	for i := range 4 {
		typ := core.ConstraintVertical
		if i%2 != 0 {
			typ = core.ConstraintHorizontal
		}
		ss.constrain(typ, core.HEntity{}, core.HEntity{}, lns[i].Entity(0))
	}
	if gw.ConstrainPointByHovered(lns[2].Entity(1), &mouse) {
		pos := sk.Entity(lns[2].Entity(1)).PointGetNum(sk)
		for i := range 4 {
			pointForceTo(lns[i].Entity(1), pos)
			pointForceTo(lns[i].Entity(2), pos)
		}
	}

	gw.pending.operation = pendingDraggingNewPoint
	gw.pending.point = lns[1].Entity(2)
	gw.pending.description = "click to place other corner of rectangle"
	hr = lns[0]
	return hr
}

// placeCircle starts a cmdCircle at the click.
func (gw *graphicsWindow) placeCircle(mx, my float64) core.HRequest {
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	hr := gw.AddRequest(sketch.RequestCircle, true)
	// Centered where we clicked
	pointForceTo(hr.Entity(1), v)
	// Normal to the screen
	sk.Entity(hr.Entity(32)).NormalForceTo(sk, geom.QuaternionFromBasis(vw.ProjRight, vw.ProjUp))
	// Initial radius zero
	sk.Entity(hr.Entity(64)).DistanceForceTo(sk, 0)

	gw.ConstrainPointByHovered(hr.Entity(1), &mouse)

	gw.ClearSuper()
	gw.AddToPending(hr)

	gw.pending.operation = pendingDraggingNewRadius
	gw.pending.circle = hr.Entity(0)
	gw.pending.description = "click to set radius"
	return hr
}

// placeArc starts a cmdArc at the click.
func (gw *graphicsWindow) placeArc(mx, my float64) core.HRequest {
	ss := gw.ss
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	var hr core.HRequest

	if !gw.LockedInWorkplane() {
		ss.Error("Can't draw arc in 3d; first, activate a workplane " +
			"with Sketch -> In Workplane.")
		gw.ClearPending(true)
		return hr
	}
	hr = gw.AddRequest(sketch.RequestArcOfCircle, true)
	// This fudge factor stops us from immediately failing to solve
	// because of the arc's implicit (equal radius) tangent.
	adj := vw.ProjRight.WithMagnitude(2 / vw.Scale)
	pointForceTo(hr.Entity(1), v.Minus(adj))
	pointForceTo(hr.Entity(2), v)
	pointForceTo(hr.Entity(3), v)
	gw.ConstrainPointByHovered(hr.Entity(2), &mouse)

	gw.ClearSuper()
	gw.AddToPending(hr)

	gw.pending.operation = pendingDraggingNewArcPoint
	gw.pending.point = hr.Entity(3)
	gw.pending.description = "click to place point"
	return hr
}

// placeCubic starts a cmdCubic at the click.
func (gw *graphicsWindow) placeCubic(mx, my float64) core.HRequest {
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	hr := gw.AddRequest(sketch.RequestCubic, true)
	for i := 1; i <= 4; i++ {
		pointForceTo(hr.Entity(i), v)
	}
	gw.ConstrainPointByHovered(hr.Entity(1), &mouse)

	gw.ClearSuper()
	gw.AddToPending(hr)

	gw.pending.operation = pendingDraggingNewCubicPoint
	gw.pending.point = hr.Entity(4)
	gw.pending.description = "click next point of cubic, or press Esc"
	return hr
}

// placeWorkplane starts a cmdWorkplane at the click.
func (gw *graphicsWindow) placeWorkplane(mx, my float64) core.HRequest {
	ss := gw.ss
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	var hr core.HRequest

	if gw.LockedInWorkplane() {
		ss.Error("Sketching in a workplane already; sketch in 3d before " +
			"creating new workplane.")
		gw.ClearSuper()
		return hr
	}
	hr = gw.AddRequest(sketch.RequestWorkplane, true)
	pointForceTo(hr.Entity(1), v)
	sk.Entity(hr.Entity(32)).NormalForceTo(sk, geom.QuaternionFromBasis(vw.ProjRight, vw.ProjUp))
	gw.ConstrainPointByHovered(hr.Entity(1), &mouse)
	gw.ClearSuper()
	return hr
}

// placeTTFText starts a cmdTTFText at the click.
func (gw *graphicsWindow) placeTTFText(mx, my float64) core.HRequest {
	ss := gw.ss
	sk := gw.sk()
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	var hr core.HRequest

	if !gw.LockedInWorkplane() {
		ss.Error("Can't draw text in 3d; first, activate a workplane " +
			"with Sketch -> In Workplane.")
		gw.ClearSuper()
		return hr
	}
	hr = gw.AddRequest(sketch.RequestTTFText, true)
	gw.AddToPending(hr)
	r := sk.Request(hr)
	r.Str = "Abc"
	r.Font = ttf.EmbeddedFont

	for i := 1; i <= 4; i++ {
		pointForceTo(hr.Entity(i), v)
	}

	gw.pending.operation = pendingDraggingNewPoint
	gw.pending.point = hr.Entity(3)
	gw.pending.description = "click to place bottom right of text"
	return hr
}

// placeImage starts a cmdImage at the click.
func (gw *graphicsWindow) placeImage(mx, my float64) core.HRequest {
	ss := gw.ss
	sk := gw.sk()
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }
	var hr core.HRequest

	if !gw.LockedInWorkplane() {
		ss.Error("Can't draw image in 3d; first, activate a workplane " +
			"with Sketch -> In Workplane.")
		gw.ClearSuper()
		return hr
	}
	hr = gw.AddRequest(sketch.RequestImage, true)
	gw.AddToPending(hr)
	r := sk.Request(hr)
	r.File = gw.pending.filename
	r.Construction = true

	for i := 1; i <= 4; i++ {
		pointForceTo(hr.Entity(i), v)
	}

	gw.pending.operation = pendingDraggingNewPoint
	gw.pending.point = hr.Entity(3)
	gw.pending.description = "click to place bottom right of image"
	return hr
}

// placeComment starts a cmdComment at the click.
func (gw *graphicsWindow) placeComment(mx, my float64) core.HConstraint {
	ss := gw.ss
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	var hc core.HConstraint

	gw.ClearSuper()
	var c core.Constraint
	c.Group = gw.activeGroupH()
	c.Workplane = gw.ActiveWorkplane()
	c.Type = core.ConstraintComment
	c.Disp.Offset = v
	c.Comment = "NEW COMMENT -- DOUBLE-CLICK TO EDIT"
	hc = ss.addConstraint(&c, true)
	return hc
}

// clickEndsRadius is a left click while pendingDraggingRadius.
func (gw *graphicsWindow) clickEndsRadius() {
	gw.ClearPending(true)
}

// clickPlacesPoint is a left click while pendingDraggingNewPoint, pendingDraggingNewArcPoint.
func (gw *graphicsWindow) clickPlacesPoint(mx, my float64, suggestion sketch.Suggestion, hasSuggestion bool) {
	mouse := geom.Point2d{U: mx, V: my}

	arc := gw.pending.operation == pendingDraggingNewArcPoint
	gw.ConstrainPointByHovered(gw.pending.point, &mouse)
	if arc && hasSuggestion {
		gw.applySuggestion(suggestion)
	}
	gw.ClearPending(true)
}

// clickPlacesCubicPoint is a left click while pendingDraggingNewCubicPoint.
func (gw *graphicsWindow) clickPlacesCubicPoint(mx, my float64) {
	ss := gw.ss
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }

	hr := gw.pending.point.Request()
	r := sk.Request(hr)

	if gw.hover.Entity == hr.Entity(1) && r.ExtraPoints >= 2 {
		// They want the endpoints coincident, which means a periodic
		// spline instead.
		r.Type = sketch.RequestCubicPeriodic
		// Remove the off-curve control points, which are no longer
		// needed here; so move [2,ep+1] down, skipping first pt.
		for i := 2; i <= r.ExtraPoints+1; i++ {
			pointForceTo(hr.Entity((i-1)+1), sk.Entity(hr.Entity(i+1)).PointGetNum(sk))
		}
		// and move ep+3 down by two, skipping both
		pointForceTo(hr.Entity((r.ExtraPoints+1)+1),
			sk.Entity(hr.Entity((r.ExtraPoints+3)+1)).PointGetNum(sk))
		r.ExtraPoints -= 2
		// And we're done.
		sk.MarkGroupDirty(r.Group, false)
		gw.ClearPending(true)
		return
	}

	if gw.ConstrainPointByHovered(gw.pending.point, &mouse) {
		gw.ClearPending(true)
		return
	}

	if r.ExtraPoints >= core.MaxPointsInEntity-4 {
		gw.ClearPending(true)
		return
	}

	sk.Request(hr).ExtraPoints++
	ss.GenerateAll(sketch.GenerateRegen)

	ep := sk.Request(hr).ExtraPoints
	last := sk.Entity(hr.Entity(3 + ep)).PointGetNum(sk)

	pointForceTo(hr.Entity(2+ep), last)
	pointForceTo(hr.Entity(3+ep), v)
	pointForceTo(hr.Entity(4+ep), v)
	gw.pending.point = hr.Entity(4 + ep)
}

// clickPlacesLinePoint is a left click while pendingDraggingNewLinePoint.
func (gw *graphicsWindow) clickPlacesLinePoint(mx, my float64, suggestion sketch.Suggestion, hasSuggestion bool) {
	ss := gw.ss
	sk := gw.sk()
	mouse := geom.Point2d{U: mx, V: my}
	vw := &gw.view
	v := vw.Offset.ScaledBy(-1)
	v = v.Plus(vw.ProjRight.ScaledBy(mx / vw.Scale))
	v = v.Plus(vw.ProjUp.ScaledBy(my / vw.Scale))
	pointForceTo := func(he core.HEntity, p geom.Vector) { sk.Entity(he).PointForceTo(sk, p) }

	if gw.hover.Entity.V != 0 {
		e := sk.Entity(gw.hover.Entity)
		if e.IsPoint() {
			hrl := gw.pending.point.Request()
			sp := sk.Entity(hrl.Entity(1))
			if e.PointGetNum(sk).Equals(sp.PointGetNum(sk)) {
				// If we constrained by the hovered point, then we would
				// create a zero-length line segment. That's not good, so
				// just stop drawing.
				gw.ClearPending(true)
				return
			}
		}
	}

	doneDragging := gw.ConstrainPointByHovered(gw.pending.point, &mouse)

	// Constrain the line segment horizontal or vertical if close
	// enough
	if hasSuggestion {
		gw.applySuggestion(suggestion)
	}

	if doneDragging {
		gw.ClearPending(true)
		return
	}

	// Create a new line segment, so that we continue drawing.
	hr := gw.AddRequest(sketch.RequestLineSegment, true)
	gw.ReplacePending(gw.pending.request, hr)
	sk.Request(hr).Construction = sk.Request(gw.pending.request).Construction
	// Displace the second point of the new line segment slightly, to
	// avoid creating zero-length edge warnings.
	pointForceTo(hr.Entity(2), v.Plus(vw.ProjRight.ScaledBy(0.5/vw.Scale)))

	// Constrain the line segments to share an endpoint
	ss.constrainCoincident(gw.pending.point, hr.Entity(1))
	pendingPos := sk.Entity(gw.pending.point).PointGetNum(sk)
	pointForceTo(hr.Entity(1), pendingPos)

	// And drag an endpoint of the new line segment
	gw.pending.operation = pendingDraggingNewLinePoint
	gw.pending.request = hr
	gw.pending.point = hr.Entity(2)
	gw.pending.description = "click next point of line, or press Esc"
}

// MouseLeftUp ends a drag or a marquee.
func (gw *graphicsWindow) MouseLeftUp(mx, my float64, shiftDown, ctrlDown bool) {
	gw.orig.mouseDown = false

	switch gw.pending.operation {
	case pendingDraggingPoints, pendingDraggingConstraint, pendingDraggingNormal, pendingDraggingRadius:
		if !gw.hoverWasSelectedOnMousedown {
			// And then clear the selection again, since they probably
			// didn't want that selected if they just were dragging it.
			gw.ClearSelection()
		}
		gw.hoverWasSelectedOnMousedown = false
		gw.extraLine.Draw = false
		gw.ClearPending(true)
		gw.Invalidate(false)

	case pendingDraggingMarquee:
		gw.SelectByMarquee()
		gw.ClearPending(true)
		gw.Invalidate(false)

	case pendingNone:
		if gw.hover.IsEmpty() && !ctrlDown {
			gw.ClearSelection()
		}
	}
}

// EditConstraint shows the editor over a constraint's label.
func (gw *graphicsWindow) EditConstraint(constraint core.HConstraint) {
	ss := gw.ss
	gw.constraintBeingEdited = constraint
	gw.ClearSuper()

	c := gw.sk().Constraint(gw.constraintBeingEdited)
	if !c.HasLabel() {
		// Not meaningful to edit a constraint without a dimension
		return
	}
	if c.Reference {
		// Not meaningful to edit a reference dimension
		return
	}

	camera := gw.Camera()
	p3 := gw.constraintLabelPos(c, &camera)
	p2 := gw.ProjectPoint(p3)

	var editValue, editPlaceholder string
	switch c.Type {
	case core.ConstraintComment:
		editValue = c.Comment
		editPlaceholder = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	default:
		if text := sketch.ConstraintText(c); text != "" {
			// A stored expression is edited as it was typed.
			editValue = text
			editPlaceholder = "10.000000"
			break
		}
		value := math.Abs(c.ValA)

		// If displayed as radius, also edit as radius.
		if c.Type == core.ConstraintDiameter && c.Other {
			value /= 2
		}

		// Try showing value with default number of digits after decimal
		// first.
		switch c.Type {
		case core.ConstraintLengthRatio, core.ConstraintArcArcLenRatio, core.ConstraintArcLineLenRatio:
			editValue = fmt.Sprintf("%.3f", value)
		case core.ConstraintAngle:
			editValue = ss.DegreeToString(value)
		default:
			editValue = ss.MmToString(value, true)
			value /= ss.MmPerUnit()
		}
		// If that's not enough to represent it exactly, show the value
		// with as many digits after decimal as required, up to 10.
		digits := 0
		for {
			shown, _ := strconv.ParseFloat(strings.TrimSpace(editValue), 64)
			if math.Abs(shown-value) <= 1e-10 {
				break
			}
			editValue = fmt.Sprintf("%.*f", digits, value)
			digits++
		}
		editPlaceholder = "10.000000"
	}

	// A named dimension is edited as "name = value"; the name may be
	// changed, or taken away with "= value" (roadmap 16.1).
	if c.Name != "" && c.Type != core.ConstraintComment {
		editValue = c.Name + " = " + editValue
	}

	width, height := gw.window.ContentSize()
	hs := c.Disp.Style
	if hs.V == 0 {
		hs.V = core.StyleConstraint
	}
	capHeight := ss.styleTextHeight(hs)
	vf := res.BuiltinVectorFont()
	fontHeight := vf.Height(capHeight)
	editMinWidth := vf.Width(capHeight, editPlaceholder)
	gw.window.ShowEditor(p2.U+width/2, height/2-p2.V, fontHeight, editMinWidth, false, editValue)
}

// MouseLeftDoubleClick edits the hovered constraint.
func (gw *graphicsWindow) MouseLeftDoubleClick(mx, my float64) {
	if gw.window.IsEditorVisible() {
		return
	}
	gw.ss.TW.HideEditControl()

	if gw.hover.Constraint.V != 0 {
		gw.EditConstraint(gw.hover.Constraint)
	}
}

// EditControlDone applies the edited constraint value.
func (gw *graphicsWindow) EditControlDone(s string) {
	ss := gw.ss
	gw.window.HideEditor()
	gw.window.Invalidate()

	c := gw.sk().Constraint(gw.constraintBeingEdited)
	if c == nil {
		return
	}

	if c.Type == core.ConstraintComment {
		ss.UndoRemember()
		c.Comment = s
		return
	}

	name, named, value := sketch.SplitNamedText(s)
	if e := ss.exprFromString(value); e != nil {
		// A length typed as a plain expression is in the view's unit;
		// one that names a parameter is kept, in mm and degrees.
		err := ss.SK.UndoRememberFor(func() error {
			if named {
				if err := ss.SK.SetConstraintName(gw.constraintBeingEdited, name); err != nil {
					return err
				}
			}
			return ss.SK.SetConstraintText(gw.constraintBeingEdited, value)
		})
		if err != nil {
			ss.Report(err)
			return
		}
		ss.SK.Unsaved = true
		ss.UndoEnableMenus()
	}
}

// MouseScroll zooms about the mouse.
func (gw *graphicsWindow) MouseScroll(zoomMultiplyer float64) {
	// scale *= exp(ln(1.2) * zoomMultiplyer), so that the same total
	// scroll delta always gives the same zoom however it was split into
	// events.
	gw.ZoomToMouse(zoomMultiplyer)
}

// MouseLeave un-hovers everything, unless a context menu is shown.
func (gw *graphicsWindow) MouseLeave() {
	if !gw.context.active {
		gw.hover.Clear()
		gw.toolbarHovered = cmdNone
		gw.Invalidate(false)
	}
	gw.extraLine.Draw = false
}

// exprFromString parses an expression, telling the user if it doesn't.
func (ss *Application) exprFromString(input string) *core.Expr {
	e, err := core.NewFromString(input)
	if err != nil {
		ss.Error("Not a valid number or expression: '%s'.\n%s.", input, err.Error())
		return nil
	}
	return e
}

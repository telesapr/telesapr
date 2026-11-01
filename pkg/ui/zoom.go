package ui

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// screenArea is a rectangle of what a camera sees, in its pixels from
// the middle, y up: where its middle is and how large it is.
type screenArea struct {
	Mid           geom.Point2d
	Width, Height float64
}

// cameraArea is the whole of what camera sees.
func cameraArea(camera *render.Camera) screenArea {
	return screenArea{Width: camera.Width, Height: camera.Height}
}

// freeArea is the part of the window camera draws that the toolbar,
// drawn over the window's left side, leaves free: what Zoom To Fit
// fits, the menu's zooms keep still and the drawing sheet fills. A
// picture, which has no toolbar over it, is free all over, and so is a
// window too narrow for the toolbar to leave half of it.
func (gw *graphicsWindow) freeArea(camera *render.Camera) screenArea {
	a := cameraArea(camera)
	if !gw.ss.ShowToolbar || gw.inPicture || gw.window == nil || gw.ss.Windowless {
		return a
	}
	tw := float64(toolbarWidth(toolbarColumns(int(camera.Height))))
	if tw >= a.Width/2 {
		return a
	}
	a.Width -= tw
	a.Mid.U = tw / 2
	return a
}

// fitPoints are the points Zoom To Fit keeps in view: those of the
// entities, where the constraints draw, and, with includeMesh, the
// vertices of the active group's solid as the window draws it (its
// display mesh, which holds the solid of every group up to it) and of
// its sketch's loops; without includeMesh, the vertices of the faces
// only.
func (gw *graphicsWindow) fitPoints(entities []*core.Entity, constraints []*core.Constraint,
	faces []core.HEntity, includeMesh bool, camera *render.Camera) []geom.Vector {
	sk := gw.sk()
	var pts []geom.Vector
	for _, e := range entities {
		switch {
		case e.IsPoint():
			pts = append(pts, e.PointGetNum(sk))
		case e.Type == core.EntityCircle:
			// Lots of entities can extend outside the bbox of their
			// points, but circles are particularly bad.
			r := e.CircleGetRadiusNum(sk)
			c := sk.Entity(e.Point[0]).PointGetNum(sk)
			q := sk.Entity(e.Normal).NormalGetNum(sk)
			u, v := q.RotationU().ScaledBy(r), q.RotationV().ScaledBy(r)
			pts = append(pts, c.Plus(u), c.Minus(u), c.Plus(v), c.Minus(v))
		case e.Type == core.EntityArcOfCircle:
			// An arc bulges past its ends and centre, and one whose ends
			// meet, as a lathe sweeps from a point, is a whole circle:
			// the points it is drawn through.
			for _, ed := range gw.getOrGenerateEdges(e).Edges {
				pts = append(pts, ed.A, ed.B)
			}
		default:
			// We have to iterate children points, because we can select
			// entities without points.
			for i := range core.MaxPointsInEntity {
				if e.Point[i].V == 0 {
					break
				}
				pts = append(pts, sk.Entity(e.Point[i]).PointGetNum(sk))
			}
		}
	}

	for _, c := range constraints {
		pts = append(pts, gw.constraintReferencePoints(c, camera)...)
	}

	if !includeMesh && len(faces) == 0 {
		return pts
	}

	g := sk.Group(gw.activeGroupH())
	g.GenerateDisplayItems(sk)
	for i := range g.DisplayMesh.Triangles {
		tr := &g.DisplayMesh.Triangles[i]
		if !includeMesh && !containsFace(faces, uint32(tr.Meta.Face)) {
			continue
		}
		pts = append(pts, tr.A, tr.B, tr.C)
	}
	if !includeMesh {
		return pts
	}
	for _, sc := range g.PolyLoops.Contours {
		for _, sp := range sc.Points {
			pts = append(pts, sp.P)
		}
	}
	return pts
}

// containsFace reports whether faces holds the face face.
func containsFace(faces []core.HEntity, face uint32) bool {
	for _, f := range faces {
		if f.V == face {
			return true
		}
	}
	return false
}

// threadHelperGroups are the groups of screw threads whose entities
// Zoom To Fit leaves out: a thread is one feature, and what its groups
// draw (the blank, whose lathe sweeps circles well past the part, the
// ridge, the datum at a face's end) is how it is made, not the model;
// the solid they make is fitted. A thread's sketch that is active, being
// edited, is fitted as any sketch is.
func (gw *graphicsWindow) threadHelperGroups() map[core.HGroup]bool {
	sk := gw.sk()
	active := sk.Group(gw.activeGroupH())
	out := map[core.HGroup]bool{}
	for hg := range sk.ThreadOfGroups() {
		if hg == active.H && (active.Type == sketch.GroupDrawingWorkplane || active.Type == sketch.GroupDrawing3D) {
			continue
		}
		out[hg] = true
	}
	return out
}

// ZoomToFit scales and centres the view on the sketch and its solid, or
// on the selection with useSelection, in the part of the window the
// toolbar leaves free.
func (gw *graphicsWindow) ZoomToFit(includingInvisibles, useSelection bool) {
	if gw.window == nil || gw.ss.Windowless {
		return
	}
	camera := gw.Camera()
	gw.view.Scale = gw.zoomToFit(&camera, gw.freeArea(&camera), includingInvisibles, useSelection)
}

// zoomToFit moves the view's offset so that what Zoom To Fit shows is
// in the middle of area, and returns the scale at which it fills nine
// tenths of area's width or height. camera is the view as it is.
func (gw *graphicsWindow) zoomToFit(camera *render.Camera, area screenArea, includingInvisibles, useSelection bool) float64 {
	sk := gw.sk()
	var entities []*core.Entity
	var constraints []*core.Constraint
	var faces []core.HEntity

	if useSelection {
		for i := range gw.selection {
			s := &gw.selection[i]
			if s.Entity.V != 0 {
				e := sk.Entity(s.Entity)
				if e.IsFace() {
					faces = append(faces, e.H)
					continue
				}
				entities = append(entities, e)
			}
			if s.Constraint.V != 0 {
				constraints = append(constraints, sk.Constraint(s.Constraint))
			}
		}
	}

	selectionUsed := len(entities) > 0 || len(constraints) > 0 || len(faces) > 0

	if !selectionUsed {
		helpers := gw.threadHelperGroups()
		for i := range sk.Entities {
			e := &sk.Entities[i]
			// We don't want to handle separate points, because we will
			// iterate them inside entities.
			if e.IsPoint() {
				continue
			}
			if !includingInvisibles && !sk.EntityIsVisible(e) {
				continue
			}
			if helpers[e.Group] {
				continue
			}
			entities = append(entities, e)
		}
		for i := range sk.Constraints {
			c := &sk.Constraints[i]
			if !gw.constraintIsVisible(c) || helpers[c.Group] {
				continue
			}
			constraints = append(constraints, c)
		}
	}

	pts := gw.fitPoints(entities, constraints, faces, !selectionUsed, camera)

	// First a parallel projection, whatever the scale: where the points
	// fall on the screen then is linear in the scale.
	pmax, pmin := geom.Point2d{U: -1e12, V: -1e12}, geom.Point2d{U: 1e12, V: 1e12}
	zmin := 0.0
	for _, p := range pts {
		pp, _ := camera.ProjectPoint4(p)
		pmax.U, pmax.V = geom.Max(pmax.U, pp.X), geom.Max(pmax.V, pp.Y)
		pmin.U, pmin.V = geom.Min(pmin.U, pp.X), geom.Min(pmin.V, pp.Y)
		zmin = geom.Min(zmin, pp.Z)
	}

	xm, ym := (pmax.U+pmin.U)/2, (pmax.V+pmin.V)/2
	dx, dy := pmax.U-pmin.U, pmax.V-pmin.V

	if dx == 0 && dy == 0 {
		// Nothing with a size, as a new sketch's lone origin: it goes
		// to the middle of the window, where a new sketch starts, and
		// the toolbar is nowhere near it.
		area.Mid = geom.Point2d{}
	}
	scale := fitScale(dx, dy, area)
	scale = inFrontOfCamera(scale, zmin, camera.Tangent)

	// The middle of the points goes to the middle of area.
	gw.view.Offset = gw.view.Offset.Plus(gw.view.ProjRight.ScaledBy(area.Mid.U/scale - xm)).Plus(
		gw.view.ProjUp.ScaledBy(area.Mid.V/scale - ym))

	if camera.Tangent != 0 && len(pts) > 0 {
		scale = gw.fitPerspective(pts, *camera, area, scale, zmin)
	}
	return scale
}

// fitScale is the scale at which dx by dy, in mm on the screen, fills
// nine tenths of area across or up, within the scales the view allows.
func fitScale(dx, dy float64, area screenArea) float64 {
	if dx == 0 && dy == 0 {
		return 5
	}
	scalex, scaley := 1e12, 1e12
	if dx != 0 {
		scalex = 0.9 * area.Width / dx
	}
	if dy != 0 {
		scaley = 0.9 * area.Height / dy
	}
	scale := geom.Min(scalex, scaley)
	scale = geom.Min(300.0, scale)
	return geom.Max(0.003, scale)
}

// inFrontOfCamera lowers scale so that no point, the nearest zmin from
// the offset along the line of sight, lies behind the camera of
// perspective tangent: w = 1 + tangent*scale*z stays at least 0.1.
func inFrontOfCamera(scale, zmin, tangent float64) float64 {
	if tangent == 0 || zmin >= 0 || 1+tangent*scale*zmin >= 0.1 {
		return scale
	}
	return geom.Min(scale, (0.1-1)/(tangent*zmin))
}

// fitPerspective corrects a fit made in parallel projection for the
// perspective: what is near the camera is drawn larger, and off to one
// side, so the bounds of the points as drawn are made the fit's again,
// a few times, as each change of scale changes the perspective.
// It moves the view's offset and returns the scale.
func (gw *graphicsWindow) fitPerspective(pts []geom.Vector, camera render.Camera, area screenArea, scale, zmin float64) float64 {
	v := &gw.view
	for range 8 {
		camera.Offset, camera.Scale = v.Offset, scale
		pmax, pmin := geom.Point2d{U: -1e12, V: -1e12}, geom.Point2d{U: 1e12, V: 1e12}
		for _, p := range pts {
			// Pixels from the middle of the screen.
			pp, w := camera.ProjectPoint4(p)
			pp = pp.ScaledBy(scale / w)
			pmax.U, pmax.V = geom.Max(pmax.U, pp.X), geom.Max(pmax.V, pp.Y)
			pmin.U, pmin.V = geom.Min(pmin.U, pp.X), geom.Min(pmin.V, pp.Y)
		}
		dx, dy := pmax.U-pmin.U, pmax.V-pmin.V
		if dx == 0 && dy == 0 {
			break
		}
		// Moving the offset by d moves a point by about d*scale/w pixels;
		// across the middle of the points w is about 1.
		shiftU := area.Mid.U - (pmax.U+pmin.U)/2
		shiftV := area.Mid.V - (pmax.V+pmin.V)/2
		v.Offset = v.Offset.Plus(v.ProjRight.ScaledBy(shiftU / scale)).Plus(v.ProjUp.ScaledBy(shiftV / scale))
		// dx, dy are pixels at scale: as mm at scale 1 they give the
		// scale they would fit at, were the perspective to stay.
		next := inFrontOfCamera(fitScale(dx/scale, dy/scale, area), zmin, camera.Tangent)
		done := math.Abs(next/scale-1) < 1e-3 && math.Abs(shiftU) < 0.5 && math.Abs(shiftV) < 0.5
		scale = next
		if done {
			break
		}
	}
	return scale
}

// ZoomToMouse zooms by 1.2 to the power zoomMultiplyer about the mouse,
// as the scroll wheel does.
func (gw *graphicsWindow) ZoomToMouse(zoomMultiplyer float64) {
	gw.zoomAbout(gw.currentMousePosition, zoomMultiplyer)
}

// zoomFromMenu zooms by 1.2 to the power zoomMultiplyer about the
// middle of the part of the window the toolbar leaves free, as View >
// Zoom In and Zoom Out and their keys do: the mouse may be anywhere, on
// the menu or out of the window.
func (gw *graphicsWindow) zoomFromMenu(zoomMultiplyer float64) {
	camera := gw.Camera()
	gw.zoomAbout(gw.freeArea(&camera).Mid, zoomMultiplyer)
}

// zoomAbout zooms by 1.2 to the power zoomMultiplyer keeping what is
// at p, in pixels from the middle of the window, where it is.
func (gw *graphicsWindow) zoomAbout(p geom.Point2d, zoomMultiplyer float64) {
	v := &gw.view
	offsetRight := v.Offset.Dot(v.ProjRight)
	offsetUp := v.Offset.Dot(v.ProjUp)

	righti := p.U/v.Scale - offsetRight
	upi := p.V/v.Scale - offsetUp

	// zoomMultiplyer of 1 gives a default zoom factor of 1.2x.
	v.Scale *= math.Exp(0.1823216 * zoomMultiplyer) // ln(1.2) = 0.1823216

	rightf := p.U/v.Scale - offsetRight
	upf := p.V/v.Scale - offsetUp

	v.Offset = v.Offset.Plus(v.ProjRight.ScaledBy(rightf - righti))
	v.Offset = v.Offset.Plus(v.ProjUp.ScaledBy(upf - upi))

	if gw.ss.TW.shown.screen == screenEditView {
		if gw.havePainted {
			gw.ss.ScheduleShowTW()
		}
	}
	gw.havePainted = false
	gw.Invalidate(false)
}

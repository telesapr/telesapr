package ui

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/srf"
)

// hiddenDepth is how far, in pixels of depth, a face must lie in front
// of a line for the line to count as hidden by it: a sketch line on a
// face or along one of its edges is not hidden, though the face's
// triangles are only within the chord tolerance of it.
const hiddenDepth = 2.0

// hiddenAround is how far round the point found, in pixels, the face
// must be in front of it.
const hiddenAround = 2.0

// hidesEntities reports whether the faces of the solid shown can hide
// the lines a click would pick: picking prefers a face to a line behind
// it where faces are drawn and can be picked, and hidden lines are not
// drawn as though they were in front.
func (gw *graphicsWindow) hidesEntities() bool {
	return gw.pending.operation == pendingNone && gw.showShaded && gw.showFaces &&
		gw.drawOccludedAs != occludedVisible
}

// hiddenByFace reports whether e, picked by picker where it is near
// the mouse, lies behind a face of the active group's solid there: a
// click then picks the face, which is what the user sees, rather than a
// line or a point within 10 pixels that the face hides. Labels stay
// pickable through the solid: picker gives no point for them.
func (gw *graphicsWindow) hiddenByFace(picker *render.ObjectPicker) bool {
	if !picker.AtKnown {
		return false
	}
	g := gw.sk().Group(gw.activeGroupH())
	if g == nil || len(g.DisplayMesh.Triangles) == 0 {
		return false
	}
	var keep *srf.HalfSpace
	if h, ok := gw.sectionKeep(); ok {
		keep = &h
	}
	at := gw.view.ProjectPoint3(picker.MinAt)
	// Hidden only where the face is in front all round the point: a
	// line along the solid's outline, or on a face seen edge on, is
	// grazed by the ray through it, which may hit a face's edge.
	for _, d := range [4]geom.Point2d{{U: hiddenAround}, {U: -hiddenAround}, {V: hiddenAround}, {V: -hiddenAround}} {
		x, y := at.X+d.U, at.Y+d.V
		rayPoint := gw.view.UnProjectPoint3(geom.Vector{X: x, Y: y, Z: 0.0})
		rayDir := gw.view.UnProjectPoint3(geom.Vector{X: x, Y: y, Z: 1.0}).Minus(rayPoint)
		face, hit := g.DisplayMesh.FirstHitIn(rayPoint, rayDir, keep)
		// Depth grows away from the viewer.
		if face == 0 || gw.view.ProjectPoint3(hit).Z >= at.Z-hiddenDepth {
			return false
		}
	}
	return true
}

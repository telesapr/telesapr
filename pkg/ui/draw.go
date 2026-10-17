package ui

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// drawSelection is Selection::Draw: highlighted, and when emphasized,
// with thick lines from the top left of the screen to its reference
// points.
func (gw *graphicsWindow) drawSelection(s *Selection, isHovered bool, canvas render.Canvas) {
	sk := gw.sk()
	camera := canvas.Camera()

	var refs []geom.Vector
	how := entityDrawSelected
	chow := constraintDrawSelected
	if isHovered {
		how, chow = entityDrawHovered, constraintDrawHovered
	}
	if s.Entity.V != 0 {
		e := sk.Entity(s.Entity)
		gw.drawEntity(e, how, canvas)
		if s.Emphasized {
			refs = append(refs, gw.entityReferencePoints(e)...)
		}
	}
	if s.Edge.A != 0 {
		gw.drawSolidEdge(s.Edge, isHovered, canvas)
	}
	if s.Constraint.V != 0 {
		c := sk.Constraint(s.Constraint)
		gw.drawConstraint(c, chow, canvas)
		if s.Emphasized {
			refs = append(refs, gw.constraintReferencePoints(c, camera)...)
		}
	}
	if s.Emphasized && (s.Constraint.V != 0 || s.Entity.V != 0) {
		// We want to emphasize this constraint or entity, by drawing a
		// thick line from the top left corner of the screen to the
		// reference point(s) of that entity or constraint.
		strokeEmphasis := render.Stroke{
			Layer: render.LayerFront,
			Color: gw.ss.styleColor(hstyle(core.StyleHovered)).WithAlpha(50),
			Width: 40,
			Unit:  render.UnitPX,
		}
		hcsEmphasis := canvas.InternStroke(strokeEmphasis)

		topLeftScreen := geom.Point2d{U: -camera.Width / 2, V: camera.Height / 2}
		topLeft := camera.UnProjectPoint(topLeftScreen)

		// std::unique: drop consecutive equal points.
		var uniq []geom.Vector
		for _, p := range refs {
			if len(uniq) > 0 && uniq[len(uniq)-1].Equals(p) {
				continue
			}
			uniq = append(uniq, p)
		}
		for _, p := range uniq {
			canvas.DrawLine(topLeft, p, hcsEmphasis)
		}
	}
}

// ClearSelection selects nothing.
func (gw *graphicsWindow) ClearSelection() {
	gw.selection = nil
	gw.ss.ScheduleShowTW()
	gw.Invalidate(false)
}

// ClearNonexistentSelectionItems drops what no longer exists from the
// selection.
func (gw *graphicsWindow) ClearNonexistentSelectionItems() {
	sk := gw.sk()
	change := false
	kept := gw.selection[:0]
	for _, s := range gw.selection {
		if s.Constraint.V != 0 && sk.Constraint(s.Constraint) == nil {
			change = true
			continue
		}
		if s.Entity.V != 0 && sk.Entity(s.Entity) == nil {
			change = true
			continue
		}
		kept = append(kept, s)
	}
	gw.selection = kept
	if change {
		gw.Invalidate(false)
	}
}

// IsSelectedEntity reports whether he is selected.
func (gw *graphicsWindow) IsSelectedEntity(he core.HEntity) bool {
	return gw.IsSelected(&Selection{Entity: he})
}

// IsSelected reports whether st is selected.
func (gw *graphicsWindow) IsSelected(st *Selection) bool {
	for i := range gw.selection {
		if gw.selection[i].Equals(st) {
			return true
		}
	}
	return false
}

// MakeUnselectedEntity unselects he.
func (gw *graphicsWindow) MakeUnselectedEntity(he core.HEntity, coincidentPointTrick bool) {
	gw.MakeUnselected(&Selection{Entity: he}, coincidentPointTrick)
}

// MakeUnselected unselects an item; with coincidentPointTrick, also
// the points coincident with it, since the lower of two coincident
// points could never be hovered to be deselected.
func (gw *graphicsWindow) MakeUnselected(stog *Selection, coincidentPointTrick bool) {
	if stog.IsEmpty() {
		return
	}
	sk := gw.sk()
	for i := range gw.selection {
		gw.selection[i].Tag = 0
	}
	// If an item was selected, then we just un-select it.
	for i := range gw.selection {
		if gw.selection[i].Equals(stog) {
			gw.selection[i].Tag = 1
		}
	}
	if stog.Entity.V != 0 && coincidentPointTrick {
		e := sk.Entity(stog.Entity)
		if e.IsPoint() {
			ep := e.PointGetNum(sk)
			for i := range gw.selection {
				s := &gw.selection[i]
				if s.Entity.V == 0 || s.Entity == stog.Entity {
					continue
				}
				se := sk.Entity(s.Entity)
				if !se.IsPoint() {
					continue
				}
				if ep.Equals(se.PointGetNum(sk)) {
					s.Tag = 1
				}
			}
		}
	}
	gw.removeTaggedSelections()
}

func (gw *graphicsWindow) removeTaggedSelections() {
	kept := gw.selection[:0]
	for _, s := range gw.selection {
		if s.Tag == 0 {
			kept = append(kept, s)
		}
	}
	gw.selection = kept
}

// MakeSelectedEntity selects he.
func (gw *graphicsWindow) MakeSelectedEntity(he core.HEntity) {
	gw.MakeSelected(&Selection{Entity: he})
}

// MakeSelectedConstraint selects hc.
func (gw *graphicsWindow) MakeSelectedConstraint(hc core.HConstraint) {
	gw.MakeSelected(&Selection{Constraint: hc})
}

// MakeSelected selects an item, if it isn't already; only
// maxSelectableFaces faces may be selected at once.
func (gw *graphicsWindow) MakeSelected(stog *Selection) {
	if stog.IsEmpty() || gw.IsSelected(stog) {
		return
	}
	sk := gw.sk()
	if stog.Entity.V != 0 && sk.Entity(stog.Entity).IsFace() {
		c := 0
		for i := range gw.selection {
			gw.selection[i].Tag = 0
		}
		for i := range gw.selection {
			he := gw.selection[i].Entity
			if he.V != 0 && sk.Entity(he).IsFace() {
				c++
				if c >= maxSelectableFaces {
					gw.selection[i].Tag = 1
				}
			}
		}
		gw.removeTaggedSelections()
	}
	gw.selection = append(gw.selection, *stog)
}

// marqueeWindow reports whether the marquee being dragged selects only
// what lies wholly inside it, dragged from left to right; dragged from
// right to left it selects whatever it crosses.
func (gw *graphicsWindow) marqueeWindow() bool {
	return gw.orig.mouse.U >= gw.ProjectPoint(gw.orig.marqueePoint).U
}

// SelectByMarquee selects what lies within the marquee: dragged from
// left to right, what lies wholly inside it; from right to left,
// whatever it crosses.
func (gw *graphicsWindow) SelectByMarquee() {
	sk := gw.sk()
	marqueePoint := gw.ProjectPoint(gw.orig.marqueePoint)
	marqueeBBox := core.NewBBox(
		geom.Vector{X: marqueePoint.U, Y: marqueePoint.V, Z: core.VeryNegative},
		geom.Vector{X: gw.orig.mouse.U, Y: gw.orig.mouse.V, Z: core.VeryPositive})
	window := gw.marqueeWindow()

	for i := range sk.Entities {
		e := &sk.Entities[i]
		if e.Group != gw.activeGroupH() {
			continue
		}
		if e.IsFace() || e.IsDistance() {
			continue
		}
		if !sk.EntityIsVisible(e) {
			continue
		}
		entityBBox, hasBBox := gw.getOrGenerateScreenBBox(e)
		if window {
			if hasBBox && marqueeBBox.Contains(geom.Point2d{U: entityBBox.MinP.X, V: entityBBox.MinP.Y}, 0) &&
				marqueeBBox.Contains(geom.Point2d{U: entityBBox.MaxP.X, V: entityBBox.MaxP.Y}, 0) {
				gw.MakeSelectedEntity(e.H)
			}
			continue
		}
		if hasBBox && entityBBox.Overlaps(marqueeBBox) {
			if e.Type == core.EntityLineSegment {
				p0 := gw.ProjectPoint3(e.EndpointStart(sk))
				p1 := gw.ProjectPoint3(e.EndpointFinish(sk))
				if !marqueeBBox.Contains(geom.Point2d{U: p0.X, V: p0.Y}, 0) &&
					!marqueeBBox.Contains(geom.Point2d{U: p1.X, V: p1.Y}, 0) {
					// The marquee contains neither end point, so the
					// segment is outside it, or crosses it.
					if !geom.BoundingBoxIntersectsLine(marqueeBBox.MaxP, marqueeBBox.MinP, p0, p1, true) {
						continue
					}
				}
			}
			gw.MakeSelectedEntity(e.H)
		}
	}
}

// onlyFace is the face of the solid selected, by its identity, when it
// is the one thing selected: a face entity, as an extrusion's, or a face
// no entity names, picked on the model.
func (gs *groupedSelection) onlyFace() (srf.FaceID, bool) {
	if gs.constraints > 0 || len(gs.solidEdge) > 0 {
		return 0, false
	}
	switch {
	case gs.n == 1 && gs.faces == 1 && len(gs.solidFace) == 0:
		return srf.FaceID(gs.face[0].V), true
	case gs.n == 0 && len(gs.solidFace) == 1:
		return gs.solidFace[0], true
	}
	return 0, false
}

// GroupSelection sorts the selection by kind into gw.gs.
func (gw *graphicsWindow) GroupSelection() {
	sk := gw.sk()
	gw.gs = groupedSelection{}
	gs := &gw.gs
	for i := range gw.selection {
		s := &gw.selection[i]
		if s.Entity.V != 0 {
			gs.n++
			e := sk.Entity(s.Entity)
			if e.IsStylable() {
				gs.stylables++
			}
			// A list of points, and a list of all entities that aren't
			// points.
			if e.IsPoint() {
				gs.points++
				gs.point = append(gs.point, s.Entity)
			} else {
				gs.entities++
				gs.entity = append(gs.entity, s.Entity)
			}
			// And an auxiliary list of normals, including normals from
			// workplanes.
			if e.IsNormal() {
				gs.anyNormals++
				gs.anyNormal = append(gs.anyNormal, s.Entity)
			} else if e.IsWorkplane() {
				gs.anyNormals++
				gs.anyNormal = append(gs.anyNormal, e.NormalEntity(sk).H)
			}
			// And of vectors (i.e., stuff with a direction to constrain)
			if e.HasVector() {
				gs.vectors++
				gs.vector = append(gs.vector, s.Entity)
			}
			// Faces (which are special, associated/drawn with triangles)
			if e.IsFace() {
				gs.faces++
				gs.face = append(gs.face, s.Entity)
			}
			if e.HasEndpoints() {
				gs.withEndpoints++
			}
			// And some aux counts too
			switch e.Type {
			case core.EntityWorkplane:
				gs.workplanes++
			case core.EntityLineSegment:
				gs.lineSegments++
			case core.EntityCubic:
				gs.cubics++
			case core.EntityCubicPeriodic:
				gs.periodicCubics++
			case core.EntityArcOfCircle:
				gs.circlesOrArcs++
				gs.arcs++
			case core.EntityCircle:
				gs.circlesOrArcs++
			}
		}
		if s.Edge.A != 0 {
			gs.solidEdge = append(gs.solidEdge, s.Edge)
		}
		if s.Face != 0 {
			gs.solidFace = append(gs.solidFace, s.Face)
		}
		if s.Constraint.V != 0 {
			gs.constraints++
			gs.constraint = append(gs.constraint, s.Constraint)
			c := sk.Constraint(s.Constraint)
			if c.IsStylable() {
				gs.stylables++
			}
			if c.HasLabel() {
				gs.constraintLabels++
			}
		}
	}
}

// ChooseFromHoverToSelect picks what a click selects: the frontmost of
// the hovered items not after the active group.
func (gw *graphicsWindow) ChooseFromHoverToSelect() Selection {
	return gw.chooseFromHover(false)
}

// ChooseFromHoverToDrag is ChooseFromHoverToSelect, less the points
// that can't be dragged.
func (gw *graphicsWindow) ChooseFromHoverToDrag() Selection {
	return gw.chooseFromHover(true)
}

func (gw *graphicsWindow) chooseFromHover(forDrag bool) Selection {
	var sel Selection
	if len(gw.hoverList) == 0 {
		return sel
	}
	sk := gw.sk()
	activeGroup := sk.Group(gw.activeGroupH())
	bestOrder := -1
	bestZIndex := 0
	bestDepth := core.VeryPositive

	for i := range gw.hoverList {
		hov := &gw.hoverList[i]
		var hg core.HGroup
		switch {
		case hov.Selection.Entity.V != 0:
			e := sk.Entity(hov.Selection.Entity)
			if e == nil {
				// The model moved under a hover list built by an
				// earlier hit test; the entity is gone.
				continue
			}
			if forDrag && !gw.entityCanBeDragged(e) {
				continue
			}
			hg = e.Group
		case hov.Selection.Constraint.V != 0:
			c := sk.Constraint(hov.Selection.Constraint)
			if c == nil {
				continue
			}
			hg = c.Group
		default:
			// A solid edge or an unnamed face: no group handle
			// names it, so nothing to order by against the active
			// group.
			continue
		}

		g := sk.Group(hg)
		if g == nil {
			continue
		}
		if g.Order > activeGroup.Order {
			continue
		}
		if bestOrder != -1 && (bestOrder > g.Order || bestZIndex > hov.ZIndex) {
			continue
		}
		// we have hov.zIndex is >= best and hov.group is >= best (but not
		// > active group)
		if hov.Depth > bestDepth && bestOrder == g.Order && bestZIndex == hov.ZIndex {
			continue
		}
		bestOrder = g.Order
		bestZIndex = hov.ZIndex
		if !forDrag {
			// bestDepth is left as it is here.
			bestDepth = hov.Depth
		}
		sel = hov.Selection
	}
	return sel
}

// HitTestMakeSelection finds what is under the mouse at mp and makes
// it the hover.
func (gw *graphicsWindow) HitTestMakeSelection(mp geom.Point2d) {
	sk := gw.sk()
	gw.hoverList = nil
	var sel Selection

	// Did the view projection change? If so, invalidate bounding boxes.
	v := &gw.view
	if !v.Offset.EqualsExactly(gw.cached.offset) ||
		!v.ProjRight.EqualsExactly(gw.cached.projRight) ||
		!v.ProjUp.EqualsExactly(gw.cached.projUp) ||
		v.Scale != gw.cached.scale {
		gw.cached.offset = v.Offset
		gw.cached.projRight = v.ProjRight
		gw.cached.projUp = v.ProjUp
		gw.cached.scale = v.Scale
		gw.invalidateScreenBBoxes()
	}

	canvas := &render.ObjectPicker{
		Cam:       gw.Camera(),
		SelRadius: 10.0,
		Point:     mp,
		MaxZIndex: -1,
	}
	canvas.SetBitmapFont(gw.bitmapFont())

	// Always do the entities; we might be dragging something that should
	// be auto-constrained, and we need the hover for that.
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if !sk.EntityIsVisible(e) {
			continue
		}
		// If faces aren't selectable, image entities aren't either.
		if e.Type == core.EntityImage && !gw.showFaces {
			continue
		}
		// Don't hover whatever's being dragged.
		if gw.IsFromPending(e.H.Request()) {
			// The one exception is when we're creating a new cubic; we
			// want to be able to hover the first point, because that's how
			// we turn it into a periodic spline.
			if !e.IsPoint() || !e.H.IsFromRequest() {
				continue
			}
			r := sk.Request(e.H.Request())
			if r.Type != sketch.RequestCubic || r.ExtraPoints < 2 {
				continue
			}
			if e.H.V != r.H.Entity(1).V {
				continue
			}
		}

		if canvas.Pick(func() { gw.drawEntity(e, entityDrawDefault, canvas) }) {
			gw.hoverList = append(gw.hoverList, Hover{
				Distance:  canvas.MinDistance,
				ZIndex:    canvas.MaxZIndex,
				Depth:     canvas.MinDepth,
				Selection: Selection{Entity: e.H},
			})
		}
	}

	// The constraints and faces happen only when nothing's in progress.
	if gw.pending.operation == pendingNone {
		for i := range sk.Constraints {
			c := &sk.Constraints[i]
			if canvas.Pick(func() { gw.drawConstraint(c, constraintDrawDefault, canvas) }) {
				gw.hoverList = append(gw.hoverList, Hover{
					Distance:  canvas.MinDistance,
					ZIndex:    canvas.MaxZIndex,
					Selection: Selection{Constraint: c.H},
				})
			}
		}
	}

	// std::sort: ties land where libstdc++'s introsort puts them.
	srf.CxxSort(gw.hoverList, func(a, b *Hover) bool {
		if a.ZIndex == b.ZIndex {
			return a.Distance < b.Distance
		}
		return a.ZIndex > b.ZIndex
	})
	sel = gw.ChooseFromHoverToSelect()

	if gw.pending.operation == pendingNone && sel.IsEmpty() && gw.showShaded {
		// Edges of the solid that no entity draws, as a Boolean makes:
		// below entities and constraints, above faces.
		if n, ok := gw.hoverSolidEdge(mp); ok {
			sel = Selection{Edge: n}
			gw.hoverList = append(gw.hoverList, Hover{Selection: sel})
		}
	}

	if gw.pending.operation == pendingNone {
		// Faces, from the triangle mesh; these are lowest priority
		if sel.IsEmpty() && gw.showShaded && gw.showFaces {
			g := sk.Group(gw.activeGroupH())
			m := &g.DisplayMesh
			rayPoint := gw.view.UnProjectPoint3(geom.Vector{X: mp.U, Y: mp.V, Z: 0.0})
			rayDir := gw.view.UnProjectPoint3(geom.Vector{X: mp.U, Y: mp.V, Z: 1.0}).Minus(rayPoint)
			var keep *srf.HalfSpace
			if h, ok := gw.sectionKeep(); ok {
				keep = &h
			}
			if face := m.FirstIntersectionIn(rayPoint, rayDir, keep); face != 0 {
				// A face entity, or for a face no entity names its
				// identity.
				if sk.Entity(core.HEntity{V: face}) != nil {
					sel.Entity.V = face
				} else {
					sel.Face = srf.FaceID(face)
				}
				gw.hoverList = append(gw.hoverList, Hover{Selection: sel})
			}
		}
	}

	if !sel.Equals(&gw.hover) {
		gw.hover = sel
		gw.Invalidate(false)
	}
}

// DrawSnapGrid draws the grid in the active workplane.
func (gw *graphicsWindow) DrawSnapGrid(canvas render.Canvas) {
	if !gw.LockedInWorkplane() {
		return
	}
	sk := gw.sk()
	camera := canvas.Camera()
	width, height := camera.Width, camera.Height
	v := &gw.view

	wrkpl := sk.Entity(gw.ActiveWorkplane())
	norm := wrkpl.NormalEntity(sk)
	n := v.ProjUp.Cross(v.ProjRight)
	wp := sk.Entity(wrkpl.Point[0]).PointGetNum(sk)
	wu, wv, wn := norm.NormalU(sk), norm.NormalV(sk), norm.NormalN(sk)

	g := gw.ss.GridSpacing

	umin, umax := core.VeryPositive, core.VeryNegative
	vmin, vmax := core.VeryPositive, core.VeryNegative
	for a := range 4 {
		// Ideally, we would just do +/- half the width and height; but
		// allow some extra slop for rounding.
		horiz := v.ProjRight.ScaledBy((0.6*width)/v.Scale + 2*g)
		vert := v.ProjUp.ScaledBy((0.6*height)/v.Scale + 2*g)
		if a == 2 || a == 3 {
			horiz = horiz.ScaledBy(-1)
		}
		if a == 1 || a == 3 {
			vert = vert.ScaledBy(-1)
		}
		tp := horiz.Plus(vert).Minus(v.Offset)

		// Project the point into our grid plane, normal to the screen
		// (not to the grid plane). If the plane is on edge then this is
		// impossible so don't try to draw the grid.
		tpp, parallel := geom.AtIntersectionOfPlaneAndLineParallel(wn, wn.Dot(wp), tp, tp.Plus(n))
		if parallel {
			return
		}
		tpp = tpp.Minus(wp)
		uu, vv := tpp.Dot(wu), tpp.Dot(wv)

		umin = geom.Min(uu, umin)
		umax = geom.Max(uu, umax)
		vmin = geom.Min(vv, vmin)
		vmax = geom.Max(vv, vmax)
	}

	i0, i1 := int(umin/g), int(umax/g)
	j0, j1 := int(vmin/g), int(vmax/g)

	if i0 > i1 || i1-i0 > 400 {
		return
	}
	if j0 > j1 || j1-j0 > 400 {
		return
	}

	hcs := canvas.InternStroke(render.Stroke{
		Layer: render.LayerBack,
		Color: gw.ss.styleColor(hstyle(core.StyleDatum)).WithAlpha(75),
		Unit:  render.UnitPX,
		Width: 1.0,
	})
	for i := i0 + 1; i < i1; i++ {
		canvas.DrawLine(wp.Plus(wu.ScaledBy(float64(i)*g)).Plus(wv.ScaledBy(float64(j0)*g)),
			wp.Plus(wu.ScaledBy(float64(i)*g)).Plus(wv.ScaledBy(float64(j1)*g)), hcs)
	}
	for j := j0 + 1; j < j1; j++ {
		canvas.DrawLine(wp.Plus(wu.ScaledBy(float64(i0)*g)).Plus(wv.ScaledBy(float64(j)*g)),
			wp.Plus(wu.ScaledBy(float64(i1)*g)).Plus(wv.ScaledBy(float64(j)*g)), hcs)
	}
}

// DrawEntities draws the entities that change with the view (or, with
// persistent, those that don't: normals and workplanes).
func (gw *graphicsWindow) DrawEntities(canvas render.Canvas, persistent bool) {
	sk := gw.sk()
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if persistent == (e.IsNormal() || e.IsWorkplane()) {
			continue
		}
		switch gw.drawOccludedAs {
		case occludedVisible:
			gw.drawEntity(e, entityDrawOverlay, canvas)
		case occludedStippled:
			gw.drawEntity(e, entityDrawHidden, canvas)
			gw.drawEntity(e, entityDrawDefault, canvas)
		case occludedInvisible:
			gw.drawEntity(e, entityDrawDefault, canvas)
		}
	}
}

// DrawPersistent draws what doesn't change with the view: the active
// group's mesh and edges, and the filled paths.
func (gw *graphicsWindow) DrawPersistent(canvas render.Canvas) {
	sk := gw.sk()
	// Draw the active group; this does stuff like the mesh and edges.
	gw.drawGroup(sk.Group(gw.activeGroupH()), canvas)

	// Now draw the entities that don't change with viewport.
	gw.DrawEntities(canvas, true)

	// Draw filled paths in all groups, when those filled paths were
	// requested specially by assigning a style with a fill color, or when
	// the filled paths are just being filled by default. This should go
	// last, to make the transparency work.
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		if !g.IsVisible() {
			continue
		}
		gw.drawFilledPaths(g, canvas)
	}
}

// Draw draws the model, constraints and overlays.
func (gw *graphicsWindow) Draw(canvas render.Canvas) {
	ss := gw.ss
	sk := gw.sk()
	camera := canvas.Camera()

	// Nothing draws a sketch that is still loading.
	if !ss.AllConsistent {
		return
	}
	// The drawing sheet in place of the model (roadmap 29.1).
	if gw.sheetShown {
		gw.drawSheet(canvas)
		return
	}

	// The snap grid, and all the things that don't change when we
	// rotate.
	gw.drawPersistentLayer(canvas)

	// Draw the entities that do change with viewport.
	gw.DrawEntities(canvas, false)

	// Draw the polygon errors.
	if ss.CheckClosedContour {
		gw.drawPolyError(sk.Group(gw.activeGroupH()), canvas)
	}

	// Draw the constraints
	for i := range sk.Constraints {
		gw.drawConstraint(&sk.Constraints[i], constraintDrawDefault, canvas)
	}

	// Draw areas
	if ss.ShowContourAreas {
		for _, hg := range sk.GroupOrder {
			g := sk.Group(hg)
			if g.H != gw.activeGroupH() || !g.IsVisible() {
				continue
			}
			gw.drawContourAreaLabels(g, canvas)
		}
	}

	// Draw the "pending" constraint, i.e. a constraint that would be
	// placed on a line that is almost horizontal or vertical.
	if (gw.pending.operation == pendingDraggingNewLinePoint || gw.pending.operation == pendingDraggingNewArcPoint ||
		gw.pending.operation == pendingDraggingNewRadius) && gw.pending.hasSuggestion {
		var c core.Constraint
		c.Group = gw.activeGroupH()
		c.Workplane = gw.ActiveWorkplane()
		c.Type = gw.pending.suggestion.Type
		c.EntityA, c.EntityB = gw.pending.suggestion.A, gw.pending.suggestion.B
		c.Other, c.Other2 = gw.pending.suggestion.Other, gw.pending.suggestion.Other2
		gw.drawConstraint(&c, constraintDrawDefault, canvas)
	}

	strokeAnalyze := ss.styleStroke(hstyle(core.StyleAnalyze))
	strokeAnalyze.Layer = render.LayerFront
	hcsAnalyze := canvas.InternStroke(strokeAnalyze)

	// Draw the traced path, if one exists
	var tracedEdges srf.SEdgeList
	ss.GW.traced.Path.MakeEdgesInto(&tracedEdges)
	canvas.DrawEdges(&tracedEdges, hcsAnalyze)

	strokeError := ss.styleStroke(hstyle(core.StyleDrawError))
	strokeError.Layer = render.LayerFront
	strokeError.Width = 12
	hcsError := canvas.InternStroke(strokeError)

	// And the naked edges, if the user did Analyze -> Show Naked Edges.
	canvas.DrawEdges(&ss.GW.nakedEdges, hcsError)

	// Then redraw whatever the mouse is hovering over, highlighted.
	gw.drawSelection(&gw.hover, true, canvas)
	gw.drawMesh(sk.Group(gw.activeGroupH()), drawMeshHovered, canvas)

	// And finally draw the selection, same mechanism.
	for i := range gw.selection {
		gw.drawSelection(&gw.selection[i], false, canvas)
	}
	gw.drawMesh(sk.Group(gw.activeGroupH()), drawMeshSelected, canvas)

	strokeDatum := ss.styleStroke(hstyle(core.StyleDatum))
	strokeDatum.Unit = render.UnitPX
	strokeDatum.Layer = render.LayerFront
	strokeDatum.Width = 1
	hcsDatum := canvas.InternStroke(strokeDatum)

	// An extra line, used to indicate the origin when rotating within the
	// plane of the monitor.
	if ss.GW.extraLine.Draw {
		canvas.DrawLine(ss.GW.extraLine.PtA, ss.GW.extraLine.PtB, hcsDatum)
	}

	scale := gw.view.Scale
	if label, at, ok := ss.measureLabel(); ok {
		h := ss.defaultTextHeight() / camera.Scale
		canvas.DrawVectorText(label, h,
			at.Plus(camera.ProjRight.ScaledBy(10/scale)).Plus(camera.ProjUp.ScaledBy(10/scale)),
			camera.ProjRight, camera.ProjUp, hcsDatum)
	}

	if ss.GW.centerOfMass.Draw && !ss.GW.centerOfMass.Dirty {
		p := ss.GW.centerOfMass.Position
		u := camera.ProjRight
		v := camera.ProjUp

		const size = 10.0
		const subdiv = 16
		h := ss.defaultTextHeight() / camera.Scale
		s := ss.MmToStringSI(p.X, 0) + ", " + ss.MmToStringSI(p.Y, 0) + ", " + ss.MmToStringSI(p.Z, 0)
		canvas.DrawVectorText(s, h,
			p.Plus(u.ScaledBy((size+5.0)/scale)).Minus(v.ScaledBy(h/2.0)),
			u, v, hcsDatum)
		u = u.WithMagnitude(size / scale)
		v = v.WithMagnitude(size / scale)

		canvas.DrawLine(p.Minus(u), p.Plus(u), hcsDatum)
		canvas.DrawLine(p.Minus(v), p.Plus(v), hcsDatum)
		var prev geom.Vector
		for i := 0; i <= subdiv; i++ {
			a := float64(i) / subdiv * 2.0 * math.Pi
			point := p.Plus(u.ScaledBy(libm.Cos(a))).Plus(v.ScaledBy(libm.Sin(a)))
			if i > 0 {
				canvas.DrawLine(point, prev, hcsDatum)
			}
			prev = point
		}
	}

	// A note to indicate the origin in the just-exported file.
	if ss.GW.justExported.Draw {
		var p, u, v geom.Vector
		if ss.GW.justExported.ShowOrigin {
			p, u, v = ss.GW.justExported.Pt, ss.GW.justExported.U, ss.GW.justExported.V
		} else {
			p = camera.Offset.ScaledBy(-1)
			u = camera.ProjRight
			v = camera.ProjUp
		}
		canvas.DrawVectorText("previewing exported geometry; press Esc to return",
			ss.defaultTextHeight()/camera.Scale,
			p.Plus(u.ScaledBy(10/scale)).Plus(v.ScaledBy(10/scale)), u, v, hcsDatum)

		if ss.GW.justExported.ShowOrigin {
			um := p.Plus(u.WithMagnitude(-15 / scale))
			up := p.Plus(u.WithMagnitude(30 / scale))
			vm := p.Plus(v.WithMagnitude(-15 / scale))
			vp := p.Plus(v.WithMagnitude(30 / scale))
			canvas.DrawLine(um, up, hcsDatum)
			canvas.DrawLine(vm, vp, hcsDatum)
			canvas.DrawVectorText("(x, y) = (0, 0) for file just exported",
				ss.defaultTextHeight()/camera.Scale,
				p.Plus(u.ScaledBy(40/scale)).Plus(v.ScaledBy(-ss.defaultTextHeight()/scale)), u, v,
				hcsDatum)
		}
	}
}

// Paint renders the window.
func (gw *graphicsWindow) Paint() {
	ss := gw.ss
	if gw.window == nil || gw.canvas == nil {
		panic("ui: cannot paint without window and canvas")
	}
	if ss.lastGeneration != ss.SK.Generation {
		// The engine regenerated outside GenerateAll's UI wrapper.
		ss.lastGeneration = ss.SK.Generation
		gw.persistentDirty = true
		ss.GW.centerOfMass.Dirty = true
	}

	gw.havePainted = true
	gw.syncSectionMenu()

	camera := gw.Camera()
	lighting := gw.Lighting()

	if !ss.ActiveGroupsOkay() {
		// Draw a different background whenever we're having solve
		// problems.
		bgColor := ss.styleColor(hstyle(core.StyleDrawError))
		bgColor = core.RGBAFromFloat(0.4*bgColor.RedF(), 0.4*bgColor.GreenF(), 0.4*bgColor.BlueF(), 1.0)
		lighting.BackgroundColor = bgColor
		// And show the text window, which has info to debug it
		gw.ForceTextWindowShown()
	}

	canvas := gw.canvas
	gw.shareBitmapFont(canvas)
	gw.lighting = lighting
	canvas.SetLighting(lighting)
	canvas.SetCamera(camera)
	canvas.StartFrame()

	// Draw the 3d objects.
	gw.Draw(canvas)
	canvas.FlushFrame()

	// Draw the 2d UI overlay.
	camera = render.CameraForPixels(camera)
	canvas.SetCamera(camera)

	uiCanvas := &render.UICanvas{Canvas: canvas}

	// If a marquee selection is in progress, then draw the selection
	// rectangle, as an outline and a transparent fill: in the hovered
	// colour for one that selects what lies wholly inside it, in the
	// selected colour for one that selects what it crosses.
	if gw.pending.operation == pendingDraggingMarquee {
		begin := gw.ProjectPoint(gw.orig.marqueePoint)
		color := ss.styleColor(hstyle(core.StyleHovered))
		if !gw.marqueeWindow() {
			color = ss.styleColor(hstyle(core.StyleSelected))
		}
		uiCanvas.DrawRect(int(gw.orig.mouse.U)+int(camera.Width)/2,
			int(begin.U)+int(camera.Width)/2,
			int(gw.orig.mouse.V)+int(camera.Height)/2,
			int(begin.V)+int(camera.Height)/2,
			color.WithAlpha(25), color, 0)
	}

	// If we've had a screenshot requested, take it now, before the UI is
	// overlaid.
	if ss.ScreenshotFile != "" {
		if err := writeFramePng(canvas.ReadFrame(), ss.ScreenshotFile); err != nil {
			ss.Error("Couldn't write to '%s'", ss.ScreenshotFile)
		}
		ss.ScreenshotFile = ""
	}

	// And finally the toolbar.
	if ss.ShowToolbar {
		canvas.SetCamera(camera)
		gw.ToolbarDraw(uiCanvas)
	}

	canvas.FlushFrame()
	canvas.FinishFrame()
	gw.window.PresentFrame(canvas)
	canvas.Clear()
}

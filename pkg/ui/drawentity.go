package ui

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// entityDrawCache holds what C++ caches on an Entity for drawing: its
// Beziers, their piecewise linear edges, and its bounding box on screen.
type entityDrawCache struct {
	beziers       srf.SBezierList
	beziersValid  bool
	edges         srf.SEdgeList
	edgesChordTol float64
	edgesValid    bool

	screenBBox      core.BBox
	screenBBoxValid bool
}

// EntityDrawAs is how an entity is drawn (C++ Entity::DrawAs).
type EntityDrawAs int

const (
	EntityDrawDefault EntityDrawAs = iota
	EntityDrawOverlay
	EntityDrawHidden
	EntityDrawHovered
	EntityDrawSelected
)

// entityCache returns e's drawing cache; a regeneration, or another
// sketch, drops them all, as C++ makes the entities anew.
func (gw *GraphicsWindow) entityCache(e *core.Entity) *entityDrawCache {
	if sk := gw.sk(); gw.entityCaches == nil || gw.cacheGen != sk.Generation || gw.cacheSketch != sk {
		gw.entityCaches = make(map[core.HEntity]*entityDrawCache)
		gw.cacheGen = sk.Generation
		gw.cacheSketch = sk
	}
	c := gw.entityCaches[e.H]
	if c == nil {
		c = &entityDrawCache{}
		gw.entityCaches[e.H] = c
	}
	return c
}

// entityDescription is C++ Entity::DescriptionString.
func (ss *Application) entityDescription(e *core.Entity) string {
	if e.H.IsFromRequest() {
		return ss.SK.GetRequest(e.H.Request()).DescriptionString()
	}
	return groupDescription(ss.SK.GetGroup(e.H.Group()))
}

// groupDescription is C++ Group::DescriptionString.
func groupDescription(g *sketch.Group) string {
	name := g.Name
	if name == "" {
		name = "(unnamed)"
	}
	return fmt.Sprintf("g%03x-%s", g.H.V, name)
}

// getOrGenerateBezierCurves is Entity::GetOrGenerateBezierCurves.
func (gw *GraphicsWindow) getOrGenerateBezierCurves(e *core.Entity) *srf.SBezierList {
	c := gw.entityCache(e)
	if !c.beziersValid {
		c.beziers = srf.SBezierList{}
		sketch.GenerateEntityBeziers(e, &c.beziers, &sketch.Runtime{Sketch: gw.sk()})
		c.beziersValid = true
	}
	return &c.beziers
}

// generateEdges is Entity::GenerateEdges.
func (gw *GraphicsWindow) generateEdges(e *core.Entity, el *srf.SEdgeList) {
	sbl := gw.getOrGenerateBezierCurves(e)
	style := gw.sk().StyleForEntity(e.H)
	for i := range sbl.Curves {
		sb := &sbl.Curves[i]
		var lv []geom.Vector
		sb.MakePwlIntoPoints(&lv, 0, 0)
		for j := 1; j < len(lv); j++ {
			el.AddEdge(lv[j-1], lv[j], int(style.V), i, 0)
		}
	}
}

// getOrGenerateEdges is Entity::GetOrGenerateEdges: the edges, made
// again if the chord tolerance changed.
func (gw *GraphicsWindow) getOrGenerateEdges(e *core.Entity) *srf.SEdgeList {
	c := gw.entityCache(e)
	chordTol := gw.ss.ChordTolMm()
	if c.edgesValid && c.edgesChordTol == chordTol {
		return &c.edges
	}
	c.edges = srf.SEdgeList{}
	gw.generateEdges(e, &c.edges)
	c.edgesValid = true
	c.edgesChordTol = chordTol
	return &c.edges
}

// getOrGenerateScreenBBox is Entity::GetOrGenerateScreenBBox; ok is
// false for entities without one, like workplanes.
func (gw *GraphicsWindow) getOrGenerateScreenBBox(e *core.Entity) (core.BBox, bool) {
	sk := gw.sk()
	sbl := gw.getOrGenerateBezierCurves(e)

	// We don't bother with bounding boxes for workplanes, etc.
	if !(e.IsPoint() || e.IsNormal() || len(sbl.Curves) > 0) {
		return core.BBox{}, false
	}
	c := gw.entityCache(e)
	if c.screenBBoxValid {
		return c.screenBBox, true
	}
	switch {
	case e.IsPoint():
		proj := gw.ProjectPoint3(e.PointGetNum(sk))
		c.screenBBox = core.NewBBox(proj, proj)
	case e.IsNormal():
		proj := gw.ProjectPoint3(sk.GetEntity(e.Point[0]).PointGetNum(sk))
		c.screenBBox = core.NewBBox(proj, proj)
	default:
		first := gw.ProjectPoint3(sbl.Curves[0].Ctrl[0])
		c.screenBBox = core.NewBBox(first, first)
		for _, sb := range sbl.Curves {
			for i := 0; i <= sb.Degree; i++ {
				c.screenBBox.Include(gw.ProjectPoint3(sb.Ctrl[i]), 0)
			}
		}
	}
	c.screenBBoxValid = true
	return c.screenBBox, true
}

// invalidateScreenBBoxes forgets the entities' screen bounding boxes,
// after the view changed.
func (gw *GraphicsWindow) invalidateScreenBBoxes() {
	for _, c := range gw.entityCaches {
		c.screenBBoxValid = false
	}
}

// entityReferencePoints is Entity::GetReferencePoints.
func (gw *GraphicsWindow) entityReferencePoints(e *core.Entity) []geom.Vector {
	sk := gw.sk()
	switch e.Type {
	case core.EntityPointNCopy, core.EntityPointNTrans, core.EntityPointNRotTrans,
		core.EntityPointNRotAA, core.EntityPointNRotAxisTrans, core.EntityPointIn3D,
		core.EntityPointIn2D:
		return []geom.Vector{gw.pointGetDrawNum(e)}

	case core.EntityNormalNCopy, core.EntityNormalNRot, core.EntityNormalNRotAA,
		core.EntityNormalIn3D, core.EntityNormalIn2D, core.EntityWorkplane,
		core.EntityCircle, core.EntityArcOfCircle, core.EntityCubic,
		core.EntityCubicPeriodic, core.EntityTTFText, core.EntityImage:
		return []geom.Vector{gw.pointGetDrawNum(sk.GetEntity(e.Point[0]))}

	case core.EntityLineSegment:
		a := gw.pointGetDrawNum(sk.GetEntity(e.Point[0]))
		b := gw.pointGetDrawNum(sk.GetEntity(e.Point[1]))
		return []geom.Vector{b.Plus(a.Minus(b).ScaledBy(0.5))}
	}
	return nil
}

// entityPositionOfPoint is Entity::GetPositionOfPoint: which of the
// entity's edges is nearest p.
func (gw *GraphicsWindow) entityPositionOfPoint(e *core.Entity, camera *render.Camera, p geom.Point2d) int {
	op := &render.ObjectPicker{Camera: *camera, Point: p, MinDistance: 1e12}
	gw.drawEntity(e, EntityDrawDefault, op)
	return int(op.Position)
}

// entityIsStylable is Entity::IsStylable.
func entityIsStylable(e *core.Entity) bool {
	return !e.IsPoint() && !e.IsWorkplane() && !e.IsNormal()
}

func (gw *GraphicsWindow) ptCanDrag(pt core.HEntity) bool {
	p := gw.sk().GetEntity(pt)
	// a numeric copy can not move
	if p.Type == core.EntityPointNCopy {
		return false
	}
	// these transforms applied zero times can not be moved
	if (p.Type == core.EntityPointNTrans || p.Type == core.EntityPointNRotAA ||
		p.Type == core.EntityPointNRotAxisTrans) && p.TimesApplied == 0 {
		return false
	}
	return true
}

// entityCanBeDragged is Entity::CanBeDragged: entities made by some
// copies can't be dragged with the mouse.
func (gw *GraphicsWindow) entityCanBeDragged(e *core.Entity) bool {
	sk := gw.sk()
	if e.IsPoint() {
		if !gw.ptCanDrag(e.H) {
			return false
		}
		// are we constrained pt-on-point from a previous group?
		for i := range sk.Constraints {
			cc := &sk.Constraints[i]
			if cc.Group == e.Group && cc.Type == core.ConstraintPointsCoincident {
				if cc.PtA == e.H {
					if sk.GetEntity(cc.PtB).Group.V < e.Group.V || !gw.ptCanDrag(cc.PtB) {
						return false
					}
				}
				if cc.PtB == e.H {
					if sk.GetEntity(cc.PtA).Group.V < e.Group.V || !gw.ptCanDrag(cc.PtA) {
						return false
					}
				}
			}
		}
	}
	// for these types of entities the first point will indicate
	// draggability
	if e.HasEndpoints() || e.Type == core.EntityCircle {
		return gw.ptCanDrag(e.Point[0])
	}
	// if we're not certain it can't be dragged then default to true
	return true
}

// groupShouldDrawExploded is Group::ShouldDrawExploded.
func (gw *GraphicsWindow) groupShouldDrawExploded(g *sketch.Group) bool {
	return gw.ss.Explode && g.H == gw.activeGroupH() && g.Type == sketch.GroupDrawingWorkplane && !gw.ss.ExportMode
}

func (gw *GraphicsWindow) entityShouldDrawExploded(e *core.Entity) bool {
	return gw.groupShouldDrawExploded(gw.sk().GetGroup(e.Group))
}

// entityExplodeOffset is Entity::ExplodeOffset.
func (gw *GraphicsWindow) entityExplodeOffset(e *core.Entity) geom.Vector {
	sk := gw.sk()
	if gw.entityShouldDrawExploded(e) && e.Workplane.V != 0 {
		requestIdx := sk.GetRequest(e.H.Request()).GroupRequestIndex
		offset := gw.ss.ExplodeDistance * float64(requestIdx+1)
		return sk.GetEntity(e.Workplane).NormalEntity(sk).NormalN(sk).ScaledBy(offset)
	}
	return geom.Vector{}
}

// pointGetDrawNum is Entity::PointGetDrawNum: where a point is drawn,
// which in an exploded view isn't where it is.
func (gw *GraphicsWindow) pointGetDrawNum(e *core.Entity) geom.Vector {
	return e.PointGetNum(gw.sk()).Plus(gw.entityExplodeOffset(e))
}

// drawEntity is Entity::Draw.
func (gw *GraphicsWindow) drawEntity(e *core.Entity, how EntityDrawAs, canvas render.Canvas) {
	ss := gw.ss
	sk := gw.sk()
	if !(how == EntityDrawHovered || how == EntityDrawSelected) && !sk.EntityIsVisible(e) {
		return
	}

	var zIndex int
	switch {
	case e.IsPoint():
		zIndex = 6
	case how == EntityDrawHidden:
		zIndex = 2
	case e.Group != gw.activeGroupH():
		zIndex = 3
	default:
		zIndex = 5
	}

	var hs core.HStyle
	switch {
	case e.IsPoint():
		hs = hstyle(core.StyleDatum)
	case e.IsNormal() || e.Type == core.EntityWorkplane:
		hs = hstyle(core.StyleNormals)
	default:
		hs = sk.StyleForEntity(e.H)
		if hs.V == core.StyleConstruction {
			zIndex = 4
		}
	}

	stroke := ss.styleStroke(hs)
	if (how == EntityDrawDefault || how == EntityDrawOverlay) && !e.IsNormal() &&
		e.Type != core.EntityWorkplane && e.Group == gw.activeGroupH() {
		if c, ok := ss.freedomColor(e); ok {
			stroke.Color = c
		}
	}
	switch how {
	case EntityDrawDefault:
		stroke.Layer = render.LayerNormal
	case EntityDrawOverlay:
		stroke.Layer = render.LayerFront
	case EntityDrawHidden:
		stroke.Layer = render.LayerOccluded
		stroke.StipplePattern = sk.StylePatternType(hstyle(core.StyleHiddenEdge))
		stroke.StippleScale = sk.StyleGet(hstyle(core.StyleHiddenEdge)).StippleScale
	case EntityDrawHovered:
		stroke.Layer = render.LayerFront
		stroke.Color = ss.styleColor(hstyle(core.StyleHovered))
	case EntityDrawSelected:
		stroke.Layer = render.LayerFront
		stroke.Color = ss.styleColor(hstyle(core.StyleSelected))
	}
	stroke.ZIndex = zIndex
	hcs := canvas.GetStroke(stroke)

	switch e.Type {
	case core.EntityPointNCopy, core.EntityPointNTrans, core.EntityPointNRotTrans,
		core.EntityPointNRotAA, core.EntityPointNRotAxisTrans, core.EntityPointIn3D,
		core.EntityPointIn2D:
		if how == EntityDrawHidden {
			return
		}
		// If we're analyzing the sketch to show the degrees of freedom,
		// then we draw big colored squares over the points that are free
		// to move.
		free := false
		switch e.Type {
		case core.EntityPointIn3D:
			px, py, pz := sk.GetParam(e.Param[0]), sk.GetParam(e.Param[1]), sk.GetParam(e.Param[2])
			free = px.Free || py.Free || pz.Free
		case core.EntityPointIn2D:
			pu, pv := sk.GetParam(e.Param[0]), sk.GetParam(e.Param[1])
			free = pu.Free || pv.Free
		}

		pointStroke := render.Stroke{
			Layer:  stroke.Layer,
			ZIndex: stroke.ZIndex,
			Color:  stroke.Color,
			Width:  7.0,
			Unit:   render.UnitPX,
		}
		if free {
			pointStroke.Layer = render.LayerFront
		}
		hcsPoint := canvas.GetStroke(pointStroke)

		p := gw.pointGetDrawNum(e)
		if free {
			analyzeStroke := ss.styleStroke(hstyle(core.StyleAnalyze))
			analyzeStroke.Width = 14.0
			analyzeStroke.Layer = render.LayerFront
			canvas.DrawPoint(p, canvas.GetStroke(analyzeStroke))
		}
		canvas.DrawPoint(p, hcsPoint)
		return

	case core.EntityNormalNCopy, core.EntityNormalNRot, core.EntityNormalNRotAA,
		core.EntityNormalIn3D, core.EntityNormalIn2D:
		camera := canvas.GetCamera()
		if how == EntityDrawHidden {
			return
		}
		for i := range 2 {
			asReference := i == 1
			if asReference {
				if !e.H.Request().IsFromReferences() {
					continue
				}
			} else {
				if !sk.GetGroup(e.Group).IsVisible() || !gw.show.Normals {
					continue
				}
			}

			if asReference {
				stroke.Layer = render.LayerFront
			} else {
				stroke.Layer = render.LayerNormal
			}
			if how != EntityDrawHovered && how != EntityDrawSelected {
				// Always draw the x, y, and z axes in red, green, and
				// blue; brighter for the ones at the bottom left of the
				// screen, dimmer for the ones at the model origin.
				hr := e.H.Request()
				luma := uint8(100)
				if asReference {
					luma = 255
				}
				switch hr {
				case sketch.HRequestReferenceXY:
					stroke.Color = rgbi(0, 0, luma)
				case sketch.HRequestReferenceYZ:
					stroke.Color = rgbi(luma, 0, 0)
				case sketch.HRequestReferenceZX:
					stroke.Color = rgbi(0, luma, 0)
				}
			}

			q := e.NormalGetNum(sk)
			var tail geom.Vector
			if asReference {
				// Draw an extra copy of the x, y, and z axes, that's
				// always in the corner of the view and at the front.
				stroke.Width = 2
				s := camera.Scale
				h := 60 - camera.Height/2.0
				w := 60 - camera.Width/2.0
				// Shift the axis to the right if they would overlap with
				// the toolbar.
				if ss.ShowToolbar {
					if h+30 > -float64(32*18+3*16+8)/2 {
						w += 60
					}
				}
				tail = camera.ProjRight.ScaledBy(w / s).Plus(
					camera.ProjUp.ScaledBy(h / s)).Minus(camera.Offset)
			} else {
				tail = gw.pointGetDrawNum(sk.GetEntity(e.Point[0]))
			}
			tail = camera.AlignToPixelGrid(tail)

			hcs = canvas.GetStroke(stroke)
			v := q.RotationN().WithMagnitude(50.0 / camera.Scale)
			tip := tail.Plus(v)
			canvas.DrawLine(tail, tip, hcs)

			v = v.WithMagnitude(12.0 / camera.Scale)
			axis := q.RotationV()
			canvas.DrawLine(tip, tip.Minus(v.RotatedAroundAxis(axis, 0.6)), hcs)
			canvas.DrawLine(tip, tip.Minus(v.RotatedAroundAxis(axis, -0.6)), hcs)

			if e.Type == core.EntityNormalIn3D {
				nw, nx := sk.GetParam(e.Param[0]), sk.GetParam(e.Param[1])
				ny, nz := sk.GetParam(e.Param[2]), sk.GetParam(e.Param[3])
				if nw.Free || nx.Free || ny.Free || nz.Free {
					analyzeStroke := ss.styleStroke(hstyle(core.StyleAnalyze))
					analyzeStroke.Layer = render.LayerFront
					canvas.DrawLine(tail, tip, canvas.GetStroke(analyzeStroke))
				}
			}
		}
		return

	case core.EntityDistance, core.EntityDistanceNCopy:
		// These are used only as data structures, nothing to display.
		return

	case core.EntityWorkplane:
		camera := canvas.GetCamera()
		p := sk.GetEntity(e.Point[0]).PointGetNum(sk)
		p = camera.AlignToPixelGrid(p)

		n := e.NormalEntity(sk)
		u, v := n.NormalU(sk), n.NormalV(sk)

		s := geom.Min(camera.Width, camera.Height) * 0.45 / camera.Scale

		us, vs := u.ScaledBy(s), v.ScaledBy(s)

		pp := p.Plus(us).Plus(vs)
		pm := p.Plus(us).Minus(vs)
		mm := p.Minus(us).Minus(vs)
		mm2 := mm
		mp := p.Minus(us).Plus(vs)

		strokeBorder := stroke
		strokeBorder.ZIndex -= 3
		strokeBorder.StipplePattern = core.StippleShortDash
		strokeBorder.StippleScale = 8.0
		hcsBorder := canvas.GetStroke(strokeBorder)

		textHeight := ss.styleTextHeight(hs) / camera.Scale

		if !e.H.IsFromRequest() {
			mm = mm.Plus(v.ScaledBy(textHeight * 4.7))
			mm2 = mm2.Plus(u.ScaledBy(textHeight * 4.7))
			canvas.DrawLine(mm2, mm, hcsBorder)
		}
		canvas.DrawLine(pp, pm, hcsBorder)
		canvas.DrawLine(mm2, pm, hcsBorder)
		canvas.DrawLine(mm, mp, hcsBorder)
		canvas.DrawLine(pp, mp, hcsBorder)

		o := mm2.Plus(u.ScaledBy(3.0 / camera.Scale)).Plus(v.ScaledBy(3.0 / camera.Scale))
		shortDesc := ss.entityDescription(e)
		if len(shortDesc) >= 5 {
			shortDesc = shortDesc[5:]
		}
		canvas.DrawVectorText(shortDesc, textHeight, o, u, v, hcs)
		return

	case core.EntityLineSegment, core.EntityCircle, core.EntityArcOfCircle,
		core.EntityCubic, core.EntityCubicPeriodic, core.EntityTTFText:
		// Generate the rational polynomial curves, then piecewise
		// linearize them, and display those.
		shouldExplode := gw.entityShouldDrawExploded(e)
		var explodeOffset geom.Vector
		beziers := gw.getOrGenerateBezierCurves(e)
		if shouldExplode {
			explodeOffset = gw.entityExplodeOffset(e)
			var offsetBeziers srf.SBezierList
			for _, b := range beziers.Curves {
				b.TransformedBy(explodeOffset, geom.Quaternion{W: 1}, 1.0)
				offsetBeziers.Curves = append(offsetBeziers.Curves, b)
			}
			beziers = &offsetBeziers
		}

		var edges *srf.SEdgeList
		if !canvas.DrawBeziers(beziers, hcs) {
			edges = gw.getOrGenerateEdges(e)
			if shouldExplode {
				var offsetEdges srf.SEdgeList
				for _, ed := range edges.Edges {
					offsetEdges.AddEdge(ed.A.Plus(explodeOffset), ed.B.Plus(explodeOffset), ed.AuxA, ed.AuxB, ed.Tag)
				}
				edges = &offsetEdges
			}
			canvas.DrawEdges(edges, hcs)
		}
		if e.Type == core.EntityCircle {
			dist := sk.GetEntity(e.Distance)
			if dist.Type == core.EntityDistance {
				p := sk.GetParam(dist.Param[0])
				if p.Free {
					analyzeStroke := ss.styleStroke(hstyle(core.StyleAnalyze))
					analyzeStroke.Layer = render.LayerFront
					hcsAnalyze := canvas.GetStroke(analyzeStroke)
					if !canvas.DrawBeziers(beziers, hcsAnalyze) {
						canvas.DrawEdges(edges, hcsAnalyze)
					}
				}
			}
		}
		return

	case core.EntityImage:
		var fill render.Fill
		var pixmap *res.Pixmap
		switch how {
		case EntityDrawHidden:
			return
		case EntityDrawHovered:
			fill.Color = ss.styleColor(hstyle(core.StyleHovered)).WithAlpha(180)
			fill.Pattern = render.FillCheckeredA
			fill.ZIndex = 2
		case EntityDrawSelected:
			fill.Color = ss.styleColor(hstyle(core.StyleSelected)).WithAlpha(180)
			fill.Pattern = render.FillCheckeredB
			fill.ZIndex = 1
		default:
			fill.Color = core.RgbaColorFromFloat(1.0, 1.0, 1.0, 1.0)
			pixmap = ss.imagePixmap(e.File)
		}

		hf := canvas.GetFill(fill)
		var v [4]geom.Vector
		for i := range 4 {
			v[i] = gw.pointGetDrawNum(sk.GetEntity(e.Point[i]))
		}
		iu := v[3].Minus(v[0])
		iv := v[1].Minus(v[0])

		if how == EntityDrawDefault && pixmap == nil {
			errStroke := ss.styleStroke(hstyle(core.StyleDrawError))
			errStroke.Color = errStroke.Color.WithAlpha(50)
			hsErr := canvas.GetStroke(errStroke)
			canvas.DrawLine(v[0], v[2], hsErr)
			canvas.DrawLine(v[1], v[3], hsErr)
			for i := range 4 {
				canvas.DrawLine(v[i], v[(i+1)%4], hsErr)
			}
		} else {
			canvas.DrawPixmap(pixmap, v[0], iu, iv,
				geom.Point2d{U: 0.0, V: 0.0}, geom.Point2d{U: 1.0, V: 1.0}, hf)
		}
		// C++ falls through into the face cases here, which do nothing.
		return

	case core.EntityFaceNormalPt, core.EntityFaceXProd, core.EntityFaceNRotTrans,
		core.EntityFaceNTrans, core.EntityFaceNRotAA, core.EntityFaceRotNormalPt,
		core.EntityFaceNRotAxisTrans:
		// Do nothing; these are drawn with the triangle mesh
		return
	}
	panic("ui: unexpected entity type")
}

// The colours of entities their constraints don't fix (an extension):
// one some of whose parameters can move, and one that nothing holds.
var (
	colorPartlyFree = rgbi(100, 180, 255)
	colorFree       = rgbi(255, 160, 70)
)

// freedomColor is the colour for an entity of the active group by how
// much of it its constraints fix, when the setting is on and the
// entity is not fixed.
func (ss *Application) freedomColor(e *core.Entity) (core.RgbaColor, bool) {
	if !ss.ColorByFreedom {
		return core.RgbaColor{}, false
	}
	switch ss.SK.EntityFreedom(e.H) {
	case sketch.FreedomPartly:
		return colorPartlyFree, true
	case sketch.FreedomFree:
		return colorFree, true
	}
	return core.RgbaColor{}, false
}

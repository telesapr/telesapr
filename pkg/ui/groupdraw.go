package ui

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// drawMeshAs is how a group's mesh is drawn.
type drawMeshAs int

const (
	drawMeshDefault drawMeshAs = iota
	drawMeshHovered
	drawMeshSelected
)

// groupActivate makes g the group the user works in.
func (ss *Application) groupActivate(g *sketch.Group) {
	gw := ss.GW
	if g.Type == sketch.GroupDrawingWorkplane || g.Type == sketch.GroupDrawing3D {
		gw.showFaces = gw.showFacesDrawing
	} else {
		gw.showFaces = gw.showFacesNonDrawing
	}
	ss.SK.MarkGroupDirty(g.H, false) // for good measure; shouldn't be needed
	ss.ScheduleShowTW()
}

// drawMesh is Group::DrawMesh.
func (gw *graphicsWindow) drawMesh(g *sketch.Group, how drawMeshAs, canvas render.Canvas) {
	ss := gw.ss
	sk := gw.sk()
	if !gw.showShaded && gw.drawOccludedAs == occludedVisible {
		return
	}

	switch how {
	case drawMeshDefault:
		// Force the shade color to something dim to not distract from
		// the sketch.
		var fillFront render.Fill
		if !gw.showShaded {
			fillFront.Layer = render.LayerDepthOnly
		}
		if dimsSolid(g) && gw.dimSolidModel {
			fillFront.Color = ss.styleColor(hstyle(core.StyleDimSolid))
		}
		hcfFront := canvas.InternFill(fillFront)

		// The back faces are drawn in red; should never see them, since
		// we draw closed shells, so that's a debugging aid.
		var hcfBack render.HFill
		if ss.DrawBackFaces && !g.DisplayMesh.IsTransparent {
			hcfBack = canvas.InternFill(render.Fill{
				Layer: fillFront.Layer,
				Color: core.RGBAFromFloat(1.0, 0.1, 0.1, 1.0),
			})
		} else {
			hcfBack = hcfFront
		}

		// Draw the shaded solid into the depth buffer for hidden line
		// removal, and if we're actually going to display it, to the
		// color buffer too.
		if mc, ok := canvas.(render.MaterialCanvas); ok && ss.Shaded {
			mc.SetMaterials(ss.materialOf())
		}
		canvas.DrawMesh(&g.DisplayMesh, hcfFront, hcfBack)

		// Draw mesh edges, for debugging.
		if gw.showMesh {
			hcsTriangle := canvas.InternStroke(render.Stroke{
				ZIndex: 1,
				Color:  core.RGBAFromFloat(0.0, 1.0, 0.0, 1.0),
				Width:  1,
				Unit:   render.UnitPX,
			})
			var edges srf.SEdgeList
			for _, t := range g.DisplayMesh.Triangles {
				edges.AddEdge(t.A, t.B, 0, 0, 0)
				edges.AddEdge(t.B, t.C, 0, 0, 0)
				edges.AddEdge(t.C, t.A, 0, 0, 0)
			}
			canvas.DrawEdges(&edges, hcsTriangle)
		}

	case drawMeshHovered:
		hcf := canvas.InternFill(render.Fill{
			Color:   ss.styleColor(hstyle(core.StyleHovered)),
			Pattern: render.FillCheckeredA,
			ZIndex:  2,
		})
		var faces []uint32
		he := gw.hover.Entity
		if he.V != 0 && sk.Entity(he).IsFace() {
			faces = append(faces, he.V)
		}
		if gw.hover.Face != 0 {
			faces = append(faces, uint32(gw.hover.Face))
		}
		unclip := gw.clipToSection(canvas)
		canvas.DrawFaces(&g.DisplayMesh, faces, hcf)
		unclip()

	case drawMeshSelected:
		hcf := canvas.InternFill(render.Fill{
			Color:   ss.styleColor(hstyle(core.StyleSelected)),
			Pattern: render.FillCheckeredB,
			ZIndex:  1,
		})
		gw.GroupSelection()
		var faces []uint32
		for _, fc := range gw.gs.face {
			faces = append(faces, fc.V)
		}
		for _, fc := range gw.gs.solidFace {
			faces = append(faces, uint32(fc))
		}
		unclip := gw.clipToSection(canvas)
		canvas.DrawFaces(&g.DisplayMesh, faces, hcf)
		unclip()
	}
}

// dimsSolid tells whether the solid is drawn dim while g is the active
// group: a sketch the user draws in, whose lines a shaded solid would
// distract from. A cosmetic thread's sketch is not one: it is the
// thread, made last and so active, and its part is what it shows, so
// the part is shaded as after a modelled thread.
func dimsSolid(g *sketch.Group) bool {
	if g.Thread.IsThread() {
		return false
	}
	return g.Type == sketch.GroupDrawing3D || g.Type == sketch.GroupDrawingWorkplane
}

// drawGroup is Group::Draw: the mesh, edges and outlines, whether or
// not the group is hidden.
func (gw *graphicsWindow) drawGroup(g *sketch.Group, canvas render.Canvas) {
	ss := gw.ss
	regenerated := g.DisplayDirty
	g.GenerateDisplayItems(gw.sk())
	// Recalculate mass center if needed
	if regenerated && ss.GW.centerOfMass.Draw && ss.GW.centerOfMass.Dirty && g.H == gw.activeGroupH() {
		ss.UpdateCenterOfMass()
	}
	unclip := gw.clipToSection(canvas)
	gw.drawMesh(g, drawMeshDefault, canvas)

	if gw.show.Edges {
		strokeEdge := ss.styleStroke(hstyle(core.StyleSolidEdge))
		strokeEdge.ZIndex = 1
		hcsEdge := canvas.InternStroke(strokeEdge)

		drawAs := render.EmphasizedAndContour
		if gw.show.Outlines {
			drawAs = render.EmphasizedWithoutContour
		}
		canvas.DrawOutlines(&g.DisplayOutlines, hcsEdge, drawAs)
		gw.drawCosmeticThreads(g, canvas)

		if gw.drawOccludedAs != occludedInvisible {
			strokeHidden := ss.styleStroke(hstyle(core.StyleHiddenEdge))
			if gw.drawOccludedAs == occludedVisible {
				strokeHidden.StipplePattern = core.StippleContinuous
			}
			strokeHidden.Layer = render.LayerOccluded
			hcsHidden := canvas.InternStroke(strokeHidden)
			canvas.DrawOutlines(&g.DisplayOutlines, hcsHidden, render.EmphasizedAndContour)
		}
	}

	if gw.show.Outlines {
		strokeOutline := ss.styleStroke(hstyle(core.StyleOutline))
		strokeOutline.ZIndex = 1
		hcsOutline := canvas.InternStroke(strokeOutline)
		canvas.DrawOutlines(&g.DisplayOutlines, hcsOutline, render.ContourOnly)
	}
	unclip()
	gw.drawSectionCap(g, canvas)
}

// drawPolyError shows what is wrong with the active group: where its
// sketch isn't a closed, planar, non-intersecting contour, or the
// problem that kept it from adding a solid. The place is marked on the
// model; the words go in a corner of the window (drawStatusMessage),
// where they cover none of it. A sketch says its contour is not good
// only when a solid is made from it, as its Problem does: a sweep's
// path, a single construction line or a drawing's sketch is open by
// design.
func (gw *graphicsWindow) drawPolyError(g *sketch.Group, canvas render.Canvas) {
	ss := gw.ss
	camera := canvas.Camera()

	strokeUnclosed := ss.styleStroke(hstyle(core.StyleDrawError))
	strokeUnclosed.Color = strokeUnclosed.Color.WithAlpha(50)
	hcsUnclosed := canvas.InternStroke(strokeUnclosed)

	strokeError := ss.styleStroke(hstyle(core.StyleDrawError))
	strokeError.Layer = render.LayerFront
	strokeError.Width = 1.0
	hcsError := canvas.InternStroke(strokeError)

	cross := func(at geom.Vector) {
		s := 6 / camera.Scale
		r, u := camera.ProjRight.ScaledBy(s), camera.ProjUp.ScaledBy(s)
		canvas.DrawLine(at.Minus(r).Minus(u), at.Plus(r).Plus(u), hcsError)
		canvas.DrawLine(at.Minus(r).Plus(u), at.Plus(r).Minus(u), hcsError)
	}

	if g.Type == sketch.GroupDrawingWorkplane && core.PolyError(g.PolyError.How) != core.PolyGood {
		if !gw.sk().ProfileOfSolid(g.H) {
			return
		}
		var msg string
		switch core.PolyError(g.PolyError.How) {
		case core.PolyNotClosed:
			msg = "not closed contour, or not all same style!"
			canvas.DrawLine(g.PolyError.NotClosed.A, g.PolyError.NotClosed.B, hcsUnclosed)
			cross(g.PolyError.NotClosed.A)
		case core.PolyNotCoplanar:
			msg = "points not all coplanar!"
			cross(g.PolyError.ErrorPoint)
		case core.PolySelfIntersecting:
			msg = "contour is self-intersecting!"
			cross(g.PolyError.ErrorPoint)
		default:
			msg = "zero-length edge!"
			cross(g.PolyError.ErrorPoint)
		}
		gw.drawStatusMessage(msg, hcsError, canvas)
		return
	}
	// Otherwise the contours will get filled in DrawFilledPaths.

	// A problem with the group's input, as an edge a chamfer can't
	// cut, is marked where it is.
	if p := g.Problem; p.Msg != "" && p.HasPoint {
		cross(p.At)
		gw.drawStatusMessage(p.Msg, hcsError, canvas)
	}
}

// drawFilledPaths is Group::DrawFilledPaths: the loops of styles with
// a fill, and in the active group a dim fill of good contours.
func (gw *graphicsWindow) drawFilledPaths(g *sketch.Group, canvas render.Canvas) {
	ss := gw.ss
	for i := range g.BezierLoops.Sets {
		sbls := &g.BezierLoops.Sets[i]
		if len(sbls.Outer.Curves) == 0 {
			continue
		}
		// In an assembled loop, all the styles should be the same; so
		// doesn't matter which one we grab.
		sb := &sbls.Outer.Curves[0]
		s := ss.SK.StyleGet(hstyle(uint32(sb.AuxA)))

		fill := render.Fill{ZIndex: 1}
		switch {
		case s.Filled:
			// This is a filled loop, where the user specified a fill
			// color.
			fill.Color = s.FillColor
		case g.H == gw.activeGroupH() && ss.CheckClosedContour &&
			core.PolyError(g.PolyError.How) == core.PolyGood:
			// If this is the active group, and we are supposed to check
			// for closed contours, and we do indeed have a closed and
			// non-intersecting contour, then fill it dimly.
			fill.Color = ss.styleColor(hstyle(core.StyleContourFill)).WithAlpha(127)
		default:
			continue
		}
		hcf := canvas.InternFill(fill)

		var sp srf.SPolygon
		sbls.MakePwlInto(&sp, gw.ss.SK.Tol())
		canvas.DrawPolygon(&sp, hcf)
	}
}

// drawContourAreaLabels is Group::DrawContourAreaLabels.
func (gw *graphicsWindow) drawContourAreaLabels(g *sketch.Group, canvas render.Canvas) {
	ss := gw.ss
	camera := canvas.Camera()
	gr := camera.ProjRight.ScaledBy(1 / camera.Scale)
	gu := camera.ProjUp.ScaledBy(1 / camera.Scale)

	for i := range g.BezierLoops.Sets {
		sbls := &g.BezierLoops.Sets[i]
		if len(sbls.Outer.Curves) == 0 {
			continue
		}
		lo := sbls.Outer.Curves[0].Ctrl[0]
		hi := lo
		var zero geom.Vector
		sbls.BoundingProjd(geom.Vector{X: 1}, zero, &lo.X, &hi.X)
		sbls.BoundingProjd(geom.Vector{Y: 1}, zero, &lo.Y, &hi.Y)
		sbls.BoundingProjd(geom.Vector{Z: 1}, zero, &lo.Z, &hi.Z)

		mid := lo.Plus(hi).ScaledBy(0.5)

		hs := hstyle(core.StyleConstraint)
		stroke := ss.styleStroke(hs)
		stroke.Layer = render.LayerFront

		label := ss.MmToStringSI(math.Abs(sbls.SignedArea()), 2)
		fontHeight := ss.styleTextHeight(hs)
		vf := res.BuiltinVectorFont()
		textWidth := vf.Width(fontHeight, label)
		textHeight := vf.GetCapHeight(fontHeight)
		pos := mid.Minus(gr.ScaledBy(textWidth / 2.0)).Minus(gu.ScaledBy(textHeight / 2.0))
		canvas.DrawVectorText(label, fontHeight, pos, gr, gu, canvas.InternStroke(stroke))
	}
}

// materialOf gives each face's material by its appearance (roadmap
// 19.2): its own, its group's, or plastic; it remembers what it found
// for the frame.
func (ss *Application) materialOf() func(face uint32) render.Material {
	seen := map[uint32]render.Material{}
	return func(face uint32) render.Material {
		if m, ok := seen[face]; ok {
			return m
		}
		m, ok := render.Materials[ss.SK.AppearanceOf(srf.FaceID(face))]
		if !ok {
			m = render.Materials["plastic"]
		}
		seen[face] = m
		return m
	}
}

// appearanceIndex is an appearance's place in render.MaterialNames.
func appearanceIndex(name string) int {
	for i, n := range render.MaterialNames {
		if n == name {
			return i
		}
	}
	return 0
}

// ScreenChangeAppearance gives the shown group the v-th appearance.
func (tw *textWindow) ScreenChangeAppearance(link int, v uint32) {
	ss := tw.ss
	g := ss.SK.Group(tw.shown.group)
	if g == nil || int(v) >= len(render.MaterialNames) {
		return
	}
	ss.UndoRemember()
	if err := ss.SK.SetGroupAppearance(g.H, render.MaterialNames[v]); err != nil {
		ss.Report(err)
	}
	ss.SK.Unsaved = true
	ss.GW.Invalidate(false)
	ss.ScheduleShowTW()
}

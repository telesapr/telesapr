package ui

// The 2d vector output stuff that isn't specific to any particular file
// format: getting the appropriate lines and curves, performing hidden
// line removal, calculating bounding boxes, and so on. Also raster and
// triangle mesh output.

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/export"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ExportSectionTo exports the section of the solid model by a plane the
// selection names: the active workplane, a face, or a point and two
// lines.
func (ss *Application) ExportSectionTo(filename string) error {
	sk := ss.SK
	gw := ss.GW
	gn := gw.view.ProjRight.Cross(gw.view.ProjUp)
	gn = gn.WithMagnitude(1)

	g := sk.Group(gw.activeGroupH())
	g.GenerateDisplayItems(sk)
	if g.DisplayMesh.IsEmpty() {
		return errNoSolidForSection
	}

	// The plane in which the exported section lies; need this because
	// we'll reorient from that plane into the xy plane before exporting.
	var origin, u, v, n geom.Vector

	gw.GroupSelection()
	gs := &gw.gs
	switch {
	case gs.n == 0 && !core.IsFreeIn3D(g.ActiveWorkplane):
		wrkpl := sk.MustEntity(g.ActiveWorkplane)
		origin = wrkpl.WorkplaneGetOffset(sk)
		normal := sk.MustEntity(wrkpl.Normal)
		n = normal.NormalN(sk)
		u = normal.NormalU(sk)
		v = normal.NormalV(sk)
	case gs.n == 1 && gs.faces == 1:
		face := sk.MustEntity(gs.entity[0])
		origin = face.FaceGetPointNum(sk)
		n = face.FaceGetNormalNum(sk)
		if n.Dot(gn) < 0 {
			n = n.ScaledBy(-1)
		}
		u = n.NormalDir(0)
		v = n.NormalDir(1)
	case gs.n == 3 && gs.vectors == 2 && gs.points == 1:
		ut := sk.MustEntity(gs.entity[0]).VectorGetNum(sk)
		vt := sk.MustEntity(gs.entity[1]).VectorGetNum(sk)
		ut = ut.WithMagnitude(1)
		vt = vt.WithMagnitude(1)

		if math.Abs(gw.view.ProjUp.Dot(vt)) < math.Abs(gw.view.ProjUp.Dot(ut)) {
			ut, vt = vt, ut
		}
		if gw.view.ProjRight.Dot(ut) < 0 {
			ut = ut.ScaledBy(-1)
		}
		if gw.view.ProjUp.Dot(vt) < 0 {
			vt = vt.ScaledBy(-1)
		}

		origin = sk.MustEntity(gs.point[0]).PointGetNum(sk)
		n = ut.Cross(vt)
		u = ut.WithMagnitude(1)
		v = n.Cross(u).WithMagnitude(1)
	default:
		return errors.New("bad selection for export section. Please select:\n\n" +
			"    * nothing, with an active workplane " +
			"(workplane is section plane)\n" +
			"    * a face (section plane through face)\n" +
			"    * a point and two line segments " +
			"(plane through point and parallel to lines)")
	}
	gw.ClearSelection()

	return ss.exportSectionIn(filename, origin, u, v, n)
}

// errNoSolidForSection is what a section of nothing says.
var errNoSolidForSection = errors.New("no solid model present; draw one with extrudes and revolves, " +
	"or use Export 2d View to export bare lines and curves")

// errSectionMissesSolid is what a section by a plane that cuts nothing
// says, rather than writing an empty file.
var errSectionMissesSolid = errors.New("the section plane doesn't cut the solid")

// ExportSectionHere exports the section of the active group's solid by
// the plane a script means: the section view's, when it is on, and
// otherwise the active workplane.
func (ss *Application) ExportSectionHere(filename string) error {
	sk := ss.SK
	g := sk.Group(sk.ActiveGroup())
	g.GenerateDisplayItems(sk)
	if g.DisplayMesh.IsEmpty() {
		return errNoSolidForSection
	}
	var origin, u, v, n geom.Vector
	keep, o, ok, err := sk.SectionCut()
	switch {
	case err != nil:
		return err
	case ok:
		origin, n = o, keep.N
		u, v = n.NormalDir(0), n.NormalDir(1)
	case core.IsFreeIn3D(g.ActiveWorkplane):
		return errors.New("no section plane: turn the section view on, or activate a workplane")
	default:
		wrkpl := sk.MustEntity(g.ActiveWorkplane)
		origin = wrkpl.WorkplaneGetOffset(sk)
		normal := sk.MustEntity(wrkpl.Normal)
		n = normal.NormalN(sk)
		u = normal.NormalU(sk)
		v = normal.NormalV(sk)
	}
	return ss.exportSectionIn(filename, origin, u, v, n)
}

// ExportSectionByPlane exports the section of the active group's solid
// by the plane through origin with normal n, as the command line names
// it.
func (ss *Application) ExportSectionByPlane(filename string, origin, n geom.Vector) error {
	if n.Magnitude() < core.LengthEps {
		return errors.New("the section plane's normal has no length")
	}
	sk := ss.SK
	g := sk.Group(sk.ActiveGroup())
	g.GenerateDisplayItems(sk)
	if g.DisplayMesh.IsEmpty() {
		return errNoSolidForSection
	}
	n = n.WithMagnitude(1)
	return ss.exportSectionIn(filename, origin, n.NormalDir(0), n.NormalDir(1), n)
}

// exportSectionIn writes the section of the active group's solid by the
// plane through origin with normal n, laid out on u and v.
func (ss *Application) exportSectionIn(filename string, origin, u, v, n geom.Vector) error {
	sk := ss.SK
	gw := ss.GW
	// A sketch drawn after the solid holds none of it: the section is of
	// the solid as it is at the active group.
	g := sk.SolidAt(gw.activeGroupH())
	if g == nil {
		return errNoSolidForSection
	}

	n = n.WithMagnitude(1)
	d := origin.Dot(n)

	var el srf.SEdgeList
	var bl srf.SBezierList

	// If there's a mesh, then grab the edges from it.
	g.RunningMesh.MakeEdgesInPlaneInto(&el, n, d)

	// If there's a shell, then grab the edges and possibly Beziers.
	exportAsPwl := ss.ExportPwlCurves || math.Abs(ss.ExportOffset) > core.LengthEps
	if exportAsPwl {
		g.RunningShell.MakeSectionEdgesInto(n, d, &el, nil)
	} else {
		g.RunningShell.MakeSectionEdgesInto(n, d, &el, &bl)
	}

	// A plane along none of the solid's faces cuts through it: the
	// section is where it cuts the triangles, as the section view's cap.
	if len(el.Edges) == 0 && len(bl.Curves) == 0 {
		shown := sk.Group(gw.activeGroupH())
		shown.GenerateDisplayItems(sk)
		var poly srf.SPolygon
		shown.DisplayMesh.MakeSectionPolygonInto(srf.HalfSpace{N: n, D: d}, &poly)
		poly.MakeEdgesInto(&el)
	}

	// All of these are solid model edges, so use the appropriate style.
	for i := range el.Edges {
		el.Edges[i].AuxA = core.StyleSolidEdge
	}
	for i := range bl.Curves {
		bl.Curves[i].AuxA = core.StyleSolidEdge
	}

	// Remove all overlapping edges/beziers to merge the areas they
	// describe.
	el.CullExtraneousEdges(true)
	bl.CullIdenticalBeziers(true)

	// Collect lines and beziers with custom style & export.
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if !sk.EntityIsVisible(e) {
			continue
		}
		if e.Style.V < core.StyleFirstCustom {
			continue
		}
		if !sk.StyleExportable(e.Style) {
			continue
		}
		if !e.IsInPlane(sk, n, d) {
			continue
		}
		if exportAsPwl {
			gw.generateEdges(e, &el)
		} else {
			sketch.GenerateEntityBeziers(e, &bl, &sketch.Runtime{Sketch: sk})
		}
	}

	// Only remove half of the overlapping edges/beziers to support TTF
	// Stick Fonts.
	el.CullExtraneousEdges(false)
	bl.CullIdenticalBeziers(false)
	if len(el.Edges) == 0 && len(bl.Curves) == 0 {
		return errSectionMissesSolid
	}

	// And write the edges.
	out, err := export.NewWriter(filename, ss.SK, ss.exportOptions())
	if err != nil {
		return err
	}
	// parallel projection (no perspective), and no mesh
	return ss.ExportLinesAndMesh(&el, &bl, nil, u, v, n, origin, 0, out)
}

// getEdgesCanvas turns what constraints draw into edges.
type getEdgesCanvas struct {
	render.CanvasBase
	camera render.Camera
	edges  *srf.SEdgeList
}

func (c *getEdgesCanvas) Camera() *render.Camera { return &c.camera }

func (c *getEdgesCanvas) DrawLine(a, b geom.Vector, hcs render.HStroke) {
	c.edges.AddEdge(a, b, core.StyleConstraint, 0, 0)
}

func (c *getEdgesCanvas) DrawEdges(el *srf.SEdgeList, hcs render.HStroke) {
	for _, e := range el.Edges {
		c.edges.AddEdge(e.A, e.B, core.StyleConstraint, 0, 0)
	}
}

func (c *getEdgesCanvas) DrawVectorText(text string, height float64, o, u, v geom.Vector, hcs render.HStroke) {
	traceEdge := func(a, b geom.Vector) { c.edges.AddEdge(a, b, core.StyleConstraint, 0, 0) }
	res.BuiltinVectorFont().TraceAligned(height, o, u, v, text, traceEdge, &c.camera)
}

func (c *getEdgesCanvas) DrawQuad(a, b, cc, d geom.Vector, hcf render.HFill) {
	// Do nothing
}

func (c *getEdgesCanvas) DrawBeziers(bl *srf.SBezierList, hcs render.HStroke) bool {
	panic("ui: not implemented")
}
func (c *getEdgesCanvas) DrawOutlines(ol *srf.SOutlineList, hcs render.HStroke, drawAs render.DrawOutlinesAs) {
	panic("ui: not implemented")
}
func (c *getEdgesCanvas) DrawPoint(o geom.Vector, hcs render.HStroke) { panic("ui: not implemented") }
func (c *getEdgesCanvas) DrawPolygon(p *srf.SPolygon, hcf render.HFill) {
	panic("ui: not implemented")
}
func (c *getEdgesCanvas) DrawMesh(m *srf.SMesh, hcfFront, hcfBack render.HFill) {
	panic("ui: not implemented")
}
func (c *getEdgesCanvas) DrawFaces(m *srf.SMesh, faces []uint32, hcf render.HFill) {
	panic("ui: not implemented")
}
func (c *getEdgesCanvas) DrawPixmap(pm *res.Pixmap, o, u, v geom.Vector, ta, tb geom.Point2d, hcf render.HFill) {
	panic("ui: not implemented")
}
func (c *getEdgesCanvas) InvalidatePixmap(pm *res.Pixmap) { panic("ui: not implemented") }

// ExportViewOrWireframeTo exports the view as 2d vector art, or the
// wireframe in 3d.
func (ss *Application) ExportViewOrWireframeTo(filename string, exportWireframe bool) error {
	sk := ss.SK
	gw := ss.GW
	var edges srf.SEdgeList
	var beziers srf.SBezierList

	out, err := export.NewWriter(filename, ss.SK, ss.exportOptions())
	if err != nil {
		return err
	}

	ss.EnterExportMode()
	ss.GenerateAll(sketch.GenerateAll)

	var sm *srf.SMesh
	if gw.showShaded || gw.drawOccludedAs != occludedVisible {
		g := sk.Group(gw.activeGroupH())
		g.GenerateDisplayItems(sk)
		sm = &g.DisplayMesh
	}
	if sm != nil && sm.IsEmpty() {
		sm = nil
	}

	for i := range sk.Entities {
		e := &sk.Entities[i]
		if !sk.EntityIsVisible(e) {
			continue
		}

		if ss.ExportPwlCurves || sm != nil || math.Abs(ss.ExportOffset) > core.LengthEps {
			// We will be doing hidden line removal, which we can't do on
			// exact curves; so we need things broken down to pwls. Same
			// problem with cutter radius compensation.
			gw.generateEdges(e, &edges)
		} else {
			sketch.GenerateEntityBeziers(e, &beziers, &sketch.Runtime{Sketch: sk})
		}
	}

	if gw.show.Edges || gw.show.Outlines {
		g := sk.Group(gw.activeGroupH())
		g.GenerateDisplayItems(sk)
		if gw.show.Edges {
			g.DisplayOutlines.ListTaggedInto(&edges, core.StyleSolidEdge, 0)
		}
	}

	if gw.showConstraints != sCMNoShow {
		if !out.OutputConstraints(sk.Constraints) {
			canvas := &getEdgesCanvas{camera: gw.Camera(), edges: &edges}

			// The output format cannot represent constraints directly,
			// so convert them to edges.
			for i := range sk.Constraints {
				gw.drawConstraint(&sk.Constraints[i], constraintDrawDefault, canvas)
			}
		}
	}

	if exportWireframe {
		u := geom.Vector{X: 1}
		v := geom.Vector{Y: 1}
		n := geom.Vector{Z: 1}
		origin := geom.Vector{}
		cameraTan := 0.0
		scale := 1.0

		out.SetModelviewProjection(u, v, n, origin, cameraTan, scale)

		return ss.ExportWireframeCurves(&edges, &beziers, out)
	}
	{
		u := gw.view.ProjRight
		v := gw.view.ProjUp
		n := u.Cross(v)
		origin := gw.view.Offset.ScaledBy(-1)

		out.SetModelviewProjection(u, v, n, origin, ss.CameraTangent()*gw.view.Scale, ss.ExportScale)

		err = ss.ExportLinesAndMesh(&edges, &beziers, sm, u, v, n, origin, ss.CameraTangent()*gw.view.Scale, out)

		if !out.HasCanvasSize() {
			// These file formats don't have a canvas size, so they just
			// get exported in the raw coordinate system. So indicate
			// what that was on-screen.
			ss.GW.justExported.ShowOrigin = true
			ss.GW.justExported.Pt = origin
			ss.GW.justExported.U = u
			ss.GW.justExported.V = v
		} else {
			ss.GW.justExported.ShowOrigin = false
		}

		ss.GW.justExported.Draw = true
		gw.Invalidate(false)
	}
	return err
}

// ExportWireframeCurves writes the edges and curves in 3d.
func (ss *Application) ExportWireframeCurves(sel *srf.SEdgeList, sbl *srf.SBezierList, out export.Writer) error {
	var sblss srf.SBezierLoopSetSet
	for _, se := range sel.Edges {
		sb := srf.SBezierFrom2(se.A.ScaledBy(1.0/ss.ExportScale), se.B.ScaledBy(1.0/ss.ExportScale))
		sblss.AddOpenPath(sb)
	}

	sbl.ScaleSelfBy(1.0 / ss.ExportScale)
	for _, sb := range sbl.Curves {
		sblss.AddOpenPath(sb)
	}

	return out.OutputLinesAndMesh(&sblss, nil)
}

// ExportLinesAndMesh projects the edges, curves and mesh, removes the
// hidden lines, and writes what's left.
func (ss *Application) ExportLinesAndMesh(sel *srf.SEdgeList, sbl *srf.SBezierList, sm *srf.SMesh,
	u, v, n, origin geom.Vector, cameraTan float64, out export.Writer) error {
	sk := ss.SK
	gw := ss.GW
	s := 1.0 / ss.ExportScale

	// Project into the export plane; so when we're done, z doesn't
	// matter, and x and y are what goes in the DXF.
	for i := range sel.Edges {
		e := &sel.Edges[i]
		// project into the specified csys, and apply export scale
		e.A = e.A.InPerspective(u, v, n, origin, cameraTan).ScaledBy(s)
		e.B = e.B.InPerspective(u, v, n, origin, cameraTan).ScaledBy(s)
	}

	if sbl != nil {
		for i := range sbl.Curves {
			b := &sbl.Curves[i]
			*b = b.InPerspective(u, v, n, origin, cameraTan)
			for k := 0; k <= b.Degree; k++ {
				b.Ctrl[k] = b.Ctrl[k].ScaledBy(s)
			}
		}
	}

	// If cutter radius compensation is requested, then perform it now
	if math.Abs(ss.ExportOffset) > core.LengthEps {
		// assemble those edges into a polygon, and clear the edge list
		var sp srf.SPolygon
		sel.AssemblePolygon(&sp, nil, false)
		sel.Clear()

		var compd srf.SPolygon
		sp.Normal = geom.Vector{Z: -1}
		sp.FixContourDirections()
		sp.OffsetInto(&compd, ss.ExportOffset*s)

		compd.MakeEdgesInto(sel)
	}

	// Now the triangle mesh; project, then build a BSP to perform
	// occlusion testing and generated the shaded surfaces.
	var smp srf.SMesh
	if sm != nil {
		l0 := ss.LightDir[0].WithMagnitude(1)
		l1 := ss.LightDir[1].WithMagnitude(1)
		for _, tr := range sm.Triangles {
			tt := tr
			tt.A = tt.A.InPerspective(u, v, n, origin, cameraTan).ScaledBy(s)
			tt.B = tt.B.InPerspective(u, v, n, origin, cameraTan).ScaledBy(s)
			tt.C = tt.C.InPerspective(u, v, n, origin, cameraTan).ScaledBy(s)

			// And calculate lighting for the triangle
			n := tt.Normal().WithMagnitude(1)
			lighting := min(1.0, ss.AmbientIntensity+
				max(0.0, ss.LightIntensity[0]*n.Dot(l0))+
				max(0.0, ss.LightIntensity[1]*n.Dot(l1)))
			r := min(1.0, float64(tt.Meta.RedF())*lighting)
			g := min(1.0, float64(tt.Meta.GreenF())*lighting)
			b := min(1.0, float64(tt.Meta.BlueF())*lighting)
			tt.Meta.RGBA = core.RGBAFromFloat(float32(r), float32(g), float32(b), 1)
			smp.AddTriangle(tt)
		}
	}

	var sms srf.SMesh

	// We need the mesh for occlusion testing, but if we don't/can't
	// export it, don't generate it.
	if gw.showShaded && out.CanOutputMesh() {
		// Use the BSP routines to generate the split triangles in paint
		// order.
		if bsp := srf.SBsp3FromMesh(&smp); bsp != nil {
			bsp.GenerateInPaintOrder(&sms)
		}
		// And cull the back-facing triangles
		kept := sms.Triangles[:0]
		for _, tr := range sms.Triangles {
			if tr.Normal().Z < 0 {
				continue
			}
			tr.Tag = 0
			kept = append(kept, tr)
		}
		sms.Triangles = kept
	}

	// And now we perform hidden line removal if requested
	var hlrd srf.SEdgeList
	if sm != nil {
		root := srf.SKdNodeFrom(&smp)

		// Generate the edges where a curved surface turns from
		// front-facing to back-facing.
		if gw.show.Edges || gw.show.Outlines {
			aux := core.StyleSolidEdge
			if gw.show.Outlines {
				aux = core.StyleOutline
			}
			root.MakeCertainEdgesInto(sel, srf.EdgeTurning, false, nil, nil, aux)
		}

		root.ClearTags()
		cnt := 1234

		for _, se := range sel.Edges {
			if se.AuxA == core.StyleConstraint {
				// Constraints should not get hidden line removed; they're
				// always on top.
				hlrd.AddEdge(se.A, se.B, se.AuxA, 0, 0)
				continue
			}

			var edges srf.SEdgeList
			// Split the original edge against the mesh
			edges.AddEdge(se.A, se.B, se.AuxA, 0, 0)
			root.OcclusionTestLine(se, &edges, cnt)
			switch gw.drawOccludedAs {
			case occludedStippled:
				for k := range edges.Edges {
					if edges.Edges[k].Tag == 1 {
						edges.Edges[k].AuxA = core.StyleHiddenEdge
					}
				}
			case occludedInvisible:
				kept := edges.Edges[:0]
				for _, e := range edges.Edges {
					if e.Tag == 0 {
						kept = append(kept, e)
					}
				}
				edges.Edges = kept
			}

			// the occlusion test splits unnecessarily; so fix those
			edges.MergeCollinearSegments(se.A, se.B)
			cnt++
			// And add the results to our output
			for _, sen := range edges.Edges {
				hlrd.AddEdge(sen.A, sen.B, sen.AuxA, 0, 0)
			}
		}

		sel = &hlrd
	}

	cleanExportEdges(sk, sel)
	return export.WriteCurves(sel, sbl, &sms, ss.exportTolerance(), out)
}

// cleanExportEdges removes from sel the segments with no length once
// projected, and the parts of collinear segments that overlap, keeping
// the one of the style drawn on top.
func cleanExportEdges(sk *sketch.Sketch, sel *srf.SEdgeList) {
	for i := range sel.Edges {
		sel.Edges[i].Tag = 0
	}
	zIndex := func(aux int) int { return sk.StyleGet(hstyle(uint32(aux))).ZIndex }
	for i := 0; i < len(sel.Edges); i++ {
		sei := &sel.Edges[i]
		if sei.Tag != 0 {
			continue
		}

		// Remove segments with zero length projections.
		ai := sei.A
		ai.Z = 0.0
		bi := sei.B
		bi.Z = 0.0
		di := bi.Minus(ai)
		if math.Abs(di.X) < core.LengthEps && math.Abs(di.Y) < core.LengthEps {
			sei.Tag = 1
			continue
		}

		for j := i + 1; j < len(sel.Edges); j++ {
			sej := &sel.Edges[j]
			if sej.Tag != 0 {
				continue
			}

			// Remove segments with zero length projections.
			aj := sej.A
			aj.Z = 0.0
			bj := sej.B
			bj.Z = 0.0
			dj := bj.Minus(aj)
			if math.Abs(dj.X) < core.LengthEps && math.Abs(dj.Y) < core.LengthEps {
				sej.Tag = 1
				continue
			}

			// Skip non-collinear segments.
			const eps = 1e-6
			if aj.DistanceToLine(ai, di) > eps {
				continue
			}
			if bj.DistanceToLine(ai, di) > eps {
				continue
			}

			ta := aj.Minus(ai).Dot(di) / di.Dot(di)
			tb := bj.Minus(ai).Dot(di) / di.Dot(di)
			// pAj and pBj are sej's A and B, swapped if they run
			// backwards along the i-segment.
			swapped := false
			if ta > tb {
				swapped = true
				ta, tb = tb, ta
			}
			pA := func() *geom.Vector {
				if swapped {
					return &sel.Edges[j].B
				}
				return &sel.Edges[j].A
			}
			pB := func() *geom.Vector {
				if swapped {
					return &sel.Edges[j].A
				}
				return &sel.Edges[j].B
			}

			canRemoveI := sej.AuxA == sei.AuxA || zIndex(sei.AuxA) < zIndex(sej.AuxA)
			canRemoveJ := sej.AuxA == sei.AuxA || zIndex(sej.AuxA) < zIndex(sei.AuxA)

			if canRemoveJ {
				// j-segment inside i-segment
				if ta > 0.0-eps && tb < 1.0+eps {
					sej.Tag = 1
					continue
				}

				// cut segment
				aInside := ta > 0.0-eps && ta < 1.0+eps
				if tb > 1.0-eps && aInside {
					*pA() = sei.B
					continue
				}

				// cut segment
				bInside := tb > 0.0-eps && tb < 1.0+eps
				if ta < 0.0-eps && bInside {
					*pB() = sei.A
					continue
				}

				// split segment
				if ta < 0.0-eps && tb > 1.0+eps {
					nb, pbj, auxA, auxB := sei.B, *pB(), sej.AuxA, sej.AuxB
					sel.AddEdge(nb, pbj, auxA, auxB, 0)
					sei = &sel.Edges[i]
					*pB() = sei.A
					continue
				}
			}

			if canRemoveI {
				// j-segment inside i-segment
				if ta < 0.0+eps && tb > 1.0-eps {
					sei.Tag = 1
					break
				}

				// cut segment
				aInside := ta > 0.0+eps && ta < 1.0-eps
				if tb > 1.0-eps && aInside {
					sei.B = *pA()
					i--
					break
				}

				// cut segment
				bInside := tb > 0.0+eps && tb < 1.0-eps
				if ta < 0.0+eps && bInside {
					sei.A = *pB()
					i--
					break
				}

				// split segment
				if ta > 0.0+eps && tb < 1.0-eps {
					pbj, bi, auxA, auxB := *pB(), sei.B, sei.AuxA, sei.AuxB
					sel.AddEdge(pbj, bi, auxA, auxB, 0)
					sei = &sel.Edges[i]
					sei.B = *pA()
					i--
					break
				}
			}
		}
	}
	kept := sel.Edges[:0]
	for _, e := range sel.Edges {
		if e.Tag == 0 {
			kept = append(kept, e)
		}
	}
	sel.Edges = kept

}

// ExportMeshTo exports the active group's mesh.
func (ss *Application) ExportMeshTo(filename string) error {
	sk := ss.SK
	ss.EnterExportMode()
	ss.GenerateAll(sketch.GenerateAll)

	g := sk.Group(ss.GW.activeGroupH())
	g.GenerateDisplayItems(sk)

	m := &g.DisplayMesh
	if m.IsEmpty() {
		return errors.New("active group mesh is empty; nothing to export")
	}

	if ss.ExportStrictMesh {
		naked, inters, leaks := sk.MeshProblems()
		if err := sketch.MeshSoundnessError(naked, inters, leaks); err != nil {
			return err
		}
	}
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("couldn't write to '%s'", filename)
	}
	var out bytes.Buffer
	ss.ShowNakedEdges(true)
	switch {
	case hasExtension(filename, "stl"):
		if err := sketch.ExportMeshAsStlTo(&out, m, ss.ExportScale); err != nil {
			_ = f.Close()
			return fmt.Errorf("writing '%s': %w", filename, err)
		}
	case hasExtension(filename, "3mf"):
		_ = sketch.ExportMeshAs3mfTo(&out, m)
	case hasExtension(filename, "glb"):
		_ = sketch.ExportMeshAsGlbTo(&out, m)
	case hasExtension(filename, "obj"):
		mtlFilename := withExtension(filename, "mtl")
		fMtl, err := os.Create(mtlFilename)
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("couldn't write to '%s'", mtlFilename)
		}

		out.WriteString(cfmt.Sprintf("mtllib %s\n", filepath.Base(mtlFilename)))
		var mtl bytes.Buffer
		export.WriteOBJ(&out, &mtl, m, ss.exportOptions())

		_, werr := fMtl.Write(mtl.Bytes())
		if cerr := fMtl.Close(); werr != nil || cerr != nil {
			_ = f.Close()
			return fmt.Errorf("couldn't write to '%s'", mtlFilename)
		}
	case hasExtension(filename, "js"), hasExtension(filename, "html"):
		e := &g.DisplayOutlines
		export.WriteThreeJS(&out, filename, m, e, &ss.GW.view, ss.exportOptions())
	case hasExtension(filename, "wrl"):
		export.WriteVRML(&out, filename, m, ss.exportOptions())
	default:
		_ = f.Close()
		return fmt.Errorf("can't identify output file type from file extension of "+
			"filename '%s'; try .stl, .3mf, .glb, .obj, .js, .html", filename)
	}

	_, werr := f.Write(out.Bytes())
	if cerr := f.Close(); werr != nil || cerr != nil {
		return fmt.Errorf("couldn't write to '%s'", filename)
	}

	ss.GW.justExported.ShowOrigin = false
	ss.GW.justExported.Draw = true
	ss.GW.Invalidate(false)
	return nil
}

// ExportAsPngTo saves the view as an image at the next redraw.
func (ss *Application) ExportAsPngTo(filename string) {
	ss.ScreenshotFile = filename
	// The rest of the work is done in the next redraw.
	ss.GW.Invalidate(false)
}

// ExportSurfacesTo writes the active group's shell as STEP.
func (ss *Application) ExportSurfacesTo(filename string) error {
	g := ss.SK.Group(ss.GW.activeGroupH())
	var out bytes.Buffer
	if err := export.WriteStep(&out, ss.SK, g, ss.exportOptions()); err != nil {
		return err
	}
	if err := os.WriteFile(filename, out.Bytes(), 0o644); err != nil {
		return fmt.Errorf("couldn't write to '%s'", filename)
	}
	return nil
}

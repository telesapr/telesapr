package ui

// The 2d vector output stuff that isn't specific to any particular file
// format: getting the appropriate lines and curves, performing hidden
// line removal, calculating bounding boxes, and so on. Also raster and
// triangle mesh output (C++ export.cpp).

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cfmt"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/version"
)

// ExportSectionTo exports the section of the solid model by a plane
// (C++ SolveSpaceUI::ExportSectionTo).
func (ss *SolveSpaceUI) ExportSectionTo(filename string) {
	sk := ss.SK
	gw := ss.GW
	gn := gw.view.ProjRight.Cross(gw.view.ProjUp)
	gn = gn.WithMagnitude(1)

	g := sk.GetGroup(gw.activeGroupH())
	g.GenerateDisplayItems(sk)
	if g.DisplayMesh.IsEmpty() {
		ss.Error("No solid model present; draw one with extrudes and revolves, " +
			"or use Export 2d View to export bare lines and curves.")
		return
	}

	// The plane in which the exported section lies; need this because
	// we'll reorient from that plane into the xy plane before exporting.
	var origin, u, v, n geom.Vector

	gw.GroupSelection()
	gs := &gw.gs
	switch {
	case gs.n == 0 && !core.IsFreeIn3D(g.ActiveWorkplane):
		wrkpl := sk.MustGetEntity(g.ActiveWorkplane)
		origin = wrkpl.WorkplaneGetOffset(sk)
		normal := sk.MustGetEntity(wrkpl.Normal)
		n = normal.NormalN(sk)
		u = normal.NormalU(sk)
		v = normal.NormalV(sk)
	case gs.n == 1 && gs.faces == 1:
		face := sk.MustGetEntity(gs.entity[0])
		origin = face.FaceGetPointNum(sk)
		n = face.FaceGetNormalNum(sk)
		if n.Dot(gn) < 0 {
			n = n.ScaledBy(-1)
		}
		u = n.NormalDir(0)
		v = n.NormalDir(1)
	case gs.n == 3 && gs.vectors == 2 && gs.points == 1:
		ut := sk.MustGetEntity(gs.entity[0]).VectorGetNum(sk)
		vt := sk.MustGetEntity(gs.entity[1]).VectorGetNum(sk)
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

		origin = sk.MustGetEntity(gs.point[0]).PointGetNum(sk)
		n = ut.Cross(vt)
		u = ut.WithMagnitude(1)
		v = n.Cross(u).WithMagnitude(1)
	default:
		ss.Error("Bad selection for export section. Please select:\n\n" +
			"    * nothing, with an active workplane " +
			"(workplane is section plane)\n" +
			"    * a face (section plane through face)\n" +
			"    * a point and two line segments " +
			"(plane through point and parallel to lines)\n")
		return
	}
	gw.ClearSelection()

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

	// And write the edges.
	if out := ss.vectorFileWriterFor(filename); out != nil {
		// parallel projection (no perspective), and no mesh
		ss.ExportLinesAndMesh(&el, &bl, nil, u, v, n, origin, 0, out)
	}
}

// getEdgesCanvas turns what constraints draw into edges (C++
// GetEdgesCanvas, "an awful temporary hack to replace
// Constraint::GetEdges").
type getEdgesCanvas struct {
	render.CanvasBase
	camera render.Camera
	edges  *srf.SEdgeList
}

func (c *getEdgesCanvas) GetCamera() *render.Camera { return &c.camera }

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
// wireframe in 3d (C++ SolveSpaceUI::ExportViewOrWireframeTo).
func (ss *SolveSpaceUI) ExportViewOrWireframeTo(filename string, exportWireframe bool) {
	sk := ss.SK
	gw := ss.GW
	var edges srf.SEdgeList
	var beziers srf.SBezierList

	out := ss.vectorFileWriterFor(filename)
	if out == nil {
		return
	}

	ss.ExportMode = true
	ss.GenerateAll(sketch.GenerateAll)

	var sm *srf.SMesh
	if gw.showShaded || gw.drawOccludedAs != OccludedVisible {
		g := sk.GetGroup(gw.activeGroupH())
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
		g := sk.GetGroup(gw.activeGroupH())
		g.GenerateDisplayItems(sk)
		if gw.show.Edges {
			g.DisplayOutlines.ListTaggedInto(&edges, core.StyleSolidEdge, 0)
		}
	}

	if gw.showConstraints != SCMNoShow {
		if !out.OutputConstraints(&sk.Constraints) {
			canvas := &getEdgesCanvas{camera: gw.GetCamera(), edges: &edges}

			// The output format cannot represent constraints directly,
			// so convert them to edges.
			for i := range sk.Constraints {
				gw.drawConstraint(&sk.Constraints[i], ConstraintDrawDefault, canvas)
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

		out.vw().SetModelviewProjection(u, v, n, origin, cameraTan, scale)

		ss.ExportWireframeCurves(&edges, &beziers, out)
	} else {
		u := gw.view.ProjRight
		v := gw.view.ProjUp
		n := u.Cross(v)
		origin := gw.view.Offset.ScaledBy(-1)

		out.vw().SetModelviewProjection(u, v, n, origin, ss.CameraTangent()*gw.view.Scale, ss.ExportScale)

		ss.ExportLinesAndMesh(&edges, &beziers, sm, u, v, n, origin, ss.CameraTangent()*gw.view.Scale, out)

		if !out.HasCanvasSize() {
			// These file formats don't have a canvas size, so they just
			// get exported in the raw coordinate system. So indicate
			// what that was on-screen.
			ss.JustExportedInfo.ShowOrigin = true
			ss.JustExportedInfo.Pt = origin
			ss.JustExportedInfo.U = u
			ss.JustExportedInfo.V = v
		} else {
			ss.JustExportedInfo.ShowOrigin = false
		}

		ss.JustExportedInfo.Draw = true
		gw.Invalidate(false)
	}
}

// ExportWireframeCurves writes the edges and curves in 3d (C++
// SolveSpaceUI::ExportWireframeCurves).
func (ss *SolveSpaceUI) ExportWireframeCurves(sel *srf.SEdgeList, sbl *srf.SBezierList, out vectorFileWriter) {
	var sblss srf.SBezierLoopSetSet
	for _, se := range sel.Edges {
		sb := srf.SBezierFrom2(se.A.ScaledBy(1.0/ss.ExportScale), se.B.ScaledBy(1.0/ss.ExportScale))
		sblss.AddOpenPath(sb)
	}

	sbl.ScaleSelfBy(1.0 / ss.ExportScale)
	for _, sb := range sbl.Curves {
		sblss.AddOpenPath(sb)
	}

	out.vw().OutputLinesAndMesh(&sblss, nil)
}

// ExportLinesAndMesh projects the edges, curves and mesh, removes the
// hidden lines, and writes what's left (C++
// SolveSpaceUI::ExportLinesAndMesh).
func (ss *SolveSpaceUI) ExportLinesAndMesh(sel *srf.SEdgeList, sbl *srf.SBezierList, sm *srf.SMesh,
	u, v, n, origin geom.Vector, cameraTan float64, out vectorFileWriter) {
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
			lighting := cppMin(1.0, ss.AmbientIntensity+
				cppMax(0.0, ss.LightIntensity[0]*n.Dot(l0))+
				cppMax(0.0, ss.LightIntensity[1]*n.Dot(l1)))
			r := cppMin(1.0, float64(tt.Meta.RedF())*lighting)
			g := cppMin(1.0, float64(tt.Meta.GreenF())*lighting)
			b := cppMin(1.0, float64(tt.Meta.BlueF())*lighting)
			tt.Meta.RgbaColor = core.RgbaColorFromFloat(float32(r), float32(g), float32(b), 1)
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
			case OccludedStippled:
				for k := range edges.Edges {
					if edges.Edges[k].Tag == 1 {
						edges.Edges[k].AuxA = core.StyleHiddenEdge
					}
				}
			case OccludedInvisible:
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

	// Clean up: remove overlapping line segments and segments with
	// zero-length projections.
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

	// We kept the line segments and Beziers separate until now; but put
	// them all together, and also project everything into the xy plane,
	// since not all export targets ignore the z component of the points.
	if sbl == nil {
		panic("ui: adding line segments to beziers assumes bezier list is non-nil")
	}
	for _, e := range sel.Edges {
		sb := srf.SBezierFrom2(e.A, e.B)
		sb.AuxA = e.AuxA
		sbl.Curves = append(sbl.Curves, sb)
	}
	for i := range sbl.Curves {
		b := &sbl.Curves[i]
		for k := 0; k <= b.Degree; k++ {
			b.Ctrl[k].Z = 0
		}
	}

	// If possible, then we will assemble these output curves into loops.
	// They will then get exported as closed paths.
	var sblss srf.SBezierLoopSetSet
	var leftovers []srf.SBezierLoop
	plane := srf.FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	var spxyz srf.SPolygon
	var allClosed bool
	var notClosedAt srf.SEdge
	for i := range sbl.Curves {
		sbl.Curves[i].Tag = 0
	}
	sblss.FindOuterFacesFrom(sbl, &spxyz, &plane, ss.ExportChordTolMm(),
		&allClosed, &notClosedAt, nil, nil, &leftovers)
	if len(leftovers) > 0 {
		sblss.Sets = append(sblss.Sets, srf.SBezierLoopSet{Outer: leftovers[0], Holes: leftovers[1:]})
	}

	// Now write the lines and triangles to the output file
	out.vw().OutputLinesAndMesh(&sblss, &sms)
}

// cppMin is std::min, which keeps the first argument unless the second
// is less.
func cppMin(a, b float64) float64 {
	if b < a {
		return b
	}
	return a
}

// cppMax is std::max, which keeps the first argument when they're equal.
func cppMax(a, b float64) float64 {
	if a < b {
		return b
	}
	return a
}

// vectorFileWriter is a 2d vector file format (C++ VectorFileWriter).
type vectorFileWriter interface {
	vw() *vectorWriter
	StartPath(strokeRgb core.RgbaColor, lineWidth float64, filled bool, fillRgb core.RgbaColor, hs core.HStyle)
	FinishPath(strokeRgb core.RgbaColor, lineWidth float64, filled bool, fillRgb core.RgbaColor,
		hs core.HStyle, closed bool)
	Bezier(sb *srf.SBezier)
	Triangle(tr *srf.STriangle)
	OutputConstraints(constraints *[]core.Constraint) bool
	Background(color core.RgbaColor)
	StartFile()
	FinishAndCloseFile()
	HasCanvasSize() bool
	CanOutputMesh() bool
}

// vectorWriter is what the formats share (C++ VectorFileWriter's
// fields and non-virtual methods). The file is written when it's
// finished.
type vectorWriter struct {
	ss   *SolveSpaceUI
	self vectorFileWriter

	u, v, n, origin  geom.Vector
	cameraTan, scale float64

	f        *os.File
	out      bytes.Buffer
	filename string

	ptMin, ptMax geom.Vector
}

func (w *vectorWriter) vw() *vectorWriter { return w }

// OutputConstraints reports whether the format writes constraints
// itself; most don't.
func (w *vectorWriter) OutputConstraints(*[]core.Constraint) bool { return false }

func (w *vectorWriter) printf(format string, args ...any) {
	w.out.WriteString(cfmt.Sprintf(format, args...))
}

// closeFile writes the file (C++ fclose).
func (w *vectorWriter) closeFile() {
	if w.f == nil {
		return
	}
	_, err := w.f.Write(w.out.Bytes())
	if cerr := w.f.Close(); err == nil {
		err = cerr
	}
	w.f = nil
	w.out.Reset()
	if err != nil {
		w.ss.Error("Couldn't write to '%s'", w.filename)
	}
}

// MmToPts converts mm to points, 72 to the inch (C++
// VectorFileWriter::MmToPts).
func MmToPts(mm float64) float64 {
	// 72 points in an inch
	return (mm / 25.4) * 72
}

// vectorFileWriterFor picks the writer for a file's extension and opens
// the file (C++ VectorFileWriter::ForFile).
func (ss *SolveSpaceUI) vectorFileWriterFor(filename string) vectorFileWriter {
	var ret vectorFileWriter
	needOpen := true
	switch {
	case hasExtension(filename, "dxf"):
		ret = &dxfFileWriter{}
		needOpen = false
	case hasExtension(filename, "ps"), hasExtension(filename, "eps"):
		ret = &epsFileWriter{}
	case hasExtension(filename, "pdf"):
		ret = &pdfFileWriter{}
	case hasExtension(filename, "svg"):
		ret = &svgFileWriter{}
	case hasExtension(filename, "plt"), hasExtension(filename, "hpgl"):
		ret = &hpglFileWriter{}
	case hasExtension(filename, "step"), hasExtension(filename, "stp"):
		ret = &step2dFileWriter{}
	case hasExtension(filename, "txt"), hasExtension(filename, "ngc"):
		ret = &gcodeFileWriter{}
	default:
		ss.Error("Can't identify output file type from file extension of "+
			"filename '%s'; try "+
			".step, .stp, .dxf, .svg, .plt, .hpgl, .pdf, .txt, .ngc, "+
			".eps, or .ps.", filename)
		return nil
	}
	w := ret.vw()
	w.ss = ss
	w.self = ret
	w.filename = filename
	if !needOpen {
		return ret
	}

	f, err := os.Create(filename)
	if err != nil {
		ss.Error("Couldn't write to '%s'", filename)
		return nil
	}
	w.f = f
	return ret
}

// SetModelviewProjection sets the projection Transform applies (C++
// VectorFileWriter::SetModelviewProjection).
func (w *vectorWriter) SetModelviewProjection(u, v, n, origin geom.Vector, cameraTan, scale float64) {
	w.u = u
	w.v = v
	w.n = n
	w.origin = origin
	w.cameraTan = cameraTan
	w.scale = scale
}

// Transform projects a point as the view was exported (C++
// VectorFileWriter::Transform).
func (w *vectorWriter) Transform(pos geom.Vector) geom.Vector {
	return pos.InPerspective(w.u, w.v, w.n, w.origin, w.cameraTan).ScaledBy(1.0 / w.scale)
}

// loopsOf is a loop set's loops in C++'s order: the outer, then the
// holes.
func loopsOf(sbls *srf.SBezierLoopSet) []*srf.SBezierLoop {
	loops := []*srf.SBezierLoop{&sbls.Outer}
	for i := range sbls.Holes {
		loops = append(loops, &sbls.Holes[i])
	}
	return loops
}

// OutputLinesAndMesh works out the canvas and writes the file (C++
// VectorFileWriter::OutputLinesAndMesh).
func (w *vectorWriter) OutputLinesAndMesh(sblss *srf.SBezierLoopSetSet, sm *srf.SMesh) {
	ss := w.ss
	sk := ss.SK

	// First calculate the bounding box.
	w.ptMin = geom.Vector{X: geom.VeryPositive, Y: geom.VeryPositive, Z: geom.VeryPositive}
	w.ptMax = geom.Vector{X: -geom.VeryPositive, Y: -geom.VeryPositive, Z: -geom.VeryPositive}
	if sm != nil {
		for _, tr := range sm.Triangles {
			tr.A.MakeMaxMin(&w.ptMax, &w.ptMin)
			tr.B.MakeMaxMin(&w.ptMax, &w.ptMin)
			tr.C.MakeMaxMin(&w.ptMax, &w.ptMin)
		}
	}
	if sblss != nil {
		for si := range sblss.Sets {
			for _, sbl := range loopsOf(&sblss.Sets[si]) {
				for _, b := range sbl.Curves {
					for i := 0; i <= b.Degree; i++ {
						b.Ctrl[i].MakeMaxMin(&w.ptMax, &w.ptMin)
					}
				}
			}
		}
	}

	// And now we compute the canvas size.
	s := 1.0 / ss.ExportScale
	if ss.ExportCanvasSizeAuto {
		// It's based on the calculated bounding box; we grow it along
		// each boundary by the specified amount.
		w.ptMin.X -= s * ss.ExportMargin.Left
		w.ptMax.X += s * ss.ExportMargin.Right
		w.ptMin.Y -= s * ss.ExportMargin.Bottom
		w.ptMax.Y += s * ss.ExportMargin.Top
	} else {
		w.ptMin.X = s * ss.ExportCanvas.Dx
		w.ptMin.Y = s * ss.ExportCanvas.Dy
		w.ptMax.X = w.ptMin.X + s*ss.ExportCanvas.Width
		w.ptMax.Y = w.ptMin.Y + s*ss.ExportCanvas.Height
	}

	w.self.StartFile()
	if ss.ExportBackgroundColor {
		w.self.Background(ss.BackgroundColor)
	}
	if sm != nil && ss.ExportShadedTriangles {
		for i := range sm.Triangles {
			w.self.Triangle(&sm.Triangles[i])
		}
	}
	if sblss != nil {
		for si := range sblss.Sets {
			for _, sbl := range loopsOf(&sblss.Sets[si]) {
				if len(sbl.Curves) == 0 || !sk.StyleExportable(hstyle(uint32(sbl.Curves[0].AuxA))) {
					continue
				}

				hs := hstyle(uint32(sbl.Curves[0].AuxA))
				stl := sk.StyleGet(hs)
				lineWidth := ss.styleWidthMm(hs) * s
				strokeRgb := ss.styleExportColor(hs)
				fillRgb := ss.styleExportFillColor(hs)

				w.self.StartPath(strokeRgb, lineWidth, stl.Filled, fillRgb, hs)
				for i := range sbl.Curves {
					w.self.Bezier(&sbl.Curves[i])
				}
				w.self.FinishPath(strokeRgb, lineWidth, stl.Filled, fillRgb, hs, sbl.IsClosed())
			}
		}
	}
	w.self.FinishAndCloseFile()
}

// BezierAsPwl writes a curve as line segments (C++
// VectorFileWriter::BezierAsPwl).
func (w *vectorWriter) BezierAsPwl(sb *srf.SBezier) {
	var lv []geom.Vector
	sb.MakePwlIntoPoints(&lv, w.ss.ExportChordTolMm(), 0)

	for i := 1; i < len(lv); i++ {
		sb := srf.SBezierFrom2(lv[i-1], lv[i])
		w.self.Bezier(&sb)
	}
}

// BezierAsNonrationalCubic writes a curve as nonrational cubics (C++
// VectorFileWriter::BezierAsNonrationalCubic).
func (w *vectorWriter) BezierAsNonrationalCubic(sb *srf.SBezier, depth int) {
	t0, t1 := sb.TangentAt(0), sb.TangentAt(1)
	// The curve is correct, and the first derivatives are correct, at
	// the endpoints.
	bnr := srf.SBezierFrom4(
		sb.Start(),
		sb.Start().Plus(t0.ScaledBy(1.0/3)),
		sb.Finish().Minus(t1.ScaledBy(1.0/3)),
		sb.Finish())

	tol := w.ss.ExportChordTolMm()
	// Arbitrary choice, but make it a little finer than pwl tolerance
	// since it should be easier to achieve that with the smooth curves.
	tol /= 2

	closeEnough := true
	for i := 1; i <= 3; i++ {
		t := float64(i) / 4.0
		p0 := sb.PointAt(t)
		pn := bnr.PointAt(t)
		d := p0.Minus(pn).Magnitude()
		if d > tol {
			closeEnough = false
		}
	}

	if closeEnough || depth > 3 {
		w.self.Bezier(&bnr)
	} else {
		var bef, aft srf.SBezier
		sb.SplitAt(0.5, &bef, &aft)
		w.BezierAsNonrationalCubic(&bef, depth+1)
		w.BezierAsNonrationalCubic(&aft, depth+1)
	}
}

// ExportMeshTo exports the active group's mesh (C++
// SolveSpaceUI::ExportMeshTo).
func (ss *SolveSpaceUI) ExportMeshTo(filename string) {
	sk := ss.SK
	ss.ExportMode = true
	ss.GenerateAll(sketch.GenerateAll)

	g := sk.GetGroup(ss.GW.activeGroupH())
	g.GenerateDisplayItems(sk)

	m := &g.DisplayMesh
	if m.IsEmpty() {
		ss.Error("Active group mesh is empty; nothing to export.")
		return
	}

	f, err := os.Create(filename)
	if err != nil {
		ss.Error("Couldn't write to '%s'", filename)
		return
	}
	var out bytes.Buffer
	ss.ShowNakedEdges(true)
	switch {
	case hasExtension(filename, "stl"):
		_ = sketch.ExportMeshAsStlTo(&out, m, ss.ExportScale)
	case hasExtension(filename, "3mf"):
		_ = sketch.ExportMeshAs3mfTo(&out, m)
	case hasExtension(filename, "obj"):
		mtlFilename := withExtension(filename, "mtl")
		fMtl, err := os.Create(mtlFilename)
		if err != nil {
			ss.Error("Couldn't write to '%s'", filename)
			_ = f.Close()
			return
		}

		out.WriteString(cfmt.Sprintf("mtllib %s\n", filepath.Base(mtlFilename)))
		var mtl bytes.Buffer
		ss.ExportMeshAsObjTo(&out, &mtl, m)

		_, werr := fMtl.Write(mtl.Bytes())
		if cerr := fMtl.Close(); werr != nil || cerr != nil {
			ss.Error("Couldn't write to '%s'", mtlFilename)
		}
	case hasExtension(filename, "js"), hasExtension(filename, "html"):
		e := &g.DisplayOutlines
		ss.ExportMeshAsThreeJsTo(&out, filename, m, e)
	case hasExtension(filename, "wrl"):
		ss.ExportMeshAsVrmlTo(&out, filename, m)
	default:
		ss.Error("Can't identify output file type from file extension of "+
			"filename '%s'; try .stl, .3mf, .obj, .js, .html.", filename)
	}

	_, werr := f.Write(out.Bytes())
	if cerr := f.Close(); werr != nil || cerr != nil {
		ss.Error("Couldn't write to '%s'", filename)
	}

	ss.JustExportedInfo.ShowOrigin = false
	ss.JustExportedInfo.Draw = true
	ss.GW.Invalidate(false)
}

// ExportMeshAsObjTo writes the mesh as Wavefront OBJ, with its
// materials (C++ SolveSpaceUI::ExportMeshAsObjTo).
func (ss *SolveSpaceUI) ExportMeshAsObjTo(fObj, fMtl *bytes.Buffer, sm *srf.SMesh) {
	colors := map[uint32]string{}
	var colorKeys []core.RgbaColor
	for _, t := range sm.Triangles {
		color := t.Meta.RgbaColor
		if _, ok := colors[color.ToARGB32()]; !ok {
			id := cfmt.Sprintf("h%02x%02x%02x", color.Red, color.Green, color.Blue)
			colors[color.ToARGB32()] = id
			colorKeys = append(colorKeys, color)
		}
		for _, vtx := range []geom.Vector{t.A, t.B, t.C} {
			p := vtx.ScaledBy(1 / ss.ExportScale)
			fObj.WriteString(cfmt.Sprintf("v %.10f %.10f %.10f\n", p.X, p.Y, p.Z))
		}
	}

	// std::map orders the colours by their ARGB value.
	sort.SliceStable(colorKeys, func(i, j int) bool {
		return colorKeys[i].ToARGB32() < colorKeys[j].ToARGB32()
	})
	for _, c := range colorKeys {
		fMtl.WriteString(cfmt.Sprintf("newmtl %s\n", colors[c.ToARGB32()]))
		fMtl.WriteString(cfmt.Sprintf("Kd %.3f %.3f %.3f\n", c.RedF(), c.GreenF(), c.BlueF()))
	}

	for _, t := range sm.Triangles {
		for _, nv := range []geom.Vector{t.NA, t.NB, t.NC} {
			n := nv.WithMagnitude(1.0)
			fObj.WriteString(cfmt.Sprintf("vn %.10f %.10f %.10f\n", n.X, n.Y, n.Z))
		}
	}

	var currentColor core.RgbaColor
	for i, t := range sm.Triangles {
		if !currentColor.Equals(t.Meta.RgbaColor) {
			currentColor = t.Meta.RgbaColor
			fObj.WriteString(cfmt.Sprintf("usemtl %s\n", colors[currentColor.ToARGB32()]))
		}

		fObj.WriteString(cfmt.Sprintf("f %d//%d %d//%d %d//%d\n",
			i*3+1, i*3+1,
			i*3+2, i*3+2,
			i*3+3, i*3+3))
	}
}

// fileStemIdentifier is the file's name without extension, with
// everything but letters, digits and non-ASCII bytes made '_'.
func fileStemIdentifier(filename string) string {
	base := filepath.Base(filename)
	if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	b := []byte(base)
	for i, c := range b {
		isAlnum := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !(isAlnum || c >= 0x80) {
			b[i] = '_'
		}
	}
	return string(b)
}

const threeJsHTMLBegin = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8"></meta>
    <title>Three.js TeleSAPR Mesh</title>
    <script id="%s">%s</script>
    <script id="%s">%s</script>
    <script id="%s">%s</script>
    <style type="text/css">
    body { margin: 0; overflow: hidden; }
    </style>
  </head>
  <body>
    <script>
`

const threeJsHTMLEnd = `
    document.body.appendChild(telesapr(telesapr_model_%s, {
        scale: %g,
        offset: new THREE.Vector3(%g, %g, %g),
        projUp: new THREE.Vector3(%g, %g, %g),
        projRight: new THREE.Vector3(%g, %g, %g)
    }));
    </script>
  </body>
</html>
`

// ExportMeshAsThreeJsTo writes the mesh as a script for three.js, or a
// page with it (C++ SolveSpaceUI::ExportMeshAsThreeJsTo).
func (ss *SolveSpaceUI) ExportMeshAsThreeJsTo(f *bytes.Buffer, filename string, sm *srf.SMesh,
	sol *srf.SOutlineList) {
	const threeFn = "three-r111.min.js"
	const hammerFn = "hammer-2.0.8.js"
	const controlsFn = "TeleSAPRControls.js"
	pf := func(format string, args ...any) { f.WriteString(cfmt.Sprintf(format, args...)) }

	// A default three.js viewer with OrthographicTrackballControls is
	// generated as a comment preceding the data.

	// x bounds should be the range of x or y, whichever is larger,
	// before aspect ratio correction is applied. y bounds should be the
	// range of x or y, whichever is larger. No aspect ratio correction
	// is applied. Near plane should be 1. Camera's z-position should be
	// the range of z + 1 or the larger of the x or y bounds, whichever
	// is larger. Far plane should be at least twice as much as the
	// camera's z-position. Edge projection bias should be about 1/500 of
	// the far plane's distance. Further corrections will be applied to
	// the z-position and far plane in the default viewer, but the
	// defaults are fine for a model which only rotates about the world
	// origin.

	bndh, bndl := sm.GetBounding()
	largerBoundXY := cppMax(bndh.X-bndl.X, bndh.Y-bndl.Y)
	largerBoundZ := cppMax(largerBoundXY, bndh.Z-bndl.Z+1)

	basename := fileStemIdentifier(filename)

	if hasExtension(filename, "html") {
		pf(threeJsHTMLBegin,
			threeFn, res.LoadStringFromGzip("threejs/"+threeFn+".gz"),
			hammerFn, res.LoadStringFromGzip("threejs/"+hammerFn+".gz"),
			controlsFn, res.LoadString("threejs/"+controlsFn))
	}

	pf("var telesapr_model_%s = {\n"+
		"  bounds: {\n"+
		"    x: %f, y: %f, near: %f, far: %f, z: %f, edgeBias: %f\n"+
		"  },\n",
		basename,
		largerBoundXY,
		largerBoundXY,
		1.0,
		largerBoundZ*2,
		largerBoundZ,
		largerBoundZ/250)

	// Output lighting information.
	f.WriteString("  lights: {\n" +
		"    d: [\n")

	// Directional.
	for lightCount := range 2 {
		d := ss.LightDir[lightCount]
		pf("      {\n"+
			"        intensity: %f, direction: [%f, %f, %f]\n"+
			"      },\n",
			ss.LightIntensity[lightCount], d.X, d.Y, d.Z)
	}

	// Global Ambience.
	pf("    ],\n"+
		"    a: %f\n", ss.AmbientIntensity)

	var spl srf.SPointList
	for _, tr := range sm.Triangles {
		spl.IncrementTagFor(tr.A)
		spl.IncrementTagFor(tr.B)
		spl.IncrementTagFor(tr.C)
	}

	// Output all the vertices.
	f.WriteString("  },\n" +
		"  points: [\n")
	for _, sp := range spl.Points {
		pf("    [%f, %f, %f],\n",
			sp.P.X/ss.ExportScale, sp.P.Y/ss.ExportScale, sp.P.Z/ss.ExportScale)
	}

	f.WriteString("  ],\n" +
		"  faces: [\n")
	// And now all the triangular faces, in terms of those vertices. This
	// time we count from zero.
	for _, tr := range sm.Triangles {
		pf("    [%d, %d, %d],\n",
			spl.IndexForPoint(tr.A), spl.IndexForPoint(tr.B), spl.IndexForPoint(tr.C))
	}

	// Output face normals.
	f.WriteString("  ],\n" +
		"  normals: [\n")
	for _, tr := range sm.Triangles {
		pf("    [[%f, %f, %f], [%f, %f, %f], [%f, %f, %f]],\n",
			tr.NA.X, tr.NA.Y, tr.NA.Z, tr.NB.X, tr.NB.Y, tr.NB.Z, tr.NC.X, tr.NC.Y, tr.NC.Z)
	}

	f.WriteString("  ],\n" +
		"  colors: [\n")
	// Output triangle colors.
	for _, tr := range sm.Triangles {
		pf("    0x%x,\n", tr.Meta.ToARGB32())
	}

	f.WriteString("  ],\n" +
		"  edges: [\n")
	// Output edges. Assume user's model colors do not obscure white
	// edges.
	for _, so := range sol.Outlines {
		if so.Tag == 0 {
			continue
		}
		pf("    [[%f, %f, %f], [%f, %f, %f]],\n",
			so.A.X/ss.ExportScale, so.A.Y/ss.ExportScale, so.A.Z/ss.ExportScale,
			so.B.X/ss.ExportScale, so.B.Y/ss.ExportScale, so.B.Z/ss.ExportScale)
	}

	f.WriteString("  ]\n};\n")

	if hasExtension(filename, "html") {
		v := &ss.GW.view
		pf(threeJsHTMLEnd,
			basename,
			v.Scale,
			v.Offset.X, v.Offset.Y, v.Offset.Z,
			v.ProjUp.X, v.ProjUp.Y, v.ProjUp.Z,
			v.ProjRight.X, v.ProjRight.Y, v.ProjRight.Z)
	}
}

// ExportMeshAsVrmlTo writes the mesh as VRML (C++
// SolveSpaceUI::ExportMeshAsVrmlTo).
func (ss *SolveSpaceUI) ExportMeshAsVrmlTo(f *bytes.Buffer, filename string, sm *srf.SMesh) {
	pf := func(format string, args ...any) { f.WriteString(cfmt.Sprintf(format, args...)) }
	basename := fileStemIdentifier(filename)

	pf("#VRML V2.0 utf8\n"+
		"#Exported from TeleSAPR %s\n"+
		"\n"+
		"DEF %s Transform {\n"+
		"  children [",
		version.Version,
		basename)

	// The runs of triangles of each opacity, by opacity (C++'s
	// std::map<uint8_t, ...>).
	type span struct{ first, pastLast int }
	opacities := map[uint8][]span{}
	start := 0
	lastOpacity := sm.Triangles[0].Meta.Alpha
	for i, tr := range sm.Triangles {
		if tr.Meta.Alpha != lastOpacity {
			opacities[lastOpacity] = append(opacities[lastOpacity], span{start, i})
			start = i
			lastOpacity = sm.Triangles[start].Meta.Alpha
		}
	}
	opacities[lastOpacity] = append(opacities[lastOpacity], span{start, len(sm.Triangles)})
	var keys []int
	for k := range opacities {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	for _, k := range keys {
		op := uint8(k)
		spans := opacities[op]
		pf("\n"+
			"    Shape {\n"+
			"      appearance Appearance {\n"+
			"        material DEF %s_material_%u Material {\n"+
			"          diffuseColor %f %f %f\n"+
			"          ambientIntensity %f\n"+
			"          transparency %f\n"+
			"        }\n"+
			"      }\n"+
			"      geometry IndexedFaceSet {\n"+
			"        colorPerVertex TRUE\n"+
			"        coord Coordinate { point [\n",
			basename,
			uint(op),
			ss.AmbientIntensity,
			ss.AmbientIntensity,
			ss.AmbientIntensity,
			ss.AmbientIntensity,
			float64(float32(1)-(float32(op)/float32(255))))

		var spl srf.SPointList
		for _, sp := range spans {
			for _, tr := range sm.Triangles[sp.first:sp.pastLast] {
				spl.IncrementTagFor(tr.A)
				spl.IncrementTagFor(tr.B)
				spl.IncrementTagFor(tr.C)
			}
		}

		// Output all the vertices.
		for _, sp := range spl.Points {
			pf("          %f %f %f,\n",
				sp.P.X/ss.ExportScale, sp.P.Y/ss.ExportScale, sp.P.Z/ss.ExportScale)
		}

		f.WriteString("        ] }\n" +
			"        coordIndex [\n")
		// And now all the triangular faces, in terms of those vertices.
		for _, sp := range spans {
			for _, tr := range sm.Triangles[sp.first:sp.pastLast] {
				pf("          %d, %d, %d, -1,\n",
					spl.IndexForPoint(tr.A), spl.IndexForPoint(tr.B), spl.IndexForPoint(tr.C))
			}
		}

		f.WriteString("        ]\n" +
			"        color Color { color [\n")
		// Output triangle colors.
		var triangleColourIDs []int
		var coloursPresent []core.RgbaColor
		for _, sp := range spans {
			for _, tr := range sm.Triangles[sp.first:sp.pastLast] {
				found := -1
				for ci, c := range coloursPresent {
					if c.Equals(tr.Meta.RgbaColor) {
						found = ci
						break
					}
				}
				if found < 0 {
					pf("          %.10f %.10f %.10f,\n",
						tr.Meta.RedF(), tr.Meta.GreenF(), tr.Meta.BlueF())
					triangleColourIDs = append(triangleColourIDs, len(coloursPresent))
					coloursPresent = append(coloursPresent, tr.Meta.RgbaColor)
				} else {
					triangleColourIDs = append(triangleColourIDs, found)
				}
			}
		}

		f.WriteString("        ] }\n" +
			"        colorIndex [\n")

		for _, idx := range triangleColourIDs {
			pf("          %d, %d, %d, -1,\n", idx, idx, idx)
		}

		f.WriteString("        ]\n" +
			"      }\n" +
			"    }\n")
	}

	f.WriteString("  ]\n" +
		"}\n")
}

// ExportAsPngTo saves the view as an image at the next redraw (C++
// SolveSpaceUI::ExportAsPngTo).
func (ss *SolveSpaceUI) ExportAsPngTo(filename string) {
	ss.ScreenshotFile = filename
	// The rest of the work is done in the next redraw.
	ss.GW.Invalidate(false)
}

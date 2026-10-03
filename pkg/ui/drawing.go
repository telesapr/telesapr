package ui

import (
	"errors"
	"math"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A drawing's sheet (roadmap 18.1, H1, and 18.2, H4): each view is the
// solid of the active group seen along its direction, orthographic, its
// sharp edges and the outlines of its curved faces, with the hidden
// ones dashed in the orthographic views as drafting has them; the views
// placed on the sheet (sketch.Drawing.Layout) at its scale, in a frame
// with a title block. It is written by the vector writers of Export 2d
// View, on a canvas the size of the sheet, in mm.

// viewEdges are the edges of the solid seen with right and up as the
// view's x and y, in model mm about the origin, visible ones in the
// SolidEdge style and, with hidden, hidden ones in HiddenEdge.
func (ss *SolveSpaceUI) viewEdges(g *sketch.Group, right, up geom.Vector, hidden bool) srf.SEdgeList {
	sk := ss.SK
	n := right.Cross(up)
	var sel srf.SEdgeList
	g.GenerateDisplayItems(sk)
	g.DisplayOutlines.ListTaggedInto(&sel, core.StyleSolidEdge, 0)
	for i := range sel.Edges {
		e := &sel.Edges[i]
		e.A = e.A.InPerspective(right, up, n, geom.Vector{}, 0)
		e.B = e.B.InPerspective(right, up, n, geom.Vector{}, 0)
	}
	var smp srf.SMesh
	for _, tr := range g.DisplayMesh.Triangles {
		tt := tr
		tt.A = tt.A.InPerspective(right, up, n, geom.Vector{}, 0)
		tt.B = tt.B.InPerspective(right, up, n, geom.Vector{}, 0)
		tt.C = tt.C.InPerspective(right, up, n, geom.Vector{}, 0)
		smp.AddTriangle(tt)
	}
	if len(smp.Triangles) == 0 {
		return sel
	}
	root := srf.SKdNodeFrom(&smp)
	// The outlines of curved faces, where they turn away.
	root.MakeCertainEdgesInto(&sel, srf.EdgeTurning, false, nil, nil, core.StyleSolidEdge)
	root.ClearTags()
	var out srf.SEdgeList
	cnt := 1234
	for _, se := range sel.Edges {
		var parts srf.SEdgeList
		parts.AddEdge(se.A, se.B, se.AuxA, 0, 0)
		root.OcclusionTestLine(se, &parts, cnt)
		cnt++
		kept := parts.Edges[:0]
		for _, p := range parts.Edges {
			if p.Tag == 1 {
				if !hidden {
					continue
				}
				p.AuxA = core.StyleHiddenEdge
			}
			kept = append(kept, p)
		}
		parts.Edges = kept
		parts.MergeCollinearSegments(se.A, se.B)
		for _, p := range parts.Edges {
			out.AddEdge(p.A, p.B, p.AuxA, 0, 0)
		}
	}
	cleanExportEdges(sk, &out)
	return out
}

// box is the 2d box about the edges.
func box(el *srf.SEdgeList) (lo, hi geom.Vector) {
	lo = geom.Vector{X: math.Inf(1), Y: math.Inf(1)}
	hi = geom.Vector{X: math.Inf(-1), Y: math.Inf(-1)}
	for _, e := range el.Edges {
		for _, p := range []geom.Vector{e.A, e.B} {
			lo.X, lo.Y = math.Min(lo.X, p.X), math.Min(lo.Y, p.Y)
			hi.X, hi.Y = math.Max(hi.X, p.X), math.Max(hi.Y, p.Y)
		}
	}
	if len(el.Edges) == 0 {
		return geom.Vector{}, geom.Vector{}
	}
	return lo, hi
}

// vec is a direction as sketch.ViewDirection gives it.
func vec(a [3]float64) geom.Vector { return geom.Vector{X: a[0], Y: a[1], Z: a[2]} }

// sheetText adds text to el, its baseline from o, height mm high, in
// the style of the solid's edges, which prints black.
func sheetText(el *srf.SEdgeList, text string, height float64, o geom.Vector) {
	res.BuiltinVectorFont().Trace(height, o, geom.Vector{X: 1}, geom.Vector{Y: 1}, text, func(a, b geom.Vector) {
		el.AddEdge(a, b, core.StyleSolidEdge, 0, 0)
	})
}

// rect adds a rectangle from lo to hi to el in style.
func rect(el *srf.SEdgeList, lo, hi geom.Vector, style int) {
	c := []geom.Vector{lo, {X: hi.X, Y: lo.Y}, hi, {X: lo.X, Y: hi.Y}}
	for i := range c {
		el.AddEdge(c[i], c[(i+1)%4], style, 0, 0)
	}
}

// DrawingSheet is the drawing of the active group's solid: the edges of
// the sheet in mm from its bottom left, the views placed and the scale
// used.
func (ss *SolveSpaceUI) DrawingSheet(d *sketch.Drawing) (srf.SEdgeList, []sketch.DrawingView, float64, error) {
	var sheet srf.SEdgeList
	if err := d.Check(); err != nil {
		return sheet, nil, 0, err
	}
	g := ss.SK.GetGroup(ss.GW.activeGroupH())
	if g == nil || len(g.RunningShell.Surface) == 0 && len(g.RunningMesh.Triangles) == 0 {
		return sheet, nil, 0, errors.New("the active group has no solid to draw")
	}
	kinds := []string{"front", "back", "top", "bottom", "left", "right", "iso"}
	edges := map[string]srf.SEdgeList{}
	sizes := map[string][2]float64{}
	mids := map[string]geom.Vector{}
	for _, k := range kinds {
		r, u, _ := sketch.ViewDirection(k)
		el := ss.viewEdges(g, vec(r), vec(u), d.Hidden && k != "iso")
		lo, hi := box(&el)
		edges[k], sizes[k], mids[k] = el, [2]float64{hi.X - lo.X, hi.Y - lo.Y}, lo.Plus(hi).ScaledBy(0.5)
	}
	views, scale := d.Layout(sizes)
	for _, v := range views {
		at := geom.Vector{X: v.X, Y: v.Y}
		for _, e := range edges[v.Kind].Edges {
			a := e.A.Minus(mids[v.Kind]).ScaledBy(scale).Plus(at)
			b := e.B.Minus(mids[v.Kind]).ScaledBy(scale).Plus(at)
			a.Z, b.Z = 0, 0
			sheet.AddEdge(a, b, e.AuxA, 0, 0)
		}
		// The view's name below it.
		label := map[string]string{"front": "FRONT", "back": "BACK", "top": "TOP", "bottom": "BOTTOM",
			"left": "LEFT", "right": "RIGHT", "iso": "ISOMETRIC"}[v.Kind]
		w := res.BuiltinVectorFont().GetWidth(3, label)
		sheetText(&sheet, label, 3, geom.Vector{X: v.X - w/2, Y: v.Y - scale*sizes[v.Kind][1]/2 - 7})
	}
	ss.sheetFrame(&sheet, d, scale)
	return sheet, views, scale, nil
}

// sheetFrame adds the frame and the title block (roadmap 18.2).
func (ss *SolveSpaceUI) sheetFrame(el *srf.SEdgeList, d *sketch.Drawing, scale float64) {
	w, h := d.Size()
	b := sketch.SheetBorder
	rect(el, geom.Vector{X: b, Y: b}, geom.Vector{X: w - b, Y: h - b}, core.StyleOutline)
	// The title block: two rows of fields above two, in the bottom
	// right corner.
	tw, th := sketch.TitleBlockWidth, sketch.TitleBlockHeight
	lo := geom.Vector{X: w - b - tw, Y: b}
	rect(el, lo, geom.Vector{X: w - b, Y: b + th}, core.StyleOutline)
	row := th / 4
	for i := 1; i < 4; i++ {
		y := lo.Y + row*float64(i)
		el.AddEdge(geom.Vector{X: lo.X, Y: y}, geom.Vector{X: w - b, Y: y}, core.StyleSolidEdge, 0, 0)
	}
	// The halves of the three rows below the title's.
	mid := lo.X + tw/2
	el.AddEdge(geom.Vector{X: mid, Y: lo.Y}, geom.Vector{X: mid, Y: lo.Y + 3*row}, core.StyleSolidEdge, 0, 0)
	field := func(label, value string, x, y float64) {
		sheetText(el, label, 1.8, geom.Vector{X: x + 1.5, Y: y + row - 3})
		if value != "" {
			sheetText(el, value, 3.2, geom.Vector{X: x + 1.5, Y: y + 1.6})
		}
	}
	// Top: the title across; then the company; then number and
	// revision, and scale and sheet; then author and material.
	field("TITLE", d.Title, lo.X, lo.Y+3*row)
	field("COMPANY", d.Company, lo.X, lo.Y+2*row)
	field("DRAWING NO.", d.Number, lo.X, lo.Y+row)
	field("REVISION", d.Revision, mid, lo.Y+row)
	proj := "THIRD ANGLE"
	if d.FirstAngle {
		proj = "FIRST ANGLE"
	}
	// The vector font has no colon: the scale is written 1/2.
	field("SCALE / SHEET / PROJECTION", strings.ReplaceAll(sketch.ScaleText(scale), ":", "/")+"  "+strings.ToUpper(d.Sheet)+"  "+proj, mid, lo.Y)
	field("DRAWN BY", d.Author, lo.X, lo.Y)
	if d.Material != "" {
		field("MATERIAL", d.Material, mid, lo.Y+2*row)
	}
}

// ExportDrawingTo writes the drawing to a vector file: PDF, SVG, DXF,
// EPS and the others Export 2d View writes, on a canvas the size of the
// sheet.
func (ss *SolveSpaceUI) ExportDrawingTo(filename string, d *sketch.Drawing) error {
	ss.ExportMode = true
	ss.GenerateAll(sketch.GenerateAll)
	sheet, _, _, err := ss.DrawingSheet(d)
	if err != nil {
		return err
	}
	out := ss.vectorFileWriterFor(filename)
	if out == nil {
		return errors.New("can't tell what to write from the name " + filename)
	}
	// The canvas is the sheet, in mm, as written.
	w, h := d.Size()
	auto, canvas, scale := ss.ExportCanvasSizeAuto, ss.ExportCanvas, ss.ExportScale
	defer func() { ss.ExportCanvasSizeAuto, ss.ExportCanvas, ss.ExportScale = auto, canvas, scale }()
	ss.ExportCanvasSizeAuto, ss.ExportScale = false, 1
	ss.ExportCanvas.Width, ss.ExportCanvas.Height, ss.ExportCanvas.Dx, ss.ExportCanvas.Dy = w, h, 0, 0
	out.vw().SetModelviewProjection(geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Vector{Z: 1}, geom.Vector{}, 0, 1)
	var sbl srf.SBezierList
	var sms srf.SMesh
	writeExportCurves(&sheet, &sbl, &sms, ss.ExportChordTolMm(), out)
	return nil
}

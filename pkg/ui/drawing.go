package ui

import (
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/export"
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
// SolidEdge style and, with hidden, hidden ones in HiddenEdge. Its
// threads are drawn by the convention (drawing_thread.go) unless
// modelled says to draw every turn of their ridges.
func (ss *Application) viewEdges(g *sketch.Group, right, up geom.Vector, hidden, modelled bool) srf.SEdgeList {
	g.GenerateDisplayItems(ss.SK)
	var sel srf.SEdgeList
	g.DisplayOutlines.ListTaggedInto(&sel, core.StyleSolidEdge, 0)
	if bands := ss.SK.ThreadBands(g.H); len(bands) > 0 && !modelled {
		m, extra, always := threadConvention(&sel, &g.DisplayMesh, bands, right, up)
		return ss.hiddenLineEdgesWith(sel, extra, always, &m, right, up, hidden)
	}
	return ss.hiddenLineEdges(sel, &g.DisplayMesh, right, up, hidden)
}

// hiddenLineEdges projects the edges sel and the mesh m, both in model
// coordinates, with right and up as x and y, adds the outlines where
// m's curved faces turn away, and splits each edge where m hides it:
// the hidden parts in the HiddenEdge style, or left out without hidden.
func (ss *Application) hiddenLineEdges(sel srf.SEdgeList, m *srf.SMesh, right, up geom.Vector, hidden bool) srf.SEdgeList {
	return ss.hiddenLineEdgesWith(sel, srf.SEdgeList{}, srf.SEdgeList{}, m, right, up, hidden)
}

// hiddenLineEdgesWith is hiddenLineEdges with the edges extra split as
// sel's are, after the outlines, and the edges always, which nothing
// hides, projected and added last.
func (ss *Application) hiddenLineEdgesWith(sel, extra, always srf.SEdgeList, m *srf.SMesh, right, up geom.Vector, hidden bool) srf.SEdgeList {
	n := right.Cross(up)
	project := func(el *srf.SEdgeList) {
		for i := range el.Edges {
			e := &el.Edges[i]
			e.A = e.A.InPerspective(right, up, n, geom.Vector{}, 0)
			e.B = e.B.InPerspective(right, up, n, geom.Vector{}, 0)
		}
	}
	project(&sel)
	project(&extra)
	project(&always)
	var smp srf.SMesh
	for _, tr := range m.Triangles {
		tt := tr
		tt.A = tt.A.InPerspective(right, up, n, geom.Vector{}, 0)
		tt.B = tt.B.InPerspective(right, up, n, geom.Vector{}, 0)
		tt.C = tt.C.InPerspective(right, up, n, geom.Vector{}, 0)
		smp.AddTriangle(tt)
	}
	if len(smp.Triangles) == 0 {
		sel.Edges = append(append(sel.Edges, extra.Edges...), always.Edges...)
		return sel
	}
	root := srf.SKdNodeFrom(&smp)
	// The outlines of curved faces, where they turn away.
	root.MakeCertainEdgesInto(&sel, srf.EdgeTurning, false, nil, nil, core.StyleSolidEdge)
	root.ClearTags()
	sel.Edges = append(sel.Edges, extra.Edges...)
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
	for _, e := range always.Edges {
		out.AddEdge(e.A, e.B, e.AuxA, 0, 0)
	}
	cleanExportEdges(ss.SK, &out)
	return out
}

// meshMiddle is the middle of the box about a mesh.
func meshMiddle(m *srf.SMesh) geom.Vector {
	lo := geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hi := lo.ScaledBy(-1)
	for _, tr := range m.Triangles {
		for _, p := range []geom.Vector{tr.A, tr.B, tr.C} {
			p.MakeMaxMin(&hi, &lo)
		}
	}
	return lo.Plus(hi).ScaledBy(0.5)
}

// sectionPlane is the plane of a section of the view with right and
// up: square to it, through the solid's middle moved offset toward the
// viewer; it keeps what is beyond.
func sectionPlane(m *srf.SMesh, right, up geom.Vector, offset float64) srf.HalfSpace {
	n := right.Cross(up).WithMagnitude(1)
	return srf.HalfSpace{N: n, D: n.Dot(meshMiddle(m)) + offset}
}

// clipMesh is the part of m in keep, each triangle cut by its plane.
func clipMesh(m *srf.SMesh, keep srf.HalfSpace) srf.SMesh {
	var out srf.SMesh
	for _, tr := range m.Triangles {
		vs := []geom.Vector{tr.A, tr.B, tr.C}
		var poly []geom.Vector
		for k := range vs {
			a, b := vs[k], vs[(k+1)%3]
			da, db := keep.N.Dot(a)-keep.D, keep.N.Dot(b)-keep.D
			if da <= 0 {
				poly = append(poly, a)
			}
			if (da < 0) != (db < 0) && da != db {
				poly = append(poly, a.Plus(b.Minus(a).ScaledBy(da/(da-db))))
			}
		}
		for k := 2; k < len(poly); k++ {
			t := tr
			t.A, t.B, t.C = poly[0], poly[k-1], poly[k]
			out.AddTriangle(t)
		}
	}
	return out
}

// clipEdges is the parts of the edges in keep.
func clipEdges(el *srf.SEdgeList, keep srf.HalfSpace) srf.SEdgeList {
	var out srf.SEdgeList
	for _, e := range el.Edges {
		da, db := keep.N.Dot(e.A)-keep.D, keep.N.Dot(e.B)-keep.D
		switch {
		case da <= 0 && db <= 0:
			out.AddEdge(e.A, e.B, e.AuxA, e.AuxB, 0)
		case da <= 0 || db <= 0:
			x := e.A.Plus(e.B.Minus(e.A).ScaledBy(da / (da - db)))
			if da <= 0 {
				out.AddEdge(e.A, x, e.AuxA, e.AuxB, 0)
			} else {
				out.AddEdge(x, e.B, e.AuxA, e.AuxB, 0)
			}
		}
	}
	return out
}

// sectionEdges are the edges of the section of the view with right and
// up by the plane offset mm toward the viewer from the solid's middle,
// as viewEdges gives a view's: the half beyond, with the cut face's
// outline and a hatch every hatch mm across it, which nothing hides.
// Its threads are drawn by the convention unless modelled says not to:
// the cut face reaches the crest, the hatch with it, and the root is a
// thin line on it (drawing_thread.go).
func (ss *Application) sectionEdges(g *sketch.Group, right, up geom.Vector, offset, hatch float64, hidden, modelled bool) srf.SEdgeList {
	g.GenerateDisplayItems(ss.SK)
	keep := sectionPlane(&g.DisplayMesh, right, up, offset)
	var outlines srf.SEdgeList
	g.DisplayOutlines.ListTaggedInto(&outlines, core.StyleSolidEdge, 0)
	sel := clipEdges(&outlines, keep)
	m := clipMesh(&g.DisplayMesh, keep)
	var capPoly srf.SPolygon
	g.DisplayMesh.MakeSectionPolygonInto(keep, &capPoly)
	var bands []sketch.ThreadBand
	if !modelled {
		bands = ss.SK.ThreadBands(g.H)
	}
	var extra, always srf.SEdgeList
	if len(bands) > 0 {
		var conv srf.SMesh
		conv, extra, always = threadConvention(&sel, &m, bands, right, up)
		m = clipMesh(&conv, keep)
		// The roots of the view are its silhouettes, which the cut's own
		// take the place of.
		notRoot := func(el *srf.SEdgeList) srf.SEdgeList {
			el.Edges = slices.DeleteFunc(el.Edges, func(e srf.SEdge) bool { return e.AuxA == core.StyleDimSolid })
			return clipEdges(el, keep)
		}
		extra, always = notRoot(&extra), notRoot(&always)
		// A plane through a thread's axis passes through the corners of
		// its turned surfaces' quarters, where the section's contours
		// come apart; a tenth of a micrometre off it they close, and are
		// laid back on it.
		capPoly.Clear()
		nudged := keep
		nudged.D += 1e-4
		g.DisplayMesh.MakeSectionPolygonInto(nudged, &capPoly)
		for ci := range capPoly.Contours {
			for i := range capPoly.Contours[ci].Points {
				p := &capPoly.Contours[ci].Points[i].P
				*p = p.Minus(keep.N.WithMagnitude(1).ScaledBy(1e-4 / keep.N.Magnitude()))
			}
		}
		threadCut(&capPoly, bands, keep.N)
		threadCutRoots(&always, bands, keep)
	}
	// The cut face hides what is beyond it.
	var capMesh srf.SMesh
	capPoly.TriangulateInto(&capMesh)
	m.Triangles = append(m.Triangles, capMesh.Triangles...)
	out := ss.hiddenLineEdgesWith(sel, extra, always, &m, right, up, hidden)

	n := right.Cross(up)
	var front srf.SEdgeList
	capPoly.MakeEdgesInto(&front)
	hatchLines(&capPoly, keep.N, hatch, &front)
	for _, e := range front.Edges {
		out.AddEdge(e.A.InPerspective(right, up, n, geom.Vector{}, 0), e.B.InPerspective(right, up, n, geom.Vector{}, 0),
			core.StyleSolidEdge, 0, 0)
	}
	return out
}

// clipToCircle is the parts of the edges, in a view's x and y, within r
// of c.
func clipToCircle(el *srf.SEdgeList, c geom.Vector, r float64) srf.SEdgeList {
	var out srf.SEdgeList
	for _, e := range el.Edges {
		a, b := e.A.Minus(c), e.B.Minus(c)
		a.Z, b.Z = 0, 0
		d := b.Minus(a)
		// |a + t d| = r: t of where the segment crosses the circle.
		qa, qb, qc := d.Dot(d), 2*a.Dot(d), a.Dot(a)-r*r
		t0, t1 := 0.0, 1.0
		if qa < 1e-18 {
			if qc > 0 {
				continue
			}
		} else {
			disc := qb*qb - 4*qa*qc
			if disc <= 0 {
				continue
			}
			s := math.Sqrt(disc)
			t0, t1 = math.Max(0, (-qb-s)/(2*qa)), math.Min(1, (-qb+s)/(2*qa))
			if t0 >= t1 {
				continue
			}
		}
		out.AddEdge(e.A.Plus(e.B.Minus(e.A).ScaledBy(t0)), e.A.Plus(e.B.Minus(e.A).ScaledBy(t1)), e.AuxA, 0, 0)
	}
	return out
}

// circle adds a circle about c of radius r to el, in style.
func circle(el *srf.SEdgeList, c geom.Vector, r float64, style int) {
	const k = 72
	for i := range k {
		a0, a1 := 2*math.Pi*float64(i)/k, 2*math.Pi*float64(i+1)/k
		el.AddEdge(c.Plus(geom.Vector{X: r * math.Cos(a0), Y: r * math.Sin(a0)}),
			c.Plus(geom.Vector{X: r * math.Cos(a1), Y: r * math.Sin(a1)}), style, 0, 0)
	}
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
func (ss *Application) DrawingSheet(d *sketch.Drawing) (srf.SEdgeList, []sketch.DrawingView, float64, error) {
	var sheet srf.SEdgeList
	if err := d.Check(); err != nil {
		return sheet, nil, 0, err
	}
	g := ss.SK.Group(ss.GW.activeGroupH())
	if g == nil || len(g.RunningShell.Surface) == 0 && len(g.RunningMesh.Triangles) == 0 {
		return sheet, nil, 0, errors.New("the active group has no solid to draw")
	}
	kinds := []string{"front", "back", "top", "bottom", "left", "right", "iso"}
	edges := map[string]srf.SEdgeList{}
	sizes := map[string][2]float64{}
	mids := map[string]geom.Vector{}
	for _, k := range kinds {
		r, u, _ := sketch.ViewDirection(k)
		el := ss.viewEdges(g, vec(r), vec(u), d.Hidden && k != "iso", d.ModelledThreads)
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
		w := res.BuiltinVectorFont().Width(3, label)
		sheetText(&sheet, label, 3, geom.Vector{X: v.X - w/2, Y: v.Y - scale*sizes[v.Kind][1]/2 - 7})
	}
	extras := ss.drawExtras(&sheet, g, d, views, scale, edges, sizes, mids)
	ss.drawDimensions(&sheet, g, d, views, scale, mids, extras)
	ss.sheetFrame(&sheet, d, scale)
	return sheet, views, scale, nil
}

// label writes text centred under a view whose box on the sheet is
// height high about y.
func label(el *srf.SEdgeList, text string, x, y, height float64) {
	w := res.BuiltinVectorFont().Width(3, text)
	sheetText(el, text, 3, geom.Vector{X: x - w/2, Y: y - height/2 - 7})
}

// drawExtras adds the section and detail views (roadmap 18.3): each
// placed by the drawing or in columns right of the views, lettered, a
// section's cutting plane drawn on the views that see it edge on, a
// detail's circle on its view. It returns where each went, as
// dimensions on it need (roadmap 29.2), the sections' first.
func (ss *Application) drawExtras(sheet *srf.SEdgeList, g *sketch.Group, d *sketch.Drawing, views []sketch.DrawingView,
	scale float64, edges map[string]srf.SEdgeList, sizes map[string][2]float64, mids map[string]geom.Vector) []dimView {
	if len(d.Sections)+len(d.Details) == 0 {
		return nil
	}
	placed := map[string]sketch.DrawingView{}
	right := sketch.SheetBorder
	for _, v := range views {
		placed[v.Kind] = v
		right = math.Max(right, v.X+scale*sizes[v.Kind][0]/2)
	}
	type extra struct {
		el   srf.SEdgeList
		mid  geom.Vector
		size [2]float64 // on the sheet
		k    float64    // sheet mm per model mm
	}
	var extras []extra
	for _, sec := range d.Sections {
		r, u, _ := sketch.ViewDirection(sec.View)
		el := ss.sectionEdges(g, vec(r), vec(u), sec.Offset, 3/scale, d.Hidden, d.ModelledThreads)
		lo, hi := box(&el)
		extras = append(extras, extra{el, lo.Plus(hi).ScaledBy(0.5), [2]float64{scale * (hi.X - lo.X), scale * (hi.Y - lo.Y)}, scale})
	}
	for _, dt := range d.Details {
		c := mids[dt.View].Plus(geom.Vector{X: dt.CX, Y: dt.CY})
		el := edgesOf(edges, dt.View)
		el = clipToCircle(&el, c, dt.R)
		k := scale * dt.Scale
		extras = append(extras, extra{el, c, [2]float64{2 * dt.R * k, 2 * dt.R * k}, k})
	}
	sizesOn := make([][2]float64, len(extras))
	for i, e := range extras {
		sizesOn[i] = e.size
	}
	at := d.PlaceExtras(right, sizesOn)
	out := make([]dimView, len(extras))
	for i, e := range extras {
		p := geom.Vector{X: at[i][0], Y: at[i][1]}
		kind := ""
		if i < len(d.Sections) {
			kind = d.Sections[i].View
		} else {
			kind = d.Details[i-len(d.Sections)].View
		}
		r, u, _ := sketch.ViewDirection(kind)
		out[i] = dimView{right: vec(r), up: vec(u), mid: e.mid, at: p, scale: e.k, pictorial: kind == "iso"}
		if i >= len(d.Sections) {
			out[i].clip, out[i].clipR = true, d.Details[i-len(d.Sections)].R
		}
		for _, ed := range e.el.Edges {
			a := ed.A.Minus(e.mid).ScaledBy(e.k).Plus(p)
			b := ed.B.Minus(e.mid).ScaledBy(e.k).Plus(p)
			a.Z, b.Z = 0, 0
			sheet.AddEdge(a, b, ed.AuxA, 0, 0)
		}
		letter := sketch.Letter(i)
		if i < len(d.Sections) {
			label(sheet, "SECTION "+letter+"-"+letter, p.X, p.Y, e.size[1])
			ss.cuttingPlane(sheet, g, d.Sections[i], letter, placed, scale, sizes, mids)
			continue
		}
		dt := d.Details[i-len(d.Sections)]
		circle(sheet, p, e.size[0]/2, core.StyleSolidEdge)
		label(sheet, "DETAIL "+letter+"  "+strings.ReplaceAll(sketch.ScaleText(e.k), ":", "/"), p.X, p.Y, e.size[1])
		// Its circle on the view it enlarges, lettered.
		if v, ok := placed[dt.View]; ok {
			c := geom.Vector{X: v.X + scale*dt.CX, Y: v.Y + scale*dt.CY}
			circle(sheet, c, scale*dt.R, core.StyleHiddenEdge)
			sheetText(sheet, letter, 3.5, c.Plus(geom.Vector{X: scale*dt.R*0.75 + 1, Y: scale*dt.R*0.75 + 1}))
		}
	}
	return out
}

// edgesOf is a copy of a view's edges.
func edgesOf(edges map[string]srf.SEdgeList, kind string) srf.SEdgeList {
	var el srf.SEdgeList
	el.Edges = append(el.Edges, edges[kind].Edges...)
	return el
}

// cuttingPlane draws a section's plane, dashed and lettered at its
// ends, on each placed view that sees it edge on.
func (ss *Application) cuttingPlane(sheet *srf.SEdgeList, g *sketch.Group, sec sketch.DrawingSection, letter string,
	placed map[string]sketch.DrawingView, scale float64, sizes map[string][2]float64, mids map[string]geom.Vector) {
	r, u, _ := sketch.ViewDirection(sec.View)
	keep := sectionPlane(&g.DisplayMesh, vec(r), vec(u), sec.Offset)
	for kind, v := range placed {
		if kind == "iso" {
			continue
		}
		vr, vu, _ := sketch.ViewDirection(kind)
		rr, uu := vec(vr), vec(vu)
		if math.Abs(rr.Cross(uu).Dot(keep.N)) > 1e-9 {
			continue
		}
		// In the view: a x + b y = D, its points' x and y the view's.
		a, b := keep.N.Dot(rr), keep.N.Dot(uu)
		base := geom.Vector{X: a * keep.D, Y: b * keep.D}
		dir := geom.Vector{X: -b, Y: a}
		half := math.Hypot(sizes[kind][0], sizes[kind][1])/2 + 6/scale
		mid := mids[kind]
		// From the view's middle along the line.
		t0 := mid.Minus(base).Dot(dir)
		toSheet := func(p geom.Vector) geom.Vector {
			q := p.Minus(mid).ScaledBy(scale).Plus(geom.Vector{X: v.X, Y: v.Y})
			q.Z = 0
			return q
		}
		// Clipped to the view's box, grown a little.
		hw, hh := sizes[kind][0]/2+5/scale, sizes[kind][1]/2+5/scale
		var ends []geom.Vector
		for _, t := range []float64{t0 - half, t0 + half} {
			p := base.Plus(dir.ScaledBy(t))
			p.X = math.Max(mid.X-hw, math.Min(mid.X+hw, p.X))
			p.Y = math.Max(mid.Y-hh, math.Min(mid.Y+hh, p.Y))
			ends = append(ends, toSheet(p))
		}
		sheet.AddEdge(ends[0], ends[1], core.StyleHiddenEdge, 0, 0)
		for _, e := range ends {
			sheetText(sheet, letter, 3.5, e.Plus(geom.Vector{X: 1.5, Y: 1.5}))
		}
	}
}

// sheetFrame adds the frame and the title block (roadmap 18.2).
func (ss *Application) sheetFrame(el *srf.SEdgeList, d *sketch.Drawing, scale float64) {
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
func (ss *Application) ExportDrawingTo(filename string, d *sketch.Drawing) error {
	ss.EnterExportMode()
	ss.GenerateAll(sketch.GenerateAll)
	sheet, _, _, err := ss.DrawingSheet(d)
	if err != nil {
		return err
	}
	// The canvas is the sheet, in mm, as written.
	o := ss.exportOptions()
	w, h := d.Size()
	o.CanvasSizeAuto, o.Scale = false, 1
	o.Canvas = export.Canvas{Width: w, Height: h}
	out, err := export.NewWriter(filename, ss.SK, o)
	if err != nil {
		return err
	}
	out.SetModelviewProjection(geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Vector{Z: 1}, geom.Vector{}, 0, 1)
	var sbl srf.SBezierList
	var sms srf.SMesh
	return export.WriteCurves(&sheet, &sbl, &sms, ss.exportTolerance(), out)
}

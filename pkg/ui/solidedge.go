package ui

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// The edges of the solid, picked on the model: the topology of the
// solid at the active group names them (topo.Name), and a selection
// holds the name, so it means the same edge after a regeneration.

// solidEdgePickRadius is how near the mouse, in pixels, an edge of the
// solid hovers; less than an entity's, which wins where both are.
const solidEdgePickRadius = 6.0

// activeTopology is the topology of the solid at the active group,
// and that solid's group, or nil.
func (gw *graphicsWindow) activeTopology() *topo.Topology {
	sk := gw.sk()
	g := sk.SolidAt(gw.activeGroupH())
	if g == nil || g.IsForcedToMesh(sk) || len(g.RunningShell.Surface) == 0 {
		return nil
	}
	tp, _ := g.Topology()
	return tp
}

// topologyOf is the topology of the running solid of group h, or of
// the active group's solid when h is zero; nil when there is none.
func (gw *graphicsWindow) topologyOf(h core.HGroup) *topo.Topology {
	if h.V == 0 {
		return gw.activeTopology()
	}
	sk := gw.sk()
	g := sk.Group(h)
	if g == nil || g.IsForcedToMesh(sk) || len(g.RunningShell.Surface) == 0 {
		return nil
	}
	tp, _ := g.Topology()
	return tp
}

// hoverSolidEdge is the named edge of the solid nearest mp on the
// screen, within the pick radius and not hidden behind the solid, and
// the point of it nearest mp.
func (gw *graphicsWindow) hoverSolidEdge(mp geom.Point2d) (topo.Name, geom.Vector, bool) {
	tp := gw.activeTopology()
	if tp == nil {
		return topo.Name{}, geom.Vector{}, false
	}
	best, bestD := -1, solidEdgePickRadius
	var bestP geom.Vector
	for e := range tp.Edges {
		if tp.Edges[e].Seam {
			continue
		}
		pts := tp.Edges[e].Points
		for i := 1; i < len(pts); i++ {
			a, b := gw.ProjectPoint(pts[i-1]), gw.ProjectPoint(pts[i])
			d, t := segmentDistance2d(mp, a, b)
			if d < bestD {
				best, bestD = e, d
				bestP = pts[i-1].Plus(pts[i].Minus(pts[i-1]).ScaledBy(t))
			}
		}
	}
	if best < 0 {
		return topo.Name{}, geom.Vector{}, false
	}
	// The piece of the edge that names it, for a circle the arc whose
	// name is lowest, so that every piece picks the same edge.
	n, err := tp.NameOf(tp.Lead(best))
	if err != nil || gw.hiddenBySolid(bestP) {
		return topo.Name{}, geom.Vector{}, false
	}
	return n, bestP, true
}

// segmentDistance2d is how far p is from the segment a-b on the
// screen, and where along it the nearest point is, 0 to 1.
func segmentDistance2d(p, a, b geom.Point2d) (float64, float64) {
	du, dv := b.U-a.U, b.V-a.V
	l := du*du + dv*dv
	t := 0.0
	if l > 0 {
		t = math.Max(0, math.Min(1, ((p.U-a.U)*du+(p.V-a.V)*dv)/l))
	}
	return math.Hypot(p.U-a.U-t*du, p.V-a.V-t*dv), t
}

// hiddenBySolid reports whether a point of the solid's surface is
// behind a face of the active group's mesh, as seen: whether the ray
// from it towards the viewer meets a triangle before the viewer. That
// ray stays on the point's pixel, along the view direction in an
// orthographic view and towards the eye in a perspective one, so only
// the triangles over that pixel are tried (hoverGrid).
func (gw *graphicsWindow) hiddenBySolid(p geom.Vector) bool {
	g := gw.sk().Group(gw.activeGroupH())
	if g == nil {
		return false
	}
	gw.view.Tangent = gw.ss.CameraTangent()
	towards, reach := gw.towardsViewer(p)
	scale := 1e-3 / gw.view.Scale
	if scale < 1e-6 {
		scale = 1e-6
	}
	// From just in front of the point.
	start := p.Plus(towards.ScaledBy(10 * scale))
	tris := g.DisplayMesh.Triangles
	h := gw.hoverGridFor(tris)
	for _, list := range [][]int32{h.at(gw.view.ProjectPoint(p)), h.behind} {
		for _, i := range list {
			if t, ok := rayHit(start, towards, &tris[i]); ok && t < reach {
				return true
			}
		}
	}
	return false
}

// towardsViewer is the direction from p towards the viewer, and how
// far the viewer is: along the view direction without end in an
// orthographic view, towards the eye in a perspective one, which is
// where ProjectPoint4's w is zero.
func (gw *graphicsWindow) towardsViewer(p geom.Vector) (geom.Vector, float64) {
	v := &gw.view
	if v.Tangent == 0 {
		// A projected point's z grows towards the viewer.
		return v.ProjRight.Cross(v.ProjUp).WithMagnitude(1), math.Inf(1)
	}
	n := v.ProjUp.Cross(v.ProjRight)
	eye := v.Offset.ScaledBy(-1).Plus(n.ScaledBy(-1 / (v.Tangent * v.Scale)))
	d := eye.Minus(p)
	return d.WithMagnitude(1), d.Magnitude()
}

// hoverGrid buckets the triangles of a mesh by the pixels they cover in
// one view, so that hiddenBySolid tries only those over its point
// (roadmap 14.4). It is kept until the view or the mesh changes.
type hoverGrid struct {
	view  sketch.View
	first *srf.STriangle
	n     int
	lo    geom.Point2d
	cell  float64
	nx    int
	cells [][]int32
	// behind are the triangles that reach behind the eye of a
	// perspective view, whose projected corners don't bound them on
	// the screen; they are tried at every point.
	behind []int32
}

// hoverGridCells is how many cells the grid has along its longer side.
const hoverGridCells = 64

// hoverGridFor is the grid of tris in the current view, made again if
// the view or the mesh changed.
func (gw *graphicsWindow) hoverGridFor(tris []srf.STriangle) *hoverGrid {
	var first *srf.STriangle
	if len(tris) > 0 {
		first = &tris[0]
	}
	if h := gw.hoverTris; h != nil && h.view == gw.view && h.first == first && h.n == len(tris) {
		return h
	}
	h := &hoverGrid{view: gw.view, first: first, n: len(tris)}
	gw.hoverTris = h
	if len(tris) == 0 {
		return h
	}
	type box struct{ lo, hi geom.Point2d }
	boxes := make([]box, len(tris))
	lo := geom.Point2d{U: math.Inf(1), V: math.Inf(1)}
	hi := geom.Point2d{U: math.Inf(-1), V: math.Inf(-1)}
	for i := range tris {
		b := box{lo: geom.Point2d{U: math.Inf(1), V: math.Inf(1)}, hi: geom.Point2d{U: math.Inf(-1), V: math.Inf(-1)}}
		for _, v := range []geom.Vector{tris[i].A, tris[i].B, tris[i].C} {
			if _, w := gw.view.ProjectPoint4(v); w <= 0 {
				h.behind = append(h.behind, int32(i))
				b = box{lo: geom.Point2d{U: math.Inf(1), V: math.Inf(1)}, hi: geom.Point2d{U: math.Inf(-1), V: math.Inf(-1)}}
				break
			}
			q := gw.view.ProjectPoint(v)
			b.lo.U, b.lo.V = math.Min(b.lo.U, q.U), math.Min(b.lo.V, q.V)
			b.hi.U, b.hi.V = math.Max(b.hi.U, q.U), math.Max(b.hi.V, q.V)
		}
		boxes[i] = b
		lo.U, lo.V = math.Min(lo.U, b.lo.U), math.Min(lo.V, b.lo.V)
		hi.U, hi.V = math.Max(hi.U, b.hi.U), math.Max(hi.V, b.hi.V)
	}
	// A pixel of slack about every box, for points on an edge.
	const slack = 1
	h.lo = geom.Point2d{U: lo.U - slack, V: lo.V - slack}
	h.cell = math.Max(hi.U-lo.U, hi.V-lo.V)/hoverGridCells + 2*slack
	h.nx = int((hi.U-lo.U+2*slack)/h.cell) + 1
	ny := int((hi.V-lo.V+2*slack)/h.cell) + 1
	h.cells = make([][]int32, h.nx*ny)
	for i, b := range boxes {
		if math.IsInf(b.lo.U, 1) {
			continue
		}
		x0, y0 := h.index(geom.Point2d{U: b.lo.U - slack, V: b.lo.V - slack})
		x1, y1 := h.index(geom.Point2d{U: b.hi.U + slack, V: b.hi.V + slack})
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				h.cells[y*h.nx+x] = append(h.cells[y*h.nx+x], int32(i))
			}
		}
	}
	return h
}

// index is the cell of a screen point, clamped to the grid.
func (h *hoverGrid) index(p geom.Point2d) (int, int) {
	ny := len(h.cells) / h.nx
	x := min(max(int((p.U-h.lo.U)/h.cell), 0), h.nx-1)
	y := min(max(int((p.V-h.lo.V)/h.cell), 0), ny-1)
	return x, y
}

// at lists the triangles whose cell holds the screen point p.
func (h *hoverGrid) at(p geom.Point2d) []int32 {
	if len(h.cells) == 0 || p.U < h.lo.U || p.V < h.lo.V {
		return nil
	}
	x, y := h.index(p)
	if p.U > h.lo.U+float64(h.nx)*h.cell || p.V > h.lo.V+float64(len(h.cells)/h.nx)*h.cell {
		return nil
	}
	return h.cells[y*h.nx+x]
}

// rayHit is the Moller and Trumbore test, for t > 0: whether the ray
// o + t d meets the triangle, and at what t.
func rayHit(o, d geom.Vector, t *srf.STriangle) (float64, bool) {
	e1, e2 := t.B.Minus(t.A), t.C.Minus(t.A)
	p := d.Cross(e2)
	det := e1.Dot(p)
	if math.Abs(det) < 1e-12 {
		return 0, false
	}
	inv := 1 / det
	s := o.Minus(t.A)
	u := s.Dot(p) * inv
	if u < 0 || u > 1 {
		return 0, false
	}
	q := s.Cross(e1)
	v := d.Dot(q) * inv
	if v < 0 || u+v > 1 {
		return 0, false
	}
	at := e2.Dot(q) * inv
	return at, at > 0
}

// drawSolidEdge draws an edge of the solid hovered or selected, all of
// it: every piece its seams cut it into.
func (gw *graphicsWindow) drawSolidEdge(n topo.Name, solid core.HGroup, hovered bool, canvas render.Canvas) {
	tp := gw.topologyOf(solid)
	if tp == nil {
		return
	}
	e, err := tp.Resolve(n)
	if err != nil {
		return
	}
	style := uint32(core.StyleSelected)
	if hovered {
		style = core.StyleHovered
	}
	stroke := gw.ss.styleStroke(hstyle(style))
	stroke.Layer = render.LayerFront
	stroke.Unit = render.UnitPX
	stroke.Width = 4
	h := canvas.InternStroke(stroke)
	pts := tp.ChainPoints(tp.Chain(e))
	for i := 1; i < len(pts); i++ {
		canvas.DrawLine(pts[i-1], pts[i], h)
	}
}

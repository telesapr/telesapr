package render

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ObjectPicker is a canvas that draws nothing but finds the geometry
// nearest Point, within SelRadius pixels; higher z-indexes win.
type ObjectPicker struct {
	CanvasBase
	Cam Camera
	// Configuration.
	Point     geom.Point2d
	SelRadius float64
	// ClipToView leaves out what lies outside the camera's Width by
	// Height: a line just past the window's edge is not seen, so the
	// mouse inside the window must not find it there.
	ClipToView bool
	// Picking state.
	MinDistance float64
	MinDepth    float64
	MaxZIndex   int
	Position    uint32
	// MinAt is where the best candidate is near the mouse, in model
	// space, where AtKnown is set: of the points of lines, edges and
	// points (a quad or text gives none) within a pixel of the best
	// distance, the one nearest the viewer, so that a circle seen edge
	// on is found at its front half, not at the half behind it. The
	// window asks whether a face hides it there.
	MinAt   geom.Vector
	AtKnown bool
	atDepth float64
}

// atSlack is how much further from the mouse than the best candidate,
// in pixels, a point may be and still be where it is found.
const atSlack = 1.0

var _ Canvas = (*ObjectPicker)(nil)

// Camera returns the picker's camera.
func (op *ObjectPicker) Camera() *Camera { return &op.Cam }

// DoCompare records a candidate at the given distance.
func (op *ObjectPicker) DoCompare(depth, distance float64, zIndex int, comparePosition uint32) {
	op.compareAt(depth, distance, zIndex, comparePosition, geom.Vector{}, false)
}

// compareAt is DoCompare for a candidate whose point nearest the mouse,
// at, is known.
func (op *ObjectPicker) compareAt(depth, distance float64, zIndex int, comparePosition uint32, at geom.Vector, known bool) {
	if distance > op.SelRadius {
		return
	}
	if (zIndex == op.MaxZIndex && distance < op.MinDistance) || zIndex > op.MaxZIndex {
		if zIndex > op.MaxZIndex || distance < op.MinDistance-atSlack {
			// Clearly better: what was found before is not where this is.
			op.AtKnown = false
		}
		op.MinDepth = depth
		op.MinDistance = distance
		op.MaxZIndex = zIndex
		op.Position = comparePosition
	}
	if known && zIndex == op.MaxZIndex && distance <= op.MinDistance+atSlack {
		// Depth grows away from the viewer.
		if d := op.Cam.ProjectPoint3(at).Z; !op.AtKnown || d < op.atDepth {
			op.MinAt, op.atDepth, op.AtKnown = at, d, true
		}
	}
}

// nearestOnSegment is the point of the segment ab whose projection is
// nearest the mouse.
func (op *ObjectPicker) nearestOnSegment(a, b geom.Vector) geom.Vector {
	ap, bp := op.Cam.ProjectPoint(a), op.Cam.ProjectPoint(b)
	d := bp.Minus(ap)
	m := d.MagSquared()
	if m == 0 {
		return a
	}
	t := min(max(op.Point.Minus(ap).DivProjected(d), 0), 1)
	return a.Plus(b.Minus(a).ScaledBy(t))
}

// DoQuad records a quad: distance 0 inside it, else to its nearest
// edge.
func (op *ObjectPicker) DoQuad(a, b, c, d geom.Vector, zIndex int, comparePosition uint32) {
	corners := [4]geom.Point2d{
		op.Cam.ProjectPoint(a),
		op.Cam.ProjectPoint(b),
		op.Cam.ProjectPoint(c),
		op.Cam.ProjectPoint(d),
	}
	minNegative, maxPositive := veryNegative, veryPositive
	for i := range 4 {
		ap, bp := corners[i], corners[(i+1)%4]
		distance := op.Point.DistanceToLineSigned(ap, bp.Minus(ap), true)
		if distance < 0 {
			minNegative = geom.Max(minNegative, distance)
		}
		if distance > 0 {
			maxPositive = geom.Min(maxPositive, distance)
		}
	}

	insideQuad := minNegative == veryNegative || maxPositive == veryPositive
	if insideQuad {
		op.DoCompare(0, 0.0, zIndex, comparePosition)
	} else {
		distance := geom.Min(math.Abs(minNegative), math.Abs(maxPositive))
		op.DoCompare(0, distance, zIndex, comparePosition)
	}
}

const (
	veryPositive = 1e10
	veryNegative = -1e10
)

// DrawLine records a segment.
func (op *ObjectPicker) DrawLine(a, b geom.Vector, hcs HStroke) {
	stroke := op.Stroke(hcs)
	ap, bp, ok := op.inView(op.Cam.ProjectPoint(a), op.Cam.ProjectPoint(b))
	if !ok {
		return
	}
	distance := op.Point.DistanceToLine(ap, bp.Minus(ap), true)
	depth := 0.5 * (op.Cam.ProjectPoint3(a).Z + op.Cam.ProjectPoint3(b).Z)
	op.compareAt(depth, distance-stroke.Width/2.0, stroke.ZIndex, 0, op.nearestOnSegment(a, b), true)
}

// inView is the part of the segment ab inside the view, when
// ClipToView is set, and whether any of it is.
func (op *ObjectPicker) inView(a, b geom.Point2d) (geom.Point2d, geom.Point2d, bool) {
	if !op.ClipToView {
		return a, b, true
	}
	w, h := op.Cam.Width/2, op.Cam.Height/2
	d := b.Minus(a)
	t0, t1 := 0.0, 1.0
	// Each side of the view in turn, as p*t <= q (Liang and Barsky).
	for _, pq := range [4][2]float64{
		{-d.U, a.U + w}, {d.U, w - a.U},
		{-d.V, a.V + h}, {d.V, h - a.V},
	} {
		p, q := pq[0], pq[1]
		if p == 0 {
			if q < 0 {
				return a, b, false
			}
			continue
		}
		t := q / p
		if p < 0 {
			t0 = max(t0, t)
		} else {
			t1 = min(t1, t)
		}
		if t0 > t1 {
			return a, b, false
		}
	}
	// An end inside stays as it was, to the bit.
	ca, cb := a, b
	if t0 > 0 {
		ca = a.Plus(d.ScaledBy(t0))
	}
	if t1 < 1 {
		cb = a.Plus(d.ScaledBy(t1))
	}
	return ca, cb, true
}

// DrawEdges records each edge, with its auxB as the position.
func (op *ObjectPicker) DrawEdges(el *srf.SEdgeList, hcs HStroke) {
	stroke := op.Stroke(hcs)
	for _, e := range el.Edges {
		ap, bp, ok := op.inView(op.Cam.ProjectPoint(e.A), op.Cam.ProjectPoint(e.B))
		if !ok {
			continue
		}
		distance := op.Point.DistanceToLine(ap, bp.Minus(ap), true)
		depth := 0.5 * (op.Cam.ProjectPoint3(e.A).Z + op.Cam.ProjectPoint3(e.B).Z)
		op.compareAt(depth, distance-stroke.Width/2.0, stroke.ZIndex, uint32(e.AuxB), op.nearestOnSegment(e.A, e.B), true)
	}
}

// DrawBeziers can't pick curves; the caller draws them as edges.
func (op *ObjectPicker) DrawBeziers(*srf.SBezierList, HStroke) bool { return false }

// DrawOutlines is not implemented.
func (op *ObjectPicker) DrawOutlines(*srf.SOutlineList, HStroke, DrawOutlinesAs) {
	panic("render: ObjectPicker.DrawOutlines not implemented")
}

// DrawVectorText records the text's bounding quad.
func (op *ObjectPicker) DrawVectorText(text string, height float64, o, u, v geom.Vector, hcs HStroke) {
	stroke := op.Stroke(hcs)
	vf := res.BuiltinVectorFont()
	w := vf.Width(height, text)
	h := vf.Height(height)
	op.DoQuad(o,
		o.Plus(v.ScaledBy(h)),
		o.Plus(u.ScaledBy(w)).Plus(v.ScaledBy(h)),
		o.Plus(u.ScaledBy(w)),
		stroke.ZIndex, 0)
}

// DrawQuad records a quad.
func (op *ObjectPicker) DrawQuad(a, b, c, d geom.Vector, hcf HFill) {
	op.DoQuad(a, b, c, d, op.Fill(hcf).ZIndex, 0)
}

// DrawPoint records a point.
func (op *ObjectPicker) DrawPoint(o geom.Vector, hcs HStroke) {
	stroke := op.Stroke(hcs)
	po := op.Cam.ProjectPoint(o)
	if _, _, ok := op.inView(po, po); !ok {
		return
	}
	distance := op.Point.DistanceTo(po) - stroke.Width/2
	depth := op.Cam.ProjectPoint3(o).Z
	op.compareAt(depth, distance, stroke.ZIndex, 0, o, true)
}

// DrawPolygon is not implemented.
func (op *ObjectPicker) DrawPolygon(*srf.SPolygon, HFill) {
	panic("render: ObjectPicker.DrawPolygon not implemented")
}

// DrawMesh is not implemented.
func (op *ObjectPicker) DrawMesh(*srf.SMesh, HFill, HFill) {
	panic("render: ObjectPicker.DrawMesh not implemented")
}

// DrawFaces is not implemented.
func (op *ObjectPicker) DrawFaces(*srf.SMesh, []uint32, HFill) {
	panic("render: ObjectPicker.DrawFaces not implemented")
}

// DrawPixmap records the pixmap's quad.
func (op *ObjectPicker) DrawPixmap(_ *res.Pixmap, o, u, v geom.Vector, _, _ geom.Point2d, hcf HFill) {
	op.DrawQuad(o, o.Plus(u), o.Plus(u).Plus(v), o.Plus(v), hcf)
}

// InvalidatePixmap does nothing.
func (op *ObjectPicker) InvalidatePixmap(*res.Pixmap) {}

// Pick runs drawFn against the picker and reports whether anything was
// within SelRadius.
func (op *ObjectPicker) Pick(drawFn func()) bool {
	op.MinDepth = veryPositive
	op.MinDistance = veryPositive
	op.MaxZIndex = math.MinInt32
	op.AtKnown = false
	drawFn()
	return op.MinDistance < op.SelRadius
}

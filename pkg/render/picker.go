package render

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ObjectPicker is a canvas that draws nothing but finds the geometry
// nearest Point, within SelRadius pixels; higher z-indexes win (C++
// ObjectPicker).
type ObjectPicker struct {
	CanvasBase
	Camera Camera
	// Configuration.
	Point     geom.Point2d
	SelRadius float64
	// Picking state.
	MinDistance float64
	MinDepth    float64
	MaxZIndex   int
	Position    uint32
}

var _ Canvas = (*ObjectPicker)(nil)

// GetCamera returns the picker's camera.
func (op *ObjectPicker) GetCamera() *Camera { return &op.Camera }

// DoCompare records a candidate at the given distance.
func (op *ObjectPicker) DoCompare(depth, distance float64, zIndex int, comparePosition uint32) {
	if distance > op.SelRadius {
		return
	}
	if (zIndex == op.MaxZIndex && distance < op.MinDistance) || zIndex > op.MaxZIndex {
		op.MinDepth = depth
		op.MinDistance = distance
		op.MaxZIndex = zIndex
		op.Position = comparePosition
	}
}

// DoQuad records a quad: distance 0 inside it, else to its nearest
// edge.
func (op *ObjectPicker) DoQuad(a, b, c, d geom.Vector, zIndex int, comparePosition uint32) {
	corners := [4]geom.Point2d{
		op.Camera.ProjectPoint(a),
		op.Camera.ProjectPoint(b),
		op.Camera.ProjectPoint(c),
		op.Camera.ProjectPoint(d),
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
	ap := op.Camera.ProjectPoint(a)
	bp := op.Camera.ProjectPoint(b)
	distance := op.Point.DistanceToLine(ap, bp.Minus(ap), true)
	depth := 0.5 * (op.Camera.ProjectPoint3(a).Z + op.Camera.ProjectPoint3(b).Z)
	op.DoCompare(depth, distance-stroke.Width/2.0, stroke.ZIndex, 0)
}

// DrawEdges records each edge, with its auxB as the position.
func (op *ObjectPicker) DrawEdges(el *srf.SEdgeList, hcs HStroke) {
	stroke := op.Stroke(hcs)
	for _, e := range el.Edges {
		ap := op.Camera.ProjectPoint(e.A)
		bp := op.Camera.ProjectPoint(e.B)
		distance := op.Point.DistanceToLine(ap, bp.Minus(ap), true)
		depth := 0.5 * (op.Camera.ProjectPoint3(e.A).Z + op.Camera.ProjectPoint3(e.B).Z)
		op.DoCompare(depth, distance-stroke.Width/2.0, stroke.ZIndex, uint32(e.AuxB))
	}
}

// DrawBeziers can't pick curves; the caller draws them as edges.
func (op *ObjectPicker) DrawBeziers(*srf.SBezierList, HStroke) bool { return false }

// DrawOutlines is not implemented, as in C++.
func (op *ObjectPicker) DrawOutlines(*srf.SOutlineList, HStroke, DrawOutlinesAs) {
	panic("render: ObjectPicker.DrawOutlines not implemented")
}

// DrawVectorText records the text's bounding quad.
func (op *ObjectPicker) DrawVectorText(text string, height float64, o, u, v geom.Vector, hcs HStroke) {
	stroke := op.Stroke(hcs)
	vf := res.BuiltinVectorFont()
	w := vf.GetWidth(height, text)
	h := vf.GetHeight(height)
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
	distance := op.Point.DistanceTo(op.Camera.ProjectPoint(o)) - stroke.Width/2
	depth := op.Camera.ProjectPoint3(o).Z
	op.DoCompare(depth, distance, stroke.ZIndex, 0)
}

// DrawPolygon is not implemented, as in C++.
func (op *ObjectPicker) DrawPolygon(*srf.SPolygon, HFill) {
	panic("render: ObjectPicker.DrawPolygon not implemented")
}

// DrawMesh is not implemented, as in C++.
func (op *ObjectPicker) DrawMesh(*srf.SMesh, HFill, HFill) {
	panic("render: ObjectPicker.DrawMesh not implemented")
}

// DrawFaces is not implemented, as in C++.
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
	drawFn()
	return op.MinDistance < op.SelRadius
}

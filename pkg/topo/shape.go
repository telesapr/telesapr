package topo

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The shapes of a solid's faces and edges, as a description of the
// solid for a reader that can't see it names them: a face flat, a
// cylinder, a cone or a sphere, an edge straight, an arc or a circle.
// Each is found from the surfaces and curve points the topology has,
// and anything else is curved; nothing here is exact enough to build
// on, only to tell.

// ShapeKind is what shape a face or an edge has.
type ShapeKind int

// The kinds of shape. A face is ShapeCurved, ShapeFlat, ShapeCylinder,
// ShapeCone or ShapeSphere; an edge ShapeCurved, ShapeStraight,
// ShapeArc or ShapeCircle.
const (
	ShapeCurved ShapeKind = iota
	ShapeFlat
	ShapeCylinder
	ShapeCone
	ShapeSphere
	ShapeStraight
	ShapeArc
	ShapeCircle
)

// Shape is the shape of a face or an edge, with what places it.
type Shape struct {
	Kind ShapeKind
	// Normal is a flat face's unit normal out of the solid, or the unit
	// normal of the plane an arc or a circle lies in.
	Normal geom.Vector
	// Centre is a point of a cylinder's axis, a cone's apex, a
	// sphere's centre, or an arc's or a circle's centre.
	Centre geom.Vector
	// Axis is a cylinder's or a cone's unit axis.
	Axis geom.Vector
	// Radius is a cylinder's, a sphere's, an arc's or a circle's.
	Radius float64
	// HalfAngle is a cone's half angle, in radians.
	HalfAngle float64
}

// SheetShape is the shape of sheet sh: the shape all its patches share,
// or ShapeCurved where they share none of those SheetShape knows.
func (t *Topology) SheetShape(sh int) Shape {
	fs := t.Sheets[sh].Faces
	if len(fs) == 0 {
		return Shape{}
	}
	if n, ok := t.sheetPlane(fs); ok {
		return Shape{Kind: ShapeFlat, Normal: n}
	}
	if s, ok := t.sheetRound(fs); ok {
		return s
	}
	if s, ok := t.sheetSphere(fs); ok {
		return s
	}
	return Shape{}
}

// sheetPlane is the outward normal of the plane patches fs all lie in.
func (t *Topology) sheetPlane(fs []int) (geom.Vector, bool) {
	n, ok := t.planarNormal(fs[0])
	if !ok {
		return geom.Vector{}, false
	}
	o := t.Surface(fs[0]).Ctrl[0][0]
	for _, f := range fs[1:] {
		m, ok := t.planarNormal(f)
		if !ok || m.Dot(n) < 1-1e-9 || math.Abs(t.Surface(f).Ctrl[0][0].Minus(o).Dot(n)) > geom.LengthEps {
			return geom.Vector{}, false
		}
	}
	return n, true
}

// sheetRound is the cylinder or the cone patches fs all lie on.
func (t *Topology) sheetRound(fs []int) (Shape, bool) {
	var first srf.RoundFace
	for i, f := range fs {
		s := t.Surface(f)
		if s == nil {
			return Shape{}, false
		}
		rf, ok := srf.RoundFaceOf(s)
		if !ok {
			return Shape{}, false
		}
		if i == 0 {
			first = rf
			continue
		}
		tol := geom.LengthEps * max(1, first.Radius)
		if rf.Axis.Cross(first.Axis).Magnitude() > 1e-6 ||
			rf.Point.Minus(first.Point).Cross(first.Axis).Magnitude() > tol ||
			math.Abs(rf.Radius-first.Radius) > tol || math.Abs(rf.HalfAngle-first.HalfAngle) > 1e-6 {
			return Shape{}, false
		}
	}
	if first.IsCone() {
		return Shape{Kind: ShapeCone, Centre: first.Point, Axis: first.Axis, HalfAngle: first.HalfAngle}, true
	}
	return Shape{Kind: ShapeCylinder, Centre: first.Point, Axis: first.Axis, Radius: first.Radius}, true
}

// sheetSphere is the sphere patches fs all lie on: the normals at two
// points of the first meet at its centre, and a grid of points over
// every patch is as far from it.
func (t *Topology) sheetSphere(fs []int) (Shape, bool) {
	s := t.Surface(fs[0])
	if s == nil {
		return Shape{}, false
	}
	p1, n1 := s.PointAt(0.3, 0.4), s.NormalAt(0.3, 0.4)
	p2, n2 := s.PointAt(0.7, 0.6), s.NormalAt(0.7, 0.6)
	c, ok := linesMeet(p1, n1, p2, n2)
	if !ok {
		return Shape{}, false
	}
	r := p1.Minus(c).Magnitude()
	if r < geom.LengthEps {
		return Shape{}, false
	}
	tol := geom.LengthEps * max(1, r)
	const steps = 4
	for _, f := range fs {
		s := t.Surface(f)
		if s == nil {
			return Shape{}, false
		}
		for i := range steps + 1 {
			for j := range steps + 1 {
				p := s.PointAt(float64(i)/steps, float64(j)/steps)
				if math.Abs(p.Minus(c).Magnitude()-r) > tol {
					return Shape{}, false
				}
			}
		}
	}
	return Shape{Kind: ShapeSphere, Centre: c, Radius: r}, true
}

// linesMeet is where the lines through a along u and through b along v
// meet, within a hundredth of a micron; false where they are parallel
// or pass each other.
func linesMeet(a, u, b, v geom.Vector) (geom.Vector, bool) {
	w := u.Cross(v)
	ww := w.Dot(w)
	if ww < 1e-18*u.Dot(u)*v.Dot(v) {
		return geom.Vector{}, false
	}
	d := b.Minus(a)
	s := d.Cross(v).Dot(w) / ww
	r := d.Cross(u).Dot(w) / ww
	p, q := a.Plus(u.ScaledBy(s)), b.Plus(v.ScaledBy(r))
	if p.Minus(q).Magnitude() > geom.LengthEps {
		return geom.Vector{}, false
	}
	return p.Plus(q).ScaledBy(0.5), true
}

// errNoPoint is SheetPoint's error for a sheet none of whose patches has
// a point inside its loops.
var errNoPoint = errors.New("the face has no point inside its loops")

// SheetPoint is a point of sheet sh away from its edges: the point
// FacePointFrame takes on its first patch that has one, which FaceAt
// finds the sheet by.
func (t *Topology) SheetPoint(sh int) (geom.Vector, error) {
	for _, f := range t.Sheets[sh].Faces {
		if s, uv, err := t.patchPoint(f, nil); err == nil {
			return s.PointAt(uv.X, uv.Y), nil
		}
	}
	return geom.Vector{}, errNoPoint
}

// EdgeShape is the shape of the edge of the solid that edge e is a piece
// of, its whole chain of pieces: straight, an arc, a circle (an arc that
// closes), or curved.
func (t *Topology) EdgeShape(e int) Shape {
	chain := t.Chain(e)
	if len(chain) == 1 && t.Straight(e) {
		return Shape{Kind: ShapeStraight}
	}
	c, ok := polylineCircle(t.ChainPoints(chain))
	if !ok {
		return Shape{}
	}
	sh := Shape{Kind: ShapeArc, Centre: c.centre, Normal: c.n, Radius: c.r}
	if t.Closed(chain) {
		sh.Kind = ShapeCircle
	}
	return sh
}

package sketch

import (
	"errors"
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/libm"
)

// This file is not a port: C++ SolveSpace shows measurements only in
// the text window's description of the selection. Measure works them
// out, with the same formulas, for the canvas and for scripts.

// MeasureKind is what a measurement is.
type MeasureKind int

const (
	// MeasureLength is a line segment's length, in mm.
	MeasureLength MeasureKind = iota
	// MeasureRadius is a circle's or an arc's radius, in mm.
	MeasureRadius
	// MeasureDistance is a distance, in mm: between two points, from a
	// point to a circle, a line, a plane face, or along a vector.
	MeasureDistance
	// MeasureAngle is the angle between two lines, normals or plane
	// faces, in degrees from 0 to 180.
	MeasureAngle
)

// Measurement is what Measure finds.
type Measurement struct {
	Kind MeasureKind
	// Value is in mm, or degrees for an angle.
	Value float64
	// What says what was measured: "length", "radius", "distance",
	// "point to circle", "point to line", "point to plane",
	// "projected distance", "angle".
	What string
	// At is a point beside what was measured, for its label.
	At geom.Vector
}

// errMeasure is Measure's error for a selection it can't measure.
var errMeasure = errors.New("can't measure that; select a line segment, a circle or an arc, " +
	"two points, a point and a line, circle or plane face, two lines or plane faces, " +
	"or two points and a line")

// Measure measures entities, as the description of the selection
// does: one line segment or circle; two points; a point and a circle,
// line segment or plane face; two lines, normals or plane faces; or two
// points and a line along which to project their distance.
func (sk *Sketch) Measure(entities ...core.HEntity) (Measurement, error) {
	var es []*core.Entity
	for _, h := range entities {
		e := sk.GetEntity(h)
		if e == nil {
			return Measurement{}, errors.New("no such entity")
		}
		es = append(es, e)
	}
	var points, lines, circles, faces, vectors []*core.Entity
	for _, e := range es {
		switch {
		case e.IsPoint():
			points = append(points, e)
		case e.IsFace():
			faces = append(faces, e)
		case e.IsCircle():
			circles = append(circles, e)
		}
		if e.Type == core.EntityLineSegment {
			lines = append(lines, e)
		}
		if e.HasVector() {
			vectors = append(vectors, e)
		}
	}
	pt := func(h core.HEntity) geom.Vector { return sk.MustGetEntity(h).PointGetNum(sk) }
	n := len(es)
	switch {
	case n == 1 && len(lines) == 1:
		a, b := pt(lines[0].Point[0]), pt(lines[0].Point[1])
		return Measurement{MeasureLength, b.Minus(a).Magnitude(), "length", a.Plus(b).ScaledBy(0.5)}, nil

	case n == 1 && len(circles) == 1:
		c := circles[0]
		center := pt(c.Point[0])
		return Measurement{MeasureRadius, c.CircleGetRadiusNum(sk), "radius", center}, nil

	case n == 2 && len(points) == 2:
		a, b := points[0].PointGetNum(sk), points[1].PointGetNum(sk)
		return Measurement{MeasureDistance, b.Minus(a).Magnitude(), "distance", a.Plus(b).ScaledBy(0.5)}, nil

	case n == 2 && len(points) == 1 && len(circles) == 1:
		p := points[0].PointGetNum(sk)
		c := pt(circles[0].Point[0])
		d := p.Minus(c).Magnitude() - circles[0].CircleGetRadiusNum(sk)
		return Measurement{MeasureDistance, d, "point to circle", p}, nil

	case n == 2 && len(points) == 1 && len(faces) == 1:
		p := points[0].PointGetNum(sk)
		nn := faces[0].FaceGetNormalNum(sk)
		pl := faces[0].FaceGetPointNum(sk)
		return Measurement{MeasureDistance, nn.Dot(pl) - nn.Dot(p), "point to plane", p}, nil

	case n == 2 && len(points) == 1 && len(lines) == 1:
		p := points[0].PointGetNum(sk)
		a, b := pt(lines[0].Point[0]), pt(lines[0].Point[1])
		return Measurement{MeasureDistance, p.DistanceToLine(a, b.Minus(a)), "point to line", p}, nil

	case n == 2 && len(faces) == 2:
		n0, n1 := faces[0].FaceGetNormalNum(sk), faces[1].FaceGetNormalNum(sk)
		at := faces[0].FaceGetPointNum(sk)
		return Measurement{MeasureAngle, libm.Acos(n0.Dot(n1)) * 180 / math.Pi, "angle", at}, nil

	case n == 2 && len(vectors) == 2:
		v0 := vectors[0].VectorGetNum(sk).WithMagnitude(1)
		v1 := vectors[1].VectorGetNum(sk).WithMagnitude(1)
		var at geom.Vector
		if l := lines; len(l) > 0 {
			at = pt(l[0].Point[0]).Plus(pt(l[0].Point[1])).ScaledBy(0.5)
		}
		return Measurement{MeasureAngle, libm.Acos(v0.Dot(v1)) * 180 / math.Pi, "angle", at}, nil

	case n == 3 && len(points) == 2 && len(vectors) == 1:
		a, b := points[0].PointGetNum(sk), points[1].PointGetNum(sk)
		v := vectors[0].VectorGetNum(sk).WithMagnitude(1)
		return Measurement{MeasureDistance, b.Minus(a).Dot(v), "projected distance", a.Plus(b).ScaledBy(0.5)}, nil
	}
	return Measurement{}, errMeasure
}

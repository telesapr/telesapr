package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Workplane groups made from geometry. The window and scripts add the
// groups these build, so a workplane that follows two lines or a normal
// is the same group whichever way it was asked for.

// NewSketchOnLines starts a sketch in the workplane through the point
// origin, parallel to the line segments lineA and lineB, which follows
// them when they move. right and up are the directions the sketch is
// viewed along, which choose which line is u and which v, and their
// signs: u is the line most along right, v the other, each turned to
// point along its own view direction. A script that has no view gives
// the lines' own directions, which keeps lineA as u and lineB as v.
func (sk *Sketch) NewSketchOnLines(origin, lineA, lineB core.HEntity, right, up geom.Vector, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupDrawingWorkplane
	g.Name = name
	if name == "" {
		g.Name = "sketch-in-plane"
	}
	if e := sk.Entity(origin); e == nil || !e.IsPoint() {
		return g, errors.New("the origin of the workplane must be a point")
	}
	var dirs [2]geom.Vector
	for i, h := range [2]core.HEntity{lineA, lineB} {
		e := sk.Entity(h)
		if e == nil || e.Type != core.EntityLineSegment {
			return g, errors.New("a workplane on lines takes two line segments")
		}
		dirs[i] = e.VectorGetNum(sk)
	}
	if lineA == lineB || dirs[0].Cross(dirs[1]).Magnitude() < 1e-9*dirs[0].Magnitude()*dirs[1].Magnitude() {
		return g, errors.New("the two lines of a workplane must not be parallel")
	}
	g.Subtype = GroupWorkplaneByLineSegments
	g.Predef.Origin = origin
	g.Predef.EntityB = lineA
	g.Predef.EntityC = lineB

	ut := dirs[0].WithMagnitude(1)
	vt := dirs[1].WithMagnitude(1)
	if math.Abs(up.Dot(vt)) < math.Abs(up.Dot(ut)) {
		ut, vt = vt, ut
		g.Predef.SwapUV = true
	}
	if right.Dot(ut) < 0 {
		g.Predef.NegateU = true
	}
	if up.Dot(vt) < 0 {
		g.Predef.NegateV = true
	}
	return g, nil
}

// NewSketchOnNormal starts a sketch in the workplane through the point
// origin, square to a normal, which it follows. The normal is a normal
// entity or a workplane, whose normal is taken.
func (sk *Sketch) NewSketchOnNormal(origin, normal core.HEntity, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupDrawingWorkplane
	g.Name = name
	if name == "" {
		g.Name = "sketch-in-plane"
	}
	if e := sk.Entity(origin); e == nil || !e.IsPoint() {
		return g, errors.New("the origin of the workplane must be a point")
	}
	n := sk.Entity(normal)
	switch {
	case n == nil:
		return g, errors.New("the normal of the workplane is not in the sketch")
	case n.IsWorkplane():
		normal = n.NormalEntity(sk).H
		n = sk.Entity(normal)
	case !n.IsNormal():
		return g, errors.New("a workplane on a normal takes a normal or a workplane")
	}
	g.Subtype = GroupWorkplaneByPointNormal
	g.Predef.EntityB = normal
	g.Predef.Q = n.NormalGetNum(sk)
	g.Predef.Origin = origin
	return g, nil
}

// TurnAxis is the point and the direction a lathe, a revolve or a helix
// turns about: the point and a vector (a line segment or a normal), the
// axis through the point parallel to it; or, with no point (a zero
// handle), a line segment, the axis along it from its first end.
func (sk *Sketch) TurnAxis(point, vector core.HEntity) (origin, dir core.HEntity, err error) {
	v := sk.Entity(vector)
	if v == nil || !v.HasVector() {
		return origin, dir, errors.New("the axis needs a line segment or a normal for its direction")
	}
	if point.V == 0 {
		if v.Type != core.EntityLineSegment {
			return origin, dir, errors.New("an axis with no point is a line segment")
		}
		return v.Point[0], vector, nil
	}
	if p := sk.Entity(point); p == nil || !p.IsPoint() {
		return origin, dir, fmt.Errorf("the axis goes through a point, and %s isn't one", sk.entityName(point))
	}
	return point, vector, nil
}

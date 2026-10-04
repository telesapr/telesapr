package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// surfaceKind names the shape of face f's surface for a message: "a
// cylinder", "a cone", or "a curved face". A patch straight one way is a
// cylinder when its lines are parallel and the same distance from their
// middle one, a cone when they meet at one point.
func (t *Topology) surfaceKind(f int) string {
	s := t.Surface(f)
	if s == nil {
		return "a curved face"
	}
	var axis, centre geom.Vector
	var r float64
	var a, b geom.Vector
	if s.IsCylinder(&axis, &centre, &r, &a, &b) {
		return "a cylinder"
	}
	// The lines of the patch, at three places across it.
	line := func(w float64) (p, d geom.Vector) {
		if s.DegN == 1 {
			p = s.PointAt(w, 0)
			return p, s.PointAt(w, 1).Minus(p)
		}
		p = s.PointAt(0, w)
		return p, s.PointAt(1, w).Minus(p)
	}
	if s.DegM != 1 && s.DegN != 1 {
		return "a curved face"
	}
	var ps, ds [3]geom.Vector
	for i, w := range []float64{0, 0.5, 1} {
		ps[i], ds[i] = line(w)
		if ds[i].Magnitude() < geom.LengthEps {
			return "a curved face"
		}
	}
	// Where line 0 comes closest to lines 1 and 2: one point for a cone.
	closest := func(i int) (geom.Vector, bool) {
		n := ds[0].Cross(ds[i])
		if n.Magnitude() < 1e-9*ds[0].Magnitude()*ds[i].Magnitude() {
			return geom.Vector{}, false
		}
		// The point of line 0 on the plane through line i holding n.
		m := ds[i].Cross(n)
		den := ds[0].Dot(m)
		if math.Abs(den) < 1e-18 {
			return geom.Vector{}, false
		}
		return ps[0].Plus(ds[0].ScaledBy(ps[i].Minus(ps[0]).Dot(m) / den)), true
	}
	p1, ok1 := closest(1)
	p2, ok2 := closest(2)
	size := ds[0].Magnitude()
	if ok1 && ok2 && p1.Minus(p2).Magnitude() < 1e-4*size {
		return "a cone"
	}
	return "a curved face"
}

// offAxisErr is the refusal of a curved edge between a flat face and
// face f that is not a circle about f's axis square to the flat face.
func (t *Topology) offAxisErr(m geom.Vector, f int, what string) error {
	kind := t.surfaceKind(f)
	if kind == "a curved face" {
		return filletErr(m, "the edge is where the flat face meets a curved face that is neither a cylinder nor a cone: "+
			"%s rounds a flat face's edge only with a cylinder or a cone about an axis square to the face", what)
	}
	return filletErr(m, "the edge is where the flat face meets %s off its axis: %s rounds a flat face's edge with "+
		"a cylinder or a cone only where the face is square to the axis, as a boss's top or a hole's rim", kind, what)
}

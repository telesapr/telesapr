package api

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The joints of their own (F9): ball, cylindrical, planar and slot, as
// Kramer's marker constraints made of the constraints there are. Each
// is made in 3d, of what the user picks, the first of the part that
// stays and the second of the part the active group places, and each
// leaves the part a known freedom, which LinkGroup.Freedom counts:
//
//	Ball         coincident              turns 3, slides 0, about a point
//	Cylindrical  in-line + parallel-z    turns 1, slides 1, on an axis
//	Planar       in-plane + parallel-z   turns 1, slides 2, in a plane
//	Slot         in-line (point on line) turns 3, slides 1, along a line

// Ball puts a point of the second part on a point of the first: the
// part may turn every way about it, and not slide, as on a ball joint.
// Each of a and b is a point, or a circle or an arc for its centre, as
// of a ball's equator. It is one coincident constraint, made in 3d.
func (m AssemblyMenu) Ball(a, b Entity) []Constraint {
	d := m.d
	var pa, pb Entity
	d.read(func(sk *sketch.Sketch) {
		mustBeFree(sk, "a ball mate")
		for i, e := range []Entity{a, b} {
			p, ok := jointPoint(sk, e)
			if !ok {
				fail("a ball mate is of two points, or circles or arcs by their centres, and %s isn't one", e.describe(sk))
			}
			if i == 0 {
				pa = p
			} else {
				pb = p
			}
		}
	})
	return []Constraint{d.Constrain.Coincident(pa, pb)}
}

// Cylindrical puts the second part's axis on the first's: the part may
// turn about the axis and slide along it, as a rod in a bore. Each of
// a and b is a line segment, a circle or an arc (its axis, through its
// centre along its normal), or a datum axis, as Concentric takes. The
// axes are made parallel, and a point of one axis put on the other: of
// two circles or axes it is what Concentric makes in 3d; where one is a
// line segment, the other's point (its centre, or its start) is put on
// that line. It is made in 3d.
func (m AssemblyMenu) Cylindrical(a, b Entity) []Constraint {
	d := m.d
	var lineA, lineB bool
	var pa, pb, va, vb Entity
	d.read(func(sk *sketch.Sketch) {
		mustBeFree(sk, "a cylindrical mate")
		for _, e := range []Entity{a, b} {
			if e.get(sk).Type == core.EntityLineSegment {
				continue
			}
			if _, _, ok := mateAxis(sk, e); !ok {
				fail("a cylindrical mate is of two line segments, circles, arcs or axes, and %s isn't one", e.describe(sk))
			}
			if k := sk.DatumKindOf(e.H); !k.IsAxis() {
				fail("%s is the datum of a flat face, a plane with no axis; mate it planar", e.describe(sk))
			}
		}
		lineA = a.get(sk).Type == core.EntityLineSegment
		lineB = b.get(sk).Type == core.EntityLineSegment
		axis := func(e Entity, line bool) (p, v Entity) {
			if line {
				return Entity{d, e.get(sk).Point[0]}, e
			}
			p, v, _ = mateAxis(sk, e)
			return p, v
		}
		pa, va = axis(a, lineA)
		pb, vb = axis(b, lineB)
	})
	if !lineA && !lineB {
		return m.Concentric(a, b)
	}
	// The axes parallel first, as for Concentric and Flush, then a point
	// of one on the other's line.
	out := d.addUnlessRedundant(nil, func() Constraint { return d.Constrain.Parallel(va, vb) })
	if len(out) == 0 {
		// Left out: either the axes are parallel already, or the solve
		// couldn't turn the part to make them so.
		d.read(func(sk *sketch.Sketch) {
			u := va.get(sk).VectorGetNum(sk).WithMagnitude(1)
			v := vb.get(sk).VectorGetNum(sk).WithMagnitude(1)
			if u.Cross(v).Magnitude() > 1e-6 {
				fail("the axes of %s and %s can't be made parallel from where they are; turn the part nearer first",
					a.describe(sk), b.describe(sk))
			}
		})
	}
	if lineA {
		return append(out, d.Constrain.On(pb, a))
	}
	return append(out, d.Constrain.On(pa, b))
}

// Planar lays a face of the second part flat on a face of the first:
// the part may slide two ways on it and turn about its normal, as a
// block on a table. It is Flush under the joint's name, of what Flush
// takes: two faces, workplanes or datums on flat faces.
func (m AssemblyMenu) Planar(faceA, faceB Entity) []Constraint {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, e := range []Entity{faceA, faceB} {
			if ent := e.get(sk); !ent.IsFace() && !ent.IsWorkplane() && !isNormalAt(ent) {
				fail("a planar mate is of two faces or workplanes, and %s isn't one", e.describe(sk))
			}
		}
	})
	return m.Flush(faceA, faceB)
}

// Slot puts a point of one part on a line of the other: the point
// slides along the line, and the part turns every way about it, as a
// pin's ball end in a groove. One of a and b is a line segment, the
// other a point, or a circle or an arc for its centre; either may come
// first. It is one point-on-line constraint, made in 3d.
func (m AssemblyMenu) Slot(a, b Entity) []Constraint {
	d := m.d
	var point, line Entity
	d.read(func(sk *sketch.Sketch) {
		mustBeFree(sk, "a slot mate")
		isLineSeg := func(e Entity) bool { return e.get(sk).Type == core.EntityLineSegment }
		switch {
		case isLineSeg(a) && !isLineSeg(b):
			line = a
			point, _ = jointPoint(sk, b)
		case isLineSeg(b) && !isLineSeg(a):
			line = b
			point, _ = jointPoint(sk, a)
		}
		if line.H.V == 0 || point.H.V == 0 {
			fail("a slot mate is of a line segment and a point, or a circle or arc by its centre, and %s and %s aren't",
				a.describe(sk), b.describe(sk))
		}
	})
	return []Constraint{d.Constrain.On(point, line)}
}

// mustBeFree fails unless the sketch draws in 3d: a joint's
// constraints, made in a workplane, would hold only what is seen in it.
func mustBeFree(sk *sketch.Sketch, what string) {
	if !core.IsFreeIn3D(sk.ActiveWorkplane()) {
		fail("%s is made in 3d; leave the workplane (Sketch.Anywhere3D)", what)
	}
}

// jointPoint is e as a point a joint holds: a point, or a circle's or
// an arc's centre; false for anything else.
func jointPoint(sk *sketch.Sketch, e Entity) (Entity, bool) {
	ent := e.get(sk)
	switch {
	case ent.IsPoint():
		return e, true
	case ent.Type == core.EntityCircle || ent.Type == core.EntityArcOfCircle:
		return Entity{e.d, ent.Point[0]}, true
	}
	return Entity{}, false
}

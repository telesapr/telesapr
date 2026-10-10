package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// ConstrainMenu holds the calls of the Constrain menu: the dimensions
// and constraints that hold a sketch. A script reaches it as
// doc.Constrain.
type ConstrainMenu struct{ d *Doc }

// Raw adds a constraint of any type, as the core package names
// them, on up to two points and two other entities; see the solver's
// documentation for what each type takes. Give Entity{} for what a
// type doesn't take.
func (m ConstrainMenu) Raw(t core.ConstraintType, ptA, ptB, entityA, entityB Entity, other, other2 bool) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		for _, e := range []Entity{ptA, ptB, entityA, entityB} {
			if e.H.V != 0 {
				e.get(sk)
			}
		}
		c.Type = t
		c.PtA, c.PtB = ptA.H, ptB.H
		c.EntityA, c.EntityB = entityA.H, entityB.H
		c.Other, c.Other2 = other, other2
		if c.HasLabel() {
			c.ModifyToSatisfy(sk)
		}
	})
}

// Coincident puts two points at the same place.
func (m ConstrainMenu) Coincident(a, b Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintPointsCoincident
		c.PtA = a.must(sk, "a point", isPoint)
		c.PtB = b.must(sk, "a point", isPoint)
	})
}

// Distance sets the distance between two points, or from a point to a
// line segment, a workplane or a face; either may come first. A
// negative distance from a line, a workplane or a face is on the
// other side of it.
func (m ConstrainMenu) Distance(a, b Entity, v float64) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		distanceBetween(sk, c, a, b, v)
	})
}

// Length sets the length of a line segment.
func (m ConstrainMenu) Length(line Entity, v float64) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		l := sk.MustEntity(line.must(sk, "a line segment", isLine))
		c.Type = core.ConstraintPtPtDistance
		c.PtA, c.PtB = l.Point[0], l.Point[1]
		dimension(sk, c, v)
	})
}

// MinDistance keeps a distance at least v: between two points, from a
// point to a line segment, a workplane or a face, as Distance takes
// them, or a line segment's length when b is Entity{}. Unlike
// Distance it leaves the sketch free to go further: it holds only
// where the sketch would come nearer, and takes a degree of freedom
// only then. A distance from a line, a workplane or a face is signed,
// as Distance's is: the point keeps to v's side, at least |v| away.
func (m ConstrainMenu) MinDistance(a, b Entity, v float64) Constraint {
	d := m.d
	return d.boundedDistance(core.LimitMin, a, b, v)
}

// MaxDistance keeps a distance at most v, taking what MinDistance
// takes. It leaves the sketch free to come nearer, and takes a degree
// of freedom only where the sketch would go further.
func (m ConstrainMenu) MaxDistance(a, b Entity, v float64) Constraint {
	d := m.d
	return d.boundedDistance(core.LimitMax, a, b, v)
}

// OnSegment puts a point on a line segment, between its ends: unlike
// On, which puts it anywhere on the segment's line.
func (m ConstrainMenu) OnSegment(point, line Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintPtOnSegment
		c.PtA = point.must(sk, "a point", isPoint)
		c.EntityA = line.must(sk, "a line segment", isLine)
	})
}

// Diameter sets the diameter of a circle or an arc.
func (m ConstrainMenu) Diameter(circle Entity, v float64) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintDiameter
		c.EntityA = circle.must(sk, "a circle or an arc", isCircle)
		dimension(sk, c, v)
	})
}

// Radius sets the radius of a circle or an arc; the drawing shows the
// dimension as a radius.
func (m ConstrainMenu) Radius(circle Entity, v float64) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintDiameter
		c.EntityA = circle.must(sk, "a circle or an arc", isCircle)
		c.Other = true
		dimension(sk, c, v)
	})
}

// Horizontal makes a line segment, or two points, horizontal in the
// active workplane.
func (m ConstrainMenu) Horizontal(lineOrPoints ...Entity) Constraint {
	d := m.d
	return d.inWorkplane(core.ConstraintHorizontal, "Horizontal", lineOrPoints)
}

// Vertical makes a line segment, or two points, vertical in the
// active workplane.
func (m ConstrainMenu) Vertical(lineOrPoints ...Entity) Constraint {
	d := m.d
	return d.inWorkplane(core.ConstraintVertical, "Vertical", lineOrPoints)
}

// Equal makes two line segments as long as each other, two circles or
// arcs of the same radius, or a line segment as long as an arc.
func (m ConstrainMenu) Equal(a, b Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		ea, eb := a.get(sk), b.get(sk)
		switch {
		case isLine(ea) && isLine(eb):
			c.Type = core.ConstraintEqualLengthLines
			c.EntityA, c.EntityB = a.H, b.H
		case ea.IsCircle() && eb.IsCircle():
			c.Type = core.ConstraintEqualRadius
			c.EntityA, c.EntityB = a.H, b.H
		case isLine(ea) && eb.Type == core.EntityArcOfCircle:
			c.Type = core.ConstraintEqualLineArcLen
			c.EntityA, c.EntityB = a.H, b.H
		case ea.Type == core.EntityArcOfCircle && isLine(eb):
			c.Type = core.ConstraintEqualLineArcLen
			c.EntityA, c.EntityB = b.H, a.H
		default:
			fail("%s and %s can't be equal", a.describe(sk), b.describe(sk))
		}
	})
}

// Parallel makes two line segments or normals parallel.
func (m ConstrainMenu) Parallel(a, b Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintParallel
		c.EntityA = a.must(sk, "a line segment or a normal", hasVector)
		c.EntityB = b.must(sk, "a line segment or a normal", hasVector)
	})
}

// Ratio makes a's length v times b's, as Constrain > Length / Arc
// Ratio does: two line segments, two arcs (the lengths along them), or
// a line segment and an arc, taken in either order, when the ratio is
// the line segment's length over the arc's.
func (m ConstrainMenu) Ratio(a, b Entity, v float64) Constraint {
	return m.lengths(a, b, v, "ratio", core.ConstraintLengthRatio, core.ConstraintArcArcLenRatio, core.ConstraintArcLineLenRatio)
}

// Difference makes a's length v more than b's, as Constrain > Length /
// Arc Difference does, taking what Ratio takes: with a line segment and
// an arc, the line segment is v longer than the arc. A difference is a
// size, as a distance is, so v is taken without its sign: for a shorter
// than b, give them the other way round.
func (m ConstrainMenu) Difference(a, b Entity, v float64) Constraint {
	return m.lengths(a, b, v, "difference", core.ConstraintLengthDifference, core.ConstraintArcArcDifference, core.ConstraintArcLineDifference)
}

// lengths adds a ratio or a difference of lengths of type lines, arcs
// or lineArc, as the two entities are.
func (m ConstrainMenu) lengths(a, b Entity, v float64, what string, lines, arcs, lineArc core.ConstraintType) Constraint {
	return m.d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		ea, eb := a.get(sk), b.get(sk)
		isArc := func(e *core.Entity) bool { return e.Type == core.EntityArcOfCircle }
		c.EntityA, c.EntityB = a.H, b.H
		switch {
		case isLine(ea) && isLine(eb):
			c.Type = lines
		case isArc(ea) && isArc(eb):
			c.Type = arcs
		case isLine(ea) && isArc(eb):
			c.Type = lineArc
		case isArc(ea) && isLine(eb):
			c.Type = lineArc
			c.EntityA, c.EntityB = b.H, a.H
		default:
			fail("a length %s is of two line segments, two arcs, or a line segment and an arc, not %s and %s", what, a.describe(sk), b.describe(sk))
		}
		dimension(sk, c, v)
	})
}

// SameOrientation makes two normals point the same way and turn the
// same way about it, as Constrain > Same Orientation does: a linked
// part's to a workplane's, say.
func (m ConstrainMenu) SameOrientation(a, b Entity) Constraint {
	return m.d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintSameOrientation
		c.EntityA = a.must(sk, "a normal", (*core.Entity).IsNormal)
		c.EntityB = b.must(sk, "a normal", (*core.Entity).IsNormal)
		sk.AlignSameOrientation(c, sk.Active)
	})
}

// Perpendicular makes two line segments or normals perpendicular.
func (m ConstrainMenu) Perpendicular(a, b Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintPerpendicular
		c.EntityA = a.must(sk, "a line segment or a normal", hasVector)
		c.EntityB = b.must(sk, "a line segment or a normal", hasVector)
	})
}

// Angle sets the angle in degrees between two line segments or
// normals. For two lines that share an end it is the angle at that
// corner.
func (m ConstrainMenu) Angle(a, b Entity, degrees float64) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintAngle
		c.EntityA = a.must(sk, "a line segment or a normal", hasVector)
		c.EntityB = b.must(sk, "a line segment or a normal", hasVector)
		ea, eb := a.get(sk), b.get(sk)
		if isLine(ea) && isLine(eb) {
			a0 := sk.MustEntity(ea.Point[0]).PointGetNum(sk)
			a1 := sk.MustEntity(ea.Point[1]).PointGetNum(sk)
			b0 := sk.MustEntity(eb.Point[0]).PointGetNum(sk)
			b1 := sk.MustEntity(eb.Point[1]).PointGetNum(sk)
			if (!a0.Equals(b0) && !a1.Equals(b1)) && (a0.Equals(b1) || a1.Equals(b0)) {
				// The lines run in opposite senses from the corner.
				c.Other = true
			}
		}
		dimension(sk, c, degrees)
	})
}

// Tangent makes an arc or a cubic tangent to a line segment, or to
// another arc or cubic, where they share an end.
func (m ConstrainMenu) Tangent(a, b Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		ea, eb := a.get(sk), b.get(sk)
		if isLine(eb) {
			ea, eb = eb, ea
		}
		var err error
		switch {
		case isLine(ea) && eb.Type == core.EntityArcOfCircle:
			err = sk.ConstrainArcLineTangent(c, ea, eb, nil)
			c.Type = core.ConstraintArcLineTangent
			c.EntityA, c.EntityB = eb.H, ea.H
		case isLine(ea) && eb.Type == core.EntityCubic:
			err = sk.ConstrainCubicLineTangent(c, ea, eb, nil)
			c.Type = core.ConstraintCubicLineTangent
			c.EntityA, c.EntityB = eb.H, ea.H
		case (ea.Type == core.EntityArcOfCircle || ea.Type == core.EntityCubic) &&
			(eb.Type == core.EntityArcOfCircle || eb.Type == core.EntityCubic):
			if core.IsFreeIn3D(c.Workplane) {
				fail("curves are made tangent in a workplane, and none is active")
			}
			err = sk.ConstrainCurveCurveTangent(c, ea, eb, nil, nil)
			c.Type = core.ConstraintCurveCurveTangent
			c.EntityA, c.EntityB = ea.H, eb.H
		default:
			fail("%s and %s can't be tangent", a.describe(sk), b.describe(sk))
		}
		if err != nil {
			panic(err)
		}
	})
}

// On puts a point on a line segment, a circle or an arc, a workplane
// or a face.
func (m ConstrainMenu) On(point, on Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.PtA = point.must(sk, "a point", isPoint)
		c.EntityA = on.H
		switch e := on.get(sk); {
		case isLine(e):
			c.Type = core.ConstraintPtOnLine
		case e.IsCircle():
			c.Type = core.ConstraintPtOnCircle
		case e.IsWorkplane():
			c.Type = core.ConstraintPtInPlane
		case e.IsFace():
			c.Type = core.ConstraintPtOnFace
		default:
			fail("a point can't be on %s", on.describe(sk))
		}
	})
}

// Midpoint puts a point at the middle of a line segment.
func (m ConstrainMenu) Midpoint(point, line Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintAtMidpoint
		c.PtA = point.must(sk, "a point", isPoint)
		c.EntityA = line.must(sk, "a line segment", isLine)
		// A point at the middle of a line needn't also be on it.
		sk.DeleteAllConstraintsFor(core.ConstraintPtOnLine, c.EntityA, c.PtA)
	})
}

// Symmetric makes two points mirror images of each other about a
// workplane or a line segment; with no such thing (Entity{}), about
// the axis of the active workplane they are across.
func (m ConstrainMenu) Symmetric(a, b, about Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.PtA = a.must(sk, "a point", isPoint)
		c.PtB = b.must(sk, "a point", isPoint)
		if about.H.V != 0 {
			c.EntityA = about.H
			switch e := about.get(sk); {
			case e.IsWorkplane():
				c.Type = core.ConstraintSymmetric
			case isLine(e):
				if core.IsFreeIn3D(c.Workplane) {
					fail("symmetry about a line is in a workplane, and none is active")
				}
				c.Type = core.ConstraintSymmetricLine
			default:
				fail("nothing is symmetric about %s", about.describe(sk))
			}
			return
		}
		if core.IsFreeIn3D(c.Workplane) {
			fail("symmetry about nothing is about an axis of the workplane, and none is active")
		}
		dp := sk.MustEntity(c.PtA).PointGetNum(sk).Minus(sk.MustEntity(c.PtB).PointGetNum(sk))
		norm := sk.MustEntity(c.Workplane).NormalEntity(sk)
		if math.Abs(dp.Dot(norm.NormalU(sk))) > math.Abs(dp.Dot(norm.NormalV(sk))) {
			c.Type = core.ConstraintSymmetricHoriz
		} else {
			c.Type = core.ConstraintSymmetricVert
		}
	})
}

// Lock keeps a point where it is.
func (m ConstrainMenu) Lock(point Entity) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintWhereDragged
		c.PtA = point.must(sk, "a point", isPoint)
	})
}

// Comment writes text on the drawing at a place.
func (m ConstrainMenu) Comment(at Vec, text string) Constraint {
	d := m.d
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintComment
		c.Disp.Offset = at
		c.Comment = text
	})
}

// Auto adds to a line, an arc or a circle of the active group
// the constraint the window adds when one is drawn nearly so (Settings,
// "auto-constrain while drawing"): a line horizontal or vertical, else
// tangent to an arc or perpendicular to a line it starts on, parallel to
// a line, or as long as one; an arc tangent at an end to a line or arc
// that ends there, else the same radius as an arc or circle; a circle
// the same radius as one. It is false, with no constraint, when the
// entity is none of these nearly, or the constraint would be redundant.
// Lines and arcs get one only in a workplane.
func (m ConstrainMenu) Auto(e Entity) (c Constraint, ok bool) {
	d := m.d
	d.change(func(sk *sketch.Sketch) {
		if sk.RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		en := e.get(sk)
		if !en.H.IsFromRequest() || en.H != en.H.Request().Entity(0) {
			fail("%s isn't a line, an arc or a circle drawn in the sketch", e.describe(sk))
		}
		r := sk.Request(en.H.Request())
		switch r.Type {
		case sketch.RequestLineSegment, sketch.RequestArcOfCircle, sketch.RequestCircle:
		default:
			fail("%s isn't a line, an arc or a circle", e.describe(sk))
		}
		sk.Active = sk.ActiveGroup()
		if r.Group != sk.Active {
			fail("%s isn't in the active group %s", e.describe(sk), sk.GroupName(sk.Active))
		}
		s, found := sk.SuggestConstraint(r.H, r.Workplane, r.Group)
		if !found {
			return
		}
		if hc := sk.ApplySuggestion(s); hc.V != 0 {
			c, ok = Constraint{d, hc}, true
		}
	})
	return c, ok
}

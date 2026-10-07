package api

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Constraint is a constraint of the sketch: a dimension, such as a
// distance or an angle, or a relation, such as two lines being
// parallel.
type Constraint struct {
	d *Doc
	H core.HConstraint
}

func (c Constraint) get(sk *sketch.Sketch) *core.Constraint {
	if c.d == nil || c.H.V == 0 {
		fail("no constraint")
	}
	constraint := sk.GetConstraint(c.H)
	if constraint == nil {
		fail("constraint c%03x doesn't exist any more", c.H.V)
	}
	return constraint
}

// String describes the constraint as the property browser does, with
// its value if it has one: "c002-pt-pt-distance = 40".
func (c Constraint) String() (s string) {
	if c.d == nil || c.H.V == 0 {
		return "no constraint"
	}
	c.d.read(func(sk *sketch.Sketch) {
		if constraint := sk.GetConstraint(c.H); constraint != nil {
			s = constraintDescription(constraint)
		} else {
			s = fmt.Sprintf("deleted constraint c%03x", c.H.V)
		}
	})
	return s
}

// shownValue is the value as the drawing labels it: positive, and the
// radius of a diameter shown as one.
func shownValue(c *core.Constraint) float64 {
	v := math.Abs(c.ValA)
	if c.Type == core.ConstraintDiameter && c.Other {
		v /= 2
	}
	return v
}

// Valid reports whether the constraint exists: a constraint goes when
// what it constrains is deleted.
func (c Constraint) Valid() (ok bool) {
	if c.d == nil || c.H.V == 0 {
		return false
	}
	c.d.read(func(sk *sketch.Sketch) { ok = sk.GetConstraint(c.H) != nil })
	return ok
}

// Type is the constraint's type, as the core package names them.
func (c Constraint) Type() (t core.ConstraintType) {
	c.d.read(func(sk *sketch.Sketch) { t = c.get(sk).Type })
	return t
}

// Group is the group the constraint is in.
func (c Constraint) Group() (g Group) {
	c.d.read(func(sk *sketch.Sketch) { g = Group{c.d, c.get(sk).Group} })
	return g
}

// Value is the constraint's value as the drawing labels it: a length
// in mm, an angle in degrees, a ratio.
func (c Constraint) Value() (v float64) {
	c.d.read(func(sk *sketch.Sketch) { v = shownValue(c.get(sk)) })
	return v
}

// Set gives the constraint a value, as editing its label on the
// drawing does: a length in mm, an angle in degrees, a ratio. A
// negative distance from a line or a plane puts the point on the
// other side.
func (c Constraint) Set(v float64) {
	c.d.change(func(sk *sketch.Sketch) {
		constraint := c.get(sk)
		if !constraint.HasLabel() {
			fail("%s has no value", constraint.DescriptionString())
		}
		sketch.SetConstraintValue(constraint, v)
		sk.MarkGroupDirty(constraint.Group, false)
	})
}

// SetExpr gives the constraint the value of text, as typing it in its
// label does: a length in the document's unit (Doc.Units), an angle in
// degrees, a ratio. A plain number is stored as its value; any other
// text, "25.4/2" or "wall*4" with a parameter (Doc.SetParameter), is
// kept and evaluated again at each regeneration.
func (c Constraint) SetExpr(expr string) (err error) {
	c.d.change(func(sk *sketch.Sketch) {
		constraint := c.get(sk)
		if !constraint.HasLabel() || constraint.Type == core.ConstraintComment {
			fail("%s has no value", constraint.DescriptionString())
		}
		err = sk.SetConstraintText(c.H, expr)
	})
	return err
}

// Expr is the expression the constraint's value comes from, as its
// label's editor shows it, or "" for a plain number.
func (c Constraint) Expr() (expr string) {
	c.d.read(func(sk *sketch.Sketch) { expr = sketch.ConstraintText(c.get(sk)) })
	return expr
}

// Name is what expressions read the constraint's value by, or "".
func (c Constraint) Name() (name string) {
	c.d.read(func(sk *sketch.Sketch) { name = c.get(sk).Name })
	return name
}

// SetName names the constraint, so that expressions read its value by
// the name as they read a parameter: the value its label shows, in the
// document's unit. Uses of an old name are renamed; "" takes the name
// away, which a name in use can't be. A reference dimension can't be
// named. Parameters and dimensions share the names.
func (c Constraint) SetName(name string) {
	c.d.change(func(sk *sketch.Sketch) {
		c.get(sk)
		if err := sk.SetConstraintName(c.H, name); err != nil {
			fail("%v", err)
		}
	})
}

// Dimension is the constraint named name.
func (d *Doc) Dimension(name string) (c Constraint) {
	d.read(func(sk *sketch.Sketch) {
		con := sk.NamedConstraint(name)
		if con == nil {
			fail("no dimension is named %q", name)
		}
		c = Constraint{d, con.H}
	})
	return c
}

// Reference reports whether the constraint is a reference dimension,
// which measures and doesn't constrain.
func (c Constraint) Reference() (reference bool) {
	c.d.read(func(sk *sketch.Sketch) { reference = c.get(sk).Reference })
	return reference
}

// SetReference makes the constraint a reference dimension, or a
// constraint again.
func (c Constraint) SetReference(reference bool) {
	c.d.change(func(sk *sketch.Sketch) {
		constraint := c.get(sk)
		if !constraint.HasLabel() {
			fail("%s has no value to be a reference", constraint.DescriptionString())
		}
		constraint.Reference = reference
		sk.MarkGroupDirty(constraint.Group, false)
	})
}

// Other is whether an angle measures the supplementary angle, or an
// equal-angle constraint pairs the other angles.
func (c Constraint) Other() (other bool) {
	c.d.read(func(sk *sketch.Sketch) { other = c.get(sk).Other })
	return other
}

// SetOther makes an angle measure the supplementary angle, the other
// one of the two its lines make, or not; for an equal-angle
// constraint it pairs the other angles. Constrain > Other
// Supplementary Angle toggles it.
func (c Constraint) SetOther(other bool) {
	c.d.change(func(sk *sketch.Sketch) {
		constraint := c.get(sk)
		switch constraint.Type {
		case core.ConstraintAngle:
			constraint.Other = other
			constraint.ModifyToSatisfy(sk)
		case core.ConstraintEqualAngle:
			constraint.Other = other
		default:
			fail("%s isn't an angle", constraint.DescriptionString())
		}
		sk.MarkGroupDirty(constraint.Group, false)
	})
}

// Comment is the text of a comment.
func (c Constraint) Comment() (text string) {
	c.d.read(func(sk *sketch.Sketch) { text = c.get(sk).Comment })
	return text
}

// SetComment changes the text of a comment.
func (c Constraint) SetComment(text string) {
	c.d.change(func(sk *sketch.Sketch) {
		constraint := c.get(sk)
		if constraint.Type != core.ConstraintComment {
			fail("%s isn't a comment", constraint.DescriptionString())
		}
		constraint.Comment = text
		sk.Unsaved = true
	})
}

// Delete deletes the constraint.
func (c Constraint) Delete() {
	c.d.change(func(sk *sketch.Sketch) {
		c.get(sk)
		sk.DeleteSelection([]sketch.Selection{{Constraint: c.H}})
	})
}

// constrain adds the constraint build fills in, in the active group
// and workplane. A dimension starts at what it measures, and a
// constraint the sketch already has is not added twice.
func (d *Doc) constrain(build func(sk *sketch.Sketch, c *core.Constraint)) (out Constraint) {
	d.change(func(sk *sketch.Sketch) {
		// What is constrained is measured as the sketch is.
		if sk.RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		sk.Active = sk.ActiveGroup()
		c := core.Constraint{
			Group:     sk.Active,
			Workplane: sk.ActiveWorkplane()}
		build(sk, &c)
		// A group refers only to what is made before it (roadmap 16.4).
		for _, he := range []core.HEntity{c.PtA, c.PtB, c.EntityA, c.EntityB, c.EntityC, c.EntityD} {
			if on, later := sk.EntityAfter(he, c.Group); later {
				fail("%s belongs to %s, which comes after the active group %s in the history; "+
					"a constraint refers to what is made before it",
					Entity{d, he}.describe(sk), sk.GroupName(on), sk.GroupName(c.Group))
			}
		}
		for i := range sk.Constraints {
			if c.Equals(&sk.Constraints[i].ConstraintBase) {
				out = Constraint{d, sk.Constraints[i].H}
				return
			}
		}
		out = Constraint{d, sk.AddConstraintRemember(&c, false)}
	})
	return out
}

// must checks that an entity is what a constraint needs, and returns
// its handle.
func (e Entity) must(sk *sketch.Sketch, what string, is func(*core.Entity) bool) core.HEntity {
	if !is(e.get(sk)) {
		fail("%s isn't %s", e.describe(sk), what)
	}
	return e.H
}

func isPoint(e *core.Entity) bool   { return e.IsPoint() }
func isLine(e *core.Entity) bool    { return e.Type == core.EntityLineSegment }
func isCircle(e *core.Entity) bool  { return e.IsCircle() }
func hasVector(e *core.Entity) bool { return e.HasVector() }

// dimension gives a new dimension the value v: it is first made to
// measure the sketch, which tells its sign.
func dimension(sk *sketch.Sketch, c *core.Constraint, v float64) {
	if c.Type == core.ConstraintPtPtDistance || c.Type == core.ConstraintProjPtDistance {
		// Where the interface puts the label.
		c.Disp.Offset = sk.DistanceLabelOffset(sk.MustGetEntity(c.PtA).PointGetNum(sk),
			sk.MustGetEntity(c.PtB).PointGetNum(sk))
	}
	c.ValA = 0
	c.ModifyToSatisfy(sk)
	sketch.SetConstraintValue(c, v)
}

// Constrain adds a constraint of any type, as the core package names
// them, on up to two points and two other entities; see the solver's
// documentation for what each type takes. Give Entity{} for what a
// type doesn't take.
func (d *Doc) Constrain(t core.ConstraintType, ptA, ptB, entityA, entityB Entity, other, other2 bool) Constraint {
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
func (d *Doc) Coincident(a, b Entity) Constraint {
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
func (d *Doc) Distance(a, b Entity, v float64) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		ea, eb := a.get(sk), b.get(sk)
		if !ea.IsPoint() {
			a, b = b, a
			ea, eb = eb, ea
		}
		if !ea.IsPoint() {
			fail("a distance is from a point, and neither %s nor %s is one", a.describe(sk), b.describe(sk))
		}
		c.PtA = a.H
		switch {
		case eb.IsPoint():
			c.Type = core.ConstraintPtPtDistance
			c.PtB = b.H
		case eb.Type == core.EntityLineSegment:
			c.Type = core.ConstraintPtLineDistance
			c.EntityA = b.H
		case eb.IsWorkplane():
			c.Type = core.ConstraintPtPlaneDistance
			c.EntityA = b.H
		case eb.IsFace():
			c.Type = core.ConstraintPtFaceDistance
			c.EntityA = b.H
		default:
			fail("no distance from a point to %s", b.describe(sk))
		}
		dimension(sk, c, math.Abs(v))
		if v < 0 {
			c.ValA = -c.ValA
		}
	})
}

// Length sets the length of a line segment.
func (d *Doc) Length(line Entity, v float64) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		l := sk.MustGetEntity(line.must(sk, "a line segment", isLine))
		c.Type = core.ConstraintPtPtDistance
		c.PtA, c.PtB = l.Point[0], l.Point[1]
		dimension(sk, c, v)
	})
}

// Diameter sets the diameter of a circle or an arc.
func (d *Doc) Diameter(circle Entity, v float64) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintDiameter
		c.EntityA = circle.must(sk, "a circle or an arc", isCircle)
		dimension(sk, c, v)
	})
}

// Radius sets the radius of a circle or an arc; the drawing shows the
// dimension as a radius.
func (d *Doc) Radius(circle Entity, v float64) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintDiameter
		c.EntityA = circle.must(sk, "a circle or an arc", isCircle)
		c.Other = true
		dimension(sk, c, v)
	})
}

// inWorkplane makes a horizontal or vertical constraint on a line
// segment, or on two points.
func (d *Doc) inWorkplane(t core.ConstraintType, what string, es []Entity) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		if core.IsFreeIn3D(c.Workplane) {
			fail("%s is in a workplane, and none is active", what)
		}
		c.Type = t
		switch len(es) {
		case 1:
			c.EntityA = es[0].must(sk, "a line segment", isLine)
		case 2:
			c.PtA = es[0].must(sk, "a point", isPoint)
			c.PtB = es[1].must(sk, "a point", isPoint)
		default:
			fail("%s takes a line segment or two points", what)
		}
	})
}

// Horizontal makes a line segment, or two points, horizontal in the
// active workplane.
func (d *Doc) Horizontal(lineOrPoints ...Entity) Constraint {
	return d.inWorkplane(core.ConstraintHorizontal, "Horizontal", lineOrPoints)
}

// Vertical makes a line segment, or two points, vertical in the
// active workplane.
func (d *Doc) Vertical(lineOrPoints ...Entity) Constraint {
	return d.inWorkplane(core.ConstraintVertical, "Vertical", lineOrPoints)
}

// Equal makes two line segments as long as each other, two circles or
// arcs of the same radius, or a line segment as long as an arc.
func (d *Doc) Equal(a, b Entity) Constraint {
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
func (d *Doc) Parallel(a, b Entity) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintParallel
		c.EntityA = a.must(sk, "a line segment or a normal", hasVector)
		c.EntityB = b.must(sk, "a line segment or a normal", hasVector)
	})
}

// Perpendicular makes two line segments or normals perpendicular.
func (d *Doc) Perpendicular(a, b Entity) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintPerpendicular
		c.EntityA = a.must(sk, "a line segment or a normal", hasVector)
		c.EntityB = b.must(sk, "a line segment or a normal", hasVector)
	})
}

// Angle sets the angle in degrees between two line segments or
// normals. For two lines that share an end it is the angle at that
// corner.
func (d *Doc) Angle(a, b Entity, degrees float64) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintAngle
		c.EntityA = a.must(sk, "a line segment or a normal", hasVector)
		c.EntityB = b.must(sk, "a line segment or a normal", hasVector)
		ea, eb := a.get(sk), b.get(sk)
		if isLine(ea) && isLine(eb) {
			a0 := sk.MustGetEntity(ea.Point[0]).PointGetNum(sk)
			a1 := sk.MustGetEntity(ea.Point[1]).PointGetNum(sk)
			b0 := sk.MustGetEntity(eb.Point[0]).PointGetNum(sk)
			b1 := sk.MustGetEntity(eb.Point[1]).PointGetNum(sk)
			if !(a0.Equals(b0) || a1.Equals(b1)) && (a0.Equals(b1) || a1.Equals(b0)) {
				// The lines run in opposite senses from the corner.
				c.Other = true
			}
		}
		dimension(sk, c, degrees)
	})
}

// Tangent makes an arc or a cubic tangent to a line segment, or to
// another arc or cubic, where they share an end.
func (d *Doc) Tangent(a, b Entity) Constraint {
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
func (d *Doc) On(point, on Entity) Constraint {
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
func (d *Doc) Midpoint(point, line Entity) Constraint {
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
func (d *Doc) Symmetric(a, b, about Entity) Constraint {
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
		dp := sk.MustGetEntity(c.PtA).PointGetNum(sk).Minus(sk.MustGetEntity(c.PtB).PointGetNum(sk))
		norm := sk.MustGetEntity(c.Workplane).NormalEntity(sk)
		if math.Abs(dp.Dot(norm.NormalU(sk))) > math.Abs(dp.Dot(norm.NormalV(sk))) {
			c.Type = core.ConstraintSymmetricHoriz
		} else {
			c.Type = core.ConstraintSymmetricVert
		}
	})
}

// Lock keeps a point where it is.
func (d *Doc) Lock(point Entity) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintWhereDragged
		c.PtA = point.must(sk, "a point", isPoint)
	})
}

// Comment writes text on the drawing at a place.
func (d *Doc) Comment(at Vec, text string) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintComment
		c.Disp.Offset = at
		c.Comment = text
	})
}

// AutoConstrain adds to a line, an arc or a circle of the active group
// the constraint the window adds when one is drawn nearly so (Settings,
// "auto-constrain while drawing"): a line horizontal or vertical, else
// tangent to an arc or perpendicular to a line it starts on, parallel to
// a line, or as long as one; an arc tangent at an end to a line or arc
// that ends there, else the same radius as an arc or circle; a circle
// the same radius as one. It is false, with no constraint, when the
// entity is none of these nearly, or the constraint would be redundant.
// Lines and arcs get one only in a workplane.
func (d *Doc) AutoConstrain(e Entity) (c Constraint, ok bool) {
	d.change(func(sk *sketch.Sketch) {
		if sk.RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		en := e.get(sk)
		if !en.H.IsFromRequest() || en.H != en.H.Request().Entity(0) {
			fail("%s isn't a line, an arc or a circle drawn in the sketch", e.describe(sk))
		}
		r := sk.GetRequest(en.H.Request())
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

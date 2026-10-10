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
	constraint := sk.Constraint(c.H)
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
		if constraint := sk.Constraint(c.H); constraint != nil {
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
	c.d.read(func(sk *sketch.Sketch) { ok = sk.Constraint(c.H) != nil })
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
// label does: a length in the document's unit (View.Units), an angle in
// degrees, a ratio. A plain number is stored as its value; any other
// text, "25.4/2" or "wall*4" with a parameter (Params.Set), is
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
	switch c.Type {
	case core.ConstraintPtPtDistance, core.ConstraintProjPtDistance:
		// Where the interface puts the label.
		c.Disp.Offset = sk.DistanceLabelOffset(sk.MustEntity(c.PtA).PointGetNum(sk),
			sk.MustEntity(c.PtB).PointGetNum(sk))
	}
	c.ValA = 0
	c.ModifyToSatisfy(sk)
	sketch.SetConstraintValue(c, v)
}

// distanceBetween makes c the distance between a and b, as Distance
// does, or a's length when b is Entity{} and a is a line segment.
func distanceBetween(sk *sketch.Sketch, c *core.Constraint, a, b Entity, v float64) {
	if b.H.V == 0 {
		l := sk.MustEntity(a.must(sk, "a line segment", isLine))
		c.Type = core.ConstraintPtPtDistance
		c.PtA, c.PtB = l.Point[0], l.Point[1]
		dimension(sk, c, v)
		return
	}
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
}

// boundedDistance makes a MinDistance or a MaxDistance.
func (d *Doc) boundedDistance(limit core.Limit, a, b Entity, v float64) Constraint {
	return d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		distanceBetween(sk, c, a, b, v)
		c.Limit = limit
	})
}

// Limits a dimension can have (Constraint.SetLimit).
const (
	// NoLimit is a dimension that holds its value.
	NoLimit = core.LimitNone
	// Minimum is a dimension that may be its value or more.
	Minimum = core.LimitMin
	// Maximum is a dimension that may be its value or less.
	Maximum = core.LimitMax
)

// SetLimit makes the dimension a bound: Minimum lets what it measures
// be its value or more, Maximum its value or less, and NoLimit makes it
// hold its value again. Any distance, a diameter or radius, and an
// angle take a limit; for an angle, Minimum is the least angle.
func (c Constraint) SetLimit(l core.Limit) {
	c.d.change(func(sk *sketch.Sketch) {
		constraint := c.get(sk)
		if !constraint.CanLimit() {
			fail("%s can't be a minimum or a maximum", constraint.DescriptionString())
		}
		if l != core.LimitNone && l != core.LimitMin && l != core.LimitMax {
			fail("no limit %d", l)
		}
		constraint.Limit = l
		sk.MarkGroupDirty(constraint.Group, false)
	})
}

// Limit is the dimension's limit: NoLimit, Minimum or Maximum.
func (c Constraint) Limit() (l core.Limit) {
	c.d.read(func(sk *sketch.Sketch) { l = c.get(sk).Limit })
	return l
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

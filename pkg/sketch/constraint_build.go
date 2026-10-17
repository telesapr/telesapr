package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file makes the constraints that the Constrain menu makes from a
// selection, as values: the window reads a selection and a script is
// given its entities, and both come here, so that they read them the
// same way. None adds the constraint; AddConstraints and the callers'
// undo steps do.

// Implied is a constraint that a new constraint makes redundant, and
// that the caller deletes in the same undo step as it adds the new one.
type Implied struct {
	Type    core.ConstraintType
	EntityA core.HEntity
	PtA     core.HEntity
}

// DeleteImplied deletes the constraints imp names.
func (sk *Sketch) DeleteImplied(imp []Implied) {
	for _, i := range imp {
		sk.DeleteAllConstraintsFor(i.Type, i.EntityA, i.PtA)
	}
}

// ErrConstraintSelection is wrapped by the errors of the builders in this
// file for entities that the constraint does not take.
var ErrConstraintSelection = errors.New("not a selection the constraint takes")

func selectionError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConstraintSelection, fmt.Sprintf(format, args...))
}

// entityOf is the entity he, or an error saying it is not there.
func (sk *Sketch) entityOf(he core.HEntity) (*core.Entity, error) {
	e := sk.Entity(he)
	if e == nil {
		return nil, selectionError("entity %08x does not exist", he.V)
	}
	return e, nil
}

// TangentConstraint makes the constraint that makes a and b tangent,
// as a copy of base: an arc or a cubic with a line segment (in either
// order), or an arc or a cubic with another, in the workplane of base.
// They are tangent where they share an end; ends are the end points to
// use when they do not, one of the arc or cubic for a line segment, and
// one of each curve for two curves.
func (sk *Sketch) TangentConstraint(base core.Constraint, a, b core.HEntity, ends ...core.HEntity) (core.Constraint, error) {
	ea, err := sk.entityOf(a)
	if err != nil {
		return base, err
	}
	eb, err := sk.entityOf(b)
	if err != nil {
		return base, err
	}
	var pts []*core.Entity
	for _, he := range ends {
		p, err := sk.entityOf(he)
		if err != nil {
			return base, err
		}
		if !p.IsPoint() {
			return base, selectionError("an end point is a point")
		}
		pts = append(pts, p)
	}
	isCurve := func(e *core.Entity) bool { return e.Type == core.EntityArcOfCircle || e.Type == core.EntityCubic }
	isLine := func(e *core.Entity) bool { return e.Type == core.EntityLineSegment }

	if isLine(eb) {
		ea, eb = eb, ea
	}
	c := base
	switch {
	case isLine(ea) && isCurve(eb):
		if len(pts) > 1 {
			return base, selectionError("a curve and a line segment take one end point, not %d", len(pts))
		}
		var ep *core.Entity
		if len(pts) == 1 {
			ep = pts[0]
		}
		if eb.Type == core.EntityArcOfCircle {
			err = sk.ConstrainArcLineTangent(&c, ea, eb, ep)
			c.Type = core.ConstraintArcLineTangent
		} else {
			err = sk.ConstrainCubicLineTangent(&c, ea, eb, ep)
			c.Type = core.ConstraintCubicLineTangent
		}
		c.EntityA, c.EntityB = eb.H, ea.H
	case isCurve(ea) && isCurve(eb):
		if core.IsFreeIn3D(c.Workplane) {
			return base, selectionError("curves are made tangent in a workplane, and none is active")
		}
		var epA, epB *core.Entity
		switch len(pts) {
		case 0:
		case 2:
			epA, epB = pts[0], pts[1]
		default:
			return base, selectionError("two curves take an end point of each, not %d points", len(pts))
		}
		err = sk.ConstrainCurveCurveTangent(&c, ea, eb, epA, epB)
		c.Type = core.ConstraintCurveCurveTangent
		c.EntityA, c.EntityB = ea.H, eb.H
	default:
		return base, selectionError("entities of types %d and %d can not be tangent", int(ea.Type), int(eb.Type))
	}
	if err != nil {
		return base, err
	}
	return c, nil
}

// ParallelConstraint makes the constraint that makes a and b parallel,
// as a copy of base: two line segments or normals in any mix, or two
// faces.
func (sk *Sketch) ParallelConstraint(base core.Constraint, a, b core.HEntity) (core.Constraint, error) {
	return sk.directionConstraint(base, core.ConstraintParallel, a, b)
}

// PerpendicularConstraint makes the constraint that makes a and b
// perpendicular, as a copy of base: two line segments or normals in any
// mix, or two faces.
func (sk *Sketch) PerpendicularConstraint(base core.Constraint, a, b core.HEntity) (core.Constraint, error) {
	return sk.directionConstraint(base, core.ConstraintPerpendicular, a, b)
}

func (sk *Sketch) directionConstraint(base core.Constraint, typ core.ConstraintType, a, b core.HEntity) (core.Constraint, error) {
	ea, err := sk.entityOf(a)
	if err != nil {
		return base, err
	}
	eb, err := sk.entityOf(b)
	if err != nil {
		return base, err
	}
	if !(ea.IsFace() && eb.IsFace()) && !(ea.HasVector() && eb.HasVector()) {
		return base, selectionError("two line segments or normals, or two faces, are parallel or perpendicular")
	}
	c := base
	c.Type, c.EntityA, c.EntityB = typ, a, b
	return c, nil
}

// MidpointConstraint makes the constraint that puts the middle of the
// line segment line at point, which may be a point or a workplane (the
// middle then lies in the plane), as a copy of base. A point at the
// middle of a line needn't also be on it, so the constraint that puts
// it on the line is implied.
func (sk *Sketch) MidpointConstraint(base core.Constraint, line, point core.HEntity) (core.Constraint, []Implied, error) {
	el, err := sk.entityOf(line)
	if err != nil {
		return base, nil, err
	}
	ep, err := sk.entityOf(point)
	if err != nil {
		return base, nil, err
	}
	if el.Type != core.EntityLineSegment {
		return base, nil, selectionError("a midpoint is of a line segment")
	}
	c := base
	c.Type, c.EntityA = core.ConstraintAtMidpoint, line
	switch {
	case ep.IsPoint():
		c.PtA = point
		return c, []Implied{{core.ConstraintPtOnLine, line, point}}, nil
	case ep.IsWorkplane():
		c.EntityB = point
		return c, nil, nil
	}
	return base, nil, selectionError("a line segment's midpoint is at a point or in a workplane")
}

// SymmetricConstraint makes the constraint that makes two points
// symmetric, as a copy of base: a and b are the points, or a is a line
// segment and b is NoEntity, and its two ends are the points. They are
// symmetric about about, a workplane or a line segment (in the workplane
// of base); with about NoEntity, about the axis of that workplane they
// are across. Where a line segment is given with no about, it will be
// symmetric about that axis, and so no longer needs the horizontal or
// vertical constraint it had, which is implied.
func (sk *Sketch) SymmetricConstraint(base core.Constraint, a, b, about core.HEntity) (core.Constraint, []Implied, error) {
	ea, err := sk.entityOf(a)
	if err != nil {
		return base, nil, err
	}
	c := base
	var implied []Implied
	switch {
	case ea.IsPoint():
		eb, err := sk.entityOf(b)
		if err != nil {
			return base, nil, err
		}
		if !eb.IsPoint() {
			return base, nil, selectionError("symmetric takes two points or a line segment")
		}
		c.PtA, c.PtB = a, b
	case ea.Type == core.EntityLineSegment && b == core.NoEntity:
		c.PtA, c.PtB = ea.Point[0], ea.Point[1]
		if about == core.NoEntity {
			implied = []Implied{{core.ConstraintHorizontal, a, core.NoEntity}, {core.ConstraintVertical, a, core.NoEntity}}
		}
	default:
		return base, nil, selectionError("symmetric takes two points or a line segment")
	}

	if about == core.NoEntity {
		if core.IsFreeIn3D(c.Workplane) {
			return base, nil, selectionError("symmetry about nothing is about an axis of the workplane, and none is active")
		}
		pa := sk.MustEntity(c.PtA).PointGetNum(sk)
		pb := sk.MustEntity(c.PtB).PointGetNum(sk)
		dp := pa.Minus(pb)
		norm := sk.MustEntity(c.Workplane).NormalEntity(sk)
		if math.Abs(dp.Dot(norm.NormalU(sk))) > math.Abs(dp.Dot(norm.NormalV(sk))) {
			c.Type = core.ConstraintSymmetricHoriz
		} else {
			c.Type = core.ConstraintSymmetricVert
		}
		return c, implied, nil
	}
	eab, err := sk.entityOf(about)
	if err != nil {
		return base, nil, err
	}
	c.EntityA = about
	switch {
	case eab.IsWorkplane():
		c.Type = core.ConstraintSymmetric
	case eab.Type == core.EntityLineSegment:
		if core.IsFreeIn3D(c.Workplane) {
			return base, nil, selectionError("symmetry about a line is in a workplane, and none is active")
		}
		c.Type = core.ConstraintSymmetricLine
	default:
		return base, nil, selectionError("nothing is symmetric about an entity of type %d", int(eab.Type))
	}
	return c, nil, nil
}

// CommentConstraint makes a comment with the given text, attached to
// the point pt when that is not NoEntity, as a copy of base.
func (sk *Sketch) CommentConstraint(base core.Constraint, pt core.HEntity, text string) (core.Constraint, error) {
	c := base
	c.Type, c.Comment = core.ConstraintComment, text
	if pt != core.NoEntity {
		p, err := sk.entityOf(pt)
		if err != nil {
			return base, err
		}
		if !p.IsPoint() {
			return base, selectionError("a comment is attached to a point")
		}
		c.PtA = pt
	}
	return c, nil
}

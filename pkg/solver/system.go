package solver

import (
	"fmt"
	"math"
	"sort"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// Resolver supplies entities and parameters needed to generate equations.
type Resolver interface {
	Entity(core.HEntity) (*core.Entity, error)
	Param(core.HParam) (*core.Param, error)
}

// Equation encapsulates a symbolic constraint equation.
type Equation struct {
	H           core.HEquation
	Expr        *core.Expr
	derivatives map[core.HParam]*core.Expr
}

func (e *Equation) derivative(h core.HParam) *core.Expr {
	if e.derivatives == nil {
		e.derivatives = make(map[core.HParam]*core.Expr)
	}
	if d, ok := e.derivatives[h]; ok {
		return d
	}
	d := e.Expr.Derivative(h).FoldConstants()
	e.derivatives[h] = d
	return d
}

// UnsupportedConstraintError is returned when a constraint type lacks an equation builder.
type UnsupportedConstraintError struct {
	Type core.ConstraintType
}

func (e UnsupportedConstraintError) Error() string {
	return fmt.Sprintf("solver: constraint type %d not supported", e.Type)
}

// System accumulates parameters and equations generated from constraints.
type System struct {
	resolver Resolver

	Params    map[core.HParam]*core.Param
	Equations []*Equation
}

// NewSystem creates an empty symbolic system backed by resolver.
func NewSystem(res Resolver) *System {
	return &System{
		resolver: res,
		Params:   make(map[core.HParam]*core.Param),
	}
}

func (s *System) entity(handle core.HEntity) (*core.Entity, error) {
	if handle == (core.HEntity{}) {
		return nil, fmt.Errorf("solver: entity handle zero")
	}
	ent, err := s.resolver.Entity(handle)
	if err != nil {
		return nil, err
	}
	if ent == nil {
		return nil, fmt.Errorf("solver: entity %08x missing", handle.V)
	}
	return ent, nil
}

// AddConstraint expands the supplied constraint into equations (where supported).
func (s *System) AddConstraint(c *core.Constraint) error {
	if c == nil {
		return fmt.Errorf("solver: nil constraint")
	}

	switch c.Type {
	case core.ConstraintPointsCoincident:
		return s.addCoincidentPoints(c)
	case core.ConstraintPtPtDistance:
		return s.addPointPointDistance(c)
	case core.ConstraintProjPtDistance:
		return s.addProjectedPointDistance(c)
	case core.ConstraintPtLineDistance:
		return s.addPointLineDistance(c)
	case core.ConstraintPtPlaneDistance:
		return s.addPointPlaneDistance(c)
	case core.ConstraintPtFaceDistance:
		return s.addPointFaceDistance(c)
	case core.ConstraintPtInPlane:
		return s.addPointInPlane(c)
	case core.ConstraintPtOnLine:
		return s.addPointOnLine(c)
	case core.ConstraintPtOnFace:
		return s.addPointOnFace(c)
	case core.ConstraintPtOnCircle:
		return s.addPointOnCircle(c)
	case core.ConstraintEqualLengthLines:
		return s.addEqualLineLength(c)
	case core.ConstraintEqualLineArcLen:
		return s.addEqualLineArcLength(c)
	case core.ConstraintEqPtLnDistances:
		return s.addEqualPointLineDistances(c)
	case core.ConstraintEqLenPtLineD:
		return s.addEqualLinePointDistance(c)
	case core.ConstraintLengthRatio:
		return s.addLengthRatio(c)
	case core.ConstraintArcArcLenRatio:
		return s.addArcArcLengthRatio(c)
	case core.ConstraintArcLineLenRatio:
		return s.addArcLineLengthRatio(c)
	case core.ConstraintLengthDifference:
		return s.addLengthDifference(c)
	case core.ConstraintArcArcDifference:
		return s.addArcArcDifference(c)
	case core.ConstraintArcLineDifference:
		return s.addArcLineDifference(c)
	case core.ConstraintDiameter:
		return s.addDiameterConstraint(c)
	case core.ConstraintEqualRadius:
		return s.addEqualRadiusConstraint(c)
	case core.ConstraintSameOrientation:
		return s.addSameOrientationConstraint(c)
	case core.ConstraintSymmetric:
		return s.addSymmetricConstraint(c)
	case core.ConstraintSymmetricHoriz:
		return s.addSymmetricHVConstraint(c, true)
	case core.ConstraintSymmetricVert:
		return s.addSymmetricHVConstraint(c, false)
	case core.ConstraintSymmetricLine:
		return s.addSymmetricLineConstraint(c)
	case core.ConstraintAtMidpoint:
		return s.addMidpointConstraint(c)
	case core.ConstraintParallel:
		return s.addParallelConstraint(c)
	case core.ConstraintPerpendicular:
		return s.addPerpendicularConstraint(c)
	case core.ConstraintAngle:
		return s.addAngleConstraint(c)
	case core.ConstraintArcLineTangent:
		return s.addArcLineTangentConstraint(c)
	case core.ConstraintCubicLineTangent:
		return s.addCubicLineTangentConstraint(c)
	case core.ConstraintCurveCurveTangent:
		return s.addCurveCurveTangentConstraint(c)
	case core.ConstraintEqualAngle:
		return s.addEqualAngleConstraint(c)
	case core.ConstraintWhereDragged:
		return s.addWhereDraggedConstraint(c)
	case core.ConstraintComment:
		return nil
	case core.ConstraintHorizontal:
		return s.addHorizontalConstraint(c, true)
	case core.ConstraintVertical:
		return s.addHorizontalConstraint(c, false)
	default:
		return UnsupportedConstraintError{Type: c.Type}
	}
}

func (s *System) addCoincidentPoints(c *core.Constraint) error {
	pa, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("coincident: %w", err)
	}
	pb, err := s.pointExpr(c.PtB)
	if err != nil {
		return fmt.Errorf("coincident: %w", err)
	}

	diff := pa.Minus(pb)
	comps := []*core.Expr{diff.X, diff.Y, diff.Z}

	for i, expr := range comps {
		if err := s.registerParams(expr); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{
			H:    c.H.Equation(i),
			Expr: expr,
		})
	}
	return nil
}

func (s *System) addPointPointDistance(c *core.Constraint) error {
	pa, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("distance: %w", err)
	}
	pb, err := s.pointExpr(c.PtB)
	if err != nil {
		return fmt.Errorf("distance: %w", err)
	}

	diff := pa.Minus(pb)
	distSq := diff.Dot(diff)
	target := s.scalarValueExpr(c)
	expr := distSq.Minus(target.Square())

	if err := s.registerParams(expr); err != nil {
		return err
	}

	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addPointInPlane(c *core.Constraint) error {
	p, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("point-in-plane: %w", err)
	}
	plane, err := s.planeExpr(c.Workplane)
	if err != nil {
		return fmt.Errorf("point-in-plane: %w", err)
	}

	expr := plane.normal.Dot(p.Minus(plane.point))
	if err := s.registerParams(expr); err != nil {
		return err
	}

	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addPointOnLine(c *core.Constraint) error {
	p, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("point-on-line: %w", err)
	}
	line, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("point-on-line: %w", err)
	}

	vec := p.Minus(line.a)
	cross := vec.Cross(line.direction)

	comps := []*core.Expr{cross.X, cross.Y, cross.Z}
	for i, expr := range comps {
		if err := s.registerParams(expr); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{
			H:    c.H.Equation(i),
			Expr: expr,
		})
	}
	return nil
}

func (s *System) addProjectedPointDistance(c *core.Constraint) error {
	pa, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("proj-pt-distance: %w", err)
	}
	pb, err := s.pointExpr(c.PtB)
	if err != nil {
		return fmt.Errorf("proj-pt-distance: %w", err)
	}
	dir, err := s.vectorExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("proj-pt-distance: %w", err)
	}
	unit := dir.WithMagnitude(core.NewConstant(1))
	delta := pb.Minus(pa)
	expr := delta.Dot(unit).Minus(s.scalarValueExpr(c))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addPointLineDistance(c *core.Constraint) error {
	p, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("point-line-distance: %w", err)
	}
	line, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("point-line-distance: %w", err)
	}
	var planePtr *planeData
	if !isFreeWorkplane(c.Workplane) {
		wp, err := s.planeExpr(c.Workplane)
		if err != nil {
			return fmt.Errorf("point-line-distance: %w", err)
		}
		planePtr = &wp
	}
	distance := pointLineDistanceExpr(p, line, planePtr)
	target := s.scalarValueExpr(c)
	expr := distance.Minus(target)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addPointPlaneDistance(c *core.Constraint) error {
	p, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("point-plane-distance: %w", err)
	}
	plane, err := s.planeExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("point-plane-distance: %w", err)
	}
	distance := pointPlaneDistanceExpr(p, plane)
	target := s.scalarValueExpr(c)
	expr := distance.Minus(target)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addPointFaceDistance(c *core.Constraint) error {
	p, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("point-face-distance: %w", err)
	}
	face, err := s.faceExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("point-face-distance: %w", err)
	}
	distance := pointPlaneDistanceExprWithNormal(p, face.point, face.normal)
	target := s.scalarValueExpr(c)
	expr := distance.Minus(target)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addPointOnFace(c *core.Constraint) error {
	p, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("point-on-face: %w", err)
	}
	face, err := s.faceExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("point-on-face: %w", err)
	}
	expr := pointPlaneDistanceExprWithNormal(p, face.point, face.normal)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addEqualLineLength(c *core.Constraint) error {
	lineA, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("equal-length: %w", err)
	}
	lineB, err := s.lineExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("equal-length: %w", err)
	}
	lenA := lineA.direction.Magnitude()
	lenB := lineB.direction.Magnitude()
	expr := lenA.Minus(lenB)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addPointOnCircle(c *core.Constraint) error {
	circleEnt, err := s.resolver.Entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("point-on-circle: %w", err)
	}
	if circleEnt == nil {
		return fmt.Errorf("point-on-circle: circle entity %08x missing", c.EntityA.V)
	}
	data, err := s.circleData(circleEnt)
	if err != nil {
		return fmt.Errorf("point-on-circle: %w", err)
	}
	point, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("point-on-circle: %w", err)
	}
	delta := data.center.Minus(point)
	du := delta.Dot(data.plane.u)
	dv := delta.Dot(data.plane.v)
	expr := du.Square().Plus(dv.Square()).Minus(data.radius.Square())
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addEqualPointLineDistances(c *core.Constraint) error {
	lineA, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("eq-pt-ln-distances: %w", err)
	}
	lineB, err := s.lineExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("eq-pt-ln-distances: %w", err)
	}
	var planePtr *planeData
	if !isFreeWorkplane(c.Workplane) {
		wp, err := s.planeExpr(c.Workplane)
		if err != nil {
			return fmt.Errorf("eq-pt-ln-distances: %w", err)
		}
		planePtr = &wp
	}
	pointA, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("eq-pt-ln-distances: %w", err)
	}
	pointB, err := s.pointExpr(c.PtB)
	if err != nil {
		return fmt.Errorf("eq-pt-ln-distances: %w", err)
	}
	d1 := pointLineDistanceExpr(pointA, lineA, planePtr)
	d2 := pointLineDistanceExpr(pointB, lineB, planePtr)
	expr := d1.Square().Minus(d2.Square())
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addEqualLinePointDistance(c *core.Constraint) error {
	line, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("eq-len-pt-line-d: %w", err)
	}
	targetLine, err := s.lineExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("eq-len-pt-line-d: %w", err)
	}
	point, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("eq-len-pt-line-d: %w", err)
	}
	length := line.direction.Magnitude()
	var planePtr *planeData
	if !isFreeWorkplane(c.Workplane) {
		wp, err := s.planeExpr(c.Workplane)
		if err != nil {
			return fmt.Errorf("eq-len-pt-line-d: %w", err)
		}
		planePtr = &wp
	}
	distance := pointLineDistanceExpr(point, targetLine, planePtr)
	expr := length.Square().Minus(distance.Square())
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addLengthRatio(c *core.Constraint) error {
	lineA, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("length-ratio: %w", err)
	}
	lineB, err := s.lineExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("length-ratio: %w", err)
	}
	lenA := lineA.direction.Magnitude()
	lenB := lineB.direction.Magnitude()
	expr := lenA.Div(lenB).Minus(s.scalarValueExpr(c))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addLengthDifference(c *core.Constraint) error {
	lineA, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("length-difference: %w", err)
	}
	lineB, err := s.lineExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("length-difference: %w", err)
	}
	lenA := lineA.direction.Magnitude()
	lenB := lineB.direction.Magnitude()
	expr := lenA.Minus(lenB).Minus(s.scalarValueExpr(c))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addDiameterConstraint(c *core.Constraint) error {
	circleEnt, err := s.resolver.Entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("diameter: %w", err)
	}
	if circleEnt == nil {
		return fmt.Errorf("diameter: circle entity %08x missing", c.EntityA.V)
	}
	data, err := s.circleData(circleEnt)
	if err != nil {
		return fmt.Errorf("diameter: %w", err)
	}
	expr := data.radius.Times(core.NewConstant(2)).Minus(s.scalarValueExpr(c))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addEqualRadiusConstraint(c *core.Constraint) error {
	circleA, err := s.resolver.Entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("equal-radius: %w", err)
	}
	if circleA == nil {
		return fmt.Errorf("equal-radius: circle entity %08x missing", c.EntityA.V)
	}
	circleB, err := s.resolver.Entity(c.EntityB)
	if err != nil {
		return fmt.Errorf("equal-radius: %w", err)
	}
	if circleB == nil {
		return fmt.Errorf("equal-radius: circle entity %08x missing", c.EntityB.V)
	}
	dataA, err := s.circleData(circleA)
	if err != nil {
		return fmt.Errorf("equal-radius: %w", err)
	}
	dataB, err := s.circleData(circleB)
	if err != nil {
		return fmt.Errorf("equal-radius: %w", err)
	}
	expr := dataA.radius.Minus(dataB.radius)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addEqualLineArcLength(c *core.Constraint) error {
	line, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("equal-line-arc: %w", err)
	}
	arc, err := s.entity(c.EntityB)
	if err != nil {
		return fmt.Errorf("equal-line-arc: %w", err)
	}
	if arc.Type != core.EntityArcOfCircle {
		return fmt.Errorf("equal-line-arc: entity %08x type %d unsupported", c.EntityB.V, arc.Type)
	}
	lineLen := line.direction.Magnitude()
	arcLen, err := s.arcLengthExpr(arc)
	if err != nil {
		return fmt.Errorf("equal-line-arc: %w", err)
	}
	expr := arcLen.Minus(lineLen)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addArcArcLengthRatio(c *core.Constraint) error {
	arcA, err := s.entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("arc-arc-ratio: %w", err)
	}
	if arcA.Type != core.EntityArcOfCircle {
		return fmt.Errorf("arc-arc-ratio: entity %08x type %d unsupported", c.EntityA.V, arcA.Type)
	}
	arcB, err := s.entity(c.EntityB)
	if err != nil {
		return fmt.Errorf("arc-arc-ratio: %w", err)
	}
	if arcB.Type != core.EntityArcOfCircle {
		return fmt.Errorf("arc-arc-ratio: entity %08x type %d unsupported", c.EntityB.V, arcB.Type)
	}
	lenA, err := s.arcLengthExpr(arcA)
	if err != nil {
		return fmt.Errorf("arc-arc-ratio: %w", err)
	}
	lenB, err := s.arcLengthExpr(arcB)
	if err != nil {
		return fmt.Errorf("arc-arc-ratio: %w", err)
	}
	expr := lenA.Div(lenB).Minus(s.scalarValueExpr(c))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addArcLineLengthRatio(c *core.Constraint) error {
	line, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("arc-line-ratio: %w", err)
	}
	arc, err := s.entity(c.EntityB)
	if err != nil {
		return fmt.Errorf("arc-line-ratio: %w", err)
	}
	if arc.Type != core.EntityArcOfCircle {
		return fmt.Errorf("arc-line-ratio: entity %08x type %d unsupported", c.EntityB.V, arc.Type)
	}
	lineLen := line.direction.Magnitude()
	arcLen, err := s.arcLengthExpr(arc)
	if err != nil {
		return fmt.Errorf("arc-line-ratio: %w", err)
	}
	expr := arcLen.Div(lineLen).Minus(s.scalarValueExpr(c))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addArcArcDifference(c *core.Constraint) error {
	arcA, err := s.entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("arc-arc-diff: %w", err)
	}
	if arcA.Type != core.EntityArcOfCircle {
		return fmt.Errorf("arc-arc-diff: entity %08x type %d unsupported", c.EntityA.V, arcA.Type)
	}
	arcB, err := s.entity(c.EntityB)
	if err != nil {
		return fmt.Errorf("arc-arc-diff: %w", err)
	}
	if arcB.Type != core.EntityArcOfCircle {
		return fmt.Errorf("arc-arc-diff: entity %08x type %d unsupported", c.EntityB.V, arcB.Type)
	}
	lenA, err := s.arcLengthExpr(arcA)
	if err != nil {
		return fmt.Errorf("arc-arc-diff: %w", err)
	}
	lenB, err := s.arcLengthExpr(arcB)
	if err != nil {
		return fmt.Errorf("arc-arc-diff: %w", err)
	}
	expr := lenA.Minus(lenB).Minus(s.scalarValueExpr(c))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addArcLineDifference(c *core.Constraint) error {
	line, err := s.lineExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("arc-line-diff: %w", err)
	}
	arc, err := s.entity(c.EntityB)
	if err != nil {
		return fmt.Errorf("arc-line-diff: %w", err)
	}
	if arc.Type != core.EntityArcOfCircle {
		return fmt.Errorf("arc-line-diff: entity %08x type %d unsupported", c.EntityB.V, arc.Type)
	}
	lineLen := line.direction.Magnitude()
	arcLen, err := s.arcLengthExpr(arc)
	if err != nil {
		return fmt.Errorf("arc-line-diff: %w", err)
	}
	expr := arcLen.Minus(lineLen).Minus(s.scalarValueExpr(c))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addParallelConstraint(c *core.Constraint) error {
	va, err := s.vectorExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("parallel: %w", err)
	}
	vb, err := s.vectorExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("parallel: %w", err)
	}
	ua := va.WithMagnitude(core.NewConstant(1))
	ub := vb.WithMagnitude(core.NewConstant(1))
	cross := ua.Cross(ub)
	for i, component := range []*core.Expr{cross.X, cross.Y, cross.Z} {
		if err := s.registerParams(component); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{
			H:    c.H.Equation(i),
			Expr: component,
		})
	}
	return nil
}

func (s *System) addPerpendicularConstraint(c *core.Constraint) error {
	va, err := s.vectorExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("perpendicular: %w", err)
	}
	vb, err := s.vectorExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("perpendicular: %w", err)
	}
	ua := va.WithMagnitude(core.NewConstant(1))
	ub := vb.WithMagnitude(core.NewConstant(1))
	dot := ua.Dot(ub)
	if err := s.registerParams(dot); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: dot,
	})
	return nil
}

func (s *System) addHorizontalConstraint(c *core.Constraint, horizontal bool) error {
	if c.Workplane == (core.HEntity{}) {
		return fmt.Errorf("horizontal/vertical constraint requires workplane")
	}
	plane, err := s.planeExpr(c.Workplane)
	if err != nil {
		return fmt.Errorf("horizontal/vertical: %w", err)
	}

	var h1, h2 core.HEntity
	if c.EntityA != (core.HEntity{}) {
		lineEnt, err := s.resolver.Entity(c.EntityA)
		if err != nil {
			return fmt.Errorf("horizontal/vertical: %w", err)
		}
		if lineEnt == nil {
			return fmt.Errorf("horizontal/vertical: line entity %08x missing", c.EntityA.V)
		}
		h1 = lineEnt.Point[0]
		h2 = lineEnt.Point[1]
	} else {
		h1 = c.PtA
		h2 = c.PtB
	}
	if h1 == (core.HEntity{}) || h2 == (core.HEntity{}) {
		return fmt.Errorf("horizontal/vertical: missing point handles")
	}
	p1, err := s.pointExpr(h1)
	if err != nil {
		return fmt.Errorf("horizontal/vertical: %w", err)
	}
	p2, err := s.pointExpr(h2)
	if err != nil {
		return fmt.Errorf("horizontal/vertical: %w", err)
	}
	u1, v1 := plane.coords(p1)
	u2, v2 := plane.coords(p2)
	var expr *core.Expr
	if horizontal {
		expr = v1.Minus(v2)
	} else {
		expr = u1.Minus(u2)
	}
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addAngleConstraint(c *core.Constraint) error {
	va, err := s.vectorExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("angle: %w", err)
	}
	vb, err := s.vectorExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("angle: %w", err)
	}
	if c.Other {
		va = negateVector(va)
	}
	if c.Other2 {
		vb = negateVector(vb)
	}
	ua := va.WithMagnitude(core.NewConstant(1))
	ub := vb.WithMagnitude(core.NewConstant(1))
	dot := ua.Dot(ub)
	angle := s.scalarValueExpr(c)
	cos := angle.Times(core.NewConstant(math.Pi / 180)).Cos()
	expr := dot.Minus(cos)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addSameOrientationConstraint(c *core.Constraint) error {
	if c.EntityA == (core.HEntity{}) || c.EntityB == (core.HEntity{}) {
		return fmt.Errorf("same-orientation: missing entity handle")
	}
	ua, _, na, err := s.normalFrameExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("same-orientation: %w", err)
	}
	ub, vb, nb, err := s.normalFrameExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("same-orientation: %w", err)
	}

	eq := vectorsParallel3D(na, nb, c.ValP)
	for i, component := range []*core.Expr{eq.X, eq.Y, eq.Z} {
		if err := s.registerParams(component); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{
			H:    c.H.Equation(i),
			Expr: component,
		})
	}

	uaNum, _, _, err := s.normalFrameNumeric(c.EntityA)
	if err != nil {
		return fmt.Errorf("same-orientation: %w", err)
	}
	ubNum, vbNum, _, err := s.normalFrameNumeric(c.EntityB)
	if err != nil {
		return fmt.Errorf("same-orientation: %w", err)
	}

	d1 := ua.Dot(vb)
	d2 := ua.Dot(ub)

	d1Val := uaNum.Dot(vbNum)
	d2Val := uaNum.Dot(ubNum)

	var orientExpr *core.Expr
	if math.Abs(d1Val) < math.Abs(d2Val) {
		orientExpr = d1
	} else {
		orientExpr = d2
	}

	if err := s.registerParams(orientExpr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(3),
		Expr: orientExpr,
	})
	return nil
}

func (s *System) addSymmetricConstraint(c *core.Constraint) error {
	if c.EntityA == (core.HEntity{}) {
		return fmt.Errorf("symmetric: missing symmetry plane")
	}
	planeEntity, err := s.planeExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("symmetric: %w", err)
	}
	pointA, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("symmetric: %w", err)
	}
	pointB, err := s.pointExpr(c.PtB)
	if err != nil {
		return fmt.Errorf("symmetric: %w", err)
	}
	mid := pointA.Plus(pointB).ScaledBy(core.NewConstant(0.5))
	if isFreeWorkplane(c.Workplane) {
		dist := pointPlaneDistanceExpr(mid, planeEntity)
		if err := s.registerParams(dist); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{
			H:    c.H.Equation(0),
			Expr: dist,
		})
		au, av := planeEntity.coords(pointA)
		bu, bv := planeEntity.coords(pointB)
		diffU := au.Minus(bu)
		diffV := av.Minus(bv)
		if err := s.registerParams(diffU); err != nil {
			return err
		}
		if err := s.registerParams(diffV); err != nil {
			return err
		}
		s.Equations = append(s.Equations,
			&Equation{H: c.H.Equation(1), Expr: diffU},
			&Equation{H: c.H.Equation(2), Expr: diffV},
		)
		return nil
	}
	workplane, err := s.planeExpr(c.Workplane)
	if err != nil {
		return fmt.Errorf("symmetric: %w", err)
	}
	mu, mv := workplane.coords(mid)
	pointOnPlane := workplane.pointFromCoords(mu, mv)
	dist := pointPlaneDistanceExprWithNormal(pointOnPlane, planeEntity.point, planeEntity.normal)
	if err := s.registerParams(dist); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: dist,
	})
	diff := pointA.Minus(pointB)
	tangential := planeEntity.normal.Cross(workplane.u.Cross(workplane.v)).Dot(diff)
	if err := s.registerParams(tangential); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(1),
		Expr: tangential,
	})
	return nil
}
func (s *System) addSymmetricHVConstraint(c *core.Constraint, horizontal bool) error {
	if isFreeWorkplane(c.Workplane) {
		return fmt.Errorf("symmetric hv: requires workplane")
	}
	workplane, err := s.planeExpr(c.Workplane)
	if err != nil {
		return fmt.Errorf("symmetric hv: %w", err)
	}
	pointA, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("symmetric hv: %w", err)
	}
	pointB, err := s.pointExpr(c.PtB)
	if err != nil {
		return fmt.Errorf("symmetric hv: %w", err)
	}
	au, av := workplane.coords(pointA)
	bu, bv := workplane.coords(pointB)
	var eq0, eq1 *core.Expr
	if horizontal {
		eq0 = av.Minus(bv)
		eq1 = au.Plus(bu)
	} else {
		eq0 = au.Minus(bu)
		eq1 = av.Plus(bv)
	}
	for i, expr := range []*core.Expr{eq0, eq1} {
		if err := s.registerParams(expr); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{H: c.H.Equation(i), Expr: expr})
	}
	return nil
}
func (s *System) addSymmetricLineConstraint(c *core.Constraint) error {
	if isFreeWorkplane(c.Workplane) {
		return fmt.Errorf("symmetric-line: requires workplane")
	}
	lineEnt, err := s.entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("symmetric-line: %w", err)
	}
	workplane, err := s.planeExpr(c.Workplane)
	if err != nil {
		return fmt.Errorf("symmetric-line: %w", err)
	}
	pa, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("symmetric-line: %w", err)
	}
	pb, err := s.pointExpr(c.PtB)
	if err != nil {
		return fmt.Errorf("symmetric-line: %w", err)
	}
	pau, pav := workplane.coords(pa)
	pbu, pbv := workplane.coords(pb)
	la, err := s.pointExpr(lineEnt.Point[0])
	if err != nil {
		return fmt.Errorf("symmetric-line: %w", err)
	}
	lb, err := s.pointExpr(lineEnt.Point[1])
	if err != nil {
		return fmt.Errorf("symmetric-line: %w", err)
	}
	lau, lav := workplane.coords(la)
	lbu, lbv := workplane.coords(lb)
	dpu := pbu.Minus(pau)
	dpv := pbv.Minus(pav)
	dlu := lbu.Minus(lau)
	dlv := lbv.Minus(lav)
	ortho := dlu.Times(dpu).Plus(dlv.Times(dpv))
	if err := s.registerParams(ortho); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{H: c.H.Equation(0), Expr: ortho})
	distA := dlv.Times(lau.Minus(pau)).Minus(dlu.Times(lav.Minus(pav)))
	distB := dlv.Times(lau.Minus(pbu)).Minus(dlu.Times(lav.Minus(pbv)))
	sum := distA.Plus(distB)
	if err := s.registerParams(sum); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{H: c.H.Equation(1), Expr: sum})
	return nil
}
func (s *System) addMidpointConstraint(c *core.Constraint) error {
	if c.EntityA == (core.HEntity{}) {
		return fmt.Errorf("midpoint: missing line entity")
	}
	lineEnt, err := s.entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("midpoint: %w", err)
	}
	start, err := s.pointExpr(lineEnt.Point[0])
	if err != nil {
		return fmt.Errorf("midpoint: %w", err)
	}
	end, err := s.pointExpr(lineEnt.Point[1])
	if err != nil {
		return fmt.Errorf("midpoint: %w", err)
	}
	mid := start.Plus(end.Minus(start).ScaledBy(core.NewConstant(0.5)))
	if isFreeWorkplane(c.Workplane) {
		if c.PtA != (core.HEntity{}) {
			target, err := s.pointExpr(c.PtA)
			if err != nil {
				return fmt.Errorf("midpoint: %w", err)
			}
			diff := mid.Minus(target)
			for i, component := range []*core.Expr{diff.X, diff.Y, diff.Z} {
				if err := s.registerParams(component); err != nil {
					return err
				}
				s.Equations = append(s.Equations, &Equation{
					H:    c.H.Equation(i),
					Expr: component,
				})
			}
			return nil
		}
		if c.EntityB == (core.HEntity{}) {
			return fmt.Errorf("midpoint: missing target plane")
		}
		plane, err := s.planeExpr(c.EntityB)
		if err != nil {
			return fmt.Errorf("midpoint: %w", err)
		}
		expr := pointPlaneDistanceExpr(mid, plane)
		if err := s.registerParams(expr); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{
			H:    c.H.Equation(0),
			Expr: expr,
		})
		return nil
	}
	workplane, err := s.planeExpr(c.Workplane)
	if err != nil {
		return fmt.Errorf("midpoint: %w", err)
	}
	mu, mv := workplane.coords(mid)
	if c.PtA != (core.HEntity{}) {
		target, err := s.pointExpr(c.PtA)
		if err != nil {
			return fmt.Errorf("midpoint: %w", err)
		}
		tu, tv := workplane.coords(target)
		diffU := tu.Minus(mu)
		diffV := tv.Minus(mv)
		for i, component := range []*core.Expr{diffU, diffV} {
			if err := s.registerParams(component); err != nil {
				return err
			}
			s.Equations = append(s.Equations, &Equation{
				H:    c.H.Equation(i),
				Expr: component,
			})
		}
		return nil
	}
	if c.EntityB == (core.HEntity{}) {
		return fmt.Errorf("midpoint: missing target plane")
	}
	targetPlane, err := s.planeExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("midpoint: %w", err)
	}
	pointOnPlane := workplane.pointFromCoords(mu, mv)
	expr := pointPlaneDistanceExprWithNormal(pointOnPlane, targetPlane.point, targetPlane.normal)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addEqualAngleConstraint(c *core.Constraint) error {
	va, err := s.vectorExpr(c.EntityA)
	if err != nil {
		return fmt.Errorf("equal-angle: %w", err)
	}
	vb, err := s.vectorExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("equal-angle: %w", err)
	}
	vc, err := s.vectorExpr(c.EntityC)
	if err != nil {
		return fmt.Errorf("equal-angle: %w", err)
	}
	vd, err := s.vectorExpr(c.EntityD)
	if err != nil {
		return fmt.Errorf("equal-angle: %w", err)
	}
	if c.Other {
		va = negateVector(va)
	}
	ua := va.WithMagnitude(core.NewConstant(1))
	ub := vb.WithMagnitude(core.NewConstant(1))
	uc := vc.WithMagnitude(core.NewConstant(1))
	ud := vd.WithMagnitude(core.NewConstant(1))
	expr := ua.Dot(ub).Minus(uc.Dot(ud))
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addArcLineTangentConstraint(c *core.Constraint) error {
	arc, err := s.entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("arc-line-tangent: %w", err)
	}
	if arc.Type != core.EntityArcOfCircle {
		return fmt.Errorf("arc-line-tangent: entity %08x type %d unsupported", c.EntityA.V, arc.Type)
	}
	line, err := s.lineExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("arc-line-tangent: %w", err)
	}
	center, err := s.pointExpr(arc.Point[0])
	if err != nil {
		return fmt.Errorf("arc-line-tangent: %w", err)
	}
	idx := 1
	if c.Other {
		idx = 2
	}
	endpoint, err := s.pointExpr(arc.Point[idx])
	if err != nil {
		return fmt.Errorf("arc-line-tangent: %w", err)
	}
	radius := center.Minus(endpoint)
	expr := line.direction.Dot(radius)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addCubicLineTangentConstraint(c *core.Constraint) error {
	cubic, err := s.entity(c.EntityA)
	if err != nil {
		return fmt.Errorf("cubic-line-tangent: %w", err)
	}
	if cubic.Type != core.EntityCubic && cubic.Type != core.EntityCubicPeriodic {
		return fmt.Errorf("cubic-line-tangent: entity %08x type %d unsupported", c.EntityA.V, cubic.Type)
	}
	line, err := s.lineExpr(c.EntityB)
	if err != nil {
		return fmt.Errorf("cubic-line-tangent: %w", err)
	}
	tangent, err := s.cubicTangentExpr(cubic, c.Other)
	if err != nil {
		return fmt.Errorf("cubic-line-tangent: %w", err)
	}
	if isFreeWorkplane(c.Workplane) {
		eq := vectorsParallel3D(tangent, line.direction, c.ValP)
		components := []*core.Expr{eq.X, eq.Y, eq.Z}
		for i, component := range components {
			if err := s.registerParams(component); err != nil {
				return err
			}
			s.Equations = append(s.Equations, &Equation{
				H:    c.H.Equation(i),
				Expr: component,
			})
		}
		return nil
	}
	plane, err := s.planeExpr(c.Workplane)
	if err != nil {
		return fmt.Errorf("cubic-line-tangent: %w", err)
	}
	expr := tangent.Cross(line.direction).Dot(plane.normal)
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addCurveCurveTangentConstraint(c *core.Constraint) error {
	dirs := [2]core.ExprVector{}
	parallel := true

	for i := 0; i < 2; i++ {
		handle := c.EntityA
		useFinish := c.Other
		if i == 1 {
			handle = c.EntityB
			useFinish = c.Other2
		}
		ent, err := s.entity(handle)
		if err != nil {
			return fmt.Errorf("curve-curve-tangent: %w", err)
		}
		switch ent.Type {
		case core.EntityArcOfCircle:
			center, err := s.pointExpr(ent.Point[0])
			if err != nil {
				return fmt.Errorf("curve-curve-tangent: %w", err)
			}
			idx := 1
			if useFinish {
				idx = 2
			}
			endpoint, err := s.pointExpr(ent.Point[idx])
			if err != nil {
				return fmt.Errorf("curve-curve-tangent: %w", err)
			}
			dirs[i] = endpoint.Minus(center)
			parallel = !parallel
		case core.EntityCubic, core.EntityCubicPeriodic:
			tangent, err := s.cubicTangentExpr(ent, useFinish)
			if err != nil {
				return fmt.Errorf("curve-curve-tangent: %w", err)
			}
			dirs[i] = tangent
		default:
			return fmt.Errorf("curve-curve-tangent: entity %08x type %d unsupported", handle.V, ent.Type)
		}
	}

	if parallel {
		if isFreeWorkplane(c.Workplane) {
			cross := dirs[0].Cross(dirs[1])
			for i, component := range []*core.Expr{cross.X, cross.Y, cross.Z} {
				if err := s.registerParams(component); err != nil {
					return err
				}
				s.Equations = append(s.Equations, &Equation{
					H:    c.H.Equation(i),
					Expr: component,
				})
			}
			return nil
		}
		plane, err := s.planeExpr(c.Workplane)
		if err != nil {
			return fmt.Errorf("curve-curve-tangent: %w", err)
		}
		expr := dirs[0].Cross(dirs[1]).Dot(plane.normal)
		if err := s.registerParams(expr); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{
			H:    c.H.Equation(0),
			Expr: expr,
		})
		return nil
	}

	expr := dirs[0].Dot(dirs[1])
	if err := s.registerParams(expr); err != nil {
		return err
	}
	s.Equations = append(s.Equations, &Equation{
		H:    c.H.Equation(0),
		Expr: expr,
	})
	return nil
}

func (s *System) addWhereDraggedConstraint(c *core.Constraint) error {
	if c.PtA == (core.HEntity{}) {
		return fmt.Errorf("where-dragged: missing point handle")
	}
	pointEnt, err := s.entity(c.PtA)
	if err != nil {
		return fmt.Errorf("where-dragged: %w", err)
	}
	pointExpr, err := s.pointExpr(c.PtA)
	if err != nil {
		return fmt.Errorf("where-dragged: %w", err)
	}
	if isFreeWorkplane(c.Workplane) {
		target := pointEnt.ActPoint
		for i, component := range []*core.Expr{
			pointExpr.X.Minus(core.NewConstant(target.X)),
			pointExpr.Y.Minus(core.NewConstant(target.Y)),
			pointExpr.Z.Minus(core.NewConstant(target.Z)),
		} {
			if err := s.registerParams(component); err != nil {
				return err
			}
			s.Equations = append(s.Equations, &Equation{
				H:    c.H.Equation(i),
				Expr: component,
			})
		}
		return nil
	}
	plane, err := s.planeExpr(c.Workplane)
	if err != nil {
		return fmt.Errorf("where-dragged: %w", err)
	}
	uExpr, vExpr := plane.coords(pointExpr)
	targetU, targetV := projectPointToPlane(pointEnt.ActPoint, plane)
	equations := []*core.Expr{
		uExpr.Minus(core.NewConstant(targetU)),
		vExpr.Minus(core.NewConstant(targetV)),
	}
	for i, expr := range equations {
		if err := s.registerParams(expr); err != nil {
			return err
		}
		s.Equations = append(s.Equations, &Equation{
			H:    c.H.Equation(i),
			Expr: expr,
		})
	}
	return nil
}

func (s *System) registerParams(expr *core.Expr) error {
	referenced := make(map[core.HParam]struct{})
	expr.ReferencedParams(referenced)

	for h := range referenced {
		if _, ok := s.Params[h]; ok {
			continue
		}
		param, err := s.resolver.Param(h)
		if err != nil {
			return fmt.Errorf("lookup param %08x: %w", h.V, err)
		}
		if param == nil {
			return fmt.Errorf("lookup param %08x: nil", h.V)
		}
		s.Params[h] = param
	}
	return nil
}

func (s *System) pointExpr(h core.HEntity) (core.ExprVector, error) {
	if h == (core.HEntity{}) {
		return core.ExprVector{}, fmt.Errorf("point handle zero")
	}
	ent, err := s.resolver.Entity(h)
	if err != nil {
		return core.ExprVector{}, err
	}
	if ent == nil {
		return core.ExprVector{}, fmt.Errorf("entity %08x not found", h.V)
	}

	switch ent.Type {
	case core.EntityPointIn3D:
		params := ent.Param
		comps := [3]*core.Expr{}
		values := []float64{ent.ActPoint.X, ent.ActPoint.Y, ent.ActPoint.Z}
		for i := 0; i < 3; i++ {
			if params[i].V != 0 {
				comps[i] = s.paramOrConstant(params[i], values[i])
			} else {
				comps[i] = core.NewConstant(values[i])
			}
		}
		return core.NewExprVector(comps[0], comps[1], comps[2]), nil
	case core.EntityPointIn2D:
		plane, err := s.planeExpr(ent.Workplane)
		if err != nil {
			return core.ExprVector{}, err
		}
		uDefault, vDefault := projectPointToPlane(ent.ActPoint, plane)
		uExpr := s.paramOrConstant(ent.Param[0], uDefault)
		vExpr := s.paramOrConstant(ent.Param[1], vDefault)
		vec := plane.point.Plus(plane.u.ScaledBy(uExpr)).Plus(plane.v.ScaledBy(vExpr))
		return vec, nil
	default:
		return core.NewExprVector(
			core.NewConstant(ent.ActPoint.X),
			core.NewConstant(ent.ActPoint.Y),
			core.NewConstant(ent.ActPoint.Z)), nil
	}
}

type planeData struct {
	point       core.ExprVector
	normal      core.ExprVector
	u           core.ExprVector
	v           core.ExprVector
	originValue geom.Vector
	uValue      geom.Vector
	vValue      geom.Vector
}

func (s *System) planeExpr(h core.HEntity) (planeData, error) {
	if h == (core.HEntity{}) {
		return planeData{}, fmt.Errorf("plane handle zero")
	}
	ent, err := s.resolver.Entity(h)
	if err != nil {
		return planeData{}, err
	}
	if ent == nil {
		return planeData{}, fmt.Errorf("plane entity %08x missing", h.V)
	}

	switch ent.Type {
	case core.EntityWorkplane:
		point, err := s.pointExpr(ent.Point[0])
		if err != nil {
			return planeData{}, err
		}
		qExpr, qNum, err := s.normalQuaternionExpr(ent.Normal)
		if err != nil {
			return planeData{}, err
		}
		normal := qExpr.RotationN()
		u := qExpr.RotationU()
		v := qExpr.RotationV()

		originEnt, err := s.resolver.Entity(ent.Point[0])
		if err != nil {
			return planeData{}, err
		}
		var originVal geom.Vector
		if originEnt != nil {
			originVal = originEnt.ActPoint
		}

		return planeData{
			point:       point,
			normal:      normal,
			u:           u,
			v:           v,
			originValue: originVal,
			uValue:      qNum.RotationU(),
			vValue:      qNum.RotationV(),
		}, nil
	default:
		numericNormal := ent.ActNormal.RotationN()
		uNum, vNum := orthonormalBasis(numericNormal)
		return planeData{
			point:       core.FromVector(ent.ActPoint),
			normal:      core.FromVector(numericNormal),
			u:           core.FromVector(uNum),
			v:           core.FromVector(vNum),
			originValue: ent.ActPoint,
			uValue:      uNum,
			vValue:      vNum,
		}, nil
	}
}

func (s *System) normalExpr(h core.HEntity) (core.ExprVector, error) {
	q, _, err := s.normalQuaternionExpr(h)
	if err != nil {
		return core.ExprVector{}, err
	}
	return q.RotationN(), nil
}

func (s *System) normalQuaternionExpr(h core.HEntity) (core.ExprQuaternion, geom.Quaternion, error) {
	if h == (core.HEntity{}) {
		return core.ExprQuaternion{}, geom.Quaternion{}, fmt.Errorf("normal handle zero")
	}
	ent, err := s.resolver.Entity(h)
	if err != nil {
		return core.ExprQuaternion{}, geom.Quaternion{}, err
	}
	if ent == nil {
		return core.ExprQuaternion{}, geom.Quaternion{}, fmt.Errorf("normal entity %08x missing", h.V)
	}

	qNumeric := ent.ActNormal.Normal()
	if qNumeric.W == 0 && qNumeric.X == 0 && qNumeric.Y == 0 && qNumeric.Z == 0 {
		qNumeric = geom.NewQuaternion(1, 0, 0, 0)
	}

	switch ent.Type {
	case core.EntityNormalIn3D:
		params := ent.Param
		if params[0].V != 0 || params[1].V != 0 || params[2].V != 0 || params[3].V != 0 {
			expr := core.NewExprQuaternion(
				core.NewParam(params[0]),
				core.NewParam(params[1]),
				core.NewParam(params[2]),
				core.NewParam(params[3]),
			)
			return expr, qNumeric, nil
		}
	}

	return core.NewExprQuaternionConst(qNumeric), qNumeric, nil
}

type lineData struct {
	a         core.ExprVector
	direction core.ExprVector
}

func (s *System) lineExpr(h core.HEntity) (lineData, error) {
	if h == (core.HEntity{}) {
		return lineData{}, fmt.Errorf("line handle zero")
	}
	ent, err := s.resolver.Entity(h)
	if err != nil {
		return lineData{}, err
	}
	if ent == nil {
		return lineData{}, fmt.Errorf("line entity %08x missing", h.V)
	}

	switch ent.Type {
	case core.EntityLineSegment:
		start, err := s.pointExpr(ent.Point[0])
		if err != nil {
			return lineData{}, err
		}
		finish, err := s.pointExpr(ent.Point[1])
		if err != nil {
			return lineData{}, err
		}
		return lineData{
			a:         start,
			direction: finish.Minus(start),
		}, nil
	default:
		return lineData{
			a:         core.FromVector(ent.ActPoint),
			direction: core.FromVector(ent.ActNormal.RotationU()),
		}, nil
	}
}

func (s *System) scalarValueExpr(c *core.Constraint) *core.Expr {
	if c.ValP != (core.HParam{}) {
		return core.NewParam(c.ValP)
	}
	return core.NewConstant(c.ValA)
}

type circleData struct {
	center      core.ExprVector
	plane       planeData
	radius      *core.Expr
	radiusValue float64
}

func (plane planeData) coords(p core.ExprVector) (*core.Expr, *core.Expr) {
	delta := p.Minus(plane.point)
	return delta.Dot(plane.u), delta.Dot(plane.v)
}

func (plane planeData) pointFromCoords(u, v *core.Expr) core.ExprVector {
	return plane.point.Plus(plane.u.ScaledBy(u)).Plus(plane.v.ScaledBy(v))
}
func projectPointToPlane(point geom.Vector, plane planeData) (float64, float64) {
	delta := point.Minus(plane.originValue)
	u := delta.Dot(plane.uValue)
	v := delta.Dot(plane.vValue)
	return u, v
}

func (s *System) paramOrConstant(h core.HParam, defaultVal float64) *core.Expr {
	if h != (core.HParam{}) {
		if param, err := s.resolver.Param(h); err == nil && param != nil {
			if param.Known && !param.Free {
				return core.NewConstant(param.Val)
			}
		}
		return core.NewParam(h)
	}
	return core.NewConstant(defaultVal)
}

func (s *System) circleData(ent *core.Entity) (circleData, error) {
	centerExpr, err := s.pointExpr(ent.Point[0])
	if err != nil {
		return circleData{}, err
	}
	centerEnt, err := s.resolver.Entity(ent.Point[0])
	if err != nil {
		return circleData{}, err
	}
	var centerVal geom.Vector
	if centerEnt != nil {
		centerVal = centerEnt.ActPoint
	}
	qExpr, qNum, err := s.normalQuaternionExpr(ent.Normal)
	if err != nil {
		return circleData{}, err
	}
	plane := planeData{
		point:       centerExpr,
		normal:      qExpr.RotationN(),
		u:           qExpr.RotationU(),
		v:           qExpr.RotationV(),
		originValue: centerVal,
		uValue:      qNum.RotationU(),
		vValue:      qNum.RotationV(),
	}
	radiusExpr, radiusVal, err := s.circleRadiusExpr(ent)
	if err != nil {
		return circleData{}, err
	}
	return circleData{center: centerExpr, plane: plane, radius: radiusExpr, radiusValue: radiusVal}, nil
}

func (s *System) circleRadiusExpr(ent *core.Entity) (*core.Expr, float64, error) {
	if ent.Distance != (core.HEntity{}) {
		expr, val, err := s.distanceExpr(ent.Distance)
		if err == nil {
			return expr, val, nil
		}
	}
	if ent.Point[1] != (core.HEntity{}) {
		center, err := s.pointExpr(ent.Point[0])
		if err != nil {
			return nil, 0, err
		}
		start, err := s.pointExpr(ent.Point[1])
		if err != nil {
			return nil, 0, err
		}
		diff := center.Minus(start)
		expr := diff.Magnitude()
		centerEnt, _ := s.resolver.Entity(ent.Point[0])
		startEnt, _ := s.resolver.Entity(ent.Point[1])
		var val float64
		if centerEnt != nil && startEnt != nil {
			val = centerEnt.ActPoint.DistanceTo(startEnt.ActPoint)
		}
		return expr, val, nil
	}
	return core.NewConstant(ent.ActDistance), ent.ActDistance, nil
}

func (s *System) distanceExpr(h core.HEntity) (*core.Expr, float64, error) {
	if h == (core.HEntity{}) {
		return nil, 0, fmt.Errorf("distance handle zero")
	}
	ent, err := s.resolver.Entity(h)
	if err != nil {
		return nil, 0, err
	}
	if ent == nil {
		return nil, 0, fmt.Errorf("distance entity %08x missing", h.V)
	}
	switch ent.Type {
	case core.EntityDistance:
		if ent.Param[0].V != 0 {
			return core.NewParam(ent.Param[0]), ent.ActDistance, nil
		}
		return core.NewConstant(ent.ActDistance), ent.ActDistance, nil
	default:
		return nil, 0, fmt.Errorf("unsupported distance entity type %d", ent.Type)
	}
}

func orthonormalBasis(n geom.Vector) (geom.Vector, geom.Vector) {
	normalized := n.Normal()
	if normalized.Magnitude() == 0 {
		normalized = geom.NewVector(0, 0, 1)
	}
	var u geom.Vector
	if math.Abs(normalized.X) > math.Abs(normalized.Z) {
		u = geom.NewVector(-normalized.Y, normalized.X, 0)
	} else {
		u = geom.NewVector(0, -normalized.Z, normalized.Y)
	}
	u = u.Normal()
	v := normalized.Cross(u)
	return u, v
}

func negateVector(v core.ExprVector) core.ExprVector {
	neg := core.NewConstant(-1)
	return core.ExprVector{
		X: v.X.Times(neg),
		Y: v.Y.Times(neg),
		Z: v.Z.Times(neg),
	}
}

type faceData struct {
	point  core.ExprVector
	normal core.ExprVector
}

func (s *System) faceExpr(h core.HEntity) (faceData, error) {
	if h == (core.HEntity{}) {
		return faceData{}, fmt.Errorf("face handle zero")
	}
	ent, err := s.resolver.Entity(h)
	if err != nil {
		return faceData{}, err
	}
	if ent == nil {
		return faceData{}, fmt.Errorf("face entity %08x missing", h.V)
	}

	var normalVec core.ExprVector
	switch ent.Type {
	case core.EntityFaceNormalPt:
		n := geom.NewVector(ent.ActNormal.X, ent.ActNormal.Y, ent.ActNormal.Z)
		if n.Magnitude() == 0 {
			n = geom.NewVector(0, 0, 1)
		}
		normalVec = core.FromVector(n.Normal())
	default:
		normalVec = core.FromVector(ent.ActNormal.RotationN())
	}
	point := core.FromVector(ent.ActPoint)
	return faceData{point: point, normal: normalVec}, nil
}

func (s *System) vectorExpr(h core.HEntity) (core.ExprVector, error) {
	if h == (core.HEntity{}) {
		return core.ExprVector{}, fmt.Errorf("vector handle zero")
	}
	ent, err := s.resolver.Entity(h)
	if err != nil {
		return core.ExprVector{}, err
	}
	if ent == nil {
		return core.ExprVector{}, fmt.Errorf("entity %08x missing", h.V)
	}

	switch ent.Type {
	case core.EntityLineSegment:
		start, err := s.pointExpr(ent.Point[0])
		if err != nil {
			return core.ExprVector{}, err
		}
		finish, err := s.pointExpr(ent.Point[1])
		if err != nil {
			return core.ExprVector{}, err
		}
		return finish.Minus(start), nil
	default:
		if ent.IsNormal() {
			return s.normalExpr(ent.H)
		}
		return core.FromVector(ent.ActNormal.RotationN()), nil
	}
}

func pointLineDistanceExpr(p core.ExprVector, line lineData, plane *planeData) *core.Expr {
	if plane == nil {
		vec := p.Minus(line.a)
		cross := vec.Cross(line.direction)
		numerator := cross.Magnitude()
		denom := line.direction.Magnitude()
		return numerator.Div(denom)
	}

	ua, va := plane.coords(line.a)
	end := line.a.Plus(line.direction)
	ub, vb := plane.coords(end)
	up, vp := plane.coords(p)

	du := ua.Minus(ub)
	dv := va.Minus(vb)
	denom := du.Square().Plus(dv.Square()).Sqrt()

	proj := dv.Times(ua.Minus(up)).Minus(du.Times(va.Minus(vp)))
	return proj.Div(denom)
}

func pointPlaneDistanceExpr(p core.ExprVector, plane planeData) *core.Expr {
	return pointPlaneDistanceExprWithNormal(p, plane.point, plane.normal)
}

func pointPlaneDistanceExprWithNormal(p, planePoint, planeNormal core.ExprVector) *core.Expr {
	norm := planeNormal.WithMagnitude(core.NewConstant(1))
	return p.Minus(planePoint).Dot(norm)
}

func isFreeWorkplane(h core.HEntity) bool {
	return h == (core.HEntity{}) || h == core.FreeIn3D
}
func vectorsParallel3D(a, b core.ExprVector, h core.HParam) core.ExprVector {
	scale := core.NewConstant(1)
	if h != (core.HParam{}) {
		scale = core.NewParam(h)
	}
	return a.Minus(b.ScaledBy(scale))
}

func (s *System) normalFrameExpr(h core.HEntity) (core.ExprVector, core.ExprVector, core.ExprVector, error) {
	qExpr, _, err := s.normalQuaternionExpr(h)
	if err != nil {
		return core.ExprVector{}, core.ExprVector{}, core.ExprVector{}, err
	}
	return qExpr.RotationU(), qExpr.RotationV(), qExpr.RotationN(), nil
}

func (s *System) normalFrameNumeric(h core.HEntity) (geom.Vector, geom.Vector, geom.Vector, error) {
	ent, err := s.resolver.Entity(h)
	if err != nil {
		return geom.Vector{}, geom.Vector{}, geom.Vector{}, err
	}
	if ent == nil {
		return geom.Vector{}, geom.Vector{}, geom.Vector{}, fmt.Errorf("normal entity %08x missing", h.V)
	}
	q := ent.ActNormal.Normal()
	return q.RotationU(), q.RotationV(), q.RotationN(), nil
}

func (s *System) arcAnglesNumeric(arc *core.Entity) (float64, float64, float64, error) {
	centerEnt, err := s.resolver.Entity(arc.Point[0])
	if err != nil || centerEnt == nil {
		return 0, 0, 0, fmt.Errorf("arc center entity missing")
	}
	startEnt, err := s.resolver.Entity(arc.Point[1])
	if err != nil || startEnt == nil {
		return 0, 0, 0, fmt.Errorf("arc start entity missing")
	}
	finishEnt, err := s.resolver.Entity(arc.Point[2])
	if err != nil || finishEnt == nil {
		return 0, 0, 0, fmt.Errorf("arc finish entity missing")
	}
	normalEnt, err := s.resolver.Entity(arc.Normal)
	if err != nil || normalEnt == nil {
		return 0, 0, 0, fmt.Errorf("arc normal entity missing")
	}
	center := centerEnt.ActPoint
	start := startEnt.ActPoint
	finish := finishEnt.ActPoint
	q := normalEnt.ActNormal.Normal()
	u := q.RotationU()
	v := q.RotationV()
	ps := start.Minus(center)
	pf := finish.Minus(center)
	ts := math.Atan2(ps.Dot(v), ps.Dot(u))
	tf := math.Atan2(pf.Dot(v), pf.Dot(u))
	dtheta := tf - ts
	for dtheta < 1e-6 {
		dtheta += 2 * math.Pi
	}
	for dtheta > 2*math.Pi {
		dtheta -= 2 * math.Pi
	}
	return ts, tf, dtheta, nil
}

func (s *System) arcLengthComponents(arc *core.Entity) (*core.Expr, *core.Expr, error) {
	center, err := s.pointExpr(arc.Point[0])
	if err != nil {
		return nil, nil, err
	}
	start, err := s.pointExpr(arc.Point[1])
	if err != nil {
		return nil, nil, err
	}
	finish, err := s.pointExpr(arc.Point[2])
	if err != nil {
		return nil, nil, err
	}
	normal, err := s.normalExpr(arc.Normal)
	if err != nil {
		return nil, nil, err
	}
	startDelta := start.Minus(center)
	finishDelta := finish.Minus(center)
	radius := finishDelta.Magnitude()
	u := startDelta.WithMagnitude(core.NewConstant(1))
	v := normal.Cross(u)
	cosTheta := finishDelta.Dot(u).Div(radius)
	sinTheta := finishDelta.Dot(v).Div(radius)
	_, _, dtheta, err := s.arcAnglesNumeric(arc)
	if err != nil {
		return nil, nil, err
	}
	var theta *core.Expr
	switch {
	case dtheta < 3*math.Pi/4:
		theta = cosTheta.ACos()
	case dtheta < 5*math.Pi/4:
		theta = core.NewConstant(math.Pi).Minus(sinTheta.ASin())
	default:
		theta = core.NewConstant(2 * math.Pi).Minus(cosTheta.ACos())
	}
	return radius, theta, nil
}

func (s *System) arcLengthExpr(arc *core.Entity) (*core.Expr, error) {
	radius, theta, err := s.arcLengthComponents(arc)
	if err != nil {
		return nil, err
	}
	return radius.Times(theta), nil
}

func (s *System) lineLengthExpr(ent *core.Entity) (*core.Expr, error) {
	data, err := s.lineExpr(ent.H)
	if err != nil {
		return nil, err
	}
	return data.direction.Magnitude(), nil
}

func (s *System) cubicTangentExpr(ent *core.Entity, finish bool) (core.ExprVector, error) {
	if ent.Type != core.EntityCubic && ent.Type != core.EntityCubicPeriodic {
		return core.ExprVector{}, fmt.Errorf("cubic tangent: unsupported entity type %d", ent.Type)
	}
	extra := ent.ExtraPoints
	if finish {
		tail, err := s.pointExpr(ent.Point[3+extra])
		if err != nil {
			return core.ExprVector{}, err
		}
		head, err := s.pointExpr(ent.Point[2+extra])
		if err != nil {
			return core.ExprVector{}, err
		}
		return tail.Minus(head), nil
	}
	head, err := s.pointExpr(ent.Point[0])
	if err != nil {
		return core.ExprVector{}, err
	}
	tail, err := s.pointExpr(ent.Point[1])
	if err != nil {
		return core.ExprVector{}, err
	}
	return head.Minus(tail), nil
}

func (s *System) arcTangentExpr(ent *core.Entity, useFinish bool) (core.ExprVector, error) {
	center, err := s.pointExpr(ent.Point[0])
	if err != nil {
		return core.ExprVector{}, err
	}
	idx := 1
	if useFinish {
		idx = 2
	}
	point, err := s.pointExpr(ent.Point[idx])
	if err != nil {
		return core.ExprVector{}, err
	}
	normal, err := s.normalExpr(ent.Normal)
	if err != nil {
		return core.ExprVector{}, err
	}
	radius := point.Minus(center)
	return normal.Cross(radius), nil
}

func (s *System) curveTangentExpr(h core.HEntity, useFinish bool) (core.ExprVector, error) {
	ent, err := s.entity(h)
	if err != nil {
		return core.ExprVector{}, err
	}
	switch ent.Type {
	case core.EntityArcOfCircle:
		return s.arcTangentExpr(ent, useFinish)
	case core.EntityCubic, core.EntityCubicPeriodic:
		return s.cubicTangentExpr(ent, useFinish)
	default:
		return core.ExprVector{}, fmt.Errorf("curve tangent: unsupported entity type %d", ent.Type)
	}
}

// ParamHandles returns the registered parameters in ascending handle order.
func (s *System) ParamHandles() []core.HParam {
	handles := make([]core.HParam, 0, len(s.Params))
	for h := range s.Params {
		handles = append(handles, h)
	}
	sort.Slice(handles, func(i, j int) bool {
		return handles[i].V < handles[j].V
	})
	return handles
}

// DOF returns a naive degree-of-freedom count (params - equations).
func (s *System) DOF() int {
	return len(s.Params) - len(s.Equations)
}

// Evaluate computes equation residuals and Jacobian numerically using the supplied overrides.
func (s *System) Evaluate(overrides map[core.HParam]float64) ([]float64, [][]float64, error) {
	if len(s.Equations) == 0 {
		return nil, nil, nil
	}

	ctx := evaluationContext{
		overrides: overrides,
		params:    s.Params,
	}

	paramOrder := s.ParamHandles()
	residuals := make([]float64, len(s.Equations))
	jacobian := make([][]float64, len(s.Equations))

	for row, eq := range s.Equations {
		residuals[row] = eq.Expr.Eval(ctx, nil)
		jacobian[row] = make([]float64, len(paramOrder))
		for col, h := range paramOrder {
			if _, ok := s.Params[h]; !ok {
				continue
			}
			d := eq.derivative(h)
			jacobian[row][col] = d.Eval(ctx, nil)
		}
	}

	return residuals, jacobian, nil
}

type evaluationContext struct {
	overrides map[core.HParam]float64
	params    map[core.HParam]*core.Param
}

func (e evaluationContext) ParamValue(h core.HParam) (float64, bool) {
	if v, ok := e.overrides[h]; ok {
		return v, true
	}
	if p, ok := e.params[h]; ok && p != nil {
		return p.Val, true
	}
	return 0, false
}

// AddExprEquation registers a symbolic equation (LHS - RHS = 0) for
// the solver to satisfy. Used by Group::GenerateEquations for group
// type-specific constraints like quaternion normalization, axis
// constraints, and extrusion orthogonality.
func (s *System) AddExprEquation(lhs, rhs *core.Expr) error {
	if lhs == nil || rhs == nil {
		return fmt.Errorf("solver: nil expr in AddExprEquation")
	}
	eq := lhs.Minus(rhs)
	s.Equations = append(s.Equations, &Equation{
		H:    core.HEquation{V: uint32(len(s.Equations) + 1)},
		Expr: eq,
	})
	return nil
}

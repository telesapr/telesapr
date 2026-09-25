package solver_test

import (
	"errors"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/solver"
)

// TestSystemReferenceConstraintAddConstraint verifies that reference
// constraints (read-only dimensions) work through AddConstraint.
// In the C++ model, reference constraints don't add a solving equation
// (the if(reference && !forReference) early-return in
// ConstraintBase::GenerateEquations). Our engine currently adds the
// equation regardless and re-evaluates via ModifyToSatisfy after solve.
func TestSystemReferenceConstraintAddConstraint(t *testing.T) {
	sys := solver.NewSystem(&stubResolver{params: map[core.HParam]*core.Param{}})
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:      core.ConstraintPtPtDistance,
		H:         core.HConstraint{V: 1},
		ValA:      5.0,
		Reference: true,
	}}
	// Just verify no panic.
	_ = sys.AddConstraint(c)
}

// TestSystemPointPlaneDistanceSignedMissingEntity verifies pt-plane
// requires a valid EntityA.
func TestSystemPointPlaneDistanceSignedMissingEntity(t *testing.T) {
	sys := solver.NewSystem(&stubResolver{params: map[core.HParam]*core.Param{}})
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type: core.ConstraintPtPlaneDistance,
		H:    core.HConstraint{V: 1},
		ValA: 3.0,
	}}
	err := sys.AddConstraint(c)
	if err == nil {
		t.Errorf("missing EntityA should error")
	}
}

// TestSystemArcLineTangentEmitterMissingEntities verifies arc-line-tangent
// requires both entities to resolve.
func TestSystemArcLineTangentEmitterMissingEntities(t *testing.T) {
	sys := solver.NewSystem(&stubResolver{params: map[core.HParam]*core.Param{}})
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintArcLineTangent,
		H:       core.HConstraint{V: 1},
		EntityA: core.HEntity{V: 1},
		EntityB: core.HEntity{V: 2},
	}}
	err := sys.AddConstraint(c)
	if err == nil {
		t.Errorf("missing entities should error")
	}
}

// TestSystemEqualRadiusEmitterMissingEntities verifies equal-radius
// requires both circle entities to resolve.
func TestSystemEqualRadiusEmitterMissingEntities(t *testing.T) {
	sys := solver.NewSystem(&stubResolver{params: map[core.HParam]*core.Param{}})
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintEqualRadius,
		H:       core.HConstraint{V: 1},
		EntityA: core.HEntity{V: 1},
		EntityB: core.HEntity{V: 2},
	}}
	err := sys.AddConstraint(c)
	if err == nil {
		t.Errorf("missing entities should error")
	}
}

// TestSystemDiameterEmitterMissingEntity verifies diameter requires EntityA.
func TestSystemDiameterEmitterMissingEntity(t *testing.T) {
	sys := solver.NewSystem(&stubResolver{params: map[core.HParam]*core.Param{}})
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintDiameter,
		H:       core.HConstraint{V: 1},
		EntityA: core.HEntity{V: 1},
	}}
	err := sys.AddConstraint(c)
	if err == nil {
		t.Errorf("missing entities should error")
	}
}

// TestSystemAllConstraintsHaveEmitter verifies every constraint type routes
// to a real addXxx method (not UnsupportedConstraintError).
func TestSystemAllConstraintsHaveEmitter(t *testing.T) {
	allTypes := []core.ConstraintType{
		core.ConstraintPointsCoincident,
		core.ConstraintPtPtDistance,
		core.ConstraintPtPlaneDistance,
		core.ConstraintPtLineDistance,
		core.ConstraintPtFaceDistance,
		core.ConstraintProjPtDistance,
		core.ConstraintPtInPlane,
		core.ConstraintPtOnLine,
		core.ConstraintPtOnFace,
		core.ConstraintEqualLengthLines,
		core.ConstraintLengthRatio,
		core.ConstraintEqLenPtLineD,
		core.ConstraintEqPtLnDistances,
		core.ConstraintEqualAngle,
		core.ConstraintEqualLineArcLen,
		core.ConstraintLengthDifference,
		core.ConstraintSymmetric,
		core.ConstraintSymmetricHoriz,
		core.ConstraintSymmetricVert,
		core.ConstraintSymmetricLine,
		core.ConstraintAtMidpoint,
		core.ConstraintHorizontal,
		core.ConstraintVertical,
		core.ConstraintDiameter,
		core.ConstraintPtOnCircle,
		core.ConstraintSameOrientation,
		core.ConstraintAngle,
		core.ConstraintParallel,
		core.ConstraintPerpendicular,
		core.ConstraintArcLineTangent,
		core.ConstraintCubicLineTangent,
		core.ConstraintCurveCurveTangent,
		core.ConstraintEqualRadius,
		core.ConstraintWhereDragged,
		core.ConstraintArcArcLenRatio,
		core.ConstraintArcLineLenRatio,
		core.ConstraintArcArcDifference,
		core.ConstraintArcLineDifference,
		core.ConstraintComment,
	}
	sys := solver.NewSystem(&stubResolver{params: map[core.HParam]*core.Param{}})
	for _, ct := range allTypes {
		c := &core.Constraint{ConstraintBase: core.ConstraintBase{
			Type: ct,
			H:    core.HConstraint{V: 1},
		}}
		err := sys.AddConstraint(c)
		// Unknown and Comment are valid no-ops; missing-entity errors are fine.
		if err != nil {
			var unsupported solver.UnsupportedConstraintError
			if errors.As(err, &unsupported) {
				t.Errorf("type %v should have an emitter, got UnsupportedConstraintError", ct)
			}
		}
	}
}

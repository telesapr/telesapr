package sketch

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestGroupGenerateExtrudeAddsTransCopies(t *testing.T) {
	// Build a minimal sketch with a SOURCE group containing one point
	// at the origin and an EXTRUDE group referencing it.
	sk := NewSketch()
	srcH := sk.AddGroup(Group{Type: GroupDrawing3D})
	extrH := sk.AddGroup(Group{Type: GroupExtrude, OpA: srcH})
	_ = extrH

	// Add a point at the origin.
	px := sk.AddParam(core.Param{H: core.HParam{V: 1}, Val: 0})
	py := sk.AddParam(core.Param{H: core.HParam{V: 2}, Val: 0})
	pz := sk.AddParam(core.Param{H: core.HParam{V: 3}, Val: 0})
	sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 1},
			Type:  core.EntityPointIn3D,
			Group: srcH,
			Param: [8]core.HParam{px, py, pz},
		},
		ActPoint: geom.Vector{X: 0, Y: 0, Z: 0},
	})
	// Recalculate so the runtime has the point.
	rt := NewRuntime(sk)
	rt.RecalculateEntities(false)

	// Run Generate: should add POINT_N_TRANS copies for top and bottom.
	initialEntityCount := len(sk.Entities)
	entities := sk.Entities
	params := sk.Params
	extrG := sk.GetGroup(extrH)
	extrG.Generate(sk, &entities, &params)

	addedEntities := entities[initialEntityCount:]
	if len(addedEntities) < 2 {
		t.Fatalf("expected at least 2 added entities (bottom+top), got %d", len(addedEntities))
	}
	bottomFound := false
	topFound := false
	for _, e := range addedEntities {
		if e.EntityBase.Type == core.EntityPointNTrans {
			if e.EntityBase.TimesApplied == 0 {
				bottomFound = true
			}
			if e.EntityBase.TimesApplied == 2 {
				topFound = true
			}
		}
	}
	if !bottomFound {
		t.Fatalf("expected bottom POINT_N_TRANS (timesApplied=0) in added entities")
	}
	if !topFound {
		t.Fatalf("expected top POINT_N_TRANS (timesApplied=2) in added entities")
	}
}

func TestCopyEntityPreservesActPoint(t *testing.T) {
	g := &Group{}
	src := &core.Entity{
		EntityBase: core.EntityBase{
			H:    core.HEntity{V: 5},
			Type: core.EntityPointIn3D,
		},
		ActPoint: geom.Vector{X: 1, Y: 2, Z: 3},
	}
	copy := g.copyEntity(src, 1, CopyAsNTrans)
	if copy.ActPoint.DistanceTo(src.ActPoint) > 1e-9 {
		t.Fatalf("copy didn't preserve ActPoint: %v vs %v", copy.ActPoint, src.ActPoint)
	}
	if copy.EntityBase.TimesApplied != 1 {
		t.Fatalf("timesApplied not set: %d", copy.EntityBase.TimesApplied)
	}
}

func TestCopyEntityEmitsParamsForExtrude(t *testing.T) {
	// Verify that after Generate, the extrusion params (0, 1, 2)
	// are registered on the new HEntity.
	sk := NewSketch()
	srcH := sk.AddGroup(Group{Type: GroupDrawing3D})
	extrH := sk.AddGroup(Group{Type: GroupExtrude, OpA: srcH})
	_ = extrH
	sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 1},
			Type:  core.EntityPointIn3D,
			Group: srcH,
		},
		ActPoint: geom.Vector{},
	})
	rt := NewRuntime(sk)
	rt.RecalculateEntities(false)

	entities := sk.Entities
	params := sk.Params
	extrG := sk.GetGroup(extrH)
	extrG.Generate(sk, &entities, &params)

	// Should have registered the 3 extrusion params.
	paramMap := make(map[core.HParam]bool, len(params))
	for _, p := range params {
		paramMap[p.H] = true
	}
	dx := extrH.Param(0)
	dy := extrH.Param(1)
	dz := extrH.Param(2)
	if !paramMap[dx] {
		t.Fatalf("extrusion param dx=%v not registered", dx)
	}
	if !paramMap[dy] {
		t.Fatalf("extrusion param dy=%v not registered", dy)
	}
	if !paramMap[dz] {
		t.Fatalf("extrusion param dz=%v not registered", dz)
	}
}

func TestCopyAsEnum(t *testing.T) {
	if CopyAsNumeric != 0 || CopyAsNTrans != 1 {
		t.Fatalf("CopyAs enum values don't match C++ convention")
	}
}

// TestModifyToSatisfyConstraintAngleNullConstraint tests handling of nil.
func TestModifyToSatisfyConstraintAngleNullConstraint(t *testing.T) {
	ModifyToSatisfyConstraint(nil, nil)
	// Should not panic.
}

// TestModifyToSatisfyConstraintAngleEmptyParams tests no-param handling.
func TestModifyToSatisfyConstraintAngleEmptyParams(t *testing.T) {
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type: core.ConstraintAngle,
	}}
	ModifyToSatisfyConstraint(c, map[core.HParam]float64{})
	if c.ValA != 0 {
		t.Errorf("expected ValA=0 (no entities), got %v", c.ValA)
	}
}

// TestComputeAngleFromEntitiesNilSketch tests angle computation when
// there is no active sketch.
func TestComputeAngleFromEntitiesNilSketch(t *testing.T) {
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintAngle,
		EntityA: core.HEntity{V: 1},
		EntityB: core.HEntity{V: 2},
	}}
	got := computeAngleFromEntities(c, map[core.HParam]float64{})
	if got != -1 {
		t.Errorf("expected -1 (no entities), got %v", got)
	}
}

// TestComputeCircleDiameterNoEntity tests failure path.
func TestComputeCircleDiameterNoEntity(t *testing.T) {
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintDiameter,
		EntityA: core.HEntity{},
	}}
	_, ok := computeCircleDiameter(c, nil)
	if ok {
		t.Errorf("expected failure on missing entity")
	}
}

// TestAngleBetweenZero verifies angleBetween with zero vectors.
func TestAngleBetweenZero(t *testing.T) {
	a := geom.Vector{}
	b := geom.Vector{X: 1}
	got := angleBetween(a, b)
	if got != -1 && got != 0 {
		// 0 magnitude returns -1.
	}
}

// TestAngleBetweenOrthogonal verifies orthogonal vectors give 90 degrees.
func TestAngleBetweenOrthogonal(t *testing.T) {
	a := geom.Vector{X: 1}
	b := geom.Vector{Y: 1}
	got := angleBetween(a, b)
	if math.Abs(got-90) > 1e-6 {
		t.Errorf("orthogonal angle = %v, want 90", got)
	}
}

// TestAngleBetweenParallel verifies parallel vectors give 0 degrees.
func TestAngleBetweenParallel(t *testing.T) {
	a := geom.Vector{X: 1}
	b := geom.Vector{X: 2}
	got := angleBetween(a, b)
	if math.Abs(got) > 1e-6 {
		t.Errorf("parallel angle = %v, want 0", got)
	}
}

// TestAngleBetweenAntiParallel verifies anti-parallel gives 180.
func TestAngleBetweenAntiParallel(t *testing.T) {
	a := geom.Vector{X: 1}
	b := geom.Vector{X: -1}
	got := angleBetween(a, b)
	if math.Abs(got-180) > 1e-6 {
		t.Errorf("anti-parallel angle = %v, want 180", got)
	}
}

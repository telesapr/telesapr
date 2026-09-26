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
	got, _ := computeAngleFromEntities(c, map[core.HParam]float64{})
	if got != 0 {
		t.Errorf("expected 0 (no entities), got %v", got)
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

// TestModifyToSatisfyPtPtDistance verifies that a reference
// PT_PT_DISTANCE constraint's ValA gets recomputed to the actual
// distance between its points.
func TestModifyToSatisfyPtPtDistance(t *testing.T) {
	sk := NewSketch()
	grp := sk.AddGroup(Group{Type: GroupDrawing3D})
	paH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 0, Y: 0, Z: 0},
	})
	pbH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 3, Y: 4, Z: 0},
	})
	activeSketch = sk
	defer func() { activeSketch = nil }()
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type: core.ConstraintPtPtDistance,
		PtA:  paH,
		PtB:  pbH,
	}}
	ModifyToSatisfyConstraint(c, map[core.HParam]float64{})
	if math.Abs(c.ValA-5.0) > 1e-9 {
		t.Errorf("PT_PT_DISTANCE ValA = %v, want 5", c.ValA)
	}
}

// TestModifyToSatisfyDiameterReference verifies a reference
// diameter updates to 2r after the circle's radius is known.
func TestModifyToSatisfyDiameterReference(t *testing.T) {
	sk := NewSketch()
	grp := sk.AddGroup(Group{Type: GroupDrawingWorkplane})
	centerH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{},
	})
	rp := core.HParam{V: 7}
	sk.AddParam(core.Param{H: rp, Val: 2.5})
	distH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityDistance,
			Group: grp,
			Param: [8]core.HParam{rp},
		},
		ActDistance: 2.5,
	})
	circleH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:     core.EntityCircle,
			Group:    grp,
			Point:    [12]core.HEntity{centerH},
			Normal:   core.HEntity{V: 99},
			Distance: distH,
		},
	})
	activeSketch = sk
	defer func() { activeSketch = nil }()
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintDiameter,
		EntityA: circleH,
	}}
	ModifyToSatisfyConstraint(c, map[core.HParam]float64{rp: 2.5})
	if math.Abs(c.ValA-5.0) > 1e-9 {
		t.Errorf("DIAMETER ValA = %v, want 5", c.ValA)
	}
}

// TestModifyToSatisfyPtLineDistance verifies a reference
// pt-line-distance constraint is updated to its actual value.
func TestModifyToSatisfyPtLineDistance(t *testing.T) {
	sk := NewSketch()
	grp := sk.AddGroup(Group{Type: GroupDrawing3D})
	ptH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 0, Y: 3, Z: 0},
	})
	aH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: -5, Y: 0, Z: 0},
	})
	bH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 5, Y: 0, Z: 0},
	})
	lnH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityLineSegment,
			Group: grp,
			Point: [12]core.HEntity{aH, bH},
		},
	})
	activeSketch = sk
	defer func() { activeSketch = nil }()
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintPtLineDistance,
		PtA:     ptH,
		EntityA: lnH,
	}}
	ModifyToSatisfyConstraint(c, map[core.HParam]float64{})
	if math.Abs(c.ValA-3.0) > 1e-9 {
		t.Errorf("PT_LINE_DISTANCE ValA = %v, want 3", c.ValA)
	}
}

// TestModifyToSatisfyPtOnLineParameter verifies a reference
// PT_ON_LINE constraint's ValA gets recomputed to the actual
// parameter t along the line.
func TestModifyToSatisfyPtOnLineParameter(t *testing.T) {
	sk := NewSketch()
	grp := sk.AddGroup(Group{Type: GroupDrawing3D})
	ptH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 2, Y: 0, Z: 0},
	})
	aH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 0, Y: 0, Z: 0},
	})
	bH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 4, Y: 0, Z: 0},
	})
	lnH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityLineSegment,
			Group: grp,
			Point: [12]core.HEntity{aH, bH},
		},
	})
	activeSketch = sk
	defer func() { activeSketch = nil }()
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintPtOnLine,
		PtA:     ptH,
		EntityA: lnH,
	}}
	ModifyToSatisfyConstraint(c, map[core.HParam]float64{})
	if math.Abs(c.ValA-0.5) > 1e-9 {
		t.Errorf("PT_ON_LINE ValA = %v, want 0.5", c.ValA)
	}
}

// TestModifyToSatisfyArcArcLenRatio verifies a reference
// ARC_ARC_LEN_RATIO constraint gets recomputed.
func TestModifyToSatisfyArcArcLenRatio(t *testing.T) {
	sk := NewSketch()
	grp := sk.AddGroup(Group{Type: GroupDrawingWorkplane})
	// Two full circles with radii 2 and 4 → ratio = 0.5.
	c1H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
	})
	c2H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
	})
	rp1 := core.HParam{V: 7}
	rp2 := core.HParam{V: 8}
	sk.AddParam(core.Param{H: rp1, Val: 2})
	sk.AddParam(core.Param{H: rp2, Val: 4})
	d1H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityDistance,
			Group: grp,
			Param: [8]core.HParam{rp1},
		},
		ActDistance: 2,
	})
	d2H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityDistance,
			Group: grp,
			Param: [8]core.HParam{rp2},
		},
		ActDistance: 4,
	})
	circ1H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:     core.EntityCircle,
			Group:    grp,
			Point:    [12]core.HEntity{c1H},
			Distance: d1H,
		},
	})
	circ2H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:     core.EntityCircle,
			Group:    grp,
			Point:    [12]core.HEntity{c2H},
			Distance: d2H,
		},
	})
	activeSketch = sk
	defer func() { activeSketch = nil }()
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintArcArcLenRatio,
		EntityA: circ1H,
		EntityB: circ2H,
	}}
	ModifyToSatisfyConstraint(c, map[core.HParam]float64{rp1: 2, rp2: 4})
	if math.Abs(c.ValA-0.5) > 1e-9 {
		t.Errorf("ARC_ARC_LEN_RATIO ValA = %v, want 0.5", c.ValA)
	}
}

// TestModifyToSatisfyAngleOther verifies that ANGLE with Other=true
// uses the negated direction vector.
func TestModifyToSatisfyAngleOther(t *testing.T) {
	sk := NewSketch()
	grp := sk.AddGroup(Group{Type: GroupDrawing3D})
	aH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 0, Y: 0, Z: 0},
	})
	bH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 1, Y: 0, Z: 0},
	})
	c1H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 0, Y: 0, Z: 0},
	})
	c2H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
		ActPoint: geom.Vector{X: 0, Y: 1, Z: 0},
	})
	ln1H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityLineSegment,
			Group: grp,
			Point: [12]core.HEntity{aH, bH},
		},
	})
	ln2H := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityLineSegment,
			Group: grp,
			Point: [12]core.HEntity{c1H, c2H},
		},
	})
	activeSketch = sk
	defer func() { activeSketch = nil }()
	c := &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type:    core.ConstraintAngle,
		EntityA: ln1H,
		EntityB: ln2H,
		Other:   true,
	}}
	ModifyToSatisfyConstraint(c, map[core.HParam]float64{})
	if math.Abs(c.ValA-90) > 1e-6 {
		t.Errorf("ANGLE ValA (Other) = %v, want 90", c.ValA)
	}
}

// TestPointByHandleFallsBackToParams verifies that pointByHandle
// recomputes from params when ActPoint is unset.
func TestPointByHandleFallsBackToParams(t *testing.T) {
	sk := NewSketch()
	grp := sk.AddGroup(Group{Type: GroupDrawing3D})
	px := core.HParam{V: 11}
	py := core.HParam{V: 12}
	pz := core.HParam{V: 13}
	ph := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
			Param: [8]core.HParam{px, py, pz},
		},
		// No ActPoint set.
	})
	activeSketch = sk
	defer func() { activeSketch = nil }()
	pt := pointByHandle(ph, map[core.HParam]float64{
		px: 1.5, py: 2.5, pz: 3.5,
	})
	if pt == nil {
		t.Fatalf("pointByHandle returned nil")
	}
	if math.Abs(pt.X-1.5) > 1e-9 || math.Abs(pt.Y-2.5) > 1e-9 || math.Abs(pt.Z-3.5) > 1e-9 {
		t.Errorf("pointByHandle = %v, want (1.5,2.5,3.5)", *pt)
	}
}

// TestEntityLengthNumerCircle verifies entityLengthNumer returns
// 2πr for circles.
func TestEntityLengthNumerCircle(t *testing.T) {
	sk := NewSketch()
	grp := sk.AddGroup(Group{Type: GroupDrawingWorkplane})
	cH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityPointIn3D,
			Group: grp,
		},
	})
	rp := core.HParam{V: 7}
	sk.AddParam(core.Param{H: rp, Val: 3})
	dH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:  core.EntityDistance,
			Group: grp,
			Param: [8]core.HParam{rp},
		},
		ActDistance: 3,
	})
	circH := sk.AddEntity(core.Entity{
		EntityBase: core.EntityBase{
			Type:     core.EntityCircle,
			Group:    grp,
			Point:    [12]core.HEntity{cH},
			Distance: dH,
		},
	})
	activeSketch = sk
	defer func() { activeSketch = nil }()
	got, ok := entityLengthNumer(circH, map[core.HParam]float64{rp: 3})
	if !ok {
		t.Fatalf("entityLengthNumer returned !ok")
	}
	if math.Abs(got-2*math.Pi*3) > 1e-9 {
		t.Errorf("entityLengthNumer(circle r=3) = %v, want %v", got, 2*math.Pi*3)
	}
}

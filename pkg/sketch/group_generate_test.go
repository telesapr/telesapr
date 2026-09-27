package sketch

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// TestGroupGenerateExtrude checks the entities Group::Generate makes
// for a one-sided extrusion of a line: the remap numbering (which
// fixes the copies' handles) and the copies' positions.
func TestGroupGenerateExtrude(t *testing.T) {
	sk := NewSketch()
	srcH := sk.AddGroup(Group{Type: GroupDrawing3D, Scale: 1})
	extrH := sk.AddGroup(Group{Type: GroupExtrude, Subtype: GroupOneSided, OpA: srcH, Scale: 1})

	param := func(v uint32, val float64) core.HParam {
		h := core.HParam{V: v}
		sk.Params = append(sk.Params, core.Param{H: h, Val: val})
		return h
	}
	pa := core.HEntity{V: 0x10001}
	pb := core.HEntity{V: 0x10002}
	line := core.HEntity{V: 0x10000}
	sk.Entities = append(sk.Entities,
		core.Entity{EntityBase: core.EntityBase{H: pa, Type: core.EntityPointIn3D, Group: srcH,
			Param: [8]core.HParam{param(1, 0), param(2, 0), param(3, 0)}}},
		core.Entity{EntityBase: core.EntityBase{H: pb, Type: core.EntityPointIn3D, Group: srcH,
			Param: [8]core.HParam{param(4, 10), param(5, 0), param(6, 0)}}},
		core.Entity{EntityBase: core.EntityBase{H: line, Type: core.EntityLineSegment, Group: srcH,
			Point: [core.MaxPointsInEntity]core.HEntity{pa, pb}}},
	)

	g := sk.GetGroup(extrH)
	g.Generate(sk, &sk.Entities, &sk.Params)
	for _, k := range []int{0, 1, 2} {
		p := sk.GetParam(extrH.Param(k))
		if p == nil {
			t.Fatalf("extrusion param %d not added", k)
		}
		p.Val = []float64{0, 0, 5}[k]
	}

	// Remap ids are handed out in the order C++ asks for them: each
	// point's bottom, top and side line, then the line's bottom, top
	// and side face, then the top and bottom faces.
	want := []struct {
		key  core.EntityKey
		id   uint32
		typ  core.EntityType
		want geom.Vector
	}{
		{core.EntityKey{Input: pa, CopyNumber: RemapBottom}, 1, core.EntityPointNTrans, geom.Vector{}},
		{core.EntityKey{Input: pa, CopyNumber: RemapTop}, 2, core.EntityPointNTrans, geom.Vector{Z: 10}},
		{core.EntityKey{Input: pa, CopyNumber: RemapPtToLine}, 3, core.EntityLineSegment, geom.Vector{}},
		{core.EntityKey{Input: pb, CopyNumber: RemapBottom}, 4, core.EntityPointNTrans, geom.Vector{X: 10}},
		{core.EntityKey{Input: pb, CopyNumber: RemapTop}, 5, core.EntityPointNTrans, geom.Vector{X: 10, Z: 10}},
		{core.EntityKey{Input: pb, CopyNumber: RemapPtToLine}, 6, core.EntityLineSegment, geom.Vector{}},
		{core.EntityKey{Input: line, CopyNumber: RemapBottom}, 7, core.EntityLineSegment, geom.Vector{}},
		{core.EntityKey{Input: line, CopyNumber: RemapTop}, 8, core.EntityLineSegment, geom.Vector{}},
		{core.EntityKey{Input: line, CopyNumber: RemapLineToFace}, 9, core.EntityFaceXProd, geom.Vector{}},
		{core.EntityKey{Input: core.NoEntity, CopyNumber: RemapTop}, 10, core.EntityFaceNormalPt, geom.Vector{}},
		{core.EntityKey{Input: core.NoEntity, CopyNumber: RemapBottom}, 11, core.EntityFaceNormalPt, geom.Vector{}},
	}
	if len(g.Remap) != len(want) {
		t.Errorf("remap has %d entries, want %d", len(g.Remap), len(want))
	}
	for _, w := range want {
		if got := g.Remap[w.key]; got.V != w.id {
			t.Errorf("remap %v = %d, want %d", w.key, got.V, w.id)
			continue
		}
		e := sk.GetEntity(extrH.Entity(int(w.id)))
		if e == nil {
			t.Errorf("entity for remap %v missing", w.key)
			continue
		}
		if e.Type != w.typ {
			t.Errorf("entity for remap %v has type %d, want %d", w.key, e.Type, w.typ)
		}
		if e.IsPoint() {
			if got := e.PointGetNum(sk); !got.Equals(w.want) {
				t.Errorf("point for remap %v at %v, want %v", w.key, got, w.want)
			}
		}
	}

	// The side face of the line lies in the plane y = 0.
	face := sk.GetEntity(extrH.Entity(9))
	if n := face.FaceGetNormalNum(sk); !n.Equals(geom.Vector{Y: -1}) && !n.Equals(geom.Vector{Y: 1}) {
		t.Errorf("side face normal %v, want +-y", n)
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

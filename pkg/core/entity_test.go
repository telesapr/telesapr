package core

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestEntityTypePredicates(t *testing.T) {
	point := &Entity{EntityBase: EntityBase{Type: EntityPointIn3D}}
	if !point.IsPoint() || point.IsNormal() || point.IsDistance() || point.IsFace() {
		t.Fatal("point entity predicate mismatch")
	}
	if point.IsWorkplane() || point.IsCircle() {
		t.Fatal("point should not be workplane or circle")
	}
	if point.HasVector() {
		t.Fatal("point should not report HasVector")
	}
	if point.HasEndpoints() {
		t.Fatal("point should not report HasEndpoints")
	}

	normal := &Entity{EntityBase: EntityBase{Type: EntityNormalIn3D}}
	if !normal.IsNormal() || !normal.HasVector() {
		t.Fatal("normal entity predicates failed")
	}

	line := &Entity{EntityBase: EntityBase{Type: EntityLineSegment}}
	if !line.HasVector() || !line.HasEndpoints() {
		t.Fatal("line segment should expose vector and endpoints")
	}

	face := &Entity{EntityBase: EntityBase{Type: EntityFaceNormalPt}}
	if !face.IsFace() {
		t.Fatal("face predicate failed")
	}

	workplane := &Entity{EntityBase: EntityBase{Type: EntityWorkplane}}
	if !workplane.IsWorkplane() || !workplane.HasVector() {
		t.Fatal("workplane predicate failed")
	}

	circle := &Entity{EntityBase: EntityBase{Type: EntityCircle}}
	if !circle.IsCircle() || circle.HasEndpoints() {
		t.Fatal("circle predicate failed")
	}
}

func TestEntityNumericAccessors(t *testing.T) {
	actPoint := geom.Vector{X: 1, Y: 2, Z: 3}
	actNormal := geom.Quaternion{W: 0.5, X: 0.5, Y: 0.5, Z: 0.5}

	point := &Entity{EntityBase: EntityBase{Type: EntityPointIn3D}, ActPoint: actPoint}
	if got := point.PointGetNum(); got != actPoint {
		t.Fatalf("PointGetNum: got %+v want %+v", got, actPoint)
	}
	if got := point.NormalGetNum(); got != (geom.Quaternion{W: 1}) {
		t.Fatalf("Point NormalGetNum default: got %+v", got)
	}

	normal := &Entity{EntityBase: EntityBase{Type: EntityNormalIn3D}, ActNormal: actNormal}
	if got := normal.NormalGetNum(); got != actNormal {
		t.Fatalf("NormalGetNum: got %+v want %+v", got, actNormal)
	}

	distance := &Entity{EntityBase: EntityBase{Type: EntityDistance}, ActDistance: 42}
	if got := distance.DistanceGetNum(); got != 42 {
		t.Fatalf("DistanceGetNum: got %f want 42", got)
	}
	if got := circleDistance().DistanceGetNum(); got != 0 {
		t.Fatalf("non-distance DistanceGetNum: got %f want 0", got)
	}

	nonPoint := &Entity{EntityBase: EntityBase{Type: EntityCircle}, ActPoint: actPoint}
	if got := nonPoint.PointGetNum(); got != (geom.Vector{}) {
		t.Fatalf("non-point PointGetNum should be zero: got %+v", got)
	}
}

func circleDistance() *Entity {
	return &Entity{EntityBase: EntityBase{Type: EntityCircle}, ActDistance: 99}
}

func TestEntityListAdd(t *testing.T) {
	var list EntityList
	e := Entity{EntityBase: EntityBase{Type: EntityPointIn3D}}
	list.Add(e)
	if len(list) != 1 || list[0].Type != EntityPointIn3D {
		t.Fatalf("EntityList.Add failed: len=%d entry=%+v", len(list), list[0])
	}
}

// TestEntityEmptyEntity verifies the zero-value Entity.
func TestEntityEmptyEntity(t *testing.T) {
	var e Entity
	if e.Type != 0 {
		t.Errorf("zero Entity should have Type=0")
	}
	if e.Group.V != 0 {
		t.Errorf("zero Entity should have Group.V=0")
	}
}

// TestEntityPointIsPoint verifies the IsPoint check.
func TestEntityPointIsPoint(t *testing.T) {
	tests := []struct {
		t    EntityType
		want bool
	}{
		{EntityPointIn3D, true},
		{EntityPointIn2D, true},
		{EntityPointNCopy, true},
		{EntityPointNTrans, true},
		{EntityLineSegment, false},
		{EntityWorkplane, false},
	}
	for _, tc := range tests {
		e := Entity{EntityBase: EntityBase{Type: tc.t}}
		if got := e.IsPoint(); got != tc.want {
			t.Errorf("IsPoint for %v: got %v, want %v", tc.t, got, tc.want)
		}
	}
}

// TestEntityNormalIsNormal verifies the IsNormal check.
func TestEntityNormalIsNormal(t *testing.T) {
	e := Entity{EntityBase: EntityBase{Type: EntityNormalIn3D}}
	if !e.IsNormal() {
		t.Errorf("EntityNormalIn3D should be a normal")
	}
	e.Type = EntityPointIn3D
	if e.IsNormal() {
		t.Errorf("EntityPointIn3D should not be a normal")
	}
}

// TestEntityIsWorkplane verifies the IsWorkplane check.
func TestEntityIsWorkplane(t *testing.T) {
	e := Entity{EntityBase: EntityBase{Type: EntityWorkplane}}
	if !e.IsWorkplane() {
		t.Errorf("EntityWorkplane should be a workplane")
	}
	e.Type = EntityPointIn3D
	if e.IsWorkplane() {
		t.Errorf("EntityPointIn3D should not be a workplane")
	}
}

// TestEntityIsFace verifies the IsFace check.
func TestEntityIsFace(t *testing.T) {
	e := Entity{EntityBase: EntityBase{Type: EntityFaceNormalPt}}
	if !e.IsFace() {
		t.Errorf("EntityFaceNormalPt should be a face")
	}
	e.Type = EntityPointIn3D
	if e.IsFace() {
		t.Errorf("EntityPointIn3D should not be a face")
	}
}

// TestEntityHasEndpoints verifies the HasEndpoints check.
func TestEntityHasEndpoints(t *testing.T) {
	e := Entity{EntityBase: EntityBase{Type: EntityLineSegment}}
	if !e.HasEndpoints() {
		t.Errorf("EntityLineSegment should have endpoints")
	}
	e.Type = EntityPointIn3D
	if e.HasEndpoints() {
		t.Errorf("EntityPointIn3D should not have endpoints")
	}
}

// TestEntityIsCircle verifies the IsCircle check.
func TestEntityIsCircle(t *testing.T) {
	e := Entity{EntityBase: EntityBase{Type: EntityCircle}}
	if !e.IsCircle() {
		t.Errorf("EntityCircle should be a circle")
	}
	e.Type = EntityPointIn3D
	if e.IsCircle() {
		t.Errorf("EntityPointIn3D should not be a circle")
	}
}

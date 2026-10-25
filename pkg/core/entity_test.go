package core

import (
	"testing"
)

func TestEntityTypePredicates(t *testing.T) {
	t.Parallel()
	point := &Entity{Type: EntityPointIn3D}
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

	normal := &Entity{Type: EntityNormalIn3D}
	if !normal.IsNormal() || !normal.HasVector() {
		t.Fatal("normal entity predicates failed")
	}

	line := &Entity{Type: EntityLineSegment}
	if !line.HasVector() || !line.HasEndpoints() {
		t.Fatal("line segment should expose vector and endpoints")
	}

	face := &Entity{Type: EntityFaceNormalPt}
	if !face.IsFace() {
		t.Fatal("face predicate failed")
	}

	workplane := &Entity{Type: EntityWorkplane}
	if !workplane.IsWorkplane() || workplane.HasVector() {
		t.Fatal("workplane predicate failed")
	}

	circle := &Entity{Type: EntityCircle}
	if !circle.IsCircle() || circle.HasEndpoints() {
		t.Fatal("circle predicate failed")
	}
}

func TestEntityListAdd(t *testing.T) {
	t.Parallel()
	var list EntityList
	e := Entity{Type: EntityPointIn3D}
	list.Add(e)
	if len(list) != 1 || list[0].Type != EntityPointIn3D {
		t.Fatalf("EntityList.Add failed: len=%d entry=%+v", len(list), list[0])
	}
}

// TestEntityEmptyEntity verifies the zero-value Entity.
func TestEntityEmptyEntity(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
		e := Entity{Type: tc.t}
		if got := e.IsPoint(); got != tc.want {
			t.Errorf("IsPoint for %v: got %v, want %v", tc.t, got, tc.want)
		}
	}
}

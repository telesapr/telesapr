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

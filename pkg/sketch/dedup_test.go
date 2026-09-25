package sketch

import (
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestDedupEntitiesByHandleKeepsNonZeroActPoint(t *testing.T) {
	e1 := core.Entity{
		EntityBase: core.EntityBase{H: core.HEntity{V: 0x1234}, Type: core.EntityPointIn3D},
		ActPoint:   geom.NewVector(1, 2, 3),
	}
	e2 := core.Entity{
		EntityBase: core.EntityBase{H: core.HEntity{V: 0x1234}, Type: core.EntityPointIn3D},
		ActPoint:   geom.NewVector(0, 0, 0),
	}
	out := dedupEntitiesByHandle([]core.Entity{e1, e2})
	if len(out) != 1 {
		t.Fatalf("dedup produced %d entries, want 1", len(out))
	}
	if !out[0].ActPoint.Equals(geom.NewVector(1, 2, 3)) {
		t.Errorf("dedup kept the wrong entry: %v want (1,2,3)", out[0].ActPoint)
	}
}

func TestDedupEntitiesByHandleDifferentHandles(t *testing.T) {
	e1 := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x1}}}
	e2 := core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: 0x2}}}
	out := dedupEntitiesByHandle([]core.Entity{e1, e2})
	if len(out) != 2 {
		t.Fatalf("dedup produced %d entries, want 2", len(out))
	}
}

func TestDedupParamsByHandleKeepsNonZeroValue(t *testing.T) {
	p1 := core.Param{H: core.HParam{V: 0x1234}, Val: 5, Known: true}
	p2 := core.Param{H: core.HParam{V: 0x1234}, Val: 0, Known: true}
	out := dedupParamsByHandle([]core.Param{p1, p2})
	if len(out) != 1 {
		t.Fatalf("dedup produced %d entries, want 1", len(out))
	}
	if out[0].Val != 5 {
		t.Errorf("dedup kept the wrong entry: Val=%v want 5", out[0].Val)
	}
}

func TestDedupParamsByHandlePrefersKnown(t *testing.T) {
	p1 := core.Param{H: core.HParam{V: 0x1234}, Val: 0, Known: true}
	p2 := core.Param{H: core.HParam{V: 0x1234}, Val: 0}
	out := dedupParamsByHandle([]core.Param{p1, p2})
	if len(out) != 1 {
		t.Fatalf("dedup produced %d entries, want 1", len(out))
	}
	if !out[0].Known {
		t.Errorf("dedup kept the unKnown entry: Known=%v want true", out[0].Known)
	}
}

package sketch

import (
	"reflect"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// The order shape a drag reads at every frame is built once and kept
// while only the params' values change, and built again, the same as a
// shape built afresh, when a constraint is added or the points moved
// change.
func TestOrderShapeKeptWhileUnchanged(t *testing.T) {
	t.Parallel()
	sk, hg, hrs := linesSketch(t, 2)
	l0, l1 := hrs[0], hrs[1]
	dist := func(a, b core.HEntity, v float64) {
		sk.AddConstraint(core.Constraint{Type: core.ConstraintPtPtDistance, Group: hg,
			Workplane: hg.Entity(0), PtA: a, PtB: b, ValA: v})
	}
	sk.AddConstraint(core.Constraint{Type: core.ConstraintPointsCoincident, Group: hg,
		Workplane: hg.Entity(0), PtA: l0.Entity(2), PtB: l1.Entity(1)})
	dist(l0.Entity(1), l0.Entity(2), 6)
	dist(l1.Entity(1), l1.Entity(2), 7)
	sk.GenerateAll()
	hgs := []core.HGroup{hg}
	params := sk.groupParams(hg)
	fresh := func(moved []core.HEntity) *orderShape {
		return sk.buildOrderShape(sk.orderInputsOf(hgs), params, moved)
	}

	first := sk.orderShapeOf(hgs, params, nil)
	if len(first.corners) != 1 {
		t.Fatalf("%d corners, want the one where the lines meet", len(first.corners))
	}
	sk.MustEntity(l1.Entity(2)).PointForceTo(sk, geom.Vector{X: 3, Y: 9})
	if again := sk.orderShapeOf(hgs, params, nil); again != first {
		t.Error("the shape was built again for a point moved")
	}

	moved := []core.HEntity{l1.Entity(2)}
	byMove := sk.orderShapeOf(hgs, params, moved)
	if byMove == first || !reflect.DeepEqual(byMove, fresh(moved)) {
		t.Error("the shape for another point dragged is not the one built afresh")
	}

	dist(l0.Entity(1), l1.Entity(2), 9)
	sk.GenerateAll()
	params = sk.groupParams(hg)
	byEdit := sk.orderShapeOf(hgs, params, moved)
	if byEdit == byMove || !reflect.DeepEqual(byEdit, fresh(moved)) {
		t.Error("the shape after a constraint added is not the one built afresh")
	}
	if len(byEdit.corners) <= len(byMove.corners) {
		t.Errorf("%d corners after a third length, want more than %d", len(byEdit.corners), len(byMove.corners))
	}
}

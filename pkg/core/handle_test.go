package core

import "testing"

func TestHGroupDerivedHandles(t *testing.T) {
	group := HGroup{V: 0x1234}

	entity := group.Entity(5)
	wantEntity := uint32(0x80000000) | (group.V << 16) | uint32(5)
	if entity.V != wantEntity {
		t.Fatalf("group.Entity: got %#x want %#x", entity.V, wantEntity)
	}
	if entity.Group() != group {
		t.Fatalf("Group: got %#v want %#v", entity.Group(), group)
	}
	if entity.IsFromRequest() {
		t.Fatal("group-backed entity should not report as request entity")
	}

	param := group.Param(7)
	wantParam := uint32(0x80000000) | (group.V << 16) | uint32(7)
	if param.V != wantParam {
		t.Fatalf("group.Param: got %#x want %#x", param.V, wantParam)
	}

	eq := group.Equation(9)
	wantEquation := uint32(0x80000000) | (group.V << 16) | uint32(9)
	if eq.V != wantEquation {
		t.Fatalf("group.Equation: got %#x want %#x", eq.V, wantEquation)
	}
	if eq.IsFromConstraint() {
		t.Fatal("a group's equation isn't from a constraint")
	}
}

func TestHRequestDerivedHandles(t *testing.T) {
	req := HRequest{V: 0x55}

	entity := req.Entity(3)
	wantEntity := (req.V << 16) | uint32(3)
	if entity.V != wantEntity {
		t.Fatalf("request.Entity: got %#x want %#x", entity.V, wantEntity)
	}
	if !entity.IsFromRequest() {
		t.Fatal("request-backed entity should report as request entity")
	}
	if entity.Request() != req {
		t.Fatalf("Request handle mismatch: got %#v want %#v", entity.Request(), req)
	}

	param := req.Param(11)
	wantParam := (req.V << 16) | uint32(11)
	if param.V != wantParam {
		t.Fatalf("request.Param: got %#x want %#x", param.V, wantParam)
	}

	if !(HRequest{V: 1}).IsFromReferences() {
		t.Fatal("IsFromReferences should be true when V == 1")
	}
	if (HRequest{V: 2}).IsFromReferences() {
		t.Fatal("IsFromReferences should be false when V != 1")
	}
}

func TestHConstraintHandles(t *testing.T) {
	constraint := HConstraint{V: 0x2345}

	param := constraint.Param(4)
	wantParam := constraint.V | 0x40000000 | uint32(4)
	if param.V != wantParam {
		t.Fatalf("constraint.Param: got %#x want %#x", param.V, wantParam)
	}

	eq := constraint.Equation(6)
	wantEquation := (constraint.V << 16) | uint32(6)
	if eq.V != wantEquation {
		t.Fatalf("constraint.Equation: got %#x want %#x", eq.V, wantEquation)
	}
	if !eq.IsFromConstraint() || eq.Constraint() != constraint {
		t.Fatalf("Constraint() = %#x, IsFromConstraint %v; want %#x, true", eq.Constraint().V, eq.IsFromConstraint(), constraint.V)
	}
	if (eq.V >> 16) != constraint.V {
		t.Fatalf("encoded constraint handle lost: got %#x want %#x", eq.V>>16, constraint.V)
	}
}

func TestHParamRequest(t *testing.T) {
	h := HParam{V: 0xABCD1234}
	if got := h.Request(); got.V != (h.V >> 16) {
		t.Fatalf("Request handle: got %#x want %#x", got.V, h.V>>16)
	}
}

// TestHandleEquality verifies Handle equality.
func TestHandleEquality(t *testing.T) {
	h1 := HEntity{V: 0x12345678}
	h2 := HEntity{V: 0x12345678}
	h3 := HEntity{V: 0x87654321}
	if h1 != h2 {
		t.Errorf("equal handles should compare equal")
	}
	if h1 == h3 {
		t.Errorf("different handles should not compare equal")
	}
}

// TestHandleZeroIsZeroValue verifies the zero Handle.
func TestHandleZeroIsZeroValue(t *testing.T) {
	var h HEntity
	if h.V != 0 {
		t.Errorf("zero HEntity should have V=0, got %d", h.V)
	}
}

// TestHandleGroupEquality verifies HGroup equality.
func TestHandleGroupEquality(t *testing.T) {
	g1 := HGroup{V: 42}
	g2 := HGroup{V: 42}
	g3 := HGroup{V: 43}
	if g1 != g2 {
		t.Errorf("equal groups should compare equal")
	}
	if g1 == g3 {
		t.Errorf("different groups should compare equal")
	}
}

// TestHandleConstraintEquality verifies HConstraint equality.
func TestHandleConstraintEquality(t *testing.T) {
	c1 := HConstraint{V: 0x80000001}
	c2 := HConstraint{V: 0x80000001}
	c3 := HConstraint{V: 0x80000002}
	if c1 != c2 {
		t.Errorf("equal constraints should compare equal")
	}
	if c1 == c3 {
		t.Errorf("different constraints should compare equal")
	}
}

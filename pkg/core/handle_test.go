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
	if eq.Constraint().V != (eq.V & 0x7FFFFFFF) {
		t.Fatalf("Constraint masking failed: got %#x want %#x", eq.Constraint().V, eq.V&0x7FFFFFFF)
	}
	if !eq.IsFromConstraint() {
		t.Fatal("expected high-bit equation to report as constraint-derived")
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
	if eq.Constraint().V != (eq.V & 0x7FFFFFFF) {
		t.Fatalf("Constraint().V mismatch: got %#x want %#x", eq.Constraint().V, eq.V&0x7FFFFFFF)
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

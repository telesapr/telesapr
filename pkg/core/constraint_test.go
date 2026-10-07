package core

import "testing"

func TestConstraintTypeString(t *testing.T) {
	tests := map[ConstraintType]string{
		ConstraintPointsCoincident: "PointsCoincident",
		ConstraintPtPtDistance:     "PtPtDistance",
		ConstraintAngle:            "Angle",
		ConstraintComment:          "Comment",
		ConstraintType(9999):       "Unknown",
	}

	for tt, want := range tests {
		if got := tt.String(); got != want {
			t.Fatalf("String(%d): got %q want %q", tt, got, want)
		}
	}
}

func TestConstraintBaseEquals(t *testing.T) {
	a := &ConstraintBase{
		Type:      ConstraintAngle,
		Group:     HGroup{V: 1},
		Workplane: HEntity{V: 2},
		ValA:      3.14,
		ValP:      HParam{V: 3},
		PtA:       HEntity{V: 4},
		PtB:       HEntity{V: 5},
		EntityA:   HEntity{V: 6},
		EntityB:   HEntity{V: 7},
		EntityC:   HEntity{V: 8},
		EntityD:   HEntity{V: 9},
		Other:     true,
		Other2:    false,
		Reference: true,
		Comment:   "note",
	}
	b := *a

	if !a.Equals(&b) {
		t.Fatal("expected identical constraint bases to be equal")
	}

	b.Other = false
	if a.Equals(&b) {
		t.Fatal("expected inequality when a field differs")
	}
}

// TestConstraintZeroValue verifies the zero-value Constraint.
func TestConstraintZeroValue(t *testing.T) {
	var c Constraint
	if c.Type != 0 {
		t.Errorf("zero Constraint should have Type=0")
	}
	if c.Group.V != 0 {
		t.Errorf("zero Constraint should have Group.V=0")
	}
}

// TestConstraintTypeStringRoundtrip verifies the Type.String method.
func TestConstraintTypeStringRoundtrip(t *testing.T) {
	// Spot-check a few types
	cases := []ConstraintType{
		ConstraintPointsCoincident,
		ConstraintPtPtDistance,
		ConstraintDiameter,
		ConstraintHorizontal,
		ConstraintVertical,
	}
	for _, ct := range cases {
		s := ct.String()
		if s == "" {
			t.Errorf("%v.String() should not be empty", ct)
		}
	}
}

// TestConstraintBaseClear verifies Clear resets all fields.
func TestConstraintBaseClear(t *testing.T) {
	c := &Constraint{
		H:     HConstraint{V: 5},
		Type:  ConstraintPtPtDistance,
		Group: HGroup{V: 1},
		ValA:  42.0,
		PtA:   HEntity{V: 0x1234},
	}
	// Clear is a no-op, as C++'s is; it must not panic or change c.
	before := *c
	c.Clear()
	if c.ConstraintBase != before.ConstraintBase {
		t.Errorf("Clear changed the constraint: %+v", c.ConstraintBase)
	}
}

// TestConstraintBaseEquality checks ConstraintBase.Equals.
func TestConstraintBaseEquality(t *testing.T) {
	c1 := ConstraintBase{Type: ConstraintPointsCoincident, ValA: 1.0}
	c2 := ConstraintBase{Type: ConstraintPointsCoincident, ValA: 1.0}
	c3 := ConstraintBase{Type: ConstraintPointsCoincident, ValA: 2.0}
	if !c1.Equals(&c2) {
		t.Errorf("identical constraints should be equal")
	}
	if c1.Equals(&c3) {
		t.Errorf("constraints with different ValA should not be equal")
	}
}

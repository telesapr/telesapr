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

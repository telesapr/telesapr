package geom

import (
	"math"
	"testing"
)

func TestClosestPointBetweenLinesParallel(t *testing.T) {
	t.Parallel()
	a0 := Vector{0, 0, 0}
	da := Vector{1, 0, 0}
	b0 := Vector{0, 0, 1}
	db := Vector{1, 0, 0}
	ta, tb := ClosestPointBetweenLines(a0, da, b0, db)
	// Parallel lines: both parameters should be 0.
	if math.Abs(ta) > 1e-9 || math.Abs(tb) > 1e-9 {
		t.Errorf("parallel lines: got ta=%v tb=%v, want 0/0", ta, tb)
	}
}

func TestClosestPointBetweenLinesPerpendicular(t *testing.T) {
	t.Parallel()
	// Line A goes from (0,0,0) along +X; Line B goes from (5,5,0) along +Y.
	// Closest points are (5,0,0) on A (ta=5) and (5,0,0) on B (tb=-5).
	// Going backward on B by 5 from (5,5,0) along (0,1,0) reaches (5,0,0).
	a0 := Vector{0, 0, 0}
	da := Vector{1, 0, 0}
	b0 := Vector{5, 5, 0}
	db := Vector{0, 1, 0}
	ta, tb := ClosestPointBetweenLines(a0, da, b0, db)
	if math.Abs(ta-5) > 1e-9 {
		t.Errorf("ta: got %v want 5", ta)
	}
	if math.Abs(tb-(-5)) > 1e-9 {
		t.Errorf("tb: got %v want -5", tb)
	}
}

func TestClosestPointBetweenLinesSkew(t *testing.T) {
	t.Parallel()
	// Line A from (0,0,0) along X; Line B from (3,0,3) along Y.
	// Closest point on A is (3,0,0) at ta=3; on B is (3,0,3) at tb=0.
	a0 := Vector{0, 0, 0}
	da := Vector{1, 0, 0}
	b0 := Vector{3, 0, 3}
	db := Vector{0, 1, 0}
	ta, tb := ClosestPointBetweenLines(a0, da, b0, db)
	if math.Abs(ta-3) > 1e-9 {
		t.Errorf("ta: got %v want 3", ta)
	}
	if math.Abs(tb) > 1e-9 {
		t.Errorf("tb: got %v want 0", tb)
	}
}

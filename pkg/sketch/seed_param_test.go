package sketch

import "testing"

// TestSeedParam gives a param that a group has not made yet its value,
// which the regeneration then starts from, and sets one that is there.
func TestSeedParam(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	sk.SeedParam(HRequestReferenceXY.Param(0), 3)
	if p := sk.Param(HRequestReferenceXY.Param(0)); p == nil || p.Val != 3 {
		t.Fatalf("added %v", p)
	}
	sk.SeedParam(HRequestReferenceXY.Param(0), 4)
	if p := sk.Param(HRequestReferenceXY.Param(0)); p.Val != 4 || len(sk.Params) != 1 {
		t.Errorf("set %v of %d", p, len(sk.Params))
	}
}

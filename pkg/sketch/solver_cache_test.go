package sketch

import (
	"math"
	"testing"
)

// A group solved again with the same equations, as at each step of a
// drag, takes the Jacobians its kept system wrote; with NoSolverCache
// it writes them again, to the same answer.
func TestSolverCacheKeptBetweenSolves(t *testing.T) {
	t.Parallel()
	var lengths [2][]float64
	for n, off := range []bool{false, true} {
		sk, hr, _ := lineSketch(t)
		sk.NoSolverCache = off
		hg := sk.GroupOrder[len(sk.GroupOrder)-1]
		x := sk.MustEntity(hr.Entity(2)).Param[0]
		for range 4 {
			sk.Param(x).Val += 0.5
			sk.MarkGroupDirty(hg, false)
			sk.GenerateAll()
			lengths[n] = append(lengths[n], lineLength(sk, hr), sk.Param(x).Val)
		}
		kept := 0
		if sys := sk.systems[hg]; sys != nil {
			kept = sys.JacobiansKept()
		}
		if off != (kept == 0) {
			t.Errorf("NoSolverCache %v: %d Jacobians kept", off, kept)
		}
	}
	for i := range lengths[0] {
		if math.Float64bits(lengths[0][i]) != math.Float64bits(lengths[1][i]) {
			t.Fatalf("kept %v, written again %v", lengths[0], lengths[1])
		}
	}
}

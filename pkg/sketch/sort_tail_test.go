package sketch

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// TestSortTailStable checks sortTailStable against a whole stable sort
// on lists sorted but for a tail, as generateAll leaves them, and on
// lists in any order, with keys repeated to see that it is stable.
func TestSortTailStable(t *testing.T) {
	t.Parallel()
	type item struct{ key, at uint32 }
	r := rand.New(rand.NewPCG(1, 2))
	key := func(e *item) uint32 { return e.key }
	for n := range 400 {
		size := n % 40
		s := make([]item, size)
		for i := range s {
			s[i] = item{key: uint32(r.IntN(30)), at: uint32(i)}
		}
		if n%3 != 0 {
			// A sorted head and a tail after it, as a group appends.
			head := r.IntN(size + 1)
			slices.SortStableFunc(s[:head], func(a, b item) int { return cmp.Compare(a.key, b.key) })
		}
		if n%7 == 0 && size > 0 {
			s[size-1].key = ^uint32(0)
		}
		want := slices.Clone(s)
		slices.SortStableFunc(want, func(a, b item) int { return cmp.Compare(a.key, b.key) })
		sortTailStable(s, key)
		if !slices.Equal(s, want) {
			t.Fatalf("list %d: got %v, want %v", n, s, want)
		}
	}
}

// TestEntityLookupAfterMove finds entities and params by handle after
// the list moved under the index, as a regeneration's sort moves it.
func TestEntityLookupAfterMove(t *testing.T) {
	t.Parallel()
	sk := NewSketch()
	for _, v := range []uint32{5, 9, 2, 7} {
		sk.Entities = append(sk.Entities, core.Entity{EntityBase: core.EntityBase{H: core.HEntity{V: v}}})
		sk.Params = append(sk.Params, core.Param{H: core.HParam{V: v}, Val: float64(v)})
	}
	for _, v := range []uint32{5, 9, 2, 7} {
		if e := sk.Entity(core.HEntity{V: v}); e == nil || e.H.V != v {
			t.Fatalf("entity %d: %v", v, e)
		}
	}
	sk.sortByHandle()
	sk.Entities = append(sk.Entities[:1], sk.Entities[2:]...) // 5 goes
	sk.Params = append(sk.Params[:1], sk.Params[2:]...)
	for _, v := range []uint32{2, 7, 9} {
		if e := sk.Entity(core.HEntity{V: v}); e == nil || e.H.V != v {
			t.Errorf("entity %d after the move: %v", v, e)
		}
		if p := sk.Param(core.HParam{V: v}); p == nil || p.Val != float64(v) {
			t.Errorf("param %d after the move: %v", v, p)
		}
	}
	if sk.Entity(core.HEntity{V: 5}) != nil || sk.Param(core.HParam{V: 5}) != nil {
		t.Error("found what was taken out")
	}
}

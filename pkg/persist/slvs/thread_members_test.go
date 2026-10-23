package slvs

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestThreadPartBeyond checks how far a thread made through finds its
// part going on past it, which the window warns of as a groove: a rod
// longer than the thread goes on, one as long doesn't, and a thread to
// a shoulder has nothing turned down past it.
func TestThreadPartBeyond(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		threadCase
		want float64
	}{
		{threadCase{"goes on", 5, 30, "M10", sketch.ThreadOptions{Length: 12, Through: true}}, 18},
		{threadCase{"ends with it", 5, 12, "M10", sketch.ThreadOptions{Length: 12, Through: true}}, 0},
		{threadCase{"to a shoulder", 5, 30, "M10", sketch.ThreadOptions{Length: 12}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sk, _, res := tc.build(t, 0)
			if got := sk.ThreadPartBeyond(res.Group); math.Abs(got-tc.want) > 0.01 {
				t.Errorf("the part goes on %g mm past the thread, want %g", got, tc.want)
			}
		})
	}
}

// TestThreadMembersOf checks the thread found from each of its groups,
// the groups next to it, and what is refused for a group that is no
// thread's helix.
func TestThreadMembersOf(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 5, 30, "M10", sketch.ThreadOptions{Length: 12}}
	sk, _, res := tc.build(t, 0)
	hs, err := sk.ThreadMembers(res.Group)
	if err != nil || len(hs) != 4 || hs[3] != res.Group {
		t.Fatalf("members %v, %v", hs, err)
	}
	of := sk.ThreadOfGroups()
	for _, h := range hs {
		if got, ok := sk.ThreadOfGroup(h); !ok || got != res.Group || of[h] != res.Group {
			t.Errorf("%v: thread %v, %v", h, got, ok)
		}
	}
	extrude := sk.GroupOrder[slices.Index(sk.GroupOrder, hs[0])-1]
	if _, ok := sk.ThreadOfGroup(extrude); ok {
		t.Error("the extrusion is one of the thread's groups")
	}
	prev, next, err := sk.ThreadNeighbours(res.Group)
	if err != nil || prev != extrude || next.V != 0 {
		t.Errorf("neighbours %v, %v, %v", prev, next, err)
	}

	// The thread's blank is drawn on the circle of the first sketch, so
	// it can't go before it; before the extrusion it can.
	if err := sk.CheckMoveThread(res.Group, sk.GroupOrder[1]); err == nil {
		t.Error("the thread could move before the sketch it is drawn on")
	}
	if err := sk.CheckMoveThread(res.Group, core.HGroup{V: 0xfff}); err == nil {
		t.Error("the thread could move before a group that doesn't exist")
	}
	if err := sk.CheckMoveThread(res.Group, core.HGroup{V: core.HGroupReferences}); err == nil {
		t.Error("the thread could move before the references")
	}
	if err := sk.MoveThread(res.Group, extrude); err != nil {
		t.Fatal(err)
	}
	if at := slices.Index(sk.GroupOrder, extrude); at != 6 {
		t.Errorf("the extrusion at %d after the thread moved before it, want 6", at)
	}

	if !sk.ThreadVisible(res.Group) {
		t.Error("not visible")
	}
	for _, f := range []func(core.HGroup) error{
		func(h core.HGroup) error { return sk.MoveThread(h, core.HGroup{}) },
		func(h core.HGroup) error { return sk.SetThreadSuppressed(h, true) },
		func(h core.HGroup) error { return sk.SetThreadVisible(h, false) },
		func(h core.HGroup) error { _, err := sk.ClearThreadItems(h); return err },
		func(h core.HGroup) error { _, _, err := sk.ThreadNeighbours(h); return err },
	} {
		if err := f(extrude); !errors.Is(err, sketch.ErrNotThread) {
			t.Errorf("on the extrusion: %v, want ErrNotThread", err)
		}
	}
	if sk.ThreadSuppressed(extrude) || sk.ThreadVisible(extrude) || sk.ThreadPartBeyond(extrude) != 0 {
		t.Error("the extrusion is taken for a thread")
	}
}

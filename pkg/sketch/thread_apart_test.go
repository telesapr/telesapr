package sketch_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestThreadGroupsApart checks that a thread whose groups were moved
// apart, its turning moved on its own to the end as the window's Move
// Group to End does, is no longer edited as a thread, but that its
// groups are still found, the same and in the same order as before the
// move, and that moved back it is a thread again: one helix, level
// starts with a chamfer, and a cosmetic thread's lathe moved away from
// its sketch.
func TestThreadGroupsApart(t *testing.T) {
	t.Parallel()
	for _, tc := range []threadCase{
		{"one start", 5, 30, "M10", sketch.ThreadOptions{Length: 12}},
		{"level starts chamfered", 6, 20, "M12xPh3P1", sketch.ThreadOptions{Length: 14, LevelStarts: true, Chamfer: true}},
		{"cosmetic", 6, 30, "M10", sketch.ThreadOptions{Length: 20, Cosmetic: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sk, _, res := tc.build(t, 0)
			want, err := sk.ThreadGroups(res.Group)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := sk.ThreadGroupsApart(res.Group); err != nil || !slices.Equal(got, want) {
				t.Fatalf("together: %v, %v; want %v", got, err, want)
			}
			lathe := want[1]
			if tc.o.Cosmetic {
				// A group put between the sketch and its lathe.
				h := sk.AddGroup(sketch.Group{Type: sketch.GroupDrawing3D, Name: "between", Visible: true, Scale: 1,
					Order: sk.Group(sk.GroupOrder[len(sk.GroupOrder)-1]).Order + 1})
				if err := sk.MoveGroup(h, lathe); err != nil {
					t.Fatal(err)
				}
			} else if err := sk.MoveGroup(lathe, core.HGroup{}); err != nil {
				t.Fatal(err)
			}
			sk.GenerateAllTyped(sketch.GenerateAll)

			if tc.o.Cosmetic {
				if hs, _ := sk.ThreadGroups(res.Group); len(hs) != 1 {
					t.Errorf("the cosmetic thread apart has groups %v, want its sketch alone", hs)
				}
			} else {
				if _, err := sk.ThreadGroups(res.Group); err == nil {
					t.Error("the thread apart is still found where it stands")
				}
				if _, err := sk.EditThread(res.Group, tc.options(t)); err == nil {
					t.Error("the thread apart is edited as one")
				}
			}
			got, err := sk.ThreadGroupsApart(res.Group)
			if err != nil || !slices.Equal(got, want) {
				t.Fatalf("apart: %v, %v; want %v", got, err, want)
			}

			// Moved back, it is a thread again.
			if !tc.o.Cosmetic {
				if err := sk.MoveGroup(lathe, want[2]); err != nil {
					t.Fatal(err)
				}
				if hs, err := sk.ThreadGroups(res.Group); err != nil || !slices.Equal(hs, want) {
					t.Errorf("moved back: %v, %v; want %v", hs, err, want)
				}
			}
		})
	}
}

// TestThreadGroupsApartUnclear checks that a thread apart one of whose
// groups is no longer made as a thread's isn't taken for one.
func TestThreadGroupsApartUnclear(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 5, 30, "M10", sketch.ThreadOptions{Length: 12}}
	sk, _, res := tc.build(t, 0)
	hs, err := sk.ThreadGroups(res.Group)
	if err != nil {
		t.Fatal(err)
	}
	// Moved apart, then its turning made about another axis.
	if err := sk.MoveGroup(hs[1], core.HGroup{}); err != nil {
		t.Fatal(err)
	}
	sk.Group(hs[1]).Predef.EntityB = core.HEntity{V: 0x30001}
	if _, err := sk.ThreadGroupsApart(res.Group); !errors.Is(err, sketch.ErrThreadGroupsUnclear) {
		t.Errorf("found with its turning gone: %v", err)
	}
	if _, err := sk.ThreadGroupsApart(hs[0]); !errors.Is(err, sketch.ErrNotThread) {
		t.Errorf("a sketch that keeps no thread: %v", err)
	}
}

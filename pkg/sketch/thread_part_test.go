package sketch_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestThreadPastPart checks threads against the part they are made on:
// one longer than the part, through or not, run off it the other way,
// or on a rod thinner than its root, is refused before anything is made; one as long as the part, or
// a nut's as deep as its block, is made.
func TestThreadPastPart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		threadCase
		want error
	}{
		{threadCase{"longer", 5, 10, "M10", sketch.ThreadOptions{Length: 20}}, sketch.ErrThreadPastPart},
		{threadCase{"longer through", 5, 10, "M10", sketch.ThreadOptions{Length: 20, Through: true}}, sketch.ErrThreadPastPart},
		{threadCase{"a little longer", 5, 10, "M10", sketch.ThreadOptions{Length: 10.5}}, sketch.ErrThreadPastPart},
		{threadCase{"the other way", 5, 10, "M10", sketch.ThreadOptions{Length: 8, Reverse: true}}, sketch.ErrThreadNoPart},
		{threadCase{"thicker than the rod", 4, 30, "M10", sketch.ThreadOptions{Length: 12}}, sketch.ErrThreadThinPart},
		{threadCase{"nut deeper", 8, 6, "M8", sketch.ThreadOptions{Length: 8, Internal: true}}, sketch.ErrThreadPastPart},
		{threadCase{"as long", 5, 10, "M10", sketch.ThreadOptions{Length: 10, Through: true}}, nil},
		{threadCase{"as long, to a shoulder", 5, 10, "M10", sketch.ThreadOptions{Length: 10}}, nil},
		{threadCase{"downwards", 3, -20, "M6", sketch.ThreadOptions{Length: 10, Reverse: true}}, nil},
		{threadCase{"nut", 7, 6.5, "M8", sketch.ThreadOptions{Length: 6.5, Through: true, Internal: true}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sk, _, res, err := tc.tryBuild(t, 0)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v, want %v", err, tc.want)
			}
			if err != nil {
				if res.Group != (core.HGroup{}) || len(sk.Groups) != 3 {
					t.Errorf("a refused thread made groups: %d groups, thread %v", len(sk.Groups), res.Group)
				}
				return
			}
			requireSoundThread(t, sk, res)
		})
	}
}

// TestThreadPastPartMessage checks that the refusal says how far the
// part reaches.
func TestThreadPastPartMessage(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 5, 10, "M10", sketch.ThreadOptions{Length: 20}}
	_, _, _, err := tc.tryBuild(t, 0)
	if err == nil || !strings.Contains(err.Error(), "reaches 10 mm") {
		t.Errorf("error %v, want how far the part reaches", err)
	}
}

// TestThreadPastPartEdit checks that an edit longer than the part is
// refused and changes nothing, and that a part made shorter than its
// thread afterwards puts a problem on the thread's helix group, which
// goes when the part is long enough again.
func TestThreadPastPartEdit(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 4, 30, "M8", sketch.ThreadOptions{Length: 12}}
	sk, _, res := tc.build(t, 0)
	sk.ChordTolPct = 0
	was := string(tlspr.Encode(sk, tlspr.Options{}))
	o := tc.options(t)
	o.Length = 40
	if _, err := sk.EditThread(res.Group, o); !errors.Is(err, sketch.ErrThreadPastPart) {
		t.Errorf("an edit longer than the part: %v", err)
	}
	if err := sk.CheckThreadEdit(res.Group, o); !errors.Is(err, sketch.ErrThreadPastPart) {
		t.Errorf("CheckThreadEdit: %v", err)
	}
	if got := string(tlspr.Encode(sk, tlspr.Options{})); got != was {
		t.Error("a refused edit changed the file")
	}
	// An edit within the part is checked against the part from before
	// the thread, not the one it turned down: a fine thread's ridge feet
	// lie where the coarse one's turning took the rod away.
	o.Length = 20
	o.Spec = threadCase{designation: "M8x0.5"}.options(t).Spec
	if err := sk.CheckThreadEdit(res.Group, o); err != nil {
		t.Errorf("an edit within the part: %v", err)
	}

	// The rod, extruded 30 one-sided as two params of 15, made 10.
	part := sk.Groups[2].H
	if sk.Group(part).Type != sketch.GroupExtrude {
		t.Fatalf("group 2 is %v", sk.Group(part).Type)
	}
	setRod := func(height float64) {
		sk.MustParam(part.Param(2)).Val = height / 2
		sk.MarkGroupDirty(part, false)
		sk.GenerateAllTyped(sketch.GenerateAll)
	}
	setRod(10)
	if p := sk.Group(res.Group).Problem.Msg; !strings.Contains(p, "longer than its part") {
		t.Errorf("problem %q on the thread past its part", p)
	}
	setRod(30)
	if p := sk.Group(res.Group).Problem.Msg; p != "" {
		t.Errorf("problem %q on the thread on its part again", p)
	}
}

// TestThreadPastMeshPart checks a thread against a part forced to a
// triangle mesh, which is measured on its triangles.
func TestThreadPastMeshPart(t *testing.T) {
	t.Parallel()
	tc := threadCase{"", 5, 10, "M10", sketch.ThreadOptions{Length: 20}}
	sk, circle, _, err := tc.tryBuild(t, 0)
	if !errors.Is(err, sketch.ErrThreadPastPart) {
		t.Fatalf("on the solid: %v", err)
	}
	part := sk.Groups[2].H
	sk.Group(part).ForceToMesh = true
	sk.MarkGroupDirty(part, false)
	sk.GenerateAllTyped(sketch.GenerateAll)
	if len(sk.Group(part).RunningShell.Surface) != 0 || len(sk.Group(part).RunningMesh.Triangles) == 0 {
		t.Fatal("the part isn't a mesh")
	}
	o := tc.options(t)
	for _, c := range []struct {
		length  float64
		reverse bool
		want    error
	}{{20, false, sketch.ErrThreadPastPart}, {8, false, nil}, {8, true, sketch.ErrThreadNoPart}} {
		o.Length, o.Reverse = c.length, c.reverse
		if err := sk.CheckThread(circle, o); !errors.Is(err, c.want) {
			t.Errorf("length %g, reverse %v: %v, want %v", c.length, c.reverse, err, c.want)
		}
	}
}

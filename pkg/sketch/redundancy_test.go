package sketch

import (
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// linesSketch is a new file with n line segments in the drawing
// group, the i-th from (0, 2i) to (5, 2i+1+i), none parallel to
// another.
func linesSketch(t *testing.T, n int) (*Sketch, core.HGroup, []core.HRequest) {
	t.Helper()
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	hg := sk.GroupOrder[len(sk.GroupOrder)-1]
	sk.Active = hg
	var hrs []core.HRequest
	for i := range n {
		hr := sk.AddRequest(Request{Type: RequestLineSegment, Group: hg, Workplane: hg.Entity(0)})
		sk.GenerateAll()
		y := float64(2 * i)
		sk.MustEntity(hr.Entity(1)).PointForceTo(sk, geom.Vector{Y: y})
		sk.MustEntity(hr.Entity(2)).PointForceTo(sk, geom.Vector{X: 5, Y: y + 1 + float64(i)})
		hrs = append(hrs, hr)
	}
	sk.GenerateAll()
	return sk, hg, hrs
}

func (sk *Sketch) constrainIn(hg core.HGroup, typ core.ConstraintType, a, b core.HEntity) core.HConstraint {
	return sk.AddConstraint(core.Constraint{Type: typ, Group: hg, Workplane: hg.Entity(0), EntityA: a, EntityB: b})
}

// Two horizontal lines made parallel too: the three constraints say two
// things, and the group is all three, though the solve substitutes away
// the horizontals' equations and only the parallel's shows in what it
// solves.
func TestRedundantGroupNamesEveryConstraint(t *testing.T) {
	t.Parallel()
	sk, hg, hrs := linesSketch(t, 2)
	l0, l1 := hrs[0].Entity(0), hrs[1].Entity(0)
	h0 := sk.constrainIn(hg, core.ConstraintHorizontal, l0, core.NoEntity)
	h1 := sk.constrainIn(hg, core.ConstraintHorizontal, l1, core.NoEntity)
	par := sk.constrainIn(hg, core.ConstraintParallel, l0, l1)
	sk.GenerateAll()
	g := sk.Group(hg)
	if g.Solved.How != core.SolveRedundantOkay {
		t.Fatalf("solved %v, want redundant", g.Solved.How)
	}
	want := [][]core.HConstraint{{h0, h1, par}}
	if got := sk.RedundantGroups(hg); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("groups %v, want %v", got, want)
	}
	if !slices.Equal(g.Solved.Remove, want[0]) {
		t.Errorf("remove %v, want %v", g.Solved.Remove, want[0])
	}
	text := sk.RedundancyText(hg)
	if text != "c001-horizontal, c002-horizontal and c003-parallel over-determine each other" {
		t.Errorf("text %q", text)
	}
	// Solved again, with the substitution the group's first solve
	// skips, the group is the same.
	sk.MarkGroupDirty(hg, false)
	sk.GenerateAll()
	if got := sk.RedundantGroups(hg); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("solved again, groups %v, want %v", got, want)
	}
}

// Two redundancies apart, each of three constraints: no one constraint
// removed makes the group whole, so a search removing one at a time
// named none; each dependency is named.
func TestRedundantGroupsApart(t *testing.T) {
	t.Parallel()
	sk, hg, hrs := linesSketch(t, 4)
	l := func(i int) core.HEntity { return hrs[i].Entity(0) }
	var want [][]core.HConstraint
	for _, pair := range [][2]int{{0, 1}, {2, 3}} {
		a := sk.constrainIn(hg, core.ConstraintVertical, l(pair[0]), core.NoEntity)
		b := sk.constrainIn(hg, core.ConstraintVertical, l(pair[1]), core.NoEntity)
		c := sk.constrainIn(hg, core.ConstraintParallel, l(pair[0]), l(pair[1]))
		want = append(want, []core.HConstraint{a, b, c})
	}
	sk.GenerateAll()
	if got := sk.RedundantGroups(hg); !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("groups %v, want %v", got, want)
	}
	if g := sk.Group(hg); g.Solved.Timeout || len(g.Solved.Remove) != 6 {
		t.Errorf("remove %v, timeout %v", g.Solved.Remove, g.Solved.Timeout)
	}
}

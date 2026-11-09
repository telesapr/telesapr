package api

import (
	"strings"
	"testing"
)

// Two pairs of vertical lines made parallel too: each pair and its
// parallel over-determine each other, and Redundancies names both sets,
// where removing one constraint at a time finds none that fixes it.
func TestRedundancies(t *testing.T) {
	t.Parallel()
	d := New()
	g := d.Active()
	var lines []Entity
	for i := range 4 {
		x := float64(10 * i)
		lines = append(lines, d.Sketch.Line(V(x, 0, 0), V(x+1+float64(i), 10, 0)))
	}
	if s := g.Redundancies(); len(s) != 0 {
		t.Errorf("a solved group has redundancies %v", s)
	}
	var want [][]Constraint
	for _, pair := range [][2]int{{0, 1}, {2, 3}} {
		a := d.Constrain.Vertical(lines[pair[0]])
		b := d.Constrain.Vertical(lines[pair[1]])
		c := d.Constrain.Parallel(lines[pair[0]], lines[pair[1]])
		want = append(want, []Constraint{a, b, c})
	}
	got := g.Redundancies()
	if g.Solved() || len(got) != len(want) {
		t.Fatalf("solved %v, redundancies %v, want %v", g.Solved(), got, want)
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("set %d is %v, want %v", i, got[i], want[i])
		}
		for k := range want[i] {
			if got[i][k].H != want[i][k].H {
				t.Errorf("set %d is %v, want %v", i, got[i], want[i])
			}
		}
	}
	if n := len(g.Conflicts()); n != 6 {
		t.Errorf("%d conflicts, want the six of both sets", n)
	}
	if desc := d.Describe(); !strings.Contains(desc, "NOT SOLVED (c001-vertical, c002-vertical and c003-parallel "+
		"over-determine each other; c004-vertical, c005-vertical and c006-parallel over-determine each other)") {
		t.Errorf("the description says:\n%s", desc)
	}
	// Allowed, the group solves, and has none.
	g.SetAllowRedundant(true)
	if !g.Solved() || len(g.Redundancies()) != 0 {
		t.Errorf("allowed: solved %v, redundancies %v", g.Solved(), g.Redundancies())
	}
}

// Constraints that cannot all hold are conflicts, not redundancies.
func TestRedundanciesNotConflicts(t *testing.T) {
	t.Parallel()
	d := New()
	g := d.Active()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 5, 0))
	d.Constrain.Length(line, 10)
	d.Constrain.Length(line, 12).SetReference(false)
	if g.Solved() || len(g.Conflicts()) == 0 || len(g.Redundancies()) != 0 {
		t.Errorf("solved %v, conflicts %v, redundancies %v", g.Solved(), g.Conflicts(), g.Redundancies())
	}
}

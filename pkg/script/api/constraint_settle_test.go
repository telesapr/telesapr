package api

import "testing"

// A dimension that the group already holds is made a reference by the
// script, as it is in the window, and the group still solves.
func TestConstrainRedundantDimensionIsReference(t *testing.T) {
	t.Parallel()
	d, g, l := boundLine(t, 10)
	d.Constrain.Lock(l.End())
	d.Edit.Regen()
	c := d.Constrain.Distance(l.Start(), l.End(), 10)
	d.Edit.Regen()
	if !c.Reference() {
		t.Errorf("%v: a redundant dimension should be a reference", c)
	}
	if !g.Solved() {
		t.Errorf("group not solved with a reference dimension")
	}
}

// With redundancy allowed, the dimension stays what it was asked to be.
func TestConstrainRedundantDimensionAllowed(t *testing.T) {
	t.Parallel()
	d, g, l := boundLine(t, 10)
	d.Constrain.Lock(l.End())
	g.SetAllowRedundant(true)
	d.Edit.Regen()
	c := d.Constrain.Distance(l.Start(), l.End(), 10)
	if c.Reference() {
		t.Errorf("%v: a group that allows redundancy should keep the dimension", c)
	}
}

// A dimension that is not redundant stays a driving one.
func TestConstrainDrivingDimension(t *testing.T) {
	t.Parallel()
	d, _, l := boundLine(t, 10)
	c := d.Constrain.Distance(l.Start(), l.End(), 12)
	if c.Reference() {
		t.Errorf("%v: should drive", c)
	}
}

// The same constraint asked for twice is added once.
func TestConstrainDuplicateNotAddedTwice(t *testing.T) {
	t.Parallel()
	d, _, l := boundLine(t, 10)
	a := d.Constrain.Distance(l.Start(), l.End(), 12)
	n := len(d.Host().Sketch().Constraints)
	b := d.Constrain.Distance(l.Start(), l.End(), 12)
	if a.H != b.H || len(d.Host().Sketch().Constraints) != n {
		t.Errorf("duplicate added: %v and %v, %d constraints, want %d", a, b, len(d.Host().Sketch().Constraints), n)
	}
}

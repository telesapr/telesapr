package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// newLabelBox is the box of new label c as ClearNewLabel judges it.
func newLabelBox(sk *Sketch, c *core.Constraint) labelBox {
	at, _ := sk.labelAnchor(sk.view(), c, labelNominalText)
	return sk.labelBoxAt(sk.view(), at, c, labelNominalText)
}

// A second distance between the same two points, as a bound on a
// dimension makes, moves its label along the normal until it is clear
// of the first; the first, alone, stays where the menu puts it.
func TestClearNewLabelOfSameLine(t *testing.T) {
	t.Parallel()
	sk, _, hc := lineSketch(t)
	first := sk.Constraint(hc)
	a := sk.MustEntity(first.PtA).PointGetNum(sk)
	b := sk.MustEntity(first.PtB).PointGetNum(sk)
	first.Disp.Offset = sk.DistanceLabelOffset(a, b)

	c := *first
	c.H = core.HConstraint{}
	c.Limit = core.LimitMax
	before := c.Disp.Offset
	sk.ClearNewLabel(&c)
	if c.Disp.Offset.Equals(before) {
		t.Fatalf("the label stayed at %v, on the first", before)
	}
	moved := c.Disp.Offset.Minus(before)
	if moved.Dot(before) <= 0 || moved.Cross(before).Magnitude() > 1e-9 {
		t.Errorf("the label moved by %v, not outward along %v", moved, before)
	}
	at, _ := sk.labelAnchor(sk.view(), first, labelText(first))
	if newLabelBox(sk, &c).overlaps(sk.labelBoxAt(sk.view(), at, first, labelText(first))) {
		t.Errorf("the label at %v is still on the first", c.Disp.Offset)
	}

	// A label already clear is left as it is.
	far := c
	far.Disp.Offset = geom.Vector{Y: 1000}
	sk.ClearNewLabel(&far)
	if !far.Disp.Offset.Equals(geom.Vector{Y: 1000}) {
		t.Errorf("a clear label moved to %v", far.Disp.Offset)
	}
}

// The texts the boxes are measured on follow what the window shows.
func TestLabelText(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		c    core.ConstraintBase
		want string
	}{
		{core.ConstraintBase{Type: core.ConstraintPtPtDistance, ValA: -4, Limit: core.LimitMin}, "min 4.00"},
		{core.ConstraintBase{Type: core.ConstraintDiameter, ValA: 6}, "⌀6.00"},
		{core.ConstraintBase{Type: core.ConstraintDiameter, ValA: 6, Other: true, Reference: true}, "R3.00 REF"},
		{core.ConstraintBase{Type: core.ConstraintAngle, ValA: 30, Name: "a"}, "a = 30.0°"},
		{core.ConstraintBase{Type: core.ConstraintLengthRatio, ValA: 2}, "2.000:1"},
		{core.ConstraintBase{Type: core.ConstraintComment, Comment: "note"}, "note"},
	} {
		if got := labelText(&core.Constraint{ConstraintBase: tc.c}); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.c.Type, got, tc.want)
		}
	}
}

// A kind of constraint whose label place is not known, and a comment,
// are left where they are.
func TestClearNewLabelLeavesOthers(t *testing.T) {
	t.Parallel()
	sk, _, hc := lineSketch(t)
	for _, typ := range []core.ConstraintType{core.ConstraintAxisAngle, core.ConstraintComment, core.ConstraintHorizontal} {
		c := *sk.Constraint(hc)
		c.Type = typ
		c.Disp.Offset = geom.Vector{}
		sk.ClearNewLabel(&c)
		if !c.Disp.Offset.Equals(geom.Vector{}) {
			t.Errorf("%v moved to %v", typ, c.Disp.Offset)
		}
	}
	if _, ok := sk.labelAnchor(sk.view(), &core.Constraint{ConstraintBase: core.ConstraintBase{
		Type: core.ConstraintPtLineDistance, PtA: core.HEntity{V: 999}}}, ""); ok {
		t.Errorf("a distance from a missing point has a label place")
	}
}

// A comment in the group is kept clear of, at its place.
func TestClearNewLabelOfComment(t *testing.T) {
	t.Parallel()
	sk, _, hc := lineSketch(t)
	c := *sk.Constraint(hc)
	c.H = core.HConstraint{}
	c.Limit = core.LimitMin
	at, _ := sk.labelAnchor(sk.view(), &c, labelNominalText)
	sk.Constraints = append(sk.Constraints, core.Constraint{ConstraintBase: core.ConstraintBase{
		Type: core.ConstraintComment, Group: sk.Active, Comment: "a long comment here"}})
	sk.Constraints[len(sk.Constraints)-1].Disp.Offset = at
	sk.ClearNewLabel(&c)
	com := &sk.Constraints[len(sk.Constraints)-1]
	cat, _ := sk.labelAnchor(sk.view(), com, labelText(com))
	if newLabelBox(sk, &c).overlaps(sk.labelBoxAt(sk.view(), cat, com, labelText(com))) {
		t.Errorf("the label at %v is on the comment", c.Disp.Offset)
	}
}

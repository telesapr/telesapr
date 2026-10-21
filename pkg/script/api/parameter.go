package api

import (
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Parameter is a named value of the document's parameter table, which
// expressions on dimensions and group values can name, as Edit >
// Parameters shows it. It is not a Param, which is one of the solver's
// unknowns.
type Parameter struct {
	Name string
	// Expr is the expression the value comes from; it may name other
	// parameters. A parameter is a plain number, which counts in the
	// document's unit where it drives a length.
	Expr string
	// Value is the expression's value at the last regeneration.
	Value float64
	// Err says why Expr could not be evaluated, a cycle or an unknown
	// name; Value then keeps the last good value.
	Err string
}

// SetExpr gives one of the group's values text, as typing it on the
// group screen does: sketch.GroupCopies for a step and repeat or a pattern,
// sketch.GroupPitch for a helix's fixed pitch, a length in the
// document's unit (View.Units), sketch.GroupScale for a link,
// sketch.GroupExtent for an extrusion's held depth, a length, or a
// revolve's or helix's held angle in degrees. A plain
// number is stored as its value; any other text is kept and evaluated
// again at each regeneration.
func (g Group) SetExpr(value sketch.GroupValue, expr string) (err error) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if value == sketch.GroupCopies && group.Type != sketch.GroupTranslate && group.Type != sketch.GroupRotate && group.Type != sketch.GroupPathPattern {
			fail("%s isn't a step and repeat or a pattern", groupDescription(group))
		}
		err = sk.SetGroupText(g.H, value, expr)
	})
	return err
}

// Expr is the expression one of the group's values comes from, or ""
// for a plain number.
func (g Group) Expr(value sketch.GroupValue) (expr string) {
	g.d.read(func(sk *sketch.Sketch) { _, e := g.get(sk).Value(value); expr = *e })
	return expr
}

// Value is one of the group's values: the number of copies, the pitch
// in mm, or the scale.
func (g Group) Value(value sketch.GroupValue) (v float64) {
	g.d.read(func(sk *sketch.Sketch) { val, _ := g.get(sk).Value(value); v = *val })
	return v
}

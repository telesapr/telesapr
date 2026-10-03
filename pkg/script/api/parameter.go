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

// Parameters lists the parameter table, in the order the parameters
// were made.
func (d *Doc) Parameters() (params []Parameter) {
	d.read(func(sk *sketch.Sketch) {
		for _, p := range sk.Parameters {
			params = append(params, Parameter(p))
		}
	})
	return params
}

// Parameter returns the parameter named name, and whether there is
// one.
func (d *Doc) Parameter(name string) (p Parameter, ok bool) {
	d.read(func(sk *sketch.Sketch) {
		if sp := sk.Parameter(name); sp != nil {
			p, ok = Parameter(*sp), true
		}
	})
	return p, ok
}

// SetParameter gives the parameter name the expression expr, and adds
// it to the table if there is none:
// SetParameter("wall", "2.5"), then SetParameter("bore", "wall*4").
// An expression that names an unknown parameter, makes a cycle or has
// no value is refused, and the table is left as it was. What uses the
// parameter regenerates.
func (d *Doc) SetParameter(name, expr string) (err error) {
	d.change(func(sk *sketch.Sketch) {
		if sk.Parameter(name) == nil {
			err = sk.AddParameter(name, expr)
		} else {
			err = sk.SetParameterExpr(name, expr)
		}
	})
	return err
}

// RenameParameter renames a parameter, and rewrites every expression
// that names it.
func (d *Doc) RenameParameter(from, to string) (err error) {
	d.change(func(sk *sketch.Sketch) { err = sk.RenameParameter(from, to) })
	return err
}

// DeleteParameter removes a parameter. One that an expression names
// is not removed, and the error says what names it.
func (d *Doc) DeleteParameter(name string) (err error) {
	d.change(func(sk *sketch.Sketch) { err = sk.DeleteParameter(name) })
	return err
}

// ParameterUsers describes what names the parameter: other
// parameters, constraints and group values.
func (d *Doc) ParameterUsers(name string) (users []string) {
	d.read(func(sk *sketch.Sketch) { users = sk.ParameterUsers(name) })
	return users
}

// ExprErrors lists the expressions on constraints and group values
// that the last regeneration could not evaluate, each with why; the
// values they drive keep their last good value.
func (d *Doc) ExprErrors() (errs []string) {
	d.read(func(sk *sketch.Sketch) {
		for _, e := range sk.ExprErrors {
			errs = append(errs, e.Error())
		}
	})
	return errs
}

// SetExpr gives one of the group's values text, as typing it on the
// group screen does: sketch.GroupCopies for a step and repeat,
// sketch.GroupPitch for a helix's fixed pitch, a length in the
// document's unit (Doc.Units), sketch.GroupScale for a link. A plain
// number is stored as its value; any other text is kept and evaluated
// again at each regeneration.
func (g Group) SetExpr(value sketch.GroupValue, expr string) (err error) {
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if value == sketch.GroupCopies && group.Type != sketch.GroupTranslate && group.Type != sketch.GroupRotate {
			fail("%s isn't a step and repeat", groupDescription(group))
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

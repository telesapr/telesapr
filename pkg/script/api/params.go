package api

import "github.com/telesapr/telesapr/pkg/sketch"

// ParamsMenu holds the calls of Edit > Parameters: the named numbers
// that expressions use, and the configurations that give them other
// values. A script reaches it as doc.Params.
type ParamsMenu struct{ d *Doc }

// List lists the parameter table, in the order the parameters
// were made.
func (m ParamsMenu) List() (params []Parameter) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, p := range sk.Parameters {
			params = append(params, Parameter(p))
		}
	})
	return params
}

// Get returns the parameter named name, and whether there is
// one.
func (m ParamsMenu) Get(name string) (p Parameter, ok bool) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		if sp := sk.Parameter(name); sp != nil {
			p, ok = Parameter(*sp), true
		}
	})
	return p, ok
}

// Set gives the parameter name the expression expr, and adds
// it to the table if there is none:
// SetParameter("wall", "2.5"), then SetParameter("bore", "wall*4").
// An expression that names an unknown parameter, makes a cycle or has
// no value is refused, and the table is left as it was. What uses the
// parameter regenerates.
func (m ParamsMenu) Set(name, expr string) (err error) {
	d := m.d
	d.change(func(sk *sketch.Sketch) {
		if sk.Parameter(name) == nil {
			err = sk.AddParameter(name, expr)
		} else {
			err = sk.SetParameterExpr(name, expr)
		}
	})
	return err
}

// Rename renames a parameter, and rewrites every expression
// that names it.
func (m ParamsMenu) Rename(from, to string) (err error) {
	d := m.d
	d.change(func(sk *sketch.Sketch) { err = sk.RenameParameter(from, to) })
	return err
}

// Delete removes a parameter. One that an expression names
// is not removed, and the error says what names it.
func (m ParamsMenu) Delete(name string) (err error) {
	d := m.d
	d.change(func(sk *sketch.Sketch) { err = sk.DeleteParameter(name) })
	return err
}

// Users describes what names the parameter: other
// parameters, constraints and group values.
func (m ParamsMenu) Users(name string) (users []string) {
	d := m.d
	d.read(func(sk *sketch.Sketch) { users = sk.ParameterUsers(name) })
	return users
}

// Errors lists the expressions on constraints and group values
// that the last regeneration could not evaluate, each with why; the
// values they drive keep their last good value.
func (m ParamsMenu) Errors() (errs []string) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, e := range sk.ExprErrors {
			errs = append(errs, e.Error())
		}
	})
	return errs
}

// AddConfiguration adds a configuration, a named set of the parameters'
// expressions, from the table as it is, and makes it active: while it
// is, SetParameter changes it as well as the table.
func (m ParamsMenu) AddConfiguration(name string) (err error) {
	d := m.d
	d.change(func(sk *sketch.Sketch) { err = sk.AddConfiguration(name) })
	return err
}

// ActivateConfiguration writes the configuration's expressions into
// the parameter table and makes it active; "" makes none active, and
// leaves the table as it is. The model regenerates.
func (m ParamsMenu) ActivateConfiguration(name string) (err error) {
	d := m.d
	d.change(func(sk *sketch.Sketch) { err = sk.ActivateConfiguration(name) })
	return err
}

// ActiveConfiguration is the active configuration's name, or "".
func (m ParamsMenu) ActiveConfiguration() (name string) {
	d := m.d
	d.read(func(sk *sketch.Sketch) { name = sk.ActiveConfiguration })
	return name
}

// Configurations are the configurations' names, in the order they were
// made.
func (m ParamsMenu) Configurations() (names []string) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, c := range sk.Configurations {
			names = append(names, c.Name)
		}
	})
	return names
}

// RenameConfiguration renames a configuration.
func (m ParamsMenu) RenameConfiguration(from, to string) (err error) {
	d := m.d
	d.change(func(sk *sketch.Sketch) { err = sk.RenameConfiguration(from, to) })
	return err
}

// DeleteConfiguration removes a configuration; the table stays as it
// is.
func (m ParamsMenu) DeleteConfiguration(name string) (err error) {
	d := m.d
	d.change(func(sk *sketch.Sketch) { err = sk.DeleteConfiguration(name) })
	return err
}

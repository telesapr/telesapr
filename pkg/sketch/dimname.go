package sketch

import (
	"fmt"
	"math"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// A dimension by name (roadmap 16.1, A3): a constraint with a value may
// be given a name, which expressions read as they read a parameter,
// "width/2" for half the constraint named width. The value read is the
// one its label shows, in the document's unit: a distance without its
// side's sign, a diameter shown as a radius as the radius. Names share
// one namespace with the parameters. A reference dimension, which
// measures rather than drives, can't be named: its value comes from a
// solve, which an expression read before it would chase.

// NamedConstraint is the constraint named name, or nil.
func (sk *Sketch) NamedConstraint(name string) *core.Constraint {
	if name == "" {
		return nil
	}
	for i := range sk.Constraints {
		if sk.Constraints[i].Name == name {
			return &sk.Constraints[i]
		}
	}
	return nil
}

// nameFree checks that name can name a new parameter or dimension.
func (sk *Sketch) nameFree(name string) error {
	if sk.Parameter(name) != nil {
		return fmt.Errorf("there is already a parameter %q", name)
	}
	if c := sk.NamedConstraint(name); c != nil {
		return fmt.Errorf("there is already a dimension %q (%s)", name, c.DescriptionString())
	}
	return nil
}

// SetConstraintName names the constraint hc, renaming every use of an
// old name, or with "" takes its name away, which a name in use can't
// be.
func (sk *Sketch) SetConstraintName(hc core.HConstraint, name string) error {
	c := sk.Constraint(hc)
	if c == nil {
		return fmt.Errorf("no constraint %08x", hc.V)
	}
	name = strings.TrimSpace(name)
	if name == c.Name {
		return nil
	}
	if name == "" {
		if users := sk.ParameterUsers(c.Name); len(users) > 0 {
			return fmt.Errorf("dimension %q is used by %s", c.Name, strings.Join(users, ", "))
		}
		c.Name = ""
		return nil
	}
	if !c.HasLabel() || c.Type == core.ConstraintComment {
		return fmt.Errorf("%s has no value to name", c.DescriptionString())
	}
	if c.Reference || sk.allDimsReference(c.Group) {
		return fmt.Errorf("%s is a reference dimension, which measures; only a dimension that drives can be named", c.DescriptionString())
	}
	if !core.ValidVariableName(name) {
		return fmt.Errorf("%q can't name a dimension: use a letter or underscore, then letters, digits and underscores, and no function or constant name", name)
	}
	if err := sk.nameFree(name); err != nil {
		return err
	}
	if from := c.Name; from != "" {
		sk.forEachExpr(func(expr *string, _ string) {
			*expr = core.RenameVariable(*expr, from, name)
		})
	}
	c.Name = name
	sk.applyParameters()
	return nil
}

// ShownValue is the value of a constraint as its label shows it, in the
// document's unit, which an expression reads by its name.
func (sk *Sketch) ShownValue(c *core.Constraint) float64 {
	v := c.ValA
	if IsLengthValue(c.Type) {
		v /= sk.Units.MmPer()
	}
	return shown(c, v)
}

// shown is the value a label shows for the stored value v, in the
// document's unit: without the sign that picks a side, and halved for
// a diameter shown as a radius.
func shown(c *core.Constraint, v float64) float64 {
	v = math.Abs(v)
	if c.Type == core.ConstraintDiameter && c.Other {
		v /= 2
	}
	return v
}

// allDimsReference reports whether group hg makes all its dimensions
// reference ones.
func (sk *Sketch) allDimsReference(hg core.HGroup) bool {
	g := sk.Group(hg)
	return g != nil && g.AllDimsReference
}

// SplitNamedText splits what was typed in a dimension's label: "name =
// value" names it, "= value" takes its name away, and a value alone
// leaves the name as it is (named false).
func SplitNamedText(s string) (name string, named bool, value string) {
	left, right, ok := strings.Cut(s, "=")
	if !ok {
		return "", false, s
	}
	left = strings.TrimSpace(left)
	if left != "" && !core.ValidVariableName(left) {
		return "", false, s
	}
	return left, true, strings.TrimSpace(right)
}

package sketch

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file is not a port: C++ SolveSpace evaluates an expression when
// it is typed and keeps only the number. Here a constraint's value and
// a group's values may keep the expression they came from, and the
// expression may name parameters from a table on the sketch. Every
// regeneration evaluates the table and then the stored expressions.
//
// Stored expressions are in the document's unit for lengths (Units)
// and in degrees for angles; parameters are plain numbers, which take
// the unit of the value they drive. A plain number the user types is
// stored as the value it gives, in millimetres; any other text is kept.

// Parameter is a named value that expressions can use.
type Parameter struct {
	Name string
	// Expr is the expression the value comes from; it may name other
	// parameters.
	Expr string
	// Value is Expr's value at the last evaluation. When Expr cannot
	// be evaluated, Value keeps the last good value and Err says why.
	Value float64
	Err   string
}

// ExprError is a stored expression of a constraint or a group that
// could not be evaluated. The value it drives keeps its last good
// value.
type ExprError struct {
	Group core.HGroup
	// Constraint is the constraint whose value it is, or zero for a
	// group's value.
	Constraint core.HConstraint
	// Field is the value's name in the file: "value" for a
	// constraint, or a group's "copies", "pitch", "val-c" or "scale".
	Field string
	Expr  string
	Err   string
}

func (e ExprError) Error() string {
	what := fmt.Sprintf("group %s %s", groupLabel(e.Group), e.Field)
	if e.Constraint.V != 0 {
		what = fmt.Sprintf("constraint c%03x", e.Constraint.V)
	}
	return fmt.Sprintf("%s = %s: %s", what, e.Expr, e.Err)
}

func groupLabel(hg core.HGroup) string { return fmt.Sprintf("g%03x", hg.V) }

// GroupValue names a group value that may hold an expression.
type GroupValue int

const (
	// GroupCopies is a step and repeat group's number of copies
	// (ValA).
	GroupCopies GroupValue = iota
	// GroupPitch is a helix's fixed pitch in millimetres, 0 when the
	// pitch is free, or a chamfer's distance (ValB).
	GroupPitch
	// GroupValC is an extrusion's draft in degrees (ValC).
	GroupValC
	// GroupScale is a link group's scale (Scale).
	GroupScale
)

// Field is the value's name in the file.
func (f GroupValue) Field() string {
	switch f {
	case GroupCopies:
		return "copies"
	case GroupPitch:
		return "pitch"
	case GroupValC:
		return "val-c"
	case GroupScale:
		return "scale"
	}
	return fmt.Sprintf("GroupValue(%d)", int(f))
}

// FieldName is the value's name in the file and in messages for this
// group: a chamfer's distance is ValB, as a helix's pitch is.
func (g *Group) FieldName(f GroupValue) string {
	if f == GroupPitch && g.Type == GroupChamfer {
		return "distance"
	}
	if f == GroupPitch && g.Type == GroupFillet {
		return "radius"
	}
	if f == GroupPitch && g.Type == GroupShell {
		return "thickness"
	}
	if f == GroupValC && g.Type == GroupExtrude {
		return "draft"
	}
	if f == GroupPitch && g.Type == GroupExtrude {
		return "draft-neutral"
	}
	if f == GroupValC && g.Type == GroupChamfer {
		return "second-distance"
	}
	if f == GroupCopies && g.Type == GroupLoft {
		return "turn"
	}
	if f == GroupPitch && g.Type == GroupLoft {
		return "start-tangency"
	}
	if f == GroupValC && g.Type == GroupLoft {
		return "end-tangency"
	}
	if f == GroupValC && g.Type == GroupSweep {
		return "twist"
	}
	return f.Field()
}

// IsLength reports whether the group's value f is a length, which the
// interface shows in its unit: a helix's pitch, a chamfer's distances.
func (g *Group) IsLength(f GroupValue) bool {
	return f == GroupPitch && g.Type != GroupLoft || f == GroupValC && g.Type == GroupChamfer
}

// Value returns the field's number and its expression, both
// addressable.
func (g *Group) Value(f GroupValue) (val *float64, expr *string) {
	switch f {
	case GroupCopies:
		return &g.ValA, &g.ExprA
	case GroupPitch:
		return &g.ValB, &g.ExprB
	case GroupValC:
		return &g.ValC, &g.ExprC
	case GroupScale:
		return &g.Scale, &g.ExprScale
	}
	panic(fmt.Sprintf("sketch: no group value %d", int(f)))
}

// EdgeRadius is a fillet's radius for one of its edges, Edge counting
// Edges and then SolidEdges: Val millimetres, from Expr when it is not
// "".
type EdgeRadius struct {
	Edge int
	Val  float64
	Expr string
}

// EdgeRadiusField names an edge's radius in messages and in the file.
func EdgeRadiusField(i int) string { return fmt.Sprintf("radius of edge %d", i+1) }

// EdgeRadius is the radius a fillet gives its edge i, counting Edges
// and then SolidEdges, and its expression: the edge's own, or the
// group's.
func (g *Group) EdgeRadius(i int) (float64, string, bool) {
	for _, er := range g.EdgeRadii {
		if er.Edge == i {
			return er.Val, er.Expr, true
		}
	}
	return g.ValB, g.ExprB, false
}

// SetEdgeRadiusText gives a fillet's edge i a radius of its own from
// what the user typed, in the document's unit, as SetGroupText does
// for the group's radius; "" gives it the group's radius again.
func (sk *Sketch) SetEdgeRadiusText(hg core.HGroup, i int, text string) error {
	g := sk.GetGroup(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if g.Type != GroupFillet {
		return fmt.Errorf("%s is not a fillet: only a fillet's edges have radii", g.Name)
	}
	if i < 0 || i >= len(g.Edges)+len(g.SolidEdges) {
		return fmt.Errorf("the fillet has no edge %d", i+1)
	}
	text = cleanText(text)
	at := -1
	for k := range g.EdgeRadii {
		if g.EdgeRadii[k].Edge == i {
			at = k
		}
	}
	if text == "" {
		if at >= 0 {
			g.EdgeRadii = append(g.EdgeRadii[:at], g.EdgeRadii[at+1:]...)
		}
		sk.MarkGroupDirty(hg, false)
		return nil
	}
	_, names, err := sk.parseNamed(text)
	if err != nil {
		return err
	}
	old := append([]EdgeRadius(nil), g.EdgeRadii...)
	if at < 0 {
		g.EdgeRadii = append(g.EdgeRadii, EdgeRadius{Edge: i})
		sort.Slice(g.EdgeRadii, func(a, b int) bool { return g.EdgeRadii[a].Edge < g.EdgeRadii[b].Edge })
		for k := range g.EdgeRadii {
			if g.EdgeRadii[k].Edge == i {
				at = k
			}
		}
	}
	er := &g.EdgeRadii[at]
	if v, plain := plainNumber(text, names); plain {
		v *= sk.Units.MmPer()
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			g.EdgeRadii = old
			return errors.New("a fillet's radius must be more than zero")
		}
		er.Val, er.Expr = v, ""
	} else {
		er.Expr = text
		sk.applyParameters()
		for _, ee := range sk.ExprErrors {
			if ee.Group == hg && ee.Constraint.V == 0 && ee.Field == EdgeRadiusField(i) {
				g.EdgeRadii = old
				sk.applyParameters()
				return errors.New(ee.Err)
			}
		}
	}
	sk.MarkGroupDirty(hg, false)
	return nil
}

// groupValues are the group values in file order.
var groupValues = []GroupValue{GroupCopies, GroupPitch, GroupValC, GroupScale}

// checkGroupValue says what is wrong with v as the value of f, as the
// group screen does when it is typed.
func checkGroupValue(g *Group, f GroupValue, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return errors.New("not a number")
	}
	switch f {
	case GroupCopies:
		if g.Type == GroupLoft {
			if v != float64(int(v)) {
				return errors.New("a loft turns its second profile by a whole number of curves")
			}
			break
		}
		if int(v) < 1 {
			return errors.New("can't repeat fewer than 1 time")
		}
		if int(v) > 999 {
			return errors.New("can't repeat more than 999 times")
		}
	case GroupScale:
		if math.Abs(v) < 1e-6 {
			return errors.New("scale cannot be zero")
		}
	case GroupValC:
		if g.Type == GroupExtrude && math.Abs(v) >= 90 {
			return fmt.Errorf("a draft is between -90 and 90 degrees, not %g", v)
		}
		if g.Type == GroupChamfer && v < 0 {
			return errors.New("a chamfer's second distance must be more than zero, or zero for the first again")
		}
	case GroupPitch:
		if g.Type == GroupChamfer && v <= 0 {
			return errors.New("a chamfer's distance must be more than zero")
		}
		if g.Type == GroupFillet && v <= 0 {
			return errors.New("a fillet's radius must be more than zero")
		}
		if g.Type == GroupShell && v <= 0 {
			return errors.New("a shell's thickness must be more than zero")
		}
	}
	return nil
}

// Parameter returns the parameter named name, or nil.
func (sk *Sketch) Parameter(name string) *Parameter {
	for i := range sk.Parameters {
		if sk.Parameters[i].Name == name {
			return &sk.Parameters[i]
		}
	}
	return nil
}

// parseNamed parses an expression and checks that every name it uses
// is a parameter.
func (sk *Sketch) parseNamed(text string) (*core.Expr, []string, error) {
	e, err := core.NewFromString(text)
	if err != nil {
		return nil, nil, fmt.Errorf("not a valid number or expression: '%s': %w", text, err)
	}
	names := e.Variables()
	for _, n := range names {
		if sk.Parameter(n) == nil && sk.NamedConstraint(n) == nil {
			return nil, nil, fmt.Errorf("unknown parameter %q", n)
		}
	}
	return e, names, nil
}

// AddParameter adds a parameter at the end of the table. Its
// expression may name the parameters already there.
func (sk *Sketch) AddParameter(name, expr string) error {
	if !core.ValidVariableName(name) {
		return fmt.Errorf("%q can't name a parameter: use a letter or underscore, then letters, digits and underscores, and no function or constant name", name)
	}
	if err := sk.nameFree(name); err != nil {
		return err
	}
	expr = cleanText(expr)
	if _, _, err := sk.parseNamed(expr); err != nil {
		return err
	}
	sk.Parameters = append(sk.Parameters, Parameter{Name: name, Expr: expr})
	sk.applyParameters()
	if p := sk.Parameter(name); p.Err != "" {
		err := errors.New(p.Err)
		sk.Parameters = sk.Parameters[:len(sk.Parameters)-1]
		sk.applyParameters()
		return err
	}
	sk.configParameterSet(name, expr)
	return nil
}

// SetParameterExpr gives a parameter a new expression. An expression
// that names an unknown parameter, that makes a cycle or that has no
// value is refused, and the parameter is left as it was.
func (sk *Sketch) SetParameterExpr(name, expr string) error {
	p := sk.Parameter(name)
	if p == nil {
		return fmt.Errorf("no parameter %q", name)
	}
	expr = cleanText(expr)
	if _, _, err := sk.parseNamed(expr); err != nil {
		return err
	}
	old := *p
	p.Expr = expr
	sk.applyParameters()
	if p = sk.Parameter(name); p.Err != "" {
		err := errors.New(p.Err)
		*p = old
		sk.applyParameters()
		return err
	}
	sk.configParameterSet(name, expr)
	return nil
}

// RenameParameter renames a parameter and every use of it, in other
// parameters, in constraints and in groups.
func (sk *Sketch) RenameParameter(from, to string) error {
	p := sk.Parameter(from)
	if p == nil {
		return fmt.Errorf("no parameter %q", from)
	}
	if from == to {
		return nil
	}
	if !core.ValidVariableName(to) {
		return fmt.Errorf("%q can't name a parameter: use a letter or underscore, then letters, digits and underscores, and no function or constant name", to)
	}
	if err := sk.nameFree(to); err != nil {
		return err
	}
	p.Name = to
	sk.forEachExpr(func(expr *string, _ string) {
		*expr = core.RenameVariable(*expr, from, to)
	})
	sk.configParameterRenamed(from, to)
	return nil
}

// DeleteParameter removes a parameter. One that is in use is not
// removed, and the error names what uses it.
func (sk *Sketch) DeleteParameter(name string) error {
	if sk.Parameter(name) == nil {
		return fmt.Errorf("no parameter %q", name)
	}
	if users := append(sk.ParameterUsers(name), sk.configurationUsers(name)...); len(users) > 0 {
		return fmt.Errorf("parameter %q is used by %s", name, strings.Join(users, ", "))
	}
	for i := range sk.Parameters {
		if sk.Parameters[i].Name == name {
			sk.Parameters = append(sk.Parameters[:i], sk.Parameters[i+1:]...)
			break
		}
	}
	sk.configParameterDeleted(name)
	return nil
}

// ParameterUsers describes what names the parameter: other
// parameters, constraints and group values.
func (sk *Sketch) ParameterUsers(name string) []string {
	var users []string
	sk.forEachExpr(func(expr *string, who string) {
		e, err := core.NewFromString(*expr)
		if err != nil {
			return
		}
		if slices.Contains(e.Variables(), name) {
			users = append(users, who)
			return
		}
	})
	return users
}

// forEachExpr calls fn on every stored expression, with a description
// of whose it is.
func (sk *Sketch) forEachExpr(fn func(expr *string, who string)) {
	for i := range sk.Parameters {
		fn(&sk.Parameters[i].Expr, "parameter "+sk.Parameters[i].Name)
	}
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Expr != "" {
			fn(&c.Expr, "constraint "+c.DescriptionString())
		}
	}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		for _, f := range groupValues {
			if _, expr := g.Value(f); *expr != "" {
				fn(expr, fmt.Sprintf("group %s %s", g.Name, g.FieldName(f)))
			}
		}
		for k := range g.EdgeRadii {
			if er := &g.EdgeRadii[k]; er.Expr != "" {
				fn(&er.Expr, fmt.Sprintf("group %s %s", g.Name, EdgeRadiusField(er.Edge)))
			}
		}
	}
}

// applyParameters evaluates the table and the stored expressions after
// an edit, and schedules a regeneration of what changed.
func (sk *Sketch) applyParameters() {
	for _, hg := range sk.evaluate() {
		sk.MarkGroupDirty(hg, false)
	}
}

// EvaluateParameters evaluates the parameter table, then every stored
// expression, and writes the numbers into the values they drive. A
// group whose values changed, or whose constraints' values did, is
// marked dirty with all the groups after it. It reports whether
// anything changed. GenerateAll calls it first.
//
// An expression that cannot be evaluated, for a cycle, an unknown name
// or a value that is not a number, leaves the value it drives as it
// was, and is reported in the parameter's Err or in ExprErrors.
func (sk *Sketch) EvaluateParameters() bool {
	changed := sk.evaluate()
	for _, hg := range changed {
		sk.markDirtyFrom(hg)
	}
	return len(changed) > 0
}

// markDirtyFrom marks hg and the groups after it in GroupOrder dirty,
// as MarkGroupDirty does without scheduling a regeneration.
func (sk *Sketch) markDirtyFrom(hg core.HGroup) {
	on := false
	for _, h := range sk.GroupOrder {
		if h == hg {
			on = true
		}
		if on {
			sk.GetGroup(h).Clean = false
		}
	}
}

// evaluate is EvaluateParameters without marking groups dirty; it
// returns the groups whose values changed.
func (sk *Sketch) evaluate() []core.HGroup {
	vars, constraintErrs := sk.evaluateTable()
	sk.ExprErrors = sk.ExprErrors[:0]
	var changed []core.HGroup
	mark := func(hg core.HGroup) {
		if slices.Contains(changed, hg) {
			return
		}
		changed = append(changed, hg)
	}

	eval := func(text string) (float64, error) {
		e, err := core.NewFromString(text)
		if err != nil {
			return 0, err
		}
		for _, n := range e.Variables() {
			if _, ok := vars[n]; !ok {
				return 0, fmt.Errorf("unknown parameter %q", n)
			}
		}
		v := e.Eval(nil, vars)
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, errors.New("not a number")
		}
		return v, nil
	}

	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Expr == "" || c.Reference {
			continue
		}
		var v float64
		var err error
		if msg, bad := constraintErrs[c.H]; bad {
			err = errors.New(msg)
		} else {
			v, err = eval(c.Expr)
		}
		if err != nil {
			sk.ExprErrors = append(sk.ExprErrors, ExprError{
				Group: c.Group, Constraint: c.H, Field: "value", Expr: c.Expr, Err: err.Error()})
			continue
		}
		if !signedValue(c.Type) {
			v = math.Abs(v)
		}
		if IsLengthValue(c.Type) {
			v *= sk.Units.MmPer()
		}
		if c.ValA != v {
			c.ValA = v
			mark(c.Group)
		}
	}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		for _, f := range groupValues {
			val, expr := g.Value(f)
			if *expr == "" {
				continue
			}
			v, err := eval(*expr)
			if err == nil {
				if g.IsLength(f) {
					v *= sk.Units.MmPer()
				}
				err = checkGroupValue(g, f, v)
			}
			if err != nil {
				sk.ExprErrors = append(sk.ExprErrors, ExprError{
					Group: g.H, Field: g.FieldName(f), Expr: *expr, Err: err.Error()})
				continue
			}
			if *val != v {
				*val = v
				mark(g.H)
			}
		}
		for k := range g.EdgeRadii {
			er := &g.EdgeRadii[k]
			if er.Expr == "" {
				continue
			}
			v, err := eval(er.Expr)
			if err == nil {
				v *= sk.Units.MmPer()
				if !(v > 0) {
					err = errors.New("a fillet's radius must be more than zero")
				}
			}
			if err != nil {
				sk.ExprErrors = append(sk.ExprErrors, ExprError{
					Group: g.H, Field: EdgeRadiusField(er.Edge), Expr: er.Expr, Err: err.Error()})
				continue
			}
			if er.Val != v {
				er.Val = v
				mark(g.H)
			}
		}
	}
	return changed
}

// evaluateTable evaluates every parameter and named constraint after
// the ones it names, and returns the values by name. A parameter in a
// cycle, or naming one that does not exist, keeps its value and gets an
// Err; a named constraint's error is in the second map, by handle.
func (sk *Sketch) evaluateTable() (map[string]float64, map[core.HConstraint]string) {
	// An entry is a parameter or a named constraint: its expression,
	// or none for a constraint's plain number, and where its value and
	// error go.
	type entry struct {
		name   string
		expr   string
		value  float64
		err    string
		e      *core.Expr
		names  []string
		state  int // 0 unvisited, 1 on the stack, 2 done
		param  *Parameter
		constr *core.Constraint
	}
	var entries []entry
	for i := range sk.Parameters {
		p := &sk.Parameters[i]
		entries = append(entries, entry{name: p.Name, expr: p.Expr, value: p.Value, param: p})
	}
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		// A reference dimension measures; read before the solve, its
		// value would chase it.
		if c.Name != "" && !c.Reference && !sk.allDimsReference(c.Group) {
			entries = append(entries, entry{name: c.Name, expr: c.Expr, value: sk.ShownValue(c), constr: c})
		}
	}
	index := make(map[string]int, len(entries))
	for i := range entries {
		en := &entries[i]
		if _, dup := index[en.name]; !dup {
			index[en.name] = i
		}
	}
	for i := range entries {
		en := &entries[i]
		if en.constr != nil && en.expr == "" {
			continue
		}
		e, err := core.NewFromString(en.expr)
		if err != nil {
			en.err = err.Error()
			continue
		}
		en.e = e
		en.names = e.Variables()
		for _, n := range en.names {
			if _, ok := index[n]; !ok {
				en.err = fmt.Sprintf("unknown parameter %q", n)
				break
			}
		}
	}

	vars := make(map[string]float64, len(entries))
	var stack []int
	var visit func(i int)
	visit = func(i int) {
		n := &entries[i]
		if n.state == 2 {
			return
		}
		n.state = 1
		stack = append(stack, i)
		for _, name := range n.names {
			j, ok := index[name]
			if !ok {
				continue
			}
			if entries[j].state == 1 {
				// A cycle: from j's place on the stack to here.
				k := len(stack) - 1
				for stack[k] != j {
					k--
				}
				chain := make([]string, 0, len(stack)-k+1)
				for _, m := range stack[k:] {
					chain = append(chain, entries[m].name)
				}
				chain = append(chain, name)
				msg := "cycle: " + strings.Join(chain, " -> ")
				for _, m := range stack[k:] {
					if entries[m].err == "" {
						entries[m].err = msg
					}
				}
				continue
			}
			visit(j)
		}
		stack = stack[:len(stack)-1]
		n.state = 2
		if n.err == "" && n.e != nil {
			v := n.e.Eval(nil, vars)
			switch {
			case math.IsNaN(v) || math.IsInf(v, 0):
				n.err = "not a number"
			case n.constr != nil:
				n.value = shown(n.constr, v)
			default:
				n.value = v
			}
		}
		vars[n.name] = n.value
	}
	for i := range entries {
		if index[entries[i].name] == i {
			visit(i)
		} else if entries[i].param != nil {
			entries[i].err = fmt.Sprintf("a second parameter %q", entries[i].name)
		} else {
			entries[i].err = fmt.Sprintf("a second name %q", entries[i].name)
		}
	}
	errs := map[core.HConstraint]string{}
	for i := range entries {
		en := &entries[i]
		if en.param != nil {
			en.param.Err = en.err
			if en.err == "" {
				en.param.Value = en.value
			}
		} else if en.err != "" {
			errs[en.constr.H] = en.err
		}
	}
	return vars, errs
}

// signedValue reports whether a constraint's value has a sign, which
// picks the side a point lies on, rather than always being positive.
func signedValue(t core.ConstraintType) bool {
	switch t {
	case core.ConstraintProjPtDistance, core.ConstraintPtLineDistance, core.ConstraintPtFaceDistance,
		core.ConstraintPtPlaneDistance, core.ConstraintLengthDifference, core.ConstraintArcArcDifference,
		core.ConstraintArcLineDifference:
		return true
	}
	return false
}

// IsLengthValue reports whether a constraint's value is a length,
// which the interface shows in its unit, rather than an angle or a
// ratio.
func IsLengthValue(t core.ConstraintType) bool {
	switch t {
	case core.ConstraintAngle, core.ConstraintLengthRatio, core.ConstraintArcArcLenRatio,
		core.ConstraintArcLineLenRatio:
		return false
	}
	return true
}

// SetConstraintValue gives a constraint the value v that the user
// typed: in millimetres, degrees or a ratio, the magnitude the label
// shows. A signed distance keeps its side unless v is negative, which
// flips it; a diameter shown as a radius takes a radius. It drops a
// stored expression.
func SetConstraintValue(c *core.Constraint, v float64) {
	c.Expr = ""
	switch {
	case signedValue(c.Type):
		if c.ValA < 0 {
			c.ValA = -v
		} else {
			c.ValA = v
		}
	case c.Type == core.ConstraintDiameter:
		c.ValA = math.Abs(v)
		if c.Other {
			// Shown as a radius.
			c.ValA *= 2
		}
	default:
		c.ValA = math.Abs(v)
	}
}

// SetConstraintText gives a constraint the value of what the user
// typed for its label, a length in the document's unit (Units). A plain
// number is stored as its value, as SetConstraintValue does; any other
// text is kept and evaluated again at each regeneration, so that "1/4"
// and "wall*2" come back as they were typed. The constraint's group is
// marked dirty.
func (sk *Sketch) SetConstraintText(hc core.HConstraint, text string) error {
	c := sk.GetConstraint(hc)
	if c == nil {
		return fmt.Errorf("no constraint %08x", hc.V)
	}
	if !c.HasLabel() || c.Type == core.ConstraintComment {
		return fmt.Errorf("%s has no value", c.DescriptionString())
	}
	text = cleanText(text)
	_, names, err := sk.parseNamed(text)
	if err != nil {
		return err
	}
	if v, plain := plainNumber(text, names); plain {
		if IsLengthValue(c.Type) {
			v *= sk.Units.MmPer()
		}
		SetConstraintValue(c, v)
	} else {
		stored := text
		switch {
		case signedValue(c.Type) && c.ValA < 0:
			stored = "-(" + text + ")"
		case c.Type == core.ConstraintDiameter && c.Other:
			stored = "2*(" + text + ")"
		}
		old := *c
		c.Expr = stored
		sk.applyParameters()
		for _, ee := range sk.ExprErrors {
			if ee.Constraint == hc {
				*c = old
				sk.applyParameters()
				return errors.New(ee.Err)
			}
		}
	}
	sk.MarkGroupDirty(c.Group, false)
	return nil
}

// ConstraintText is the stored expression of a constraint as its
// label's editor shows it, the inverse of SetConstraintText: without
// the sign of a distance on the far side, and as a radius for a
// diameter shown as one. It is "" for a constraint with no expression.
func ConstraintText(c *core.Constraint) string {
	if c.Expr == "" {
		return ""
	}
	switch {
	case signedValue(c.Type) && c.ValA < 0:
		if inner, ok := unwrap(c.Expr, "-("); ok {
			return inner
		}
		return "-(" + c.Expr + ")"
	case c.Type == core.ConstraintDiameter && c.Other:
		if inner, ok := unwrap(c.Expr, "2*("); ok {
			return inner
		}
		return "(" + c.Expr + ")/2"
	}
	return c.Expr
}

// unwrap returns X for s of the form prefix X ")", when the brackets
// pair up.
func unwrap(s, prefix string) (string, bool) {
	if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, ")") {
		return "", false
	}
	inner := s[len(prefix) : len(s)-1]
	return inner, balanced(inner)
}

// SetGroupText gives a group value what the user typed, as
// SetConstraintText does for a constraint: a pitch is in the
// document's unit. The value is checked as the group screen checks it:
// from 1 to 999 copies, a scale that is not zero. The group is marked
// dirty.
func (sk *Sketch) SetGroupText(hg core.HGroup, f GroupValue, text string) error {
	g := sk.GetGroup(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	text = cleanText(text)
	_, names, err := sk.parseNamed(text)
	if err != nil {
		return err
	}
	val, expr := g.Value(f)
	if v, plain := plainNumber(text, names); plain {
		if g.IsLength(f) {
			v *= sk.Units.MmPer()
		}
		if err := checkGroupValue(g, f, v); err != nil {
			return err
		}
		*val, *expr = v, ""
	} else {
		oldVal, oldExpr := *val, *expr
		*expr = text
		sk.applyParameters()
		for _, ee := range sk.ExprErrors {
			if ee.Group == hg && ee.Constraint.V == 0 && ee.Field == g.FieldName(f) {
				*val, *expr = oldVal, oldExpr
				sk.applyParameters()
				return errors.New(ee.Err)
			}
		}
	}
	sk.MarkGroupDirty(hg, false)
	return nil
}

// cleanText is typed text as it is stored: each run of white space,
// line ends included, made one space, so that a file holds it on one
// line.
func cleanText(s string) string { return strings.Join(strings.Fields(s), " ") }

// plainNumber reports whether typed text that parsed, naming the
// parameters names, is a plain number, and its value.
func plainNumber(text string, names []string) (float64, bool) {
	if len(names) > 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(text, 64)
	return v, err == nil
}

// SetUnits makes u the document's unit, keeping every size: a plain
// value is a number of millimetres and does not change, and a stored
// expression of a length is rewritten to give the same length in u,
// "(1/4)*25.4" from "1/4" in inches to millimetres, or back again
// without the factor. It returns the parameters that lengths are
// computed from, whose numbers now count in u: those the caller should
// tell the user to check.
func (sk *Sketch) SetUnits(u core.Unit) []string {
	from := sk.Units
	if from == u {
		return nil
	}
	used := map[string]bool{}
	convert := func(expr *string) {
		if *expr == "" {
			return
		}
		if e, err := core.NewFromString(*expr); err == nil {
			for _, n := range e.Variables() {
				used[n] = true
				// A named length is read in the new unit; its old
				// value is what the expression was written for.
				if c := sk.NamedConstraint(n); c != nil && IsLengthValue(c.Type) {
					*expr = core.RenameVariable(*expr, n, "("+ConvertLengthExpr(n, u.MmPer(), from.MmPer())+")")
				}
			}
		}
		*expr = ConvertLengthExpr(*expr, from.MmPer(), u.MmPer())
	}
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if IsLengthValue(c.Type) {
			convert(&c.Expr)
		}
	}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		for _, f := range groupValues {
			if g.IsLength(f) {
				_, expr := g.Value(f)
				convert(expr)
			}
		}
		for k := range g.EdgeRadii {
			convert(&g.EdgeRadii[k].Expr)
		}
	}
	sk.Units = u
	// What the lengths name, and what those name in turn.
	for changed := true; changed; {
		changed = false
		for _, p := range sk.Parameters {
			if !used[p.Name] {
				continue
			}
			if e, err := core.NewFromString(p.Expr); err == nil {
				for _, n := range e.Variables() {
					if !used[n] {
						used[n], changed = true, true
					}
				}
			}
		}
	}
	// And what names a named length, whose value changes with the unit.
	for _, p := range sk.Parameters {
		if e, err := core.NewFromString(p.Expr); err == nil {
			for _, n := range e.Variables() {
				if c := sk.NamedConstraint(n); c != nil && IsLengthValue(c.Type) {
					used[p.Name] = true
				}
			}
		}
	}
	var names []string
	for _, p := range sk.Parameters {
		if used[p.Name] {
			names = append(names, p.Name)
		}
	}
	if len(sk.GroupOrder) > 0 {
		sk.MarkGroupDirty(sk.GroupOrder[0], false)
	}
	return names
}

// ConvertLengthExpr rewrites the expression of a length in a unit of fromMm
// millimetres to give the same length in one of toMm: the factor
// between them is appended, or taken off when it is the one a
// conversion the other way appended.
func ConvertLengthExpr(expr string, fromMm, toMm float64) string {
	if fromMm == toMm {
		return expr
	}
	var by string
	if fromMm > toMm {
		by = "*" + strconv.FormatFloat(fromMm/toMm, 'g', -1, 64)
	} else {
		by = "/" + strconv.FormatFloat(toMm/fromMm, 'g', -1, 64)
	}
	// Undo a conversion that went the other way: "(X)*25.4" back to
	// inches is X.
	inverse := map[byte]byte{'*': '/', '/': '*'}
	if strings.HasPrefix(expr, "(") {
		tail := string(inverse[by[0]]) + by[1:]
		if strings.HasSuffix(expr, ")"+tail) {
			inner := expr[1 : len(expr)-len(tail)-1]
			if balanced(inner) {
				return inner
			}
		}
	}
	return "(" + expr + ")" + by
}

// balanced reports whether the brackets of s pair up.
func balanced(s string) bool {
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

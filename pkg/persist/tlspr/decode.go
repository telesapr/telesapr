package tlspr

import (
	"fmt"
	"github.com/telesapr/telesapr/pkg/topo"
	"sort"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Document is a decoded file.
type Document struct {
	Sketch *sketch.Sketch

	// Version is the major version in the header.
	Version int
	// Units is the unit the document was authored in.
	Units core.Unit

	// Warnings are the statements, fields and names the reader did not
	// know and skipped, at their places in the file.
	Warnings []*Error
	// ExprErrors are the stored expressions that could not be
	// evaluated, at their places in the file: a cycle, an unknown name
	// or a value out of range. They are kept as they are, so a save
	// loses nothing; what they drive takes a default value.
	ExprErrors []*Error

	// Mesh and Shell are the generated mesh and shell the file holds
	// for files that link it; Sketch.Entities holds its generated
	// entities.
	Mesh  srf.SMesh
	Shell srf.SShell
}

// VersionError is the error for a file of a newer major version than
// this reader knows.
type VersionError struct {
	File, Reader int
}

func (e *VersionError) Error() string {
	return fmt.Sprintf("the file is format version %d; this TeleSAPR reads up to version %d", e.File, e.Reader)
}

// Decode reads a file. A syntax error or a malformed value of a known
// field is an error; an unknown statement, field or name is skipped and
// reported in Warnings.
func Decode(data []byte) (doc *Document, err error) {
	f, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return DecodeTree(f)
}

// decodeError carries an error out of the decoder's helpers.
type decodeError struct{ err error }

// DecodeTree is Decode for a parsed file.
func DecodeTree(f *File) (doc *Document, err error) {
	d := &decoder{doc: &Document{Sketch: sketch.NewSketch()}}
	defer func() {
		if r := recover(); r != nil {
			de, ok := r.(decodeError)
			if !ok {
				panic(r)
			}
			doc, err = nil, de.err
		}
	}()
	d.file(f)
	return d.doc, nil
}

type decoder struct {
	doc *Document
	// exprPos is where each stored expression was read.
	exprPos map[exprPlace]Pos
}

func (d *decoder) fail(pos Pos, format string, args ...any) {
	panic(decodeError{&Error{pos, fmt.Sprintf(format, args...)}})
}

func (d *decoder) warn(pos Pos, format string, args ...any) {
	d.doc.Warnings = append(d.doc.Warnings, &Error{pos, fmt.Sprintf(format, args...)})
}

func (d *decoder) file(f *File) {
	h := f.Nodes[0]
	args := d.fieldArgs(h, 1)
	v, err := strconv.Atoi(args[0].Text)
	if err != nil || args[0].Quoted || v < 1 {
		d.fail(args[0].Pos, "bad format version %q", args[0].Text)
	}
	if v > Version {
		panic(decodeError{&VersionError{File: v, Reader: Version}})
	}
	d.doc.Version = v

	sk := d.doc.Sketch
	for _, n := range f.Nodes[1:] {
		switch n.Name {
		case "units":
			a := d.fieldArgs(n, 1)
			if u, ok := enumValue(d, docUnits, a[0]); ok {
				d.doc.Units = u
			}
		case "active":
			sk.Active = core.HGroup{V: d.handleField(n)}
		case "group":
			if g, ok := d.group(n); ok {
				sk.Groups = append(sk.Groups, g)
			}
		case "request":
			if r, ok := d.request(n); ok {
				sk.Requests = append(sk.Requests, r)
			}
		case "constraint":
			if c, ok := d.constraint(n); ok {
				sk.Constraints = append(sk.Constraints, c)
			}
		case "style":
			sk.Styles = append(sk.Styles, d.style(n))
		case "value":
			a := d.fieldArgs(n, 2)
			p := core.Param{H: core.HParam{V: d.handle(a[0])}, Val: d.real(a[1])}
			// As in .slvs, a parameter is free by its handle.
			if p.H.V&0x40000000 != 0 {
				p.Free = true
			} else {
				p.Known = true
			}
			sk.Params = append(sk.Params, p)
		case "entity":
			if e, ok := d.entity(n); ok {
				sk.Entities = append(sk.Entities, e)
			}
		case "mesh":
			d.mesh(n)
		case "shell":
			d.shell(n)
		case "telesapr":
			d.fail(n.Pos, "a second header")
		case "param":
			d.param(n)
		default:
			d.warn(n.Pos, "unknown statement %q; skipped", n.Name)
		}
	}

	// Lists are kept in handle order, as C++ IdLists are; groups are
	// solved in their order.
	sort.SliceStable(sk.Groups, func(i, j int) bool { return sk.Groups[i].H.V < sk.Groups[j].H.V })
	sk.GroupOrder = sk.GroupOrder[:0]
	for i := range sk.Groups {
		sk.GroupOrder = append(sk.GroupOrder, sk.Groups[i].H)
	}
	sort.SliceStable(sk.GroupOrder, func(i, j int) bool {
		return sk.GetGroup(sk.GroupOrder[i]).Order < sk.GetGroup(sk.GroupOrder[j]).Order
	})
	sort.SliceStable(sk.Requests, func(i, j int) bool { return sk.Requests[i].H.V < sk.Requests[j].H.V })
	sort.SliceStable(sk.Constraints, func(i, j int) bool { return sk.Constraints[i].H.V < sk.Constraints[j].H.V })
	sort.SliceStable(sk.Styles, func(i, j int) bool { return sk.Styles[i].H.V < sk.Styles[j].H.V })
	sort.SliceStable(sk.Params, func(i, j int) bool { return sk.Params[i].H.V < sk.Params[j].H.V })
	sort.SliceStable(sk.Entities, func(i, j int) bool { return sk.Entities[i].H.V < sk.Entities[j].H.V })
	d.lengths()
	d.evaluate()
}

// fieldArgs checks that n is a plain field with count values, and
// returns them.
func (d *decoder) fieldArgs(n *Node, count int) []Value {
	d.plain(n)
	if len(n.Args) != count {
		d.fail(n.Pos, "%q takes %d values, not %d", n.Name, count, len(n.Args))
	}
	return n.Args
}

// plain checks that n is neither a block nor an expression.
func (d *decoder) plain(n *Node) {
	if n.Block {
		d.fail(n.Pos, "%q is not a block", n.Name)
	}
	if n.HasExpr {
		d.fail(n.Pos, "%q takes no expression", n.Name)
	}
}

// head checks that n is a block with count values before its brace.
func (d *decoder) head(n *Node, count int) []Value {
	if !n.Block {
		d.fail(n.Pos, "%q must be a block", n.Name)
	}
	if n.HasExpr {
		d.fail(n.Pos, "%q takes no expression", n.Name)
	}
	if len(n.Args) != count {
		d.fail(n.Pos, "%q takes %d values, not %d", n.Name, count, len(n.Args))
	}
	return n.Args
}

func (d *decoder) word(v Value) string {
	if v.Quoted {
		d.fail(v.Pos, "expected a name or number, not a string")
	}
	return v.Text
}

func (d *decoder) handle(v Value) uint32 {
	s := d.word(v)
	if !strings.HasPrefix(s, "0x") {
		d.fail(v.Pos, "expected a handle such as 0x1f, not %q", s)
	}
	h, err := strconv.ParseUint(s[2:], 16, 32)
	if err != nil {
		d.fail(v.Pos, "bad handle %q", s)
	}
	return uint32(h)
}

func (d *decoder) int(v Value) int {
	i, err := strconv.ParseInt(d.word(v), 10, 32)
	if err != nil {
		d.fail(v.Pos, "expected an integer, not %q", v.Text)
	}
	return int(i)
}

func (d *decoder) real(v Value) float64 {
	return d.parseReal(v.Pos, d.word(v))
}

func (d *decoder) parseReal(pos Pos, s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); !ok || ne.Err != strconv.ErrRange {
			d.fail(pos, "expected a number, not %q", s)
		}
	}
	return f
}

func (d *decoder) str(v Value) string {
	if !v.Quoted {
		d.fail(v.Pos, "expected a string in quotes, not %q", v.Text)
	}
	return v.Text
}

// expr reads a value that may hold an expression: a number, or the
// text of an expression, which a later evaluation turns into the
// number. where names the value for the evaluation's warnings.
func (d *decoder) expr(n *Node, where exprPlace) (float64, string) {
	if !n.HasExpr || n.Block || len(n.Args) != 0 {
		d.fail(n.Pos, "%q is written as '%s = <expression>'", n.Name, n.Name)
	}
	if _, err := strconv.ParseFloat(n.Expr, 64); err == nil || isRangeError(err) {
		return d.parseReal(n.ExprPos, n.Expr), ""
	}
	if _, err := core.NewFromString(n.Expr); err != nil {
		d.fail(n.ExprPos, "%q is not a valid expression: %v", n.Expr, err)
	}
	if d.exprPos == nil {
		d.exprPos = map[exprPlace]Pos{}
	}
	d.exprPos[where] = n.ExprPos
	return 0, n.Expr
}

func isRangeError(err error) bool {
	ne, ok := err.(*strconv.NumError)
	return ok && ne.Err == strconv.ErrRange
}

// exprPlace is where a stored expression is: a parameter by name, a
// constraint's value, or a group's value by its field name.
type exprPlace struct {
	param      string
	group      core.HGroup
	constraint core.HConstraint
	field      string
}

// param reads "param <name> = <expression>".
func (d *decoder) param(n *Node) {
	if !n.HasExpr || n.Block || len(n.Args) != 1 || n.Args[0].Quoted {
		d.fail(n.Pos, "a parameter is written as 'param <name> = <expression>'")
	}
	name := n.Args[0].Text
	if !core.ValidVariableName(name) {
		d.fail(n.Args[0].Pos, "%q can't name a parameter", name)
	}
	sk := d.doc.Sketch
	if sk.Parameter(name) != nil {
		d.fail(n.Args[0].Pos, "a second parameter %q", name)
	}
	if _, err := core.NewFromString(n.Expr); err != nil {
		d.fail(n.ExprPos, "%q is not a valid expression: %v", n.Expr, err)
	}
	if d.exprPos == nil {
		d.exprPos = map[exprPlace]Pos{}
	}
	d.exprPos[exprPlace{param: name}] = n.ExprPos
	sk.Parameters = append(sk.Parameters, sketch.Parameter{Name: name, Expr: n.Expr})
}

// lengths puts the lengths the user typed into the document's unit,
// once every statement is read. Version 1 stored them in millimetres:
// a number is already what the sketch holds, and an expression is
// rewritten to give the same length in the document's unit. From
// version 2 they are in the document's unit: a number is turned into
// millimetres, and an expression is evaluated in the unit.
func (d *decoder) lengths() {
	sk := d.doc.Sketch
	sk.Units = d.doc.Units
	mmPer := sk.Units.MmPer()
	if mmPer == 1 {
		return
	}
	length := func(val *float64, expr *string) {
		switch {
		case d.doc.Version >= 2 && *expr == "":
			*val *= mmPer
		case d.doc.Version < 2 && *expr != "":
			*expr = sketch.ConvertLengthExpr(*expr, 1, mmPer)
		}
	}
	for i := range sk.Constraints {
		if c := &sk.Constraints[i]; sketch.IsLengthValue(c.Type) {
			length(&c.ValA, &c.Expr)
		}
	}
	for i := range sk.Groups {
		g := &sk.Groups[i]
		length(&g.ValB, &g.ExprB)
		for k := range g.EdgeRadii {
			length(&g.EdgeRadii[k].Val, &g.EdgeRadii[k].Expr)
		}
	}
}

// evaluate gives the stored expressions their values, and reports
// those it could not evaluate, at their places in the file.
func (d *decoder) evaluate() {
	sk := d.doc.Sketch
	if d.exprPos == nil {
		return
	}
	report := func(pos Pos, format string, args ...any) {
		d.doc.ExprErrors = append(d.doc.ExprErrors, &Error{pos, fmt.Sprintf(format, args...)})
	}
	sk.EvaluateParameters()
	for _, p := range sk.Parameters {
		if p.Err != "" {
			report(d.exprPos[exprPlace{param: p.Name}], "parameter %s: %s", p.Name, p.Err)
		}
	}
	for _, e := range sk.ExprErrors {
		at := exprPlace{constraint: e.Constraint}
		if e.Constraint.V == 0 {
			at = exprPlace{group: e.Group, field: e.Field}
			if g := sk.GetGroup(e.Group); g != nil && e.Field == "copies" {
				// No copies is no group a user can make.
				g.ValA = 1
			}
		}
		report(d.exprPos[at], "%s", e.Error())
	}
}

func (d *decoder) flag(n *Node) bool {
	d.fieldArgs(n, 0)
	return true
}

func (d *decoder) handleField(n *Node) uint32 { return d.handle(d.fieldArgs(n, 1)[0]) }

func (d *decoder) intField(n *Node) int { return d.int(d.fieldArgs(n, 1)[0]) }

func (d *decoder) realField(n *Node) float64 { return d.real(d.fieldArgs(n, 1)[0]) }

func (d *decoder) strField(n *Node) string { return d.str(d.fieldArgs(n, 1)[0]) }

func (d *decoder) vector(args []Value) geom.Vector {
	return geom.Vector{X: d.real(args[0]), Y: d.real(args[1]), Z: d.real(args[2])}
}

func (d *decoder) vectorField(n *Node) geom.Vector { return d.vector(d.fieldArgs(n, 3)) }

func (d *decoder) quaternionField(n *Node) geom.Quaternion {
	a := d.fieldArgs(n, 4)
	return geom.Quaternion{W: d.real(a[0]), X: d.real(a[1]), Y: d.real(a[2]), Z: d.real(a[3])}
}

func (d *decoder) color(args []Value) core.RgbaColor {
	var c [4]uint8
	for i := range c {
		v := d.int(args[i])
		if v < 0 || v > 255 {
			d.fail(args[i].Pos, "a colour component is 0 to 255, not %d", v)
		}
		c[i] = uint8(v)
	}
	return core.RgbaColor{Red: c[0], Green: c[1], Blue: c[2], Alpha: c[3]}
}

func (d *decoder) colorField(n *Node) core.RgbaColor { return d.color(d.fieldArgs(n, 4)) }

// enumValue reads a name of e; an unknown name is reported, and false.
func enumValue[T comparable](d *decoder, e enum[T], v Value) (T, bool) {
	s := d.word(v)
	t, ok := e.values[s]
	if !ok {
		d.warn(v.Pos, "unknown %s %q", e.what, s)
	}
	return t, ok
}

// enumField reads a field holding one name of e into *dst, leaving it
// alone for an unknown name.
func enumField[T comparable](d *decoder, e enum[T], n *Node, dst *T) {
	if t, ok := enumValue(d, e, d.fieldArgs(n, 1)[0]); ok {
		*dst = t
	}
}

func (d *decoder) unknownField(n *Node, in string) {
	d.warn(n.Pos, "unknown field %q in %s; skipped", n.Name, in)
}

func (d *decoder) group(n *Node) (sketch.Group, bool) {
	a := d.head(n, 3)
	g := sketch.Group{H: core.HGroup{V: d.handle(a[0])}, Scale: 1, Remap: make(core.EntityMap)}
	typ, ok := enumValue(d, groupTypes, a[1])
	g.Type = typ
	g.Name = d.str(a[2])
	for _, c := range n.Children {
		switch c.Name {
		case "order":
			g.Order = d.intField(c)
		case "subtype":
			enumField(d, groupSubtypes, c, &g.Subtype)
		case "workplane":
			g.ActiveWorkplane = core.HEntity{V: d.handleField(c)}
		case "op-a":
			g.OpA = core.HGroup{V: d.handleField(c)}
		case "op-b":
			g.OpB = core.HGroup{V: d.handleField(c)}
		case "copies", "turn":
			g.ValA, g.ExprA = d.expr(c, exprPlace{group: g.H, field: c.Name})
		case "pitch", "distance", "radius", "thickness":
			g.ValB, g.ExprB = d.expr(c, exprPlace{group: g.H, field: c.Name})
		case "val-c", "draft":
			g.ValC, g.ExprC = d.expr(c, exprPlace{group: g.H, field: c.Name})
		case "color":
			g.Color = d.colorField(c)
		case "skip-first":
			g.SkipFirst = d.flag(c)
		case "mirror-solid":
			g.MirrorSolid = d.flag(c)
		case "combine":
			enumField(d, combines, c, &g.MeshCombine)
		case "force-to-mesh":
			g.ForceToMesh = d.flag(c)
		case "edges":
			d.plain(c)
			for _, v := range c.Args {
				g.Edges = append(g.Edges, core.HEntity{V: d.handle(v)})
			}
		case "solid-edge":
			d.plain(c)
			if len(c.Args) != 3 && len(c.Args) != 5 {
				d.fail(c.Pos, "solid-edge takes two face handles, an ordinal, and 'of' with a count")
			}
			n := d.int(c.Args[2])
			if n < 0 {
				d.fail(c.Args[2].Pos, "an edge's ordinal is not negative")
			}
			name := topo.MakeName(srf.FaceID(d.handle(c.Args[0])), srf.FaceID(d.handle(c.Args[1])), n)
			if len(c.Args) == 5 {
				if d.word(c.Args[3]) != "of" {
					d.fail(c.Args[3].Pos, "solid-edge's count follows the word 'of'")
				}
				name.Of = d.int(c.Args[4])
				if name.Of <= n {
					d.fail(c.Args[4].Pos, "an edge's ordinal is below the count of edges")
				}
			}
			g.SolidEdges = append(g.SolidEdges, name)
		case "edge-radius":
			// "edge-radius <n> = <expression>": the fillet's n-th edge,
			// counting its entities and then its solid edges.
			if !c.HasExpr || c.Block || len(c.Args) != 1 {
				d.fail(c.Pos, "an edge's radius is written as 'edge-radius <n> = <expression>'")
			}
			i := d.int(c.Args[0]) - 1
			if i < 0 {
				d.fail(c.Args[0].Pos, "edges count from 1")
			}
			for _, er := range g.EdgeRadii {
				if er.Edge == i {
					d.fail(c.Pos, "a second radius for edge %d", i+1)
				}
			}
			at := *c
			at.Args = nil
			val, expr := d.expr(&at, exprPlace{group: g.H, field: sketch.EdgeRadiusField(i)})
			g.EdgeRadii = append(g.EdgeRadii, sketch.EdgeRadius{Edge: i, Val: val, Expr: expr})
		case "solid-face":
			d.plain(c)
			if len(c.Args) != 1 {
				d.fail(c.Pos, "solid-face takes a face handle")
			}
			g.SolidFaces = append(g.SolidFaces, srf.FaceID(d.handle(c.Args[0])))
		case "predef":
			d.head(c, 0)
			for _, p := range c.Children {
				switch p.Name {
				case "q":
					g.Predef.Q = d.quaternionField(p)
				case "origin":
					g.Predef.Origin = core.HEntity{V: d.handleField(p)}
				case "entity-b":
					g.Predef.EntityB = core.HEntity{V: d.handleField(p)}
				case "entity-c":
					g.Predef.EntityC = core.HEntity{V: d.handleField(p)}
				case "swap-uv":
					g.Predef.SwapUV = d.flag(p)
				case "negate-u":
					g.Predef.NegateU = d.flag(p)
				case "negate-v":
					g.Predef.NegateV = d.flag(p)
				default:
					d.unknownField(p, "predef")
				}
			}
		case "visible":
			g.Visible = d.flag(c)
		case "suppress":
			g.Suppress = d.flag(c)
		case "suppress-all":
			g.SuppressAll = d.flag(c)
		case "relax-constraints":
			g.RelaxConstraints = d.flag(c)
		case "allow-redundant":
			g.AllowRedundant = d.flag(c)
		case "suppress-dof":
			g.SuppressDofCalculation = d.flag(c)
		case "all-dims-reference":
			g.AllDimsReference = d.flag(c)
		case "scale":
			// A plain number is written as a field; an expression
			// with "=".
			if c.HasExpr {
				g.Scale, g.ExprScale = d.expr(c, exprPlace{group: g.H, field: c.Name})
			} else {
				g.Scale = d.realField(c)
			}
		case "link":
			g.LinkFile = d.strField(c)
		case "remap":
			d.head(c, 0)
			for _, m := range c.Children {
				if m.Name != "map" {
					d.unknownField(m, "remap")
					continue
				}
				ma := d.fieldArgs(m, 3)
				idx := d.int(ma[0])
				key := core.EntityKey{Input: core.HEntity{V: d.handle(ma[1])}, CopyNumber: d.int(ma[2])}
				if idx < 0 {
					d.fail(ma[0].Pos, "a remap index is not negative")
				}
				g.Remap[key] = core.EntityID{V: uint32(idx)}
			}
		default:
			d.unknownField(c, "group")
		}
	}
	if !ok {
		d.warn(n.Pos, "group %s skipped", a[0].Text)
	}
	return g, ok
}

func (d *decoder) request(n *Node) (sketch.Request, bool) {
	a := d.head(n, 2)
	r := sketch.Request{H: core.HRequest{V: d.handle(a[0])}, Workplane: core.FreeIn3D}
	typ, ok := enumValue(d, requestTypes, a[1])
	r.Type = typ
	for _, c := range n.Children {
		switch c.Name {
		case "group":
			r.Group = core.HGroup{V: d.handleField(c)}
		case "workplane":
			r.Workplane = core.HEntity{V: d.handleField(c)}
		case "construction":
			r.Construction = d.flag(c)
		case "style":
			r.Style = core.HStyle{V: d.handleField(c)}
		case "extra-points":
			r.ExtraPoints = d.intField(c)
		case "text":
			r.Str = d.strField(c)
		case "font":
			r.Font = d.strField(c)
		case "file":
			r.File = d.strField(c)
		case "aspect-ratio":
			r.AspectRatio = d.realField(c)
		default:
			d.unknownField(c, "request")
		}
	}
	if !ok {
		d.warn(n.Pos, "request %s skipped", a[0].Text)
	}
	return r, ok
}

func (d *decoder) constraint(n *Node) (core.Constraint, bool) {
	a := d.head(n, 2)
	c := core.Constraint{}
	c.H = core.HConstraint{V: d.handle(a[0])}
	typ, ok := enumValue(d, constraintTypes, a[1])
	c.Type = typ
	for _, f := range n.Children {
		switch f.Name {
		case "group":
			c.Group = core.HGroup{V: d.handleField(f)}
		case "workplane":
			c.Workplane = core.HEntity{V: d.handleField(f)}
		case "value":
			c.ValA, c.Expr = d.expr(f, exprPlace{constraint: c.H})
		case "param":
			c.ValP = core.HParam{V: d.handleField(f)}
		case "pt-a":
			c.PtA = core.HEntity{V: d.handleField(f)}
		case "pt-b":
			c.PtB = core.HEntity{V: d.handleField(f)}
		case "entity-a":
			c.EntityA = core.HEntity{V: d.handleField(f)}
		case "entity-b":
			c.EntityB = core.HEntity{V: d.handleField(f)}
		case "entity-c":
			c.EntityC = core.HEntity{V: d.handleField(f)}
		case "entity-d":
			c.EntityD = core.HEntity{V: d.handleField(f)}
		case "other":
			c.Other = d.flag(f)
		case "other2":
			c.Other2 = d.flag(f)
		case "reference":
			c.Reference = d.flag(f)
		case "comment":
			c.Comment = d.strField(f)
		case "label-offset":
			c.Disp.Offset = d.vectorField(f)
		case "label-style":
			c.Disp.Style = core.HStyle{V: d.handleField(f)}
		default:
			d.unknownField(f, "constraint")
		}
	}
	if !ok {
		d.warn(n.Pos, "constraint %s skipped", a[0].Text)
	}
	return c, ok
}

func (d *decoder) style(n *Node) core.Style {
	a := d.head(n, 2)
	s := core.Style{H: core.HStyle{V: d.handle(a[0])}, Name: d.str(a[1])}
	if s.H.V < core.StyleFirstCustom {
		d.fail(a[0].Pos, "style %s is a built-in style, which is not saved", a[0].Text)
	}
	for _, c := range n.Children {
		switch c.Name {
		case "width":
			s.Width = d.realField(c)
		case "width-as":
			enumField(d, unitsAs, c, &s.WidthAs)
		case "text-height":
			s.TextHeight = d.realField(c)
		case "text-height-as":
			enumField(d, unitsAs, c, &s.TextHeightAs)
		case "text-angle":
			s.TextAngle = d.realField(c)
		case "text-origin":
			d.plain(c)
			for _, v := range c.Args {
				if o, ok := enumValue(d, textOrigins, v); ok {
					s.TextOrigin |= o
				}
			}
		case "color":
			s.Color = d.colorField(c)
		case "fill-color":
			s.FillColor = d.colorField(c)
		case "filled":
			s.Filled = d.flag(c)
		case "visible":
			s.Visible = d.flag(c)
		case "exportable":
			s.Exportable = d.flag(c)
		case "stipple":
			enumField(d, stipples, c, &s.StippleType)
		case "stipple-scale":
			s.StippleScale = d.realField(c)
		default:
			d.unknownField(c, "style")
		}
	}
	return s
}

func (d *decoder) entity(n *Node) (core.Entity, bool) {
	a := d.head(n, 2)
	e := core.Entity{}
	e.H = core.HEntity{V: d.handle(a[0])}
	typ, ok := enumValue(d, entityTypes, a[1])
	e.Type = typ
	for _, c := range n.Children {
		switch c.Name {
		case "group":
			e.Group = core.HGroup{V: d.handleField(c)}
		case "workplane":
			e.Workplane = core.HEntity{V: d.handleField(c)}
		case "construction":
			e.Construction = d.flag(c)
		case "style":
			e.Style = core.HStyle{V: d.handleField(c)}
		case "text":
			e.Str = d.strField(c)
		case "font":
			e.Font = d.strField(c)
		case "file":
			e.File = d.strField(c)
		case "points":
			d.plain(c)
			if len(c.Args) > len(e.Point) {
				d.fail(c.Pos, "an entity has at most %d points", len(e.Point))
			}
			for i, v := range c.Args {
				e.Point[i] = core.HEntity{V: d.handle(v)}
			}
		case "extra-points":
			e.ExtraPoints = d.intField(c)
		case "normal":
			e.Normal = core.HEntity{V: d.handleField(c)}
		case "distance":
			e.Distance = core.HEntity{V: d.handleField(c)}
		case "act-point":
			e.ActPoint = d.vectorField(c)
		case "act-normal":
			e.ActNormal = d.quaternionField(c)
		case "act-distance":
			e.ActDistance = d.realField(c)
		case "act-visible":
			e.ActVisible = d.flag(c)
		default:
			d.unknownField(c, "entity")
		}
	}
	if !ok {
		d.warn(n.Pos, "entity %s skipped", a[0].Text)
	}
	return e, ok
}

func (d *decoder) mesh(n *Node) {
	d.head(n, 0)
	for _, c := range n.Children {
		if c.Name != "triangle" {
			d.unknownField(c, "mesh")
			continue
		}
		a := d.fieldArgs(c, 14)
		meta := srf.STriMeta{Face: srf.FaceID(d.handle(a[0])), RgbaColor: d.color(a[1:5])}
		d.doc.Mesh.AddTriangle3(meta, d.vector(a[5:8]), d.vector(a[8:11]), d.vector(a[11:14]))
	}
}

func (d *decoder) shell(n *Node) {
	d.head(n, 0)
	sh := &d.doc.Shell
	for _, c := range n.Children {
		switch c.Name {
		case "surface":
			sh.Surface = append(sh.Surface, d.surface(c))
		case "curve":
			sh.Curve = append(sh.Curve, d.curve(c))
		default:
			d.unknownField(c, "shell")
		}
	}
}

func (d *decoder) surface(n *Node) srf.SSurface {
	a := d.head(n, 1)
	sf := srf.SSurface{H: core.HSSurface{V: d.handle(a[0])}, TrimBuilt: true}
	for _, c := range n.Children {
		switch c.Name {
		case "color":
			sf.Color = d.colorField(c)
		case "face":
			sf.Face = srf.FaceID(d.handleField(c))
		case "sheet":
			sf.Sheet = d.handleField(c)
		case "degree":
			da := d.fieldArgs(c, 2)
			sf.DegM, sf.DegN = d.degree(da[0], len(sf.Ctrl)-1), d.degree(da[1], len(sf.Ctrl[0])-1)
		case "ctrl":
			ca := d.fieldArgs(c, 6)
			i, j := d.degree(ca[0], len(sf.Ctrl)-1), d.degree(ca[1], len(sf.Ctrl[0])-1)
			sf.Ctrl[i][j] = d.vector(ca[2:5])
			sf.Weight[i][j] = d.real(ca[5])
		case "trim":
			ta := d.fieldArgs(c, 8)
			t := srf.STrimBy{Curve: core.HSCurve{V: d.handle(ta[0])}}
			switch d.word(ta[1]) {
			case "forward":
			case "backward":
				t.Backwards = true
			default:
				d.fail(ta[1].Pos, "a trim runs forward or backward, not %q", ta[1].Text)
			}
			t.StartP, t.FinishP = d.vector(ta[2:5]), d.vector(ta[5:8])
			sf.Trim = append(sf.Trim, t)
		default:
			d.unknownField(c, "surface")
		}
	}
	return sf
}

func (d *decoder) curve(n *Node) srf.SCurve {
	a := d.head(n, 1)
	sc := srf.SCurve{H: core.HSCurve{V: d.handle(a[0])}}
	for _, c := range n.Children {
		switch c.Name {
		case "exact":
			sc.IsExact = d.flag(c)
		case "degree":
			sc.Exact.Degree = d.degree(d.fieldArgs(c, 1)[0], len(sc.Exact.Ctrl)-1)
		case "surfaces":
			sa := d.fieldArgs(c, 2)
			sc.SurfA, sc.SurfB = core.HSSurface{V: d.handle(sa[0])}, core.HSSurface{V: d.handle(sa[1])}
		case "ctrl":
			ca := d.fieldArgs(c, 5)
			i := d.degree(ca[0], len(sc.Exact.Ctrl)-1)
			sc.Exact.Ctrl[i] = d.vector(ca[1:4])
			sc.Exact.Weight[i] = d.real(ca[4])
		case "point", "vertex":
			sc.Points = append(sc.Points, srf.SCurvePt{Vertex: c.Name == "vertex", P: d.vectorField(c)})
		default:
			d.unknownField(c, "curve")
		}
	}
	return sc
}

// degree reads an index or degree from 0 to max.
func (d *decoder) degree(v Value, max int) int {
	i := d.int(v)
	if i < 0 || i > max {
		d.fail(v.Pos, "%d is out of range 0 to %d", i, max)
	}
	return i
}

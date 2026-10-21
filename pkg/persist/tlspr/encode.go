package tlspr

import (
	"context"
	"math"
	"runtime/trace"
	"slices"
	"sort"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Version is the major version of the format this package writes, and
// the newest it reads.
const Version = 2

// Options are what a save records beside the sketch; none yet. The
// document's unit is the sketch's own (Sketch.Units).
type Options struct{}

// Encode writes a sketch the caller has just regenerated, as a user
// interface does through its own GenerateAll. Each entity's numeric
// values and visibility are refreshed first, as for export, so the
// sketch changes in those fields only.
func Encode(sk *sketch.Sketch, o Options) []byte {
	defer trace.StartRegion(context.Background(), "save").End()
	for i := range sk.Entities {
		e := &sk.Entities[i]
		func() {
			defer func() { _ = recover() }()
			e.CalculateNumerical(sk, true)
		}()
	}
	return Print(EncodeTree(sk, o))
}

// EncodeTree is Encode without the refresh, as a tree.
func EncodeTree(sk *sketch.Sketch, o Options) *File {
	// What a file linking this one reads: the last group's mesh or
	// shell.
	var m *srf.SMesh
	var sh *srf.SShell
	if len(sk.GroupOrder) > 0 {
		if g := sk.Group(sk.GroupOrder[len(sk.GroupOrder)-1]); g != nil {
			m, sh = &g.RunningMesh, &g.RunningShell
		}
	}
	return encodeTree(sk, o, m, sh)
}

// Encode writes the document as it was decoded, without regenerating
// it: its saved entities, mesh and shell as the file held them. A
// document decoded from a file this package wrote encodes to the same
// bytes.
func (d *Document) Encode() []byte {
	return Print(encodeTree(d.Sketch, Options{}, &d.Mesh, &d.Shell))
}

func encodeTree(sk *sketch.Sketch, o Options, m *srf.SMesh, sh *srf.SShell) *File {
	f := &File{}
	add := func(n *Node) { f.Nodes = append(f.Nodes, n) }
	add(field("telesapr", Word(strconv.Itoa(Version))))
	add(field("units", Word(enumName(docUnits, sk.Units))))
	// The active group, when it is not the last: what is shown, and so
	// each entity's act-visible, follows it.
	if a := sk.Active; a.V != 0 && sk.Group(a) != nil && len(sk.GroupOrder) > 0 &&
		a != sk.GroupOrder[len(sk.GroupOrder)-1] {
		add(field("active", hex(a.V)))
	}
	mmPer := sk.Units.MmPer()
	for _, p := range sk.Parameters {
		add(&Node{Name: "param", Args: []Value{Word(p.Name)}, HasExpr: true, Expr: p.Expr})
	}
	// Each configuration's expressions, in the table's order, then any
	// of parameters the table no longer has.
	for _, c := range sk.Configurations {
		n := block("configuration", String(c.Name))
		seen := map[string]bool{}
		for _, p := range sk.Parameters {
			if e, ok := c.Exprs[p.Name]; ok {
				n.add(&Node{Name: "param", Args: []Value{Word(p.Name)}, HasExpr: true, Expr: e})
				seen[p.Name] = true
			}
		}
		var rest []string
		for k := range c.Exprs {
			if !seen[k] {
				rest = append(rest, k)
			}
		}
		slices.Sort(rest)
		for _, k := range rest {
			n.add(&Node{Name: "param", Args: []Value{Word(k)}, HasExpr: true, Expr: c.Exprs[k]})
		}
		add(n)
	}
	if sk.ActiveConfiguration != "" {
		add(field("active-configuration", String(sk.ActiveConfiguration)))
	}
	if d := sk.Drawing; d != nil {
		n := block("drawing")
		n.add(field("sheet", Word(d.Sheet)))
		n.flag("portrait", d.Portrait)
		if d.Scale != 0 {
			n.add(field("scale", Word(FormatReal(d.Scale))))
		}
		n.flag("first-angle", d.FirstAngle)
		n.flag("hidden", d.Hidden)
		for _, v := range d.Views {
			n.add(field("view", Word(v.Kind), Word(FormatReal(v.X)), Word(FormatReal(v.Y))))
		}
		for _, s := range d.Sections {
			n.add(field("section", Word(s.View), Word(FormatReal(s.Offset)), Word(FormatReal(s.X)), Word(FormatReal(s.Y))))
		}
		for _, dm := range d.Dimensions {
			c := block("dimension", Word(dm.View), Word(dm.Kind), Word(FormatReal(dm.Offset)))
			for _, e := range dm.Edges {
				c.add(solidEdgeField(e))
			}
			for _, f := range dm.Faces {
				c.add(field("solid-face", hex(uint32(f))))
			}
			n.add(c)
		}
		for _, dt := range d.Details {
			n.add(field("detail", Word(dt.View), Word(FormatReal(dt.CX)), Word(FormatReal(dt.CY)), Word(FormatReal(dt.R)),
				Word(FormatReal(dt.Scale)), Word(FormatReal(dt.X)), Word(FormatReal(dt.Y))))
		}
		n.str("title", d.Title)
		n.str("author", d.Author)
		n.str("number", d.Number)
		n.str("revision", d.Revision)
		n.str("material", d.Material)
		n.str("company", d.Company)
		add(n)
	}

	groups := make([]*sketch.Group, len(sk.Groups))
	for i := range sk.Groups {
		groups[i] = &sk.Groups[i]
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Order != groups[j].Order {
			return groups[i].Order < groups[j].Order
		}
		return groups[i].H.V < groups[j].H.V
	})
	for _, g := range groups {
		add(encodeGroup(g, mmPer))
	}

	for _, i := range sortedBy(len(sk.Requests), func(i int) uint32 { return sk.Requests[i].H.V }) {
		add(encodeRequest(&sk.Requests[i]))
	}
	for _, i := range sortedBy(len(sk.Constraints), func(i int) uint32 { return sk.Constraints[i].H.V }) {
		add(encodeConstraint(&sk.Constraints[i], mmPer))
	}
	for _, i := range sortedBy(len(sk.Styles), func(i int) uint32 { return sk.Styles[i].H.V }) {
		if sk.Styles[i].H.V >= core.StyleFirstCustom {
			add(encodeStyle(&sk.Styles[i]))
		}
	}
	for _, i := range sortedBy(len(sk.Params), func(i int) uint32 { return sk.Params[i].H.V }) {
		p := &sk.Params[i]
		add(field("value", hex(p.H.V), num(p.Val)))
	}
	for _, i := range sortedBy(len(sk.Entities), func(i int) uint32 { return sk.Entities[i].H.V }) {
		add(encodeEntity(&sk.Entities[i]))
	}

	if m != nil && len(m.Triangles) > 0 {
		add(encodeMesh(m))
	}
	if sh != nil && (len(sh.Surface) > 0 || len(sh.Curve) > 0) {
		add(encodeShell(sh))
	}
	return f
}

// sortedBy returns the indices 0..n-1 ordered by key, ties in order.
func sortedBy(n int, key func(int) uint32) []int {
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return key(idx[a]) < key(idx[b]) })
	return idx
}

func field(name string, args ...Value) *Node { return &Node{Name: name, Args: args} }

func block(name string, args ...Value) *Node { return &Node{Name: name, Args: args, Block: true} }

func (n *Node) add(c *Node) { n.Children = append(n.Children, c) }

func (n *Node) flag(name string, v bool) {
	if v {
		n.add(field(name))
	}
}

func (n *Node) handle(name string, v uint32) {
	if v != 0 {
		n.add(field(name, hex(v)))
	}
}

func (n *Node) int(name string, v int) {
	if v != 0 {
		n.add(field(name, Word(strconv.Itoa(v))))
	}
}

func (n *Node) real(name string, v float64) {
	if !isPlusZero(v) {
		n.add(field(name, num(v)))
	}
}

// expr writes a value that may hold an expression: the expression's
// text when it has one, else the number, left out when it is 0.
func (n *Node) expr(name string, v float64, text string) {
	switch {
	case text != "":
		n.add(&Node{Name: name, HasExpr: true, Expr: text})
	case !isPlusZero(v):
		n.add(&Node{Name: name, HasExpr: true, Expr: FormatReal(v)})
	}
}

func (n *Node) str(name, v string) {
	if v != "" {
		n.add(field(name, String(v)))
	}
}

func (n *Node) vector(name string, v geom.Vector) {
	if !isPlusZero(v.X) || !isPlusZero(v.Y) || !isPlusZero(v.Z) {
		n.add(field(name, num(v.X), num(v.Y), num(v.Z)))
	}
}

func (n *Node) quaternion(name string, q geom.Quaternion) {
	if !isPlusZero(q.W) || !isPlusZero(q.X) || !isPlusZero(q.Y) || !isPlusZero(q.Z) {
		n.add(field(name, num(q.W), num(q.X), num(q.Y), num(q.Z)))
	}
}

func (n *Node) color(name string, c core.RGBA) {
	n.add(field(name, colorArgs(c)...))
}

func isPlusZero(v float64) bool { return v == 0 && !math.Signbit(v) }

func hex(v uint32) Value { return Word("0x" + strconv.FormatUint(uint64(v), 16)) }

func num(v float64) Value { return Word(FormatReal(v)) }

func colorArgs(c core.RGBA) []Value {
	return []Value{
		Word(strconv.Itoa(int(c.Red))), Word(strconv.Itoa(int(c.Green))),
		Word(strconv.Itoa(int(c.Blue))), Word(strconv.Itoa(int(c.Alpha))),
	}
}

// FormatReal writes v in the shortest form that reads back to the same
// bits: plain notation from 1e-6 up to 1e21, exponent notation outside.
func FormatReal(v float64) string {
	switch {
	case math.IsNaN(v):
		return "nan"
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	if a := math.Abs(v); a == 0 || (a >= 1e-6 && a < 1e21) {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// encodeGroup writes a group; a length the user typed is written in
// the document's unit, mmPer millimetres.
func encodeGroup(g *sketch.Group, mmPer float64) *Node {
	n := block("group", hex(g.H.V), Word(enumName(groupTypes, g.Type)), String(g.Name))
	n.int("order", g.Order)
	if g.Subtype != 0 {
		n.add(field("subtype", Word(enumName(groupSubtypes, g.Subtype))))
	}
	n.handle("workplane", g.ActiveWorkplane.V)
	n.handle("op-a", g.OpA.V)
	n.handle("op-b", g.OpB.V)
	if g.SweepUp.Magnitude() > 0 {
		n.add(field("sweep-up", num(g.SweepUp.X), num(g.SweepUp.Y), num(g.SweepUp.Z)))
	}
	if len(g.LoftThrough) > 0 {
		args := make([]Value, len(g.LoftThrough))
		for i, h := range g.LoftThrough {
			args[i] = hex(h.V)
		}
		n.add(field("loft-through", args...))
	}
	n.expr(g.FieldName(sketch.GroupCopies), g.ValA, g.ExprA)
	valB := g.ValB
	if g.IsLength(sketch.GroupPitch) {
		valB /= mmPer
	}
	n.expr(g.FieldName(sketch.GroupPitch), valB, g.ExprB)
	valC := g.ValC
	if g.IsLength(sketch.GroupValC) {
		valC /= mmPer
	}
	n.expr(g.FieldName(sketch.GroupValC), valC, g.ExprC)
	extent := g.Extent
	if g.IsLength(sketch.GroupExtent) {
		extent /= mmPer
	}
	n.expr(g.FieldName(sketch.GroupExtent), extent, g.ExprExtent)
	n.color("color", g.Color)
	n.flag("skip-first", g.SkipFirst)
	n.flag("start-along-sides", g.LoftAlongSides[0])
	n.flag("end-along-sides", g.LoftAlongSides[1])
	n.flag("mirror-solid", g.MirrorSolid)
	if g.MeshCombine != sketch.CombineUnion {
		n.add(field("combine", Word(enumName(combines, g.MeshCombine))))
	}
	n.flag("force-to-mesh", g.ForceToMesh)
	if len(g.Edges) > 0 {
		args := make([]Value, len(g.Edges))
		for i, he := range g.Edges {
			args[i] = hex(he.V)
		}
		n.add(field("edges", args...))
	}
	if len(g.DraftFaces) > 0 {
		args := make([]Value, len(g.DraftFaces))
		for i, he := range g.DraftFaces {
			args[i] = hex(he.V)
		}
		n.add(field("draft-faces", args...))
	}
	for _, e := range g.SolidEdges {
		n.add(solidEdgeField(e))
	}
	if g.Appearance != "" {
		n.add(field("appearance", Word(g.Appearance)))
	}
	for _, fa := range g.FaceAppearances {
		n.add(field("face-appearance", hex(uint32(fa.Face)), Word(fa.Appearance)))
	}
	n.real("density", g.Density)
	for _, f := range g.SolidFaces {
		n.add(field("solid-face", hex(uint32(f))))
	}
	for _, er := range g.EdgeRadii {
		text := er.Expr
		if text == "" {
			text = FormatReal(er.Val / mmPer)
		}
		n.add(&Node{Name: "edge-radius", Args: []Value{Word(strconv.Itoa(er.Edge + 1))}, HasExpr: true, Expr: text})
	}

	pd := block("predef")
	pd.quaternion("q", g.Predef.Q)
	pd.handle("origin", g.Predef.Origin.V)
	pd.vector("at", g.Predef.At)
	pd.handle("entity-b", g.Predef.EntityB.V)
	pd.handle("entity-c", g.Predef.EntityC.V)
	pd.flag("swap-uv", g.Predef.SwapUV)
	pd.flag("negate-u", g.Predef.NegateU)
	pd.flag("negate-v", g.Predef.NegateV)
	if len(pd.Children) > 0 {
		n.add(pd)
	}

	n.flag("visible", g.Visible)
	n.flag("suppress", g.Suppress)
	n.flag("suppress-all", g.SuppressAll)
	n.flag("relax-constraints", g.RelaxConstraints)
	n.flag("allow-redundant", g.AllowRedundant)
	n.flag("suppress-dof", g.SuppressDofCalculation)
	n.flag("all-dims-reference", g.AllDimsReference)
	if g.ExprScale != "" {
		n.expr("scale", g.Scale, g.ExprScale)
	} else if g.Scale != 1 && g.Scale != 0 {
		// 0 is no scale: the groups a new file starts with have it, and
		// only link groups use the scale. A reader takes a missing
		// scale as 1.
		n.add(field("scale", num(g.Scale)))
	}
	n.str("link", g.LinkFile)
	n.flag("link-as-solid", g.LinkAsSolid)

	if len(g.Remap) > 0 {
		type entry struct {
			id  uint32
			key core.EntityKey
		}
		entries := make([]entry, 0, len(g.Remap))
		for k, v := range g.Remap {
			entries = append(entries, entry{id: v.V, key: k})
		}
		sort.Slice(entries, func(i, j int) bool {
			a, b := entries[i], entries[j]
			if a.id != b.id {
				return a.id < b.id
			}
			if a.key.Input.V != b.key.Input.V {
				return a.key.Input.V < b.key.Input.V
			}
			return a.key.CopyNumber < b.key.CopyNumber
		})
		rm := block("remap")
		for _, e := range entries {
			rm.add(field("map", Word(strconv.FormatUint(uint64(e.id), 10)),
				hex(e.key.Input.V), Word(strconv.Itoa(e.key.CopyNumber))))
		}
		n.add(rm)
	}
	return n
}

// enumName is a value's name, or "unknown" for one the format has no
// name for, which a reader reports.
func enumName[T comparable](e enum[T], v T) string {
	if s, ok := e.names[v]; ok {
		return s
	}
	return "unknown"
}

func encodeRequest(r *sketch.Request) *Node {
	n := block("request", hex(r.H.V), Word(enumName(requestTypes, r.Type)))
	n.handle("group", r.Group.V)
	if !core.IsFreeIn3D(r.Workplane) {
		n.add(field("workplane", hex(r.Workplane.V)))
	}
	n.flag("construction", r.Construction)
	n.handle("style", r.Style.V)
	n.int("extra-points", r.ExtraPoints)
	n.str("text", r.Str)
	n.str("font", r.Font)
	n.str("file", r.File)
	n.real("aspect-ratio", r.AspectRatio)
	return n
}

// encodeConstraint writes a constraint; a length is written in the
// document's unit, mmPer millimetres.
func encodeConstraint(c *core.Constraint, mmPer float64) *Node {
	n := block("constraint", hex(c.H.V), Word(enumName(constraintTypes, c.Type)))
	n.handle("group", c.Group.V)
	n.handle("workplane", c.Workplane.V)
	v := c.ValA
	if sketch.IsLengthValue(c.Type) {
		v /= mmPer
	}
	n.expr("value", v, c.Expr)
	n.str("name", c.Name)
	n.handle("param", c.ValP.V)
	n.handle("pt-a", c.PtA.V)
	n.handle("pt-b", c.PtB.V)
	n.handle("entity-a", c.EntityA.V)
	n.handle("entity-b", c.EntityB.V)
	n.handle("entity-c", c.EntityC.V)
	n.handle("entity-d", c.EntityD.V)
	n.flag("other", c.Other)
	n.flag("other2", c.Other2)
	n.flag("reference", c.Reference)
	if c.Limit != core.LimitNone {
		n.add(field("limit", Word(enumName(limits, c.Limit))))
	}
	n.str("comment", c.Comment)
	n.vector("label-offset", c.Disp.Offset)
	n.handle("label-style", c.Disp.Style.V)
	return n
}

func encodeStyle(s *core.Style) *Node {
	n := block("style", hex(s.H.V), String(s.Name))
	n.real("width", s.Width)
	if s.WidthAs != 0 {
		n.add(field("width-as", Word(enumName(unitsAs, s.WidthAs))))
	}
	n.real("text-height", s.TextHeight)
	if s.TextHeightAs != 0 {
		n.add(field("text-height-as", Word(enumName(unitsAs, s.TextHeightAs))))
	}
	n.real("text-angle", s.TextAngle)
	if s.TextOrigin != 0 {
		var args []Value
		for _, bit := range []core.TextOrigin{core.TextOriginLeft, core.TextOriginRight, core.TextOriginBot, core.TextOriginTop} {
			if s.TextOrigin&bit != 0 {
				args = append(args, Word(textOrigins.names[bit]))
			}
		}
		if rest := s.TextOrigin &^ 0x0f; rest != 0 {
			args = append(args, Word("unknown"))
		}
		n.add(field("text-origin", args...))
	}
	n.color("color", s.Color)
	n.color("fill-color", s.FillColor)
	n.flag("filled", s.Filled)
	n.flag("visible", s.Visible)
	n.flag("exportable", s.Exportable)
	if s.StippleType != core.StippleContinuous {
		n.add(field("stipple", Word(enumName(stipples, s.StippleType))))
	}
	n.real("stipple-scale", s.StippleScale)
	// Always written: 0 is a level a style can be set to, so a file that
	// leaves it out is one saved before the field existed.
	n.add(field("z-index", Word(strconv.Itoa(s.ZIndex))))
	return n
}

func encodeEntity(e *core.Entity) *Node {
	n := block("entity", hex(e.H.V), Word(enumName(entityTypes, e.Type)))
	n.handle("group", e.Group.V)
	n.handle("workplane", e.Workplane.V)
	n.flag("construction", e.Construction)
	n.handle("style", e.Style.V)
	n.str("text", e.Str)
	n.str("font", e.Font)
	n.str("file", e.File)
	last := -1
	for i, p := range e.Point {
		if p.V != 0 {
			last = i
		}
	}
	if last >= 0 {
		pts := make([]Value, last+1)
		for i := range pts {
			pts[i] = hex(e.Point[i].V)
		}
		n.add(field("points", pts...))
	}
	n.int("extra-points", e.ExtraPoints)
	n.handle("normal", e.Normal.V)
	n.handle("distance", e.Distance.V)
	n.vector("act-point", e.ActPoint)
	n.quaternion("act-normal", e.ActNormal)
	n.real("act-distance", e.ActDistance)
	n.flag("act-visible", e.ActVisible)
	return n
}

func encodeMesh(m *srf.SMesh) *Node {
	n := block("mesh")
	for i := range m.Triangles {
		tr := &m.Triangles[i]
		args := []Value{hex(uint32(tr.Meta.Face))}
		args = append(args, colorArgs(tr.Meta.RGBA)...)
		for _, v := range []geom.Vector{tr.A, tr.B, tr.C} {
			args = append(args, num(v.X), num(v.Y), num(v.Z))
		}
		n.add(field("triangle", args...))
	}
	return n
}

func encodeShell(s *srf.SShell) *Node {
	n := block("shell")
	for i := range s.Surface {
		sf := &s.Surface[i]
		b := block("surface", hex(sf.H.V))
		b.color("color", sf.Color)
		b.handle("face", uint32(sf.Face))
		if sf.Sheet != 0 {
			b.handle("sheet", sf.Sheet)
		}
		b.add(field("degree", Word(strconv.Itoa(sf.DegM)), Word(strconv.Itoa(sf.DegN))))
		for i := 0; i <= sf.DegM && i < len(sf.Ctrl); i++ {
			for j := 0; j <= sf.DegN && j < len(sf.Ctrl[i]); j++ {
				p := sf.Ctrl[i][j]
				b.add(field("ctrl", Word(strconv.Itoa(i)), Word(strconv.Itoa(j)),
					num(p.X), num(p.Y), num(p.Z), num(sf.Weight[i][j])))
			}
		}
		for _, t := range sf.Trim {
			dir := "forward"
			if t.Backwards {
				dir = "backward"
			}
			b.add(field("trim", hex(t.Curve.V), Word(dir),
				num(t.StartP.X), num(t.StartP.Y), num(t.StartP.Z),
				num(t.FinishP.X), num(t.FinishP.Y), num(t.FinishP.Z)))
		}
		n.add(b)
	}
	for i := range s.Curve {
		sc := &s.Curve[i]
		b := block("curve", hex(sc.H.V))
		b.flag("exact", sc.IsExact)
		b.int("degree", sc.Exact.Degree)
		b.add(field("surfaces", hex(sc.SurfA.V), hex(sc.SurfB.V)))
		if sc.IsExact {
			for i := 0; i <= sc.Exact.Degree && i < len(sc.Exact.Ctrl); i++ {
				p := sc.Exact.Ctrl[i]
				b.add(field("ctrl", Word(strconv.Itoa(i)), num(p.X), num(p.Y), num(p.Z), num(sc.Exact.Weight[i])))
			}
		}
		for _, pt := range sc.Points {
			name := "point"
			if pt.Vertex {
				name = "vertex"
			}
			b.add(field(name, num(pt.P.X), num(pt.P.Y), num(pt.P.Z)))
		}
		n.add(b)
	}
	return n
}

// solidEdgeField is an edge of the solid by name: "solid-edge <face>
// <face> <n> of <m>", or without the count.
func solidEdgeField(e topo.Name) *Node {
	if e.Of > 0 {
		return field("solid-edge", hex(uint32(e.A)), hex(uint32(e.B)), Word(strconv.Itoa(e.N)),
			Word("of"), Word(strconv.Itoa(e.Of)))
	}
	return field("solid-edge", hex(uint32(e.A)), hex(uint32(e.B)), Word(strconv.Itoa(e.N)))
}

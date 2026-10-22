package api

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// This file is the model written as a script that builds it again:
// the groups in order, each made by the call the window's command
// reaches, then its requests and constraints.

// Script is the drawing as a Go program that builds it again on a new
// document through this package: the unit, the parameters and their
// configurations, every group in the order the model is built, made by
// the Features or Assembly call that makes one of its kind, with its
// options, the requests of a sketch where they are solved to, and the
// constraints with their values, names and expressions. Run with
// telesapr-cli run, Script > Run File or the AI connection's run_script,
// it gives the model again, its handles numbered afresh. What it can't
// write yet is a comment in the program starting "not written:" and a
// line of notes.
func (d *Doc) Script() (src string, notes []string) {
	d.read(func(sk *sketch.Sketch) {
		w := newScriptWriter(sk)
		if fn, ok := d.h.(FileNamer); ok {
			w.file = fn.FileName()
		}
		w.write()
		src, notes = w.String(), w.notes
	})
	return src, notes
}

// ExportScript writes the drawing to path as the program Script gives,
// and returns what it couldn't write, as Script's notes.
func (m FileMenu) ExportScript(path string) (notes []string, err error) {
	d := m.d
	if err := d.checkFile(path, true); err != nil {
		return nil, err
	}
	src, notes := d.Script()
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		return nil, fmt.Errorf("writing the script: %w", err)
	}
	return notes, nil
}

// scriptWriter writes one model's script.
type scriptWriter struct {
	sk    *sketch.Sketch
	b     strings.Builder
	notes []string

	groupVar map[core.HGroup]string
	reqVar   map[core.HRequest]string
	styleVar map[core.HStyle]string
	// skip are groups written by another's call: a thread's sketches
	// and lathe, written with its helix.
	skip map[core.HGroup]bool
	// last is the group written last, after which the next goes.
	last core.HGroup
	// wp is the workplane drawn in, in the group written last.
	wp core.HEntity
	// file is the model's own file, which a link's path is relative to.
	file string
	// atEnd are lines written after every group.
	atEnd []string
}

func newScriptWriter(sk *sketch.Sketch) *scriptWriter {
	return &scriptWriter{
		sk:       sk,
		groupVar: map[core.HGroup]string{},
		reqVar:   map[core.HRequest]string{},
		styleVar: map[core.HStyle]string{},
		skip:     map[core.HGroup]bool{},
	}
}

func (w *scriptWriter) String() string { return w.b.String() }

func (w *scriptWriter) line(format string, args ...any) {
	w.b.WriteString("\t")
	fmt.Fprintf(&w.b, format, args...)
	w.b.WriteString("\n")
}

// notWritten says in the script and in the notes what isn't written.
func (w *scriptWriter) notWritten(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	w.line("// not written: %s", msg)
	w.notes = append(w.notes, msg)
}

// num is v as Go reads it back to the same bits.
func num(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func vecText(v geom.Vector) string {
	return "api.V(" + num(v.X) + ", " + num(v.Y) + ", " + num(v.Z) + ")"
}

// write is the whole program.
func (w *scriptWriter) write() {
	sk := w.sk
	w.b.WriteString(scriptHeader)
	if sk.Units != core.UnitMM {
		w.line("doc.View.SetUnits(core.Unit(%d)) // %s", int(sk.Units), unitNames[sk.Units])
	}
	w.writeParameters()
	w.writeStyles()
	w.writeGroups()
	if len(w.atEnd) > 0 {
		w.b.WriteString("\n")
		for _, l := range w.atEnd {
			w.line("%s", l)
		}
	}
	w.writeDatums()
	w.b.WriteString("}\n")
}

const scriptHeader = `// The model, written by TeleSAPR's File > Export as Script: run it with
// telesapr-cli run, or Script > Run File, on a new drawing.
package main

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Each name keeps the compiler from refusing a package not used.
var _, _ = core.Unit(0), sketch.GroupValue(0)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// linked is the group a link made, or stops the script.
func linked(g api.Group, err error) api.Group {
	must(err)
	return g
}

func main() {
	doc := api.Current()
`

// writeParameters writes the table, each configuration from its own
// expressions, then the one active or the table as it is.
func (w *scriptWriter) writeParameters() {
	sk := w.sk
	for i, c := range sk.Configurations {
		if i > 0 {
			// Adding one makes it active, and what is set then goes
			// into it too.
			w.line(`must(doc.Params.ActivateConfiguration(""))`)
		}
		for _, p := range sk.Parameters {
			if e, ok := c.Exprs[p.Name]; ok {
				w.line("must(doc.Params.Set(%q, %q))", p.Name, e)
			}
		}
		w.line("must(doc.Params.AddConfiguration(%q))", c.Name)
	}
	if len(sk.Configurations) > 0 {
		w.line(`must(doc.Params.ActivateConfiguration(""))`)
	}
	for _, p := range sk.Parameters {
		w.line("must(doc.Params.Set(%q, %q))", p.Name, p.Expr)
	}
	if sk.ActiveConfiguration != "" {
		w.line("must(doc.Params.ActivateConfiguration(%q))", sk.ActiveConfiguration)
	}
}

// writeDatums names entities once every group is made; a datum on a
// face or edge of the solid was named as its group was made.
func (w *scriptWriter) writeDatums() {
	for _, dt := range w.sk.Datums {
		if g := w.sk.Group(dt.Entity.Group()); g != nil && g.Subtype == sketch.GroupWorkplaneByDatum && dt.Entity == g.H.Entity(0) {
			if _, ok := w.datumCall(g); ok {
				continue
			}
		}
		w.line("doc.Datums.Name(%q, %s)", dt.Name, w.ref(dt.Entity))
	}
}

// writeGroups writes every group in the order the model is built.
func (w *scriptWriter) writeGroups() {
	sk := w.sk
	order := slices.Clone(sk.GroupOrder)
	for _, hg := range order {
		g := sk.Group(hg)
		if g == nil || hg.V == core.HGroupReferences {
			continue
		}
		if g.Thread.IsThread() {
			if hs, err := sk.ThreadGroups(hg); err == nil {
				for _, h := range hs {
					if h != hg {
						w.skip[h] = true
					}
				}
			}
		}
	}
	first := true
	for _, hg := range order {
		g := sk.Group(hg)
		if g == nil || hg.V == core.HGroupReferences {
			continue
		}
		if w.skip[hg] {
			continue
		}
		w.b.WriteString("\n")
		if first && w.isDefaultSketch(g) {
			// A new drawing starts with this group active.
			v := w.groupName(g)
			w.line("%s := doc.Active()", v)
			if g.Name != "sketch-in-plane" {
				w.line("%s.SetName(%q)", v, g.Name)
			}
			w.wp = g.H.Entity(0)
			w.last = hg
			w.writeOptions(g, v)
			w.writeRequests(g)
			w.writeConstraints(g)
			w.drawIn(g.ActiveWorkplane)
		} else {
			if first {
				w.line("doc.Active().Delete() // the group a new drawing starts with")
			}
			w.writeGroup(g)
		}
		first = false
	}
}

// isDefaultSketch reports whether g is the group a new drawing starts
// with: a sketch in the XY workplane through the origin.
func (w *scriptWriter) isDefaultSketch(g *sketch.Group) bool {
	return g.Type == sketch.GroupDrawingWorkplane && g.Subtype == sketch.GroupWorkplaneByPointOrtho &&
		g.Predef.Origin == sketch.HRequestReferenceXY.Entity(1) && g.Predef.Q == geom.Quaternion{W: 1}
}

// groupName is the variable a group is held in.
func (w *scriptWriter) groupName(g *sketch.Group) string {
	v := fmt.Sprintf("g%d", g.H.V)
	w.groupVar[g.H] = v
	return v
}

// activate makes hg active, as what is made next is made from it.
func (w *scriptWriter) activate(hg core.HGroup) {
	if hg == w.last {
		return
	}
	if v, ok := w.groupVar[hg]; ok {
		w.line("doc.Activate(%s)", v)
		if g := w.sk.Group(hg); g != nil {
			w.wp = g.ActiveWorkplane
		}
	}
}

// writeGroup writes a group's call, options, parameters, requests and
// constraints.
func (w *scriptWriter) writeGroup(g *sketch.Group) {
	prev := w.last
	call, from := w.groupCall(g)
	if call == "" {
		return
	}
	if from.V != 0 {
		w.activate(from)
	} else {
		w.activate(prev)
	}
	switch g.Type {
	case sketch.GroupExtrude, sketch.GroupTranslate:
		// They take the workplane drawn in.
		w.drawIn(g.Predef.EntityB)
	}
	v := w.groupName(g)
	w.line("%s := %s", v, call)
	if from.V != 0 && from != prev {
		// Made after what it is made from; it goes where it was.
		w.line("must(%s.History().MoveToEnd())", v)
	}
	w.last = g.H
	// The workplane the new group draws in, as its call leaves it.
	switch g.Type {
	case sketch.GroupDrawingWorkplane:
		w.wp = g.H.Entity(0)
	case sketch.GroupTranslate:
		w.wp = g.Predef.EntityB
	default:
		w.wp = core.NoEntity
	}
	w.writeOptions(g, v)
	w.writeGroupParams(g, v)
	w.writeRequests(g)
	w.writeConstraints(g)
	// Left drawing in its own workplane, for what is made from it.
	w.drawIn(g.ActiveWorkplane)
}

// writeGroupParams sets the group's own parameters, as an extrusion's
// vector or a link's placement, to what they are solved to.
func (w *scriptWriter) writeGroupParams(g *sketch.Group, v string) {
	sk := w.sk
	switch g.Type {
	case sketch.GroupDrawing3D, sketch.GroupDrawingWorkplane:
		return
	}
	for i := range sk.Params {
		p := &sk.Params[i]
		if p.H.V&0x80000000 == 0 || (p.H.V>>16)&0x7fff != g.H.V {
			continue
		}
		w.line("%s.Param(%d).Set(%s)", v, p.H.V&0xffff, num(p.Val))
	}
}

// writeOptions sets what the group's call leaves at its default.
func (w *scriptWriter) writeOptions(g *sketch.Group, v string) {
	def := sketch.NewGroup()
	if g.Color != def.Color && g.Color != (core.RGBA{}) {
		w.line("%s.SetColor(%d, %d, %d)", v, g.Color.Red, g.Color.Green, g.Color.Blue)
		if g.Color.Alpha != 255 {
			w.line("%s.SetOpacity(%s)", v, num(float64(g.Color.Alpha)/255))
		}
	}
	if !g.Visible {
		w.line("%s.SetVisible(false)", v)
	}
	if g.RelaxConstraints {
		w.line("%s.SetRelaxConstraints(true)", v)
	}
	if g.AllowRedundant {
		w.line("%s.SetAllowRedundant(true)", v)
	}
	if g.SuppressDofCalculation {
		w.line("%s.SetSuppressDof(true)", v)
	}
	if g.AllDimsReference {
		w.line("%s.SetAllDimsReference(true)", v)
	}
	if g.ForceToMesh {
		w.line("%s.SetForceToMesh(true)", v)
	}
	if g.Appearance != "" {
		w.line("%s.SetAppearance(%q)", v, g.Appearance)
	}
	w.writeFaceAppearances(g, v)
	if g.Density != 0 {
		w.line("%s.SetDensity(%s)", v, num(g.Density))
	}
	if g.HasCombine() {
		if g.MeshCombine != w.defaultCombine(g) {
			w.line("%s.SetCombine(sketch.CombineAs(%d))", v, int(g.MeshCombine))
		}
	}
	w.writeKindOptions(g, v)
	for _, ge := range []struct {
		val  sketch.GroupValue
		expr string
	}{{sketch.GroupCopies, g.ExprA}, {sketch.GroupPitch, g.ExprB}, {sketch.GroupValC, g.ExprC},
		{sketch.GroupScale, g.ExprScale}, {sketch.GroupExtent, g.ExprExtent}} {
		if ge.expr != "" {
			w.line("must(%s.SetExpr(sketch.GroupValue(%d), %q))", v, int(ge.val), ge.expr)
		}
	}
	// Left out once everything is made: what comes after is made from
	// it as it was.
	switch {
	case g.SuppressAll:
		w.atEnd = append(w.atEnd, fmt.Sprintf("%s.SetSuppress(api.SuppressWhole)", v))
	case g.Suppress:
		w.atEnd = append(w.atEnd, fmt.Sprintf("%s.SetSuppress(api.SuppressSolid)", v))
	}
}

// writeFaceAppearances gives the group's faces their own appearances
// once everything is made, each face found by a point in it on the
// group's solid.
func (w *scriptWriter) writeFaceAppearances(g *sketch.Group, v string) {
	if len(g.FaceAppearances) == 0 {
		return
	}
	tp := topologyAt(w.sk, g)
	if tp == nil {
		w.notWritten("%s's faces' own appearances: the group has no solid with faces", g.Name)
		return
	}
	mesh := displayMesh(w.sk, g)
	for _, fa := range g.FaceAppearances {
		p, ok := facePoint(tp, mesh, fa.Face)
		if !ok {
			w.notWritten("%s: the appearance %q of a face no point finds", g.Name, fa.Appearance)
			continue
		}
		w.atEnd = append(w.atEnd, fmt.Sprintf("doc.View.SetFaceAppearance(%s.Solid().FaceAt(%s), %q)", v, vecText(p), fa.Appearance))
	}
}

// defaultCombine is how a group of g's kind is combined when made.
func (w *scriptWriter) defaultCombine(g *sketch.Group) sketch.CombineAs {
	switch g.Type {
	case sketch.GroupLinked:
		return sketch.CombineAssemble
	case sketch.GroupWrap:
		return g.MeshCombine
	}
	return sketch.CombineUnion
}

// ref is how the script names an entity.
func (w *scriptWriter) ref(h core.HEntity) string {
	switch {
	case h.V == 0:
		return "api.Entity{}"
	case h.IsFromRequest():
		hr := h.Request()
		if hr.IsFromReferences() {
			return referenceName(h)
		}
		v, ok := w.reqVar[hr]
		if !ok {
			return fmt.Sprintf("doc.Entity(0x%x)", h.V)
		}
		if i := h.V & 0xffff; i != 0 {
			return fmt.Sprintf("%s.Part(%d)", v, i)
		}
		return v
	}
	v, ok := w.groupVar[h.Group()]
	g := w.sk.Group(h.Group())
	if !ok || g == nil {
		return fmt.Sprintf("doc.Entity(0x%x)", h.V)
	}
	for k, id := range g.Remap {
		if g.H.Entity(int(id.V)) == h {
			return fmt.Sprintf("%s.Made(%s, %d)", v, w.ref(k.Input), k.CopyNumber)
		}
	}
	return fmt.Sprintf("doc.Entity(0x%x)", h.V)
}

// referenceName is how the script names an entity of the reference
// requests: a workplane or the origin by name, the rest by handle.
func referenceName(h core.HEntity) string {
	switch h {
	case sketch.HRequestReferenceXY.Entity(0):
		return `doc.Workplane("XY")`
	case sketch.HRequestReferenceYZ.Entity(0):
		return `doc.Workplane("YZ")`
	case sketch.HRequestReferenceZX.Entity(0):
		return `doc.Workplane("ZX")`
	case sketch.HRequestReferenceXY.Entity(1):
		return "doc.Origin()"
	}
	return fmt.Sprintf("doc.Entity(0x%x)", h.V)
}

// refs is a slice of entities, as the script names them.
func (w *scriptWriter) refs(hs []core.HEntity) string {
	parts := make([]string, len(hs))
	for i, h := range hs {
		parts[i] = w.ref(h)
	}
	return "[]api.Entity{" + strings.Join(parts, ", ") + "}"
}

// angleOf is a quaternion's u and v, for a call that takes them.
func uvOf(q geom.Quaternion) (u, v geom.Vector) {
	return q.RotationU(), q.RotationV()
}

// snap rounds what floating point leaves a hair off a round number,
// for a script a reader can read; it changes nothing a solve can see.
func snap(v float64) float64 {
	if r := math.Round(v); math.Abs(v-r) < 1e-12 {
		return r
	}
	return v
}

// Made is the entity the group made from the entity from, its copy
// number n: the group's own record of what it made, kept as the
// model is edited, by which Doc.Script names what a group makes (an
// extrusion's faces, a copy of a point). from is Entity{} for what a
// group makes of no entity, as an extrusion's sides.
func (g Group) Made(from Entity, n int) (e Entity) {
	g.d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		id, ok := group.Remap[core.EntityKey{Input: from.H, CopyNumber: n}]
		if !ok {
			fail("%s made nothing from %s as copy %d", groupDescription(group), from.describe(sk), n)
		}
		e = Entity{g.d, group.H.Entity(int(id.V))}
		if sk.Entity(e.H) == nil {
			fail("%s has no entity made from %s as copy %d", groupDescription(group), from.describe(sk), n)
		}
	})
	return e
}

// Part is the entity number i of the request that drew e: 0 the
// request's own entity, then its points and its normal and distance as
// the request makes them (a line's ends are 1 and 2, a circle's centre
// 1), as Doc.Script names them.
func (e Entity) Part(i int) (p Entity) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if !ent.H.IsFromRequest() || i < 0 || i > 0xffff {
			fail("%s isn't drawn by a request with a part %d", e.describe(sk), i)
		}
		p = Entity{e.d, ent.H.Request().Entity(i)}
		if sk.Entity(p.H) == nil {
			fail("%s has no part %d", e.describe(sk), i)
		}
	})
	return p
}

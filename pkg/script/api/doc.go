package api

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Vec is a point or a direction in space, in mm.
type Vec = geom.Vector

// V is the vector (x, y, z).
func V(x, y, z float64) Vec { return Vec{X: x, Y: y, Z: z} }

// Inch is v inches in mm.
func Inch(v float64) float64 { return v * 25.4 }

// Doc is a sketch: the drawing a script reads and changes.
type Doc struct {
	h Host
}

// NewDoc is the document of a host. The application makes the one a
// script gets from Current; a program of its own makes them with New
// and Open.
func NewDoc(h Host) *Doc { return &Doc{h: h} }

// Host is the program around the sketch.
func (d *Doc) Host() Host { return d.h }

// current is the document Current returns when the package is used
// from compiled Go; an interpreter has its own.
var current *Doc

// Current is the document the script was run on: the drawing on screen
// in the application, the file named on the command line of the
// command-line tool, or a new sketch. A script compiled as a program
// of its own gets a new sketch too, unless it called SetCurrent.
func Current() *Doc {
	if current == nil {
		current = New()
	}
	return current
}

// SetCurrent sets what Current returns in a compiled program.
func SetCurrent(d *Doc) { current = d }

// New is a new, empty sketch, as File > New makes: the three reference
// workplanes and a group to draw in, in the XY plane.
func New() *Doc {
	sk := sketch.NewSketch()
	sk.NewFile()
	return NewDoc(NewSketchHost(sk, ""))
}

// Open loads a sketch from a .tlspr file. A SolveSpace model (.slvs) is
// converted with Import instead.
func Open(path string) (*Doc, error) {
	doc, err := loadModel(path)
	if err != nil {
		return nil, err
	}
	return NewDoc(NewSketchHost(doc.Sketch, path)), nil
}

// Import converts the SolveSpace model src (.slvs) to the TeleSAPR
// model dst (.tlspr), and opens it. Every SolveSpace model src links is
// converted too, to a .tlspr file beside it, and the links are pointed
// at those; a converted file that exists already is used as it is. The
// .slvs files are not changed. It fails when a linked file is missing,
// when the links form a cycle, or when a file can't be written.
func Import(src, dst string) (*Doc, error) {
	plan, err := convert.MakePlan(src, dst)
	if err != nil {
		return nil, err
	}
	if missing := plan.Missing(); len(missing) > 0 {
		var names []string
		for _, it := range missing {
			names = append(names, fmt.Sprintf("%s (linked from %s)", it.Src, it.LinkedFrom))
		}
		return nil, fmt.Errorf("can't import %s: missing linked files: %s", src, strings.Join(names, ", "))
	}
	res, err := convert.Run(plan, convert.Options{Units: core.UnitMM, Quiet: true})
	if err != nil {
		return nil, err
	}
	if len(res.Failed) > 0 {
		var failed []string
		for path, err := range res.Failed {
			failed = append(failed, fmt.Sprintf("%s: %v", path, err))
		}
		sort.Strings(failed)
		return nil, fmt.Errorf("can't import %s: %s", src, strings.Join(failed, "; "))
	}
	return Open(plan.Root().Dst)
}

// fail stops the call with an error.
func fail(format string, args ...any) {
	panic(fmt.Errorf(format, args...))
}

// read runs fn on the sketch, regenerated if it was changed since.
func (d *Doc) read(fn func(sk *sketch.Sketch)) {
	d.h.Run(func() {
		if d.h.Sketch().RegenerationPending() {
			d.h.Regenerate()
		}
		fn(d.h.Sketch())
	})
}

// change runs fn, which changes the sketch.
func (d *Doc) change(fn func(sk *sketch.Sketch)) {
	d.h.Run(func() {
		d.h.Changing()
		fn(d.h.Sketch())
	})
}

// Load replaces the sketch by the one in a .tlspr file.
func (d *Doc) Load(path string) (err error) {
	d.h.Run(func() { err = d.h.Load(path) })
	return err
}

// Save regenerates the sketch and writes it to a .tlspr file. A
// SolveSpace model (.slvs) is not written.
func (d *Doc) Save(path string) (err error) {
	d.h.Run(func() { err = d.h.Save(path) })
	return err
}

// exportKinds says what each file extension holds; the ones that can
// hold several kinds are exported as the first.
var exportKinds = map[string]ExportKind{
	".stl": ExportMesh, ".3mf": ExportMesh, ".obj": ExportMesh, ".js": ExportMesh, ".html": ExportMesh, ".wrl": ExportMesh,
	".svg": ExportView, ".dxf": ExportView, ".pdf": ExportView, ".eps": ExportView, ".ps": ExportView,
	".plt": ExportView, ".hpgl": ExportView, ".ngc": ExportView, ".txt": ExportView,
	".step": ExportSurfaces, ".stp": ExportSurfaces,
}

// Export writes the sketch in the format the file's extension names:
// the mesh of the solid (.stl, .obj, ...), the view (.svg, .dxf, .pdf,
// ...), the surfaces of the solid (.step), or a picture as Render
// makes it (.png).
func (d *Doc) Export(path string) error {
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return d.Render(path, 1024, 768, "isometric")
	}
	kind, ok := exportKinds[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return fmt.Errorf("can't tell what to export from the name '%s'", path)
	}
	return d.ExportAs(kind, path)
}

// ExportAs is Export for a format that can hold several things, such
// as a wireframe in a .dxf or a .step.
func (d *Doc) ExportAs(kind ExportKind, path string) (err error) {
	d.read(func(*sketch.Sketch) { err = d.h.Export(kind, path) })
	return err
}

// views are the directions a picture can be seen from: what is to
// the right, and what is up.
var views = map[string][2]Vec{
	"top":       {{X: 1}, {Z: -1}},
	"bottom":    {{X: 1}, {Z: 1}},
	"left":      {{Z: 1}, {Y: 1}},
	"right":     {{Z: -1}, {Y: 1}},
	"front":     {{X: 1}, {Y: 1}},
	"back":      {{X: -1}, {Y: 1}},
	"isometric": {{X: 0.707, Z: -0.707}, {X: -0.408, Y: 0.816, Z: -0.408}},
	// The window's own view, no direction: as the user sees it.
	"window": {},
}

// Render draws the sketch into a PNG file of width by height pixels,
// fitted to it and seen from "top", "bottom", "left", "right",
// "front", "back" or "isometric"; or "window", as the application's
// window shows it, the same middle and as much of it across.
func (d *Doc) Render(path string, width, height int, view string) (err error) {
	v, ok := views[strings.ToLower(view)]
	if !ok {
		return fmt.Errorf("no view '%s': they are top, bottom, left, right, front, back, isometric and window", view)
	}
	if width < 1 || height < 1 {
		return fmt.Errorf("a picture of %d by %d pixels", width, height)
	}
	d.read(func(*sketch.Sketch) { err = d.h.Render(path, width, height, v[0], v[1]) })
	return err
}

// Regen regenerates the whole sketch and solves it.
func (d *Doc) Regen() {
	d.h.Run(func() {
		for i := range d.h.Sketch().Groups {
			d.h.Sketch().Groups[i].Clean = false
		}
		d.h.Sketch().ScheduleGenerateAll()
		d.h.Regenerate()
	})
}

// SetChordTolerance sets how closely meshes follow curves, as a
// percentage of the model's size (0.1 unless set; smaller is finer and
// slower), and regenerates everything. In the application it is the
// setting Configuration shows; scripts that check volumes against a
// formula use a fine one.
func (d *Doc) SetChordTolerance(pct float64) {
	if !(pct > 0) || math.IsInf(pct, 0) {
		fail("a chord tolerance must be more than zero, not %g", pct)
	}
	d.h.Run(func() {
		d.h.SetChordTolerance(pct)
		for i := range d.h.Sketch().Groups {
			d.h.Sketch().Groups[i].Clean = false
		}
		d.h.Sketch().ScheduleGenerateAll()
		d.h.Regenerate()
	})
}

// ChordTolerance is how closely meshes follow curves, as a percentage
// of the model's size.
func (d *Doc) ChordTolerance() (pct float64) {
	d.read(func(sk *sketch.Sketch) { pct = sk.ChordTolPercent() })
	return pct
}

// Undo takes back the last step: in a console, what the last line
// that changed the sketch did.
func (d *Doc) Undo() (ok bool) {
	d.h.Run(func() { ok = d.h.Undo() })
	return ok
}

// Redo does again what Undo took back.
func (d *Doc) Redo() (ok bool) {
	d.h.Run(func() { ok = d.h.Redo() })
	return ok
}

// Unsaved reports whether the sketch changed since it was saved.
func (d *Doc) Unsaved() (unsaved bool) {
	d.h.Run(func() { unsaved = d.h.Sketch().Unsaved })
	return unsaved
}

// ForgetChanges lets the changes made since the sketch was saved go, as
// "Don't Save" does when the application asks: File > New, Open and
// the others that replace the drawing refuse to run from a script while
// it has unsaved changes, so that nothing is lost unasked. The
// drawing itself stays as it is until such a command replaces it.
func (d *Doc) ForgetChanges() {
	d.h.Run(func() {
		if f, ok := d.h.(interface{ ForgetChanges() }); ok {
			f.ForgetChanges()
			return
		}
		d.h.Sketch().Unsaved = false
	})
}

// MmPerUnit is the size in mm of the unit the user interface shows
// lengths in.
func (d *Doc) MmPerUnit() (mm float64) {
	d.h.Run(func() { mm = d.h.MmPerUnit() })
	return mm
}

// With runs fn on the sketch itself, for what this package has no
// call for; see the sketch and core packages. The sketch and what is
// got from it must not be kept after fn returns. The active group is
// taken to have changed.
func (d *Doc) With(fn func(sk *sketch.Sketch)) {
	d.change(func(sk *sketch.Sketch) {
		fn(sk)
		sk = d.h.Sketch()
		sk.MarkGroupDirty(sk.ActiveGroup(), false)
	})
}

// Batch runs fn in one go. In the application, where each call of
// this package waits for the user interface, that is much faster for
// a script that makes many things, and the drawing isn't shown half
// done.
func (d *Doc) Batch(fn func()) {
	d.h.Run(fn)
}

// Groups are the groups, in order, without the references.
func (d *Doc) Groups() (groups []Group) {
	d.read(func(sk *sketch.Sketch) {
		for _, hg := range sk.GroupOrder {
			if hg.V != core.HGroupReferences {
				groups = append(groups, Group{d, hg})
			}
		}
	})
	return groups
}

// Active is the group that is worked in: what is drawn goes into it.
func (d *Doc) Active() (g Group) {
	d.read(func(sk *sketch.Sketch) { g = Group{d, sk.ActiveGroup()} })
	return g
}

// Activate makes g the group that is worked in.
func (d *Doc) Activate(g Group) {
	d.read(func(sk *sketch.Sketch) {
		group := g.get(sk)
		if why := sk.SuppressedReason(g.H); why != "" {
			fail("%s is left out of the model (%s)", groupDescription(group), why)
		}
		d.h.Activate(g.H)
	})
}

// handleOf reads the handle a description starts with: the hex digits
// of "g003-extrude", after the letter.
func handleOf(desc string, letter byte) (uint32, bool) {
	if len(desc) < 2 || desc[0] != letter {
		return 0, false
	}
	digits := desc[1:]
	if i := strings.IndexByte(digits, '-'); i >= 0 {
		digits = digits[:i]
	}
	v, err := strconv.ParseUint(digits, 16, 32)
	return uint32(v), err == nil
}

// Group is the group with a name, or with a description as the
// property browser shows it: "g003-extrude" or just "g003". Check
// Valid if there may be none.
func (d *Doc) Group(name string) (g Group) {
	d.read(func(sk *sketch.Sketch) {
		g.d = d
		for i := range sk.Groups {
			if sk.Groups[i].Name == name {
				g.H = sk.Groups[i].H
				return
			}
		}
		if v, ok := handleOf(name, 'g'); ok && sk.GetGroup(core.HGroup{V: v}) != nil {
			g.H = core.HGroup{V: v}
		}
	})
	return g
}

// Request is the request with a description as the property browser
// shows it: "r004-line-segment" or just "r004".
func (d *Doc) Request(name string) (r Request) {
	d.read(func(sk *sketch.Sketch) {
		r.d = d
		if v, ok := handleOf(name, 'r'); ok && sk.GetRequest(core.HRequest{V: v}) != nil {
			r.H = core.HRequest{V: v}
		}
	})
	return r
}

// Constraint is the constraint with a description as the property
// browser shows it: "c002-pt-pt-distance" or just "c002".
func (d *Doc) Constraint(name string) (c Constraint) {
	d.read(func(sk *sketch.Sketch) {
		c.d = d
		if v, ok := handleOf(name, 'c'); ok && sk.GetConstraint(core.HConstraint{V: v}) != nil {
			c.H = core.HConstraint{V: v}
		}
	})
	return c
}

// Entity is the entity with a handle.
func (d *Doc) Entity(h uint32) Entity { return Entity{d, core.HEntity{V: h}} }

// Workplane is one of the three reference workplanes: "XY", "YZ" or
// "ZX".
func (d *Doc) Workplane(name string) Entity {
	switch strings.ToUpper(name) {
	case "XY":
		return Entity{d, sketch.HRequestReferenceXY.Entity(0)}
	case "YZ":
		return Entity{d, sketch.HRequestReferenceYZ.Entity(0)}
	case "ZX", "XZ":
		return Entity{d, sketch.HRequestReferenceZX.Entity(0)}
	}
	fail("no reference workplane '%s': they are XY, YZ and ZX", name)
	return Entity{}
}

// Origin is the point at the origin.
func (d *Doc) Origin() Entity {
	return Entity{d, sketch.HRequestReferenceXY.Entity(1)}
}

// ActiveWorkplane is the workplane that is drawn in; it isn't Valid
// when drawing freely in 3d.
func (d *Doc) ActiveWorkplane() (wp Entity) {
	d.read(func(sk *sketch.Sketch) {
		wp.d = d
		if h := sk.ActiveWorkplane(); !core.IsFreeIn3D(h) {
			wp.H = h
		}
	})
	return wp
}

// SetWorkplane makes wp the workplane that is drawn in, in the active
// group.
func (d *Doc) SetWorkplane(wp Entity) {
	d.read(func(sk *sketch.Sketch) {
		if !wp.get(sk).IsWorkplane() {
			fail("%s isn't a workplane", wp.describe(sk))
		}
		sk.GetGroup(sk.ActiveGroup()).ActiveWorkplane = wp.H
	})
}

// Free3D draws freely in 3d, in no workplane, in the active group.
func (d *Doc) Free3D() {
	d.read(func(sk *sketch.Sketch) {
		sk.GetGroup(sk.ActiveGroup()).ActiveWorkplane = core.HEntity{}
	})
}

// UV is the point at (u, v) in the active workplane.
func (d *Doc) UV(u, v float64) (p Vec) {
	d.read(func(sk *sketch.Sketch) {
		h := sk.ActiveWorkplane()
		if core.IsFreeIn3D(h) {
			fail("UV needs a workplane to be active")
		}
		wp := sk.MustGetEntity(h)
		n := wp.NormalEntity(sk)
		p = wp.WorkplaneGetOffset(sk).
			Plus(n.NormalU(sk).ScaledBy(u)).
			Plus(n.NormalV(sk).ScaledBy(v))
	})
	return p
}

// Selection is what is selected on the drawing, but for the
// constraints.
func (d *Doc) Selection() (entities []Entity) {
	d.read(func(*sketch.Sketch) {
		for _, s := range d.h.Selection() {
			if s.Entity.V != 0 {
				entities = append(entities, Entity{d, s.Entity})
			}
		}
	})
	return entities
}

// SelectedConstraints are the constraints selected on the drawing.
func (d *Doc) SelectedConstraints() (constraints []Constraint) {
	d.read(func(*sketch.Sketch) {
		for _, s := range d.h.Selection() {
			if s.Constraint.V != 0 {
				constraints = append(constraints, Constraint{d, s.Constraint})
			}
		}
	})
	return constraints
}

// Select selects entities on the drawing, and nothing else.
func (d *Doc) Select(entities ...Entity) {
	d.read(func(sk *sketch.Sketch) {
		sel := make([]sketch.Selection, 0, len(entities))
		for _, e := range entities {
			e.get(sk)
			sel = append(sel, sketch.Selection{Entity: e.H})
		}
		d.h.Select(sel)
	})
}

// SelectEdges selects edges of the solid on the drawing, and nothing
// else, as picking them on the model does: for a fillet or a chamfer
// made from the menu. The edges must be of the solid at the active
// group, which such a group is made after.
func (d *Doc) SelectEdges(edges ...SolidEdge) {
	d.read(func(sk *sketch.Sketch) {
		sel := make([]sketch.Selection, 0, len(edges))
		for _, e := range edges {
			e.resolve(sk)
			sel = append(sel, sketch.Selection{Edge: e.name})
		}
		d.h.Select(sel)
	})
}

// SelectFaces selects faces of the solid on the drawing, and nothing
// else, as picking them on the model does: for a shell made from the
// menu, or a face to draw on. A face an entity made is selected by its
// entity, one that no entity names by its identity.
func (d *Doc) SelectFaces(faces ...SolidFace) {
	d.read(func(sk *sketch.Sketch) {
		sel := make([]sketch.Selection, 0, len(faces))
		for _, f := range faces {
			h := core.HEntity{V: uint32(f.id)}
			if sk.GetEntity(h) != nil {
				sel = append(sel, sketch.Selection{Entity: h})
			} else {
				sel = append(sel, sketch.Selection{Face: f.id})
			}
		}
		d.h.Select(sel)
	})
}

// SelectedEdges are the edges of the solid selected on the drawing,
// picked on the model or by SelectEdges, as edges of the solid at the
// active group.
func (d *Doc) SelectedEdges() (edges []SolidEdge) {
	d.read(func(sk *sketch.Sketch) {
		for _, s := range d.h.Selection() {
			if s.Edge.A != 0 {
				edges = append(edges, SolidEdge{s: Solid{d: d, g: sk.ActiveGroup()}, name: s.Edge})
			}
		}
	})
	return edges
}

// SelectedFaces are the faces of the solid selected on the drawing,
// picked on the model or by SelectFaces, as faces of the solid at the
// active group: a face entity's, or a face no entity names by its
// identity. A face is given as the first with its identity.
func (d *Doc) SelectedFaces() (faces []SolidFace) {
	d.read(func(sk *sketch.Sketch) {
		s := Solid{d: d, g: sk.ActiveGroup()}
		for _, x := range d.h.Selection() {
			switch {
			case x.Face != 0:
				faces = append(faces, SolidFace{s: s, id: x.Face})
			case x.Entity.V != 0:
				if e := sk.GetEntity(x.Entity); e != nil && e.IsFace() {
					faces = append(faces, SolidFace{s: s, id: srf.FaceID(x.Entity.V)})
				}
			}
		}
	})
	return faces
}

// String describes the document.
func (d *Doc) String() (s string) {
	d.read(func(sk *sketch.Sketch) {
		s = fmt.Sprintf("sketch: %d groups, %d requests, %d constraints; active %s",
			len(sk.Groups)-1, len(sk.Requests)-3, len(sk.Constraints),
			groupDescription(sk.GetGroup(sk.ActiveGroup())))
	})
	return s
}

// commandList lists the interface's commands, when the program has
// one (SetCommandList).
var commandList func() []string

// SetCommandList is how the user interface tells this package its
// commands; scripts don't call it.
func SetCommandList(list func() []string) { commandList = list }

// Commands lists the commands of the main menu as the command search
// (Edit > Find Command) shows them, "Menu > Entry", for RunCommand;
// none in a program without the interface.
func Commands() []string {
	if commandList == nil {
		return nil
	}
	return commandList()
}

// RunCommand runs a command of the main menu, as choosing it does:
// "View > Zoom To Fit", or the entry's own name, "Zoom To Fit", when
// only one entry has it. telesapr-cli runs scripts on the interface
// without a window, so commands run there too; a document of a Go
// program's own, from New, has no menu to run them from, and it is an
// error. What the command says went wrong is the error too: a command
// that would replace a drawing with unsaved changes, as File > New,
// refuses; see ForgetChanges. What the command changes is part of the
// script's step of undo.
func (d *Doc) RunCommand(label string) (err error) {
	d.h.Run(func() {
		sk := d.h.Sketch()
		undo := sk.Undo.Write
		err = d.h.RunCommand(label)
		if d.h.Sketch() == sk {
			d.h.Remembered(sk.UndoPushedSince(undo))
		}
	})
	return err
}

// Measure measures what it is given, as the description of the
// selection and Analyze > Measure do, without adding a constraint: one
// line segment (its length) or circle (its radius); two points; a point
// and a circle, line segment or plane face; two lines, normals or
// plane faces (their angle in degrees); or two points and a line to
// project their distance on. Lengths are in mm. A face counts by its
// outward normal: a point outside the solid is at a positive distance
// from it, and two opposite faces are at 180 degrees.
func (d *Doc) Measure(entities ...Entity) (m sketch.Measurement) {
	d.read(func(sk *sketch.Sketch) {
		hs := make([]core.HEntity, len(entities))
		for i, e := range entities {
			hs[i] = e.get(sk).H
		}
		var err error
		if m, err = sk.Measure(hs...); err != nil {
			fail("%v", err)
		}
	})
	return m
}

// Units is the document's unit of length: what lengths typed as text,
// in Constraint.SetExpr and Group.SetExpr and in the user interface,
// are in. Every other length this package takes or gives is in mm.
func (d *Doc) Units() (u core.Unit) {
	d.read(func(sk *sketch.Sketch) { u = sk.Units })
	return u
}

// SetUnits makes u the document's unit, keeping every size, as View >
// Dimension Units does: a length expression is rewritten to give the
// same length in u, "(1/4)*25.4" for "1/4" from inches to mm. It
// returns the parameters that lengths are computed from, which now
// count in u.
func (d *Doc) SetUnits(u core.Unit) (check []string) {
	d.change(func(sk *sketch.Sketch) { check = sk.SetUnits(u) })
	return check
}

// BrokenReference is something a regeneration deleted because what it
// used was gone: a group whose sketch was deleted, or a constraint on a
// line that was.
type BrokenReference struct {
	// Group is the group it was in, or the deleted group itself; it
	// may be gone too.
	Group Group
	// Item names what was deleted: "c012-pt-pt-distance" or
	// "g005-extrude".
	Item string
	// Missing names what it used: "r005-line-segment of
	// g002-sketch-in-plane", or a group.
	Missing string
	// Text says it all in a line: "deleted g005-extrude, which used
	// g004-sketch-in-plane".
	Text string
}

// BrokenReferences lists what the last regeneration that deleted
// something deleted, as Edit > Broken References shows it. The list
// stays until a regeneration deletes something else, an undo or redo,
// or ClearBrokenReferences. Undo brings the deleted items back.
func (d *Doc) BrokenReferences() (refs []BrokenReference) {
	d.read(func(sk *sketch.Sketch) {
		for _, b := range sk.BrokenReferences {
			refs = append(refs, BrokenReference{Group: Group{d, b.Group}, Item: b.Item, Missing: b.Missing, Text: b.String()})
		}
	})
	return refs
}

// ClearBrokenReferences empties the list BrokenReferences gives.
func (d *Doc) ClearBrokenReferences() {
	d.h.Run(func() { d.h.Sketch().ClearBrokenReferences() })
}

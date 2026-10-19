package api

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Vec is a point or a direction in space, in mm.
type Vec = geom.Vector

// V is the vector (x, y, z).
func V(x, y, z float64) Vec { return Vec{X: x, Y: y, Z: z} }

// Inch is v inches in mm.
func Inch(v float64) float64 { return v * 25.4 }

// Doc is a sketch: the drawing a script reads and changes. Its own
// methods find what the drawing holds; what changes it is in the
// fields, one for each menu of the window, so that a call is where
// the command it does is: doc.Sketch.Line is Sketch > Line Segment,
// doc.Features.Extrude is Features > Extrude.
type Doc struct {
	h Host

	File      FileMenu
	Edit      EditMenu
	Params    ParamsMenu
	View      ViewMenu
	Drawing   DrawingMenu
	Sketch    SketchMenu
	Constrain ConstrainMenu
	Features  FeaturesMenu
	Assembly  AssemblyMenu
	Analyze   AnalyzeMenu
}

// NewDoc is the document of a host. The application makes the one a
// script gets from Current; a program of its own makes them with New
// and Open.
func NewDoc(h Host) *Doc {
	d := &Doc{h: h}
	d.File = FileMenu{d}
	d.Edit = EditMenu{d}
	d.Params = ParamsMenu{d}
	d.View = ViewMenu{d}
	d.Drawing = DrawingMenu{d}
	d.Sketch = SketchMenu{d}
	d.Constrain = ConstrainMenu{d}
	d.Features = FeaturesMenu{d}
	d.Assembly = AssemblyMenu{d}
	d.Analyze = AnalyzeMenu{d}
	return d
}

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
		slices.Sort(failed)
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

// readMesh is read for what measures or returns the solid's mesh: the
// solids of groups a change after them left at another tolerance are
// made again first (Sketch.RemakeStaleMeshes).
func (d *Doc) readMesh(fn func(sk *sketch.Sketch)) {
	d.read(func(sk *sketch.Sketch) {
		sk.RemakeStaleMeshes()
		fn(sk)
	})
}

// change runs fn, which changes the sketch.
func (d *Doc) change(fn func(sk *sketch.Sketch)) {
	d.h.Run(func() {
		d.h.Changing()
		fn(d.h.Sketch())
	})
}

// exportKinds says what each file extension holds; the ones that can
// hold several kinds are exported as the first.
var exportKinds = map[string]ExportKind{
	".stl": ExportMesh, ".3mf": ExportMesh, ".obj": ExportMesh, ".js": ExportMesh, ".html": ExportMesh, ".wrl": ExportMesh,
	".svg": ExportView, ".dxf": ExportView, ".pdf": ExportView, ".eps": ExportView, ".ps": ExportView,
	".plt": ExportView, ".hpgl": ExportView, ".ngc": ExportView, ".txt": ExportView,
	".step": ExportSurfaces, ".stp": ExportSurfaces,
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
		if found := sk.GroupNamed(name); found != nil {
			g.H = found.H
		}
	})
	return g
}

// Request is the request with a description as the property browser
// shows it: "r004-line-segment" or just "r004".
func (d *Doc) Request(name string) (r Request) {
	d.read(func(sk *sketch.Sketch) {
		r.d = d
		if v, ok := handleOf(name, 'r'); ok && sk.Request(core.HRequest{V: v}) != nil {
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
		if v, ok := handleOf(name, 'c'); ok && sk.Constraint(core.HConstraint{V: v}) != nil {
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

// String describes the document.
func (d *Doc) String() (s string) {
	d.read(func(sk *sketch.Sketch) {
		s = fmt.Sprintf("sketch: %d groups, %d requests, %d constraints; active %s",
			len(sk.Groups)-1, len(sk.Requests)-3, len(sk.Constraints),
			groupDescription(sk.Group(sk.ActiveGroup())))
	})
	return s
}

// Commands lists the commands of the main menu as the command search
// (Edit > Find Command) shows them, "Menu > Entry", for RunCommand.
// In a script the interpreter binds it to the host's; a program
// compiled on its own has no interface, and none.
func Commands() []string { return nil }

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

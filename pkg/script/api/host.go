// Package api is what a script sees of a sketch: a Doc and the groups,
// requests, entities, constraints and parameters in it. Those are held
// by handle, so they stay valid when the sketch regenerates, and are
// looked up again at every call; a Doc therefore always shows the
// drawing as it is.
//
// Lengths and coordinates are in millimetres, whatever the document's
// unit, and angles in degrees. Only text typed for a value, as the
// SetExpr methods take, is in the document's unit (Doc.Units), as it
// is in the user interface.
//
// A call that can't do what it was asked, because of what it was given
// or of the sketch's state, panics with an error; the interpreter
// reports it and goes on. What reads or writes files returns an error
// instead.
//
// The package is plain Go: a script that works can be compiled as it
// is.
package api

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// ExportKind is what Export writes.
type ExportKind int

const (
	// ExportMesh is the triangle mesh of the solid (STL, OBJ, ...).
	ExportMesh ExportKind = iota
	// ExportView is the 2d view of the sketch (SVG, DXF, PDF, ...).
	ExportView
	// ExportWireframe is the 3d wireframe (DXF, STEP).
	ExportWireframe
	// ExportSurfaces is the exact surfaces of the solid (STEP).
	ExportSurfaces
)

// Host owns a sketch on behalf of a Doc: it is the program around the
// sketch, with or without windows. All of it but Run must be called
// inside Run.
type Host interface {
	// Sketch is the sketch as it is now; it changes when a file loads.
	Sketch() *sketch.Sketch
	// Run runs fn on the goroutine that owns the sketch and waits for
	// it. A panic in fn is raised in the caller.
	Run(fn func())
	// Changing is called before each change to the sketch, so that the
	// first one in a script's step can be remembered for undo.
	Changing()
	// Regenerate regenerates what is dirty, now.
	Regenerate()
	// AddGroup adds g after the active group, activates it and
	// regenerates.
	AddGroup(g sketch.Group) core.HGroup
	// DeleteGroup deletes a group, and with it what depends on it.
	DeleteGroup(hg core.HGroup)
	// Activate makes a group the one that is worked in.
	Activate(hg core.HGroup)
	Undo() bool
	Redo() bool
	// RunCommand runs a menu command by its label; an error on a host
	// with no window.
	RunCommand(label string) error
	// MmPerUnit is the size in mm of the unit lengths are shown in.
	MmPerUnit() float64
	Load(path string) error
	Save(path string) error
	Export(kind ExportKind, path string) error
	// Render draws the sketch, seen with right to the right and up
	// up and fitted to the picture, into a PNG file.
	Render(path string, width, height int, right, up geom.Vector) error
	// Selection is what is selected in the user interface, if any.
	Selection() []sketch.Selection
	Select(sel []sketch.Selection)
	// Hovered is what the mouse is over in the user interface, if
	// anything.
	Hovered() sketch.Selection
	// SetChordTolerance sets how closely meshes follow curves, as a
	// percentage of the model's size: for the application, its setting,
	// as Configuration sets it, which it gives every sketch it
	// regenerates; for a bare sketch, the sketch's own.
	SetChordTolerance(pct float64)
}

// errUnsupported is what a sketch without a program around it answers
// to what only the program can do.
var errUnsupported = errors.New("not available for a sketch opened without the application")

// SketchHost is a Host for a bare sketch, with no user interface: what
// Open and New make outside the application.
type SketchHost struct {
	sk       *sketch.Sketch
	saveFile string
	changed  bool
}

var _ Host = (*SketchHost)(nil)

// NewSketchHost hosts sk, which was loaded from or will be saved to
// saveFile ("" for none).
func NewSketchHost(sk *sketch.Sketch, saveFile string) *SketchHost {
	h := &SketchHost{sk: sk, saveFile: saveFile}
	h.adopt(sk)
	return h
}

func (h *SketchHost) adopt(sk *sketch.Sketch) {
	h.sk = sk
	sk.ReloadAllLinked = func(sk *sketch.Sketch) { link.ReloadAllLinked(sk, h.saveFile) }
	sk.GenerateAll()
	sk.Active = sk.ActiveGroup()
	if g := sk.GetGroup(sk.Active); g != nil && g.Type == sketch.GroupDrawingWorkplane &&
		g.ActiveWorkplane.V == 0 {
		g.ActiveWorkplane = g.H.Entity(0)
	}
	sk.Unsaved = false
}

// Sketch is the hosted sketch.
func (h *SketchHost) Sketch() *sketch.Sketch { return h.sk }

// Run runs fn at once: a bare sketch has no goroutine of its own.
func (h *SketchHost) Run(fn func()) { fn() }

// BeginEval starts a step of a script: its first change is remembered
// for undo.
func (h *SketchHost) BeginEval() { h.changed = false }

// EndEval ends a step of a script: what it changed is regenerated.
func (h *SketchHost) EndEval() { h.Regenerate() }

// Changing remembers the sketch for undo before the first change of
// a script's step.
func (h *SketchHost) Changing() {
	if !h.changed {
		h.changed = true
		h.sk.UndoRemember()
	}
}

// Rollback takes back what the script's last step changed, for good:
// it can't be done again as what Undo takes back can. It is false if
// the step changed nothing.
func (h *SketchHost) Rollback() bool {
	if !h.changed {
		return false
	}
	h.changed = false
	if !h.sk.UndoUndo() {
		return false
	}
	h.sk.UndoClearStack(&h.sk.Redo)
	return true
}

// Regenerate regenerates what is dirty.
func (h *SketchHost) Regenerate() { h.sk.Refresh() }

// AddGroup adds g after the active group, activates it, and
// regenerates.
func (h *SketchHost) AddGroup(g sketch.Group) core.HGroup {
	sk := h.sk
	hg := sk.AddGroupAfterActive(g)
	gg := sk.GetGroup(hg)
	if gg.Type == sketch.GroupLinked && sk.ReloadAllLinked != nil {
		sk.ReloadAllLinked(sk)
	}
	gg.Clean = false
	sk.Active = hg
	sk.GenerateAll()
	gg = sk.GetGroup(hg)
	if gg != nil && gg.Type == sketch.GroupDrawingWorkplane {
		gg.ActiveWorkplane = gg.H.Entity(0)
	}
	return hg
}

// DeleteGroup deletes a group, activating the one before it if it was
// active, and regenerates.
func (h *SketchHost) DeleteGroup(hg core.HGroup) {
	sk := h.sk
	if hg == sk.Active {
		if prev := sk.GetGroup(hg).PreviousGroup(sk); prev != nil {
			sk.Active = prev.H
		}
	}
	for i := range sk.Groups {
		if sk.Groups[i].H == hg {
			sk.Groups = append(sk.Groups[:i], sk.Groups[i+1:]...)
			break
		}
	}
	sk.GenerateAll()
}

// Activate makes a group the active one.
func (h *SketchHost) Activate(hg core.HGroup) {
	h.sk.Active = hg
	h.sk.MarkGroupDirty(hg, false)
}

// Undo takes back the last step, and reports whether there was one.
func (h *SketchHost) Undo() bool {
	// What the script's step did so far is gone with it.
	h.changed = false
	return h.sk.UndoUndo()
}

// Redo does again the last step undone, and reports whether there was
// one.
func (h *SketchHost) Redo() bool {
	h.changed = false
	return h.sk.UndoRedo()
}

// MmPerUnit is 1: with no interface, lengths are shown in mm.
func (h *SketchHost) MmPerUnit() float64 { return h.sk.Units.MmPer() }

// Load replaces the sketch by the one in a .tlspr file.
func (h *SketchHost) Load(path string) error {
	doc, err := loadModel(path)
	if err != nil {
		return err
	}
	h.saveFile = path
	h.adopt(doc.Sketch)
	return nil
}

// loadModel opens a .tlspr file. A .slvs file is refused, since it is
// imported, and so is a file with parts this version doesn't know,
// which a save would lose.
func loadModel(path string) (*tlspr.Document, error) {
	if strings.EqualFold(filepath.Ext(path), ".slvs") {
		return nil, fmt.Errorf("%s is a SolveSpace model; convert it with Import first", path)
	}
	doc, err := tlspr.LoadFile(path)
	if err != nil {
		return nil, err
	}
	if len(doc.Warnings) > 0 {
		var b strings.Builder
		for _, w := range doc.Warnings {
			b.WriteString("\n  ")
			b.WriteString(w.Error())
		}
		return nil, fmt.Errorf("%s has parts this version doesn't know, which a save would lose:%s", path, b.String())
	}
	return doc, nil
}

// checkSavePath refuses to save anywhere but a .tlspr file.
func checkSavePath(path string) error {
	switch ext := filepath.Ext(path); {
	case strings.EqualFold(ext, "."+tlspr.Extension):
		return nil
	case strings.EqualFold(ext, ".slvs"):
		return fmt.Errorf("can't save %s: TeleSAPR doesn't write SolveSpace models; save as .%s", path, tlspr.Extension)
	default:
		return fmt.Errorf("can't save %s: a sketch is saved as a .%s file", path, tlspr.Extension)
	}
}

// Save writes the sketch to a .tlspr file, its links made relative to
// it.
func (h *SketchHost) Save(path string) error {
	if err := checkSavePath(path); err != nil {
		return err
	}
	h.sk.GenerateAll()
	err := tlspr.SaveFile(h.sk, h.saveFile, path, tlspr.Options{})
	var le *tlspr.LinkError
	if errors.As(err, &le) {
		return fmt.Errorf("the sketch links '%s', so it can only be saved on the same volume", le.Link)
	}
	if err != nil {
		return err
	}
	h.saveFile = path
	h.sk.Unsaved = false
	return nil
}

// Export writes the mesh as STL; the other formats need the
// application around the sketch.
func (h *SketchHost) Export(kind ExportKind, path string) error {
	if ext := filepath.Ext(path); kind != ExportMesh || !strings.EqualFold(ext, ".stl") && !strings.EqualFold(ext, ".3mf") {
		return fmt.Errorf("export of %s: %w", filepath.Base(path), errUnsupported)
	}
	return h.sk.ExportMeshTo(path, sketch.ThawExportSettings(core.GetSettings()))
}

// Render fails: drawing needs the application around the sketch.
func (h *SketchHost) Render(path string, _, _ int, _, _ geom.Vector) error {
	return fmt.Errorf("picture %s: %w", filepath.Base(path), errUnsupported)
}

// Selection is nothing: there is no interface to select in.
func (h *SketchHost) Selection() []sketch.Selection { return nil }

// Select does nothing, for the same reason.
func (h *SketchHost) Select([]sketch.Selection) {}

// Hovered is nothing, for the same reason.
func (h *SketchHost) Hovered() sketch.Selection { return sketch.Selection{} }

// SetChordTolerance sets the sketch's chord tolerance.
func (h *SketchHost) SetChordTolerance(pct float64) { h.sk.ChordTolPct = pct }

// RunCommand is an error: a sketch with no window has no menu.
func (h *SketchHost) RunCommand(label string) error {
	return fmt.Errorf("no window to run %q in: commands need the application", label)
}

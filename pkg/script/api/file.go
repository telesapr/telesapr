package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/dxf"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// FileMenu holds the calls of the File menu: loading, saving and
// exporting the drawing. A script reaches it as doc.File.
type FileMenu struct{ d *Doc }

// Load replaces the sketch by the one in a .tlspr file.
func (m FileMenu) Load(path string) (err error) {
	d := m.d
	if err := d.checkFile(path, false); err != nil {
		return err
	}
	d.h.Run(func() { err = d.h.Load(path) })
	return err
}

// Save regenerates the sketch and writes it to a .tlspr file. A
// SolveSpace model (.slvs) is not written.
func (m FileMenu) Save(path string) (err error) {
	d := m.d
	if err := d.checkFile(path, true); err != nil {
		return err
	}
	d.h.Run(func() { err = d.h.Save(path) })
	return err
}

// Export writes the sketch in the format the file's extension names:
// the mesh of the solid (.stl, .obj, ...), the view (.svg, .dxf, .pdf,
// ...), the surfaces of the solid (.step), or a picture as Render
// makes it (.png).
func (m FileMenu) Export(path string) error {
	d := m.d
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return d.File.Render(path, 1024, 768, "isometric")
	}
	kind, ok := exportKinds[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return fmt.Errorf("can't tell what to export from the name '%s'", path)
	}
	return d.File.ExportAs(kind, path)
}

// ExportAs is Export for a format that can hold several things, such
// as a wireframe in a .dxf or a .step.
func (m FileMenu) ExportAs(kind ExportKind, path string) (err error) {
	d := m.d
	if err := d.checkFile(path, true); err != nil {
		return err
	}
	d.read(func(*sketch.Sketch) { err = d.h.Export(kind, path) })
	return err
}

// ExportSection writes the section of the active group's solid by a
// plane as a 2d file (.svg, .dxf, .pdf, ...), as File > Export > 2d
// Section does. The plane is the section view's (Doc.Group views'
// SetSection) when it is on, and otherwise the active workplane; its
// normal is the section's, so a flipped section view lays the section
// out mirrored. It needs the application's export writers, as the other
// 2d exports do.
func (m FileMenu) ExportSection(path string) error {
	return m.ExportAs(ExportSection, path)
}

// Render draws the sketch into a PNG file of width by height pixels,
// fitted to it and seen from "top", "bottom", "left", "right",
// "front", "back" or "isometric"; or "window", as the application's
// window shows it, the same middle and as much of it across.
func (m FileMenu) Render(path string, width, height int, view string) (err error) {
	d := m.d
	v, ok := views[strings.ToLower(view)]
	if !ok {
		return fmt.Errorf("no view '%s': they are top, bottom, left, right, front, back, isometric and window", view)
	}
	if width < 1 || height < 1 {
		return fmt.Errorf("a picture of %d by %d pixels", width, height)
	}
	if err := d.checkFile(path, true); err != nil {
		return err
	}
	d.read(func(*sketch.Sketch) { err = d.h.Render(path, width, height, v[0], v[1]) })
	return err
}

// Unsaved reports whether the sketch changed since it was saved.
func (m FileMenu) Unsaved() (unsaved bool) {
	d := m.d
	d.h.Run(func() { unsaved = d.h.Sketch().Unsaved })
	return unsaved
}

// ForgetChanges lets the changes made since the sketch was saved go, as
// "Don't Save" does when the application asks: File > New, Open and
// the others that replace the drawing refuse to run from a script while
// it has unsaved changes, so that nothing is lost unasked. The
// drawing itself stays as it is until such a command replaces it.
func (m FileMenu) ForgetChanges() {
	d := m.d
	d.h.Run(func() {
		if f, ok := d.h.(interface{ ForgetChanges() }); ok {
			f.ForgetChanges()
			return
		}
		d.h.Sketch().Unsaved = false
	})
}

// ImportDXF adds the entities of a DXF file to the active group, as
// File > Import does: in the group's workplane, or, when the file has
// entities off the XY plane, as construction entities in 3d, leaving the
// workplane (undo brings it back). It returns how many entities of
// kinds the importer doesn't know were left out. It is one step of undo.
func (m FileMenu) ImportDXF(path string) (ignored int, err error) {
	d := m.d
	if err := d.checkFile(path, false); err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", filepath.Base(path), err)
	}
	d.change(func(sk *sketch.Sketch) {
		scale := sketch.DefaultView().Scale
		if sk.View != nil {
			scale = sk.View.Scale
		}
		res, ierr := dxf.ImportActive(sk, data, sk.StyleTextHeight(core.HStyle{V: core.StyleConstraint}, scale))
		if ierr != nil {
			err = fmt.Errorf("%s: %w", filepath.Base(path), ierr)
			return
		}
		ignored = res.Unknown
	})
	return ignored, err
}

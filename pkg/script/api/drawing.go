package api

import (
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Drawing is the model's sheet of views (roadmap step 18): its size,
// scale, projection, views and title block. See sketch.Drawing.
type Drawing = sketch.Drawing

// DrawingView is a view on the sheet, see sketch.DrawingView.
type DrawingView = sketch.DrawingView

// Drawing is the model's drawing, or the default one, an A3 sheet
// fitted, if it has none yet. It is a copy: SetDrawing changes it.
func (d *Doc) Drawing() (dr Drawing) {
	d.read(func(sk *sketch.Sketch) {
		if sk.Drawing != nil {
			dr = *sk.Drawing.Clone()
		} else {
			dr = *sketch.DefaultDrawing()
		}
	})
	return dr
}

// SetDrawing gives the model the drawing dr: a sheet A4 to A0, Letter
// or Tabloid; a scale, 0 to fit; views, none for the standard ones
// placed by the projection.
func (d *Doc) SetDrawing(dr Drawing) {
	if err := dr.Check(); err != nil {
		fail("%v", err)
	}
	d.change(func(sk *sketch.Sketch) { sk.Drawing = dr.Clone() })
}

// ExportDrawing writes the model's drawing to a vector file, by its
// extension: .pdf, .svg, .dxf, .eps, .ps, .plt. It needs the
// application, or telesapr-cli export-drawing.
func (d *Doc) ExportDrawing(path string) (err error) {
	dr := d.Drawing()
	d.read(func(*sketch.Sketch) { err = d.h.ExportDrawing(path, &dr) })
	return err
}

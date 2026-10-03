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

// DrawingDimension is a dimension on a view of the drawing, see
// sketch.DrawingDimension.
type DrawingDimension = sketch.DrawingDimension

// AddDrawingDimension adds a dimension to the drawing's view: "length"
// of an edge, "distance" between two, "diameter" or "radius" of a
// circular one; offset is how far its line stands off what it measures,
// in sheet mm, out from the view's middle (0 for 10; negative, in toward
// it). The dimension holds
// the edges' names, so it measures them again as the model changes.
func (d *Doc) AddDrawingDimension(view, kind string, offset float64, edges ...SolidEdge) {
	dr := d.Drawing()
	dm := DrawingDimension{View: view, Kind: kind, Offset: offset}
	for _, e := range edges {
		dm.Edges = append(dm.Edges, e.name)
	}
	dr.Dimensions = append(dr.Dimensions, dm)
	d.SetDrawing(dr)
}

// DrawingDimensions measures the drawing's dimensions on the solid as
// it is: each one's value in mm, and an error for one that can't be
// measured, as for an edge no longer in the solid. It needs the
// application, or a view of it.
func (d *Doc) DrawingDimensions() (values []float64, errs []error) {
	dr := d.Drawing()
	d.read(func(*sketch.Sketch) { values, errs = d.h.MeasureDrawing(&dr) })
	return values, errs
}

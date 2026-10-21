package api

import (
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Drawing is the model's sheet of views (roadmap step 18): its size,
// scale, projection, views and title block. See sketch.Drawing.
type Drawing = sketch.Drawing

// DrawingView is a view on the sheet, see sketch.DrawingView.
type DrawingView = sketch.DrawingView

// DrawingDimension is a dimension on a view of the drawing, see
// sketch.DrawingDimension.
type DrawingDimension = sketch.DrawingDimension

// DrawingMenu holds the calls of View > Drawing Sheet and File > Export
// > Drawing: the sheet of views, sections and dimensions. A script
// reaches it as doc.Drawing.
type DrawingMenu struct{ d *Doc }

// Get is the model's drawing, or the default one, an A3 sheet
// fitted, if it has none yet. It is a copy: SetDrawing changes it.
func (m DrawingMenu) Get() (dr Drawing) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		if sk.Drawing != nil {
			dr = *sk.Drawing.Clone()
		} else {
			dr = *sketch.DefaultDrawing()
		}
	})
	return dr
}

// Set gives the model the drawing dr: a sheet A4 to A0, Letter
// or Tabloid; a scale, 0 to fit; views, none for the standard ones
// placed by the projection.
func (m DrawingMenu) Set(dr Drawing) {
	d := m.d
	if err := dr.Check(); err != nil {
		fail("%v", err)
	}
	d.change(func(sk *sketch.Sketch) { sk.Drawing = dr.Clone() })
}

// Export writes the model's drawing to a vector file, by its
// extension: .pdf, .svg, .dxf, .eps, .ps, .plt. It needs the
// application, or telesapr-cli export-drawing.
func (m DrawingMenu) Export(path string) (err error) {
	d := m.d
	if err := d.checkFile(path, true); err != nil {
		return err
	}
	dr := d.Drawing.Get()
	d.read(func(*sketch.Sketch) { err = d.h.ExportDrawing(path, &dr) })
	return err
}

// AddDimension adds a dimension to the drawing's view: "length"
// of an edge, "distance" between two, "diameter" or "radius" of a
// circular one; offset is how far its line stands off what it measures,
// in sheet mm, out from the view's middle (0 for 10; negative, in toward
// it). The dimension holds
// the edges' names, so it measures them again as the model changes.
// The view is front, back, top, bottom, left, right or iso, where a
// length is the edge's in the model, not as foreshortened, or a section
// or a detail by its letter (A, B, ...: the sections first).
func (m DrawingMenu) AddDimension(view, kind string, offset float64, edges ...SolidEdge) {
	d := m.d
	dr := d.Drawing.Get()
	dm := DrawingDimension{View: view, Kind: kind, Offset: offset}
	for _, e := range edges {
		dm.Edges = append(dm.Edges, e.name)
	}
	dr.Dimensions = append(dr.Dimensions, dm)
	d.Drawing.Set(dr)
}

// Dimensions measures the drawing's dimensions on the solid as
// it is: each one's value in mm, and an error for one that can't be
// measured, as for an edge no longer in the solid. It needs the
// application, or a view of it.
func (m DrawingMenu) Dimensions() (values []float64, errs []error) {
	d := m.d
	dr := d.Drawing.Get()
	d.read(func(*sketch.Sketch) { values, errs = d.h.MeasureDrawing(&dr) })
	return values, errs
}

// AddFaceDimension adds a dimension of faces to the drawing's
// view, as AddDrawingDimension does of edges: "distance" between two
// parallel flat faces, "diameter" or "radius" of a face's circular
// edge, the largest, as a cylinder's or a round end's. The dimension
// holds the faces' identities, so it measures them again as the model
// changes.
func (m DrawingMenu) AddFaceDimension(view, kind string, offset float64, faces ...SolidFace) {
	d := m.d
	dr := d.Drawing.Get()
	dm := DrawingDimension{View: view, Kind: kind, Offset: offset}
	for _, f := range faces {
		dm.Faces = append(dm.Faces, f.id)
	}
	dr.Dimensions = append(dr.Dimensions, dm)
	d.Drawing.Set(dr)
}

// AddEdgeFaceDistance adds to the drawing's view the distance
// from a straight edge to a flat face it runs parallel to, as
// AddDrawingDimension does between edges.
func (m DrawingMenu) AddEdgeFaceDistance(view string, offset float64, edge SolidEdge, face SolidFace) {
	d := m.d
	dr := d.Drawing.Get()
	dr.Dimensions = append(dr.Dimensions, DrawingDimension{View: view, Kind: "distance", Offset: offset,
		Edges: []topo.Name{edge.name}, Faces: []srf.FaceID{face.id}})
	d.Drawing.Set(dr)
}

// Show shows the drawing sheet in the window in place of
// the model, as View > Show Drawing Sheet does, or the model again; it
// is view state, not saved. It is an error for a bare sketch with no
// application around it (New), or when the sheet can't be made, as when
// the active group has no solid. With the sheet shown, Render with the view
// "window" pictures the sheet.
func (m DrawingMenu) Show(show bool) (err error) {
	d := m.d
	d.read(func(*sketch.Sketch) { err = d.h.ShowDrawingSheet(show) })
	return err
}

// Shown reports whether the window shows the drawing sheet.
func (m DrawingMenu) Shown() (shown bool) {
	d := m.d
	d.read(func(*sketch.Sketch) { shown = d.h.DrawingSheetShown() })
	return shown
}

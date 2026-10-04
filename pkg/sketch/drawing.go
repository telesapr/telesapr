package sketch

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A drawing (roadmap step 18) is a sheet of paper with views of the
// solid: the standard ones (front, top, a side and an isometric)
// placed by the projection's convention, at a scale, in a border with a
// title block. It is part of the model, saved with it; what is drawn is
// made from the solid each time it is exported (pkg/ui, drawing.go).

// DrawingView is a view on the sheet: what it looks at, from where,
// and where its middle is on the sheet, in mm from the bottom left.
type DrawingView struct {
	// Kind is front, back, top, bottom, left, right or iso.
	Kind string
	X, Y float64
}

// DrawingSection is a section view (roadmap 18.3): the solid cut by
// the plane square to View's direction through the solid's middle,
// moved Offset mm toward the viewer, the half beyond seen, the cut face
// hatched; at X, Y on the sheet, or placed when both are 0.
type DrawingSection struct {
	View         string
	Offset, X, Y float64
}

// DrawingDetail is a detail view (roadmap 18.3): the part of View
// within R mm of the point CX, CY (mm right and up from the view's
// middle, on the model), enlarged Scale times the sheet's scale; at X,
// Y on the sheet, or placed when both are 0.
type DrawingDetail struct {
	View                   string
	CX, CY, R, Scale, X, Y float64
}

// DrawingDimension is a dimension on a view of the drawing (roadmap
// 18.4, H2): the length of an edge, the distance between two, or the
// diameter or radius of a circular one, the edges held by their names
// (step 9), so that it measures them again as the model changes. Offset
// is how far, in sheet mm, the dimension line stands off what it
// measures, out from the view's middle (in toward it when negative; 0
// for 10).
//
// Since roadmap 29.2 a dimension may also measure faces, by their
// identity: the distance between two flat faces, or between a flat face
// and a straight edge, and the diameter or radius of a face's circular
// edge, as a cylinder's or a hole's; what it measures is its Edges, then
// its Faces. Its View may be the isometric, where it gives the length
// in the model, not as foreshortened, or a section or a detail by its
// letter.
type DrawingDimension struct {
	View   string
	Kind   string
	Edges  []topo.Name
	Faces  []srf.FaceID
	Offset float64
}

// DimensionKinds are the kinds of drawing dimension and how many edges
// each takes.
var DimensionKinds = map[string]int{"length": 1, "distance": 2, "diameter": 1, "radius": 1}

// Drawing is the model's sheet.
type Drawing struct {
	// Sheet is A4 to A0, Letter or Tabloid; Portrait turns it.
	Sheet    string
	Portrait bool
	// Scale is mm on paper per mm of the model; 0 picks the largest
	// standard one at which the views fit.
	Scale float64
	// FirstAngle places the views as ISO's first-angle projection
	// does (the top view below the front, the left view on the right);
	// otherwise third-angle (top above, right on the right).
	FirstAngle bool
	// Views, when not empty, are the views and where they are; empty,
	// the standard views are placed (Layout).
	Views []DrawingView
	// Hidden draws the hidden edges of the orthographic views, dashed.
	Hidden bool
	// Sections and Details are the section and detail views, lettered
	// A, B, ... in that order.
	Sections []DrawingSection
	Details  []DrawingDetail
	// Dimensions are the dimensions on the views.
	Dimensions []DrawingDimension
	// The title block (roadmap 18.2).
	Title, Author, Number, Revision, Material, Company string
}

// SheetSize is the size of a sheet in mm, landscape.
func SheetSize(name string) (w, h float64, ok bool) {
	switch strings.ToUpper(name) {
	case "A4":
		return 297, 210, true
	case "A3":
		return 420, 297, true
	case "A2":
		return 594, 420, true
	case "A1":
		return 841, 594, true
	case "A0":
		return 1189, 841, true
	case "LETTER":
		return 279.4, 215.9, true
	case "TABLOID":
		return 431.8, 279.4, true
	}
	return 0, 0, false
}

// Size is the drawing's sheet in mm, turned as it is.
func (d *Drawing) Size() (w, h float64) {
	w, h, ok := SheetSize(d.Sheet)
	if !ok {
		w, h, _ = SheetSize("A3")
	}
	if d.Portrait {
		w, h = h, w
	}
	return w, h
}

// Check reports what is wrong with the drawing's settings.
func (d *Drawing) Check() error {
	if _, _, ok := SheetSize(d.Sheet); !ok {
		return fmt.Errorf("no sheet %q: the sheets are A4, A3, A2, A1, A0, Letter and Tabloid", d.Sheet)
	}
	if d.Scale < 0 || math.IsNaN(d.Scale) || math.IsInf(d.Scale, 0) {
		return fmt.Errorf("a drawing's scale is more than zero, or zero to fit, not %g", d.Scale)
	}
	for _, v := range d.Views {
		if _, _, ok := ViewDirection(v.Kind); !ok {
			return fmt.Errorf("no view %q: the views are front, back, top, bottom, left, right and iso", v.Kind)
		}
	}
	for _, s := range d.Sections {
		if _, _, ok := ViewDirection(s.View); !ok || s.View == "iso" {
			return fmt.Errorf("no section of %q: a section is of front, back, top, bottom, left or right", s.View)
		}
	}
	for _, dt := range d.Details {
		if _, _, ok := ViewDirection(dt.View); !ok {
			return fmt.Errorf("no detail of %q: the views are front, back, top, bottom, left, right and iso", dt.View)
		}
		if !(dt.R > 0) || !(dt.Scale > 0) {
			return fmt.Errorf("a detail's radius and scale are more than zero, not %g and %g", dt.R, dt.Scale)
		}
	}
	for _, dm := range d.Dimensions {
		if _, ok := d.DimensionView(dm.View); !ok {
			return fmt.Errorf("no dimension on %q: dimensions are on front, back, top, bottom, left, right or iso, "+
				"or on a section or detail by its letter", dm.View)
		}
		n, ok := DimensionKinds[dm.Kind]
		if !ok {
			return fmt.Errorf("no dimension %q: they are length, distance, diameter and radius", dm.Kind)
		}
		if got := len(dm.Edges) + len(dm.Faces); got != n {
			return fmt.Errorf("a %s dimension measures %d edge%s or face%s, not %d", dm.Kind, n,
				map[bool]string{true: "s"}[n > 1], map[bool]string{true: "s"}[n > 1], got)
		}
		if dm.Kind == "length" && len(dm.Faces) > 0 {
			return errors.New("a length is of an edge, not of a face")
		}
	}
	return nil
}

// DimensionView is what a dimension's view names: a view kind (front,
// ..., iso), or the n-th section or detail by its letter, sections
// first; n is -1 for a view kind, and ok false when it names none.
func (d *Drawing) DimensionView(view string) (n int, ok bool) {
	if _, _, ok := ViewDirection(view); ok {
		return -1, true
	}
	for i := range len(d.Sections) + len(d.Details) {
		if Letter(i) == view {
			return i, true
		}
	}
	return -1, false
}

// ViewDirection is what is to the right and what is up in a view.
func ViewDirection(kind string) (right, up [3]float64, ok bool) {
	switch kind {
	case "front":
		return [3]float64{1, 0, 0}, [3]float64{0, 1, 0}, true
	case "back":
		return [3]float64{-1, 0, 0}, [3]float64{0, 1, 0}, true
	case "top":
		return [3]float64{1, 0, 0}, [3]float64{0, 0, -1}, true
	case "bottom":
		return [3]float64{1, 0, 0}, [3]float64{0, 0, 1}, true
	case "right":
		return [3]float64{0, 0, -1}, [3]float64{0, 1, 0}, true
	case "left":
		return [3]float64{0, 0, 1}, [3]float64{0, 1, 0}, true
	case "iso":
		return [3]float64{0.7071067811865476, 0, -0.7071067811865476},
			[3]float64{-0.4082482904638631, 0.8164965809277261, -0.4082482904638631}, true
	}
	return right, up, false
}

// StandardScales are the scales a drawing picks from, largest first.
var StandardScales = []float64{10, 5, 2, 1, 0.5, 0.2, 0.1, 0.05, 0.02, 0.01, 0.005, 0.002, 0.001}

// ScaleText is a scale as a drawing writes it: 1:2, 5:1.
func ScaleText(s float64) string {
	if s >= 1 {
		return fmt.Sprintf("%g:1", s)
	}
	return fmt.Sprintf("1:%g", math.Round(1/s*1000)/1000)
}

// TitleBlockSize is the title block's size in mm, at the sheet's bottom
// right inside the border.
const (
	TitleBlockWidth  = 180.0
	TitleBlockHeight = 40.0
	// SheetBorder is the margin from the paper's edge to the frame.
	SheetBorder = 10.0
	// viewGap is the space between views.
	viewGap = 15.0
)

// Layout places the views: the drawing's own, or the standard ones,
// front, top, a side and the isometric, by the projection's convention,
// given each view's width and height on the model in mm (sizes, by
// kind). It returns the views and the scale, which is the drawing's or,
// for 0, the largest standard one at which they fit above the title
// block.
func (d *Drawing) Layout(sizes map[string][2]float64) ([]DrawingView, float64) {
	w, h := d.Size()
	if len(d.Views) > 0 {
		scale := d.Scale
		if scale == 0 {
			scale = 1
		}
		return append([]DrawingView(nil), d.Views...), scale
	}
	side := "right"
	if d.FirstAngle {
		side = "left"
	}
	fw, fh := sizes["front"][0], sizes["front"][1]
	tw, th := sizes["top"][0], sizes["top"][1]
	sw, sh := sizes[side][0], sizes[side][1]
	iw, ih := sizes["iso"][0], sizes["iso"][1]
	// The room: the frame less the title block's height below.
	roomW := w - 2*SheetBorder - 2*viewGap
	roomH := h - 2*SheetBorder - TitleBlockHeight - 2*viewGap
	fits := func(s float64) bool {
		return s*(math.Max(fw, tw)+math.Max(sw, iw))+viewGap <= roomW &&
			s*(math.Max(fh, sh)+math.Max(th, ih))+viewGap <= roomH
	}
	scale := d.Scale
	if scale == 0 {
		scale = StandardScales[len(StandardScales)-1]
		for _, s := range StandardScales {
			if fits(s) {
				scale = s
				break
			}
		}
	}
	// A grid of two columns and two rows: the front and the top share a
	// column, the front and the side a row; the isometric the corner
	// left.
	col0 := scale * math.Max(fw, tw)
	col1 := scale * math.Max(sw, iw)
	row0 := scale * math.Max(fh, sh) // the front's row
	row1 := scale * math.Max(th, ih) // the top's row
	left := SheetBorder + viewGap + (roomW-(col0+col1+viewGap))/2
	bottom := SheetBorder + TitleBlockHeight + viewGap + (roomH-(row0+row1+viewGap))/2
	frontX, sideX := left+col0/2, left+col0+viewGap+col1/2
	var frontY, topY float64
	if d.FirstAngle {
		// Top below the front; the left view to the front's right.
		topY, frontY = bottom+row1/2, bottom+row1+viewGap+row0/2
	} else {
		frontY, topY = bottom+row0/2, bottom+row0+viewGap+row1/2
	}
	return []DrawingView{
		{Kind: "front", X: frontX, Y: frontY},
		{Kind: "top", X: frontX, Y: topY},
		{Kind: side, X: sideX, Y: frontY},
		{Kind: "iso", X: sideX, Y: topY},
	}, scale
}

// Clone is a copy of the drawing that shares nothing with it; nil for
// none.
func (d *Drawing) Clone() *Drawing {
	if d == nil {
		return nil
	}
	c := *d
	c.Views = append([]DrawingView(nil), d.Views...)
	c.Sections = append([]DrawingSection(nil), d.Sections...)
	c.Details = append([]DrawingDetail(nil), d.Details...)
	c.Dimensions = nil
	for _, dm := range d.Dimensions {
		dm.Edges = append([]topo.Name(nil), dm.Edges...)
		dm.Faces = append([]srf.FaceID(nil), dm.Faces...)
		c.Dimensions = append(c.Dimensions, dm)
	}
	return &c
}

// DefaultDrawing is the drawing a model starts one with: an A3 sheet,
// landscape, fitted, third-angle, with hidden edges.
func DefaultDrawing() *Drawing {
	return &Drawing{Sheet: "A3", Hidden: true}
}

// Letter is the letter of the n-th section or detail view: A, B, ...,
// Z, AA, AB, ...
func Letter(n int) string {
	if n < 26 {
		return string(rune('A' + n))
	}
	return Letter(n/26-1) + Letter(n%26)
}

// PlaceExtras places the sections and details that have no place, in
// columns to the right of the views, from the top down, given each
// one's size on the sheet in mm, the sections' first; right is where
// the views end. It returns where each goes, the sections' first.
func (d *Drawing) PlaceExtras(right float64, sizes [][2]float64) [][2]float64 {
	w, h := d.Size()
	top := h - SheetBorder - viewGap
	x, y, colW := right+viewGap, top, 0.0
	given := func(i int) (float64, float64, bool) {
		if i < len(d.Sections) {
			s := d.Sections[i]
			return s.X, s.Y, s.X != 0 || s.Y != 0
		}
		dt := d.Details[i-len(d.Sections)]
		return dt.X, dt.Y, dt.X != 0 || dt.Y != 0
	}
	out := make([][2]float64, len(sizes))
	for i, sz := range sizes {
		if gx, gy, ok := given(i); ok {
			out[i] = [2]float64{gx, gy}
			continue
		}
		// The next column when this one is full, down to the title
		// block.
		if y-sz[1]-10 < SheetBorder+TitleBlockHeight && y != top {
			x, y, colW = x+colW+viewGap, top, 0
		}
		if x+sz[0] > w-SheetBorder {
			x = w - SheetBorder - sz[0]
		}
		out[i] = [2]float64{x + sz[0]/2, y - sz[1]/2}
		y -= sz[1] + viewGap + 7
		colW = math.Max(colW, sz[0])
	}
	return out
}

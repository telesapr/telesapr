package sketch

import (
	"errors"
	"fmt"
	"slices"
)

// ViewKinds are the kinds of view a drawing can show, in the order its
// screen lists them.
var ViewKinds = []string{"front", "back", "top", "bottom", "left", "right", "iso"}

// The calls below change which views a drawing shows and where, which
// is what a script does by setting Drawing.Views, Sections and Details
// whole. They take a drawing with its views placed (Views not empty): a
// drawing with none shows the standard views, whose places depend on the
// solid's size, so the caller that has the solid places them first
// (pkg/ui, DrawingSheet).

// ViewIndex is the position of the view of that kind in Views, or -1
// when the drawing does not show it.
func (d *Drawing) ViewIndex(kind string) int {
	return slices.IndexFunc(d.Views, func(v DrawingView) bool { return v.Kind == kind })
}

// PlaceView shows the view of the given kind with its middle at x, y mm
// from the sheet's bottom left, moving it if the drawing shows it
// already.
func (d *Drawing) PlaceView(kind string, x, y float64) error {
	if len(d.Views) == 0 {
		return errors.New("the drawing's views are the standard ones, placed by the solid's size: place them first")
	}
	if _, _, ok := ViewDirection(kind); !ok {
		return fmt.Errorf("no view %q: the views are front, back, top, bottom, left, right and iso", kind)
	}
	if i := d.ViewIndex(kind); i >= 0 {
		d.Views[i].X, d.Views[i].Y = x, y
		return nil
	}
	d.Views = append(d.Views, DrawingView{Kind: kind, X: x, Y: y})
	return nil
}

// RemoveView stops showing the view of that kind. The last view stays,
// as a drawing with none shows the standard ones, and so does a view
// with a dimension on it.
func (d *Drawing) RemoveView(kind string) error {
	i := d.ViewIndex(kind)
	switch {
	case i < 0:
		return fmt.Errorf("the drawing does not show the %s view", kind)
	case len(d.Views) == 1:
		return fmt.Errorf("the %s view is the last: a drawing with no views shows the standard ones", kind)
	}
	for _, dm := range d.Dimensions {
		if dm.View == kind {
			return fmt.Errorf("the %s view has a dimension on it: delete the dimension first", kind)
		}
	}
	d.Views = slices.Delete(d.Views, i, i+1)
	return nil
}

// PlaceSection puts the i-th section with its middle at x, y mm on the
// sheet; 0, 0 gives it back to the automatic placing.
func (d *Drawing) PlaceSection(i int, x, y float64) error {
	if i < 0 || i >= len(d.Sections) {
		return fmt.Errorf("no section %d: the drawing has %d", i, len(d.Sections))
	}
	d.Sections[i].X, d.Sections[i].Y = x, y
	return nil
}

// PlaceDetail puts the i-th detail with its middle at x, y mm on the
// sheet; 0, 0 gives it back to the automatic placing.
func (d *Drawing) PlaceDetail(i int, x, y float64) error {
	if i < 0 || i >= len(d.Details) {
		return fmt.Errorf("no detail %d: the drawing has %d", i, len(d.Details))
	}
	d.Details[i].X, d.Details[i].Y = x, y
	return nil
}

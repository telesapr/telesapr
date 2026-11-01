package sketch

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The calls below add and remove a drawing's section and detail views:
// what the drawing screen's links and a script's Drawing calls both do.
// A section or a detail is checked against the solid it looks at, which
// the caller gives (DrawingSolid), so that one that would show nothing
// is refused rather than drawn empty. Sections and details are lettered
// in order, the sections first, and a dimension names its view by that
// letter, so adding or removing one moves the letters of the
// dimensions on those after it.

// ErrNoDrawingSolid is returned when the active group has no solid for
// the drawing's views to look at.
var ErrNoDrawingSolid = errors.New("the active group has no solid to draw")

// DrawingSolid is the mesh the drawing's views look at: the active
// group's solid, as displayed.
func (sk *Sketch) DrawingSolid() (*srf.SMesh, error) {
	g := sk.Group(sk.ActiveGroup())
	if g == nil {
		return nil, ErrNoDrawingSolid
	}
	g.GenerateDisplayItems(sk)
	if len(g.DisplayMesh.Triangles) == 0 {
		return nil, ErrNoDrawingSolid
	}
	return &g.DisplayMesh, nil
}

// meshHalfExtent is half the extent of the mesh's points along dir, a
// unit vector: how far from the solid's middle a plane square to dir
// still cuts it.
func meshHalfExtent(m *srf.SMesh, dir geom.Vector) float64 {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, tr := range m.Triangles {
		for _, p := range []geom.Vector{tr.A, tr.B, tr.C} {
			v := dir.Dot(p)
			lo, hi = min(lo, v), max(hi, v)
		}
	}
	return (hi - lo) / 2
}

func vec3(v [3]float64) geom.Vector { return geom.Vector{X: v[0], Y: v[1], Z: v[2]} }

// AddSection adds a section of the view (front, back, top, bottom, left
// or right) cut offset mm toward the viewer from the middle of solid,
// placed automatically. The plane must cut the solid.
func (d *Drawing) AddSection(view string, offset float64, solid *srf.SMesh) error {
	r, u, ok := ViewDirection(view)
	if !ok || view == "iso" {
		return fmt.Errorf("no section of %q: a section is of front, back, top, bottom, left or right", view)
	}
	if solid == nil || len(solid.Triangles) == 0 {
		return ErrNoDrawingSolid
	}
	half := meshHalfExtent(solid, vec3(r).Cross(vec3(u)))
	if !(math.Abs(offset) < half) {
		return fmt.Errorf("a section of the %s view cuts the solid within %g mm of its middle, not %g mm from it",
			view, roundMicron(half), offset)
	}
	// The details' letters move one on.
	d.reletter(len(d.Sections), 1)
	d.Sections = append(d.Sections, DrawingSection{View: view, Offset: offset})
	return nil
}

// AddDetail adds a detail of the view: the part within r mm of the
// point cx, cy (mm right and up from the view's middle), enlarged times
// the sheet's scale, placed automatically. The point must be on the
// view, within the solid's outline seen from it.
func (d *Drawing) AddDetail(view string, cx, cy, r, times float64, solid *srf.SMesh) error {
	right, up, ok := ViewDirection(view)
	if !ok {
		return fmt.Errorf("no detail of %q: the views are front, back, top, bottom, left, right and iso", view)
	}
	if !(r > 0) || !(times > 0) {
		return fmt.Errorf("a detail's radius and scale are more than zero, not %g and %g", r, times)
	}
	if solid == nil || len(solid.Triangles) == 0 {
		return ErrNoDrawingSolid
	}
	hw, hh := meshHalfExtent(solid, vec3(right)), meshHalfExtent(solid, vec3(up))
	if !(math.Abs(cx) <= hw) || !(math.Abs(cy) <= hh) {
		return fmt.Errorf("a detail's middle is on the %s view, within %g mm right or left and %g up or down of its middle, not at %g, %g",
			view, roundMicron(hw), roundMicron(hh), cx, cy)
	}
	d.Details = append(d.Details, DrawingDetail{View: view, CX: cx, CY: cy, R: r, Scale: times})
	return nil
}

// RemoveSection removes the i-th section. One with a dimension on it
// stays.
func (d *Drawing) RemoveSection(i int) error {
	if i < 0 || i >= len(d.Sections) {
		return fmt.Errorf("no section %d: the drawing has %d", i, len(d.Sections))
	}
	if err := d.removable(i); err != nil {
		return err
	}
	d.reletter(i+1, -1)
	d.Sections = slices.Delete(d.Sections, i, i+1)
	return nil
}

// RemoveDetail removes the i-th detail. One with a dimension on it
// stays.
func (d *Drawing) RemoveDetail(i int) error {
	if i < 0 || i >= len(d.Details) {
		return fmt.Errorf("no detail %d: the drawing has %d", i, len(d.Details))
	}
	n := len(d.Sections) + i
	if err := d.removable(n); err != nil {
		return err
	}
	d.reletter(n+1, -1)
	d.Details = slices.Delete(d.Details, i, i+1)
	return nil
}

// ExtraIndex is which section or detail a letter names: the index into
// Sections or into Details, and section true for a section; ok is false
// when it names neither.
func (d *Drawing) ExtraIndex(letter string) (i int, section, ok bool) {
	n, ok := d.DimensionView(letter)
	if !ok || n < 0 {
		return 0, false, false
	}
	if n < len(d.Sections) {
		return n, true, true
	}
	return n - len(d.Sections), false, true
}

// removable reports why the n-th section or detail, by its letter's
// number, cannot go: a dimension is on it.
func (d *Drawing) removable(n int) error {
	letter := Letter(n)
	for _, dm := range d.Dimensions {
		if dm.View == letter {
			return fmt.Errorf("view %s has a dimension on it: delete the dimension first", letter)
		}
	}
	return nil
}

// reletter moves by by the letters of the dimensions on the sections
// and details from the from-th on, as one is added or removed before
// them; it is called before the change, while the letters name what
// they did.
func (d *Drawing) reletter(from, by int) {
	for k, dm := range d.Dimensions {
		if _, _, ok := ViewDirection(dm.View); ok {
			continue
		}
		if n, ok := d.DimensionView(dm.View); ok && n >= from {
			d.Dimensions[k].View = Letter(n + by)
		}
	}
}

// roundMicron is v to the thousandth of a mm, for a message.
func roundMicron(v float64) float64 { return math.Round(v*1000) / 1000 }

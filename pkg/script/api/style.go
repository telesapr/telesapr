package api

import (
	"cmp"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The stipple patterns a line may be drawn in, as the style screen
// lists them.
const (
	StippleContinuous = core.StippleContinuous
	StippleShortDash  = core.StippleShortDash
	StippleDash       = core.StippleDash
	StippleLongDash   = core.StippleLongDash
	StippleDashDot    = core.StippleDashDot
	StippleDashDotDot = core.StippleDashDotDot
	StippleDot        = core.StippleDot
	StippleFreehand   = core.StippleFreehand
	StippleZigzag     = core.StippleZigzag
)

// The units a line's width, its dashes and a dimension's text are
// measured in: the screen's pixels, or millimetres.
const (
	UnitsPixels = core.UnitsPixels
	UnitsMM     = core.UnitsMM
)

// ErrNotStylable is what a style given to a request or a constraint the
// window can't style gives: a point, a workplane, a normal, or a
// constraint other than a comment.
var ErrNotStylable = errors.New("can't be given a style")

// Style is a line style: how a request or a dimension is drawn, and
// whether it goes in an exported file. A script reaches the drawing's
// styles as doc.Styles, and makes a new one as doc.NewStyle, which is
// what Edit > Line Styles does on the group screen.
//
// The styles the program has of its own — the active group's, a solid
// edge's, a datum's — may be changed too, but not deleted, and a
// custom style may not be renamed away from what the screen gives it
// without the window's edit control: Style.SetName does both, and says
// so.
type Style struct {
	d *Doc
	H core.HStyle
}

// get is the style for a change: one of the program's own that the
// sketch doesn't have yet is made, which is how it comes to exist
// before anything is drawn in it.
func (s Style) get(sk *sketch.Sketch) *core.Style {
	if s.d == nil || s.H.V == 0 {
		fail("no style")
	}
	if st := sk.Style(s.H); st != nil {
		return st
	}
	if sk.StyleView(s.H) == nil {
		fail("no style %s", s)
	}
	return sk.StyleGet(s.H)
}

// view is the style for a read, which leaves the sketch as it is: a
// style of the program's own that the sketch doesn't have yet reads as
// the default it will be made as.
func (s Style) view(sk *sketch.Sketch) *core.Style {
	if s.d == nil || s.H.V == 0 {
		fail("no style")
	}
	st := sk.StyleView(s.H)
	if st == nil {
		fail("no style s%08x", s.H.V)
	}
	return st
}

// String is the style's name, as the styles list shows it.
func (s Style) String() (name string) {
	if s.d == nil {
		return "no style"
	}
	s.d.read(func(sk *sketch.Sketch) {
		if s.H.V == 0 {
			name = "no style"
			return
		}
		name = s.view(sk).Name
	})
	return name
}

// Valid reports whether the style is still in the drawing.
func (s Style) Valid() (ok bool) {
	if s.d == nil || s.H.V == 0 {
		return false
	}
	s.d.read(func(sk *sketch.Sketch) { ok = sk.StyleView(s.H) != nil })
	return ok
}

// Custom reports whether the style is one of the user's own, rather
// than one of the program's.
func (s Style) Custom() (custom bool) {
	s.d.read(func(sk *sketch.Sketch) { custom = s.H.V >= core.StyleFirstCustom })
	return custom
}

// Name is the style's name, as the styles list shows it.
func (s Style) Name() (name string) {
	s.d.read(func(sk *sketch.Sketch) { name = s.view(sk).Name })
	return name
}

// SetName renames the style, as the style screen's rename does. A style
// of the program's own keeps its name: the program finds it by the name
// it has, in its settings and in a file.
func (s Style) SetName(name string) {
	s.change(func(style *core.Style) {
		if !s.customStyle() {
			fail("%s is one of the program's own styles, and keeps its name", s.Name())
		}
		if strings.TrimSpace(name) == "" {
			fail("a style needs a name")
		}
		style.Name = name
	})
}

// Color is the style's line colour, each of red, green and blue 0 to
// 255.
func (s Style) Color() (red, green, blue int) {
	s.d.read(func(sk *sketch.Sketch) {
		c := s.view(sk).Color
		red, green, blue = int(c.Red), int(c.Green), int(c.Blue)
	})
	return red, green, blue
}

// SetColor colours the style's lines.
func (s Style) SetColor(red, green, blue int) {
	s.change(func(style *core.Style) {
		style.Color = core.NewRGBA(rgb(red), rgb(green), rgb(blue), 255)
	})
}

// FillColor is the colour of the contours the style fills, which only
// a custom style has.
func (s Style) FillColor() (red, green, blue int) {
	s.d.read(func(sk *sketch.Sketch) {
		c := s.view(sk).FillColor
		red, green, blue = int(c.Red), int(c.Green), int(c.Blue)
	})
	return red, green, blue
}

// SetFillColor sets the colour of the contours the style fills.
func (s Style) SetFillColor(red, green, blue int) {
	s.change(func(style *core.Style) {
		style.FillColor = core.NewRGBA(rgb(red), rgb(green), rgb(blue), 255)
	})
}

// Filled reports whether the style fills the contours it draws, which
// is how a face or a section cap is given a colour.
func (s Style) Filled() (filled bool) {
	s.d.read(func(sk *sketch.Sketch) { filled = s.view(sk).Filled })
	return filled
}

// SetFilled fills or stops filling the contours the style draws.
func (s Style) SetFilled(filled bool) {
	s.change(func(style *core.Style) { style.Filled = filled })
}

// Width is the style's line width, in the unit Style.WidthAs names.
func (s Style) Width() (width float64) {
	s.d.read(func(sk *sketch.Sketch) { width = s.view(sk).Width })
	return width
}

// SetWidth sets the line width, zero or more.
func (s Style) SetWidth(width float64) {
	s.change(func(style *core.Style) { style.Width = nonNegative("width", width) })
}

// WidthAs is whether the width and the stipple width are in pixels or
// in the document's unit.
func (s Style) WidthAs() (as core.UnitsAs) {
	s.d.read(func(sk *sketch.Sketch) { as = s.view(sk).WidthAs })
	return as
}

// SetWidthAs measures the width in pixels or in the document's unit.
func (s Style) SetWidthAs(as core.UnitsAs) {
	s.change(func(style *core.Style) {
		if as != core.UnitsPixels && as != core.UnitsMM {
			fail("a width is in pixels (%d) or in millimetres (%d), not %d",
				core.UnitsPixels, core.UnitsMM, as)
		}
		style.WidthAs = as
	})
}

// Stipple is the style's dash pattern, one of the Stipple constants.
func (s Style) Stipple() (stipple core.StipplePattern) {
	s.d.read(func(sk *sketch.Sketch) { stipple = s.view(sk).StippleType })
	return stipple
}

// SetStipple dashes the style's lines in the given pattern.
func (s Style) SetStipple(stipple core.StipplePattern) {
	s.change(func(style *core.Style) {
		if stipple < core.StippleContinuous || stipple > core.StippleZigzag {
			fail("no stipple pattern %d", stipple)
		}
		style.StippleType = stipple
	})
}

// StippleScale is how far apart the dashes of a stippled line are, in
// the unit Style.WidthAs names.
func (s Style) StippleScale() (scale float64) {
	s.d.read(func(sk *sketch.Sketch) { scale = s.view(sk).StippleScale })
	return scale
}

// SetStippleScale sets how far apart the dashes are, zero or more. A
// scale too small for the dashes to be a pixel apart draws the line
// continuous.
func (s Style) SetStippleScale(scale float64) {
	s.change(func(style *core.Style) { style.StippleScale = nonNegative("stipple scale", scale) })
}

// TextHeight is the height of the text of a dimension drawn in the
// style, in the unit Style.TextHeightAs names.
func (s Style) TextHeight() (height float64) {
	s.d.read(func(sk *sketch.Sketch) { height = s.view(sk).TextHeight })
	return height
}

// SetTextHeight sets the height of the text of a dimension drawn in the
// style.
func (s Style) SetTextHeight(height float64) {
	s.change(func(style *core.Style) { style.TextHeight = height })
}

// TextHeightAs is whether the text height is in pixels or in the
// document's unit.
func (s Style) TextHeightAs() (as core.UnitsAs) {
	s.d.read(func(sk *sketch.Sketch) { as = s.view(sk).TextHeightAs })
	return as
}

// SetTextHeightAs measures the text height in pixels (UnitsPixels) or
// in millimetres (UnitsMM). The height is converted at the view's
// scale, so the text keeps the size it is drawn at, as the style
// screen's pixels and mm links do.
func (s Style) SetTextHeightAs(as core.UnitsAs) {
	if as != core.UnitsPixels && as != core.UnitsMM {
		fail("a text height is in pixels (%d) or in millimetres (%d), not %d",
			core.UnitsPixels, core.UnitsMM, as)
	}
	s.d.change(func(sk *sketch.Sketch) {
		s.get(sk)
		sk.SetStyleTextHeightUnits(s.H, as)
	})
}

// TextAlign is where the text of a dimension sits against its anchor:
// horizontal is "left", "center" or "right", vertical is "bottom",
// "center" or "top".
func (s Style) TextAlign() (horizontal, vertical string) {
	var o core.TextOrigin
	s.d.read(func(sk *sketch.Sketch) { o = s.view(sk).TextOrigin })
	horizontal, vertical = "center", "center"
	switch {
	case o&core.TextOriginLeft != 0:
		horizontal = "left"
	case o&core.TextOriginRight != 0:
		horizontal = "right"
	}
	switch {
	case o&core.TextOriginBot != 0:
		vertical = "bottom"
	case o&core.TextOriginTop != 0:
		vertical = "top"
	}
	return horizontal, vertical
}

// SetTextAlign aligns the text of a dimension drawn in the style, as
// the style screen's L/H/R and B/V/T links do. horizontal is "left",
// "center" or "right"; vertical is "bottom", "center" or "top".
func (s Style) SetTextAlign(horizontal, vertical string) {
	var h, v core.TextOrigin
	switch horizontal {
	case "left":
		h = core.TextOriginLeft
	case "right":
		h = core.TextOriginRight
	case "center":
	default:
		fail("a horizontal alignment is \"left\", \"center\" or \"right\", not %q", horizontal)
	}
	switch vertical {
	case "bottom":
		v = core.TextOriginBot
	case "top":
		v = core.TextOriginTop
	case "center":
	default:
		fail("a vertical alignment is \"bottom\", \"center\" or \"top\", not %q", vertical)
	}
	s.d.change(func(sk *sketch.Sketch) {
		s.get(sk)
		sk.SetStyleTextHorizontal(s.H, h)
		sk.SetStyleTextVertical(s.H, v)
	})
}

// TextAngle is the angle in degrees the text of a dimension is drawn
// at.
func (s Style) TextAngle() (degrees float64) {
	s.d.read(func(sk *sketch.Sketch) { degrees = s.view(sk).TextAngle })
	return degrees
}

// SetTextAngle turns the text of a dimension drawn in the style.
func (s Style) SetTextAngle(degrees float64) {
	s.change(func(style *core.Style) { style.TextAngle = degrees })
}

// ZIndex is how high the style is drawn: of the styles in one group,
// the one with the highest index goes on top, and a file's entities
// are exported in that order too. It is what puts a dimension over the
// geometry it measures.
func (s Style) ZIndex() (index int) {
	s.d.read(func(sk *sketch.Sketch) { index = s.view(sk).ZIndex })
	return index
}

// SetZIndex draws the style above or below the others: a dimension's
// style at 4 puts it over the geometry, which is what the program's
// own styles do.
func (s Style) SetZIndex(index int) {
	s.change(func(style *core.Style) { style.ZIndex = index })
}

// Visible reports whether lines in the style are drawn.
func (s Style) Visible() (visible bool) {
	s.d.read(func(sk *sketch.Sketch) { visible = s.view(sk).Visible })
	return visible
}

// SetVisible shows or hides the lines in the style, which leaves the
// drawing as it is.
func (s Style) SetVisible(visible bool) {
	s.change(func(style *core.Style) { style.Visible = visible })
}

// Exportable reports whether lines in the style go in an exported
// file: a style is exported only when it is both visible and
// exportable.
func (s Style) Exportable() (exportable bool) {
	s.d.read(func(sk *sketch.Sketch) { exportable = s.view(sk).Exportable })
	return exportable
}

// SetExportable puts the style's lines in exported files, or leaves
// them out of them.
func (s Style) SetExportable(exportable bool) {
	s.change(func(style *core.Style) { style.Exportable = exportable })
}

// Requests are what is drawn in the style.
func (s Style) Requests() (requests []Request) {
	s.d.read(func(sk *sketch.Sketch) {
		for i := range sk.Requests {
			if sk.Requests[i].Style == s.H {
				requests = append(requests, Request{s.d, sk.Requests[i].H})
			}
		}
	})
	return requests
}

// Constraints are the constraints, dimensions among them, drawn in the
// style.
func (s Style) Constraints() (constraints []Constraint) {
	s.d.read(func(sk *sketch.Sketch) {
		for i := range sk.Constraints {
			if sk.Constraints[i].Disp.Style == s.H {
				constraints = append(constraints, Constraint{s.d, sk.Constraints[i].H})
			}
		}
	})
	return constraints
}

// Delete takes the style out of the drawing; whatever was drawn in it
// falls back to the style it would have without one. A style of the
// program's own can't be deleted: the program has it of its own, and
// makes it again at the next regeneration.
func (s Style) Delete() {
	s.change(func(style *core.Style) {
		if !s.customStyle() {
			fail("%s is one of the program's own styles", s.Name())
		}
		sk := s.d.h.Sketch()
		for i := range sk.Styles {
			if sk.Styles[i].H == s.H {
				sk.Styles = append(sk.Styles[:i], sk.Styles[i+1:]...)
				break
			}
		}
	})
}

// customStyle reports whether the style is one of the user's own,
// rather than one of the program's, which the styles list marks and
// which may not be renamed or deleted.
func (s Style) customStyle() bool { return s.H.V >= core.StyleFirstCustom }

// change changes one field of the style, which is one step of undo, as
// the style screen's own edits are.
func (s Style) change(set func(style *core.Style)) {
	s.d.change(func(sk *sketch.Sketch) { set(s.get(sk)) })
}

// nonNegative is a length of a style, which must be a number, zero or
// more, as the style screen keeps it.
func nonNegative(what string, v float64) float64 {
	if !(v >= 0) || math.IsInf(v, 1) {
		fail("a style's %s is a number, zero or more, not %g", what, v)
	}
	return v
}

// rgb is one channel of a colour, which must be 0 to 255.
func rgb(v int) uint8 {
	if v < 0 || v > 255 {
		fail("a colour's channel is 0 to 255, not %d", v)
	}
	return uint8(v)
}

// Styles are the drawing's line styles, in the order the styles list
// shows them: the program's own first, then the user's own.
func (d *Doc) Styles() (styles []Style) {
	d.read(func(sk *sketch.Sketch) {
		// The defaults the sketch hasn't made yet are listed all the
		// same, but not added to it: a read leaves no trace in the file.
		var handles []core.HStyle
		for i := range sk.Styles {
			handles = append(handles, sk.Styles[i].H)
		}
		for _, def := range core.StyleDefaults {
			if sk.Style(def.H) == nil {
				handles = append(handles, def.H)
			}
		}
		slices.SortFunc(handles, func(a, b core.HStyle) int { return cmp.Compare(a.V, b.V) })
		for _, h := range handles {
			styles = append(styles, Style{d, h})
		}
	})
	return styles
}

// Style is the style of that name, as the styles list names it:
// "#def-solid-edge" for one of the program's own styles, the name the
// user gave a custom one. The call fails when the drawing has none, as
// the zero Style it would otherwise give means no style to
// Request.SetStyle.
func (d *Doc) Style(name string) (s Style) {
	d.read(func(sk *sketch.Sketch) {
		for i := range sk.Styles {
			if sk.Styles[i].Name == name {
				s = Style{d, sk.Styles[i].H}
			}
		}
		if s.d != nil {
			return
		}
		for _, def := range core.StyleDefaults {
			if core.CnfPrefixToName(def.CnfPrefix) == name && sk.Style(def.H) == nil {
				s = Style{d, def.H}
			}
		}
		if s.d == nil {
			fail("no style named %q", name)
		}
	})
	return s
}

// ResetStyles puts the program's own styles back to their factory
// settings, as Load Factory Defaults does on the styles screen. The
// user's own styles are kept. It is one step of undo.
func (d *Doc) ResetStyles() {
	d.change(func(sk *sketch.Sketch) { sk.LoadFactoryDefaults() })
}

// NewStyle adds a style of the user's own, with the settings a new one
// has, as Edit > Line Styles does on the group screen. It is drawn in
// nothing until a request or a dimension is given it with
// Request.SetStyle or Constraint.SetStyle.
func (d *Doc) NewStyle() (s Style) {
	d.change(func(sk *sketch.Sketch) { s = Style{d, sk.NewCustomStyle()} })
	return s
}

// handle is the handle a request or a dimension keeps for the style,
// the style made in the sketch if it isn't there yet, which is how one
// of the program's own styles comes to exist before anything is drawn
// in it. The zero Style is no style, which puts what is drawn back on
// the style it has by where it is.
func (s Style) handle(sk *sketch.Sketch) core.HStyle {
	if s.d == nil && s.H.V == 0 {
		return core.HStyle{}
	}
	return s.get(sk).H
}

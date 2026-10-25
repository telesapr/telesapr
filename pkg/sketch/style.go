package sketch

import (
	"sort"

	"github.com/telesapr/telesapr/pkg/core"
)

// This file ports the sketch side of solvespace/src/style.cpp; the
// style screens of the text window are user interface.

// StyleGet is style h, created with default settings if the sketch
// doesn't have it yet; no style means the active group's.
func (sk *Sketch) StyleGet(h core.HStyle) *core.Style {
	if h.V == 0 {
		h.V = core.StyleActiveGrp
	}

	if s := sk.Style(h); s != nil {
		// It exists, good.
		return s
	}
	// It doesn't exist; so we should create it and then return that.
	sk.CreateDefaultStyle(h)
	return sk.Style(h)
}

// StyleView is style h as it would be drawn, without changing the
// sketch: the style itself when the sketch has it, else a copy of the
// default it would be made as, and nil for a handle that is neither.
// A read of the styles uses this, so that looking does not add styles
// that the next save would write.
func (sk *Sketch) StyleView(h core.HStyle) *core.Style {
	if h.V == 0 {
		h.V = core.StyleActiveGrp
	}
	if s := sk.Style(h); s != nil {
		return s
	}
	for i := range core.StyleDefaults {
		if core.StyleDefaults[i].H == h {
			var ns core.Style
			core.FillDefaultStyle(&ns, &core.StyleDefaults[i], false, sk.SettingsStore())
			ns.Name = core.CnfPrefixToName(core.StyleDefaults[i].CnfPrefix)
			ns.H = h
			return &ns
		}
	}
	return nil
}

// CreateAllDefaultStyles makes sure the sketch has every default style.
func (sk *Sketch) CreateAllDefaultStyles() {
	for _, d := range core.StyleDefaults {
		sk.StyleGet(d.H)
	}
}

// CreateDefaultStyle adds style h: the default style it names, or else
// a custom one like the active group's.
func (sk *Sketch) CreateDefaultStyle(h core.HStyle) {
	var d *core.StyleDefault
	for i := range core.StyleDefaults {
		if core.StyleDefaults[i].H == h {
			d = &core.StyleDefaults[i]
		}
	}

	ns := core.Style{}
	if d != nil {
		core.FillDefaultStyle(&ns, d, false, sk.SettingsStore())
		ns.Name = core.CnfPrefixToName(d.CnfPrefix)
	} else {
		// Not a default style; so just create it the same as our default
		// active group entity style.
		core.FillDefaultStyle(&ns, nil, false, sk.SettingsStore())
		ns.Name = "new-custom-style"
	}
	ns.H = h

	// The styles are kept sorted by handle.
	i := sort.Search(len(sk.Styles), func(i int) bool { return sk.Styles[i].H.V > h.V })
	sk.Styles = append(sk.Styles, core.Style{})
	copy(sk.Styles[i+1:], sk.Styles[i:])
	sk.Styles[i] = ns
}

// StyleForEntity is the style entity he draws in: its own, else by
// whether it's in the active group, or construction.
func (sk *Sketch) StyleForEntity(he core.HEntity) core.HStyle {
	e := sk.MustEntity(he)
	// If the entity has a special style, use that. If that style doesn't
	// exist yet, then it will get created automatically later.
	if e.Style.V != 0 {
		return e.Style
	}

	// Otherwise, we use the default rules.
	switch {
	case e.Group != sk.ActiveGroup():
		return core.HStyle{V: core.StyleInactiveGrp}
	case e.Construction:
		return core.HStyle{V: core.StyleConstruction}
	}
	return core.HStyle{V: core.StyleActiveGrp}
}

// StyleColor is style h's stroke colour, rewritten for export as
// RewriteColor says.
func (sk *Sketch) StyleColor(h core.HStyle, forExport, fixExportColors bool) core.RGBA {
	s := sk.StyleGet(h)
	if forExport {
		return core.RewriteColor(s.Color, fixExportColors)
	}
	return s.Color
}

// StyleFillColor is style h's fill colour.
func (sk *Sketch) StyleFillColor(h core.HStyle, forExport, fixExportColors bool) core.RGBA {
	s := sk.StyleGet(h)
	if forExport {
		return core.RewriteColor(s.FillColor, fixExportColors)
	}
	return s.FillColor
}

// LoadFactoryDefaults resets the default styles to their factory
// settings.
func (sk *Sketch) LoadFactoryDefaults() {
	for i := range core.StyleDefaults {
		d := &core.StyleDefaults[i]
		s := sk.StyleGet(d.H)
		core.FillDefaultStyle(s, d, true, sk.SettingsStore())
	}
}

// FreezeDefaultStyles saves the default styles as the user's settings,
// with widths and text heights at the view's scale.
func (sk *Sketch) FreezeDefaultStyles(settings core.Settings) {
	for i := range core.StyleDefaults {
		d := &core.StyleDefaults[i]
		core.FreezeColor(settings, core.CnfColor(d.CnfPrefix), sk.StyleColor(d.H, false, false))
		settings.FreezeFloat(core.CnfWidth(d.CnfPrefix), float64(float32(sk.StyleWidth(d.H, sk.view().Scale))))
		settings.FreezeString(core.CnfStippleType(d.CnfPrefix), core.StipplePatternName(sk.StylePatternType(d.H)))
		settings.FreezeFloat(core.CnfStippleScale(d.CnfPrefix), float64(float32(sk.StyleStippleScale(d.H))))
		settings.FreezeFloat(core.CnfTextHeight(d.CnfPrefix), float64(float32(sk.StyleTextHeight(d.H, sk.view().Scale))))
		core.FreezeBool(settings, core.CnfExportable(d.CnfPrefix), sk.StyleExportable(d.H))
	}
}

// StyleWidth is style h's width in pixels, at the view's scale in
// pixels per mm.
func (sk *Sketch) StyleWidth(h core.HStyle, scale float64) float64 {
	s := sk.StyleGet(h)
	switch s.WidthAs {
	case core.UnitsMM:
		return s.Width * scale
	case core.UnitsPixels:
		return s.Width
	}
	panic("Unexpected units")
}

// StyleTextHeight is style h's text height in pixels.
func (sk *Sketch) StyleTextHeight(h core.HStyle, scale float64) float64 {
	s := sk.StyleGet(h)
	switch s.TextHeightAs {
	case core.UnitsMM:
		return s.TextHeight * scale
	case core.UnitsPixels:
		return s.TextHeight
	}
	panic("Unexpected units")
}

// StyleExportable reports whether lines of style h go in exported
// files: if it's both shown and exportable.
func (sk *Sketch) StyleExportable(h core.HStyle) bool {
	s := sk.StyleGet(h)
	return s.Exportable && s.Visible
}

// StylePatternType is style h's stipple pattern.
func (sk *Sketch) StylePatternType(h core.HStyle) core.StipplePattern {
	return sk.StyleGet(h).StippleType
}

// StyleStippleScale is style h's stipple scale.
func (sk *Sketch) StyleStippleScale(h core.HStyle) float64 {
	return sk.StyleGet(h).StippleScale
}

// NewCustomStyle adds a custom style, numbered after the last, with the
// defaults, and returns its handle.
func (sk *Sketch) NewCustomStyle() core.HStyle {
	maxID := uint32(0)
	for i := range sk.Styles {
		maxID = max(maxID, sk.Styles[i].H.V)
	}
	hs := core.HStyle{V: max(uint32(core.StyleFirstCustom), maxID+1)}
	sk.StyleGet(hs)
	return hs
}

// SetStyleWidthUnits measures style h's line width and stipple width in
// as, keeping what its lines look like on the screen: both numbers are
// converted at the view's scale. It is what the style screen's pixels
// and mm links and Style.SetWidthAs both do.
func (sk *Sketch) SetStyleWidthUnits(h core.HStyle, as core.UnitsAs) {
	s := sk.StyleGet(h)
	if s.WidthAs == as {
		return
	}
	scale := sk.view().Scale
	switch as {
	case core.UnitsMM:
		s.Width /= scale
		s.StippleScale /= scale
	case core.UnitsPixels:
		s.Width *= scale
		s.StippleScale *= scale
	}
	s.WidthAs = as
}

// SetStyleTextHeightUnits measures style h's text height in as, keeping
// the height it is drawn at on the screen: the number is converted at
// the view's scale, so a switch of units changes what the style says
// and not how big its text looks. It is what the style screen's pixels
// and mm links and Style.SetTextHeightAs both do.
func (sk *Sketch) SetStyleTextHeightUnits(h core.HStyle, as core.UnitsAs) {
	s := sk.StyleGet(h)
	if s.TextHeightAs == as {
		return
	}
	scale := sk.view().Scale
	switch as {
	case core.UnitsMM:
		s.TextHeight /= scale
	case core.UnitsPixels:
		s.TextHeight *= scale
	}
	s.TextHeightAs = as
}

// SetStyleTextHorizontal aligns style h's text on the left of its
// anchor (core.TextOriginLeft), on the right (core.TextOriginRight) or
// centred (core.TextOriginNone), leaving the vertical alignment as it
// is.
func (sk *Sketch) SetStyleTextHorizontal(h core.HStyle, o core.TextOrigin) {
	s := sk.StyleGet(h)
	s.TextOrigin = s.TextOrigin&^(core.TextOriginLeft|core.TextOriginRight) |
		o&(core.TextOriginLeft|core.TextOriginRight)
}

// SetStyleTextVertical aligns style h's text on the bottom of its
// anchor (core.TextOriginBot), on the top (core.TextOriginTop) or
// centred (core.TextOriginNone), leaving the horizontal alignment as
// it is.
func (sk *Sketch) SetStyleTextVertical(h core.HStyle, o core.TextOrigin) {
	s := sk.StyleGet(h)
	s.TextOrigin = s.TextOrigin&^(core.TextOriginBot|core.TextOriginTop) |
		o&(core.TextOriginBot|core.TextOriginTop)
}

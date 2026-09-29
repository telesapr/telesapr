package sketch

import (
	"sort"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// This file ports the sketch side of solvespace/src/style.cpp; the
// style screens of the text window are user interface.

// StyleGet is style h, created with default settings if the sketch
// doesn't have it yet; no style means the active group's (C++
// Style::Get).
func (sk *Sketch) StyleGet(h core.HStyle) *core.Style {
	if h.V == 0 {
		h.V = core.StyleActiveGrp
	}

	if s := sk.GetStyle(h); s != nil {
		// It exists, good.
		return s
	}
	// It doesn't exist; so we should create it and then return that.
	sk.CreateDefaultStyle(h)
	return sk.GetStyle(h)
}

// CreateAllDefaultStyles makes sure the sketch has every default style
// (C++ Style::CreateAllDefaultStyles).
func (sk *Sketch) CreateAllDefaultStyles() {
	for _, d := range core.StyleDefaults {
		sk.StyleGet(d.H)
	}
}

// CreateDefaultStyle adds style h: the default style it names, or else a
// custom one like the active group's (C++ Style::CreateDefaultStyle).
func (sk *Sketch) CreateDefaultStyle(h core.HStyle) {
	var d *core.StyleDefault
	for i := range core.StyleDefaults {
		if core.StyleDefaults[i].H == h {
			d = &core.StyleDefaults[i]
		}
	}

	ns := core.Style{}
	if d != nil {
		core.FillDefaultStyle(&ns, d, false)
		ns.Name = core.CnfPrefixToName(d.CnfPrefix)
	} else {
		// Not a default style; so just create it the same as our default
		// active group entity style.
		core.FillDefaultStyle(&ns, nil, false)
		ns.Name = "new-custom-style"
	}
	ns.H = h

	// C++ IdList keeps the styles sorted by handle.
	i := sort.Search(len(sk.Styles), func(i int) bool { return sk.Styles[i].H.V > h.V })
	sk.Styles = append(sk.Styles, core.Style{})
	copy(sk.Styles[i+1:], sk.Styles[i:])
	sk.Styles[i] = ns
}

// StyleForEntity is the style entity he draws in: its own, else by
// whether it's in the active group, or construction (C++
// Style::ForEntity).
func (sk *Sketch) StyleForEntity(he core.HEntity) core.HStyle {
	e := sk.MustGetEntity(he)
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
// RewriteColor says (C++ Style::Color).
func (sk *Sketch) StyleColor(h core.HStyle, forExport, fixExportColors bool) core.RgbaColor {
	s := sk.StyleGet(h)
	if forExport {
		return core.RewriteColor(s.Color, fixExportColors)
	}
	return s.Color
}

// StyleFillColor is style h's fill colour (C++ Style::FillColor).
func (sk *Sketch) StyleFillColor(h core.HStyle, forExport, fixExportColors bool) core.RgbaColor {
	s := sk.StyleGet(h)
	if forExport {
		return core.RewriteColor(s.FillColor, fixExportColors)
	}
	return s.FillColor
}

// LoadFactoryDefaults resets the default styles to their factory
// settings (C++ Style::LoadFactoryDefaults; the background colour it
// also resets is the user interface's).
func (sk *Sketch) LoadFactoryDefaults() {
	for i := range core.StyleDefaults {
		d := &core.StyleDefaults[i]
		s := sk.StyleGet(d.H)
		core.FillDefaultStyle(s, d, true)
	}
}

// FreezeDefaultStyles saves the default styles as the user's settings,
// with widths and text heights at the view's scale (C++
// Style::FreezeDefaultStyles).
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
// pixels per mm (C++ Style::Width).
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

// StyleTextHeight is style h's text height in pixels (C++
// Style::TextHeight).
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

// StyleExportable reports whether lines of style h go in exported files:
// if it's both shown and exportable (C++ Style::Exportable).
func (sk *Sketch) StyleExportable(h core.HStyle) bool {
	s := sk.StyleGet(h)
	return s.Exportable && s.Visible
}

// StylePatternType is style h's stipple pattern (C++ Style::PatternType).
func (sk *Sketch) StylePatternType(h core.HStyle) core.StipplePattern {
	return sk.StyleGet(h).StippleType
}

// StyleStippleScale is style h's stipple scale (C++ Style::StippleScale).
func (sk *Sketch) StyleStippleScale(h core.HStyle) float64 {
	return sk.StyleGet(h).StippleScale
}

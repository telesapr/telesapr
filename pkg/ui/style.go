package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/render"
)

func hstyle(v uint32) core.HStyle { return core.HStyle{V: v} }

// styleStroke is a style as a canvas stroke (C++ Style::Stroke).
func (ss *SolveSpaceUI) styleStroke(hs core.HStyle) render.Stroke {
	s := ss.SK.StyleGet(hs)
	st := render.Stroke{
		Color:          s.Color,
		StipplePattern: s.StippleType,
		StippleScale:   s.StippleScale,
		Width:          s.Width,
	}
	switch s.WidthAs {
	case core.UnitsPixels:
		st.Unit = render.UnitPX
	case core.UnitsMM:
		st.Unit = render.UnitMM
	}
	return st
}

// styleColor is a style's colour on screen (C++ Style::Color).
func (ss *SolveSpaceUI) styleColor(hs core.HStyle) core.RgbaColor {
	return ss.SK.StyleColor(hs, false, false)
}

// styleTextHeight is a style's text height in pixels (C++
// Style::TextHeight).
func (ss *SolveSpaceUI) styleTextHeight(hs core.HStyle) float64 {
	return ss.SK.StyleTextHeight(hs, ss.GW.view.Scale)
}

// defaultTextHeight is the constraint style's text height (C++
// Style::DefaultTextHeight).
func (ss *SolveSpaceUI) defaultTextHeight() float64 {
	return ss.styleTextHeight(hstyle(core.StyleConstraint))
}

// styleWidth is a style's width in pixels (C++ Style::Width).
func (ss *SolveSpaceUI) styleWidth(hs core.HStyle) float64 {
	return ss.SK.StyleWidth(hs, ss.GW.view.Scale)
}

// styleWidthMm is a style's width in mm (C++ Style::WidthMm).
func (ss *SolveSpaceUI) styleWidthMm(hs core.HStyle) float64 {
	return ss.styleWidth(hs) / ss.GW.view.Scale
}

// styleStippleScaleMm is a style's stipple scale in mm (C++
// Style::StippleScaleMm).
func (ss *SolveSpaceUI) styleStippleScaleMm(hs core.HStyle) float64 {
	s := ss.SK.StyleGet(hs)
	switch s.WidthAs {
	case core.UnitsMM:
		return s.StippleScale
	case core.UnitsPixels:
		return s.StippleScale / ss.GW.view.Scale
	}
	return 1.0
}

// styleExportColor and styleExportFillColor are a style's colours for
// export, near-white rewritten to black if the user asks (C++
// Style::Color and FillColor with forExport).
func (ss *SolveSpaceUI) styleExportColor(hs core.HStyle) core.RgbaColor {
	return ss.SK.StyleColor(hs, true, ss.FixExportColors)
}

func (ss *SolveSpaceUI) styleExportFillColor(hs core.HStyle) core.RgbaColor {
	return ss.SK.StyleFillColor(hs, true, ss.FixExportColors)
}

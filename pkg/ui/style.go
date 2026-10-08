package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/render"
)

func hstyle(v uint32) core.HStyle { return core.HStyle{V: v} }

// styleStroke is a style as a canvas stroke.
func (ss *Application) styleStroke(hs core.HStyle) render.Stroke {
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

// styleColor is a style's colour on screen.
func (ss *Application) styleColor(hs core.HStyle) core.RGBA {
	return ss.SK.StyleColor(hs, false, false)
}

// styleTextHeight is a style's text height in pixels.
func (ss *Application) styleTextHeight(hs core.HStyle) float64 {
	return ss.SK.StyleTextHeight(hs, ss.GW.view.Scale)
}

// defaultTextHeight is the constraint style's text height.
func (ss *Application) defaultTextHeight() float64 {
	return ss.styleTextHeight(hstyle(core.StyleConstraint))
}

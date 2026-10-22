package api

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// writeStyles makes the user's own styles, in the order of their
// handles, and sets what differs from a new one in every style, the
// program's own as well; a style that is only drawn in is named.
func (w *scriptWriter) writeStyles() {
	sk := w.sk
	used := map[core.HStyle]bool{}
	for i := range sk.Requests {
		used[sk.Requests[i].Style] = true
	}
	for i := range sk.Constraints {
		used[sk.Constraints[i].Disp.Style] = true
	}
	var handles []core.HStyle
	for i := range sk.Styles {
		handles = append(handles, sk.Styles[i].H)
	}
	for h := range used {
		if h.V != 0 && sk.Style(h) == nil {
			handles = append(handles, h)
		}
	}
	slices.SortFunc(handles, func(a, b core.HStyle) int { return cmp.Compare(a.V, b.V) })
	for _, h := range handles {
		ref := w.refStyle(h)
		s := sk.Style(h)
		if s == nil {
			s = &ref
		}
		custom := s.H.V >= core.StyleFirstCustom
		setters := styleSetters(s, &ref)
		if !custom && len(setters) == 0 && !used[s.H] {
			continue
		}
		v := fmt.Sprintf("st%d", s.H.V)
		w.styleVar[s.H] = v
		if custom {
			w.line("%s := doc.NewStyle()", v)
		} else {
			w.line("%s := doc.Style(%q)", v, ref.Name)
		}
		for _, l := range setters {
			w.line("%s.%s", v, l)
		}
	}
}

// refStyle is style h as it is made: a program's style at its factory
// settings, a custom one as a new one.
func (w *scriptWriter) refStyle(h core.HStyle) core.Style {
	var ns core.Style
	for i := range core.StyleDefaults {
		if d := &core.StyleDefaults[i]; d.H == h {
			core.FillDefaultStyle(&ns, d, false, w.sk.SettingsStore())
			ns.Name = core.CnfPrefixToName(d.CnfPrefix)
			ns.H = h
			return ns
		}
	}
	core.FillDefaultStyle(&ns, nil, false, w.sk.SettingsStore())
	ns.Name = "new-custom-style"
	ns.H = h
	return ns
}

// styleSetters are the calls that make ref's settings s's. A unit is
// set before the number it measures: setting it converts the number.
func styleSetters(s, ref *core.Style) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }
	if s.Name != ref.Name && s.H.V >= core.StyleFirstCustom {
		add("SetName(%q)", s.Name)
	}
	if s.Color.Red != ref.Color.Red || s.Color.Green != ref.Color.Green || s.Color.Blue != ref.Color.Blue {
		add("SetColor(%d, %d, %d)", s.Color.Red, s.Color.Green, s.Color.Blue)
	}
	if s.FillColor.Red != ref.FillColor.Red || s.FillColor.Green != ref.FillColor.Green || s.FillColor.Blue != ref.FillColor.Blue {
		add("SetFillColor(%d, %d, %d)", s.FillColor.Red, s.FillColor.Green, s.FillColor.Blue)
	}
	if s.Filled != ref.Filled {
		add("SetFilled(%v)", s.Filled)
	}
	if s.WidthAs != ref.WidthAs {
		add("SetWidthAs(core.UnitsAs(%d))", int(s.WidthAs))
	}
	if s.Width != ref.Width {
		add("SetWidth(%s)", num(s.Width))
	}
	if s.StippleType != ref.StippleType {
		add("SetStipple(core.StipplePattern(%d))", int(s.StippleType))
	}
	if s.StippleScale != ref.StippleScale {
		add("SetStippleScale(%s)", num(s.StippleScale))
	}
	if s.TextHeightAs != ref.TextHeightAs {
		add("SetTextHeightAs(core.UnitsAs(%d))", int(s.TextHeightAs))
	}
	if s.TextHeight != ref.TextHeight {
		add("SetTextHeight(%s)", num(s.TextHeight))
	}
	if s.TextOrigin != ref.TextOrigin {
		h, v := "center", "center"
		switch {
		case s.TextOrigin&core.TextOriginLeft != 0:
			h = "left"
		case s.TextOrigin&core.TextOriginRight != 0:
			h = "right"
		}
		switch {
		case s.TextOrigin&core.TextOriginBot != 0:
			v = "bottom"
		case s.TextOrigin&core.TextOriginTop != 0:
			v = "top"
		}
		add("SetTextAlign(%q, %q)", h, v)
	}
	if s.TextAngle != ref.TextAngle {
		add("SetTextAngle(%s)", num(s.TextAngle))
	}
	if s.ZIndex != ref.ZIndex {
		add("SetZIndex(%d)", s.ZIndex)
	}
	if s.Visible != ref.Visible {
		add("SetVisible(%v)", s.Visible)
	}
	if s.Exportable != ref.Exportable {
		add("SetExportable(%v)", s.Exportable)
	}
	return out
}

// styleRef is the script's variable for style h, as writeStyles made
// it.
func (w *scriptWriter) styleRef(h core.HStyle) (string, bool) {
	v, ok := w.styleVar[h]
	return v, ok
}

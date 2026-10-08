package ui

// The styles' user interface: custom styles, assigning them, and their
// text window screens.

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// CreateCustomStyle adds a custom style and returns its handle.
func (ss *Application) CreateCustomStyle(rememberForUndo bool) uint32 {
	if rememberForUndo {
		ss.UndoRemember()
	}
	return ss.SK.NewCustomStyle().V
}

// AssignSelectionToStyle gives the selected entities and constraints
// style v, or no style for 0.
func (ss *Application) AssignSelectionToStyle(v uint32) {
	sk := ss.SK
	gw := ss.GW
	showError := false
	gw.GroupSelection()

	ss.UndoRemember()
	for i := range gw.gs.entities {
		he := gw.gs.entity[i]
		e := sk.MustEntity(he)
		if !entityIsStylable(e) {
			continue
		}

		if !he.IsFromRequest() {
			showError = true
			continue
		}

		r := sk.Request(he.Request())
		r.Style = core.HStyle{V: v}
		sk.MarkGroupDirty(r.Group, false)
	}
	for i := range gw.gs.constraints {
		c := sk.Constraint(gw.gs.constraint[i])
		if !constraintIsStylable(c) {
			continue
		}

		c.Disp.Style = core.HStyle{V: v}
		sk.MarkGroupDirty(c.Group, false)
	}

	if showError {
		ss.Error("Can't assign style to an entity that's derived from another " +
			"entity; try assigning a style to this entity's parent.")
	}

	gw.ClearSelection()
	gw.Invalidate(false)

	// And show that style's info screen in the text window.
	ss.TW.GoToScreen(ScreenStyleInfo)
	ss.TW.shown.style = core.HStyle{V: v}
	ss.ScheduleShowTW()
}

// LoadFactoryDefaultStyles resets the default styles and the background
// colour.
func (ss *Application) LoadFactoryDefaultStyles() {
	ss.SK.LoadFactoryDefaults()
	ss.BackgroundColor = rgbi(0, 0, 0)
}

// ScreenShowListOfStyles shows the list of styles.
func (tw *TextWindow) ScreenShowListOfStyles(link int, v uint32) {
	tw.GoToScreen(ScreenListOfStyles)
}

// ScreenShowStyleInfo shows one style.
func (tw *TextWindow) ScreenShowStyleInfo(link int, v uint32) {
	tw.ss.GW.MenuEdit(CmdUnselectAll)
	tw.GoToScreen(ScreenStyleInfo)
	tw.shown.style = core.HStyle{V: v}
}

// ScreenLoadFactoryDefaultStyles resets the default styles.
func (tw *TextWindow) ScreenLoadFactoryDefaultStyles(link int, v uint32) {
	tw.ss.LoadFactoryDefaultStyles()
	tw.GoToScreen(ScreenListOfStyles)
	tw.ss.GW.persistentDirty = true
}

// ScreenCreateCustomStyle adds a custom style.
func (tw *TextWindow) ScreenCreateCustomStyle(link int, v uint32) {
	tw.ss.CreateCustomStyle(true)
}

// ScreenChangeBackgroundColor edits the background colour.
func (tw *TextWindow) ScreenChangeBackgroundColor(link int, v uint32) {
	rgb := tw.ss.BackgroundColor
	tw.ShowEditControlWithColorPicker(3, rgb)
	tw.edit.meaning = EditBackgroundColor
}

// ShowListOfStyles is the screen with every style.
func (tw *TextWindow) ShowListOfStyles() {
	ss := tw.ss
	tw.Printf(true, "%Ft color  style-name")

	darkbg := false
	for i := range ss.SK.Styles {
		s := &ss.SK.Styles[i]
		bg := byte('a')
		if darkbg {
			bg = 'd'
		}
		tw.Printf(false, "%Bp  %Bz   %Bp   %Fl%Ll%f%D%s%E",
			bg, &s.Color, bg,
			lf(tw.ScreenShowStyleInfo), s.H.V,
			s.DescriptionString())

		darkbg = !darkbg
	}

	tw.Printf(true, "  %Fl%Ll%fcreate a new custom style%E", lf(tw.ScreenCreateCustomStyle))

	tw.Printf(false, "")

	rgb := ss.BackgroundColor
	tw.Printf(false, "%Ft background color (r, g, b)%E")
	tw.Printf(false, "%Ba   %@, %@, %@ %Fl%D%f%Ll[change]%E",
		float64(rgb.RedF()), float64(rgb.GreenF()), float64(rgb.BlueF()),
		tw.lineTop(tw.rows-1)+2, lf(tw.ScreenChangeBackgroundColor))

	tw.Printf(false, "")
	tw.Printf(false, "  %Fl%Ll%fload factory defaults%E", lf(tw.ScreenLoadFactoryDefaultStyles))
}

// ScreenChangeStyleName renames a style.
func (tw *TextWindow) ScreenChangeStyleName(link int, v uint32) {
	hs := core.HStyle{V: v}
	s := tw.ss.SK.StyleGet(hs)
	tw.ShowEditControl(12, s.Name)
	tw.edit.style = hs
	tw.edit.meaning = EditStyleName
}

// ScreenDeleteStyle deletes a style.
func (tw *TextWindow) ScreenDeleteStyle(link int, v uint32) {
	ss := tw.ss
	ss.UndoRemember()
	hs := core.HStyle{V: v}
	for i := range ss.SK.Styles {
		if ss.SK.Styles[i].H == hs {
			ss.SK.Styles = append(ss.SK.Styles[:i], ss.SK.Styles[i+1:]...)
			// And it will get recreated automatically if something is
			// still using the style, so no need to do anything else.
			break
		}
	}
	tw.GoToScreen(ScreenListOfStyles)
	ss.GW.Invalidate(false)
}

// ScreenChangeStylePatternType sets a style's stipple pattern.
func (tw *TextWindow) ScreenChangeStylePatternType(link int, v uint32) {
	s := tw.ss.SK.StyleGet(core.HStyle{V: v})
	s.StippleType = core.StipplePattern(link - 1)
	tw.ss.GW.persistentDirty = true
}

// ScreenChangeStyleMetric edits a style's width, stipple period or text
// height.
func (tw *TextWindow) ScreenChangeStyleMetric(link int, v uint32) {
	hs := core.HStyle{V: v}
	s := tw.ss.SK.StyleGet(hs)
	var val float64
	var units core.UnitsAs
	var meaning Edit
	var col int
	switch link {
	case 't':
		val = s.TextHeight
		units = s.TextHeightAs
		col = 10
		meaning = EditStyleTextHeight

	case 's':
		val = s.StippleScale
		units = s.WidthAs
		col = 17
		meaning = EditStyleStipplePeriod

	case 'w', 'W':
		val = s.Width
		units = s.WidthAs
		col = 9
		meaning = EditStyleWidth

	default:
		panic("ui: unexpected link")
	}

	var editValue string
	if units == core.UnitsPixels {
		editValue = fmt.Sprintf("%.2f", val)
	} else {
		editValue = tw.ss.MmToString(val, true)
	}
	tw.ShowEditControl(col, editValue)
	tw.edit.style = hs
	tw.edit.meaning = meaning
}

// ScreenChangeStyleTextAngle edits a style's text angle.
func (tw *TextWindow) ScreenChangeStyleTextAngle(link int, v uint32) {
	hs := core.HStyle{V: v}
	s := tw.ss.SK.StyleGet(hs)
	tw.ShowEditControl(9, fmt.Sprintf("%.2f", s.TextAngle))
	tw.edit.style = hs
	tw.edit.meaning = EditStyleTextAngle
}

// ScreenChangeStyleColor edits a style's stroke or fill colour.
func (tw *TextWindow) ScreenChangeStyleColor(link int, v uint32) {
	hs := core.HStyle{V: v}
	s := tw.ss.SK.StyleGet(hs)
	// Same function used for stroke and fill colors
	var em Edit
	var rgb core.RGBA
	switch link {
	case 's':
		em = EditStyleColor
		rgb = s.Color
	case 'f':
		em = EditStyleFillColor
		rgb = s.FillColor
	default:
		panic("ui: unexpected link")
	}
	tw.ShowEditControlWithColorPicker(13, rgb)
	tw.edit.style = hs
	tw.edit.meaning = em
}

// ScreenChangeStyleYesNo flips one of a style's options.
func (tw *TextWindow) ScreenChangeStyleYesNo(link int, v uint32) {
	ss := tw.ss
	ss.UndoRemember()
	s := ss.SK.StyleGet(core.HStyle{V: v})
	scale := ss.GW.view.Scale
	switch link {
	// Units for the width
	case 'w':
		if s.WidthAs != core.UnitsMM {
			s.WidthAs = core.UnitsMM
			s.Width /= scale
			s.StippleScale /= scale
		}
	case 'W':
		if s.WidthAs != core.UnitsPixels {
			s.WidthAs = core.UnitsPixels
			s.Width *= scale
			s.StippleScale *= scale
		}

	// Units for the height
	case 'g':
		if s.TextHeightAs != core.UnitsMM {
			s.TextHeightAs = core.UnitsMM
			s.TextHeight /= scale
		}

	case 'G':
		if s.TextHeightAs != core.UnitsPixels {
			s.TextHeightAs = core.UnitsPixels
			s.TextHeight *= scale
		}

	case 'e':
		s.Exportable = !s.Exportable

	case 'v':
		s.Visible = !s.Visible

	case 'f':
		s.Filled = !s.Filled

	// Horizontal text alignment
	case 'L':
		s.TextOrigin |= core.TextOriginLeft
		s.TextOrigin &^= core.TextOriginRight
	case 'H':
		s.TextOrigin &^= core.TextOriginLeft
		s.TextOrigin &^= core.TextOriginRight
	case 'R':
		s.TextOrigin &^= core.TextOriginLeft
		s.TextOrigin |= core.TextOriginRight

	// Vertical text alignment
	case 'B':
		s.TextOrigin |= core.TextOriginBot
		s.TextOrigin &^= core.TextOriginTop
	case 'V':
		s.TextOrigin &^= core.TextOriginBot
		s.TextOrigin &^= core.TextOriginTop
	case 'T':
		s.TextOrigin &^= core.TextOriginBot
		s.TextOrigin |= core.TextOriginTop
	}
	ss.GW.Invalidate(true)
}

// EditControlDoneForStyles takes an edited style value, and reports
// whether the edit was one of the styles'.
func (tw *TextWindow) EditControlDoneForStyles(str string) bool {
	ss := tw.ss
	switch tw.edit.meaning {
	case EditStyleStipplePeriod, EditStyleTextHeight, EditStyleWidth:
		ss.UndoRemember()
		s := ss.SK.StyleGet(tw.edit.style)

		units := s.WidthAs
		if tw.edit.meaning == EditStyleTextHeight {
			units = s.TextHeightAs
		}
		var v float64
		if units == core.UnitsMM {
			v = ss.StringToMm(str)
		} else {
			v = atof(str)
		}
		v = geom.Max(0.0, v) // std::max: 0 for NaN
		switch tw.edit.meaning {
		case EditStyleTextHeight:
			s.TextHeight = v
		case EditStyleStipplePeriod:
			s.StippleScale = v
		default:
			s.Width = v
		}

	case EditStyleTextAngle:
		ss.UndoRemember()
		s := ss.SK.StyleGet(tw.edit.style)
		s.TextAngle = srf.WrapSymmetric(atof(str), 360)

	case EditBackgroundColor, EditStyleFillColor, EditStyleColor:
		if x, y, z, ok := scanVector3(str); ok {
			rgb := rgbfVec(vec3(x, y, z).ClampWithin(0, 1))
			switch tw.edit.meaning {
			case EditStyleColor:
				ss.UndoRemember()
				ss.SK.StyleGet(tw.edit.style).Color = rgb
			case EditStyleFillColor:
				ss.UndoRemember()
				ss.SK.StyleGet(tw.edit.style).FillColor = rgb
			default:
				ss.BackgroundColor = rgb
			}
		} else {
			ss.Error("Bad format: specify color as r, g, b")
		}

	case EditStyleName:
		if str == "" {
			ss.Error("Style name cannot be empty")
		} else {
			ss.UndoRemember()
			ss.SK.StyleGet(tw.edit.style).Name = str
		}

	default:
		return false
	}
	ss.GW.persistentDirty = true
	return true
}

// ShowStyleInfo is the screen for one style.
func (tw *TextWindow) ShowStyleInfo() {
	ss := tw.ss
	tw.Printf(true, "%Fl%f%Ll(back to list of styles)%E", lf(tw.ScreenShowListOfStyles))

	s := ss.SK.StyleGet(tw.shown.style)
	custom := s.H.V >= core.StyleFirstCustom

	if !custom {
		tw.Printf(true, "%FtSTYLE  %E%s ", s.DescriptionString())
	} else {
		tw.Printf(true, "%FtSTYLE  %E%s "+
			"[%Fl%Ll%D%frename%E/%Fl%Ll%D%fdel%E]",
			s.DescriptionString(),
			s.H.V, lf(tw.ScreenChangeStyleName),
			s.H.V, lf(tw.ScreenDeleteStyle))
	}
	tw.Printf(true, "%Ft line stroke style%E")
	tw.Printf(false, "%Ba   %Ftcolor %E%Bz  %Ba (%@, %@, %@) %D%f%Ls%Fl[change]%E",
		&s.Color,
		float64(s.Color.RedF()), float64(s.Color.GreenF()), float64(s.Color.BlueF()),
		s.H.V, lf(tw.ScreenChangeStyleColor))

	widthLink := byte('W')
	if !custom {
		widthLink = 'w'
	}
	// The line width, and its units
	if s.WidthAs == core.UnitsPixels {
		tw.Printf(false, "   %Ftwidth%E %@ %D%f%Lp%Fl[change]%E",
			s.Width, s.H.V, lf(tw.ScreenChangeStyleMetric), widthLink)
	} else {
		tw.Printf(false, "   %Ftwidth%E %s %D%f%Lp%Fl[change]%E",
			ss.MmToString(s.Width, false), s.H.V, lf(tw.ScreenChangeStyleMetric), widthLink)
	}

	if s.WidthAs == core.UnitsPixels {
		tw.Printf(false, "%Ba   %Ftstipple width%E %@ %D%f%Lp%Fl[change]%E",
			s.StippleScale, s.H.V, lf(tw.ScreenChangeStyleMetric), 's')
	} else {
		tw.Printf(false, "%Ba   %Ftstipple width%E %s %D%f%Lp%Fl[change]%E",
			ss.MmToString(s.StippleScale, false), s.H.V, lf(tw.ScreenChangeStyleMetric), 's')
	}

	widthpx := s.WidthAs == core.UnitsPixels
	if !custom {
		tw.Printf(false, "   %Ftin units of %Fdpixels%E")
	} else {
		tw.Printf(false, "%Ba   %Ftin units of  %Fd"+
			"%D%f%LW%s pixels%E  "+
			"%D%f%Lw%s %s",
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(widthpx),
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(!widthpx),
			ss.UnitName())
	}

	tw.Printf(false, "%Ba   %Ftstipple type:%E")

	patternsSource := [...]string{
		"___________",
		"-  -  -  - ",
		"- - - - - -",
		"__ __ __ __",
		"-.-.-.-.-.-",
		"..-..-..-..",
		"...........",
		"~~~~~~~~~~~",
		"__~__~__~__",
	}
	for i, src := range patternsSource {
		var pattern []byte
		for k := range len(src) {
			switch src[k] {
			case ' ':
				pattern = append(pattern, ' ')
			case '.':
				pattern = append(pattern, ""...)
			case '_':
				pattern = append(pattern, ""...)
			case '-':
				pattern = append(pattern, ""...)
			case '~':
				pattern = append(pattern, ""...)
			default:
				panic("ui: unexpected stipple pattern element")
			}
		}
		bg := byte('a')
		if i%2 == 0 {
			bg = 'd'
		}
		tw.Printf(false, "%Bp     %D%f%Lp%s %s%E",
			bg, s.H.V, lf(tw.ScreenChangeStylePatternType),
			i+1, radioMark(s.StippleType == core.StipplePattern(i)), string(pattern))
	}

	if custom {
		// The fill color, and whether contours are filled

		tw.Printf(false, "")
		tw.Printf(false, "%Ft contour fill style%E")
		tw.Printf(false,
			"%Ba   %Ftcolor %E%Bz  %Ba (%@, %@, %@) %D%f%Lf%Fl[change]%E",
			&s.FillColor,
			float64(s.FillColor.RedF()), float64(s.FillColor.GreenF()), float64(s.FillColor.BlueF()),
			s.H.V, lf(tw.ScreenChangeStyleColor))

		tw.Printf(false, "%Bd   %D%f%Lf%s  contours are filled%E",
			s.H.V, lf(tw.ScreenChangeStyleYesNo), checkMark(s.Filled))
	}

	// The text height, and its units
	tw.Printf(false, "")
	tw.Printf(false, "%Ft text style%E")

	if s.TextHeightAs == core.UnitsPixels {
		tw.Printf(false, "%Ba   %Ftheight %E%@ %D%f%Lt%Fl%s%E",
			s.TextHeight, s.H.V, lf(tw.ScreenChangeStyleMetric), "[change]")
	} else {
		tw.Printf(false, "%Ba   %Ftheight %E%s %D%f%Lt%Fl%s%E",
			ss.MmToString(s.TextHeight, false), s.H.V, lf(tw.ScreenChangeStyleMetric), "[change]")
	}

	textHeightpx := s.TextHeightAs == core.UnitsPixels
	if !custom {
		tw.Printf(false, "%Bd   %Ftin units of %Fdpixels")
	} else {
		tw.Printf(false, "%Bd   %Ftin units of  %Fd"+
			"%D%f%LG%s pixels%E  "+
			"%D%f%Lg%s %s",
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(textHeightpx),
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(!textHeightpx),
			ss.UnitName())
	}

	if custom {
		tw.Printf(false, "%Ba   %Ftangle %E%@ %D%f%Ll%Fl[change]%E",
			s.TextAngle, s.H.V, lf(tw.ScreenChangeStyleTextAngle))

		tw.Printf(false, "")
		tw.Printf(false, "%Ft text comment alignment%E")
		neither := s.TextOrigin&(core.TextOriginLeft|core.TextOriginRight) == 0
		tw.Printf(false, "%Ba   "+
			"%D%f%LL%s left%E    "+
			"%D%f%LH%s center%E  "+
			"%D%f%LR%s right%E  ",
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(s.TextOrigin&core.TextOriginLeft != 0),
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(neither),
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(s.TextOrigin&core.TextOriginRight != 0))

		neither = s.TextOrigin&(core.TextOriginBot|core.TextOriginTop) == 0
		tw.Printf(false, "%Bd   "+
			"%D%f%LB%s bottom%E  "+
			"%D%f%LV%s center%E  "+
			"%D%f%LT%s top%E  ",
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(s.TextOrigin&core.TextOriginBot != 0),
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(neither),
			s.H.V, lf(tw.ScreenChangeStyleYesNo), radioMark(s.TextOrigin&core.TextOriginTop != 0))
	}

	tw.Printf(false, "")

	if custom {
		tw.Printf(false, "  %Fd%D%f%Lv%s  show these objects on screen%E",
			s.H.V, lf(tw.ScreenChangeStyleYesNo), checkMark(s.Visible))
	}

	tw.Printf(false, "  %Fd%D%f%Le%s  export these objects%E",
		s.H.V, lf(tw.ScreenChangeStyleYesNo), checkMark(s.Exportable))

	if custom {
		tw.Printf(false, "")
		tw.Printf(false, "To assign lines or curves to this style,")
		tw.Printf(false, "right-click them on the drawing.")
	}
}

// ScreenAssignSelectionToStyle gives the selection style v.
func (tw *TextWindow) ScreenAssignSelectionToStyle(link int, v uint32) {
	tw.ss.AssignSelectionToStyle(v)
}

package core

import (
	"fmt"
	"math"
	"strings"
)

type StipplePattern int

const (
	StippleContinuous StipplePattern = iota
	StippleShortDash
	StippleDash
	StippleLongDash
	StippleDashDot
	StippleDashDotDot
	StippleDot
	StippleFreehand
	StippleZigzag
)

type UnitsAs int

const (
	UnitsPixels UnitsAs = iota
	UnitsMM
)

type TextOrigin int

const (
	TextOriginNone  TextOrigin = 0x00
	TextOriginLeft  TextOrigin = 0x01
	TextOriginRight TextOrigin = 0x02
	TextOriginBot   TextOrigin = 0x04
	TextOriginTop   TextOrigin = 0x08
)

const (
	StyleNoStyle      = 0
	StyleActiveGrp    = 1
	StyleConstruction = 2
	StyleInactiveGrp  = 3
	StyleDatum        = 4
	StyleSolidEdge    = 5
	StyleConstraint   = 6
	StyleSelected     = 7
	StyleHovered      = 8
	StyleContourFill  = 9
	StyleNormals      = 10
	StyleAnalyze      = 11
	StyleDrawError    = 12
	StyleDimSolid     = 13
	StyleHiddenEdge   = 14
	StyleOutline      = 15
	// StyleSectionCap fills and hatches the cut of the section view
	// (an extension).
	StyleSectionCap  = 16
	StyleFirstCustom = 0x100
)

type Style struct {
	Tag int
	H   HStyle

	Name string

	Width        float64
	WidthAs      UnitsAs
	TextHeight   float64
	TextHeightAs UnitsAs
	TextOrigin   TextOrigin
	TextAngle    float64
	Color        RGBA
	Filled       bool
	FillColor    RGBA
	Visible      bool
	Exportable   bool
	StippleType  StipplePattern
	StippleScale float64
	ZIndex       int
}

type StyleList []Style

// StyleDefault is a default style's factory settings (C++
// Style::Default).
type StyleDefault struct {
	H           HStyle
	CnfPrefix   string
	Color       RGBA
	Width       float64
	ZIndex      int
	Exportable  bool
	StippleType StipplePattern
}

func rgbf(r, g, b float32) RGBA { return RGBAFromFloat(r, g, b, 1.0) }

// StyleDefaults are the default styles (C++ Style::Defaults).
var StyleDefaults = []StyleDefault{
	{HStyle{StyleActiveGrp}, "ActiveGrp", rgbf(1.0, 1.0, 1.0), 1.5, 4, true, StippleContinuous},
	{HStyle{StyleConstruction}, "Construction", rgbf(0.1, 0.7, 0.1), 1.5, 0, false, StippleContinuous},
	{HStyle{StyleInactiveGrp}, "InactiveGrp", rgbf(0.5, 0.3, 0.0), 1.5, 3, true, StippleContinuous},
	{HStyle{StyleDatum}, "Datum", rgbf(0.0, 0.8, 0.0), 1.5, 0, true, StippleContinuous},
	{HStyle{StyleSolidEdge}, "SolidEdge", rgbf(0.8, 0.8, 0.8), 1.0, 2, true, StippleContinuous},
	{HStyle{StyleConstraint}, "Constraint", rgbf(1.0, 0.1, 1.0), 1.0, 0, true, StippleContinuous},
	{HStyle{StyleSelected}, "Selected", rgbf(1.0, 0.0, 0.0), 1.5, 0, true, StippleContinuous},
	{HStyle{StyleHovered}, "Hovered", rgbf(1.0, 1.0, 0.0), 1.5, 0, true, StippleContinuous},
	{HStyle{StyleContourFill}, "ContourFill", rgbf(0.0, 0.1, 0.1), 1.0, 0, true, StippleContinuous},
	{HStyle{StyleNormals}, "Normals", rgbf(0.0, 0.4, 0.4), 1.0, 0, true, StippleContinuous},
	{HStyle{StyleAnalyze}, "Analyze", rgbf(0.0, 1.0, 1.0), 3.0, 0, true, StippleContinuous},
	{HStyle{StyleDrawError}, "DrawError", rgbf(1.0, 0.0, 0.0), 8.0, 0, true, StippleContinuous},
	{HStyle{StyleDimSolid}, "DimSolid", rgbf(0.1, 0.1, 0.1), 1.0, 0, true, StippleContinuous},
	{HStyle{StyleHiddenEdge}, "HiddenEdge", rgbf(0.8, 0.8, 0.8), 1.0, 1, true, StippleDash},
	{HStyle{StyleOutline}, "Outline", rgbf(0.8, 0.8, 0.8), 3.0, 5, true, StippleContinuous},
	{HStyle{StyleSectionCap}, "SectionCap", RGBA{Red: 205, Green: 100, Blue: 80, Alpha: 255}, 1.0, 1, true, StippleContinuous},
}

// FillDefaultStyle sets s to default style d, or to the active group's
// style for nil: its factory settings if factory, else the user's
// settings for it (C++ Style::FillDefaultStyle).
func FillDefaultStyle(s *Style, d *StyleDefault, factory bool) {
	settings := GetSettings()

	if d == nil {
		d = &StyleDefaults[0]
	}
	if factory {
		s.Color = d.Color
		s.Width = d.Width
	} else {
		s.Color = ThawColor(settings, CnfColor(d.CnfPrefix), d.Color)
		s.Width = settings.ThawFloat(CnfWidth(d.CnfPrefix), float64(float32(d.Width)))
	}
	s.WidthAs = UnitsPixels
	if factory {
		s.TextHeight = 11.5
	} else {
		s.TextHeight = settings.ThawFloat(CnfTextHeight(d.CnfPrefix), 11.5)
	}
	s.TextHeightAs = UnitsPixels
	s.TextOrigin = TextOriginNone
	s.TextAngle = 0
	s.Visible = true
	if factory {
		s.Exportable = d.Exportable
	} else {
		s.Exportable = ThawBool(settings, CnfExportable(d.CnfPrefix), d.Exportable)
	}
	s.Filled = false
	s.FillColor = rgbf(0.3, 0.3, 0.3)
	if factory {
		s.StippleType = d.StippleType
		s.StippleScale = 15.0
	} else {
		s.StippleType = StipplePatternFromString(
			settings.ThawString(CnfStippleType(d.CnfPrefix), StipplePatternName(d.StippleType)))
		s.StippleScale = settings.ThawFloat(CnfStippleScale(d.CnfPrefix), 15.0)
	}
	s.ZIndex = d.ZIndex
}

// CnfPrefixToName is a default style's name, e.g. "#def-active-grp"
// (C++ Style::CnfPrefixToName).
func CnfPrefixToName(prefix string) string {
	var name strings.Builder
	name.WriteString("#def-")
	for i := range len(prefix) {
		ch := prefix[i]
		if ch >= 'A' && ch <= 'Z' && i != 0 {
			name.WriteString("-")
		}
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		name.WriteString(string(ch))
	}
	return name.String()
}

// DescriptionString names the style, e.g. "s100-thick" (C++
// Style::DescriptionString).
func (s *Style) DescriptionString() string {
	if s.Name == "" {
		return fmt.Sprintf("s%03x-(unnamed)", s.H.V)
	}
	return fmt.Sprintf("s%03x-%s", s.H.V, s.Name)
}

// StipplePatternFromString is the pattern with the given name, ignoring
// case; continuous if none (C++ Style::StipplePatternFromString).
func StipplePatternFromString(name string) StipplePattern {
	switch strings.ToLower(name) {
	case "continuous":
		return StippleContinuous
	case "shortdash":
		return StippleShortDash
	case "dash":
		return StippleDash
	case "longdash":
		return StippleLongDash
	case "dashdot":
		return StippleDashDot
	case "dashdotdot":
		return StippleDashDotDot
	case "dot":
		return StippleDot
	case "freehand":
		return StippleFreehand
	case "zigzag":
		return StippleZigzag
	}
	return StippleContinuous
}

// StipplePatternName is the pattern's name (C++
// Style::StipplePatternName).
func StipplePatternName(p StipplePattern) string {
	switch p {
	case StippleContinuous:
		return "Continuous"
	case StippleShortDash:
		return "ShortDash"
	case StippleDash:
		return "Dash"
	case StippleLongDash:
		return "LongDash"
	case StippleDashDot:
		return "DashDot"
	case StippleDashDotDot:
		return "DashDotDot"
	case StippleDot:
		return "Dot"
	case StippleFreehand:
		return "FreeHand"
	case StippleZigzag:
		return "ZigZag"
	}
	return "Continuous"
}

// RewriteColor turns an almost-white colour black when fixExportColors,
// so it doesn't vanish on a light background (C++ Style::RewriteColor,
// with SS.fixExportColors as an argument).
func RewriteColor(rgbin RGBA, fixExportColors bool) RGBA {
	dr := float64(rgbin.RedF()) - 1
	dg := float64(rgbin.GreenF()) - 1
	db := float64(rgbin.BlueF()) - 1
	if math.Sqrt(dr*dr+dg*dg+db*db) < 0.4 && fixExportColors {
		// This is an almost-white color in a default style, which is
		// good for the default on-screen view (black bg) but probably
		// not desired in the exported files, which typically are shown
		// against white backgrounds.
		return NewRGBA(0, 0, 0, 255)
	}
	return rgbin
}

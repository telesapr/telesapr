package core

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
	StyleFirstCustom  = 0x100
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
	Color        RgbaColor
	Filled       bool
	FillColor    RgbaColor
	Visible      bool
	Exportable   bool
	StippleType  StipplePattern
	StippleScale float64
	ZIndex       int
}

func (s *Style) Clear() {}

type StyleList []Style

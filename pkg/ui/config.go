package ui

import (
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/export"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Config is the user's configuration: what the configuration screen
// and the menus set and the settings store keeps between runs. The
// application embeds it.
type Config struct {
	ModelColor               [ModelColors]core.RGBA
	LightDir                 [2]geom.Vector
	LightIntensity           [2]float64
	AmbientIntensity         float64
	ChordTol                 float64
	MaxSegments              int
	ExportChordTol           float64
	ExportMaxSegments        int
	TimeoutRedundantConstr   int // milliseconds
	AnimationSpeed           int // milliseconds
	CameraTangentSetting     float64
	GridSpacing              float64
	ExportScale              float64
	ExportOffset             float64
	ArcDimDefaultDiameter    bool
	ShowFullFilePath         bool
	FixExportColors          bool
	ExportBackgroundColor    bool
	DrawBackFaces            bool
	ShowContourAreas         bool
	CheckClosedContour       bool
	CameraNav                bool
	TurntableNav             bool
	ImmediatelyEditDimension bool
	AutomaticLineConstraints bool
	// ColorByFreedom colours the active group's entities by how much
	// of them their constraints fix.
	ColorByFreedom bool
	// Shaded lights the solid by the faces' appearances, through a
	// shader with the GL renderer.
	Shaded bool
	// UseOpenGL draws the windows with OpenGL where the platform has
	// it.
	UseOpenGL             bool
	ShowToolbar           bool
	BackgroundColor       core.RGBA
	ExportShadedTriangles bool
	ExportPwlCurves       bool
	ExportCanvasSizeAuto  bool
	ExportMargin          export.Margin
	ExportCanvas          export.Canvas
	GCode                 export.GCode
	// DefaultUnits is the unit of a new document, the last one the
	// user chose.
	DefaultUnits       core.Unit
	AfterDecimalMm     int
	AfterDecimalInch   int
	AfterDecimalDegree int
	UseSIPrefixes      bool
	AutosaveInterval   int // minutes
	RecentFiles        []string
	// AgentAllowed is the setting of Script > Allow AI Connections: a
	// program that draws by writing scripts, such as an AI model's, may
	// connect (see Application.AllowAgent). AgentConfirm has the user
	// asked before each script of such a program runs.
	AgentAllowed bool
	AgentConfirm bool
}

// Thaw reads the configuration from the store, with the defaults for
// what it doesn't hold.
func (c *Config) Thaw(s core.Settings) {

	defaults := [ModelColors]core.RGBA{
		rgbi(150, 150, 150), rgbi(100, 100, 100), rgbi(30, 30, 30), rgbi(150, 0, 0),
		rgbi(0, 100, 0), rgbi(0, 80, 80), rgbi(0, 0, 130), rgbi(80, 0, 80),
	}
	for i := range c.ModelColor {
		c.ModelColor[i] = core.ThawColor(s, "ModelColor_"+strconv.Itoa(i), defaults[i])
	}
	c.LightIntensity[0] = s.ThawFloat("LightIntensity_0", 1.0)
	c.LightIntensity[1] = s.ThawFloat("LightIntensity_1", 0.5)
	c.AmbientIntensity = s.ThawFloat("Light_Ambient", 0.3)
	c.LightDir[0].X = s.ThawFloat("LightDir_0_Right", -1.0)
	c.LightDir[0].Y = s.ThawFloat("LightDir_0_Up", 1.0)
	c.LightDir[0].Z = s.ThawFloat("LightDir_0_Forward", 0.0)
	c.LightDir[1].X = s.ThawFloat("LightDir_1_Right", 1.0)
	c.LightDir[1].Y = s.ThawFloat("LightDir_1_Up", 0.0)
	c.LightDir[1].Z = s.ThawFloat("LightDir_1_Forward", 0.0)

	c.ChordTol = s.ThawFloat("ChordTolerancePct", 0.1)
	c.MaxSegments = int(s.ThawInt("MaxSegments", 20))
	c.ExportChordTol = s.ThawFloat("ExportChordTolerance", 0.1)
	c.ExportMaxSegments = int(s.ThawInt("ExportMaxSegments", 64))
	c.TimeoutRedundantConstr = int(s.ThawInt("TimeoutRedundantConstraints", 1000))
	c.AnimationSpeed = int(s.ThawInt("AnimationSpeed", 800))
	c.DefaultUnits = core.Unit(s.ThawInt("ViewUnits", uint32(core.UnitMM)))
	c.AfterDecimalMm = int(s.ThawInt("AfterDecimalMm", 2))
	c.AfterDecimalInch = int(s.ThawInt("AfterDecimalInch", 3))
	c.AfterDecimalDegree = int(s.ThawInt("AfterDecimalDegree", 2))
	c.UseSIPrefixes = core.ThawBool(s, "UseSIPrefixes", false)
	c.CameraTangentSetting = s.ThawFloat("CameraTangent", float64(float32(0.3))/1e3) // C++ 0.3f/1e3, in double
	c.GridSpacing = s.ThawFloat("GridSpacing", 5.0)
	c.ExportScale = s.ThawFloat("ExportScale", 1.0)
	c.ExportOffset = s.ThawFloat("ExportOffset", 0.0)
	c.ArcDimDefaultDiameter = core.ThawBool(s, "ArcDimDefaultDiameter", false)
	c.ShowFullFilePath = core.ThawBool(s, "ShowFullFilePath", true)
	c.FixExportColors = core.ThawBool(s, "FixExportColors", true)
	c.ExportBackgroundColor = core.ThawBool(s, "ExportBackgroundColor", false)
	c.DrawBackFaces = core.ThawBool(s, "DrawBackFaces", true)
	c.CameraNav = core.ThawBool(s, "CameraNav", false)
	c.TurntableNav = core.ThawBool(s, "TurntableNav", false)
	c.ImmediatelyEditDimension = core.ThawBool(s, "ImmediatelyEditDimension", true)
	c.CheckClosedContour = core.ThawBool(s, "CheckClosedContour", true)
	c.AutomaticLineConstraints = core.ThawBool(s, "AutomaticLineConstraints", true)
	c.ColorByFreedom = core.ThawBool(s, "ColorByFreedom", true)
	c.Shaded = core.ThawBool(s, "Shaded", false)
	c.UseOpenGL = core.ThawBool(s, "UseOpenGL", true)
	c.ShowContourAreas = core.ThawBool(s, "ShowContourAreas", false)
	c.ExportShadedTriangles = core.ThawBool(s, "ExportShadedTriangles", true)
	c.ExportPwlCurves = core.ThawBool(s, "ExportPwlCurves", false)
	c.BackgroundColor = core.ThawColor(s, "BackgroundColor", rgbi(0, 0, 0))
	c.ExportCanvasSizeAuto = core.ThawBool(s, "ExportCanvasSizeAuto", true)
	c.ExportMargin.Left = s.ThawFloat("ExportMargin_Left", 5.0)
	c.ExportMargin.Right = s.ThawFloat("ExportMargin_Right", 5.0)
	c.ExportMargin.Bottom = s.ThawFloat("ExportMargin_Bottom", 5.0)
	c.ExportMargin.Top = s.ThawFloat("ExportMargin_Top", 5.0)
	c.ExportCanvas.Width = s.ThawFloat("ExportCanvas_Width", 100.0)
	c.ExportCanvas.Height = s.ThawFloat("ExportCanvas_Height", 100.0)
	c.ExportCanvas.Dx = s.ThawFloat("ExportCanvas_Dx", 5.0)
	c.ExportCanvas.Dy = s.ThawFloat("ExportCanvas_Dy", 5.0)
	c.GCode.Depth = s.ThawFloat("GCode_Depth", 10.0)
	c.GCode.SafeHeight = s.ThawFloat("GCode_SafeHeight", 5.0)
	c.GCode.Passes = int(s.ThawInt("GCode_Passes", 1))
	c.GCode.Feed = s.ThawFloat("GCode_Feed", 10.0)
	c.GCode.PlungeFeed = s.ThawFloat("GCode_PlungeFeed", 10.0)
	c.ShowToolbar = core.ThawBool(s, "ShowToolbar", true)
	c.AgentAllowed = core.ThawBool(s, "AgentAllowed", false)
	c.AgentConfirm = core.ThawBool(s, "AgentConfirm", false)
	c.RecentFiles = nil
	for i := range MaxRecent {
		if raw := s.ThawString("RecentFile_"+strconv.Itoa(i), ""); raw != "" {
			c.RecentFiles = append(c.RecentFiles, raw)
		}
	}
	c.AutosaveInterval = int(s.ThawInt("AutosaveInterval", 5))
}

// Freeze writes the configuration to the store.
func (c *Config) Freeze(s core.Settings) {
	for i := range MaxRecent {
		raw := ""
		if i < len(c.RecentFiles) {
			raw = c.RecentFiles[i]
		}
		s.FreezeString("RecentFile_"+strconv.Itoa(i), raw)
	}
	for i := range c.ModelColor {
		core.FreezeColor(s, "ModelColor_"+strconv.Itoa(i), c.ModelColor[i])
	}
	f32 := func(v float64) float64 { return float64(float32(v)) }
	s.FreezeFloat("LightIntensity_0", f32(c.LightIntensity[0]))
	s.FreezeFloat("LightIntensity_1", f32(c.LightIntensity[1]))
	s.FreezeFloat("Light_Ambient", f32(c.AmbientIntensity))
	s.FreezeFloat("LightDir_0_Right", f32(c.LightDir[0].X))
	s.FreezeFloat("LightDir_0_Up", f32(c.LightDir[0].Y))
	s.FreezeFloat("LightDir_0_Forward", f32(c.LightDir[0].Z))
	s.FreezeFloat("LightDir_1_Right", f32(c.LightDir[1].X))
	s.FreezeFloat("LightDir_1_Up", f32(c.LightDir[1].Y))
	s.FreezeFloat("LightDir_1_Forward", f32(c.LightDir[1].Z))
	s.FreezeFloat("ChordTolerancePct", f32(c.ChordTol))
	s.FreezeInt("MaxSegments", uint32(c.MaxSegments))
	s.FreezeFloat("ExportChordTolerance", f32(c.ExportChordTol))
	s.FreezeInt("ExportMaxSegments", uint32(c.ExportMaxSegments))
	s.FreezeInt("TimeoutRedundantConstraints", uint32(c.TimeoutRedundantConstr))
	s.FreezeInt("AnimationSpeed", uint32(c.AnimationSpeed))
	s.FreezeInt("ViewUnits", uint32(c.DefaultUnits))
	s.FreezeInt("AfterDecimalMm", uint32(c.AfterDecimalMm))
	s.FreezeInt("AfterDecimalInch", uint32(c.AfterDecimalInch))
	s.FreezeInt("AfterDecimalDegree", uint32(c.AfterDecimalDegree))
	core.FreezeBool(s, "UseSIPrefixes", c.UseSIPrefixes)
	s.FreezeFloat("CameraTangent", f32(c.CameraTangentSetting))
	s.FreezeFloat("GridSpacing", c.GridSpacing)
	s.FreezeFloat("ExportScale", c.ExportScale)
	s.FreezeFloat("ExportOffset", c.ExportOffset)
	core.FreezeBool(s, "ArcDimDefaultDiameter", c.ArcDimDefaultDiameter)
	core.FreezeBool(s, "ShowFullFilePath", c.ShowFullFilePath)
	core.FreezeBool(s, "FixExportColors", c.FixExportColors)
	core.FreezeBool(s, "ExportBackgroundColor", c.ExportBackgroundColor)
	core.FreezeBool(s, "DrawBackFaces", c.DrawBackFaces)
	core.FreezeBool(s, "ShowContourAreas", c.ShowContourAreas)
	core.FreezeBool(s, "CheckClosedContour", c.CheckClosedContour)
	core.FreezeBool(s, "CameraNav", c.CameraNav)
	core.FreezeBool(s, "TurntableNav", c.TurntableNav)
	core.FreezeBool(s, "ImmediatelyEditDimension", c.ImmediatelyEditDimension)
	core.FreezeBool(s, "AutomaticLineConstraints", c.AutomaticLineConstraints)
	core.FreezeBool(s, "ColorByFreedom", c.ColorByFreedom)
	core.FreezeBool(s, "Shaded", c.Shaded)
	core.FreezeBool(s, "UseOpenGL", c.UseOpenGL)
	core.FreezeBool(s, "ExportShadedTriangles", c.ExportShadedTriangles)
	core.FreezeBool(s, "ExportPwlCurves", c.ExportPwlCurves)
	core.FreezeColor(s, "BackgroundColor", c.BackgroundColor)
	core.FreezeBool(s, "ExportCanvasSizeAuto", c.ExportCanvasSizeAuto)
	s.FreezeFloat("ExportMargin_Left", c.ExportMargin.Left)
	s.FreezeFloat("ExportMargin_Right", c.ExportMargin.Right)
	s.FreezeFloat("ExportMargin_Bottom", c.ExportMargin.Bottom)
	s.FreezeFloat("ExportMargin_Top", c.ExportMargin.Top)
	s.FreezeFloat("ExportCanvas_Width", c.ExportCanvas.Width)
	s.FreezeFloat("ExportCanvas_Height", c.ExportCanvas.Height)
	s.FreezeFloat("ExportCanvas_Dx", c.ExportCanvas.Dx)
	s.FreezeFloat("ExportCanvas_Dy", c.ExportCanvas.Dy)
	s.FreezeFloat("GCode_Depth", c.GCode.Depth)
	s.FreezeInt("GCode_Passes", uint32(c.GCode.Passes))
	s.FreezeFloat("GCode_Feed", c.GCode.Feed)
	s.FreezeFloat("GCode_PlungeFeed", c.GCode.PlungeFeed)
	core.FreezeBool(s, "ShowToolbar", c.ShowToolbar)
	core.FreezeBool(s, "AgentAllowed", c.AgentAllowed)
	core.FreezeBool(s, "AgentConfirm", c.AgentConfirm)
	s.FreezeInt("AutosaveInterval", uint32(c.AutosaveInterval))
}

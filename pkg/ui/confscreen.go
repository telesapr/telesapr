package ui

// The configuration screen: settings that are not specific to the file
// being edited.

import (
	"fmt"
	"math"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// toF32 rounds to a single-precision float, as the setting is stored.
func toF32(v float64) float64 { return float64(float32(v)) }

func (tw *TextWindow) ScreenChangeColor(link int, v uint32) {
	tw.ShowEditControlWithColorPicker(13, tw.ss.ModelColor[v])

	tw.edit.meaning = EditColor
	tw.edit.i = int(v)
}

func (tw *TextWindow) ScreenChangeChordTolerance(link int, v uint32) {
	tw.ShowEditControl(3, cFormatG(6, tw.ss.ChordTol))
	tw.edit.meaning = EditChordTolerance
	tw.edit.i = 0
}

func (tw *TextWindow) ScreenChangeMaxSegments(link int, v uint32) {
	tw.ShowEditControl(3, fmt.Sprintf("%d", tw.ss.MaxSegments))
	tw.edit.meaning = EditMaxSegments
	tw.edit.i = 0
}

func (tw *TextWindow) ScreenChangeExportChordTolerance(link int, v uint32) {
	tw.ShowEditControl(3, cFormatG(6, tw.ss.ExportChordTol))
	tw.edit.meaning = EditChordTolerance
	tw.edit.i = 1
}

func (tw *TextWindow) ScreenChangeExportMaxSegments(link int, v uint32) {
	tw.ShowEditControl(3, fmt.Sprintf("%d", tw.ss.ExportMaxSegments))
	tw.edit.meaning = EditMaxSegments
	tw.edit.i = 1
}

func (tw *TextWindow) ScreenChangeGridSpacing(link int, v uint32) {
	tw.ShowEditControl(3, tw.ss.MmToString(tw.ss.GridSpacing, true))
	tw.edit.meaning = EditGridSpacing
}

func (tw *TextWindow) ScreenChangeDigitsAfterDecimal(link int, v uint32) {
	tw.ShowEditControl(14, fmt.Sprintf("%d", tw.ss.UnitDigitsAfterDecimal()))
	tw.edit.meaning = EditDigitsAfterDecimal
}

func (tw *TextWindow) ScreenChangeDigitsAfterDecimalDegree(link int, v uint32) {
	tw.ShowEditControl(14, fmt.Sprintf("%d", tw.ss.AfterDecimalDegree))
	tw.edit.meaning = EditDigitsAfterDecimalDegree
}

func (tw *TextWindow) ScreenChangeUseSIPrefixes(link int, v uint32) {
	tw.ss.UseSIPrefixes = !tw.ss.UseSIPrefixes
	tw.ss.GW.Invalidate(false)
}

func (tw *TextWindow) ScreenChangeExportScale(link int, v uint32) {
	tw.ShowEditControl(5, fmt.Sprintf("%.3f", tw.ss.ExportScale))
	tw.edit.meaning = EditExportScale
}

func (tw *TextWindow) ScreenChangeExportOffset(link int, v uint32) {
	tw.ShowEditControl(3, tw.ss.MmToString(tw.ss.ExportOffset, true))
	tw.edit.meaning = EditExportOffset
}

func (tw *TextWindow) ScreenChangeArcDimDefault(link int, v uint32) {
	tw.ss.ArcDimDefaultDiameter = !tw.ss.ArcDimDefaultDiameter
}

func (tw *TextWindow) ScreenChangeShowFullFilePath(link int, v uint32) {
	tw.ss.ShowFullFilePath = !tw.ss.ShowFullFilePath
	tw.ss.UpdateWindowTitles()
}

func (tw *TextWindow) ScreenChangeFixExportColors(link int, v uint32) {
	tw.ss.FixExportColors = !tw.ss.FixExportColors
}

func (tw *TextWindow) ScreenChangeExportBackgroundColor(link int, v uint32) {
	tw.ss.ExportBackgroundColor = !tw.ss.ExportBackgroundColor
}

func (tw *TextWindow) ScreenChangeBackFaces(link int, v uint32) {
	tw.ss.DrawBackFaces = !tw.ss.DrawBackFaces
	tw.ss.GW.Invalidate(true)
}

func (tw *TextWindow) ScreenChangeTurntableNav(link int, v uint32) {
	ss := tw.ss
	ss.TurntableNav = !ss.TurntableNav
	if ss.TurntableNav {
		// If turntable nav is being turned on, align view so Z is
		// vertical
		ss.GW.AnimateOnto(geom.QuaternionFromBasis(geom.Vector{X: -1}, geom.Vector{Z: 1}),
			ss.GW.view.Offset)
	}
}

// ScreenChangeUseOpenGL switches between the OpenGL and the software
// renderer (an extension).
func (tw *TextWindow) ScreenChangeUseOpenGL(link int, v uint32) {
	tw.ss.SetUseOpenGL(!tw.ss.UseGL())
}

// ScreenChangeAgentConfirm turns asking before each script of a
// program that connected through Script > Allow AI Connections on or
// off (an extension).
func (tw *TextWindow) ScreenChangeAgentConfirm(link int, v uint32) {
	tw.ss.AgentConfirm = !tw.ss.AgentConfirm
}

func (tw *TextWindow) ScreenChangeCameraNav(link int, v uint32) {
	tw.ss.CameraNav = !tw.ss.CameraNav
}

func (tw *TextWindow) ScreenChangeImmediatelyEditDimension(link int, v uint32) {
	tw.ss.ImmediatelyEditDimension = !tw.ss.ImmediatelyEditDimension
	tw.ss.GW.Invalidate(true)
}

func (tw *TextWindow) ScreenChangeShowContourAreas(link int, v uint32) {
	tw.ss.ShowContourAreas = !tw.ss.ShowContourAreas
	tw.ss.GW.Invalidate(false)
}

func (tw *TextWindow) ScreenChangeCheckClosedContour(link int, v uint32) {
	tw.ss.CheckClosedContour = !tw.ss.CheckClosedContour
	tw.ss.GW.Invalidate(false)
}

// ScreenChangeColorByFreedom turns the colouring of entities by
// constraint state on or off (an extension).
func (tw *TextWindow) ScreenChangeColorByFreedom(link int, v uint32) {
	tw.ss.ColorByFreedom = !tw.ss.ColorByFreedom
	tw.ss.GW.Invalidate(false)
}

// ScreenChangeShaded turns the shading by appearances on or off
// (roadmap 19.1).
func (tw *TextWindow) ScreenChangeShaded(link int, v uint32) {
	tw.ss.Shaded = !tw.ss.Shaded
	tw.ss.GW.Invalidate(false)
}

func (tw *TextWindow) ScreenChangeAutomaticLineConstraints(link int, v uint32) {
	tw.ss.AutomaticLineConstraints = !tw.ss.AutomaticLineConstraints
	tw.ss.GW.Invalidate(false)
}

func (tw *TextWindow) ScreenChangeShadedTriangles(link int, v uint32) {
	tw.ss.ExportShadedTriangles = !tw.ss.ExportShadedTriangles
	tw.ss.GW.Invalidate(false)
}

func (tw *TextWindow) ScreenChangePwlCurves(link int, v uint32) {
	tw.ss.ExportPwlCurves = !tw.ss.ExportPwlCurves
	tw.ss.GW.Invalidate(false)
}

func (tw *TextWindow) ScreenChangeCanvasSizeAuto(link int, v uint32) {
	tw.ss.ExportCanvasSizeAuto = link == 't'
	tw.ss.GW.Invalidate(false)
}

func (tw *TextWindow) ScreenChangeCanvasSize(link int, v uint32) {
	ss := tw.ss
	var d float64
	switch v {
	case 0:
		d = ss.ExportMargin.Left
	case 1:
		d = ss.ExportMargin.Right
	case 2:
		d = ss.ExportMargin.Bottom
	case 3:
		d = ss.ExportMargin.Top

	case 10:
		d = ss.ExportCanvas.Width
	case 11:
		d = ss.ExportCanvas.Height
	case 12:
		d = ss.ExportCanvas.Dx
	case 13:
		d = ss.ExportCanvas.Dy

	default:
		return
	}
	col := 13
	if v < 10 {
		col = 11
	}
	tw.ShowEditControl(col, ss.MmToString(d, true))
	tw.edit.meaning = EditCanvasSize
	tw.edit.i = int(v)
}

func (tw *TextWindow) ScreenChangeGCodeParameter(link int, v uint32) {
	ss := tw.ss
	var buf string
	switch link {
	case 'd':
		tw.edit.meaning = EditGCodeDepth
		buf += ss.MmToString(ss.GCode.Depth, true)

	case 'h':
		tw.edit.meaning = EditGCodeSafeHeight
		buf += ss.MmToString(ss.GCode.SafeHeight, true)

	case 's':
		tw.edit.meaning = EditGCodePasses
		buf += strconv.Itoa(ss.GCode.Passes)

	case 'F':
		tw.edit.meaning = EditGCodeFeed
		buf += ss.MmToString(ss.GCode.Feed, true)

	case 'P':
		tw.edit.meaning = EditGCodePlungeFeed
		buf += ss.MmToString(ss.GCode.PlungeFeed, true)
	}
	tw.ShowEditControl(14, buf)
}

func (tw *TextWindow) ScreenChangeAutosaveInterval(link int, v uint32) {
	tw.ShowEditControl(3, strconv.Itoa(tw.ss.AutosaveInterval))
	tw.edit.meaning = EditAutosaveInterval
}

func (tw *TextWindow) ScreenChangeFindConstraintTimeout(link int, v uint32) {
	tw.ShowEditControl(3, strconv.Itoa(tw.ss.TimeoutRedundantConstr))
	tw.edit.meaning = EditFindConstraintTimeout
}

func (tw *TextWindow) ScreenChangeAnimationSpeed(link int, v uint32) {
	tw.ShowEditControl(3, strconv.Itoa(tw.ss.AnimationSpeed))
	tw.edit.meaning = EditAnimationSpeed
}

// ShowConfiguration is the configuration screen.
func (tw *TextWindow) ShowConfiguration() {
	ss := tw.ss
	tw.Printf(true, "%Ft user color (r, g, b)")

	for i := range ModelColors {
		bg := byte('a')
		if i&1 != 0 {
			bg = 'd'
		}
		c := &ss.ModelColor[i]
		tw.Printf(false, "%Bp   #%d:  %Bz  %Bp  (%@, %@, %@) %f%D%Ll%Fl[change]%E",
			bg, i, c, bg,
			float64(c.RedF()), float64(c.GreenF()), float64(c.BlueF()),
			lf(tw.ScreenChangeColor), i)
	}

	tw.Printf(false, "")
	tw.Printf(false, "%Ft chord tolerance (in percents)%E")
	tw.Printf(false, "%Ba   %@ %% %Fl%Ll%f%D[change]%E; %@ mm, %d triangles",
		ss.ChordTol, lf(tw.ScreenChangeChordTolerance), 0, ss.ChordTolMm(),
		len(ss.SK.Group(ss.GW.activeGroupH()).DisplayMesh.Triangles))
	tw.Printf(false, "%Ft max piecewise linear segments%E")
	tw.Printf(false, "%Ba   %d %Fl%Ll%f[change]%E",
		ss.MaxSegments, lf(tw.ScreenChangeMaxSegments))

	tw.Printf(false, "")
	tw.Printf(false, "%Ft export chord tolerance (in mm)%E")
	tw.Printf(false, "%Ba   %@ %Fl%Ll%f%D[change]%E",
		ss.ExportChordTol, lf(tw.ScreenChangeExportChordTolerance), 0)
	tw.Printf(false, "%Ft export max piecewise linear segments%E")
	tw.Printf(false, "%Ba   %d %Fl%Ll%f[change]%E",
		ss.ExportMaxSegments, lf(tw.ScreenChangeExportMaxSegments))

	tw.Printf(false, "%Ft snap grid spacing%E")
	tw.Printf(false, "%Ba   %s %Fl%Ll%f%D[change]%E",
		ss.MmToString(ss.GridSpacing, false), lf(tw.ScreenChangeGridSpacing), 0)

	tw.Printf(false, "")
	tw.Printf(false, "%Ft digits after decimal point to show%E")
	tw.Printf(false, "%Ba%Ft   distances: %Fd%d %Fl%Ll%f%D[change]%E (e.g. '%s')",
		ss.UnitDigitsAfterDecimal(), lf(tw.ScreenChangeDigitsAfterDecimal), 0,
		ss.MmToString(ss.StringToMm("1.23456789"), false))
	tw.Printf(false, "%Bd%Ft   angles:    %Fd%d %Fl%Ll%f%D[change]%E (e.g. '%s')",
		ss.AfterDecimalDegree, lf(tw.ScreenChangeDigitsAfterDecimalDegree), 0,
		ss.DegreeToString(1.23456789))
	tw.Printf(false, "  %Fd%f%Ll%s  use SI prefixes for distances%E",
		lf(tw.ScreenChangeUseSIPrefixes), checkMark(ss.UseSIPrefixes))

	tw.Printf(false, "")
	tw.Printf(false, "%Ft export scale factor (1:1=mm, 1:25.4=inch)")
	tw.Printf(false, "%Ba   1:%# %Fl%Ll%f%D[change]%E",
		ss.ExportScale, lf(tw.ScreenChangeExportScale), 0)
	tw.Printf(false, "%Ft cutter radius offset (0=no offset) ")
	tw.Printf(false, "%Ba   %s %Fl%Ll%f%D[change]%E",
		ss.MmToString(ss.ExportOffset, false), lf(tw.ScreenChangeExportOffset), 0)

	tw.Printf(false, "")
	tw.Printf(false, "  %Fd%f%Ll%s  export shaded 2d triangles%E",
		lf(tw.ScreenChangeShadedTriangles), checkMark(ss.ExportShadedTriangles))
	if math.Abs(ss.ExportOffset) > core.LengthEps {
		tw.Printf(false, "  %Fd%s  curves as piecewise linear%E "+
			"(since cutter radius is not zero)", checkTrue)
	} else {
		tw.Printf(false, "  %Fd%f%Ll%s  export curves as piecewise linear%E",
			lf(tw.ScreenChangePwlCurves), checkMark(ss.ExportPwlCurves))
	}
	tw.Printf(false, "  %Fd%f%Ll%s  fix white exported lines%E",
		lf(tw.ScreenChangeFixExportColors), checkMark(ss.FixExportColors))
	tw.Printf(false, "  %Fd%f%Ll%s  export background color%E",
		lf(tw.ScreenChangeExportBackgroundColor), checkMark(ss.ExportBackgroundColor))

	tw.Printf(false, "")
	tw.Printf(false, "%Ft export canvas size:  "+
		"%f%Fd%Lf%s fixed%E  "+
		"%f%Fd%Lt%s auto%E",
		lf(tw.ScreenChangeCanvasSizeAuto), radioMark(!ss.ExportCanvasSizeAuto),
		lf(tw.ScreenChangeCanvasSizeAuto), radioMark(ss.ExportCanvasSizeAuto))

	if ss.ExportCanvasSizeAuto {
		tw.Printf(false, "%Ft (by margins around exported geometry)")
		tw.Printf(false, "%Ba%Ft   left:   %Fd%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(ss.ExportMargin.Left, false), lf(tw.ScreenChangeCanvasSize), 0)
		tw.Printf(false, "%Bd%Ft   right:  %Fd%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(ss.ExportMargin.Right, false), lf(tw.ScreenChangeCanvasSize), 1)
		tw.Printf(false, "%Ba%Ft   bottom: %Fd%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(ss.ExportMargin.Bottom, false), lf(tw.ScreenChangeCanvasSize), 2)
		tw.Printf(false, "%Bd%Ft   top:    %Fd%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(ss.ExportMargin.Top, false), lf(tw.ScreenChangeCanvasSize), 3)
	} else {
		tw.Printf(false, "%Ft (by absolute dimensions and offsets)")
		tw.Printf(false, "%Ba%Ft   width:    %Fd%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(ss.ExportCanvas.Width, false), lf(tw.ScreenChangeCanvasSize), 10)
		tw.Printf(false, "%Bd%Ft   height:   %Fd%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(ss.ExportCanvas.Height, false), lf(tw.ScreenChangeCanvasSize), 11)
		tw.Printf(false, "%Ba%Ft   offset x: %Fd%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(ss.ExportCanvas.Dx, false), lf(tw.ScreenChangeCanvasSize), 12)
		tw.Printf(false, "%Bd%Ft   offset y: %Fd%s %Fl%Ll%f%D[change]%E",
			ss.MmToString(ss.ExportCanvas.Dy, false), lf(tw.ScreenChangeCanvasSize), 13)
	}

	tw.Printf(false, "")
	tw.Printf(false, "%Ft exported g code parameters")
	tw.Printf(false, "%Ba%Ft   depth:     %Fd%s %Fl%Ld%f[change]%E",
		ss.MmToString(ss.GCode.Depth, false), lf(tw.ScreenChangeGCodeParameter))
	tw.Printf(false, "%Bd%Ft   passes:    %Fd%d %Fl%Ls%f[change]%E",
		ss.GCode.Passes, lf(tw.ScreenChangeGCodeParameter))
	tw.Printf(false, "%Ba%Ft   feed:      %Fd%s %Fl%LF%f[change]%E",
		ss.MmToString(ss.GCode.Feed, false), lf(tw.ScreenChangeGCodeParameter))
	tw.Printf(false, "%Bd%Ft   plunge fd: %Fd%s %Fl%LP%f[change]%E",
		ss.MmToString(ss.GCode.PlungeFeed, false), lf(tw.ScreenChangeGCodeParameter))

	tw.Printf(false, "")
	tw.Printf(false, "  %Fd%f%Ll%s  draw triangle back faces in red%E",
		lf(tw.ScreenChangeBackFaces), checkMark(ss.DrawBackFaces))
	tw.Printf(false, "  %Fd%f%Ll%s  check sketch for closed contour%E",
		lf(tw.ScreenChangeCheckClosedContour), checkMark(ss.CheckClosedContour))
	tw.Printf(false, "  %Fd%f%Ll%s  show areas of closed contours%E",
		lf(tw.ScreenChangeShowContourAreas), checkMark(ss.ShowContourAreas))
	tw.Printf(false, "  %Fd%f%Ll%s  enable automatic line constraints%E",
		lf(tw.ScreenChangeAutomaticLineConstraints), checkMark(ss.AutomaticLineConstraints))
	tw.Printf(false, "  %Fd%f%Ll%s  colour entities by constraint state%E",
		lf(tw.ScreenChangeColorByFreedom), checkMark(ss.ColorByFreedom))
	tw.Printf(false, "  %Fd%f%Ll%s  shade solids by their appearance%E",
		lf(tw.ScreenChangeShaded), checkMark(ss.Shaded))
	tw.Printf(false, "  %Fd%f%Ll%s  use camera mouse navigation%E",
		lf(tw.ScreenChangeCameraNav), checkMark(ss.CameraNav))
	tw.Printf(false, "  %Fd%f%Ll%s  use turntable mouse navigation%E",
		lf(tw.ScreenChangeTurntableNav), checkMark(ss.TurntableNav))
	tw.Printf(false, "  %Fd%f%Ll%s  edit newly added dimensions%E",
		lf(tw.ScreenChangeImmediatelyEditDimension), checkMark(ss.ImmediatelyEditDimension))
	tw.Printf(false, "  %Fd%f%Ll%s  arc default is diameter%E",
		lf(tw.ScreenChangeArcDimDefault), checkMark(ss.ArcDimDefaultDiameter))
	tw.Printf(false, "  %Fd%f%Ll%s  display the full path in the title bar%E",
		lf(tw.ScreenChangeShowFullFilePath), checkMark(ss.ShowFullFilePath))
	tw.Printf(false, "  %Fd%f%Ll%s  draw with OpenGL%E",
		lf(tw.ScreenChangeUseOpenGL), checkMark(ss.UseGL()))
	tw.Printf(false, "  %Fd%f%Ll%s  ask before an AI program's script runs%E",
		lf(tw.ScreenChangeAgentConfirm), checkMark(ss.AgentConfirm))
	tw.Printf(false, "")
	tw.Printf(false, "%Ft autosave interval (in minutes)%E")
	tw.Printf(false, "%Ba   %d %Fl%Ll%f[change]%E",
		ss.AutosaveInterval, lf(tw.ScreenChangeAutosaveInterval))
	tw.Printf(false, "")
	tw.Printf(false, "%Ft redundant constraint timeout (in ms)%E")
	tw.Printf(false, "%Ba   %d %Fl%Ll%f[change]%E",
		ss.TimeoutRedundantConstr, lf(tw.ScreenChangeFindConstraintTimeout))
	tw.Printf(false, "")
	tw.Printf(false, "%Ft animation speed (in ms; 0 to disable)%E")
	tw.Printf(false, "%Ba   %d %Fl%Ll%f[change]%E",
		ss.AnimationSpeed, lf(tw.ScreenChangeAnimationSpeed))

	if tw.canvas != nil {
		vendor, renderer, version := tw.canvas.Ident()
		tw.Printf(false, "")
		tw.Printf(false, " %Ftgl vendor   %E%s", vendor)
		tw.Printf(false, " %Ft   renderer %E%s", renderer)
		tw.Printf(false, " %Ft   version  %E%s", version)
	}
}

// The clamps below are std::min and std::max (geom.Min/Max), which keep
// the bound when the value is NaN, where Go's min and max give NaN.

// EditControlDoneForConfiguration takes an edited configuration value,
// and reports whether the edit was one of the configuration's.
func (tw *TextWindow) EditControlDoneForConfiguration(s string) bool {
	ss := tw.ss
	switch tw.edit.meaning {
	case EditLightIntensity:
		ss.LightIntensity[tw.edit.i] = geom.Min(1.0, geom.Max(0.0, atof(s)))
		ss.GW.Invalidate(false)
	case EditLightAmbient:
		ss.AmbientIntensity = geom.Min(1.0, geom.Max(0.0, atof(s)))
		ss.GW.Invalidate(false)
	case EditLightDirection:
		if x, y, z, ok := scanVector3(s); ok {
			ss.LightDir[tw.edit.i] = vec3(x, y, z)
			ss.GW.Invalidate(false)
		} else {
			ss.Error("Bad format: specify coordinates as x, y, z")
		}
	case EditColor:
		if x, y, z, ok := scanVector3(s); ok {
			ss.ModelColor[tw.edit.i] = rgbfVec(vec3(x, y, z).ClampWithin(0, 1))
		} else {
			ss.Error("Bad format: specify color as r, g, b")
		}
	case EditChordTolerance:
		if tw.edit.i == 0 {
			ss.ChordTol = geom.Max(0.0, atof(s))
			ss.GenerateAll(sketch.GenerateAll)
		} else {
			ss.ExportChordTol = geom.Max(0.0, atof(s))
		}
	case EditMaxSegments:
		if tw.edit.i == 0 {
			ss.MaxSegments = min(1000, max(7, atoi(s)))
			ss.GenerateAll(sketch.GenerateAll)
		} else {
			ss.ExportMaxSegments = min(1000, max(7, atoi(s)))
		}
	case EditCameraTangent:
		ss.CameraTangentSetting = geom.Min(2.0, geom.Max(0.0, atof(s))) / 1000.0
		ss.GW.Invalidate(false)
		if !ss.UsePerspectiveProj {
			ss.Message("The perspective factor will have no effect until you " +
				"enable View -> Use Perspective Projection.")
		}
	case EditGridSpacing:
		ss.GridSpacing = toF32(geom.Min(1e4, geom.Max(1e-3, ss.StringToMm(s))))
		ss.GW.Invalidate(false)
	case EditExplodeDistance:
		ss.ExplodeDistance = geom.Min(1e4, geom.Max(-1e4, ss.StringToMm(s)))
		ss.SK.MarkGroupDirty(ss.GW.activeGroupH(), true)
	case EditDigitsAfterDecimal:
		v := atoi(s)
		if v < 0 || v > 8 {
			ss.Error("Specify between 0 and %d digits after the decimal.", 8)
		} else {
			ss.SetUnitDigitsAfterDecimal(v)
			ss.GW.Invalidate(false)
		}
	case EditDigitsAfterDecimalDegree:
		v := atoi(s)
		if v < 0 || v > 4 {
			ss.Error("Specify between 0 and %d digits after the decimal.", 4)
		} else {
			ss.AfterDecimalDegree = v
			ss.GW.Invalidate(false)
		}
	case EditExportScale:
		if e := ss.exprFromString(s); e != nil {
			ev := ss.evalExpr(e)
			if math.Abs(ev) < 0.001 || isUnreasonable(ev) {
				ss.Error("Export scale must not be zero!")
			} else {
				ss.ExportScale = toF32(ev)
			}
		}
	case EditExportOffset:
		if e := ss.exprFromString(s); e != nil {
			ev := ss.ExprToMm(e)
			if isUnreasonable(ev) || ev < 0 {
				ss.Error("Cutter radius offset must not be negative!")
			} else {
				ss.ExportOffset = toF32(ev)
			}
		}
	case EditCanvasSize:
		e := ss.exprFromString(s)
		if e == nil {
			break
		}
		d := toF32(ss.ExprToMm(e))
		switch tw.edit.i {
		case 0:
			ss.ExportMargin.Left = d
		case 1:
			ss.ExportMargin.Right = d
		case 2:
			ss.ExportMargin.Bottom = d
		case 3:
			ss.ExportMargin.Top = d

		case 10:
			ss.ExportCanvas.Width = d
		case 11:
			ss.ExportCanvas.Height = d
		case 12:
			ss.ExportCanvas.Dx = d
		case 13:
			ss.ExportCanvas.Dy = d
		}
	case EditGCodeDepth:
		if e := ss.exprFromString(s); e != nil {
			ss.GCode.Depth = toF32(ss.ExprToMm(e))
		}
	case EditGCodeSafeHeight:
		if e := ss.exprFromString(s); e != nil {
			ss.GCode.SafeHeight = toF32(ss.ExprToMm(e))
		}
	case EditGCodePasses:
		if e := ss.exprFromString(s); e != nil {
			ss.GCode.Passes = int(ss.evalExpr(e))
		}
		ss.GCode.Passes = max(1, min(1000, ss.GCode.Passes))
	case EditGCodeFeed:
		if e := ss.exprFromString(s); e != nil {
			ss.GCode.Feed = toF32(ss.ExprToMm(e))
		}
	case EditGCodePlungeFeed:
		if e := ss.exprFromString(s); e != nil {
			ss.GCode.PlungeFeed = toF32(ss.ExprToMm(e))
		}
	case EditAutosaveInterval:
		interval := atoi(s)
		if interval != 0 {
			if interval >= 1 {
				ss.AutosaveInterval = interval
				ss.ScheduleAutosave()
			} else {
				ss.Error("Bad value: autosave interval should be positive")
			}
		} else {
			ss.Error("Bad format: specify interval in integral minutes")
		}
	case EditFindConstraintTimeout:
		timeout := atoi(s)
		if timeout != 0 {
			if timeout >= 1 {
				ss.TimeoutRedundantConstr = timeout
			} else {
				ss.TimeoutRedundantConstr = 1000
			}
		}
	case EditAnimationSpeed:
		speed := atoi(s)
		if speed >= 0 {
			ss.AnimationSpeed = speed
		} else {
			ss.AnimationSpeed = 800
		}

	default:
		return false
	}
	return true
}

// isUnreasonable is true for NaN and huge values.
func isUnreasonable(x float64) bool {
	return math.IsNaN(x) || x > 1e11 || x < -1e11
}

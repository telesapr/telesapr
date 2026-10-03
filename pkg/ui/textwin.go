package ui

// The text-based browser window (C++ textwin.cpp and the TextWindow
// part of ui.h).

import (
	"fmt"
	"math"
	"reflect"
	"unicode/utf8"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
)

// The text window's size limits.
const (
	MaxCols = 100
	MinCols = 45
	MaxRows = 4000
)

// The text window's character grid, in pixels.
const (
	charWidth  = 9
	charHeight = 16
	lineHeight = 20
	leftMargin = 6
)

// The check box and radio button glyphs of the private font range.
const (
	checkFalse = ""
	checkTrue  = ""
	radioFalse = ""
	radioTrue  = ""
)

// LinkFunction runs when a link is clicked or hovered (C++
// TextWindow::LinkFunction).
type LinkFunction func(link int, v uint32)

// notALink is the link code of plain text.
const notALink = 0

// Screen is a text window screen (C++ TextWindow::Screen).
type Screen int

const (
	ScreenListOfGroups Screen = iota
	ScreenGroupInfo
	ScreenGroupSolveInfo
	ScreenConfiguration
	ScreenStepDimension
	ScreenListOfStyles
	ScreenStyleInfo
	ScreenPasteTransformed
	ScreenEditView
	ScreenTangentArc
	// Extensions: screens C++ SolveSpace doesn't have.
	ScreenThread
	ScreenParameters
	ScreenCommands
	ScreenBrokenReferences
	ScreenSection
	ScreenBillOfMaterials
	ScreenMotionStudy
	ScreenDrawing
)

// Edit is what the edit control is editing (C++ TextWindow::Edit).
type Edit int

const (
	EditNothing Edit = 0
	// For multiple groups
	EditTimesRepeated Edit = 1
	EditGroupName     Edit = 2
	EditGroupScale    Edit = 3
	EditGroupColor    Edit = 4
	EditGroupOpacity  Edit = 5
	// For the configuration screen
	EditLightDirection           Edit = 100
	EditLightIntensity           Edit = 101
	EditColor                    Edit = 102
	EditChordTolerance           Edit = 103
	EditMaxSegments              Edit = 104
	EditCameraTangent            Edit = 105
	EditGridSpacing              Edit = 106
	EditDigitsAfterDecimal       Edit = 107
	EditDigitsAfterDecimalDegree Edit = 108
	EditExportScale              Edit = 109
	EditExportOffset             Edit = 110
	EditCanvasSize               Edit = 111
	EditGCodeDepth               Edit = 112
	EditGCodeSafeHeight          Edit = 113
	EditGCodePasses              Edit = 114
	EditGCodeFeed                Edit = 115
	EditGCodePlungeFeed          Edit = 116
	EditAutosaveInterval         Edit = 117
	EditLightAmbient             Edit = 118
	EditFindConstraintTimeout    Edit = 119
	EditExplodeDistance          Edit = 120
	EditAnimationSpeed           Edit = 121
	// For TTF text
	EditTTFText Edit = 300
	// For the step dimension screen
	EditStepDimFinish Edit = 400
	EditStepDimSteps  Edit = 401
	// For the styles stuff
	EditStyleWidth         Edit = 500
	EditStyleTextHeight    Edit = 501
	EditStyleTextAngle     Edit = 502
	EditStyleColor         Edit = 503
	EditStyleFillColor     Edit = 504
	EditStyleName          Edit = 505
	EditBackgroundColor    Edit = 506
	EditStyleStipplePeriod Edit = 508
	// For paste transforming
	EditPasteTimesRepeated Edit = 600
	EditPasteAngle         Edit = 601
	EditPasteScale         Edit = 602
	// For view
	EditViewScale     Edit = 700
	EditViewOrigin    Edit = 701
	EditViewProjRight Edit = 702
	EditViewProjUp    Edit = 703
	// For tangent arc
	EditTangentArcRadius Edit = 800
	// For helix pitch
	EditHelixPitch Edit = 802
	// For a fillet's edge's own radius (an extension)
	EditEdgeRadius Edit = 803
	// For an extrusion's draft (an extension)
	EditGroupDraft Edit = 804
	// For the screw thread (an extension)
	EditThreadDesignation Edit = 900
	EditThreadLength      Edit = 901
	EditThreadClearance   Edit = 902
	// For the parameter table (an extension)
	EditParameterAdd  Edit = 910
	EditParameterExpr Edit = 911
	EditParameterName Edit = 912
	// For configurations (an extension)
	EditConfigurationAdd  Edit = 913
	EditConfigurationName Edit = 914
	// For the motion study (an extension)
	EditMotionParameter Edit = 915
	EditMotionFrom      Edit = 916
	EditMotionTo        Edit = 917
	EditMotionSteps     Edit = 918
	// For the drawing (an extension)
	EditDrawingScale   Edit = 919
	EditDrawingField   Edit = 921
	EditDrawingSection Edit = 922
	EditDrawingDetail  Edit = 923
	// For the command search (an extension)
	EditCommandQuery Edit = 920
	// For the section view (an extension)
	EditSectionOffset Edit = 930
)

// drawOrHitHow is what DrawOrHitTestIcons and the colour picker do
// (C++ TextWindow::DrawOrHitHow).
type drawOrHitHow int

const (
	howPaint drawOrHitHow = iota
	howHover
	howClick
)

// shownState is the screen shown and its arguments (C++
// TextWindow::ShownState).
type shownState struct {
	screen     Screen
	group      core.HGroup
	style      core.HStyle
	constraint core.HConstraint
	paste      struct {
		times  int
		trans  geom.Vector
		theta  float64
		origin geom.Vector
		scale  float64
	}
	thread   threadScreen
	commands commandScreen
}

// cellMeta is how one character cell is drawn and what clicking it
// does (C++ TextWindow::meta).
type cellMeta struct {
	fg, bg byte
	bgRgb  core.RgbaColor
	link   int
	data   uint32
	f, h   LinkFunction
	// fID identifies f's code, standing in for C++'s comparison of
	// function pointers.
	fID uintptr
}

// textRow is a row of the text window: its top in half-rows, and its
// characters (C++ TextWindow::text, meta and top).
type textRow struct {
	top  int
	text [MaxCols]rune
	meta [MaxCols]cellMeta
}

// clear resets the row as C++ ClearScreen does.
func (row *textRow) clear() {
	for j := range MaxCols {
		row.text[j] = ' '
		row.meta[j].fg = 'd'
		row.meta[j].bg = 'd'
		row.meta[j].link = notALink
	}
}

// TextWindow is the property browser (C++ TextWindow).
type TextWindow struct {
	ss *SolveSpaceUI

	window platform.Window
	canvas render.ViewportCanvas

	scrollPos int // The scrollbar position, in half-row units
	halfRows  int // The height of our window, in half-row units

	// lines are the rows written since ClearScreen; C++'s top[] of the
	// unused rows, i*2, is lineTop.
	lines                  []textRow
	rows                   int
	hoveredRow, hoveredCol int

	hoveredButton textButton

	shown shownState

	edit struct {
		showAgain bool
		meaning   Edit
		i         int
		group     core.HGroup
		request   core.HRequest
		style     core.HStyle
	}

	editControl struct {
		halfRow int
		col     int

		colorPicker struct {
			rgb            core.RgbaColor
			h, s, v        float64
			show           bool
			picker1dActive bool
			picker2dActive bool
		}
	}

	// motion is the motion study's screen (motion.go).
	motion motionStudy

	stepDim struct {
		isDistance bool
		finish     float64
		steps      int

		timer platform.Timer
		time  int64
		step  int
	}

	// buttons are the icons at the top (C++ static buttons[]).
	buttons []textButton
}

func newTextWindow(ss *SolveSpaceUI) *TextWindow {
	tw := &TextWindow{ss: ss}
	tw.buttons = newTextButtons(ss)
	return tw
}

// The foreground and background colour codes (C++ fgColors and
// bgColors).
var (
	fgColors = []struct {
		c     byte
		color core.RgbaColor
	}{
		{'d', rgbi(255, 255, 255)}, // Default   : white
		{'l', rgbi(100, 200, 255)}, // links     : blue
		{'t', rgbi(255, 200, 100)}, // tree/text : yellow
		{'h', rgbi(90, 90, 90)},
		{'s', rgbi(40, 255, 40)}, // Ok        : green
		{'m', rgbi(200, 200, 0)},
		{'r', rgbi(0, 0, 0)},       // Reverse   : black
		{'x', rgbi(255, 20, 20)},   // Error     : red
		{'i', rgbi(0, 255, 255)},   // Info      : cyan
		{'g', rgbi(128, 128, 128)}, // Disabled  : gray
		{'b', rgbi(200, 200, 200)},
	}
	bgColors = []struct {
		c     byte
		color core.RgbaColor
	}{
		{'d', rgbi(0, 0, 0)}, // Default   : black
		{'t', rgbi(34, 15, 15)},
		{'a', rgbi(25, 25, 25)},    // Alternate : dark gray
		{'r', rgbi(255, 255, 255)}, // Reverse   : white
	}
)

// colorTables are C++'s fgColorTable and bgColorTable: the colours by
// code, as floats; codes not in the lists are black.
var fgColorTable, bgColorTable = func() (fg, bg [256][3]float32) {
	for _, c := range fgColors {
		fg[c.c] = [3]float32{c.color.RedF(), c.color.GreenF(), c.color.BlueF()}
	}
	for _, c := range bgColors {
		bg[c.c] = [3]float32{c.color.RedF(), c.color.GreenF(), c.color.BlueF()}
	}
	return
}()

func colorFromTable(t *[256][3]float32, c byte) core.RgbaColor {
	return core.RgbaColorFromFloat(t[c][0], t[c][1], t[c][2], 1)
}

// Init opens the window the first time, and resets it (C++
// TextWindow::Init).
func (tw *TextWindow) Init() {
	ss := tw.ss
	if tw.window == nil {
		tw.window = ss.Platform.CreateWindow(platform.WindowTool, ss.GW.window)
		if tw.window != nil {
			tw.canvas = tw.window.CreateRenderer(ss.UseGL())

			ev := tw.window.Events()
			ev.OnClose = func() {
				ss.TW.HideEditControl()
				ss.GW.showTextWindow = false
				ss.GW.EnsureValidActives()
			}
			ev.OnMouseEvent = func(event platform.MouseEvent) bool {
				switch event.Type {
				case platform.MousePress, platform.MouseDblPress, platform.MouseMotion:
					isClick := event.Type != platform.MouseMotion
					leftDown := event.Button == platform.ButtonLeft
					tw.MouseEvent(isClick, leftDown, event.X, event.Y)
					return true
				case platform.MouseLeave:
					tw.MouseLeave()
					return true
				case platform.MouseScrollVert:
					if event.ScrollDelta == 0 {
						return true
					}
					if math.Abs(event.ScrollDelta) < 0.2 {
						if event.ScrollDelta > 0 {
							event.ScrollDelta = 0.2
						} else {
							event.ScrollDelta = -0.2
						}
					}
					offset := lineHeight / 2 * event.ScrollDelta
					tw.ScrollbarEvent(tw.window.GetScrollbarPosition() - offset)
				}
				return false
			}
			if ss.GW.window != nil {
				ev.OnKeyboardEvent = ss.GW.window.Events().OnKeyboardEvent
			}
			ev.OnRender = tw.Paint
			ev.OnEditingDone = tw.EditControlDone
			ev.OnScrollbarAdjusted = tw.ScrollbarEvent
			tw.window.SetMinContentSize(370, 370)
		}
	}

	tw.ClearSuper()
}

// ClearSuper resets everything but the window, and shows the home
// screen (C++ TextWindow::ClearSuper).
func (tw *TextWindow) ClearSuper() {
	// C++ destroys the step dimension's timer with the rest; a step in
	// progress must not go on with the reset state.
	if t := tw.stepDim.timer; t != nil {
		t.SetOnTimeout(nil)
	}
	*tw = TextWindow{ss: tw.ss, window: tw.window, canvas: tw.canvas, buttons: tw.buttons}

	tw.HideEditControl()

	tw.ClearScreen()
	tw.Show()
}

// HideEditControl hides the edit box and the colour picker (C++
// TextWindow::HideEditControl).
func (tw *TextWindow) HideEditControl() {
	tw.editControl.colorPicker.show = false
	if tw.window != nil {
		tw.window.HideEditor()
		tw.window.Invalidate()
	}
}

// ShowEditControl shows the edit box at a column of a half-row, by
// default the hovered row (C++ TextWindow::ShowEditControl).
func (tw *TextWindow) ShowEditControl(col int, str string, halfRow ...int) {
	hr := -1
	if len(halfRow) > 0 {
		hr = halfRow[0]
	}
	if hr < 0 {
		hr = tw.lineTop(tw.hoveredRow)
	}
	tw.editControl.halfRow = hr
	tw.editControl.col = col

	x := leftMargin + charWidth*col
	y := (hr - tw.ss.TW.scrollPos) * (lineHeight / 2)

	if tw.window == nil {
		return
	}
	width, _ := tw.window.GetContentSize()
	tw.window.ShowEditor(float64(x), float64(y+lineHeight-2), lineHeight-4,
		width-float64(x), true, str)
}

// ShowEditControlWithColorPicker shows the edit box with a colour
// picker under it (C++ TextWindow::ShowEditControlWithColorPicker).
func (tw *TextWindow) ShowEditControlWithColorPicker(col int, rgb core.RgbaColor) {
	tw.ss.ScheduleShowTW()

	tw.editControl.colorPicker.show = true
	tw.editControl.colorPicker.rgb = rgb

	// Convert the current RGB color to HSV so the picker reflects it,
	// rather than always resetting to black. RgbToHsv() returns the hue
	// in [0,6), but the picker stores it normalized to [0,1], so divide
	// by 6 before storing.
	hs := rgbToHsv(geom.Vector{X: float64(rgb.RedF()), Y: float64(rgb.GreenF()), Z: float64(rgb.BlueF())})

	tw.editControl.colorPicker.h = hs.X / 6
	tw.editControl.colorPicker.s = hs.Y
	tw.editControl.colorPicker.v = hs.Z

	tw.ShowEditControl(col, sprintf("%.2f, %.2f, %.2f", rgb.RedF(), rgb.GreenF(), rgb.BlueF()))
}

// ClearScreen empties the text (C++ TextWindow::ClearScreen).
func (tw *TextWindow) ClearScreen() {
	tw.lines = tw.lines[:0]
	tw.rows = 0
}

// lineTop is C++ top[r]: a written row's top, or r*2 for one ClearScreen
// left unused.
func (tw *TextWindow) lineTop(r int) int {
	if r >= 0 && r < len(tw.lines) {
		return tw.lines[r].top
	}
	return r * 2
}

// addRow starts row r, cleared.
func (tw *TextWindow) addRow() *textRow {
	r := len(tw.lines)
	if r < cap(tw.lines) {
		tw.lines = tw.lines[:r+1]
	} else {
		tw.lines = append(tw.lines, textRow{})
	}
	row := &tw.lines[r]
	row.top = r * 2
	row.clear()
	return row
}

// endString was added when someone had too many fonts for the text
// window: scrolling seemed to be broken, but was actually at MaxRows.
const endString = "    **** End of Text Screen ****"

// funcID identifies a link function's code (C++ compares the function
// pointers).
func funcID(f LinkFunction) uintptr {
	if f == nil {
		return 0
	}
	return reflect.ValueOf(f).Pointer()
}

// intArg is a printf integer argument of any integer type.
func intArg(a any) int64 {
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return int64(v.Uint())
	case reflect.Bool:
		if v.Bool() {
			return 1
		}
		return 0
	}
	panic(fmt.Sprintf("ui: Printf: %T is not an integer", a))
}

func floatArg(a any) float64 {
	switch v := a.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	}
	panic(fmt.Sprintf("ui: Printf: %T is not a float", a))
}

func linkArg(a any) LinkFunction {
	switch f := a.(type) {
	case nil:
		return nil
	case LinkFunction:
		return f
	case func(int, uint32):
		return f
	}
	panic(fmt.Sprintf("ui: Printf: %T is not a link function", a))
}

// Printf writes a row in the text window's format language (C++
// TextWindow::Printf): %d, %x, %@, %2, %3, %#, %s, %c and %% print
// their argument; %Fx and %Bx set the colours (p takes the code as an
// argument, and z a *core.RgbaColor); %Lx starts a link (p: code as an
// argument); %f and %h take the click and hover LinkFunctions; %D the
// link's data; and %E ends the link and the foreground colour.
func (tw *TextWindow) Printf(halfLine bool, format string, args ...any) {
	if tw.canvas == nil {
		return
	}

	if tw.rows >= MaxRows {
		return
	}

	if tw.rows >= MaxRows-2 && format != endString {
		// twice due to some half-row issues on resizing
		tw.Printf(halfLine, endString)
		tw.Printf(halfLine, endString)
		return
	}

	argi := 0
	next := func() any {
		a := args[argi]
		argi++
		return a
	}

	r := tw.rows
	row := tw.addRow()
	if r == 0 {
		row.top = 0
	} else if halfLine {
		row.top = tw.lines[r-1].top + 3
	} else {
		row.top = tw.lines[r-1].top + 2
	}
	tw.rows++

	for c := range MaxCols {
		row.text[c] = ' '
		row.meta[c].link = notALink
	}

	var fg, bg byte = 'd', 'd'
	bgRgb := rgbi(0, 0, 0)
	link := notALink
	var data uint32
	var f, h LinkFunction

	font := tw.ss.GW.bitmapFont()
	c := 0
	i := 0
	for i < len(format) {
		var buf string

		if format[i] == '%' {
			i++
			if i >= len(format) {
				return
			}
			switch format[i] {
			case 'd':
				buf = sprintf("%d", intArg(next()))
			case 'x':
				buf = sprintf("%08x", uint32(intArg(next())))
			case '@':
				buf = cFixed(2, floatArg(next()))
			case '2':
				v := floatArg(next())
				sp := " "
				if v < 0 {
					sp = ""
				}
				buf = sp + cFixed(2, v)
			case '3':
				v := floatArg(next())
				sp := " "
				if v < 0 {
					sp = ""
				}
				buf = sp + cFixed(3, v)
			case '#':
				buf = cFixed(3, floatArg(next()))
			case 's':
				buf = next().(string)
			case 'c':
				// 'char' is promoted to 'int' when passed through '...'
				if v := intArg(next()); v != 0 {
					buf = string([]byte{byte(v)})
				}
			case 'E':
				fg = 'd'
				// leave the background, though
				link = notALink
				data = 0
				f = nil
				h = nil

			case 'F', 'B':
				if i+1 >= len(format) {
					return // truncated directive
				}
				cc := format[i+1] // color code
				var rgbPtr *core.RgbaColor
				switch cc {
				case 'p':
					cc = byte(intArg(next()))
				case 'z':
					rgbPtr = next().(*core.RgbaColor)
				}
				if format[i] == 'F' {
					fg = cc
				} else {
					bg = cc
					if rgbPtr != nil {
						bgRgb = *rgbPtr
					}
				}
				i++

			case 'L':
				if i+1 >= len(format) {
					return
				}
				i++
				if format[i] == 'p' {
					link = int(intArg(next()))
				} else {
					link = int(format[i])
				}

			case 'f':
				f = linkArg(next())

			case 'h':
				h = linkArg(next())

			case 'D':
				data = uint32(intArg(next()))

			case '%':
				buf = "%"
			}
		} else {
			_, size := utf8.DecodeRuneInString(format[i:])
			buf = format[i : i+size]
		}

		fid := funcID(f)
		for _, ch := range buf {
			for k := 0; k < font.GetWidth(ch); k++ {
				if c >= MaxCols {
					return
				}
				if k == 0 {
					row.text[c] = ch
				} else {
					row.text[c] = ' '
				}
				row.meta[c] = cellMeta{fg: fg, bg: bg, bgRgb: bgRgb, link: link, data: data,
					f: f, h: h, fID: fid}
				c++
			}
		}

		_, size := utf8.DecodeRuneInString(format[i:])
		i += size
	}
	for c < MaxCols {
		row.meta[c].fg = fg
		row.meta[c].bg = bg
		row.meta[c].bgRgb = bgRgb
		c++
	}
}

// Show fills the window with the screen for the state of things (C++
// TextWindow::Show).
func (tw *TextWindow) Show() {
	ss := tw.ss
	gw := ss.GW
	if gw.pending.operation == PendingNone {
		gw.ClearPending(false)
	}

	gw.GroupSelection()
	gs := &gw.gs

	// Make sure these tests agree with test used to draw indicator line
	// on main list of groups screen.
	if gw.pending.description != "" {
		// A pending operation (that must be completed with the mouse in
		// the graphics window) will preempt our usual display.
		tw.HideEditControl()
		tw.ShowHeader(false)
		tw.Printf(false, "")
		tw.Printf(false, "%s", gw.pending.description)
		tw.Printf(true, "%Fl%f%Ll(cancel operation)%E", LinkFunction(tw.ScreenUnselectAll))
	} else if (gs.n > 0 || gs.constraints > 0 || len(gs.solidEdge) > 0) && tw.shown.screen != ScreenPasteTransformed {
		if tw.edit.meaning != EditTTFText {
			tw.HideEditControl()
		}
		tw.ShowHeader(false)
		tw.DescribeSelection()
	} else {
		if tw.edit.meaning == EditTTFText {
			tw.HideEditControl()
		}
		tw.ShowHeader(true)
		switch tw.shown.screen {
		default:
			tw.shown.screen = ScreenListOfGroups
			tw.ShowListOfGroups()
		case ScreenListOfGroups:
			tw.ShowListOfGroups()
		case ScreenGroupInfo:
			tw.ShowGroupInfo()
		case ScreenGroupSolveInfo:
			tw.ShowGroupSolveInfo()
		case ScreenConfiguration:
			tw.ShowConfiguration()
		case ScreenStepDimension:
			tw.ShowStepDimension()
		case ScreenListOfStyles:
			tw.ShowListOfStyles()
		case ScreenStyleInfo:
			tw.ShowStyleInfo()
		case ScreenPasteTransformed:
			tw.ShowPasteTransformed()
		case ScreenEditView:
			tw.ShowEditView()
		case ScreenTangentArc:
			tw.ShowTangentArc()
		case ScreenThread:
			tw.ShowThread()
		case ScreenParameters:
			tw.ShowParameters()
		case ScreenCommands:
			tw.ShowCommands()
		case ScreenBrokenReferences:
			tw.ShowBrokenReferences()
		case ScreenSection:
			tw.ShowSection()
		case ScreenBillOfMaterials:
			tw.ShowBillOfMaterials()
		case ScreenMotionStudy:
			tw.ShowMotionStudy()
		case ScreenDrawing:
			tw.ShowDrawing()
		}
	}
	tw.Printf(false, "")

	// Make sure there's room for the color picker
	if tw.editControl.colorPicker.show {
		pickerHeight := 25
		halfRow := tw.editControl.halfRow
		if tw.lineTop(tw.rows-1)-halfRow < pickerHeight && tw.rows < MaxRows {
			tw.rows++
			row := tw.addRow()
			row.top = halfRow + pickerHeight
		}
	}

	if tw.window != nil {
		tw.Resize()
	}
}

// Resize fits the scrollbar to the window (C++ TextWindow::Resize).
func (tw *TextWindow) Resize() {
	_, height := tw.window.GetContentSize()

	tw.halfRows = int(height) / (lineHeight / 2)

	bottom := tw.lineTop(tw.rows-1) + 2
	tw.scrollPos = min(tw.scrollPos, bottom-tw.halfRows)
	tw.scrollPos = max(tw.scrollPos, 0)

	tw.window.ConfigureScrollbar(0, float64(tw.lineTop(tw.rows-1)+1), float64(tw.halfRows))
	tw.window.SetScrollbarPosition(float64(tw.scrollPos))
	tw.window.SetScrollbarVisible(tw.lineTop(tw.rows-1)+1 > tw.halfRows)
	tw.window.Invalidate()
}

// DrawOrHitTestIcons draws the icons at the top, or finds the one under
// the mouse (C++ TextWindow::DrawOrHitTestIcons).
func (tw *TextWindow) DrawOrHitTestIcons(uiCanvas *render.UiCanvas, how drawOrHitHow, mx, my float64) {
	width, _ := tw.window.GetContentSize()

	x, y := 20, 33+lineHeight
	y -= tw.scrollPos * (lineHeight / 2)

	if how == howPaint {
		top, bot := y-28, y+4
		uiCanvas.DrawRect(0, int(width), top, bot, core.NewRgbaColor(30, 30, 30, 255), core.RgbaColor{}, 0)
	}

	oldHovered := tw.hoveredButton
	if how != howPaint {
		tw.hoveredButton = nil
	}

	var hoveredX, hoveredY float64
	for _, button := range tw.buttons {
		if how == howPaint {
			button.Draw(uiCanvas, x, y, button == tw.hoveredButton)
		} else if mx > float64(x-2) && mx < float64(x+26) && my < float64(y+2) && my > float64(y-26) {
			tw.hoveredButton = button
			hoveredX = float64(x - 2)
			hoveredY = float64(y - 26)
			if how == howClick {
				button.Click()
			}
		}

		x += button.AdvanceWidth()
	}

	if how != howPaint && tw.hoveredButton != oldHovered {
		if tw.hoveredButton == nil {
			tw.window.SetTooltip("", 0, 0, 0, 0)
		} else {
			tw.window.SetTooltip(tw.hoveredButton.Tooltip(), hoveredX, hoveredY, 28, 28)
		}
		tw.window.Invalidate()
	}
}

// hsvToRgb takes (h, s, v) in [0,6), [0,1], [0,1] to (r, g, b) all in
// [0, 1] (C++ TextWindow::HsvToRgb).
func hsvToRgb(hsv geom.Vector) geom.Vector {
	if hsv.X >= 6 {
		hsv.X -= 6
	}

	var rgb geom.Vector
	hmod2 := hsv.X
	for hmod2 >= 2 {
		hmod2 -= 2
	}
	x := 1 - math.Abs(hmod2-1)
	switch {
	case hsv.X < 1:
		rgb = geom.Vector{X: 1, Y: x, Z: 0}
	case hsv.X < 2:
		rgb = geom.Vector{X: x, Y: 1, Z: 0}
	case hsv.X < 3:
		rgb = geom.Vector{X: 0, Y: 1, Z: x}
	case hsv.X < 4:
		rgb = geom.Vector{X: 0, Y: x, Z: 1}
	case hsv.X < 5:
		rgb = geom.Vector{X: x, Y: 0, Z: 1}
	default:
		rgb = geom.Vector{X: 1, Y: 0, Z: x}
	}
	c := hsv.Y * hsv.Z
	m := 1 - hsv.Z
	rgb = rgb.ScaledBy(c)
	rgb = rgb.Plus(geom.Vector{X: m, Y: m, Z: m})

	return rgb
}

// rgbToHsv takes (r, g, b) in [0,1) to (h, s, v) in [0,6], [0,1],
// [0,1] (C++ TextWindow::RgbToHsv).
func rgbToHsv(rgb geom.Vector) geom.Vector {
	r, g, b := rgb.X, rgb.Y, rgb.Z

	maxc := math.Max(r, math.Max(g, b))
	minc := math.Min(r, math.Min(g, b))
	delta := maxc - minc

	h, s := 0.0, 0.0
	v := 1 - minc

	if delta != 0 && v != 0 {
		s = delta / v

		switch maxc {
		case r:
			h = (g - b) / delta
		case g:
			h = 2 + (b-r)/delta
		default:
			h = 4 + (r-g)/delta
		}

		if h < 0 {
			h += 6
		}
	}

	return geom.Vector{X: h, Y: s, Z: v}
}

func hsvPattern2d(w, h int) *res.Pixmap {
	pixmap := res.NewPixmap(res.FormatRGB, w, h)
	for j := 0; j < pixmap.Height; j++ {
		p := pixmap.Stride * j
		for i := 0; i < pixmap.Width; i++ {
			hsv := geom.Vector{X: 6.0 * float64(i) / float64(pixmap.Width-1),
				Y: 1.0 * float64(j) / float64(pixmap.Height-1), Z: 1}
			rgb := hsvToRgb(hsv).ScaledBy(255)
			pixmap.Data[p] = uint8(rgb.X)
			pixmap.Data[p+1] = uint8(rgb.Y)
			pixmap.Data[p+2] = uint8(rgb.Z)
			p += 3
		}
	}
	return pixmap
}

func hsvPattern1d(hue, sat float64, w, h int) *res.Pixmap {
	pixmap := res.NewPixmap(res.FormatRGB, w, h)
	for i := 0; i < pixmap.Height; i++ {
		p := i * pixmap.Stride
		for j := 0; j < pixmap.Width; j++ {
			hsv := geom.Vector{X: 6 * hue, Y: sat,
				Z: 1.0 * float64(pixmap.Width-1-j) / float64(pixmap.Width)}
			rgb := hsvToRgb(hsv).ScaledBy(255)
			pixmap.Data[p] = uint8(rgb.X)
			pixmap.Data[p+1] = uint8(rgb.Y)
			pixmap.Data[p+2] = uint8(rgb.Z)
			p += 3
		}
	}
	return pixmap
}

// ColorPickerDone enters the picked colour (C++
// TextWindow::ColorPickerDone).
func (tw *TextWindow) ColorPickerDone() {
	rgb := tw.editControl.colorPicker.rgb
	tw.EditControlDone(sprintf("%.2f, %.2f, %.3f", rgb.RedF(), rgb.GreenF(), rgb.BlueF()))
}

var pickerBaseColor = [12]core.RgbaColor{
	rgbi(255, 0, 0),
	rgbi(0, 255, 0),
	rgbi(0, 0, 255),

	rgbi(0, 255, 255),
	rgbi(255, 0, 255),
	rgbi(255, 255, 0),

	rgbi(255, 127, 0),
	rgbi(255, 0, 127),
	rgbi(0, 255, 127),
	rgbi(127, 255, 0),
	rgbi(127, 0, 255),
	rgbi(0, 127, 255),
}

func rgbfVec(rgb geom.Vector) core.RgbaColor {
	return core.RgbaColorFromFloat(float32(rgb.X), float32(rgb.Y), float32(rgb.Z), 1)
}

// DrawOrHitTestColorPicker draws the colour picker, or handles the
// mouse on it; it reports whether the picker is shown and has the
// mouse (C++ TextWindow::DrawOrHitTestColorPicker).
func (tw *TextWindow) DrawOrHitTestColorPicker(uiCanvas *render.UiCanvas, how drawOrHitHow,
	leftDown bool, x, y float64) bool {
	cp := &tw.editControl.colorPicker
	mousePointerAsHand := false

	if how == howHover && !leftDown {
		cp.picker1dActive = false
		cp.picker2dActive = false
	}

	if !cp.show {
		return false
	}
	if how == howClick || (how == howHover && leftDown) {
		tw.window.Invalidate()
	}

	width, _ := tw.window.GetContentSize()

	px := leftMargin + charWidth*tw.editControl.col
	py := (tw.editControl.halfRow - tw.ss.TW.scrollPos) * (lineHeight / 2)

	py += lineHeight + 5

	const pWidth, pHeight = 16, 12
	const pitch, size = 18, 15

	px = min(px, int(width)-(pWidth*pitch+40))

	pxm := px + pWidth*pitch + 11
	pym := py + pHeight*pitch + 7

	bw := 6
	if how == howPaint {
		uiCanvas.DrawRect(px, pxm+bw, py, pym+bw, core.NewRgbaColor(50, 50, 50, 255), core.RgbaColor{}, 1)
		uiCanvas.DrawRect(px+(bw/2), pxm+(bw/2), py+(bw/2), pym+(bw/2),
			core.NewRgbaColor(0, 0, 0, 255), core.RgbaColor{}, 1)
	} else if x < float64(px) || x > float64(pxm+(bw/2)) || y < float64(py) || y > float64(pym+(bw/2)) {
		return false
	}
	px += bw / 2
	py += bw / 2

	in := func(x0, y0, x1, y1 int) bool {
		return x >= float64(x0) && x <= float64(x1) && y >= float64(y0) && y <= float64(y1)
	}

	for i := 0; i < pWidth/2; i++ {
		for j := 0; j < pHeight; j++ {
			var rgb geom.Vector
			switch {
			case i == 0 && j < 8:
				d := tw.ss.ModelColor[j]
				rgb = geom.Vector{X: float64(d.RedF()), Y: float64(d.GreenF()), Z: float64(d.BlueF())}
			case i == 0:
				a := (float64(j) - 8.0) / 3.0
				rgb = geom.Vector{X: a, Y: a, Z: a}
			default:
				d := pickerBaseColor[j]
				rgb = geom.Vector{X: float64(d.RedF()), Y: float64(d.GreenF()), Z: float64(d.BlueF())}
				if i >= 2 && i <= 4 {
					a := 0.4
					if i == 2 {
						a = 0.2
					} else if i == 3 {
						a = 0.3
					}
					rgb = rgb.Plus(geom.Vector{X: a, Y: a, Z: a})
				}
				if i >= 5 && i <= 7 {
					a := 0.18
					if i == 5 {
						a = 0.7
					} else if i == 6 {
						a = 0.4
					}
					rgb = rgb.ScaledBy(a)
				}
			}

			rgb = rgb.ClampWithin(0, 1)
			sx, sy := px+5+pitch*(i+8)+4, py+5+pitch*j

			switch how {
			case howPaint:
				uiCanvas.DrawRect(sx, sx+size, sy, sy+size, rgbfVec(rgb), core.RgbaColor{}, 2)
			case howClick:
				if in(sx, sy, sx+size, sy+size) {
					cp.rgb = rgbfVec(rgb)
					tw.ColorPickerDone()
				}
			case howHover:
				if in(sx, sy, sx+size, sy+size) {
					mousePointerAsHand = true
				}
			}
		}
	}

	hx, hy := px+5, py+5
	hxm := hx + pitch*7 + size
	hym := hy + pitch*2 + size
	switch how {
	case howPaint:
		uiCanvas.DrawRect(hx, hxm, hy, hym, cp.rgb, core.RgbaColor{}, 2)
	case howClick:
		if in(hx, hy, hxm, hym) {
			tw.ColorPickerDone()
		}
	case howHover:
		if in(hx, hy, hxm, hym) {
			mousePointerAsHand = true
		}
	}

	hy += pitch * 3

	hxm = hx + pitch*7 + size
	hym = hy + pitch*1 + size
	// The one-dimensional thing to pick the color's value
	if how == howPaint {
		uiCanvas.DrawPixmap(hsvPattern1d(cp.h, cp.s, hxm-hx, hym-hy), hx, hy, 2)

		cx := hx + int(float64(hxm-hx)*(1.0-cp.v))
		// C++ passes the fill and outline colours of DrawRect to
		// DrawLine: the outline becomes a width of 0.
		uiCanvas.DrawLine(cx, hy, cx, hym, core.NewRgbaColor(0, 0, 0, 255), 0, 3)
	} else if how == howClick || (how == howHover && leftDown && cp.picker1dActive) {
		if in(hx, hy, hxm, hym) {
			cp.v = 1 - (x-float64(hx))/float64(hxm-hx)

			rgb := hsvToRgb(geom.Vector{X: 6 * cp.h, Y: cp.s, Z: cp.v})
			cp.rgb = rgbfVec(rgb)

			cp.picker1dActive = true
		}
	}
	// and advance our vertical position
	hy += pitch * 2

	hxm = hx + pitch*7 + size
	hym = hy + pitch*6 + size
	// Two-dimensional thing to pick a color by hue and saturation
	if how == howPaint {
		uiCanvas.DrawPixmap(hsvPattern2d(hxm-hx, hym-hy), hx, hy, 2)

		cx := hx + int(float64(hxm-hx)*cp.h)
		cy := hy + int(float64(hym-hy)*cp.s)
		uiCanvas.DrawLine(cx-5, cy, cx+5, cy, core.NewRgbaColor(255, 255, 255, 255), 0, 3)
		uiCanvas.DrawLine(cx, cy-5, cx, cy+5, core.NewRgbaColor(255, 255, 255, 255), 0, 3)
	} else if how == howClick || (how == howHover && leftDown && cp.picker2dActive) {
		if in(hx, hy, hxm, hym) {
			h := (x - float64(hx)) / float64(hxm-hx)
			s := (y - float64(hy)) / float64(hym-hy)
			cp.h = h
			cp.s = s

			rgb := hsvToRgb(geom.Vector{X: 6 * cp.h, Y: cp.s, Z: cp.v})
			cp.rgb = rgbfVec(rgb)

			cp.picker2dActive = true
		}
	}

	if mousePointerAsHand {
		tw.window.SetCursor(platform.CursorHand)
	} else {
		tw.window.SetCursor(platform.CursorPointer)
	}
	return true
}

// Paint draws the window (C++ TextWindow::Paint).
func (tw *TextWindow) Paint() {
	if tw.canvas == nil {
		return
	}
	ss := tw.ss

	width, height := tw.window.GetContentSize()
	if tw.halfRows != int(height)/(lineHeight/2) {
		tw.Resize()
	}

	var camera render.Camera
	camera.Width = width
	camera.Height = height
	camera.PixelRatio = tw.window.GetDevicePixelRatio()
	camera.GridFit = tw.window.GetDevicePixelRatio() == 1
	camera.LoadIdentity()
	camera.Offset.X = -camera.Width / 2.0
	camera.Offset.Y = -camera.Height / 2.0

	var lighting render.Lighting
	lighting.BackgroundColor = rgbi(0, 0, 0)

	canvas := tw.canvas
	ss.GW.shareBitmapFont(canvas)
	canvas.SetLighting(lighting)
	canvas.SetCamera(camera)
	canvas.StartFrame()

	uiCanvas := &render.UiCanvas{Canvas: canvas, Flip: true}

	cols := min(int(width)/charWidth+1, MaxCols)
	for a := range 2 {
		for r := 0; r < tw.rows; r++ {
			ltop := tw.lineTop(r)
			if ltop < tw.scrollPos-1 {
				continue
			}
			if ltop > tw.scrollPos+tw.halfRows {
				break
			}
			row := &tw.lines[r]

			for c := range cols {
				x := leftMargin + c*charWidth
				y := (ltop-tw.scrollPos)*(lineHeight/2) + 4

				fg := row.meta[c].fg
				bg := row.meta[c].bg

				// On the first pass, all the background quads; on the
				// next pass, all the foreground (i.e., font) quads.
				if a == 0 {
					bgRgb := row.meta[c].bgRgb
					bh, adj := lineHeight, 0
					if bg == 'z' {
						bh = charHeight
						adj += 2
					} else {
						bgRgb = colorFromTable(&bgColorTable, bg)
					}

					if bg != 'd' {
						// Move the quad down a bit, so that the
						// descenders still have the correct background.
						uiCanvas.DrawRect(x, x+charWidth, y+adj, y+adj+bh, bgRgb, core.RgbaColor{}, 0)
					}
				} else {
					fgRgb := colorFromTable(&fgColorTable, fg)
					if row.text[c] != ' ' {
						uiCanvas.DrawBitmapChar(row.text[c], x, y+charHeight, fgRgb, 0)
					}

					// If this is a link and it's hovered, then draw the
					// underline
					m := &row.meta[c]
					if m.link != 0 && m.link != 'n' && r == tw.hoveredRow && c == tw.hoveredCol {
						same := func(k int) bool {
							o := &row.meta[k]
							return o.link != 0 && o.fID == m.fID && o.data == m.data
						}
						cs, cf := c, c
						for cs >= 0 && same(cs) {
							cs--
						}
						cs++

						for cf < MaxCols && same(cf) {
							cf++
						}

						// But don't underline checkboxes or radio buttons
						for ((row.text[cs] >= 0xe000 && row.text[cs] <= 0xefff) || row.text[cs] == ' ') &&
							cs < cf {
							cs++
						}

						// Always use the color of the rightmost character
						// in the link, so that underline is consistent
						// color
						fg = row.meta[cf-1].fg
						fgRgb = colorFromTable(&fgColorTable, fg)
						yp := y + charHeight
						uiCanvas.DrawLine(leftMargin+cs*charWidth, yp, leftMargin+cf*charWidth, yp, fgRgb, 1, 0)
					}
				}
			}
		}
	}

	// The line to indicate the column of radio buttons that indicates
	// the active group.
	ss.GW.GroupSelection()
	gs := &ss.GW.gs
	// Make sure this test agrees with test to determine which screen is
	// drawn
	if ss.GW.pending.description == "" && gs.n == 0 && gs.constraints == 0 &&
		tw.shown.screen == ScreenListOfGroups {
		x, y := 29, 70+lineHeight
		y -= tw.scrollPos * (lineHeight / 2)

		color := colorFromTable(&fgColorTable, 't')
		uiCanvas.DrawLine(x, y, x, y+40, color, 1, 0)
	}

	// The header has some icons that are drawn separately from the text
	tw.DrawOrHitTestIcons(uiCanvas, howPaint, 0, 0)

	// And we may show a color picker for certain editable fields
	tw.DrawOrHitTestColorPicker(uiCanvas, howPaint, false, 0, 0)

	canvas.FlushFrame()
	canvas.FinishFrame()
	tw.window.PresentFrame(canvas)
	canvas.Clear()
}

// MouseEvent handles the mouse moving or clicking in the window (C++
// TextWindow::MouseEvent).
func (tw *TextWindow) MouseEvent(leftClick, leftDown bool, x, y float64) {
	ss := tw.ss
	if tw.window.IsEditorVisible() || (ss.GW.window != nil && ss.GW.window.IsEditorVisible()) {
		how := howHover
		if leftClick {
			how = howClick
		}
		if tw.DrawOrHitTestColorPicker(nil, how, leftDown, x, y) {
			return
		}

		if leftClick {
			tw.HideEditControl()
			if ss.GW.window != nil {
				ss.GW.window.HideEditor()
			}
		} else {
			tw.window.SetCursor(platform.CursorPointer)
		}
		return
	}

	how := howHover
	if leftClick {
		how = howClick
	}
	tw.DrawOrHitTestIcons(nil, how, x, y)

	ps := ss.GW.hover
	ss.GW.hover.Clear()

	prevHoveredRow, prevHoveredCol := tw.hoveredRow, tw.hoveredCol
	tw.hoveredRow = 0
	tw.hoveredCol = 0

	// Find the corresponding character in the text buffer
	c := int((x - leftMargin) / charWidth)
	hh := float64(lineHeight / 2)
	y += float64(tw.scrollPos) * hh
	r := 0
	for r = 0; r < tw.rows; r++ {
		if y >= float64(tw.lineTop(r))*hh && y <= float64(tw.lineTop(r)+2)*hh {
			break
		}
	}
	if r >= 0 && c >= 0 && r < tw.rows && c < MaxCols {
		tw.window.SetCursor(platform.CursorPointer)

		tw.hoveredRow = r
		tw.hoveredCol = c

		item := tw.lines[r].meta[c]
		if leftClick {
			if item.link != 0 && item.f != nil {
				item.f(item.link, item.data)
				tw.Show()
				ss.GW.Invalidate(false)
			}
		} else {
			if item.link != 0 {
				tw.window.SetCursor(platform.CursorHand)
				if item.h != nil {
					item.h(item.link, item.data)
				}
			} else {
				tw.window.SetCursor(platform.CursorPointer)
			}
		}
	}

	if !ps.Equals(&ss.GW.hover) || prevHoveredRow != tw.hoveredRow || prevHoveredCol != tw.hoveredCol {
		ss.GW.Invalidate(false)
		tw.window.Invalidate()
	}
}

// MouseLeave forgets the hovered cell (C++ TextWindow::MouseLeave).
func (tw *TextWindow) MouseLeave() {
	tw.hoveredButton = nil
	tw.hoveredRow = 0
	tw.hoveredCol = 0
	tw.window.Invalidate()
}

// ScrollbarEvent scrolls to a position in half-rows (C++
// TextWindow::ScrollbarEvent).
func (tw *TextWindow) ScrollbarEvent(newPos float64) {
	if tw.window.IsEditorVisible() {
		// An edit field is active. Do not move the scrollbar.
		return
	}

	bottom := tw.lineTop(tw.rows-1) + 2
	pos := min(int(newPos), bottom-tw.halfRows)
	pos = max(pos, 0)
	if pos != tw.scrollPos {
		tw.scrollPos = pos
		tw.window.SetScrollbarPosition(float64(tw.scrollPos))
		tw.window.Invalidate()
	}
}

// GoToScreen shows another screen at the next Show (C++
// TextWindow::GoToScreen).
func (tw *TextWindow) GoToScreen(screen Screen) {
	tw.shown.screen = screen
}

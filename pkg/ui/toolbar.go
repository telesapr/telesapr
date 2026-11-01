package ui

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
)

// toolIcon is a toolbar button; an empty name is a spacer.
type toolIcon struct {
	name    string
	command Command
	tooltip string
}

// toolbar is the toolbar at the left of the graphics window.
var toolbar = []toolIcon{
	{"line", cmdLineSegment, "Sketch line segment"},
	{"rectangle", cmdRectangle, "Sketch rectangle"},
	{"circle", cmdCircle, "Sketch circle"},
	{"arc", cmdArc, "Sketch arc of a circle"},
	{"text", cmdTTFText, "Sketch curves from text in a TrueType font"},
	{"image", cmdImage, "Sketch image from a file"},
	{"tangent-arc", cmdTangentArc, "Create tangent arc at selected point"},
	{"bezier", cmdCubic, "Sketch cubic Bezier spline"},
	{"point", cmdDatumPoint, "Sketch datum point"},
	{"construction", cmdConstruction, "Toggle construction"},
	{"trim", cmdSplitCurves, "Split lines / curves where they intersect"},
	{"", cmdNone, ""},

	{"length", cmdDistanceDia, "Constrain distance / diameter / length"},
	{"angle", cmdAngle, "Constrain angle"},
	{"horiz", cmdHorizontal, "Constrain to be horizontal"},
	{"vert", cmdVertical, "Constrain to be vertical"},
	{"parallel", cmdParallel, "Constrain to be parallel or tangent"},
	{"perpendicular", cmdPerpendicular, "Constrain to be perpendicular"},
	{"pointonx", cmdOnEntity, "Constrain point on line / curve / plane / point"},
	{"symmetric", cmdSymmetric, "Constrain symmetric"},
	{"equal", cmdEqual, "Constrain equal length / radius / angle"},
	{"same-orientation", cmdOrientedSame, "Constrain normals in same orientation"},
	{"other-supp", cmdOtherAngle, "Other supplementary angle"},
	{"ref", cmdReference, "Toggle reference dimension"},
	{"", cmdNone, ""},

	{"extrude", cmdGroupExtrude, "New group extruding active sketch"},
	{"lathe", cmdGroupLathe, "New group rotating active sketch"},
	{"helix", cmdGroupHelix, "New group helix from active sketch"},
	{"revolve", cmdGroupRevolve, "New group revolve active sketch"},
	{"step-rotate", cmdGroupRot, "New group step and repeat rotating"},
	{"step-translate", cmdGroupTrans, "New group step and repeat translating"},
	{"sketch-in-plane", cmdGroupWrkpl, "New group in new workplane (thru given entities)"},
	{"sketch-in-3d", cmdGroup3D, "New group in 3d"},
	{"assemble", cmdGroupLink, "New group linking / assembling file"},
	{"", cmdNone, ""},

	{"fillet", cmdGroupFillet, "Fillet selected edges"},
	{"chamfer", cmdGroupChamfer, "Chamfer selected edges"},
	{"shell", cmdGroupShell, "Shell solid, open at selected faces"},
	{"sweep", cmdGroupSweep, "Sweep active sketch along selected path"},
	{"loft", cmdGroupLoft, "Loft active sketch to selected profiles"},
	{"mirror", cmdGroupMirror, "Mirror in selected plane"},
	{"pattern", cmdGroupPathPattern, "Pattern along selected path"},
	{"thread", cmdGroupThread, "Screw thread on selected face, edge or circle"},
	{"", cmdNone, ""},

	{"in3d", cmdNearestIso, "Nearest isometric view"},
	{"ontoworkplane", cmdOntoWorkplane, "Align view to active workplane"},
	{"measure", cmdMeasure, "Measure selection"},
	{"section", cmdSectionView, "Section view"},
}

// Toolbar geometry, in pixels: a button's pitch, the gap a spacer adds,
// and the room left below the menu bar and round the buttons.
const (
	toolbarPitch  = 32
	toolbarSpacer = 16
	toolbarFudge  = 8
	toolbarTop    = 21 // the menu bar is 20 high
)

// toolbarIcon is a button's picture, loaded the first time the window
// draws it. Each window keeps its own: the toolbar table is shared by
// every window, which may draw at the same time.
func (gw *graphicsWindow) toolbarIcon(name string) *res.Pixmap {
	if pm, ok := gw.toolbarIcons[name]; ok {
		return pm
	}
	if gw.toolbarIcons == nil {
		gw.toolbarIcons = map[string]*res.Pixmap{}
	}
	pm := res.LoadPng("icons/graphics-window/" + name + ".png")
	gw.toolbarIcons[name] = pm
	return pm
}

// toolbarRows is the number of rows the toolbar's buttons take in cols
// columns, and the number of spacers between their sections.
func toolbarRows(cols int) (rows, spacers int) {
	inSection := 0
	for _, icon := range toolbar {
		if icon.name == "" {
			rows += (inSection + cols - 1) / cols
			spacers++
			inSection = 0
			continue
		}
		inSection++
	}
	rows += (inSection + cols - 1) / cols
	return rows, spacers
}

// toolbarHeight is the height of the toolbar in cols columns.
func toolbarHeight(cols int) int {
	rows, spacers := toolbarRows(cols)
	return rows*toolbarPitch + spacers*toolbarSpacer + toolbarFudge
}

// toolbarColumns is how many columns the toolbar takes in a window of
// that height: two, as SolveSpace's, or three when two would not fit.
func toolbarColumns(height int) int {
	if toolbarHeight(2) <= height-toolbarTop {
		return 2
	}
	return 3
}

// toolbarWidth is the width of the toolbar in cols columns.
func toolbarWidth(cols int) int { return 2 + toolbarPitch*cols }

// toolbarToggled reports whether a button's command is a mode that is
// on, which its button shows as it shows the pending one.
func (gw *graphicsWindow) toolbarToggled(cmd Command) bool {
	switch cmd {
	case cmdMeasure:
		return gw.measuring
	case cmdSectionView:
		return gw.sectionMenuOn
	}
	return false
}

// ToolbarDraw draws the toolbar.
func (gw *graphicsWindow) ToolbarDraw(canvas *render.UICanvas) {
	gw.toolbarDrawOrHitTest(0, 0, canvas)
}

// ToolbarMouseMoved highlights the button under the mouse and sets its
// tooltip; it reports whether the mouse is over the toolbar.
func (gw *graphicsWindow) ToolbarMouseMoved(x, y int) bool {
	width, height := gw.window.ContentSize()
	x += int(width) / 2
	y += int(height) / 2

	within, hitCommand, hitX, hitY := gw.toolbarDrawOrHitTest(x, y, nil)

	if hitCommand != gw.toolbarHovered {
		gw.toolbarHovered = hitCommand
		gw.Invalidate(false)
	}

	if gw.toolbarHovered != cmdNone {
		var tooltip string
		for _, icon := range toolbar {
			if gw.toolbarHovered == icon.command {
				tooltip = icon.tooltip
			}
		}
		accel := gw.AcceleratorForCommand(gw.toolbarHovered)
		if accelDesc := platform.AcceleratorDescription(accel); accelDesc != "" {
			tooltip += fmt.Sprintf(" (%s)", accelDesc)
		}
		gw.window.SetTooltip(tooltip, float64(hitX), float64(hitY), 32, 32)
	} else {
		gw.window.SetTooltip("", 0, 0, 0, 0)
	}
	return within
}

// ToolbarMouseDown runs the button clicked; it reports whether the
// click was on the toolbar.
func (gw *graphicsWindow) ToolbarMouseDown(x, y int) bool {
	width, height := gw.window.ContentSize()
	x += int(width) / 2
	y += int(height) / 2

	within, hitCommand, _, _ := gw.toolbarDrawOrHitTest(x, y, nil)
	if hitCommand != cmdNone {
		gw.ActivateCommand(hitCommand)
	}
	return within
}

// toolbarDrawOrHitTest draws the toolbar, or with a nil canvas finds
// the button at mx, my. The buttons run in rows of two, or of three in a
// short window, each section of the table starting a row of its own.
func (gw *graphicsWindow) toolbarDrawOrHitTest(mx, my int, canvas *render.UICanvas) (within bool, hitCommand Command, hitX, hitY int) {
	_, height := gw.window.ContentSize()
	cols := toolbarColumns(int(height))
	h := toolbarHeight(cols)

	const x0 = 17
	y := int(height) - toolbarTop
	if h < y {
		// If there is enough vertical space leave up to 32 pixels between
		// the menu bar and the toolbar.
		y -= min(y-h, 32)
	}

	// When changing these, also change the asReference drawing code in
	// drawentity.go and the SetMinContentSize in graphicswin.go, which
	// use toolbarHeight and toolbarWidth.
	aleft, aright, atop, abot := 0, toolbarWidth(cols), y+16+toolbarFudge/2, y+16-h

	within = mx >= aleft && mx <= aright && my <= atop && my >= abot

	if canvas == nil && !within {
		// This gets called every MouseMove event, so return quickly.
		return false, cmdNone, 0, 0
	}

	if canvas != nil {
		canvas.DrawRect(aleft, aright, atop, abot, core.NewRGBA(30, 30, 30, 255), core.RGBA{}, 0)
	}

	col := 0
	x := x0
	for i := range toolbar {
		icon := &toolbar[i]
		if icon.name == "" { // spacer
			if col != 0 {
				col, x = 0, x0
				y -= toolbarPitch
			}
			y -= toolbarSpacer

			if canvas != nil {
				// Draw a separator bar in a slightly different color.
				divw, divh := toolbarWidth(cols)/2-3, 2
				mid := aright / 2
				canvas.DrawRect(mid+divw, mid-divw, y+24+divh, y+24-divh,
					core.NewRGBA(45, 45, 45, 255), core.RGBA{}, 0)
			}
			continue
		}

		if canvas != nil {
			pm := gw.toolbarIcon(icon.name)
			canvas.DrawPixmap(pm, x-pm.Width/2, y-pm.Height/2, 0)

			if gw.toolbarHovered == icon.command || gw.toolbarToggled(icon.command) ||
				(gw.pending.operation == pendingCommand && gw.pending.command == icon.command) {
				// Highlight the hovered, toggled or pending item.
				const boxhw = 15
				canvas.DrawRect(x+boxhw, x-boxhw, y+boxhw, y-boxhw,
					core.NewRGBA(255, 255, 0, 75), core.RGBA{}, 0)
			}
		} else {
			const boxhw = 16
			if mx < x+boxhw && mx > x-boxhw && my < y+boxhw && my > y-boxhw {
				hitCommand = icon.command
				hitX = x - boxhw
				hitY = int(height) - (y + boxhw)
			}
		}

		col++
		x += toolbarPitch
		if col == cols {
			col, x = 0, x0
			y -= toolbarPitch
		}
	}
	return within, hitCommand, hitX, hitY
}

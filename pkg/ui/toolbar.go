package ui

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/render"
	"github.com/spacesolve/spacesolve/pkg/res"
)

// toolIcon is a toolbar button; an empty name is a spacer (C++
// ToolIcon).
type toolIcon struct {
	name    string
	command Command
	tooltip string
	pixmap  *res.Pixmap
}

// toolbar is the toolbar at the left of the graphics window (C++
// Toolbar[], toolbar.cpp).
var toolbar = []toolIcon{
	{"line", CmdLineSegment, "Sketch line segment", nil},
	{"rectangle", CmdRectangle, "Sketch rectangle", nil},
	{"circle", CmdCircle, "Sketch circle", nil},
	{"arc", CmdArc, "Sketch arc of a circle", nil},
	{"text", CmdTTFText, "Sketch curves from text in a TrueType font", nil},
	{"image", CmdImage, "Sketch image from a file", nil},
	{"tangent-arc", CmdTangentArc, "Create tangent arc at selected point", nil},
	{"bezier", CmdCubic, "Sketch cubic Bezier spline", nil},
	{"point", CmdDatumPoint, "Sketch datum point", nil},
	{"construction", CmdConstruction, "Toggle construction", nil},
	{"trim", CmdSplitCurves, "Split lines / curves where they intersect", nil},
	{"", CmdNone, "", nil},

	{"length", CmdDistanceDia, "Constrain distance / diameter / length", nil},
	{"angle", CmdAngle, "Constrain angle", nil},
	{"horiz", CmdHorizontal, "Constrain to be horizontal", nil},
	{"vert", CmdVertical, "Constrain to be vertical", nil},
	{"parallel", CmdParallel, "Constrain to be parallel or tangent", nil},
	{"perpendicular", CmdPerpendicular, "Constrain to be perpendicular", nil},
	{"pointonx", CmdOnEntity, "Constrain point on line / curve / plane / point", nil},
	{"symmetric", CmdSymmetric, "Constrain symmetric", nil},
	{"equal", CmdEqual, "Constrain equal length / radius / angle", nil},
	{"same-orientation", CmdOrientedSame, "Constrain normals in same orientation", nil},
	{"other-supp", CmdOtherAngle, "Other supplementary angle", nil},
	{"ref", CmdReference, "Toggle reference dimension", nil},
	{"", CmdNone, "", nil},

	{"extrude", CmdGroupExtrude, "New group extruding active sketch", nil},
	{"lathe", CmdGroupLathe, "New group rotating active sketch", nil},
	{"helix", CmdGroupHelix, "New group helix from active sketch", nil},
	{"revolve", CmdGroupRevolve, "New group revolve active sketch", nil},
	{"step-rotate", CmdGroupRot, "New group step and repeat rotating", nil},
	{"step-translate", CmdGroupTrans, "New group step and repeat translating", nil},
	{"sketch-in-plane", CmdGroupWrkpl, "New group in new workplane (thru given entities)", nil},
	{"sketch-in-3d", CmdGroup3D, "New group in 3d", nil},
	{"assemble", CmdGroupLink, "New group linking / assembling file", nil},
	{"", CmdNone, "", nil},

	{"in3d", CmdNearestIso, "Nearest isometric view", nil},
	{"ontoworkplane", CmdOntoWorkplane, "Align view to active workplane", nil},
}

// ToolbarDraw draws the toolbar.
func (gw *GraphicsWindow) ToolbarDraw(canvas *render.UiCanvas) {
	gw.toolbarDrawOrHitTest(0, 0, canvas)
}

// ToolbarMouseMoved highlights the button under the mouse and sets its
// tooltip; it reports whether the mouse is over the toolbar.
func (gw *GraphicsWindow) ToolbarMouseMoved(x, y int) bool {
	width, height := gw.window.GetContentSize()
	x += int(width) / 2
	y += int(height) / 2

	within, hitCommand, hitX, hitY := gw.toolbarDrawOrHitTest(x, y, nil)

	if hitCommand != gw.toolbarHovered {
		gw.toolbarHovered = hitCommand
		gw.Invalidate(false)
	}

	if gw.toolbarHovered != CmdNone {
		var tooltip string
		for _, icon := range toolbar {
			if gw.toolbarHovered == icon.command {
				tooltip = icon.tooltip
			}
		}
		accel := gw.AcceleratorForCommand(gw.toolbarHovered)
		if accelDesc := platform.AcceleratorDescription(accel); accelDesc != "" {
			tooltip += sprintf(" (%s)", accelDesc)
		}
		gw.window.SetTooltip(tooltip, float64(hitX), float64(hitY), 32, 32)
	} else {
		gw.window.SetTooltip("", 0, 0, 0, 0)
	}
	return within
}

// ToolbarMouseDown runs the button clicked; it reports whether the
// click was on the toolbar.
func (gw *GraphicsWindow) ToolbarMouseDown(x, y int) bool {
	width, height := gw.window.GetContentSize()
	x += int(width) / 2
	y += int(height) / 2

	within, hitCommand, _, _ := gw.toolbarDrawOrHitTest(x, y, nil)
	if hitCommand != CmdNone {
		gw.ActivateCommand(hitCommand)
	}
	return within
}

// toolbarDrawOrHitTest draws the toolbar, or with a nil canvas finds
// the button at mx, my (C++ ToolbarDrawOrHitTest).
func (gw *GraphicsWindow) toolbarDrawOrHitTest(mx, my int, canvas *render.UiCanvas) (within bool, hitCommand Command, hitX, hitY int) {
	_, height := gw.window.GetContentSize()

	x, y := 17, int(height-21) // 20 is the menu bar height

	// When changing these values, also change the asReference drawing
	// code in drawentity.go as well as the SetMinContentSize in
	// graphicswin.go.
	const fudge = 8
	const h = 32*18 + 3*16 + fudge // 18 icons * 32 pixels + 3 dividers * 16 pixels + fudge

	if h < y {
		// If there is enough vertical space leave up to 32 pixels between
		// the menu bar and the toolbar.
		if y-h < 32 {
			y -= y - h
		} else {
			y -= 32
		}
	}

	aleft, aright, atop, abot := 0, 66, y+16+fudge/2, y+16-h

	within = mx >= aleft && mx <= aright && my <= atop && my >= abot

	if canvas == nil && !within {
		// This gets called every MouseMove event, so return quickly.
		return false, CmdNone, 0, 0
	}

	if canvas != nil {
		canvas.DrawRect(aleft, aright, atop, abot, core.NewRgbaColor(30, 30, 30, 255), core.RgbaColor{}, 0)
	}

	leftpos := true
	for i := range toolbar {
		icon := &toolbar[i]
		if icon.name == "" { // spacer
			if !leftpos {
				leftpos = true
				y -= 32
				x -= 32
			}
			y -= 16

			if canvas != nil {
				// Draw a separator bar in a slightly different color.
				divw, divh := 30, 2
				canvas.DrawRect(x+16+divw, x+16-divw, y+24+divh, y+24-divh,
					core.NewRgbaColor(45, 45, 45, 255), core.RgbaColor{}, 0)
			}
			continue
		}

		if icon.pixmap == nil {
			icon.pixmap = res.LoadPng("icons/graphics-window/" + icon.name + ".png")
		}

		if canvas != nil {
			canvas.DrawPixmap(icon.pixmap, x-icon.pixmap.Width/2, y-icon.pixmap.Height/2, 0)

			if gw.toolbarHovered == icon.command ||
				(gw.pending.operation == PendingCommand && gw.pending.command == icon.command) {
				// Highlight the hovered or pending item.
				const boxhw = 15
				canvas.DrawRect(x+boxhw, x-boxhw, y+boxhw, y-boxhw,
					core.NewRgbaColor(255, 255, 0, 75), core.RgbaColor{}, 0)
			}
		} else {
			const boxhw = 16
			if mx < x+boxhw && mx > x-boxhw && my < y+boxhw && my > y-boxhw {
				hitCommand = icon.command
				hitX = x - boxhw
				hitY = int(height) - (y + boxhw)
			}
		}

		if leftpos {
			x += 32
			leftpos = false
		} else {
			x -= 32
			y -= 32
			leftpos = true
		}
	}
	return within, hitCommand, hitX, hitY
}

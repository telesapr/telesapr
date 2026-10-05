package ui

// The icons at the top of the text window (C++ textwin.cpp's Button
// classes and buttons[]).

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/res"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// textButton is one icon (C++ Button).
type textButton interface {
	Tooltip() string
	Draw(uiCanvas *render.UiCanvas, x, y int, asHovered bool)
	AdvanceWidth() int
	Click()
}

// spacerButton is a gap between groups of icons (C++ SpacerButton).
type spacerButton struct{}

func (*spacerButton) Tooltip() string { return "" }

func (*spacerButton) Draw(uiCanvas *render.UiCanvas, x, y int, asHovered bool) {
	// Draw a darker-grey spacer in between the groups of icons.
	uiCanvas.DrawRect(x, x+4, y, y-24, core.NewRgbaColor(45, 45, 45, 255), core.RgbaColor{}, 0)
}

func (*spacerButton) AdvanceWidth() int { return 12 }
func (*spacerButton) Click()            {}

// showHideButton toggles a show flag (C++ ShowHideButton).
type showHideButton struct {
	gw       *GraphicsWindow
	variable *bool
	tooltip  string
	iconName string
	icon     *res.Pixmap
	// tooltipFn replaces the tooltip (C++ FacesButton).
	tooltipFn func() string
}

func (b *showHideButton) Tooltip() string {
	if b.tooltipFn != nil {
		return b.tooltipFn()
	}
	if *b.variable {
		return "Hide " + b.tooltip
	}
	return "Show " + b.tooltip
}

func (b *showHideButton) Draw(uiCanvas *render.UiCanvas, x, y int, asHovered bool) {
	if b.icon == nil {
		b.icon = res.LoadPng("icons/text-window/" + b.iconName + ".png")
	}

	uiCanvas.DrawPixmap(b.icon, x, y-24, 0)
	if asHovered {
		uiCanvas.DrawRect(x-2, x+26, y+2, y-26, core.NewRgbaColor(255, 255, 0, 75), core.RgbaColor{}, 0)
	}
	if !*b.variable {
		s, f := 0, 23
		color := core.NewRgbaColor(255, 0, 0, 150)
		uiCanvas.DrawLine(x+s, y-s, x+f, y-f, color, 2, 0)
		uiCanvas.DrawLine(x+s, y-f, x+f, y-s, color, 2, 0)
	}
}

func (b *showHideButton) AdvanceWidth() int { return 32 }
func (b *showHideButton) Click()            { b.gw.ToggleBool(b.variable) }

// triStateButton cycles through three modes (C++ TriStateButton).
type triStateButton struct {
	ss        *Application
	get       func() uint32
	set       func(uint32)
	states    [3]uint32
	tooltips  [3]string
	iconNames [3]string
	icons     [3]*res.Pixmap
}

func (b *triStateButton) Tooltip() string {
	for k := range 3 {
		if b.get() == b.states[k] {
			return b.tooltips[k]
		}
	}
	panic("ui: unexpected mode")
}

func (b *triStateButton) Draw(uiCanvas *render.UiCanvas, x, y int, asHovered bool) {
	for k := range 3 {
		if b.icons[k] == nil {
			b.icons[k] = res.LoadPng("icons/text-window/" + b.iconNames[k] + ".png")
		}
	}

	var icon *res.Pixmap
	for k := range 3 {
		if b.get() == b.states[k] {
			icon = b.icons[k]
			break
		}
	}

	uiCanvas.DrawPixmap(icon, x, y-24, 0)
	if asHovered {
		uiCanvas.DrawRect(x-2, x+26, y+2, y-26, core.NewRgbaColor(255, 255, 0, 75), core.RgbaColor{}, 0)
	}
}

func (b *triStateButton) AdvanceWidth() int { return 32 }

func (b *triStateButton) Click() {
	for k := range 3 {
		if b.get() == b.states[k] {
			b.set(b.states[(k+1)%3])
			break
		}
	}

	b.ss.GenerateAll(sketch.GenerateDirty)
	b.ss.GW.Invalidate(false)
	b.ss.ScheduleShowTW()
}

// newTextButtons makes the icons (C++ buttons[]).
func newTextButtons(ss *Application) []textButton {
	gw := ss.GW
	spacer := &spacerButton{}
	sh := func(v *bool, icon, tooltip string) *showHideButton {
		return &showHideButton{gw: gw, variable: v, iconName: icon, tooltip: tooltip}
	}
	faces := sh(&gw.showFaces, "faces", "")
	faces.tooltipFn = func() string {
		if gw.showFaces {
			return "Don't make faces selectable with mouse"
		}
		return "Make faces selectable with mouse"
	}
	constraints := &triStateButton{
		ss:     ss,
		get:    func() uint32 { return uint32(gw.showConstraints) },
		set:    func(v uint32) { gw.showConstraints = ShowConstraintMode(v) },
		states: [3]uint32{uint32(SCMShowAll), uint32(SCMShowDim), uint32(SCMNoShow)},
		tooltips: [3]string{"Show only dimensions", "Hide constraints and dimensions",
			"Show constraints and dimensions"},
		iconNames: [3]string{"constraint", "constraint-dimo", "constraint-wo"},
	}
	occludedLines := &triStateButton{
		ss:  ss,
		get: func() uint32 { return uint32(gw.drawOccludedAs) },
		set: func(v uint32) { gw.drawOccludedAs = DrawOccludedAs(v) },
		states: [3]uint32{uint32(OccludedInvisible), uint32(OccludedStippled),
			uint32(OccludedVisible)},
		tooltips: [3]string{"Stipple occluded lines", "Draw occluded lines",
			"Don't draw occluded lines"},
		iconNames: [3]string{"occluded-invisible", "occluded-stippled", "occluded-visible"},
	}
	return []textButton{
		sh(&gw.show.Workplanes, "workplane", "workplanes from inactive groups"),
		sh(&gw.show.Normals, "normal", "normals"),
		sh(&gw.show.Points, "point", "points"),
		sh(&gw.show.Construction, "construction", "construction entities"),
		constraints,
		faces,
		spacer,
		sh(&gw.showShaded, "shaded", "shaded view of solid model"),
		sh(&gw.show.Edges, "edges", "edges of solid model"),
		sh(&gw.show.Outlines, "outlines", "outline of solid model"),
		sh(&gw.showMesh, "mesh", "triangle mesh of solid model"),
		spacer,
		occludedLines,
	}
}

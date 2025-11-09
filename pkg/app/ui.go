package app

import (
	"cogentcore.org/core/colors"
	"cogentcore.org/core/core"
	"cogentcore.org/core/events"
	"cogentcore.org/core/icons"
	"cogentcore.org/core/styles"
	"cogentcore.org/core/styles/units"
	textstyle "cogentcore.org/core/text/text"
)

// UI wires the SolveSpace application state to the Cogent Core widgets.
type UI struct {
	body        *core.Body
	menu        []*MenuItem
	statusText  *core.Text
	recentFiles []string
}

// New constructs a UI instance and prepares the widget hierarchy. Call Run to
// start the event loop.
func New() *UI {
	ui := &UI{
		menu:        buildMenuTree(menuEntries),
		recentFiles: []string{}, // filled lazily by persistence layer later on
	}

	ui.body = core.NewBody("SolveSpace")
	ui.body.Styler(func(s *styles.Style) {
		s.Direction = styles.Column
		s.Grow.Set(1, 1)
		s.Padding.Zero()
	})

	ui.body.AddTopBar(ui.buildTopBar)
	ui.body.AddBottomBar(ui.buildStatusBar)
	ui.buildWorkspace()

	return ui
}

// Run launches the main application window.
func (ui *UI) Run() {
	ui.body.RunMainWindow()
}

func (ui *UI) buildTopBar(bar *core.Frame) {
	bar.Styler(func(s *styles.Style) {
		s.Direction = styles.Column
		s.Align.Items = styles.Start
		s.Gap.Set(units.Dp(2))
		s.Padding.Zero()
	})

	menuRow := core.NewFrame(bar)
	menuRow.SetName("menu-bar")
	menuRow.Styler(func(s *styles.Style) {
		s.Direction = styles.Row
		s.Align.Items = styles.Center
		s.Grow.Set(1, 0)
		s.Gap.Set(units.Dp(6))
		s.Padding.Set(units.Dp(4))
	})
	ui.populateMenuRow(menuRow)
}

func (ui *UI) populateMenuRow(parent *core.Frame) {
	for _, item := range ui.menu {
		label := stripMnemonics(item.Label)
		button := core.NewButton(parent).SetText(label)
		button.SetType(core.ButtonText)
		if len(item.Children) == 0 && item.ChildBuilder == nil {
			// Top level button with direct action (rare but handle anyway)
			ui.attachLeafHandler(button, item)
			continue
		}

		button.SetMenu(func(sc *core.Scene) {
			ui.populateMenuScene(sc, item)
		})
	}
}

func (ui *UI) populateMenuScene(sc *core.Scene, item *MenuItem) {
	children := item.Children
	if item.ChildBuilder != nil {
		children = item.ChildBuilder(ui, item)
	}

	for _, child := range children {
		if child.Separator {
			core.NewSeparator(sc)
			continue
		}

		label := stripMnemonics(child.Label)
		btn := core.NewButton(sc).SetText(label)
		btn.SetType(core.ButtonMenu)

		if child.ChildBuilder != nil || len(child.Children) > 0 {
			btn.SetMenu(func(sub *core.Scene) {
				ui.populateMenuScene(sub, child)
			})
			continue
		}

		ui.attachLeafHandler(btn, child)
	}
}

func (ui *UI) attachLeafHandler(btn *core.Button, item *MenuItem) {
	switch {
	case item.OnTriggered != nil:
		handler := item.OnTriggered
		btn.OnClick(func(e events.Event) {
			handler(ui)
		})
	case item.Handler != nil && item.Command != CommandNone:
		handler := item.Handler
		cmd := item.Command
		btn.OnClick(func(e events.Event) {
			handler(ui, cmd)
		})
	default:
		btn.SetEnabled(false)
	}
}

func (ui *UI) addToolbarButton(parent *core.Frame, icon icons.Icon, label string, handler MenuHandler, cmd Command) {
	btn := core.NewButton(parent).SetText(label).SetIcon(icon)
	btn.SetType(core.ButtonAction)
	btn.OnClick(func(e events.Event) {
		handler(ui, cmd)
	})
}

func (ui *UI) buildWorkspace() {
	splits := core.NewSplits(ui.body).SetSplits(0.12, 0.58, 0.30)
	splits.SetName("workspace")
	splits.Styler(func(s *styles.Style) {
		s.Direction = styles.Row
		s.Grow.Set(1, 1)
		s.Min.Y.Em(20)
		s.Gap.Set(units.Dp(2))
		s.Padding.Set(units.Dp(2))
	})

	toolsPane := core.NewFrame(splits)
	toolsPane.SetName("tool-palette")
	toolsPane.Styler(func(s *styles.Style) {
		s.Direction = styles.Column
		s.Grow.Set(0, 1)
		s.Min.X.Dp(80)
		s.Padding.Set(units.Dp(6))
		s.Gap.Set(units.Dp(4))
		s.Background = colors.Scheme.SurfaceContainerHighest
	})
	ui.populateToolPalette(toolsPane)

	mainPane := core.NewFrame(splits)
	mainPane.SetName("main-view")
	mainPane.Styler(func(s *styles.Style) {
		s.Grow.Set(1, 1)
		s.Direction = styles.Column
		s.Padding.Set(units.Dp(8))
		s.Gap.Set(units.Dp(8))
	})

	core.NewText(mainPane).
		SetText("3D workspace placeholder\n(The model view will render here).").
		Styler(func(s *styles.Style) {
			s.Align.Self = styles.Center
			s.Color = colors.Scheme.OnSurface
			s.Text.Align = textstyle.Center
		})

	propsPane := core.NewFrame(splits)
	propsPane.SetName("property-panel")
	propsPane.Styler(func(s *styles.Style) {
		s.Direction = styles.Column
		s.Grow.Set(0, 1)
		s.Padding.Set(units.Dp(6))
		s.Gap.Set(units.Dp(6))
		s.Background = colors.Scheme.SurfaceContainerHigh
	})

	propToolbar := core.NewFrame(propsPane)
	propToolbar.SetName("property-toolbar")
	propToolbar.Styler(func(s *styles.Style) {
		s.Direction = styles.Row
		s.Align.Items = styles.Center
		s.Gap.Set(units.Dp(4))
	})
}

func (ui *UI) buildStatusBar(bar *core.Frame) {
	bar.Styler(func(s *styles.Style) {
		s.Direction = styles.Row
		s.Align.Items = styles.Center
		s.Gap.Set(units.Dp(4))
		s.Padding.Set(units.Dp(4))
		s.Grow.Set(1, 0)
	})

	ui.statusText = core.NewText(bar).SetText("Ready")
	ui.statusText.Styler(func(s *styles.Style) {
		s.Grow.Set(1, 0)
	})
}

func (ui *UI) buildOpenRecentMenu(_ *MenuItem) []*MenuItem {
	if len(ui.recentFiles) == 0 {
		return []*MenuItem{
			{
				Label:       "(no recent files)",
				Command:     CommandNone,
				OnTriggered: nil,
			},
		}
	}

	items := make([]*MenuItem, 0, len(ui.recentFiles))
	for _, path := range ui.recentFiles {
		p := path
		items = append(items, &MenuItem{
			Label: p,
			OnTriggered: func(ui *UI) {
				ui.openRecentFile(p)
			},
		})
	}
	return items
}

func (ui *UI) buildLinkRecentMenu(_ *MenuItem) []*MenuItem {
	// Until linking is implemented, mirror the file list.
	return ui.buildOpenRecentMenu(nil)
}

func (ui *UI) buildLocaleMenu(_ *MenuItem) []*MenuItem {
	// Placeholder until localization infrastructure is ported.
	return []*MenuItem{
		{Label: "System Default", OnTriggered: func(ui *UI) {
			ui.setStatus("Language switching is not yet implemented.")
		}},
	}
}

func (ui *UI) openRecentFile(path string) {
	ui.setStatus("Open recent: " + path + " (not yet implemented)")
}

func (ui *UI) setStatus(msg string) {
	if ui.statusText != nil {
		ui.statusText.SetText(msg)
		ui.statusText.Update()
	}
}

func (ui *UI) populateToolPalette(panel *core.Frame) {
	tools := []string{
		"Select",
		"Line",
		"Circle",
		"Arc",
		"Spline",
		"Constraint",
		"Measure",
		"Group",
	}
	for _, name := range tools {
		toolName := name
		btn := core.NewButton(panel).SetText(toolName)
		btn.SetType(core.ButtonAction)
		btn.Styler(func(s *styles.Style) {
			s.Grow.Set(1, 0)
		})
		btn.OnClick(func(e events.Event) {
			ui.setStatus(toolName + " tool is not implemented yet")
		})
	}
}

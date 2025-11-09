package app

import (
	"fmt"

	"cogentcore.org/core/core"
)

func (ui *UI) MenuFile(cmd Command) {
	switch cmd {
	case CommandNew:
		ui.setStatus("Starting a new sketch (not yet implemented)")
	case CommandOpen:
		ui.setStatus("Open file dialog not yet wired up")
	case CommandSave:
		ui.setStatus("Save functionality not yet implemented")
	case CommandSaveAs:
		ui.setStatus("Save As... functionality not yet implemented")
	case CommandExportImage, CommandExportView, CommandExportSection,
		CommandExportWireframe, CommandExportMesh, CommandExportSurfaces:
		ui.setStatus(fmt.Sprintf("%s export is pending", ui.commandLabel(cmd)))
	case CommandImport:
		ui.setStatus("Import functionality not yet implemented")
	case CommandExit:
		core.TheApp.QuitReq()
	default:
		ui.notifyPending(cmd)
	}
}

func (ui *UI) MenuEdit(cmd Command) {
	switch cmd {
	case CommandUndo, CommandRedo:
		ui.setStatus(fmt.Sprintf("%s history is not connected yet", ui.commandLabel(cmd)))
	case CommandRegenerateAll:
		ui.setStatus("Regenerate All will be available once the solver UI is connected")
	case CommandSnapToGrid, CommandRotate90:
		ui.notifyPending(cmd)
	case CommandSelectChain, CommandSelectAll, CommandUnselectAll,
		CommandEditLineStyles, CommandViewProjection, CommandConfiguration:
		ui.notifyPending(cmd)
	default:
		ui.notifyPending(cmd)
	}
}

func (ui *UI) MenuClipboard(cmd Command) {
	ui.notifyPending(cmd)
}

func (ui *UI) MenuView(cmd Command) {
	switch cmd {
	case CommandZoomIn, CommandZoomOut, CommandZoomToFit:
		ui.setStatus(fmt.Sprintf("%s is not wired to the 3D view yet", ui.commandLabel(cmd)))
	case CommandShowGrid, CommandDimSolidModel, CommandPerspectiveProj,
		CommandExplodeSketch, CommandShowToolbar, CommandShowTextWindow,
		CommandFullScreen:
		ui.setStatus(fmt.Sprintf("Toggle %s is pending", ui.commandLabel(cmd)))
	case CommandUnitsMillimeters, CommandUnitsMeters, CommandUnitsInches, CommandUnitsFeetInches:
		ui.setStatus(fmt.Sprintf("Unit change to %s will be available soon", ui.commandLabel(cmd)))
	default:
		ui.notifyPending(cmd)
	}
}

func (ui *UI) MenuGroup(cmd Command) {
	ui.notifyPending(cmd)
}

func (ui *UI) MenuRequest(cmd Command) {
	ui.notifyPending(cmd)
}

func (ui *UI) MenuConstrain(cmd Command) {
	ui.notifyPending(cmd)
}

func (ui *UI) MenuAnalyze(cmd Command) {
	ui.notifyPending(cmd)
}

func (ui *UI) MenuHelp(cmd Command) {
	switch cmd {
	case CommandWebsite:
		ui.setStatus("Website link launching is not implemented yet")
	case CommandGitHub:
		ui.setStatus("GitHub shortcut is not implemented yet")
	case CommandAbout:
		ui.setStatus("About dialog coming soon")
	default:
		ui.notifyPending(cmd)
	}
}

func (ui *UI) notifyPending(cmd Command) {
	ui.setStatus(fmt.Sprintf("%s is not implemented yet", ui.commandLabel(cmd)))
}

func (ui *UI) commandLabel(cmd Command) string {
	for _, root := range ui.menu {
		if label := findCommandLabel(root, cmd); label != "" {
			return label
		}
	}
	return fmt.Sprintf("Command %d", cmd)
}

func findCommandLabel(item *MenuItem, cmd Command) string {
	if item.Command == cmd && item.Label != "" {
		return stripMnemonics(item.Label)
	}
	children := item.Children
	if item.ChildBuilder != nil {
		// Dynamic menus do not contribute static labels for commands.
		return ""
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		if label := findCommandLabel(child, cmd); label != "" {
			return label
		}
	}
	return ""
}

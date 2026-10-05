package ui

// The file handling half of solvespace.cpp and file.cpp: loading,
// saving, autosave, the recent files and the File menu.

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// backupExt is the autosave file extension (C++ BACKUP_EXT).
const backupExt = tlspr.Extension + "~"

// maxWarnings is how many of a file's warnings a message lists.
const maxWarnings = 8

// warningList lists warnings for a message, at most maxWarnings.
func warningList(ws []string) string {
	if len(ws) > maxWarnings {
		return strings.Join(ws[:maxWarnings], "\n") + fmt.Sprintf("\n... and %d more", len(ws)-maxWarnings)
	}
	return strings.Join(ws, "\n")
}

// withExtension replaces the extension of path's file name with ext
// (C++ Path::WithExtension).
func withExtension(path, ext string) string {
	dir, base := filepath.Split(path)
	if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	return dir + base + "." + ext
}

// hasExtension reports whether path's extension is ext, ignoring case
// (C++ Path::HasExtension).
func hasExtension(path, ext string) bool {
	return strings.EqualFold(strings.TrimPrefix(filepath.Ext(path), "."), ext)
}

// errorStrings is the text of each error.
func errorStrings[E error](errs []E) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Error()
	}
	return out
}

// LoadAutosaveFor offers to load filename's autosave file instead, and
// loads it when the user agrees (C++ SolveSpaceUI::LoadAutosaveFor).
func (ss *Application) LoadAutosaveFor(filename string) bool {
	autosaveFile := withExtension(filename, backupExt)
	if _, err := os.Stat(autosaveFile); err != nil {
		return false
	}

	dialog := ss.Platform.CreateMessageDialog(ss.GW.window)
	dialog.SetType(platform.DialogQuestion)
	dialog.SetTitle("Autosave Available")
	dialog.SetMessage("An autosave file is available for this sketch.")
	dialog.SetDescription("Do you want to load the autosave file instead?")
	dialog.AddButton("&Load autosave", platform.ResponseYes, true)
	dialog.AddButton("Do&n't Load", platform.ResponseNo, false)

	if dialog.RunModal() == platform.ResponseYes {
		if !ss.LoadFromFile(autosaveFile) {
			return false
		}
		ss.SK.Unsaved = true
		return true
	}
	return false
}

// Load opens filename, or its autosave, as the sketch being edited (C++
// SolveSpaceUI::Load).
func (ss *Application) Load(filename string) bool {
	autosaveLoaded := ss.LoadAutosaveFor(filename)
	fileLoaded := autosaveLoaded || ss.LoadFromFile(filename)
	if fileLoaded {
		ss.SaveFile = filename
		ss.AddToRecentList(filename)
	} else {
		ss.SaveFile = ""
		ss.NewFile()
	}
	ss.AfterNewFile()
	ss.SK.Unsaved = autosaveLoaded
	return fileLoaded
}

// LoadFromFile reads filename into a new sketch (C++
// SolveSpaceUI::LoadFromFile). A linked file that is missing counts as
// declined rather than prompting for its new place.
func (ss *Application) LoadFromFile(filename string) bool {
	ss.AllConsistent = false

	if _, err := os.Stat(filename); err != nil {
		ss.Error("Couldn't read from file '%s'", filename)
		return false
	}
	if hasExtension(filename, "slvs") {
		ss.Error("'%s' is a SolveSpace model. Use File > Import, or telesapr-cli "+
			"convert, to convert it to a TeleSAPR model, which can then be opened.", filename)
		return false
	}

	doc, err := tlspr.LoadFile(filename)
	var ve *tlspr.VersionError
	switch {
	case errors.Is(err, tlspr.ErrCancelled):
		// The user gave up at a missing linked file.
		return false
	case errors.As(err, &ve):
		ss.Error("The file '%s' was saved by a newer version of the program, in format "+
			"version %d. This version reads files up to version %d.", filename, ve.File, ve.Reader)
		return false
	case err != nil:
		ss.Error("Unrecognized data in file. This file may be corrupt, or "+
			"from a newer version of the program.\n\n%v", err)
		return false
	}
	if len(doc.Warnings) > 0 {
		ss.Error("Some of the file '%s' was not recognized and was left out. It may "+
			"be from a newer version of the program.\n\n%s", filename, warningList(errorStrings(doc.Warnings)))
	}
	if len(doc.ExprErrors) > 0 {
		ss.Error("Some expressions in the file '%s' could not be evaluated. They are kept, "+
			"and what they drive keeps a default value until they are fixed.\n\n%s",
			filename, warningList(errorStrings(doc.ExprErrors)))
	}
	ss.setSketch(doc.Sketch)
	return true
}

// SaveToFile writes the sketch to filename (C++ SolveSpaceUI::SaveToFile).
func (ss *Application) SaveToFile(filename string) bool {
	if hasExtension(filename, "slvs") {
		ss.Error("TeleSAPR doesn't write SolveSpace models; save the sketch as a " +
			".tlspr file.")
		return false
	}

	// Make sure all the entities are regenerated up to date, since they
	// will be exported.
	ss.ScheduleShowTW()
	ss.GenerateAll(sketch.GenerateAll)

	err := tlspr.SaveFile(ss.SK, ss.SaveFile, filename, tlspr.Options{})
	var le *tlspr.LinkError
	switch {
	case errors.As(err, &le):
		ss.Error("This sketch links the sketch '%s'; it can only be saved "+
			"on the same volume.", le.Link)
		return false
	case err != nil:
		ss.Error("Couldn't write to file '%s'", filename)
		return false
	}
	return true
}

// AddToRecentList puts filename first in the recent files (C++
// SolveSpaceUI::AddToRecentList).
func (ss *Application) AddToRecentList(filename string) {
	for i, p := range ss.RecentFiles {
		if p == filename {
			ss.RecentFiles = append(ss.RecentFiles[:i], ss.RecentFiles[i+1:]...)
			break
		}
	}
	if len(ss.RecentFiles) > MaxRecent {
		ss.RecentFiles = append(ss.RecentFiles[:MaxRecent], ss.RecentFiles[MaxRecent+1:]...)
	}
	ss.RecentFiles = append([]string{filename}, ss.RecentFiles...)
	ss.GW.PopulateRecentFiles()
}

// GetFilenameAndSave saves the sketch, asking for a file name for a new
// sketch or for Save As (C++ SolveSpaceUI::GetFilenameAndSave).
func (ss *Application) GetFilenameAndSave(saveAs bool) bool {
	newSaveFile := ss.SaveFile

	if saveAs || newSaveFile == "" {
		dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
		dialog.AddFilters(platform.ModelFileFilters)
		dialog.ThawChoices(ss.Settings, "Sketch")
		if newSaveFile != "" {
			dialog.SetFilename(newSaveFile)
		}
		if !dialog.RunModal() {
			return false
		}
		dialog.FreezeChoices(ss.Settings, "Sketch")
		newSaveFile = modelFileName(dialog.GetFilename())
	}

	if !ss.SaveToFile(newSaveFile) {
		return false
	}
	ss.AddToRecentList(newSaveFile)
	ss.RemoveAutosave()
	ss.SaveFile = newSaveFile
	ss.SK.Unsaved = false
	return true
}

// modelFileName is the file a sketch is saved in when the user chose
// name: name with the .tlspr extension, in place of a .slvs extension or
// added to another.
func modelFileName(name string) string {
	switch {
	case hasExtension(name, tlspr.Extension):
		return name
	case hasExtension(name, "slvs"):
		return withExtension(name, tlspr.Extension)
	default:
		return name + "." + tlspr.Extension
	}
}

// Autosave saves a backup of an unsaved sketch next to its file (C++
// SolveSpaceUI::Autosave).
func (ss *Application) Autosave() {
	ss.ScheduleAutosave()

	if ss.SaveFile != "" && ss.SK.Unsaved {
		ss.SaveToFile(withExtension(ss.SaveFile, backupExt))
	}
}

// RemoveAutosave deletes the sketch's autosave file (C++
// SolveSpaceUI::RemoveAutosave).
func (ss *Application) RemoveAutosave() {
	if ss.SaveFile == "" {
		return
	}
	_ = os.Remove(withExtension(ss.SaveFile, backupExt))
}

// OkayToStartNewFile asks whether to save unsaved changes, and reports
// whether the sketch may be replaced (C++
// SolveSpaceUI::OkayToStartNewFile).
func (ss *Application) OkayToStartNewFile() bool {
	if !ss.SK.Unsaved {
		return true
	}
	if ss.messageSink != nil {
		// A script runs: a dialog would stop the loop under it, and
		// nobody is there to answer. The changes stay.
		ss.Error("The drawing has unsaved changes, which this would lose. A script saves them first with doc.Save, or lets them go with doc.ForgetChanges().")
		return false
	}

	dialog := ss.Platform.CreateMessageDialog(ss.GW.window)
	dialog.SetType(platform.DialogQuestion)
	dialog.SetTitle("Modified File")
	if ss.SaveFile != "" {
		dialog.SetMessage("Do you want to save the changes you made to the sketch “" +
			ss.SaveFile + "”?")
	} else {
		dialog.SetMessage("Do you want to save the changes you made to the new sketch?")
	}
	dialog.SetDescription("Your changes will be lost if you don't save them.")
	dialog.AddButton("&Save", platform.ResponseYes, true)
	dialog.AddButton("Do&n't Save", platform.ResponseNo, false)
	dialog.AddButton("&Cancel", platform.ResponseCancel, false)

	switch dialog.RunModal() {
	case platform.ResponseYes:
		return ss.GetFilenameAndSave(false)
	case platform.ResponseNo:
		ss.RemoveAutosave()
		return true
	default:
		return false
	}
}

// locateMissingFile asks whether to find a linked file that couldn't be
// read, and where it is (C++ LocateImportedFile, and the file dialogs
// of ReloadAllLinked and ReloadLinkedImage).
func (ss *Application) locateMissingFile(name string, isImage, canCancel bool) (string, link.LocateResponse) {
	dialog := ss.Platform.CreateMessageDialog(ss.GW.window)
	dialog.SetType(platform.DialogQuestion)
	dialog.SetTitle("Missing File")
	dialog.SetMessage("The linked file “" + name + "” is not present.")
	dialog.SetDescription("Do you want to locate it manually?\n\n" +
		"If you decline, any geometry that depends on " +
		"the missing file will be permanently removed.")
	dialog.AddButton("&Yes", platform.ResponseYes, true)
	dialog.AddButton("&No", platform.ResponseNo, false)
	if canCancel {
		dialog.AddButton("&Cancel", platform.ResponseCancel, false)
	}

	switch dialog.RunModal() {
	case platform.ResponseYes:
		fd := ss.Platform.CreateOpenFileDialog(ss.GW.window)
		key := "LinkSketch"
		if isImage {
			key = "LinkImage"
			fd.AddFilters(platform.RasterFileFilters)
		} else {
			fd.AddFilters(platform.LinkFileFilters)
		}
		fd.ThawChoices(ss.Settings, key)
		fd.SuggestFilename(name)
		if fd.RunModal() {
			fd.FreezeChoices(ss.Settings, key)
			return fd.GetFilename(), link.LocateYes
		}
		if canCancel {
			return "", link.LocateCancel
		}
		return "", link.LocateGaveUp
	case platform.ResponseNo:
		return "", link.LocateNo
	default:
		return "", link.LocateCancel
	}
}

// chooseLinkedImage asks for a picture for a new image request and
// loads it (C++ SolveSpaceUI::ReloadLinkedImage with an empty name).
func (ss *Application) chooseLinkedImage() (string, bool) {
	dialog := ss.Platform.CreateOpenFileDialog(ss.GW.window)
	dialog.AddFilters(platform.RasterFileFilters)
	dialog.ThawChoices(ss.Settings, "LinkImage")
	if !dialog.RunModal() {
		return "", false
	}
	dialog.FreezeChoices(ss.Settings, "LinkImage")
	filename := dialog.GetFilename()
	if !link.LoadLinkedImage(ss.SK, ss.SaveFile, filename) {
		ss.Error("The image '%s' is corrupted.", filename)
	}
	return filename, true
}

// Exit saves the settings and quits (C++ SolveSpaceUI::Exit).
func (ss *Application) Exit() {
	ss.SaveSettings()
	// The socket goes with the program; the setting stays.
	_ = ss.AllowAgent(false)
	ss.Platform.Exit()
}

// MenuFile runs a File menu command (C++ SolveSpaceUI::MenuFile).
func (ss *Application) MenuFile(id Command) {
	settings := ss.Settings

	switch id {
	case CmdNew:
		if !ss.OkayToStartNewFile() {
			break
		}
		ss.SaveFile = ""
		ss.NewFile()
		ss.AfterNewFile()

	case CmdOpen:
		if !ss.OkayToStartNewFile() {
			break
		}
		dialog := ss.Platform.CreateOpenFileDialog(ss.GW.window)
		dialog.AddFilters(platform.ModelFileFilters)
		dialog.ThawChoices(settings, "Sketch")
		if dialog.RunModal() {
			dialog.FreezeChoices(settings, "Sketch")
			ss.Load(dialog.GetFilename())
		}

	case CmdSave:
		ss.GetFilenameAndSave(false)

	case CmdSaveAs:
		ss.GetFilenameAndSave(true)

	case CmdExportImage, CmdExportView, CmdExportWireframe, CmdExportSection,
		CmdExportMesh, CmdExportSurfaces, CmdImport:
		ss.menuExport(id)

	case CmdExit:
		if !ss.OkayToStartNewFile() {
			break
		}
		ss.Exit()

	default:
		panic("ui: unexpected menu ID")
	}

	ss.UpdateWindowTitles()
}

// menuExport runs the File menu's export and import commands (those
// cases of C++ SolveSpaceUI::MenuFile).
func (ss *Application) menuExport(id Command) {
	settings := ss.Settings
	gw := ss.GW

	// saveDialog asks where to save, remembering the folder and type
	// under key.
	saveDialog := func(filters []platform.FileFilter, key string) (string, bool) {
		dialog := ss.Platform.CreateSaveFileDialog(gw.window)
		dialog.AddFilters(filters)
		dialog.ThawChoices(settings, key)
		dialog.SuggestFilename(ss.SaveFile)
		if !dialog.RunModal() {
			return "", false
		}
		dialog.FreezeChoices(settings, key)
		return dialog.GetFilename(), true
	}

	switch id {
	case CmdExportImage:
		if filename, ok := saveDialog(platform.RasterFileFilters, "ExportImage"); ok {
			ss.ExportAsPngTo(filename)
		}

	case CmdExportView:
		filename, ok := saveDialog(platform.VectorFileFilters, "ExportView")
		if !ok {
			break
		}

		// If the user is exporting something where it would be
		// inappropriate to include the constraints, then warn.
		if gw.showConstraints != SCMNoShow &&
			(hasExtension(filename, "txt") || math.Abs(ss.ExportOffset) > core.LengthEps) {
			ss.Message("Constraints are currently shown, and will be exported " +
				"in the toolpath. This is probably not what you want; " +
				"hide them by clicking the link at the top of the " +
				"text window.")
		}

		ss.ExportViewOrWireframeTo(filename, false)

	case CmdExportWireframe:
		if filename, ok := saveDialog(platform.Vector3dFileFilters, "ExportWireframe"); ok {
			ss.ExportViewOrWireframeTo(filename, true)
		}

	case CmdExportSection:
		if filename, ok := saveDialog(platform.VectorFileFilters, "ExportSection"); ok {
			ss.ExportSectionTo(filename)
		}

	case CmdExportMesh:
		if filename, ok := saveDialog(platform.MeshFileFilters, "ExportMesh"); ok {
			ss.ExportMeshTo(filename)
		}

	case CmdExportSurfaces:
		if filename, ok := saveDialog(platform.SurfaceFileFilters, "ExportSurfaces"); ok {
			ss.ExportSurfacesTo(filename)
		}

	case CmdImport:
		dialog := ss.Platform.CreateOpenFileDialog(gw.window)
		dialog.AddFilters(platform.ImportFileFilters)
		dialog.ThawChoices(settings, "Import")
		if !dialog.RunModal() {
			break
		}
		dialog.FreezeChoices(settings, "Import")

		importFile := dialog.GetFilename()
		switch {
		case hasExtension(importFile, "dxf"):
			ss.ImportDxf(importFile)
		case hasExtension(importFile, "dwg"):
			ss.ImportDwg(importFile)
		case hasExtension(importFile, "slvs"):
			// A whole model: it replaces the sketch.
			ss.ImportSolveSpace(importFile)
			return
		default:
			ss.Error("Can't identify file type from file extension of "+
				"filename '%s'; try .dxf, .dwg or .slvs.", importFile)
			return
		}

		ss.GenerateAll(sketch.GenerateUntilActive)
		ss.ScheduleShowTW()
	}
}

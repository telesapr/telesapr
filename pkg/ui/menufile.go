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

// backupExt is the autosave file extension.
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

// withExtension replaces the extension of path's file name with ext.
func withExtension(path, ext string) string {
	dir, base := filepath.Split(path)
	if dot := strings.LastIndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	return dir + base + "." + ext
}

// hasExtension reports whether path's extension is ext, ignoring case.
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
// loads it when the user agrees; loaded says whether it did.
func (ss *Application) LoadAutosaveFor(filename string) (loaded bool, err error) {
	autosaveFile := withExtension(filename, backupExt)
	if _, err := os.Stat(autosaveFile); err != nil {
		return false, nil //nolint:nilerr // without an autosave there is nothing to load
	}

	dialog := ss.Platform.CreateMessageDialog(ss.GW.window)
	dialog.SetType(platform.DialogQuestion)
	dialog.SetTitle("Autosave Available")
	dialog.SetMessage("An autosave file is available for this sketch.")
	dialog.SetDescription("Do you want to load the autosave file instead?")
	dialog.AddButton("&Load autosave", platform.ResponseYes, true)
	dialog.AddButton("Do&n't Load", platform.ResponseNo, false)

	if dialog.RunModal() == platform.ResponseYes {
		if err := ss.LoadFromFile(autosaveFile); err != nil {
			return false, err
		}
		ss.SK.Unsaved = true
		return true, nil
	}
	return false, nil
}

// Load opens filename, or its autosave, as the sketch being edited; a
// file that can't be read leaves a new sketch, and the error says why.
func (ss *Application) Load(filename string) error {
	// The links of the sketch are relative to the file it is read from,
	// and reading it reads them, so the name is put in place before,
	// not after: what reads them again while the sketch is being set up
	// must know the file to read them relative to.
	ss.SaveFile = filename
	autosaveLoaded, err := ss.LoadAutosaveFor(filename)
	if !autosaveLoaded {
		ss.Report(err)
		err = ss.LoadFromFile(filename)
	}
	if err == nil {
		ss.SaveFile = filename
		ss.AddToRecentList(filename)
	} else {
		ss.SaveFile = ""
		ss.NewFile()
	}
	ss.AfterNewFile()
	ss.SK.Unsaved = autosaveLoaded
	return err
}

// LoadFromFile reads filename into a new sketch; the error is what to
// tell the user, errDeclined when they gave up at a missing linked
// file. What was left out of the file is shown as it is read.
func (ss *Application) LoadFromFile(filename string) error {
	ss.AllConsistent = false

	if _, err := os.Stat(filename); err != nil {
		return fmt.Errorf("couldn't read from file '%s'", filename)
	}
	if hasExtension(filename, "slvs") {
		return fmt.Errorf("'%s' is a SolveSpace model. Use File > Import, or telesapr-cli "+
			"convert, to convert it to a TeleSAPR model, which can then be opened", filename)
	}

	doc, err := tlspr.LoadFile(filename)
	var ve *tlspr.VersionError
	switch {
	case errors.Is(err, tlspr.ErrCancelled):
		// The user gave up at a missing linked file.
		return errDeclined
	case errors.As(err, &ve):
		return fmt.Errorf("the file '%s' was saved by a newer version of the program, in format "+
			"version %d. This version reads files up to version %d", filename, ve.File, ve.Reader)
	case err != nil:
		return fmt.Errorf("unrecognized data in file. This file may be corrupt, or "+
			"from a newer version of the program.\n\n%v", err)
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
	return nil
}

// SaveToFile writes the sketch to filename; the error is what to tell
// the user.
func (ss *Application) SaveToFile(filename string) error {
	if hasExtension(filename, "slvs") {
		return errors.New("a SolveSpace model can't be written; save the sketch as a " +
			".tlspr file")
	}

	// Make sure all the entities are regenerated up to date, since they
	// will be exported.
	ss.ScheduleShowTW()
	ss.GenerateAll(sketch.GenerateAll)

	err := tlspr.SaveFile(ss.SK, ss.SaveFile, filename, tlspr.Options{})
	var le *tlspr.LinkError
	switch {
	case errors.As(err, &le):
		return fmt.Errorf("this sketch links the sketch '%s'; it can only be saved "+
			"on the same volume", le.Link)
	case err != nil:
		return fmt.Errorf("couldn't write to file '%s'", filename)
	}
	return nil
}

// AddToRecentList puts filename first in the recent files.
func (ss *Application) AddToRecentList(filename string) {
	for i, p := range ss.RecentFiles {
		if p == filename {
			ss.RecentFiles = append(ss.RecentFiles[:i], ss.RecentFiles[i+1:]...)
			break
		}
	}
	if len(ss.RecentFiles) > maxRecent {
		ss.RecentFiles = append(ss.RecentFiles[:maxRecent], ss.RecentFiles[maxRecent+1:]...)
	}
	ss.RecentFiles = append([]string{filename}, ss.RecentFiles...)
	ss.GW.PopulateRecentFiles()
}

// SaveToChosenFile saves the sketch, asking for a file name for a new
// sketch or for Save As; errDeclined when the user gave up.
func (ss *Application) SaveToChosenFile(saveAs bool) error {
	newSaveFile := ss.SaveFile

	if saveAs || newSaveFile == "" {
		dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
		dialog.AddFilters(platform.ModelFileFilters)
		dialog.ThawChoices(ss.Settings, "Sketch")
		if newSaveFile != "" {
			dialog.SetFilename(newSaveFile)
		}
		if !dialog.RunModal() {
			return errDeclined
		}
		dialog.FreezeChoices(ss.Settings, "Sketch")
		newSaveFile = modelFileName(dialog.Filename())
	}

	if err := ss.SaveToFile(newSaveFile); err != nil {
		return err
	}
	ss.AddToRecentList(newSaveFile)
	ss.RemoveAutosave()
	ss.SaveFile = newSaveFile
	ss.SK.Unsaved = false
	return nil
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

// Autosave saves a backup of an unsaved sketch next to its file.
func (ss *Application) Autosave() {
	ss.ScheduleAutosave()

	if ss.SaveFile != "" && ss.SK.Unsaved {
		ss.Report(ss.SaveToFile(withExtension(ss.SaveFile, backupExt)))
	}
}

// RemoveAutosave deletes the sketch's autosave file.
func (ss *Application) RemoveAutosave() {
	if ss.SaveFile == "" {
		return
	}
	_ = os.Remove(withExtension(ss.SaveFile, backupExt))
}

// OkayToStartNewFile asks whether to save unsaved changes, and reports
// whether the sketch may be replaced.
func (ss *Application) OkayToStartNewFile() bool {
	if !ss.SK.Unsaved {
		return true
	}
	if ss.messageSink != nil {
		// A script runs: a dialog would stop the loop under it, and
		// nobody is there to answer. The changes stay.
		ss.Error("The drawing has unsaved changes, which this would lose. A script saves them first with doc.Save, or lets them go with doc.File.ForgetChanges().")
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
		err := ss.SaveToChosenFile(false)
		ss.Report(err)
		return err == nil
	case platform.ResponseNo:
		ss.RemoveAutosave()
		return true
	default:
		return false
	}
}

// locateMissingFile asks whether to find a linked file that couldn't be
// read, and where it is.
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
			return fd.Filename(), link.LocateYes
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
// loads it.
func (ss *Application) chooseLinkedImage() (string, bool) {
	dialog := ss.Platform.CreateOpenFileDialog(ss.GW.window)
	dialog.AddFilters(platform.RasterFileFilters)
	dialog.ThawChoices(ss.Settings, "LinkImage")
	if !dialog.RunModal() {
		return "", false
	}
	dialog.FreezeChoices(ss.Settings, "LinkImage")
	filename := dialog.Filename()
	if !link.LoadLinkedImage(ss.SK, ss.SaveFile, filename) {
		ss.Error("The image '%s' is corrupted.", filename)
	}
	return filename, true
}

// Exit saves the settings and quits.
func (ss *Application) Exit() {
	ss.SaveSettings()
	// The socket goes with the program; the setting stays.
	_ = ss.AllowAgent(false)
	ss.Platform.Exit()
}

// MenuFile runs a File menu command.
func (ss *Application) MenuFile(id Command) {
	settings := ss.Settings

	switch id {
	case cmdNew:
		if !ss.OkayToStartNewFile() {
			break
		}
		ss.SaveFile = ""
		ss.NewFile()
		ss.AfterNewFile()

	case cmdOpen:
		if !ss.OkayToStartNewFile() {
			break
		}
		dialog := ss.Platform.CreateOpenFileDialog(ss.GW.window)
		dialog.AddFilters(platform.ModelFileFilters)
		dialog.ThawChoices(settings, "Sketch")
		if dialog.RunModal() {
			dialog.FreezeChoices(settings, "Sketch")
			ss.Report(ss.Load(dialog.Filename()))
		}

	case cmdSave:
		ss.Report(ss.SaveToChosenFile(false))

	case cmdSaveAs:
		ss.Report(ss.SaveToChosenFile(true))

	case cmdExportImage, cmdExportView, cmdExportWireframe, cmdExportSection,
		cmdExportMesh, cmdExportSurfaces, cmdImport:
		ss.menuExport(id)

	case cmdExit:
		if !ss.OkayToStartNewFile() {
			break
		}
		ss.Exit()

	default:
		panic("ui: unexpected menu ID")
	}

	ss.UpdateWindowTitles()
}

// exportDialog asks where to export to, remembering the folder and
// type under key.
func (ss *Application) exportDialog(filters []platform.FileFilter, key string) (string, bool) {
	dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
	dialog.AddFilters(filters)
	dialog.ThawChoices(ss.Settings, key)
	dialog.SuggestFilename(ss.SaveFile)
	if !dialog.RunModal() {
		return "", false
	}
	dialog.FreezeChoices(ss.Settings, key)
	return dialog.Filename(), true
}

// menuExport runs the File menu's export and import commands.
func (ss *Application) menuExport(id Command) {
	switch id {
	case cmdExportImage:
		if filename, ok := ss.exportDialog(platform.RasterFileFilters, "ExportImage"); ok {
			ss.ExportAsPngTo(filename)
		}

	case cmdExportView:
		ss.menuExportView()

	case cmdExportWireframe:
		if filename, ok := ss.exportDialog(platform.Vector3dFileFilters, "ExportWireframe"); ok {
			ss.Report(ss.ExportViewOrWireframeTo(filename, true))
		}

	case cmdExportSection:
		if filename, ok := ss.exportDialog(platform.VectorFileFilters, "ExportSection"); ok {
			ss.Report(ss.ExportSectionTo(filename))
		}

	case cmdExportMesh:
		ss.TW.GoToScreen(screenExportMesh)
		ss.GW.ForceTextWindowShown()
		ss.ScheduleShowTW()

	case cmdExportSurfaces:
		if filename, ok := ss.exportDialog(platform.SurfaceFileFilters, "ExportSurfaces"); ok {
			ss.Report(ss.ExportSurfacesTo(filename))
		}

	case cmdImport:
		ss.menuImport()
	}
}

// menuExportView runs the File menu's cmdExportView.
func (ss *Application) menuExportView() {
	gw := ss.GW

	filename, ok := ss.exportDialog(platform.VectorFileFilters, "ExportView")
	if !ok {
		return
	}

	// If the user is exporting something where it would be
	// inappropriate to include the constraints, then warn.
	if gw.showConstraints != sCMNoShow &&
		(hasExtension(filename, "txt") || math.Abs(ss.ExportOffset) > core.LengthEps) {
		ss.Message("Constraints are currently shown, and will be exported " +
			"in the toolpath. This is probably not what you want; " +
			"hide them by clicking the link at the top of the " +
			"text window.")
	}

	ss.Report(ss.ExportViewOrWireframeTo(filename, false))
}

// menuImport runs the File menu's cmdImport.
func (ss *Application) menuImport() {
	settings := ss.Settings
	gw := ss.GW

	dialog := ss.Platform.CreateOpenFileDialog(gw.window)
	dialog.AddFilters(platform.ImportFileFilters)
	dialog.ThawChoices(settings, "Import")
	if !dialog.RunModal() {
		return
	}
	dialog.FreezeChoices(settings, "Import")

	importFile := dialog.Filename()
	switch {
	case hasExtension(importFile, "dxf"):
		ss.ImportDxf(importFile)
	case hasExtension(importFile, "dwg"):
		ss.ImportDwg(importFile)
	case hasExtension(importFile, "slvs"):
		// A whole model: it replaces the sketch.
		ss.Report(ss.ImportSolveSpace(importFile))
		return
	default:
		ss.Error("Can't identify file type from file extension of "+
			"filename '%s'; try .dxf, .dwg or .slvs.", importFile)
		return
	}

	ss.GenerateAll(sketch.GenerateUntilActive)
	ss.ScheduleShowTW()
}

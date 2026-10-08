package ui

// File > Import of a SolveSpace model: the .slvs file, and the .slvs
// files it links, converted to TeleSAPR models (pkg/persist/convert).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/persist/dxf"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/platform"
)

// ImportSolveSpace converts the SolveSpace model src to a TeleSAPR
// model and opens it. It asks where to save the converted model, then
// shows every file the import will write before it writes any, and,
// when converted copies of linked models exist already, asks once
// whether to reuse or overwrite them. A linked model that is missing is
// asked for as when a model is opened; ErrDeclined when the user gave
// up at one of the questions.
func (ss *Application) ImportSolveSpace(src string) error {
	if !ss.OkayToStartNewFile() {
		return ErrDeclined
	}

	dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
	dialog.SetTitle("Save the Imported Model")
	dialog.AddFilters(platform.ModelFileFilters)
	dialog.ThawChoices(ss.Settings, "Sketch")
	dialog.SetFilename(withExtension(src, tlspr.Extension))
	if !dialog.RunModal() {
		return ErrDeclined
	}
	dialog.FreezeChoices(ss.Settings, "Sketch")
	dst := modelFileName(dialog.Filename())

	plan, err := convert.MakePlan(src, dst)
	if err != nil {
		return fmt.Errorf("can't import '%s': %w", src, err)
	}

	ask := ss.Platform.CreateMessageDialog(ss.GW.window)
	ask.SetType(platform.DialogQuestion)
	ask.SetTitle("Import SolveSpace Model")
	ask.SetMessage("The import will write these files:")
	ask.SetDescription(describePlan(plan))
	overwrite := false
	if len(plan.Existing()) > 0 {
		ask.AddButton("&Overwrite", platform.ResponseYes, false)
		ask.AddButton("&Reuse", platform.ResponseNo, true)
		ask.AddButton("&Cancel", platform.ResponseCancel, false)
		switch ask.RunModal() {
		case platform.ResponseYes:
			overwrite = true
		case platform.ResponseNo:
		default:
			return ErrDeclined
		}
	} else {
		ask.AddButton("&Import", platform.ResponseYes, true)
		ask.AddButton("&Cancel", platform.ResponseCancel, false)
		if ask.RunModal() != platform.ResponseYes {
			return ErrDeclined
		}
	}

	res, err := convert.Run(plan, convert.Options{Overwrite: overwrite, Units: ss.ViewUnits})
	if errors.Is(err, convert.ErrCancelled) {
		return ErrDeclined
	}
	if err != nil {
		return fmt.Errorf("can't import '%s': %w", src, err)
	}
	if report := describeResult(res); report != "" {
		ss.Error("The import of '%s' was not complete.\n\n%s", src, report)
	}
	ss.SaveFile = ""
	return ss.Load(dst)
}

// describePlan lists the files a plan writes, the linked copies that
// exist already, and the linked models that are missing.
func describePlan(p *convert.Plan) string {
	var b strings.Builder
	for _, it := range p.Items {
		if it.Missing {
			continue
		}
		fmt.Fprintf(&b, "%s\n", it.Dst)
	}
	if existing := p.Existing(); len(existing) > 0 {
		b.WriteString("\nThese converted models exist already. Reuse them as they are, " +
			"or convert them again and overwrite them?\n")
		for _, it := range existing {
			fmt.Fprintf(&b, "%s\n", it.Dst)
		}
	}
	if missing := p.Missing(); len(missing) > 0 {
		b.WriteString("\nThese linked models are missing; you will be asked where they are:\n")
		for _, it := range missing {
			fmt.Fprintf(&b, "%s, linked from %s\n", it.Src, filepath.Base(it.LinkedFrom))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// describeResult lists what failed and the warnings of an import, or
// is empty.
func describeResult(r *convert.Result) string {
	var lines []string
	for path, err := range r.Failed {
		lines = append(lines, fmt.Sprintf("Not written: %s: %v", path, err))
	}
	slices.Sort(lines)
	lines = append(lines, r.Warnings...)
	return warningList(lines)
}

// ImportDxf imports a DXF file into the active group: in its workplane,
// or as construction entities in 3d when the file leaves the plane.
func (ss *Application) ImportDxf(filename string) {
	fileType := strings.ToUpper(strings.TrimPrefix(filepath.Ext(filename), "."))

	data, err := os.ReadFile(filename)
	if err != nil {
		ss.Error("Couldn't read from '%s'", filename)
		return
	}

	asConstruction := true
	if ss.GW.LockedInWorkplane() {
		if dxf.Has3D(data) {
			ss.Message("This %s file contains entities with non-zero Z coordinate; "+
				"the entire file will be imported as construction entities in 3d.", fileType)
			ss.GW.SetWorkplaneFreeIn3d()
			ss.GW.EnsureValidActives()
		} else {
			asConstruction = false
		}
	}

	ss.UndoRemember()
	rep, err := dxf.Import(ss.SK, data, dxf.Options{AsConstruction: asConstruction, TextHeight: ss.defaultTextHeight()})
	if err != nil {
		ss.Error("Corrupted %s file.", fileType)
		return
	}
	if rep.Unknown > 0 {
		ss.Message("%d %s entities of unknown type were ignored.", rep.Unknown, fileType)
	}
}

// ImportDwg would import a DWG file; reading DWG isn't ported.
func (ss *Application) ImportDwg(filename string) {
	ss.Error("DWG import is not supported; save the drawing as DXF.")
}

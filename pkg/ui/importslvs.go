package ui

// File > Import of a SolveSpace model: the .slvs file, and the .slvs
// files it links, converted to spacesolve models (pkg/persist/convert).

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spacesolve/spacesolve/pkg/persist/convert"
	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
	"github.com/spacesolve/spacesolve/pkg/platform"
)

// ImportSolveSpace converts the SolveSpace model src to a spacesolve
// model and opens it. It asks where to save the converted model, then
// shows every file the import will write before it writes any, and,
// when converted copies of linked models exist already, asks once
// whether to reuse or overwrite them. A linked model that is missing is
// asked for as when a model is opened. It reports whether the model
// was imported and opened.
func (ss *SolveSpaceUI) ImportSolveSpace(src string) bool {
	if !ss.OkayToStartNewFile() {
		return false
	}

	dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
	dialog.SetTitle("Save the Imported Model")
	dialog.AddFilters(platform.ModelFileFilters)
	dialog.ThawChoices(ss.Settings, "Sketch")
	dialog.SetFilename(withExtension(src, spcs.Extension))
	if !dialog.RunModal() {
		return false
	}
	dialog.FreezeChoices(ss.Settings, "Sketch")
	dst := modelFileName(dialog.GetFilename())

	plan, err := convert.MakePlan(src, dst)
	if err != nil {
		ss.Error("Can't import '%s': %v", src, err)
		return false
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
			return false
		}
	} else {
		ask.AddButton("&Import", platform.ResponseYes, true)
		ask.AddButton("&Cancel", platform.ResponseCancel, false)
		if ask.RunModal() != platform.ResponseYes {
			return false
		}
	}

	res, err := convert.Run(plan, convert.Options{Overwrite: overwrite, Units: ss.ViewUnits})
	if errors.Is(err, convert.ErrCancelled) {
		return false
	}
	if err != nil {
		ss.Error("Can't import '%s': %v", src, err)
		return false
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
	sort.Strings(lines)
	lines = append(lines, r.Warnings...)
	return warningList(lines)
}

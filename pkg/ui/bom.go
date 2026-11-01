package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// bomTextCols is the width of a part's file and groups on the bill of
// materials, the text window's less their indent.
const bomTextCols = maxCols - 10

// wrapPath breaks a file's name into lines of at most width characters,
// after a folder separator where it can, so that each line but the last
// ends with a whole folder's name.
func wrapPath(path string, width int) []string {
	var lines []string
	r := []rune(path)
	for len(r) > width {
		cut := width
		for cut > width/2 && r[cut-1] != '/' && r[cut-1] != filepath.Separator {
			cut--
		}
		if cut == width/2 {
			cut = width
		}
		lines = append(lines, string(r[:cut]))
		r = r[cut:]
	}
	return append(lines, string(r))
}

// ShowBillOfMaterials lists the parts the model places by linking files,
// with a link to write them to a CSV file (roadmap 16.7). A file is
// named relative to the model's file, as a link group's screen names
// it; a name or a list of groups too long for a line goes on to the
// next.
func (tw *textWindow) ShowBillOfMaterials() {
	items := tw.ss.SK.BillOfMaterials(tw.ss.SaveFile)
	tw.Printf(true, "%FtBILL OF MATERIALS%E")
	if len(items) == 0 {
		tw.Printf(false, "%Ba   (none: no group links a part)")
	} else {
		tw.Printf(false, "%Ft   count  part%E")
	}
	for i, it := range items {
		bg := byte('a')
		if i%2 == 1 {
			bg = 'd'
		}
		tw.Printf(false, "%Bp   %s  %s", bg, fmt.Sprintf("%5d", it.Count), it.Part)
		for _, line := range wrapPath(it.File, bomTextCols) {
			tw.Printf(false, "%Bp          %Fd%s%E", bg, line)
		}
		for _, line := range wrapWords("by "+strings.Join(it.Groups, ", "), bomTextCols) {
			tw.Printf(false, "%Bp          %Fd%s%E", bg, line)
		}
		if it.Mass > 0 {
			tw.Printf(false, "%Bp          %Fd%s each%E", bg, massString(it.Mass))
		}
	}
	if len(items) > 0 {
		tw.Printf(true, "%Fl%Lc%f[export to a CSV file...]%E", lf(tw.ScreenExportBOM))
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}

// ScreenExportBOM asks where, and writes the bill of materials there.
func (tw *textWindow) ScreenExportBOM(link int, v uint32) {
	ss := tw.ss
	dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
	dialog.AddFilters(platform.CsvFileFilters)
	dialog.ThawChoices(ss.Settings, "BillOfMaterials")
	dialog.SetFilename(ss.SaveFile)
	if !dialog.RunModal() {
		return
	}
	dialog.FreezeChoices(ss.Settings, "BillOfMaterials")
	var b bytes.Buffer
	if err := sketch.WriteBOMCSV(&b, ss.SK.BillOfMaterials(ss.SaveFile)); err != nil {
		ss.Report(err)
		return
	}
	if err := os.WriteFile(dialog.Filename(), b.Bytes(), 0o644); err != nil {
		ss.Error("Couldn't write to '%s'", dialog.Filename())
	}
}

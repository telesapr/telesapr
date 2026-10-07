package ui

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// ShowBillOfMaterials lists the parts the model places by linking files,
// with a link to write them to a CSV file (roadmap 16.7).
func (tw *TextWindow) ShowBillOfMaterials() {
	items := tw.ss.SK.BillOfMaterials()
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
		tw.Printf(false, "%Bp          %Fd%s%E", bg, it.File)
		tw.Printf(false, "%Bp          %Fdby %s%E", bg, strings.Join(it.Groups, ", "))
	}
	if len(items) > 0 {
		tw.Printf(true, "%Fl%Lc%f[export to a CSV file...]%E", lf(tw.ScreenExportBOM))
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}

// ScreenExportBOM asks where, and writes the bill of materials there.
func (tw *TextWindow) ScreenExportBOM(link int, v uint32) {
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
	if err := sketch.WriteBOMCSV(&b, ss.SK.BillOfMaterials()); err != nil {
		ss.Error("%s.", capitalize(err.Error()))
		return
	}
	if err := os.WriteFile(dialog.Filename(), b.Bytes(), 0o644); err != nil {
		ss.Error("Couldn't write to '%s'", dialog.Filename())
	}
}

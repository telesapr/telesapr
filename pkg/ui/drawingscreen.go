package ui

import (
	"strconv"
	"strings"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The drawing's screen (roadmap 18.1, 18.2): the sheet, its scale and
// projection, whether hidden edges are drawn, and the title block, each
// with a link to change it, and the export. A change is a step of undo.

// sheetNames are the sheets in the order the screen's link goes
// through them.
var sheetNames = []string{"A4", "A3", "A2", "A1", "A0", "Letter", "Tabloid"}

// drawing is the model's drawing, made the default one if it has none.
func (ss *SolveSpaceUI) drawing() *sketch.Drawing {
	if ss.SK.Drawing == nil {
		ss.SK.Drawing = sketch.DefaultDrawing()
	}
	return ss.SK.Drawing
}

// MenuDrawingSheet shows the drawing's screen.
func (ss *SolveSpaceUI) MenuDrawingSheet() {
	ss.GW.ClearSelection()
	ss.TW.GoToScreen(ScreenDrawing)
	ss.GW.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// MenuExportDrawing asks where, and writes the drawing there.
func (ss *SolveSpaceUI) MenuExportDrawing() {
	dialog := ss.Platform.CreateSaveFileDialog(ss.GW.window)
	dialog.AddFilters(platform.VectorFileFilters)
	dialog.ThawChoices(ss.Settings, "ExportDrawing")
	dialog.SuggestFilename(ss.SaveFile)
	if !dialog.RunModal() {
		return
	}
	dialog.FreezeChoices(ss.Settings, "ExportDrawing")
	d := ss.SK.Drawing
	if d == nil {
		d = sketch.DefaultDrawing()
	}
	if err := ss.ExportDrawingTo(dialog.GetFilename(), d); err != nil {
		ss.Error("Can't export the drawing: %s.", err.Error())
	}
}

// ShowDrawing is the drawing's screen.
func (tw *TextWindow) ShowDrawing() {
	d := tw.ss.SK.Drawing
	if d == nil {
		d = sketch.DefaultDrawing()
	}
	radio := func(b bool) string {
		if b {
			return radioTrue
		}
		return radioFalse
	}
	check := func(b bool) string {
		if b {
			return checkTrue
		}
		return checkFalse
	}
	tw.Printf(true, "%FtDRAWING%E   (File > Export Drawing writes it)")
	w, h := d.Size()
	tw.Printf(true, "%Ba   %Ftsheet%E      %s, %s x %s mm %Fl%Ls%f[next]%E",
		d.Sheet, strconv.FormatFloat(w, 'g', -1, 64), strconv.FormatFloat(h, 'g', -1, 64), lf(tw.ScreenChangeDrawing))
	tw.Printf(false, "%Bd   %f%Lo%Fd%s landscape%E  %f%LO%Fd%s portrait%E",
		lf(tw.ScreenChangeDrawing), radio(!d.Portrait), lf(tw.ScreenChangeDrawing), radio(d.Portrait))
	scale := "fitted to the sheet"
	if d.Scale != 0 {
		scale = sketch.ScaleText(d.Scale)
	}
	tw.Printf(false, "%Ba   %Ftscale%E      %s %Fl%Lc%f[change]%E", scale, lf(tw.ScreenChangeDrawing))
	tw.Printf(false, "%Bd   %f%Lt%Fd%s third angle%E  %f%Lf%Fd%s first angle%E",
		lf(tw.ScreenChangeDrawing), radio(!d.FirstAngle), lf(tw.ScreenChangeDrawing), radio(d.FirstAngle))
	tw.Printf(false, "%Ba   %f%Lh%Fd%s hidden edges, dashed%E", lf(tw.ScreenChangeDrawing), check(d.Hidden))

	tw.Printf(true, "%Ft   title block%E")
	fields := []struct {
		link  int
		label string
		value string
	}{
		{'1', "title", d.Title}, {'2', "drawn by", d.Author}, {'3', "drawing no.", d.Number},
		{'4', "revision", d.Revision}, {'5', "material", d.Material}, {'6', "company", d.Company},
	}
	for i, f := range fields {
		bg := byte('a')
		if i%2 == 1 {
			bg = 'd'
		}
		v := f.value
		if v == "" {
			v = "-"
		}
		tw.Printf(false, "%Bp   %s %s %Fl%Lp%f[change]%E", bg, pad(f.label, 12), v, f.link, lf(tw.ScreenChangeDrawing))
	}
	tw.Printf(true, "(%Fl%Ll%fback to the groups%E)", lf(tw.ScreenHome))
}

// ScreenChangeDrawing changes the drawing: the next sheet ('s'),
// landscape or portrait ('o', 'O'), the scale ('c'), the projection
// ('t', 'f'), hidden edges ('h'), or a field of the title block ('1' to
// '6').
func (tw *TextWindow) ScreenChangeDrawing(link int, v uint32) {
	ss := tw.ss
	change := func(fn func(d *sketch.Drawing)) {
		ss.UndoRemember()
		fn(ss.drawing())
		ss.SK.Unsaved = true
		ss.ScheduleShowTW()
	}
	switch link {
	case 's':
		change(func(d *sketch.Drawing) {
			i := 0
			for k, n := range sheetNames {
				if strings.EqualFold(n, d.Sheet) {
					i = k
				}
			}
			d.Sheet = sheetNames[(i+1)%len(sheetNames)]
		})
	case 'o', 'O':
		change(func(d *sketch.Drawing) { d.Portrait = link == 'O' })
	case 't', 'f':
		change(func(d *sketch.Drawing) { d.FirstAngle = link == 'f' })
	case 'h':
		change(func(d *sketch.Drawing) { d.Hidden = !d.Hidden })
	case 'c':
		d := ss.SK.Drawing
		text := "0"
		if d != nil && d.Scale != 0 {
			text = strconv.FormatFloat(d.Scale, 'g', -1, 64)
		}
		tw.ShowEditControl(12, text)
		tw.edit.meaning = EditDrawingScale
	case '1', '2', '3', '4', '5', '6':
		d := ss.drawing()
		text := []string{d.Title, d.Author, d.Number, d.Revision, d.Material, d.Company}[link-'1']
		tw.ShowEditControl(18, text)
		tw.edit.meaning = EditDrawingField
		tw.edit.i = link - '1'
	}
}

// editDrawing takes an edit of the drawing's screen, and reports
// whether the edit was one of those.
func (tw *TextWindow) editDrawing(s string) bool {
	ss := tw.ss
	switch tw.edit.meaning {
	case EditDrawingScale:
		// "1:2", "2:1" or a number; 0 fits the sheet.
		var scale float64
		var err error
		if a, b, ok := strings.Cut(strings.TrimSpace(s), ":"); ok {
			var x, y float64
			if x, err = strconv.ParseFloat(strings.TrimSpace(a), 64); err == nil {
				if y, err = strconv.ParseFloat(strings.TrimSpace(b), 64); err == nil && y != 0 {
					scale = x / y
				}
			}
		} else {
			scale, err = strconv.ParseFloat(strings.TrimSpace(s), 64)
		}
		if err != nil || scale < 0 {
			ss.Error("A scale is written 1:2, 2:1 or as a number; 0 fits the sheet.")
			return true
		}
		ss.UndoRemember()
		ss.drawing().Scale = scale
	case EditDrawingField:
		ss.UndoRemember()
		d := ss.drawing()
		v := strings.TrimSpace(s)
		switch tw.edit.i {
		case 0:
			d.Title = v
		case 1:
			d.Author = v
		case 2:
			d.Number = v
		case 3:
			d.Revision = v
		case 4:
			d.Material = v
		case 5:
			d.Company = v
		}
	default:
		return false
	}
	ss.SK.Unsaved = true
	ss.ScheduleShowTW()
	return true
}

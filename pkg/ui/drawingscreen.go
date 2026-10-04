package ui

import (
	"fmt"
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
// What is selected stays, for dimensions.
func (ss *SolveSpaceUI) MenuDrawingSheet() {
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
	show := "[show it in the window]"
	if tw.ss.GW.sheetShown {
		show = "[show the model]"
	}
	tw.Printf(true, "%FtDRAWING%E   (File > Export Drawing writes it)  %Fl%Lw%f%s%E", lf(tw.ScreenChangeDrawing), show)
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

	// The section and detail views (roadmap 18.3), lettered in order.
	tw.Printf(true, "%Ft   sections and details%E")
	n := 0
	num := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	for i, sec := range d.Sections {
		tw.Printf(false, "%Bp   %s: section of the %s view, %s mm toward it %Fl%Lx%D%f[delete]%E",
			byte('a'+n%2*3), sketch.Letter(n), sec.View, num(sec.Offset), i, lf(tw.ScreenChangeDrawing))
		n++
	}
	for i, dt := range d.Details {
		tw.Printf(false, "%Bp   %s: detail of the %s view at %s, %s, radius %s, %s times %Fl%LX%D%f[delete]%E",
			byte('a'+n%2*3), sketch.Letter(n), dt.View, num(dt.CX), num(dt.CY), num(dt.R), num(dt.Scale),
			i, lf(tw.ScreenChangeDrawing))
		n++
	}
	tw.Printf(false, "   %Fl%La%f[add a section]%E   (type: view offset, as front 0)", lf(tw.ScreenChangeDrawing))
	tw.Printf(false, "   %Fl%Ld%f[add a detail]%E   (type: view x y radius times, as front 10 5 8 2)",
		lf(tw.ScreenChangeDrawing))

	// The dimensions (roadmap 18.4, 29.2), measured as the solid is.
	tw.Printf(true, "%Ft   dimensions%E")
	var results []DimensionResult
	if len(d.Dimensions) > 0 {
		results = tw.ss.DrawingDimensions(d)
	}
	for i, dm := range d.Dimensions {
		bg := byte('a' + i%2*3)
		value := "?"
		if i < len(results) && results[i].Err == nil {
			value = formatValue(results[i].Value)
		}
		var of []string
		if len(dm.Edges) > 0 {
			of = append(of, fmt.Sprintf("%d edge%s", len(dm.Edges), map[bool]string{true: "s"}[len(dm.Edges) > 1]))
		}
		if len(dm.Faces) > 0 {
			of = append(of, fmt.Sprintf("%d face%s", len(dm.Faces), map[bool]string{true: "s"}[len(dm.Faces) > 1]))
		}
		on := "the " + dm.View + " view"
		if n, _ := d.DimensionView(dm.View); n >= 0 {
			on = "view " + dm.View
		}
		tw.Printf(false, "%Bp   %s of %s on %s: %s %Fl%Lr%D%f[delete]%E",
			bg, dm.Kind, strings.Join(of, " and "), on, value, i, lf(tw.ScreenChangeDrawing))
		if i < len(results) && results[i].Err != nil {
			tw.Printf(false, "%Bp     %Fx%s%E", bg, results[i].Err.Error())
		}
	}
	tw.Printf(false, "   %Fl%Lm%f[dimension the selected edges or faces]%E   (type: view kind, as front length)",
		lf(tw.ScreenChangeDrawing))
	tw.ss.GW.GroupSelection()
	if ne, nf := len(tw.ss.GW.gs.solidEdge), len(tw.ss.GW.selectedFaceIDs()); ne+nf > 0 {
		tw.Printf(false, "     %Fd%d edge%s and %d face%s selected%E", ne, map[bool]string{true: "s"}[ne != 1],
			nf, map[bool]string{true: "s"}[nf != 1])
	}

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
	case 'w':
		ss.GW.MenuShowDrawingSheet()
		ss.ScheduleShowTW()
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
	case 'x':
		change(func(d *sketch.Drawing) {
			if int(v) < len(d.Sections) {
				d.Sections = append(d.Sections[:v], d.Sections[v+1:]...)
			}
		})
	case 'X':
		change(func(d *sketch.Drawing) {
			if int(v) < len(d.Details) {
				d.Details = append(d.Details[:v], d.Details[v+1:]...)
			}
		})
	case 'r':
		change(func(d *sketch.Drawing) {
			if int(v) < len(d.Dimensions) {
				d.Dimensions = append(d.Dimensions[:v], d.Dimensions[v+1:]...)
			}
		})
	case 'm':
		ss.GW.GroupSelection()
		if n := len(ss.GW.gs.solidEdge) + len(ss.GW.selectedFaceIDs()); n < 1 || n > 2 {
			ss.Error("Select one edge of the solid for a length, one edge or face for a diameter or a radius, " +
				"or two edges or flat faces for the distance between them, then dimension them on a view " +
				"of the drawing, a section or a detail.")
			return
		}
		tw.ShowEditControl(18, "front length")
		tw.edit.meaning = EditDrawingDimension
	case 'a':
		tw.ShowEditControl(18, "front 0")
		tw.edit.meaning = EditDrawingSection
	case 'd':
		tw.ShowEditControl(18, "front 0 0 10 2")
		tw.edit.meaning = EditDrawingDetail
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
	case EditDrawingSection, EditDrawingDetail:
		f := strings.Fields(s)
		var nums []float64
		for _, w := range f[min(1, len(f)):] {
			x, err := strconv.ParseFloat(w, 64)
			if err != nil {
				nums = nil
				break
			}
			nums = append(nums, x)
		}
		d := ss.drawing().Clone()
		if tw.edit.meaning == EditDrawingSection {
			if len(f) != 2 || len(nums) != 1 {
				ss.Error("A section is typed as its view and its offset, as front 0.")
				return true
			}
			d.Sections = append(d.Sections, sketch.DrawingSection{View: f[0], Offset: nums[0]})
		} else {
			if len(f) != 5 || len(nums) != 4 {
				ss.Error("A detail is typed as its view, the x and y of its middle, its radius and how many times larger, as front 10 5 8 2.")
				return true
			}
			d.Details = append(d.Details, sketch.DrawingDetail{View: f[0], CX: nums[0], CY: nums[1], R: nums[2], Scale: nums[3]})
		}
		if err := d.Check(); err != nil {
			ss.Error("%s.", capitalize(err.Error()))
			return true
		}
		ss.UndoRemember()
		ss.SK.Drawing = d
	case EditDrawingDimension:
		f := strings.Fields(s)
		if len(f) < 2 || len(f) > 3 {
			ss.Error("A dimension is typed as its view and kind, as front length, and how far out, as front length 10.")
			return true
		}
		dm := sketch.DrawingDimension{View: f[0], Kind: f[1]}
		if len(f) == 3 {
			off, err := strconv.ParseFloat(f[2], 64)
			if err != nil {
				ss.Error("Not a number: '%s'.", f[2])
				return true
			}
			dm.Offset = off
		}
		ss.GW.GroupSelection()
		dm.Edges = append(dm.Edges, ss.GW.gs.solidEdge...)
		dm.Faces = ss.GW.selectedFaceIDs()
		d := ss.drawing().Clone()
		d.Dimensions = append(d.Dimensions, dm)
		if err := d.Check(); err != nil {
			ss.Error("%s.", capitalize(err.Error()))
			return true
		}
		ss.UndoRemember()
		ss.SK.Drawing = d
		ss.GW.ClearSelection()
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

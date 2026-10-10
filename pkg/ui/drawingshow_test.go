package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// litBox is the box about a picture's pixels that differ from its
// corner's, the background, and how many there are.
func litBox(t *testing.T, path string) (image.Rectangle, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	bg := img.At(b.Min.X, b.Min.Y)
	box, n := image.Rectangle{}, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.At(x, y) != bg {
				box = box.Union(image.Rect(x, y, x+1, y+1))
				n++
			}
		}
	}
	return box, n
}

// The drawing sheet shown in the window (roadmap 29.1): the window's
// picture is the sheet, the paper's edge nearly the whole picture's
// width, and it follows the drawing: a sheet turned to portrait is
// narrower than it is tall. Shown again, the model comes back. The menu
// item and the script call agree, and the mouse picks nothing while it
// is shown. A group with no solid has no sheet to show, and says so.
func TestShowDrawingSheet(t *testing.T) {
	ss, doc := drawingModel(t)
	dir := t.TempDir()
	model := filepath.Join(dir, "model.png")
	if err := doc.File.Render(model, 400, 300, "window"); err != nil {
		t.Fatal(err)
	}
	if err := doc.Drawing.Show(true); err != nil {
		t.Fatal(err)
	}
	if !doc.Drawing.Shown() || !ss.GW.SheetShown() {
		t.Fatal("the sheet isn't shown")
	}
	sheet := filepath.Join(dir, "sheet.png")
	if err := doc.File.Render(sheet, 400, 300, "window"); err != nil {
		t.Fatal(err)
	}
	box, n := litBox(t, sheet)
	if n == 0 || box.Dx() < 340 || box.Dx() > 400 {
		t.Errorf("the sheet covers %v (%d pixels), want nearly the picture's width", box, n)
	}

	d := doc.Drawing.Get()
	d.Portrait = true
	doc.Drawing.Set(d)
	if err := doc.File.Render(sheet, 400, 300, "window"); err != nil {
		t.Fatal(err)
	}
	if box, _ := litBox(t, sheet); box.Dx() >= box.Dy() {
		t.Errorf("a portrait sheet covers %v", box)
	}

	// Picking: nothing under the mouse in the middle of the window.
	ss.GW.MouseMoved(0, 0, false, false, false, false, false)
	if !ss.GW.hover.IsEmpty() {
		t.Errorf("hovered %+v on the sheet", ss.GW.hover)
	}

	ss.GW.MenuShowDrawingSheet()
	if doc.Drawing.Shown() {
		t.Error("the menu didn't show the model again")
	}
	if err := doc.File.Render(sheet, 400, 300, "window"); err != nil {
		t.Fatal(err)
	}
	if _, n := litBox(t, sheet); n == 0 {
		t.Error("nothing drawn after the sheet")
	}

	// No solid, no sheet.
	empty, _ := newTestApp(t)
	bare := api.NewDoc(empty.ScriptHost())
	bare.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	if err := bare.Drawing.Show(true); err == nil || bare.Drawing.Shown() {
		t.Errorf("a sketch with no solid showed its sheet (%v)", err)
	}
	if err := api.New().Drawing.Show(true); err == nil {
		t.Error("a bare sketch showed a sheet")
	}
}

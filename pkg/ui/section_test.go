package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// capPixels counts the pixels of a rendered PNG in the cap's colour.
func capPixels(t *testing.T, path string) int {
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
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r>>8 == uint32(sectionCapColor.Red) && g>>8 == uint32(sectionCapColor.Green) &&
				bl>>8 == uint32(sectionCapColor.Blue) {
				n++
			}
		}
	}
	return n
}

// Render draws the section view: the cut solid with its cap, which
// moves with the plane; turned off, there is no cap.
func TestSectionRender(t *testing.T) {
	t.Parallel()
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 40, 0))
	doc.Sketch.Circle(api.V(20, 20, 0), 8)
	block := doc.Features.Extrude(30, "")
	var side api.Entity
	for _, f := range block.Faces() {
		doc.View.SetSection(f, "", false)
		if doc.View.Section().Normal.Y < -0.99 {
			side = f
		}
	}
	dir := t.TempDir()
	render := func(name string) int {
		path := filepath.Join(dir, name+".png")
		if err := doc.File.Render(path, 320, 240, "isometric"); err != nil {
			t.Fatal(err)
		}
		return capPixels(t, path)
	}

	doc.View.SetSection(side, "-20", true)
	through := render("through")
	if through < 500 {
		t.Errorf("%d pixels of cap through the bore", through)
	}
	cap1 := ss.GW.sectionCap
	doc.View.SetSection(side, "-5", true)
	if near := render("near"); near < 500 || near == through {
		t.Errorf("%d pixels of cap 5 mm in, %d through the bore", near, through)
	}
	if ss.GW.sectionCap.keep == cap1.keep {
		t.Error("the cap was not computed again for the new plane")
	}
	if ss.GW.sectionCap.closed != true || len(ss.GW.sectionCap.poly.Contours) != 1 {
		t.Errorf("cap 5 mm in: closed %v, %d loops", ss.GW.sectionCap.closed, len(ss.GW.sectionCap.poly.Contours))
	}
	doc.View.ClearSection()
	if off := render("off"); off != 0 {
		t.Errorf("%d pixels of cap with the section view off", off)
	}
}

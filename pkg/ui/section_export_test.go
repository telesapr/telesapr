package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// TestExportSectionPaths writes the 2d section of a block with a bore
// through the window's File > Export > 2d Section (the active
// workplane is the section plane) and through File.ExportSection, and
// requires the same file; a script can also cut where the section view
// is.
func TestExportSectionPaths(t *testing.T) {
	ss, _ := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	// A plane half way up the block, to cut by.
	doc.Sketch.Anywhere3D()
	mid := doc.Sketch.NewWorkplane(api.V(0, 0, 15), api.V(1, 0, 0), api.V(0, 1, 0))
	doc.Sketch.InWorkplane(doc.Workplane("XY"))
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 40, 0))
	doc.Sketch.Circle(api.V(20, 20, 0), 8)
	doc.Features.Extrude(30, "")
	doc.Sketch.InWorkplane(mid)
	if core.IsFreeIn3D(ss.SK.ActiveWorkplane()) {
		t.Fatal("no active workplane")
	}
	dir := t.TempDir()

	win := filepath.Join(dir, "window.svg")
	if err := ss.ExportSectionTo(win); err != nil {
		t.Fatal(err)
	}
	scr := filepath.Join(dir, "script.svg")
	if err := doc.File.ExportSection(scr); err != nil {
		t.Fatal(err)
	}
	a, err := os.ReadFile(win)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(scr)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) == 0 || string(a) != string(b) {
		t.Fatalf("the window wrote %d bytes and the script %d, which differ at %s",
			len(a), len(b), firstDiffLine(string(a), string(b)))
	}

	// The section view's plane, 20 mm up from the workplane, cuts the
	// bore too; the section is another picture.
	var top api.Entity
	for _, f := range doc.Groups()[len(doc.Groups())-1].Faces() {
		if doc.View.SetSection(f, "", false); doc.View.Section().Normal.Z > 0.99 {
			top = f
		}
	}
	doc.View.SetSection(top, "-10", false)
	cut := filepath.Join(dir, "cut.svg")
	if err := doc.File.ExportAs(api.ExportSection, cut); err != nil {
		t.Fatal(err)
	}
	if c, err := os.ReadFile(cut); err != nil || !strings.Contains(string(c), "<svg") {
		t.Errorf("the cut section: %v", err)
	}
}

package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// TestScriptExportGlb is the glTF export's path pair: File > Export >
// Triangle Mesh with a .glb name, a script run by the window, and a
// script with no window (api.New) write the same bytes, since all three
// reach sketch.ExportMeshAsGlbTo.
func TestScriptExportGlb(t *testing.T) {
	build := func(doc *api.Doc) {
		doc.Sketch.Rect(api.V(-10, -10, 0), api.V(10, 10, 0))
		g := doc.Features.Extrude(5, "")
		g.SetColor(200, 40, 40)
		doc.Features.SketchIn(doc.Workplane("XY"), "")
		doc.Sketch.Circle(api.V(25, 0, 0), 4)
		pin := doc.Features.Extrude(12, "")
		pin.SetCombine(api.Assemble)
	}
	dir := t.TempDir()

	win, _ := newTestApp(t)
	build(api.NewDoc(win.ScriptHost()))
	byMenu := filepath.Join(dir, "menu.glb")
	p := win.Platform.(*headless.Platform)
	p.FormResponses = []headless.FormResponse{{}}
	p.FilePaths = []string{byMenu}
	win.menuExportMesh()

	inWin, _ := newTestApp(t)
	doc := api.NewDoc(inWin.ScriptHost())
	build(doc)
	byScript := filepath.Join(dir, "script.glb")
	if err := doc.File.Export(byScript); err != nil {
		t.Fatal(err)
	}

	alone := api.New()
	build(alone)
	byHeadless := filepath.Join(dir, "headless.glb")
	if err := alone.File.Export(byHeadless); err != nil {
		t.Fatal(err)
	}

	var files [3][]byte
	for i, f := range []string{byMenu, byScript, byHeadless} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) < 12 || string(b[:4]) != "glTF" {
			t.Fatalf("%s is not binary glTF", filepath.Base(f))
		}
		files[i] = b
	}
	if !bytes.Equal(files[0], files[1]) || !bytes.Equal(files[0], files[2]) {
		t.Errorf("the menu wrote %d bytes, the script %d, the script with no window %d, and they differ",
			len(files[0]), len(files[1]), len(files[2]))
	}
}

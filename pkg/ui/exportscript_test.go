package ui

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// File > Export > Script writes what File.ExportScript writes in the
// window and with no window, and the script builds the model again.
func TestExportScriptMatchesScript(t *testing.T) {
	t.Parallel()
	build := func(doc *api.Doc) {
		if err := doc.Params.Set("side", "20"); err != nil {
			t.Fatal(err)
		}
		lines := doc.Sketch.Rect(api.V(0, 0, 0), api.V(20, 12, 0))
		if err := doc.Constrain.Length(lines[0], 20).SetExpr("side"); err != nil {
			t.Fatal(err)
		}
		doc.Constrain.Length(lines[1], 12)
		doc.Constrain.Coincident(lines[0].Start(), doc.Origin())
		plate := doc.Features.Extrude(4, "plate")
		plate.AsExtrude().SetDepth(4)
		plate.SetColor(40, 120, 200)
	}
	dir := t.TempDir()

	win, _ := newTestApp(t)
	build(api.NewDoc(win.ScriptHost()))
	byMenu := filepath.Join(dir, "menu.go")
	win.Platform.(*headless.Platform).FilePaths = []string{byMenu}
	win.menuExport(cmdExportScript)

	inWin, _ := newTestApp(t)
	doc := api.NewDoc(inWin.ScriptHost())
	build(doc)
	byScript := filepath.Join(dir, "script.go")
	if _, err := doc.File.ExportScript(byScript); err != nil {
		t.Fatal(err)
	}

	alone := api.New()
	build(alone)
	byHeadless := filepath.Join(dir, "headless.go")
	notes, err := alone.File.ExportScript(byHeadless)
	if err != nil || len(notes) > 0 {
		t.Fatalf("the script left out %q, %v", notes, err)
	}

	var files [3][]byte
	for i, f := range []string{byMenu, byScript, byHeadless} {
		if files[i], err = os.ReadFile(f); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(files[0], files[1]) || !bytes.Equal(files[0], files[2]) {
		t.Fatalf("the menu wrote\n%s\nthe script\n%s\nthe script with no window\n%s", files[0], files[1], files[2])
	}

	sk := sketch.NewSketch()
	sk.NewFile()
	host := api.NewSketchHost(sk, "")
	if err := script.RunSource(context.Background(), script.Options{Host: host}, "model.go", string(files[0])); err != nil {
		t.Fatalf("running the script: %v\n%s", err, files[0])
	}
	made := api.NewDoc(host)
	g := made.Group("plate")
	if v := g.Analyze().Check().Volume; math.Abs(v-20*12*4) > 1e-6 {
		t.Errorf("the plate made again is %g mm³, want 960", v)
	}
	if g.Dof() != 0 || !g.AsExtrude().DepthHeld() {
		t.Errorf("the plate made again has %d DOF, depth held %v", g.Dof(), g.AsExtrude().DepthHeld())
	}
	if p, ok := made.Params.Get("side"); !ok || p.Expr != "20" {
		t.Errorf("the parameter made again is %+v", p)
	}
}

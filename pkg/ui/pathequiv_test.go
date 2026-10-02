package ui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
	"github.com/spacesolve/spacesolve/pkg/script"
	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// requireSamePaths builds a model twice, through the interface and
// through a script, and requires the same saved file from both: the
// interface script testdata/ui/scripts/<uiScript>.script runs on the
// headless platform, and the Go script testdata/ui/scripts/<goScript>.go
// runs on a new sketch with no interface, as spacesolve-cli run does.
// Both paths must reach the same engine entry points, so neither can
// drift from the other (ROADMAP-JUMP.md, "Definition of done").
// The pairs are named path_*, and have no golden of their own: each
// path is the other's judge. An interface script must not save: a save
// regenerates, and the solver is not a fixed point, so a solved sketch
// solved again can move by an ulp. SPACESOLVE_PATHEQUIV_DIR names a
// directory to write both saves to.
func requireSamePaths(t *testing.T, uiScript, goScript string) (fromUI, fromScript *sketch.Sketch) {
	t.Helper()
	var r *uiScriptRunner
	runUIScriptWith(t, uiScript, func(rr *uiScriptRunner) { r = rr })
	savedUI := spcs.Encode(r.ss.SK, spcs.Options{})

	path := filepath.Join("..", "..", "testdata", "ui", "scripts", goScript+".go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := api.New()
	opts := script.Options{Host: doc.Host(), Stdout: io.Discard, Stderr: io.Discard}
	if err := script.RunSource(context.Background(), opts, path, string(src)); err != nil {
		t.Fatalf("%s: %v", goScript, err)
	}
	savedScript := spcs.Encode(doc.Host().Sketch(), spcs.Options{})

	if dir := os.Getenv("SPACESOLVE_PATHEQUIV_DIR"); dir != "" {
		// Both saves, for a diff.
		for name, data := range map[string][]byte{uiScript + ".ui.spcs": savedUI, goScript + ".go.spcs": savedScript} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if string(savedUI) != string(savedScript) {
		t.Fatalf("the interface (%s) and the script (%s) save different files, at %s",
			uiScript, goScript, firstDiffLine(string(savedUI), string(savedScript)))
	}
	return r.ss.SK, doc.Host().Sketch()
}

// A rectangle drawn with the mouse and extruded, and the same made by
// a script.
func TestPathEquivalenceExtrude(t *testing.T) {
	requireSamePaths(t, "path_extrude", "path_extrude")
}

// A screw thread made through its screen, and by a script.
func TestPathEquivalenceThread(t *testing.T) {
	requireSamePaths(t, "path_thread", "path_thread")
}

// A rectangle whose sides come from parameters, made through the
// parameters screen and the dimension editor, and by a script.
func TestPathEquivalenceParameters(t *testing.T) {
	requireSamePaths(t, "path_parameters", "path_parameters")
}

// A sketch in 3d moved earlier in the history through the Edit menu,
// and by a script.
func TestPathEquivalenceMoveGroup(t *testing.T) {
	requireSamePaths(t, "path_move_group", "path_move_group")
}

// A sketch suppressed whole through its group screen, and by a script.
func TestPathEquivalenceSuppressAll(t *testing.T) {
	requireSamePaths(t, "path_suppress_all", "path_suppress_all")
}

// An extrusion deleted from its group screen, and by a script.
func TestPathEquivalenceDeleteGroup(t *testing.T) {
	requireSamePaths(t, "path_delete_group", "path_delete_group")
}

// A section view cut parallel to a face picked with the mouse, its
// offset typed and its side flipped on its screen, and the same set by
// a script. The section isn't saved, so its state and where it cuts
// are compared too.
func TestPathEquivalenceSection(t *testing.T) {
	ui, script := requireSamePaths(t, "path_section", "path_section")
	if ui.Section == nil || script.Section == nil {
		t.Fatalf("section view on: interface %v, script %v", ui.Section != nil, script.Section != nil)
	}
	if *ui.Section != *script.Section {
		t.Errorf("the interface's section %+v, the script's %+v", *ui.Section, *script.Section)
	}
	keepUI, _, okUI, _ := ui.SectionCut()
	keepScript, _, okScript, _ := script.SectionCut()
	if !okUI || !okScript || keepUI != keepScript {
		t.Errorf("the interface cuts by %v (%v), the script by %v (%v)", keepUI, okUI, keepScript, okScript)
	}
}

// A circle swept along a path picked with the mouse, and by a script.
func TestPathEquivalenceSweep(t *testing.T) {
	requireSamePaths(t, "path_sweep", "path_sweep")
}

// An extrusion mirrored in a face picked with the mouse, and by a
// script.
func TestPathEquivalenceMirror(t *testing.T) {
	requireSamePaths(t, "path_mirror", "path_mirror")
}

// Two edges picked with the mouse and chamfered, and by a script.
func TestPathEquivalenceChamfer(t *testing.T) {
	requireSamePaths(t, "path_chamfer", "path_chamfer")
}

package ui

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/script"
	"github.com/spacesolve/spacesolve/pkg/script/api"
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
func requireSamePaths(t *testing.T, uiScript, goScript string) {
	t.Helper()
	var r *uiScriptRunner
	runUIScriptWith(t, uiScript, func(rr *uiScriptRunner) { r = rr })
	fromUI := slvs.NewDocument(r.ss.SK).SaveRegenerated()

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
	fromScript := slvs.NewDocument(doc.Host().Sketch()).SaveRegenerated()

	if dir := os.Getenv("SPACESOLVE_PATHEQUIV_DIR"); dir != "" {
		// Both saves, for a diff.
		for name, data := range map[string][]byte{uiScript + ".ui.slvs": fromUI, goScript + ".go.slvs": fromScript} {
			if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if string(fromUI) != string(fromScript) {
		t.Fatalf("the interface (%s) and the script (%s) save different files, at %s",
			uiScript, goScript, firstDiffLine(string(fromUI), string(fromScript)))
	}
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

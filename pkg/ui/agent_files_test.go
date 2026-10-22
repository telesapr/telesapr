package ui

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/script/agent"
)

// A program measures the drawing on screen, looks inside it through a
// section that leaves the user's view as it was, and exports and opens
// files only as the user allows, each shown in the console as the
// script that would do the same; an open never replaces changes that
// aren't saved.
func TestAgentFiles(t *testing.T) {
	a := newAgentApp(t)
	ctx := context.Background()
	dir := t.TempDir()
	// Guarded, as by default, with the test's folder allowed: the tools
	// keep to it as the program's scripts do.
	a.on(func() { a.ss.SetAgentFiles(true, []string{dir}) })
	if res, err := a.r.Run(ctx, "doc.Sketch.Rect(V(0, 0, 0), V(10, 20, 0))\ndoc.Features.Extrude(5, \"plate\")"); err != nil || res.Error != "" {
		t.Fatalf("the plate gave %+v, %v", res, err)
	}

	if text, err := a.r.Measure(ctx, agent.Measuring{}); err != nil || !strings.Contains(text, "g003-plate") || !strings.Contains(text, "1000") {
		t.Errorf("the plate measures as %q, %v", text, err)
	}

	whole, err := a.r.Render(ctx, agent.Picture{View: "isometric", Width: 200, Height: 150})
	if err != nil {
		t.Fatal(err)
	}
	cut, err := a.r.Render(ctx, agent.Picture{View: "isometric", Width: 200, Height: 150, Section: "YZ", SectionOffset: "5"})
	if err != nil {
		t.Fatal(err)
	}
	if img, err := png.Decode(bytes.NewReader(cut)); err != nil || img.Bounds().Dx() != 200 {
		t.Errorf("the section doesn't decode as 200 wide: %v", err)
	}
	if bytes.Equal(whole, cut) {
		t.Errorf("the section is drawn as the whole plate")
	}
	var section bool
	a.on(func() { section = a.ss.SK.Section != nil })
	if section {
		t.Errorf("the picture's section was left on")
	}
	if _, err := a.r.Render(ctx, agent.Picture{Section: "XX"}); err == nil || !strings.Contains(err.Error(), "XY") {
		t.Errorf("a section by no workplane: %v", err)
	}

	a.on(func() {
		a.ss.AgentConfirm = true
		a.p.DialogResponses = []platform.Response{platform.ResponseNo, platform.ResponseYes}
	})
	start := 0
	a.on(func() { start = len(a.w.Texts) })
	stl := filepath.Join(dir, "plate.stl")
	if _, err := a.r.Export(ctx, stl); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Errorf("a refused export: %v", err)
	}
	if _, err := os.Stat(stl); err == nil {
		t.Errorf("the refused export wrote %s", stl)
	}
	if text, err := a.r.Export(ctx, stl); err != nil || !strings.HasPrefix(text, "wrote "+stl) {
		t.Errorf("the allowed export says %q, %v", text, err)
	}
	shown := a.shown(start)
	if !strings.Contains(shown, "asked to export to "+stl+"; it was refused") ||
		!strings.Contains(shown, "ai> doc.File.Export("+strconv.Quote(stl)+")") {
		t.Errorf("the console shows\n%s", shown)
	}

	saved := filepath.Join(dir, "plate.tlspr")
	if _, err := a.r.Open(ctx, saved); err == nil || err.Error() != agent.ErrUnsaved.Error() {
		t.Errorf("opening over unsaved changes: %v", err)
	}
	a.on(func() { a.ss.AgentConfirm = false })
	if res, err := a.r.Run(ctx, "doc.File.Save("+strconv.Quote(saved)+")"); err != nil || res.Error != "" {
		t.Fatalf("the save gave %+v, %v", res, err)
	}
	if text, err := a.r.Open(ctx, saved); err != nil || !strings.HasPrefix(text, "opened "+saved+"\n") {
		t.Errorf("opening the saved plate: %q, %v", text, err)
	}
	var file string
	a.on(func() { file = a.ss.SaveFile })
	if file != saved {
		t.Errorf("the window has %q open, want %q", file, saved)
	}

	l, err := a.r.Files(ctx, dir)
	if err != nil || len(l.Entries) != 2 || l.Entries[0].Name != "plate.stl" || l.Entries[1].Name != "plate.tlspr" {
		t.Errorf("the directory is listed as %+v, %v", l, err)
	}
}

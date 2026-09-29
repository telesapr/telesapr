package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/headless"
)

func newTestApp(t *testing.T) (*SolveSpaceUI, *headless.Platform) {
	t.Helper()
	p := headless.New()
	ss := New(p)
	ss.Init()
	return ss, p
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSaveNewFileVsCpp saves a new sketch through File > Save and
// compares it with C++'s save of a new file.
func TestSaveNewFileVsCpp(t *testing.T) {
	ss, p := newTestApp(t)
	path := filepath.Join(t.TempDir(), "a.slvs")
	p.FilePaths = []string{path}
	ss.MenuFile(CmdSave)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cppsave", "newfile.cppsave"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("saved new file differs from C++ (%d vs %d bytes)", len(got), len(want))
	}
	if ss.SaveFile != path || ss.SK.Unsaved {
		t.Errorf("SaveFile %q unsaved %v", ss.SaveFile, ss.SK.Unsaved)
	}
	if len(ss.RecentFiles) == 0 || ss.RecentFiles[0] != path {
		t.Errorf("recent files %v", ss.RecentFiles)
	}
	// C++ shows the full path by default.
	if title := ss.GW.window.(*headless.Window).Title; title != path {
		t.Errorf("title %q", title)
	}

	// Saving again doesn't ask for a name.
	ss.MenuFile(CmdSave)
	if len(p.FileDialogs) != 1 {
		t.Errorf("%d file dialogs", len(p.FileDialogs))
	}
}

// TestOpenAndUnsavedChanges opens a fixture, and checks the question
// about unsaved changes before a new file.
func TestOpenAndUnsavedChanges(t *testing.T) {
	ss, p := newTestApp(t)
	path := filepath.Join(t.TempDir(), "normal.slvs")
	copyFile(t, filepath.Join("..", "..", "testdata", "group", "translate_nd", "normal.slvs"), path)

	p.FilePaths = []string{path}
	ss.MenuFile(CmdOpen)
	if ss.SaveFile != path || len(ss.SK.Groups) <= 2 {
		t.Fatalf("open: SaveFile %q, %d groups", ss.SaveFile, len(ss.SK.Groups))
	}
	if ss.SK.Unsaved {
		t.Error("unsaved right after opening")
	}
	groups := len(ss.SK.Groups)

	ss.SK.Unsaved = true
	p.DialogResponses = []platform.Response{platform.ResponseCancel}
	ss.MenuFile(CmdNew)
	if len(ss.SK.Groups) != groups || ss.SaveFile != path {
		t.Error("New went ahead after Cancel")
	}
	if d := p.DialogsShown[len(p.DialogsShown)-1]; d.Title != "Modified File" {
		t.Errorf("dialog %q", d.Title)
	}

	p.DialogResponses = []platform.Response{platform.ResponseNo}
	ss.MenuFile(CmdNew)
	if len(ss.SK.Groups) != 2 || ss.SaveFile != "" {
		t.Errorf("New: %d groups, SaveFile %q", len(ss.SK.Groups), ss.SaveFile)
	}
	if title := ss.GW.window.(*headless.Window).Title; title != "(new sketch)" {
		t.Errorf("title %q", title)
	}
}

// TestAutosave writes an autosave for an unsaved sketch and offers it
// when the file is opened again.
func TestAutosave(t *testing.T) {
	ss, p := newTestApp(t)
	path := filepath.Join(t.TempDir(), "normal.slvs")
	copyFile(t, filepath.Join("..", "..", "testdata", "group", "translate_nd", "normal.slvs"), path)
	if !ss.Load(path) {
		t.Fatal("load failed")
	}

	ss.Autosave()
	backup := strings.TrimSuffix(path, ".slvs") + ".slvs~"
	if _, err := os.Stat(backup); err == nil {
		t.Fatal("autosave of a saved sketch")
	}
	ss.SK.Unsaved = true
	ss.Autosave()
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("no autosave: %v", err)
	}

	p.DialogResponses = []platform.Response{platform.ResponseYes}
	if !ss.Load(path) {
		t.Fatal("load of autosave failed")
	}
	if !ss.SK.Unsaved || ss.SaveFile != path {
		t.Errorf("after autosave load: unsaved %v, SaveFile %q", ss.SK.Unsaved, ss.SaveFile)
	}

	// Don't Save removes the autosave.
	p.DialogResponses = []platform.Response{platform.ResponseNo}
	ss.MenuFile(CmdNew)
	if _, err := os.Stat(backup); err == nil {
		t.Error("autosave left after Don't Save")
	}
}

// TestSaveAsKeepsLinks saves a sketch that links an STL into another
// folder, and checks the link still resolves from there.
func TestSaveAsKeepsLinks(t *testing.T) {
	ss, p := newTestApp(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "normal.slvs")
	copyFile(t, filepath.Join("..", "..", "testdata", "group", "link_stl", "normal.slvs"), src)
	copyFile(t, filepath.Join("..", "..", "testdata", "group", "link_stl", "box.stl"),
		filepath.Join(dir, "src", "box.stl"))
	if !ss.Load(src) {
		t.Fatal("load failed")
	}
	triangles := func() int {
		n := 0
		for i := range ss.SK.Groups {
			n += len(ss.SK.Groups[i].RunningMesh.Triangles)
		}
		return n
	}
	before := triangles()
	if before == 0 {
		t.Fatal("linked mesh didn't load")
	}

	dst := filepath.Join(dir, "other", "deeper", "b.slvs")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	p.FilePaths = []string{dst}
	ss.MenuFile(CmdSaveAs)
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Group.impFileRel=../../src/box.stl\n") {
		t.Error("link not rewritten relative to the new file")
	}

	if !ss.Load(dst) {
		t.Fatal("reload failed")
	}
	if after := triangles(); after != before {
		t.Errorf("linked mesh after Save As: %d triangles, want %d", after, before)
	}
}

// TestAnalyzeVolume runs Analyze > Volume on an extrusion.
func TestAnalyzeVolume(t *testing.T) {
	ss, p := newTestApp(t)
	if !ss.Load(filepath.Join("..", "..", "testdata", "group", "translate_nd", "normal.slvs")) {
		t.Fatal("load failed")
	}
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	ss.MenuAnalyze(CmdVolume)
	d := p.DialogsShown[len(p.DialogsShown)-1]
	if !strings.HasPrefix(d.Message, "The volume of the solid model is:") {
		t.Errorf("message %q", d.Message)
	}
	t.Logf("%s / %s", d.Message, d.Description)
}

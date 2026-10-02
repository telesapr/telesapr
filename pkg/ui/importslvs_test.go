package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/headless"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// copyFixtureDir copies the files of a testdata directory into a new
// temporary directory and returns it.
func copyFixtureDir(t *testing.T, dir string) string {
	t.Helper()
	out := t.TempDir()
	from := filepath.Join("..", "..", "testdata", filepath.FromSlash(dir))
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			copyFile(t, filepath.Join(from, e.Name()), filepath.Join(out, e.Name()))
		}
	}
	return out
}

// lastDialog is the last message dialog shown.
func lastDialog(t *testing.T, p *headless.Platform) *headless.MessageDialog {
	t.Helper()
	if len(p.DialogsShown) == 0 {
		t.Fatal("no dialog shown")
	}
	return p.DialogsShown[len(p.DialogsShown)-1]
}

// dialogText is all a message dialog says.
func dialogText(d *headless.MessageDialog) string { return d.Message + "\n" + d.Description }

// TestSlvsNotOpenedOrSaved: the product imports SolveSpace models, and
// neither opens nor writes them.
func TestSlvsNotOpenedOrSaved(t *testing.T) {
	ss, p := newTestApp(t)
	p.DialogResponses = []platform.Response{platform.ResponseOK, platform.ResponseOK}
	if ss.LoadFromFile(filepath.Join("..", "..", "testdata", "group", "translate_nd", "normal.slvs")) {
		t.Error("opened a .slvs file")
	}
	if d := lastDialog(t, p); !strings.Contains(dialogText(d), "Use File > Import") {
		t.Errorf("message %q", dialogText(d))
	}
	path := filepath.Join(t.TempDir(), "a.slvs")
	if ss.SaveToFile(path) {
		t.Error("saved a .slvs file")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("a .slvs file was written")
	}
	if d := lastDialog(t, p); !strings.Contains(dialogText(d), "doesn't write SolveSpace models") {
		t.Errorf("message %q", dialogText(d))
	}
}

// TestLoadSpcsProblems: a file that doesn't decode is refused with the
// place of the problem, one from a newer version names both versions,
// and one with parts the reader doesn't know loads the rest and says so.
func TestLoadSpcsProblems(t *testing.T) {
	ss, p := newTestApp(t)
	dir := t.TempDir()
	write := func(name, data string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	for _, c := range []struct {
		name, data, message string
		loads               bool
	}{
		{"empty.spcs", "", "the file is empty", false},
		{"syntax.spcs", "spacesolve 1\ngroup {\n", "2:1: block 'group' is not closed", false},
		{"newer.spcs", "spacesolve 3\n", "This version reads files up to version 2", false},
	} {
		p.DialogResponses = []platform.Response{platform.ResponseOK}
		if got := ss.LoadFromFile(write(c.name, c.data)); got != c.loads {
			t.Errorf("%s: loaded %v", c.name, got)
		}
		if d := lastDialog(t, p); !strings.Contains(dialogText(d), c.message) {
			t.Errorf("%s: message %q, want %q", c.name, dialogText(d), c.message)
		}
	}

	good, err := os.ReadFile(importFixture(t, "group/translate_nd/normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	future := write("future.spcs", string(good)+"\nhologram 0x1 {\n  shimmer\n}\n")
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	if !ss.LoadFromFile(future) || len(ss.SK.Groups) <= 2 {
		t.Errorf("a file with an unknown block didn't load: %d groups", len(ss.SK.Groups))
	}
	if d := lastDialog(t, p); !strings.Contains(dialogText(d), "was not recognized and was left out") ||
		!strings.Contains(dialogText(d), `unknown statement "hologram"`) {
		t.Errorf("message %q", dialogText(d))
	}

	var many strings.Builder
	many.WriteString("spacesolve 1\n")
	for range maxWarnings + 3 {
		many.WriteString("odd\n")
	}
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	ss.LoadFromFile(write("many.spcs", many.String()))
	if d := lastDialog(t, p); !strings.Contains(dialogText(d), "... and 3 more") {
		t.Errorf("message %q", dialogText(d))
	}
}

// TestImportSolveSpace imports a model through File > Import: it asks
// where to save, shows the plan, writes the .spcs file and opens it.
func TestImportSolveSpace(t *testing.T) {
	ss, p := newTestApp(t)
	dir := copyFixtureDir(t, "group/translate_nd")
	src := filepath.Join(dir, "normal.slvs")
	dst := filepath.Join(dir, "part") // the extension is added

	p.FilePaths = []string{src, dst}
	p.DialogResponses = []platform.Response{platform.ResponseYes}
	ss.MenuFile(CmdImport)
	dst += ".spcs"
	if ss.SaveFile != dst || ss.SK.Unsaved || len(ss.SK.Groups) <= 2 {
		t.Fatalf("SaveFile %q, unsaved %v, %d groups", ss.SaveFile, ss.SK.Unsaved, len(ss.SK.Groups))
	}
	if len(p.FileDialogs) != 2 || p.FileDialogs[1].Title != "Save the Imported Model" ||
		p.FileDialogs[1].Filename != dst[:len(dst)-len(".spcs")] {
		t.Errorf("file dialogs %+v", p.FileDialogs)
	}
	d := lastDialog(t, p)
	if d.Title != "Import SolveSpace Model" || d.Description != dst {
		t.Errorf("plan dialog %q: %q", d.Title, d.Description)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Error(err)
	}
	if ss.RecentFiles[0] != dst {
		t.Errorf("recent %v", ss.RecentFiles)
	}

	// Cancelling at the save dialog or at the plan writes nothing.
	other := filepath.Join(dir, "other.spcs")
	p.FilePaths = []string{""}
	if ss.ImportSolveSpace(src) {
		t.Error("imported after cancelling the save dialog")
	}
	p.FilePaths = []string{other}
	p.DialogResponses = []platform.Response{platform.ResponseCancel}
	if ss.ImportSolveSpace(src) {
		t.Error("imported after cancelling the plan")
	}
	if _, err := os.Stat(other); err == nil {
		t.Error("a cancelled import wrote its file")
	}
	if ss.SaveFile != dst {
		t.Errorf("SaveFile %q after cancelled imports", ss.SaveFile)
	}

	// Unsaved changes are asked about first.
	ss.SK.Unsaved = true
	p.DialogResponses = []platform.Response{platform.ResponseCancel}
	if ss.ImportSolveSpace(src) || lastDialog(t, p).Title != "Modified File" {
		t.Error("imported over unsaved changes")
	}
	ss.SK.Unsaved = false

	// An unknown extension names the ones Import reads.
	p.FilePaths = []string{filepath.Join(dir, "x.txt")}
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	ss.MenuFile(CmdImport)
	if d := lastDialog(t, p); !strings.Contains(dialogText(d), "try .dxf, .dwg or .slvs") {
		t.Errorf("message %q", dialogText(d))
	}
}

// TestImportSolveSpaceAssembly imports a model that links another:
// both are converted, and a converted copy that exists is reused or
// overwritten as the user says.
func TestImportSolveSpaceAssembly(t *testing.T) {
	ss, p := newTestApp(t)
	dir := copyFixtureDir(t, "group/link")
	src := filepath.Join(dir, "normal.slvs")
	dst := filepath.Join(dir, "assembly.spcs")
	linked := filepath.Join(dir, "rect_v20.spcs")

	p.FilePaths = []string{dst}
	p.DialogResponses = []platform.Response{platform.ResponseYes}
	if !ss.ImportSolveSpace(src) {
		t.Fatal("import failed")
	}
	if d := lastDialog(t, p); d.Description != linked+"\n"+dst {
		t.Errorf("plan %q", d.Description)
	}
	var g *sketch.Group
	for i := range ss.SK.Groups {
		if ss.SK.Groups[i].Type == sketch.GroupLinked {
			g = &ss.SK.Groups[i]
		}
	}
	if g == nil || g.LinkFile != "rect_v20.spcs" || len(g.ImpEntity) == 0 {
		t.Fatalf("link group %+v", g)
	}

	// A mark in the converted copy tells whether it was rewritten.
	mark := func() {
		data, err := os.ReadFile(linked)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(linked, append(data, "# mark\n"...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	marked := func() bool {
		data, err := os.ReadFile(linked)
		if err != nil {
			t.Fatal(err)
		}
		return strings.HasSuffix(string(data), "# mark\n")
	}

	mark()
	p.FilePaths = []string{dst}
	p.DialogResponses = []platform.Response{platform.ResponseNo}
	if !ss.ImportSolveSpace(src) || !marked() {
		t.Error("Reuse rewrote the linked copy")
	}
	if d := lastDialog(t, p); !strings.Contains(d.Description, "exist already") ||
		!strings.HasSuffix(d.Description, "\n"+linked) {
		t.Errorf("plan %q", d.Description)
	}
	p.FilePaths = []string{dst}
	p.DialogResponses = []platform.Response{platform.ResponseYes}
	if !ss.ImportSolveSpace(src) || marked() {
		t.Error("Overwrite kept the linked copy")
	}
	p.FilePaths = []string{dst}
	p.DialogResponses = []platform.Response{platform.ResponseCancel}
	if ss.ImportSolveSpace(src) {
		t.Error("imported after Cancel")
	}

	// A missing linked model is listed, then asked for; declined, its
	// geometry is left out.
	if err := os.Remove(filepath.Join(dir, "rect_v20.slvs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(linked); err != nil {
		t.Fatal(err)
	}
	p.FilePaths = []string{dst}
	p.DialogResponses = []platform.Response{platform.ResponseYes, platform.ResponseNo, platform.ResponseOK,
		platform.ResponseNo}
	shown := len(p.DialogsShown)
	ss.ImportSolveSpace(src)
	var plan, locate *headless.MessageDialog
	for _, d := range p.DialogsShown[shown:] {
		switch d.Title {
		case "Import SolveSpace Model":
			plan = d
		case "Missing File":
			locate = d
		}
	}
	if plan == nil || !strings.Contains(plan.Description, "These linked models are missing") ||
		!strings.Contains(plan.Description, "rect_v20.slvs, linked from normal.slvs") {
		t.Errorf("plan %+v", plan)
	}
	if locate == nil || locate.Message != "The linked file “rect_v20.spcs” is not present." {
		t.Errorf("locate dialog %+v", locate)
	}
}

// TestImportSolveSpaceCycle: models that link each other aren't
// imported.
func TestImportSolveSpaceCycle(t *testing.T) {
	ss, p := newTestApp(t)
	dir := t.TempDir()
	model, err := os.ReadFile(filepath.Join("..", "..", "testdata", "group", "link", "normal.slvs"))
	if err != nil {
		t.Fatal(err)
	}
	for name, other := range map[string]string{"a.slvs": "b.slvs", "b.slvs": "a.slvs"} {
		data := strings.Replace(string(model), "Group.impFileRel=rect_v20.slvs", "Group.impFileRel="+other, 1)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p.FilePaths = []string{filepath.Join(dir, "a.spcs")}
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	if ss.ImportSolveSpace(filepath.Join(dir, "a.slvs")) {
		t.Error("imported a cycle")
	}
	if d := lastDialog(t, p); !strings.Contains(dialogText(d), "the linked files form a cycle") {
		t.Errorf("message %q", dialogText(d))
	}
}

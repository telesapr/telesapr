package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func newTestApp(t *testing.T) (*Application, *headless.Platform) {
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

// TestSaveNewFile saves a new sketch through File > Save: the name the
// user gives gets the .tlspr extension.
func TestSaveNewFile(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	path := filepath.Join(t.TempDir(), "a")
	p.FilePaths = []string{path}
	ss.MenuFile(cmdSave)
	path += ".tlspr"

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "telesapr 2\n\nunits mm\n") {
		t.Errorf("saved file starts %.40q", got)
	}
	if ss.SaveFile != path || ss.SK.Unsaved {
		t.Errorf("SaveFile %q unsaved %v", ss.SaveFile, ss.SK.Unsaved)
	}
	if len(ss.RecentFiles) == 0 || ss.RecentFiles[0] != path {
		t.Errorf("recent files %v", ss.RecentFiles)
	}
	// SolveSpace shows the full path by default.
	if title := ss.GW.window.(*headless.Window).Title; title != path {
		t.Errorf("title %q", title)
	}

	// Saving again doesn't ask for a name.
	ss.MenuFile(cmdSave)
	if len(p.FileDialogs) != 1 {
		t.Errorf("%d file dialogs", len(p.FileDialogs))
	}

	// What the file holds opens as the sketch saved.
	if ss.Load(path) != nil || len(ss.SK.Groups) != 2 {
		t.Errorf("reopened with %d groups", len(ss.SK.Groups))
	}
}

func TestModelFileName(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"a.tlspr": "a.tlspr", "a.TLSPR": "a.TLSPR", "a.slvs": "a.tlspr", "a": "a.tlspr", "a.b": "a.b.tlspr",
	} {
		if got := modelFileName(in); got != want {
			t.Errorf("modelFileName(%q) = %q, want %q", in, got, want)
		}
	}
}

// importFixture converts a fixture to a .tlspr file in a new temporary
// directory, with the files beside it, and returns the .tlspr file.
func importFixture(t *testing.T, fixture string) string {
	t.Helper()
	dir := t.TempDir()
	from := filepath.Join("..", "..", "testdata", filepath.FromSlash(filepath.Dir(fixture)))
	entries, err := os.ReadDir(from)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			copyFile(t, filepath.Join(from, e.Name()), filepath.Join(dir, e.Name()))
		}
	}
	src := filepath.Join(dir, filepath.Base(fixture))
	dst := withExtension(src, "tlspr")
	if _, _, err := convert.File(src, dst, true); err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestOpenAndUnsavedChanges opens a fixture, and checks the question
// about unsaved changes before a new file.
func TestOpenAndUnsavedChanges(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	path := importFixture(t, "group/translate_nd/normal.slvs")

	p.FilePaths = []string{path}
	ss.MenuFile(cmdOpen)
	if ss.SaveFile != path || len(ss.SK.Groups) <= 2 {
		t.Fatalf("open: SaveFile %q, %d groups", ss.SaveFile, len(ss.SK.Groups))
	}
	if ss.SK.Unsaved {
		t.Error("unsaved right after opening")
	}
	groups := len(ss.SK.Groups)

	ss.SK.Unsaved = true
	p.DialogResponses = []platform.Response{platform.ResponseCancel}
	ss.MenuFile(cmdNew)
	if len(ss.SK.Groups) != groups || ss.SaveFile != path {
		t.Error("New went ahead after Cancel")
	}
	if d := p.DialogsShown[len(p.DialogsShown)-1]; d.Title != "Modified File" {
		t.Errorf("dialog %q", d.Title)
	}

	p.DialogResponses = []platform.Response{platform.ResponseNo}
	ss.MenuFile(cmdNew)
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
	t.Parallel()
	ss, p := newTestApp(t)
	path := importFixture(t, "group/translate_nd/normal.slvs")
	if ss.Load(path) != nil {
		t.Fatal("load failed")
	}

	ss.Autosave()
	backup := path + "~"
	if _, err := os.Stat(backup); err == nil {
		t.Fatal("autosave of a saved sketch")
	}
	ss.SK.Unsaved = true
	ss.Autosave()
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("no autosave: %v", err)
	}

	p.DialogResponses = []platform.Response{platform.ResponseYes}
	if ss.Load(path) != nil {
		t.Fatal("load of autosave failed")
	}
	if !ss.SK.Unsaved || ss.SaveFile != path {
		t.Errorf("after autosave load: unsaved %v, SaveFile %q", ss.SK.Unsaved, ss.SaveFile)
	}

	// Don't Save removes the autosave.
	p.DialogResponses = []platform.Response{platform.ResponseNo}
	ss.MenuFile(cmdNew)
	if _, err := os.Stat(backup); err == nil {
		t.Error("autosave left after Don't Save")
	}
}

// TestSaveAsKeepsLinks saves a sketch that links an STL into another
// folder, and checks the link still resolves from there.
func TestSaveAsKeepsLinks(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	src := importFixture(t, "group/link_stl/normal.slvs")
	dir := filepath.Dir(filepath.Dir(src))
	if ss.Load(src) != nil {
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

	dst := filepath.Join(dir, "other", "deeper", "b.tlspr")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	p.FilePaths = []string{dst}
	ss.MenuFile(cmdSaveAs)
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if want := "  link \"../../" + filepath.Base(filepath.Dir(src)) + "/box.stl\"\n"; !strings.Contains(string(data), want) {
		t.Error("link not rewritten relative to the new file")
	}

	if ss.Load(dst) != nil {
		t.Fatal("reload failed")
	}
	if after := triangles(); after != before {
		t.Errorf("linked mesh after Save As: %d triangles, want %d", after, before)
	}
}

// TestAnalyzeVolume runs Analyze > Volume on an extrusion.
func TestAnalyzeVolume(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	if !loadModel(t, ss, filepath.Join("..", "..", "testdata", "tlspr", "golden", "group", "translate_nd", "normal.tlspr")) {
		t.Fatal("load failed")
	}
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	ss.MenuAnalyze(cmdVolume)
	d := p.DialogsShown[len(p.DialogsShown)-1]
	if !strings.HasPrefix(d.Message, "The volume of the solid model is:") {
		t.Errorf("message %q", d.Message)
	}
	t.Logf("%s / %s", d.Message, d.Description)
}

// TestLoadFindsLinks opens a model whose linked file is beside it: the
// links must be found and read, and the user asked about none of them,
// whatever chord tolerance is in effect.
func TestLoadFindsLinks(t *testing.T) {
	t.Parallel()
	from := filepath.Join("..", "..", "testdata", "tlspr", "golden", "group", "link")
	for _, tc := range []struct {
		name string
		tol  float64
	}{
		// The settings file keeps the tolerance as a single-precision
		// number, so what a second session reads back is not quite the
		// default; reading the links again for that is not a change.
		{"the tolerance the settings file holds", float64(float32(0.1))},
		// A tolerance the user did set, which is a change and does read
		// the links again while the file is still being opened. That read
		// must find them, as the one the read of the file does.
		{"a tolerance the user set", 0.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{"normal.tlspr", "rect_v20.tlspr"} {
				copyFile(t, filepath.Join(from, name), filepath.Join(dir, name))
			}

			ss, p := newTestApp(t)
			ss.ChordTol = tc.tol
			p.DialogResponses = []platform.Response{platform.ResponseNo, platform.ResponseNo,
				platform.ResponseNo, platform.ResponseNo}
			if err := ss.Load(filepath.Join(dir, "normal.tlspr")); err != nil {
				t.Fatal(err)
			}
			if len(p.DialogsShown) != 0 {
				t.Errorf("%d dialogs about the links, first %q", len(p.DialogsShown), p.DialogsShown[0].Message)
			}
			var link *sketch.Group
			for i := range ss.SK.Groups {
				if ss.SK.Groups[i].Type == sketch.GroupLinked {
					link = &ss.SK.Groups[i]
				}
			}
			if link == nil {
				t.Fatal("no link group")
			}
			if len(link.ImpEntity) == 0 {
				t.Error("the linked file was not read")
			}
		})
	}
}

// TestLoadMissingLinkAsks loads a sketch whose linked file is missing:,
// the user is asked to locate it, and may decline, cancel the load, or
// pick the file.
func TestLoadMissingLinkAsks(t *testing.T) {
	t.Parallel()
	fixture := importFixture(t, "group/link/normal.slvs")
	linked, err := filepath.Abs(filepath.Join(filepath.Dir(fixture), "rect_v20.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		responses []platform.Response
		files     []string
		loaded    bool
		found     bool
	}{
		{"no", []platform.Response{platform.ResponseNo}, nil, true, false},
		{"cancel", []platform.Response{platform.ResponseCancel}, nil, false, false},
		{"yes", []platform.Response{platform.ResponseYes}, []string{linked}, true, true},
		{"yes but none", []platform.Response{platform.ResponseYes}, []string{""}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ss, p := newTestApp(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "moved.tlspr")
			copyFile(t, fixture, path)
			p.DialogResponses = tc.responses
			p.FilePaths = tc.files

			loaded := ss.Load(path) == nil
			if loaded != tc.loaded {
				t.Fatalf("loaded %v, want %v", loaded, tc.loaded)
			}
			if len(p.DialogsShown) != 1 {
				t.Fatalf("%d dialogs shown, want 1", len(p.DialogsShown))
			}
			d := p.DialogsShown[0]
			if d.Message != "The linked file “rect_v20.tlspr” is not present." || d.Type != platform.DialogQuestion {
				t.Errorf("dialog %q type %d", d.Message, d.Type)
			}
			if !tc.loaded {
				if ss.SaveFile != "" {
					t.Errorf("SaveFile %q after a cancelled load", ss.SaveFile)
				}
				return
			}
			var link *sketch.Group
			for i := range ss.SK.Groups {
				if ss.SK.Groups[i].Type == sketch.GroupLinked {
					link = &ss.SK.Groups[i]
				}
			}
			if link == nil {
				t.Fatal("no link group")
			}
			if found := len(link.ImpEntity) > 0; found != tc.found {
				t.Errorf("link group has %d entities", len(link.ImpEntity))
			}
			if tc.found {
				rel, err := filepath.Rel(dir, linked)
				if err != nil {
					t.Fatal(err)
				}
				if link.LinkFile != filepath.ToSlash(rel) {
					t.Errorf("link file %q, want %q", link.LinkFile, filepath.ToSlash(rel))
				}
			} else if link.LinkFile != "rect_v20.tlspr" {
				t.Errorf("link file %q changed", link.LinkFile)
			}
		})
	}
}

// TestOpenDamagedFile: a file that refers to what it lacks opens, with
// a message saying what was put back and what was left out, and the
// Broken References screen lists the same.
func TestOpenDamagedFile(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	path := filepath.Join("..", "..", "testdata", "tlspr", "damaged", "group_missing_workplane.tlspr")
	if err := ss.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	if len(ss.SK.Groups) != 2 {
		t.Errorf("%d groups", len(ss.SK.Groups))
	}
	text := dialogText(lastDialog(t, p))
	if !strings.Contains(text, "is damaged") || !strings.Contains(text, "deleted g003-on-missing, which used r007") {
		t.Errorf("message %q", text)
	}
	ss.TW.GoToScreen(screenBrokenReferences)
	ss.TW.Show()
	if !shows(ss, "deleted g003-on-missing, which used r007") || !shows(ss, "The file lacked what they used.") {
		t.Error("the broken references screen doesn't list what was left out")
	}
	if !shows(ss, "LEFT OUT OF THE MODEL, KEPT IN THE FILE") || !shows(ss, "r008-datum-point of g003-on-missing") {
		t.Error("the broken references screen doesn't list what the file keeps")
	}
	// Discarding it is a step of undo; undo keeps it again.
	clickText(t, ss, "[discard them]")
	if ss.SK.LeftOut != nil || !ss.SK.Unsaved {
		t.Error("discard kept what was left out")
	}
	ss.UndoUndo()
	if got := ss.SK.LeftOutNames(); len(got) != 2 {
		t.Errorf("after undo, left out %q", got)
	}
	ss.UndoRedo()
	if ss.SK.LeftOut != nil {
		t.Error("redo didn't discard it again")
	}

	p.DialogResponses = []platform.Response{platform.ResponseOK}
	path = filepath.Join("..", "..", "testdata", "tlspr", "damaged", "missing_references.tlspr")
	if err := ss.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	if text := dialogText(lastDialog(t, p)); !strings.Contains(text, "the reference workplane YZ was missing and was made again") {
		t.Errorf("message %q", text)
	}
	ss.TW.GoToScreen(screenBrokenReferences)
	ss.TW.Show()
	if !shows(ss, "MENDED WHEN THE FILE WAS OPENED") || !shows(ss, "the reference workplane ZX was missing") {
		t.Error("the broken references screen doesn't list what was put back")
	}
}

// What a damaged file's load left out is mended from the Broken
// References screen: the entity selected takes the place of what the
// file lacks, and what then has all it uses comes back; one step of
// undo. Nothing, or something of the wrong kind, selected is refused.
func TestMendDamagedFile(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	path := filepath.Join("..", "..", "testdata", "tlspr", "damaged", "group_missing_workplane.tlspr")
	if err := ss.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	ss.TW.GoToScreen(screenBrokenReferences)
	ss.TW.Show()
	if !shows(ss, "Or select what to use in place of what the file lacks:") || !shows(ss, "r007") {
		t.Fatal("the broken references screen offers no mend")
	}
	// With nothing selected, the screen waits for it to be picked.
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	clickText(t, ss, "[use the selected]")
	if ss.SK.LeftOut == nil {
		t.Fatal("a mend with nothing selected went ahead")
	}
	ss.TW.Show()
	// The XY workplane is no point, which the group's origin is.
	ss.GW.selection = []Selection{{Entity: core.HEntity{V: 0x10000}}}
	ss.TW.Show()
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	clickText(t, ss, "[take the selection]")
	if text := dialogText(lastDialog(t, p)); !strings.Contains(text, "as a point") {
		t.Errorf("message %q", text)
	}
	ss.GW.selection = []Selection{{Entity: core.HEntity{V: 0x10001}}}
	ss.TW.Show()
	clickText(t, ss, "[take the selection]")
	ss.SK.Refresh()
	if ss.SK.LeftOut != nil || ss.SK.Group(core.HGroup{V: 3}) == nil || ss.SK.Request(core.HRequest{V: 8}) == nil {
		t.Fatalf("the mend didn't bring the group back; left out %q", ss.SK.LeftOutNames())
	}
	ss.UndoUndo()
	if ss.SK.Group(core.HGroup{V: 3}) != nil || len(ss.SK.LeftOutNames()) != 2 {
		t.Error("undo didn't take the mend back")
	}
	ss.UndoRedo()
	if ss.SK.Group(core.HGroup{V: 3}) == nil || ss.SK.LeftOut != nil {
		t.Error("redo didn't mend it again")
	}
}

// A group a damaged file lacks, an extrusion's sketch, is mended from
// the Broken References screen: [choose a group] lists the model's
// groups, and clicking one brings the extrusion back; one step of undo.
func TestMendDamagedGroup(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	path := filepath.Join("..", "..", "testdata", "tlspr", "damaged", "group_missing_operand.tlspr")
	if err := ss.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	ss.TW.GoToScreen(screenBrokenReferences)
	ss.TW.Show()
	if !shows(ss, "Or choose a group in place of a group the file lacks:") || !shows(ss, "g009, a group") {
		t.Fatal("the broken references screen offers no group to choose")
	}
	if shows(ss, "[use g002-sketch-in-plane]") {
		t.Error("the groups are listed before [choose a group] is clicked")
	}
	clickText(t, ss, "[choose a group]")
	ss.TW.Show()
	if shows(ss, "[use #references]") {
		t.Error("the references group is offered")
	}
	clickText(t, ss, "[use g002-sketch-in-plane]")
	ss.SK.Refresh()
	if ss.SK.LeftOut != nil || ss.SK.Group(core.HGroup{V: 3}) == nil {
		t.Fatalf("the mend didn't bring the extrusion back; left out %q", ss.SK.LeftOutNames())
	}
	ss.UndoUndo()
	if ss.SK.Group(core.HGroup{V: 3}) != nil || len(ss.SK.LeftOutNames()) != 1 {
		t.Error("undo didn't take the mend back")
	}
	ss.UndoRedo()
	if ss.SK.Group(core.HGroup{V: 3}) == nil || ss.SK.LeftOut != nil {
		t.Error("redo didn't mend it again")
	}
}

// A constraint's circle the damaged file lacks is named for its kind,
// and the message for a mend with nothing selected asks for that kind.
func TestMendDamagedCircle(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	path := filepath.Join("..", "..", "testdata", "tlspr", "damaged", "constraint_missing_circle.tlspr")
	if err := ss.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	ss.TW.GoToScreen(screenBrokenReferences)
	ss.TW.Show()
	if !shows(ss, "r009, a circle or an arc") {
		t.Fatal("the missing circle isn't named for its kind")
	}
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	clickText(t, ss, "[use the selected]")
	if text := dialogText(lastDialog(t, p)); !strings.Contains(text, "Select a circle or an arc") {
		t.Errorf("message %q", text)
	}
	ss.GW.selection = []Selection{{Entity: core.HEntity{V: 0x40000}}}
	ss.TW.Show()
	clickText(t, ss, "[take the selection]")
	ss.SK.Refresh()
	if ss.SK.LeftOut != nil || ss.SK.Constraint(core.HConstraint{V: 1}) == nil {
		t.Fatalf("the diameter didn't come back; left out %q", ss.SK.LeftOutNames())
	}
}

// A lathe's axis and a mirror's plane the damaged file lacks are named
// for their kind, a selection of another kind is refused, and one of
// the kind brings the group back in one step of undo.
func TestMendDamagedAxisAndPlane(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		file, kind  string
		wrong, with core.HEntity
		group       core.HGroup
	}{
		{"lathe_missing_axis", "a line or a normal", core.HEntity{V: 0x40000}, core.HEntity{V: 0x50000}, core.HGroup{V: 3}},
		{"mirror_missing_plane", "a workplane or a planar face", core.HEntity{V: 0x30020}, core.HEntity{V: 0x20000}, core.HGroup{V: 4}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			ss, p := newTestApp(t)
			p.DialogResponses = []platform.Response{platform.ResponseOK}
			path := filepath.Join("..", "..", "testdata", "tlspr", "damaged", tc.file+".tlspr")
			if err := ss.LoadFromFile(path); err != nil {
				t.Fatal(err)
			}
			ss.TW.GoToScreen(screenBrokenReferences)
			ss.TW.Show()
			if !shows(ss, "r009, "+tc.kind) {
				t.Fatal("the missing entity isn't named for its kind")
			}
			// With nothing selected the screen stays, asking for the kind,
			// while the entity is picked.
			p.DialogResponses = []platform.Response{platform.ResponseOK}
			clickText(t, ss, "[use the selected]")
			if text := dialogText(lastDialog(t, p)); !strings.Contains(text, "Select "+tc.kind) {
				t.Errorf("message %q", text)
			}
			p.DialogResponses = []platform.Response{platform.ResponseOK}
			ss.GW.selection = []Selection{{Entity: tc.wrong}}
			ss.TW.Show()
			clickText(t, ss, "[take the selection]")
			if text := dialogText(lastDialog(t, p)); !strings.Contains(text, tc.kind) {
				t.Errorf("refusal %q", text)
			}
			if ss.SK.Group(tc.group) != nil {
				t.Fatal("a selection of another kind mended it")
			}
			ss.GW.selection = []Selection{{Entity: tc.with}}
			ss.TW.Show()
			clickText(t, ss, "[take the selection]")
			ss.SK.Refresh()
			if ss.SK.LeftOut != nil || ss.SK.Group(tc.group) == nil {
				t.Fatalf("the group didn't come back; left out %q", ss.SK.LeftOutNames())
			}
			ss.UndoUndo()
			if ss.SK.Group(tc.group) != nil || len(ss.SK.LeftOutNames()) != 1 {
				t.Error("undo didn't take the mend back")
			}
			ss.UndoRedo()
			if ss.SK.Group(tc.group) == nil || ss.SK.LeftOut != nil {
				t.Error("redo didn't mend it again")
			}
		})
	}
}

// A chamfer's screen says which of its edges the file names and
// doesn't hold, and its row in the list of groups is marked.
func TestDamagedChamferScreen(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	p.DialogResponses = []platform.Response{platform.ResponseOK}
	path := filepath.Join("..", "..", "testdata", "tlspr", "damaged", "chamfer_missing_edge.tlspr")
	if err := ss.LoadFromFile(path); err != nil {
		t.Fatal(err)
	}
	ss.TW.shown.screen, ss.TW.shown.group = screenGroupInfo, core.HGroup{V: 4}
	ss.TW.Show()
	if !shows(ss, "The file names an edge it doesn't hold") || !shows(ss, "entity 3853 of") {
		t.Error("the chamfer's screen doesn't say what edge is missing")
	}
}

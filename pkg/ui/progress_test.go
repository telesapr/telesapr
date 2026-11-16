package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// onlyProgress is the one progress window p showed, closed.
func onlyProgress(t *testing.T, p *headless.Platform) *headless.Progress {
	t.Helper()
	if len(p.ProgressShown) != 1 {
		t.Fatalf("%d progress windows, want 1", len(p.ProgressShown))
	}
	pr := p.ProgressShown[0]
	if !pr.Closed {
		t.Error("the progress window was left up")
	}
	return pr
}

// updateWith is the first update of pr whose text holds part, or nil.
func updateWith(pr *headless.Progress, part string) *headless.ProgressUpdate {
	for i := range pr.Updates {
		if strings.Contains(pr.Updates[i].Text, part) {
			return &pr.Updates[i]
		}
	}
	return nil
}

// TestOpenShowsProgress opens a file through File > Open: a progress
// window goes up once the file dialog has closed, shows the groups
// regenerating with how far it has got, and is closed at the end.
func TestOpenShowsProgress(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	path := importFixture(t, "group/translate_nd/normal.slvs")

	p.FilePaths = []string{path}
	ss.MenuFile(cmdOpen)
	pr := onlyProgress(t, p)
	if pr.Title != "Opening "+filepath.Base(path) {
		t.Errorf("title %q", pr.Title)
	}
	if pr.FileDialogsBefore != 1 {
		t.Errorf("shown after %d file dialogs, want 1", pr.FileDialogsBefore)
	}
	if updateWith(pr, "Reading ") == nil {
		t.Errorf("no reading step in %v", pr.Updates)
	}
	u := updateWith(pr, "Regenerating ")
	if u == nil {
		t.Fatalf("no regeneration in %v", pr.Updates)
	}
	if !strings.Contains(u.Text, "of ") || u.Fraction < 0 || u.Fraction >= 1 {
		t.Errorf("regeneration update %+v", *u)
	}
	if ss.progress != nil {
		t.Error("the application still holds the progress window")
	}
}

// TestOpenAutosaveAsksBeforeProgress answers the autosave question
// before the progress window goes up, not under it.
func TestOpenAutosaveAsksBeforeProgress(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	path := importFixture(t, "group/translate_nd/normal.slvs")
	if ss.Load(path) != nil {
		t.Fatal("load failed")
	}
	ss.SK.Unsaved = true
	ss.Autosave()
	p.ProgressShown = nil

	dialogs := len(p.DialogsShown)
	p.DialogResponses = []platform.Response{platform.ResponseYes}
	if err := ss.Load(path); err != nil {
		t.Fatal(err)
	}
	pr := onlyProgress(t, p)
	if pr.DialogsBefore != dialogs+1 || p.DialogsShown[dialogs].Title != "Autosave Available" {
		t.Errorf("progress after %d dialogs, want the autosave question (%d) first", pr.DialogsBefore, dialogs+1)
	}
	if !ss.SK.Unsaved {
		t.Error("the autosave was not taken")
	}
}

// TestExportMeshShowsProgress exports a mesh through its menu entry:
// the progress window goes up once the options form and the file
// dialog have closed, says when it writes, and is closed at the end.
func TestExportMeshShowsProgress(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	doc := api.NewDoc(ss.ScriptHost())
	doc.Sketch.Circle(api.V(0, 0, 0), 10)
	doc.Features.Extrude(5, "")

	file := filepath.Join(t.TempDir(), "part.stl")
	p.FormResponses = []headless.FormResponse{{}}
	p.FilePaths = []string{file}
	files := len(p.FileDialogs)
	ss.menuExportMesh()
	pr := onlyProgress(t, p)
	if pr.Title != "Exporting part.stl" {
		t.Errorf("title %q", pr.Title)
	}
	if pr.FormsBefore != 1 || pr.FileDialogsBefore != files+1 {
		t.Errorf("shown after %d forms and %d file dialogs, want 1 and %d", pr.FormsBefore,
			pr.FileDialogsBefore, files+1)
	}
	if updateWith(pr, "Writing part.stl") == nil {
		t.Errorf("no writing step in %v", pr.Updates)
	}
}

// TestCancelledExportShowsNoProgress gives up at the file dialog: no
// progress window goes up.
func TestCancelledExportShowsNoProgress(t *testing.T) {
	t.Parallel()
	ss, p := newTestApp(t)
	p.FilePaths = []string{""}
	ss.menuExport(cmdExportSurfaces)
	if len(p.ProgressShown) != 0 {
		t.Errorf("%d progress windows after a cancel", len(p.ProgressShown))
	}
}

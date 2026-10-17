package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/dxf"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// oneUndoStep runs do and checks that it is one step of undo: undo
// brings back what state() showed before, redo what it shows after.
func oneUndoStep(t *testing.T, ss *Application, state func() string, do func()) {
	t.Helper()
	before, steps := state(), ss.SK.Undo.Cnt
	do()
	after := state()
	if after == before {
		t.Fatalf("the edit changed nothing: %s", before)
	}
	if got := ss.SK.Undo.Cnt - steps; got != 1 {
		t.Fatalf("the edit made %d undo steps, want 1", got)
	}
	if !ss.SK.Unsaved {
		t.Error("the sketch is not marked unsaved")
	}
	ss.UndoUndo()
	if got := state(); got != before {
		t.Errorf("undo left %s, want %s", got, before)
	}
	ss.UndoRedo()
	if got := state(); got != after {
		t.Errorf("redo left %s, want %s", got, after)
	}
}

// The helix's "fixed pitch" check box, the group's opacity, the list of
// shown groups and the stipple pattern of a style are each one step of
// undo.
func TestScreenEditsAreUndoSteps(t *testing.T) {
	ss, _ := newTestApp(t)
	scriptConsole(t, ss,
		`axis := doc.Sketch.Line(V(0, 0, 0), V(0, 10, 0))`,
		`axis.SetConstruction(true)`,
		`doc.Sketch.Rect(V(10, 0, 0), V(20, 10, 0))`,
		`doc.Features.Helix(axis, 270, 30, "spring")`,
	)
	tw := ss.TW
	hg := ss.GW.activeGroupH()
	if ss.SK.Group(hg).Type != sketch.GroupHelix {
		t.Fatalf("the active group is type %v", ss.SK.Group(hg).Type)
	}
	tw.shown.group = hg
	group := func() *sketch.Group { return ss.SK.Group(hg) }
	hs := core.HStyle{V: core.StyleActiveGrp}

	t.Run("pitch", func(t *testing.T) {
		oneUndoStep(t, ss, func() string { return fmt.Sprint(group().ValB) },
			func() { tw.ScreenChangePitchOption(0, hg.V) })
	})
	t.Run("opacity", func(t *testing.T) {
		oneUndoStep(t, ss, func() string { return fmt.Sprint(group().Color.Alpha) },
			func() {
				tw.edit.meaning = editGroupOpacity
				tw.edit.group = hg
				tw.EditControlDone("0.5")
			})
	})
	t.Run("visibility", func(t *testing.T) {
		oneUndoStep(t, ss, func() string { return fmt.Sprint(group().Visible) },
			func() { tw.ScreenToggleGroupShown(0, hg.V) })
		oneUndoStep(t, ss, func() string { return fmt.Sprint(group().Visible) },
			func() { tw.ScreenShowGroupsSpecial('s', 0) })
	})
	t.Run("stipple", func(t *testing.T) {
		oneUndoStep(t, ss, func() string { return fmt.Sprint(ss.SK.StyleGet(hs).StippleType) },
			func() { tw.ScreenChangeStylePatternType(int(core.StippleDashDot)+1, hs.V) })
	})
	t.Run("factory styles", func(t *testing.T) {
		ss.SK.StyleGet(hs).Width = 7
		oneUndoStep(t, ss, func() string { return fmt.Sprint(ss.SK.StyleGet(hs).Width) },
			func() { tw.ScreenLoadFactoryDefaultStyles(0, 0) })
	})
}

// A DXF import that leaves the workplane is undone with the workplane.
func TestImportDxfUndoRestoresWorkplane(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "importdxf")
	var path string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && path == "" && !d.IsDir() && strings.HasSuffix(p, ".dxf") {
			if data, err := os.ReadFile(p); err == nil && dxf.Has3D(data) {
				path = p
			}
		}
		return nil
	})
	if path == "" {
		t.Skip("no DXF file with a Z coordinate in testdata/importdxf")
	}
	p := headless.New()
	for range 10 {
		p.DialogResponses = append(p.DialogResponses, platform.ResponseOK)
	}
	ss := New(p)
	ss.Init()
	if !ss.GW.LockedInWorkplane() {
		t.Skip("the new sketch is not in a workplane")
	}
	ss.ImportDxf(path)
	if ss.GW.LockedInWorkplane() {
		t.Fatal("the import did not leave the workplane")
	}
	ss.UndoUndo()
	if !ss.GW.LockedInWorkplane() {
		t.Error("undo did not bring the workplane back")
	}
}

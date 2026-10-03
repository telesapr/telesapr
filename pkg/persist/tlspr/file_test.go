package tlspr_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// newPart makes a new sketch with one extruded square, regenerated.
func newPart(t *testing.T) *sketch.Sketch {
	t.Helper()
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	sk.Active = sk.GroupOrder[len(sk.GroupOrder)-1]
	g := sketch.Group{Type: sketch.GroupDrawingWorkplane, Subtype: sketch.GroupWorkplaneByPointOrtho, Name: "sketch", Visible: true, Scale: 1}
	g.Predef.Q.W = 1
	g.Predef.Origin = core.HEntity{V: 0x10001} // the origin
	sk.Active = sk.AddGroupAfterActive(g)
	sk.GenerateAll()
	return sk
}

func TestSaveAndLoadFile(t *testing.T) {
	dir := t.TempDir()
	sk := newPart(t)
	path := filepath.Join(dir, "part.tlspr")
	sk.Units = core.UnitMeters
	if err := tlspr.SaveFile(sk, "", path, tlspr.Options{}); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(saved), "telesapr 2\n\nunits m\n") {
		t.Errorf("header:\n%.60s", saved)
	}

	doc, err := tlspr.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Units != core.UnitMeters || doc.Sketch.Units != core.UnitMeters || len(doc.Warnings) > 0 {
		t.Errorf("units %v, warnings %v", doc.Units, doc.Warnings)
	}
	if doc.Sketch.ReloadAllLinked == nil {
		t.Error("the loaded sketch can't reload its links")
	}
	doc.Sketch.GenerateAll()
	if again := tlspr.Encode(doc.Sketch, tlspr.Options{}); string(again) != string(saved) {
		t.Error("load, regenerate and save changed the file")
	}
}

func TestLoadFileErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := tlspr.LoadFile(filepath.Join(dir, "missing.tlspr")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}

	bad := filepath.Join(dir, "bad.tlspr")
	if err := os.WriteFile(bad, []byte("telesapr 1\ngroup {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := tlspr.LoadFile(bad)
	var se *tlspr.Error
	if !errors.As(err, &se) || !strings.HasPrefix(err.Error(), bad+": 2:1: ") {
		t.Errorf("syntax error: %v", err)
	}

	// A sketch without the reference workplanes can't regenerate.
	if _, err := tlspr.Load([]byte("telesapr 1\ngroup 0x1 sketch-3d \"r\" {\n}\nrequest 0x4 line-segment {\n  group 0x1\n  workplane 0x80020000\n}\n"),
		filepath.Join(dir, "broken.tlspr")); err == nil || !strings.Contains(err.Error(), "can't regenerate") {
		t.Errorf("unregenerable sketch: %v", err)
	}
}

func TestLinkedTlsprAndCancel(t *testing.T) {
	dir := t.TempDir()
	part := newPart(t)
	if err := tlspr.SaveFile(part, "", filepath.Join(dir, "sub", "part.tlspr"), tlspr.Options{}); err == nil {
		t.Fatal("saved into a folder that does not exist")
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := tlspr.SaveFile(part, "", filepath.Join(dir, "sub", "part.tlspr"), tlspr.Options{}); err != nil {
		t.Fatal(err)
	}

	// An assembly linking the part twice, saved in dir.
	asm := sketch.NewSketch()
	asm.NewFile()
	asm.GenerateAll()
	asm.Active = asm.GroupOrder[len(asm.GroupOrder)-1]
	for i := 0; i < 2; i++ {
		asm.Active = asm.AddGroupAfterActive(sketch.Group{
			Type: sketch.GroupLinked, Name: "part", LinkFile: "sub/part.tlspr", Scale: 1, MeshCombine: sketch.CombineAssemble,
		})
	}
	link.ReloadAllLinked(asm, filepath.Join(dir, "asm.tlspr"))
	asm.GenerateAll()
	linked := func(sk *sketch.Sketch) []*sketch.Group {
		var out []*sketch.Group
		for i := range sk.Groups {
			if sk.Groups[i].Type == sketch.GroupLinked {
				out = append(out, &sk.Groups[i])
			}
		}
		return out
	}
	for _, g := range linked(asm) {
		if len(g.ImpEntity) == 0 {
			t.Fatalf("group %v linked no entities", g.H)
		}
	}
	asmPath := filepath.Join(dir, "asm.tlspr")
	if err := tlspr.SaveFile(asm, filepath.Join(dir, "asm.tlspr"), asmPath, tlspr.Options{}); err != nil {
		t.Fatal(err)
	}
	doc, err := tlspr.LoadFile(asmPath)
	if err != nil {
		t.Fatal(err)
	}
	if g := linked(doc.Sketch)[1]; g.LinkFile != "sub/part.tlspr" || len(g.ImpEntity) == 0 {
		t.Errorf("linked group %q with %d entities", g.LinkFile, len(g.ImpEntity))
	}

	// Saved elsewhere, the link is rebased.
	other := filepath.Join(dir, "sub", "asm2.tlspr")
	if err := tlspr.SaveFile(doc.Sketch, asmPath, other, tlspr.Options{}); err != nil {
		t.Fatal(err)
	}
	if g := linked(doc.Sketch)[0]; g.LinkFile != "part.tlspr" {
		t.Errorf("rebased link %q", g.LinkFile)
	}

	// A missing link the user cancels at stops loading.
	if err := os.Remove(filepath.Join(dir, "sub", "part.tlspr")); err != nil {
		t.Fatal(err)
	}
	link.LocateMissingFile = func(string, bool, bool) (string, link.LocateResponse) { return "", link.LocateCancel }
	defer func() { link.LocateMissingFile = nil }()
	if _, err := tlspr.LoadFile(asmPath); !errors.Is(err, tlspr.ErrCancelled) {
		t.Errorf("cancelled load: %v", err)
	}
}

func TestLinkError(t *testing.T) {
	cause := errors.New("different volume")
	e := &tlspr.LinkError{Link: "D:/part.tlspr", Err: cause}
	if !errors.Is(e, cause) || !strings.Contains(e.Error(), "D:/part.tlspr") {
		t.Errorf("%v", e)
	}
}

package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/internal/golden"
	"github.com/spacesolve/spacesolve/pkg/persist/spcs"

	"github.com/spacesolve/spacesolve/pkg/platform"
	"github.com/spacesolve/spacesolve/pkg/platform/headless"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// TestImportDxf imports each DXF file in testdata/importdxf into a new
// sketch, in its workplane and free in 3d, and compares the saved sketch
// with the golden in testdata/importdxf/golden, byte for byte. Until the
// C++ comparison was frozen, the .slvs save was C++'s byte for byte.
func TestImportDxf(t *testing.T) {
	if testing.Short() {
		t.Skip("slow; skipped with -short")
	}
	root := filepath.Join("..", "..", "testdata", "importdxf")
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".dxf") &&
			!strings.Contains(path, string(filepath.Separator)+"golden"+string(filepath.Separator)) {
			rel, _ := filepath.Rel(root, path)
			files = append(files, rel)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) < 20 {
		t.Fatalf("only %d DXF files", len(files))
	}
	for _, f := range files {
		for _, in3d := range []bool{false, true} {
			name := f
			want := strings.TrimSuffix(f, ".dxf") + ".spcs"
			if in3d {
				name += " in 3d"
				want = strings.TrimSuffix(f, ".dxf") + ".3d.spcs"
			}
			t.Run(name, func(t *testing.T) {
				golden.Check(t, filepath.Join(root, "golden", want), importDxfFor(t, filepath.Join(root, f), in3d))
			})
		}
	}
}

// importDxfFor runs the harness's import on a file, and returns the
// saved sketch.
func importDxfFor(t testing.TB, path string, in3d bool) []byte {
	p := headless.New()
	for range 100 {
		p.DialogResponses = append(p.DialogResponses, platform.ResponseOK)
	}
	ss := New(p)
	ss.Init()
	if in3d {
		ss.GW.SetWorkplaneFreeIn3d()
	}
	ss.ImportDxf(path)
	ss.GenerateAll(sketch.GenerateUntilActive)
	ss.ScheduleShowTW()
	// A save regenerates again.
	ss.SK.GenerateAll()
	return spcs.Encode(ss.SK, spcs.Options{})
}

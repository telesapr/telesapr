package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/script/api"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/platform/headless"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestImportDxf imports a DXF file through the interface, with its
// dialogs, into a new sketch, in its workplane and free in 3d, and
// compares the saved sketch with the golden in testdata/importdxf/golden,
// byte for byte. It takes the first file of each kind; every file is
// compared with its golden by TestImport in pkg/persist/dxf, which
// the interface's import calls.
func TestImportDxf(t *testing.T) {
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
	slices.Sort(files)
	if len(files) < 20 {
		t.Fatalf("only %d DXF files", len(files))
	}
	files = files[:2]
	for _, f := range files {
		for _, in3d := range []bool{false, true} {
			name := f
			want := strings.TrimSuffix(f, ".dxf") + ".tlspr"
			if in3d {
				name += " in 3d"
				want = strings.TrimSuffix(f, ".dxf") + ".3d.tlspr"
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
	return tlspr.Encode(ss.SK, tlspr.Options{})
}

// TestImportDxfPathEquivalence imports DXF files through the interface
// and through File.ImportDXF into new sketches, and requires the same
// saved file from both: a flat file, which stays in the workplane, and
// one with a Z coordinate, which leaves it. Both reach
// dxf.ImportActive.
func TestImportDxfPathEquivalence(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "importdxf")
	for _, name := range []string{"basic.dxf", "extrusions.dxf", "polylines.dxf"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, name)
			// The solver is not a fixed point, so both sides regenerate
			// the same number of times.
			pl := headless.New()
			for range 10 {
				pl.DialogResponses = append(pl.DialogResponses, platform.ResponseOK)
			}
			ss := New(pl)
			ss.Init()
			ss.ImportDxf(path)
			ss.SK.GenerateAll()
			fromUI := tlspr.Encode(ss.SK, tlspr.Options{})

			doc := api.New()
			if _, err := doc.File.ImportDXF(path); err != nil {
				t.Fatal(err)
			}
			doc.Host().Sketch().GenerateAll()
			fromScript := tlspr.Encode(doc.Host().Sketch(), tlspr.Options{})
			if string(fromUI) != string(fromScript) {
				t.Fatalf("the interface and the script save different files, at %s",
					firstDiffLine(string(fromUI), string(fromScript)))
			}
		})
	}
}

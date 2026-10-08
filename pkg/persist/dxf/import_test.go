package dxf

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// importFor imports a DXF file into a new sketch, in its workplane or
// free in 3d, as the application's File > Import does, and returns
// the sketch as saved.
func importFor(t testing.TB, path string, in3d bool) []byte {
	t.Helper()
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	g := sk.Group(sk.ActiveGroup())
	if in3d {
		g.ActiveWorkplane = core.HEntity{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	asConstruction := true
	if !core.IsFreeIn3D(g.ActiveWorkplane) {
		if Has3D(data) {
			g.ActiveWorkplane = core.HEntity{}
		} else {
			asConstruction = false
		}
	}
	textHeight := sk.StyleTextHeight(core.HStyle{V: core.StyleConstraint}, sketch.DefaultView().Scale)
	if _, err := Import(sk, data, Options{AsConstruction: asConstruction, TextHeight: textHeight}); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAllTyped(sketch.GenerateUntilActive)
	sk.GenerateAll()
	return tlspr.Encode(sk, tlspr.Options{})
}

// TestImport imports each DXF file in testdata/importdxf, in a
// workplane and in 3d, and compares the saved sketch with the golden
// the application's import made.
func TestImport(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..", "testdata", "importdxf")
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
	for _, f := range files {
		for _, in3d := range []bool{false, true} {
			name, want := f, strings.TrimSuffix(f, ".dxf")+".tlspr"
			if in3d {
				name, want = f+" in 3d", strings.TrimSuffix(f, ".dxf")+".3d.tlspr"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				golden.Check(t, filepath.Join(root, "golden", want), importFor(t, filepath.Join(root, f), in3d))
			})
		}
	}
}

// TestCorrupt refuses what isn't DXF.
func TestCorrupt(t *testing.T) {
	t.Parallel()
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	if _, err := Import(sk, []byte("not a drawing"), Options{TextHeight: 1}); err == nil {
		t.Error("imported")
	}
}

package spcs_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/check"
	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/ttf"
)

var testdata = filepath.Join("..", "..", "..", "testdata")

// TestMain gives TrueType text the font the fixtures use.
func TestMain(m *testing.M) {
	ttf.SetFontFiles([]string{filepath.Join(testdata, "Gentium-R.ttf")})
	os.Exit(m.Run())
}

// TestV1Corpus opens every file of testdata/spcs/v1, which every later
// reader must load: it decodes without warnings, loads with its links,
// and regenerates with every solid sound. fields.spcs, which is not a
// model, is only decoded, and must encode to the same bytes.
func TestV1Corpus(t *testing.T) {
	root := filepath.Join(testdata, "spcs", "v1")
	n := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".spcs") {
			return err
		}
		n++
		t.Run(filepath.ToSlash(strings.TrimPrefix(path, root)), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := spcs.Decode(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Warnings) > 0 || doc.Version != 1 {
				t.Fatalf("version %d, warnings %v", doc.Version, doc.Warnings)
			}
			if filepath.Base(path) == "fields.spcs" {
				if again := doc.Encode(); !bytes.Equal(again, data) {
					t.Error("does not encode to the same bytes")
				}
				return
			}
			srf.SetChordTolMm(0)
			loaded, err := spcs.LoadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sk := loaded.Sketch
			sk.GenerateAll()
			for i := range sk.Groups {
				g := &sk.Groups[i]
				if g.SketchDiagnostic != nil {
					t.Errorf("group %q: %v", g.Name, g.SketchDiagnostic)
				}
				if r := check.Group(sk, g); !r.Empty() && !r.Sound() {
					t.Errorf("group %q is not sound: %s", g.Name, r.String())
				}
				if g.LinkFile != "" && len(g.ImpEntity) == 0 {
					t.Errorf("group %q linked nothing from %s", g.Name, g.LinkFile)
				}
			}
			for file, img := range sk.Images {
				if img == nil {
					t.Errorf("image %s did not load", file)
				}
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n < 9 {
		t.Errorf("only %d files", n)
	}
}

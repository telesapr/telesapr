package spcs_test

import (
	"bytes"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/check"
	"github.com/spacesolve/spacesolve/pkg/persist/spcs"
	"github.com/spacesolve/spacesolve/pkg/sketch"
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
// model, is only decoded: it must upgrade to a version 2 file that
// holds the same lengths in its unit and encodes the same again.
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
				upgraded := doc.Encode()
				v2, err := spcs.Decode(upgraded)
				if err != nil || v2.Version != 2 || len(v2.Warnings) > 0 {
					t.Fatalf("upgrade: %v, version %d, warnings %v", err, v2.Version, v2.Warnings)
				}
				if again := v2.Encode(); !bytes.Equal(again, upgraded) {
					t.Error("the upgraded file does not encode to the same bytes")
				}
				sameLengths(t, doc.Sketch, v2.Sketch)
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

// sameLengths checks that two sketches hold the same constraint and
// group values, but for the rounding of a change of unit.
func sameLengths(t *testing.T, a, b *sketch.Sketch) {
	t.Helper()
	near := func(x, y float64) bool {
		return x == y || math.Abs(x-y) <= 1e-12*math.Max(math.Abs(x), math.Abs(y))
	}
	for i := range a.Constraints {
		if ca, cb := &a.Constraints[i], &b.Constraints[i]; !near(ca.ValA, cb.ValA) || ca.Expr != cb.Expr {
			t.Errorf("constraint %v: %v %q, then %v %q", ca.H, ca.ValA, ca.Expr, cb.ValA, cb.Expr)
		}
	}
	for i := range a.Groups {
		if ga, gb := &a.Groups[i], &b.Groups[i]; !near(ga.ValB, gb.ValB) || ga.ExprB != gb.ExprB {
			t.Errorf("group %v: pitch %v %q, then %v %q", ga.H, ga.ValB, ga.ExprB, gb.ValB, gb.ExprB)
		}
	}
}

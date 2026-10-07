package convert_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/ttf"
)

var testdata = filepath.Join("..", "..", "..", "testdata")

// TestMain gives TrueType text the font the fixtures use.
func TestMain(m *testing.M) {
	ttf.SetFontFiles([]string{filepath.Join(testdata, "Gentium-R.ttf")})
	os.Exit(m.Run())
}

// notFixedPoints are the fixtures whose solve moves the last digits of
// solved values when they are regenerated, whatever the format: loaded
// from their own .slvs save they drift the same way. They round trip
// through the format exactly, but not through a regeneration.
var notFixedPoints = map[string]string{
	"constraint/same_orientation/normal.slvs": "drifts at every regeneration",
	"edit/tangent.slvs":                       "drifts at every regeneration",
	"edit/tangent_arc.slvs":                   "drifts at every regeneration",
	"edit/tangent_arc_modify.slvs":            "drifts at every regeneration",
	"edit/tangent_manual.slvs":                "drifts at every regeneration",
	"edit/tangent_modify.slvs":                "drifts at every regeneration",
	"thread/blind_m6.slvs":                    "drifts at every regeneration",
	"thread/bolt_m8x30.slvs":                  "drifts at every regeneration",
	"thread/lead_tr20.slvs":                   "drifts at every regeneration",
	"thread/screw_m8.slvs":                    "drifts at every regeneration",
	"thread/screw_m8_lh.slvs":                 "drifts at every regeneration",
	"thread/screw_unc.slvs":                   "drifts at every regeneration",
	"thread/triple_m12.slvs":                  "drifts at every regeneration",
	// Written by older versions; they settle after two and three.
	"constraint/large_dimensions/normal.slvs":   "settles after more regenerations",
	"constraint/pt_on_circle/negative_dia.slvs": "settles after more regenerations",
}

// invalidFixture is not a valid SolveSpace file; it does not import.
const invalidFixture = "solver/param_point_distance.slvs"

// fixtures lists every .slvs file under testdata, as paths from
// testdata with forward slashes.
func fixtures(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(testdata, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".slvs") {
			rel, err := filepath.Rel(testdata, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	return files
}

// copyDir copies the files of testdata's directory dir, without its
// subdirectories, under root, and returns the copy's path. Conversion
// writes beside the files it converts, and testdata stays untouched.
func copyDir(t *testing.T, root, dir string) string {
	t.Helper()
	out := filepath.Join(root, filepath.FromSlash(dir))
	if _, err := os.Stat(out); err == nil {
		return out
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(testdata, filepath.FromSlash(dir)))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(testdata, filepath.FromSlash(dir), e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// firstDiff describes the first line where a and b differ.
func firstDiff(a, b []byte) string {
	al, bl := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return fmt.Sprintf("line %d: %q, then %q", i+1, al[i], bl[i])
		}
	}
	return fmt.Sprintf("%d lines, then %d", len(al), len(bl))
}

// goldenPath is the .tlspr golden of a fixture: its import, which is
// what loading and regenerating it gives. For the fixtures C++ saved,
// that was C++'s own save of them, byte for byte, until the C++
// comparison was frozen (ROADMAP-JUMP.md, package 5.1).
func goldenPath(rel string) string {
	return filepath.Join(testdata, "tlspr", "golden", filepath.FromSlash(strings.TrimSuffix(rel, ".slvs"))+".tlspr")
}

// TestFixturesRoundTrip imports every fixture and requires:
//
//   - decoding the .tlspr file and encoding it again gives the same
//     bytes, so the format loses nothing;
//   - loading it, regenerating and saving gives the same bytes, except
//     for the fixtures in notFixedPoints;
//   - every group's check report matches testdata/invariants.txt, the
//     baseline the .slvs loader recorded;
//   - each fixture imports to its golden in testdata/tlspr/golden.
//     TELESAPR_REGEN_GOLDEN=1 rewrites the goldens
//     (scripts/regen-tlspr-golden.sh).
func TestFixturesRoundTrip(t *testing.T) {
	baseline, _, err := check.ReadBaseline(filepath.Join(testdata, "invariants.txt"))
	if err != nil {
		t.Fatal(err)
	}
	files := fixtures(t)
	regen := golden.Regen()
	root := t.TempDir()

	for i, rel := range files {
		if raceDetector && i%5 != 0 && !regen {
			continue
		}
		t.Run(rel, func(t *testing.T) {
			dir := copyDir(t, root, filepath.ToSlash(filepath.Dir(rel)))
			src := filepath.Join(dir, filepath.Base(rel))
			dst := convert.SiblingPath(src)

			// The fixtures were saved in a fresh process, whose chord
			// tolerance starts at 0; linking an IDF file uses it.
			srf.SetChordTolMm(0)
			_, res, err := convert.File(src, dst, true)
			if rel == invalidFixture {
				if err == nil {
					t.Fatal("imported an invalid file")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Failed) > 0 || len(res.Warnings) > 0 {
				t.Fatalf("failed %v, warnings %v", res.Failed, res.Warnings)
			}
			saved, err := os.ReadFile(dst)
			if err != nil {
				t.Fatal(err)
			}

			doc, err := tlspr.Decode(saved)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Warnings) > 0 {
				t.Errorf("warnings: %v", doc.Warnings)
			}
			if again := doc.Encode(); !bytes.Equal(again, saved) {
				t.Errorf("decode and encode changed the file: %s", firstDiff(saved, again))
			}

			srf.SetChordTolMm(0)
			loaded, err := tlspr.LoadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			sk := loaded.Sketch
			sk.GenerateAll()
			resaved := tlspr.Encode(sk, tlspr.Options{})
			if notFixedPoints[rel] == "" && !bytes.Equal(resaved, saved) {
				t.Errorf("load, regenerate and save changed the file: %s", firstDiff(saved, resaved))
			}

			for _, hg := range sk.GroupOrder {
				r := check.Group(sk, sk.GetGroup(hg))
				line := fmt.Sprintf("%s g%x %s", rel, hg.V, r.String())
				want, ok := baseline[check.BaselineKey(line)]
				if !ok {
					t.Errorf("not in the baseline: %s", line)
					continue
				}
				if msg := check.CompareBaseline(line, want, check.BaselineRelTol); msg != "" {
					t.Errorf("%s\n got: %s\nwant: %s", msg, line, want)
				}
			}

			golden.Check(t, goldenPath(rel), saved)
		})
	}
}

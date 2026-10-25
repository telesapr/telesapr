package convert_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/convert"
	"github.com/telesapr/telesapr/pkg/persist/tlspr"
	"github.com/telesapr/telesapr/pkg/ttf"
)

var testdata = filepath.Join("..", "..", "..", "testdata")

// TestMain gives TrueType text the font the fixtures use.
func TestMain(m *testing.M) {
	ttf.DefaultFonts = ttf.NewFontList([]string{filepath.Join(testdata, "Gentium-R.ttf")})
	os.Exit(m.Run())
}

// notFixedPoints are the fixtures whose solve moves the last digits of
// solved values when they are regenerated, whatever the format: loaded
// from their own .slvs save they drift the same way. They round trip
// through the format exactly, but not through a regeneration.
var notFixedPoints = map[string]string{
	"constraint/same_orientation/normal.slvs": "drifts at every regeneration",
	// Written by older versions; they settle after two and three.
	"constraint/large_dimensions/normal.slvs":   "settles after more regenerations",
	"constraint/pt_on_circle/negative_dia.slvs": "settles after more regenerations",
}

// invalidFixture is not a valid SolveSpace file; it does not import.
const invalidFixture = "solver/param_point_distance.slvs"

// slvsFixtures lists every .slvs file under testdata, as paths from
// testdata with forward slashes.
func slvsFixtures(t *testing.T) []string {
	t.Helper()
	files, err := fixtures.Walk(".slvs")
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

// goldenPath is the .tlspr golden of a fixture: its import, which is
// what loading and regenerating it gives. For the fixtures SolveSpace saved,
// that was SolveSpace's own save of them, byte for byte, until the SolveSpace
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
//   - every group's check report matches testdata/invariants.txt, and
//     every line of it is produced; TELESAPR_REGEN_GOLDEN=1 rewrites it
//     too;
//   - each fixture imports to its golden in testdata/tlspr/golden.
//     TELESAPR_REGEN_GOLDEN=1 rewrites the goldens
//     (scripts/regen-tlspr-golden.sh).
func TestFixturesRoundTrip(t *testing.T) {
	t.Parallel()
	invariantsPath := filepath.Join(testdata, "invariants.txt")
	baseline, order, err := check.ReadBaseline(invariantsPath)
	if err != nil {
		t.Fatal(err)
	}
	files := slvsFixtures(t)
	regen := golden.Regen()
	root := t.TempDir()
	var lines []string
	sampled := false

	for i, rel := range files {
		if fixtures.RaceDetector && i%5 != 0 && !regen {
			sampled = true
			continue
		}
		t.Run(rel, func(t *testing.T) {
			dir := copyDir(t, root, filepath.ToSlash(filepath.Dir(rel)))
			src := filepath.Join(dir, filepath.Base(rel))
			dst := convert.SiblingPath(src)

			// The fixtures were saved in a fresh process, whose chord
			// tolerance starts at 0; linking an IDF file uses it.
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
				t.Errorf("decode and encode changed the file: %s", golden.FirstDifference(saved, again))
			}

			loaded, err := tlspr.LoadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			sk := loaded.Sketch
			sk.GenerateAll()
			resaved := tlspr.Encode(sk, tlspr.Options{})
			if notFixedPoints[rel] == "" && !bytes.Equal(resaved, saved) {
				t.Errorf("load, regenerate and save changed the file: %s", golden.FirstDifference(saved, resaved))
			}

			for _, hg := range sk.GroupOrder {
				r := check.Group(sk, sk.Group(hg))
				line := fmt.Sprintf("%s g%x %s", rel, hg.V, r.String())
				lines = append(lines, line)
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
	if regen {
		data := "# Invariant baseline: one line per fixture and group, from\n" +
			"# TestFixturesRoundTrip in pkg/persist/convert. TELESAPR_REGEN_GOLDEN=1\n" +
			"# rewrites it; review its diff.\n" + strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(invariantsPath, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if sampled || t.Failed() {
		return
	}
	seen := map[string]bool{}
	for _, line := range lines {
		seen[check.BaselineKey(line)] = true
	}
	for _, key := range order {
		if !seen[key] {
			t.Errorf("in the baseline but not produced: %s", baseline[key])
		}
	}
}

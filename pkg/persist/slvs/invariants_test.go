package slvs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/srf"
)

// invariantsPath is the baseline TestInvariants compares with.
var invariantsPath = filepath.Join("..", "..", "..", "testdata", "invariants.txt")

// invariantFixtures lists every .slvs file under testdata, as paths
// from testdata with forward slashes.
func invariantFixtures(t *testing.T) []string {
	root := filepath.Join("..", "..", "..", "testdata")
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".slvs") {
			rel, err := filepath.Rel(root, path)
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

// invariantLines loads a fixture, regenerates it, and returns one line
// per group: the fixture, the group's handle and its check.Report. A
// fixture that does not load, on purpose, gives one line with the error.
func invariantLines(fixture string) []string {
	// As in a fresh process, whose chord tolerance starts at 0; linking
	// an IDF file uses it, so it must not be what the fixture before
	// left.
	srf.SetChordTolMm(0)
	doc, err := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(fixture)))
	if err != nil {
		return []string{fmt.Sprintf("%s error %q", fixture, err.Error())}
	}
	sk := doc.Sketch
	sk.GenerateAll()
	var lines []string
	for _, hg := range sk.GroupOrder {
		r := check.Group(sk, sk.Group(hg))
		lines = append(lines, fmt.Sprintf("%s g%x %s", fixture, hg.V, r.String()))
	}
	return lines
}

// TestInvariants loads every fixture, regenerates it, checks the solid
// of each group, and compares the reports with testdata/invariants.txt.
// Counts and flags must match exactly; volume, centre of mass and
// bounding box to check.BaselineRelTol. TELESAPR_REGEN_INVARIANTS=1
// rewrites the file; review its diff before committing.
//
// The baseline was recorded while the kernel's output was verified
// byte for byte against C++ SolveSpace, so it is the judge of every
// later kernel change.
func TestInvariants(t *testing.T) {
	fixtures := invariantFixtures(t)
	var got []string
	for _, f := range fixtures {
		got = append(got, invariantLines(f)...)
	}

	if os.Getenv("TELESAPR_REGEN_INVARIANTS") != "" {
		data := "# Invariant baseline: one line per fixture and group, from TestInvariants\n" +
			"# in pkg/persist/slvs. Regenerate with TELESAPR_REGEN_INVARIANTS=1.\n" +
			strings.Join(got, "\n") + "\n"
		if err := os.WriteFile(invariantsPath, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, order, err := check.ReadBaseline(invariantsPath)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, line := range got {
		key := check.BaselineKey(line)
		seen[key] = true
		w, ok := want[key]
		if !ok {
			t.Errorf("not in the baseline: %s", line)
			continue
		}
		if msg := check.CompareBaseline(line, w, check.BaselineRelTol); msg != "" {
			t.Errorf("%s: %s\n got: %s\nwant: %s", key, msg, line, w)
		}
	}
	for _, key := range order {
		if !seen[key] {
			t.Errorf("in the baseline but not produced: %s", want[key])
		}
	}
}

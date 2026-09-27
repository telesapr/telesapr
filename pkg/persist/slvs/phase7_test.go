package slvs_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
)

// TestPhase7AllFixturesLoad walks every .slvs fixture in testdata/
// and verifies it loads cleanly with no errors. This is the
// Phase 7 acceptance test: any fixture that fails to load points
// to a missing field handler in pkg/persist/slvs/slvs.go.
func TestPhase7AllFixturesLoad(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	persistDir := filepath.Dir(dir)
	pkgDir := filepath.Dir(persistDir)
	root := filepath.Join(pkgDir, "..", "testdata")
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".slvs" {
			return nil
		}
		doc, err := slvs.LoadFile(path)
		rel, _ := filepath.Rel(root, path)
		if roundTripExclusions[filepath.ToSlash(rel)] == "not a valid file" {
			if err == nil {
				t.Errorf("%s: loaded, but C++ can't load it", path)
			}
			return nil
		}
		if err != nil {
			t.Errorf("load %s: %v", path, err)
			return nil
		}
		if doc.Sketch == nil {
			t.Errorf("nil sketch for %s", path)
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if count == 0 {
		t.Fatalf("no fixtures loaded")
	}
	t.Logf("loaded %d fixtures successfully", count)
}

// roundTripExclusions are the fixtures TestPhase7AllFixturesRoundTrip
// doesn't compare, with the reason. The others are what current C++
// writes when it loads and saves them (checked with a C++ harness).
var roundTripExclusions = map[string]string{
	// Written by older versions: current C++ also rewrites them (for
	// example it marks the reference origins as construction).
	"constraint/angle/skew.slvs":                "older version",
	"constraint/equal_line_arc_len/pi.slvs":     "older version",
	"constraint/equal_line_arc_len/tau.slvs":    "older version",
	"constraint/pt_line_distance/extended.slvs": "older version",
	"constraint/pt_on_circle/negative_dia.slvs": "older version",
	"constraint/large_dimensions/normal.slvs":   "older version",
	// The solved values differ from C++'s in the last digit.
	"constraint/same_orientation/normal.slvs": "last-digit solver difference",
	// The meshes and shells C++ appends for linking aren't written.
	"group/translate_asy/normal.slvs": "shell not written",
	"group/translate_nd/normal.slvs":  "shell not written",
	// LINKED import isn't ported.
	"group/link/normal.slvs":    "LINKED import",
	"request/image/linked.slvs": "LINKED import",
	// Not a valid SolveSpace file; C++ can't load it either.
	"solver/param_point_distance.slvs": "not a valid file",
}

// TestPhase7AllFixturesRoundTrip loads each fixture and saves it
// again, requiring the bytes C++ itself writes; see
// roundTripExclusions.
func TestPhase7AllFixturesRoundTrip(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	persistDir := filepath.Dir(dir)
	pkgDir := filepath.Dir(persistDir)
	root := filepath.Join(pkgDir, "..", "testdata")
	count := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".slvs" {
			return nil
		}
		// Skip boolean fixtures (mesh/shell blocks are not round-tripped).
		dirName := filepath.Base(filepath.Dir(path))
		if len(dirName) >= 7 && dirName[:7] == "boolean" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if _, ok := roundTripExclusions[filepath.ToSlash(rel)]; ok {
			return nil
		}
		doc, err := slvs.LoadFile(path)
		if err != nil {
			t.Errorf("load %s: %v", path, err)
			return nil
		}
		saved := doc.Save()
		original, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("rewind %s: %v", path, err)
			return nil
		}
		if string(saved) != string(original) {
			t.Errorf("roundtrip mismatch for %s", path)
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if count == 0 {
		t.Fatalf("no fixtures round-tripped")
	}
	t.Logf("round-tripped %d fixtures successfully", count)
}

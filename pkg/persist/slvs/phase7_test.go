package slvs_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
	"github.com/spacesolve/spacesolve/pkg/srf"
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
// writes when it loads and saves them (checked with a C++ harness);
// for these, TestExcludedFixturesVsCppSave compares with what C++
// writes instead.
var roundTripExclusions = map[string]string{
	// Written by older versions: current C++ also rewrites them (for
	// example it marks the reference origins as construction).
	"constraint/angle/skew.slvs":                "older version",
	"constraint/equal_line_arc_len/pi.slvs":     "older version",
	"constraint/equal_line_arc_len/tau.slvs":    "older version",
	"constraint/pt_line_distance/extended.slvs": "older version",
	"constraint/pt_on_circle/negative_dia.slvs": "older version",
	"constraint/large_dimensions/normal.slvs":   "older version",
	// Saved before C++ appended the running shell, or with an older
	// boolean's curve numbering: current C++ writes what Go does.
	"constraint/pt_face_distance/normal.slvs":    "older version",
	"constraint/pt_face_distance/reference.slvs": "older version",
	"constraint/pt_on_face/normal.slvs":          "older version",
	"group/boolean_coplanar_union/normal.slvs":   "older version",
	"group/boolean_knife_edge/normal.slvs":       "older version",
	"group/boolean_tangent_crossing/normal.slvs": "older version",
	"group/boolean_tangent_edge/normal.slvs":     "older version",
	"group/boolean_tangent_fillet/normal.slvs":   "older version",
	"group/boolean_tangent_spline/normal.slvs":   "older version",
	"group/translate_asy/normal.slvs":            "older version",
	"group/translate_nd/normal.slvs":             "older version",
	// Written by SolveSpace 2.0, for group/link/normal.slvs to link.
	"group/link/rect_v20.slvs": "older version",
	// C++'s own load and save of these changes the last digits of
	// solved values.
	"edit/split_arc_cubic.slvs":    "C++ doesn't reproduce it",
	"edit/tangent.slvs":            "C++ doesn't reproduce it",
	"edit/tangent_arc.slvs":        "C++ doesn't reproduce it",
	"edit/tangent_arc_modify.slvs": "C++ doesn't reproduce it",
	"edit/tangent_manual.slvs":     "C++ doesn't reproduce it",
	"edit/tangent_modify.slvs":     "C++ doesn't reproduce it",
	// Not a valid SolveSpace file; C++ can't load it either.
	"solver/param_point_distance.slvs": "not a valid file",
	// Threads C++ saved from the files Go made (TestThreadVsCpp pins
	// those); opened and saved again, C++ itself moves the last digits
	// of their solved values, every time.
	"thread/lead_tr20.slvs": "not a fixed point of C++'s save",
	"thread/screw_unc.slvs": "not a fixed point of C++'s save",
	// Made in the application, not by C++ or a generator a script
	// reruns (scripts/regen-thread-golden.sh remakes the others): it
	// lacks the scale of its first two groups, which C++ always writes.
	"thread/bolt_m8x30.slvs": "not saved by C++",
}

// withCppHeader is a saved file with the header C++ writes: the
// version line and two blank lines. Most fixtures have one blank line.
func withCppHeader(file []byte) string {
	s := string(file)
	i := strings.IndexByte(s, '\n')
	return s[:i] + "\n\n\n" + strings.TrimLeft(s[i:], "\n")
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
		rel, _ := filepath.Rel(root, path)
		if _, ok := roundTripExclusions[filepath.ToSlash(rel)]; ok {
			return nil
		}
		// C++ saved the fixtures in a fresh process, whose chord
		// tolerance starts at 0; linking an IDF file uses it.
		srf.SetChordTolMm(0)
		doc, err := slvs.LoadFile(path)
		if err != nil {
			t.Errorf("load %s: %v", path, err)
			return nil
		}
		// C++ AfterNewFile generates (and solves) the loaded sketch
		// before the harness saves it, and SaveToFile generates again.
		doc.Sketch.GenerateAll()
		saved := doc.Save()
		original, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("rewind %s: %v", path, err)
			return nil
		}
		if string(saved) != withCppHeader(original) {
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

// TestExcludedFixturesVsCppSave requires what C++ writes when it opens
// and saves the fixtures TestPhase7AllFixturesRoundTrip excludes
// (testdata/cppsave, from scripts/regen-cpp-saves.sh).
func TestExcludedFixturesVsCppSave(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata")
	for rel, reason := range roundTripExclusions {
		if strings.HasPrefix(reason, "not") {
			continue
		}
		t.Run(rel, func(t *testing.T) {
			srf.SetChordTolMm(0)
			doc, err := slvs.LoadFile(filepath.Join(root, rel))
			if err != nil {
				t.Fatal(err)
			}
			// C++ AfterNewFile generates the loaded sketch before the
			// harness saves it.
			doc.Sketch.GenerateAll()
			want, err := os.ReadFile(filepath.Join(root, "cppsave", strings.TrimSuffix(rel, ".slvs")+".cppsave"))
			if err != nil {
				t.Fatal(err)
			}
			if string(doc.Save()) != string(want) {
				t.Errorf("save differs from C++'s")
			}
		})
	}
}

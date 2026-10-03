package slvs_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/slvs"
)

// TestPhase7AllFixturesLoad walks every .slvs fixture in testdata/
// and verifies it loads cleanly with no errors: any fixture that fails
// to load points to a missing field handler in pkg/persist/slvs/slvs.go.
// What a fixture regenerates to is pinned by its import golden
// (TestFixturesRoundTrip in pkg/persist/convert).
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
		if filepath.ToSlash(rel) == invalidFixture {
			if err == nil {
				t.Errorf("%s: loaded, but C++ can't load it", path)
			}
			return nil
		}
		if err != nil {
			t.Errorf("load %s: %v", path, err)
			return nil
		}
		if doc.Sketch == nil || len(doc.Sketch.Groups) == 0 || len(doc.Sketch.Entities) == 0 {
			t.Errorf("no sketch, groups or entities for %s", path)
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

// invalidFixture is not a valid SolveSpace file; C++ can't load it
// either.
const invalidFixture = "solver/param_point_distance.slvs"

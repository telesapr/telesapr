package srf_test

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestFixturesReachCurveRedo checks that the fixtures have Booleans in
// which a surface's exact intersection curve follows another
// surface's, so that TestParallelSameAsSerial (pkg/persist/slvs) covers
// making such a curve over.
func TestFixturesReachCurveRedo(t *testing.T) {
	before := srf.SinkRedone()
	err := filepath.WalkDir(filepath.Join("..", "..", "testdata"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".slvs" {
			return err
		}
		srf.SetChordTolMm(0)
		doc, err := slvs.LoadFile(path)
		if err != nil {
			return nil // not a valid file
		}
		n := srf.SinkRedone()
		doc.Sketch.GenerateAll()
		if srf.SinkRedone() > n {
			t.Logf("%s: %d curves made over", path, srf.SinkRedone()-n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if srf.SinkRedone() == before {
		t.Error("no fixture has an exact curve made over")
	}
}

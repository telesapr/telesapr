package srf_test

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/srf"
)

// TestFixturesReachCurveRedo logs the fixtures in whose Booleans a
// surface's exact intersection curve follows another surface's, which
// TestParallelSameAsSerial (pkg/persist/slvs) then covers making over.
// Since roadmap 13.3 drops the surfaces a Boolean leaves untrimmed,
// which were what met the same curve twice, no fixture does; the
// making over is covered by TestIntersectionCurvesAtOnceSameAsInOrder in
// this package.
func TestFixturesReachCurveRedo(t *testing.T) {
	before := srf.SinkRedone()
	err := filepath.WalkDir(filepath.Join("..", "..", "testdata"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".slvs" {
			return err
		}
		srf.SetChordTolMm(0)
		doc, err := slvs.LoadFile(path)
		if err != nil {
			return nil //nolint:nilerr // a file that isn't a model is skipped
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
	t.Logf("%d curves made over in all", srf.SinkRedone()-before)
}

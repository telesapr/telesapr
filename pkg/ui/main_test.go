package ui

import (
	"errors"
	"testing"

	"github.com/telesapr/telesapr/pkg/persist/slvs"
)

// loadSlvs opens a .slvs fixture as the sketch being edited, as Load
// opens a .tlspr file, for the tests and scenarios whose goldens were
// recorded from those fixtures. The product only imports .slvs files.
func loadSlvs(t testing.TB, ss *SolveSpaceUI, path string) bool {
	t.Helper()
	doc, err := slvs.LoadFile(path)
	if err != nil {
		if errors.Is(err, slvs.ErrCancelled) {
			return false
		}
		t.Fatalf("load %s: %v", path, err)
	}
	// A .slvs file has no unit; it is shown in the one in use.
	doc.Sketch.Units = ss.ViewUnits
	ss.SaveFile = path
	ss.setSketch(doc.Sketch)
	ss.AddToRecentList(path)
	ss.AfterNewFile()
	ss.SK.Unsaved = false
	return true
}

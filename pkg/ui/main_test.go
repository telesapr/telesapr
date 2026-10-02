package ui

import (
	"os"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/internal/slvswrite"
)

// TestMain lets the interface open and save .slvs files, as the tests
// that compare with C++ need until ROADMAP-JUMP.md package 5.1 freezes
// them. A test of the product's own behaviour with .slvs files sets
// legacySlvsSave to nil for its duration (withoutLegacySlvs).
func TestMain(m *testing.M) {
	legacySlvsSave = slvswrite.SaveRegenerated
	os.Exit(m.Run())
}

// withoutLegacySlvs runs the rest of the test as the product: .slvs
// files are imported, never opened or saved.
func withoutLegacySlvs(t *testing.T) {
	t.Helper()
	saved := legacySlvsSave
	legacySlvsSave = nil
	t.Cleanup(func() { legacySlvsSave = saved })
}

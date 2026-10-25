package ui

import (
	"errors"
	"os"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// TestMain gives TrueType text the font the models and scenarios use,
// once for every test, so that the tests can run side by side.
func TestMain(m *testing.M) {
	ttf.DefaultFonts = ttf.NewFontList([]string{fixtures.Path("Gentium-R.ttf")})
	os.Exit(m.Run())
}

// loadModel opens a model as File > Open does; false when the user gave
// up at a missing linked file.
func loadModel(t testing.TB, ss *Application, path string) bool {
	t.Helper()
	if err := ss.Load(path); err != nil {
		if errors.Is(err, errDeclined) {
			return false
		}
		t.Fatalf("load %s: %v", path, err)
	}
	return true
}

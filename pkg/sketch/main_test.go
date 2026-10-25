package sketch_test

import (
	"os"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/ttf"
)

// TestMain gives TrueType text the font the models of the corpus use.
func TestMain(m *testing.M) {
	ttf.DefaultFonts = ttf.NewFontList([]string{fixtures.Path("Gentium-R.ttf")})
	os.Exit(m.Run())
}

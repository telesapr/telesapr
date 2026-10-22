package script

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/ttf"
)

// TestMain gives TrueType text the font the fixtures use, which the
// round trip's scripts set their text in as the models do.
func TestMain(m *testing.M) {
	ttf.DefaultFonts = ttf.NewFontList([]string{filepath.Join("..", "..", "testdata", "Gentium-R.ttf")})
	os.Exit(m.Run())
}

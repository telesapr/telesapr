package slvs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/ttf"
)

// TestMain gives TrueType text the font the fixtures use, as the C++
// test harness does (it puts test/Gentium-R.ttf on the font list).
func TestMain(m *testing.M) {
	ttf.SetFontFiles([]string{filepath.Join("..", "..", "..", "testdata", "Gentium-R.ttf")})
	os.Exit(m.Run())
}

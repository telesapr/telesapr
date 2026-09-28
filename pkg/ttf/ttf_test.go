package ttf

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTTFVsCpp requires the font list, cap heights, aspect ratios and
// PlotString's Beziers to be C++'s, bit for bit, for Gentium and the
// builtin font over testdata/ttf/strings.txt (golden data from
// scripts/regen-ttf-golden.sh).
func TestTTFVsCpp(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")
	strs, err := os.ReadFile(filepath.Join(root, "ttf", "strings.txt"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(root, "ttf", "cpp-dump.txt.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	want, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}

	got := dump([]string{filepath.Join(root, "Gentium-R.ttf")}, lines(string(strs)))
	wl, gl := strings.Split(string(want), "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if wl[i] != gl[i] {
			t.Fatalf("line %d differs:\nC++: %s\nGo:  %s", i+1, wl[i], gl[i])
		}
	}
	if len(wl) != len(gl) {
		t.Fatalf("C++ dumps %d lines, Go %d", len(wl), len(gl))
	}
}

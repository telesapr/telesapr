package ttf

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
)

// TestTTF requires the font list, cap heights, aspect ratios and
// PlotString's Beziers to be the golden's, bit for bit, for Gentium and
// the builtin font over testdata/ttf/strings.txt (golden data first
// recorded from SolveSpace ttf.cpp).
func TestTTF(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")
	testDumpVsGolden(t, []string{filepath.Join(root, "Gentium-R.ttf")},
		filepath.Join(root, "ttf", "strings.txt"), filepath.Join(root, "ttf", "cpp-dump.txt.gz"))
}

// TestSynthFonts does the same for the test fonts in
// testdata/ttf/synth (from scripts/gen-ttf-testfonts.py), which reach
// the parts of the port that no system font does.
func TestSynthFonts(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")
	fonts, err := filepath.Glob(filepath.Join(root, "ttf", "synth", "*"))
	if err != nil || len(fonts) == 0 {
		t.Fatal("no test fonts", err)
	}
	testDumpVsGolden(t, fonts, filepath.Join(root, "ttf", "synth-strings.txt"),
		filepath.Join(root, "ttf", "cpp-synth-dump.txt.gz"))
}

func testDumpVsGolden(t *testing.T, fonts []string, strsFile, goldenFile string) {
	strs, err := os.ReadFile(strsFile)
	if err != nil {
		t.Fatal(err)
	}
	got := dump(fonts, lines(string(strs)))
	if golden.Regen() {
		var b bytes.Buffer
		zw, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
		if err == nil {
			_, err = zw.Write([]byte(got))
		}
		if err == nil {
			err = zw.Close()
		}
		if err != nil {
			t.Fatal(err)
		}
		golden.Check(t, goldenFile, b.Bytes())
		return
	}
	f, err := os.Open(goldenFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	want, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}

	wl, gl := strings.Split(string(want), "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if wl[i] != gl[i] {
			t.Fatalf("line %d differs:\nwant: %s\n got: %s", i+1, wl[i], gl[i])
		}
	}
	if len(wl) != len(gl) {
		t.Fatalf("the golden has %d lines, the dump %d", len(wl), len(gl))
	}
}

// A text request in the builtin font finds it.
func TestEmbeddedFontFound(t *testing.T) {
	if DefaultFonts.LoadFont(EmbeddedFont) == nil || DefaultFonts.AspectRatio(EmbeddedFont, "Abc", false) <= 0 {
		t.Errorf("the builtin font isn't found by the name text requests store, %q", EmbeddedFont)
	}
}

func lines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
